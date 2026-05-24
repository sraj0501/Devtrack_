"""Telegram bot for DevTrack remote control."""
import asyncio
import logging
import os
import threading
from typing import Optional, Set

from telegram import Update
from telegram.ext import Application, CommandHandler, ContextTypes

from backend.ipc_client import IPCClient, MessageType, IPCMessage

logger = logging.getLogger(__name__)


class DevTrackBot:
    """Telegram bot that provides remote control of DevTrack daemon."""

    def __init__(self, token: str, allowed_chat_ids: Set[int]):
        self.token = token
        self.allowed_chat_ids = allowed_chat_ids
        self.app: Optional[Application] = None
        self.ipc_client: Optional[IPCClient] = None
        self._loop: Optional[asyncio.AbstractEventLoop] = None

    def _is_authorized(self, update: Update) -> bool:
        """Check if the chat is authorized."""
        if not self.allowed_chat_ids:
            return True  # No whitelist = allow all (dev mode)
        chat_id = update.effective_chat.id
        return chat_id in self.allowed_chat_ids

    async def _check_auth(self, update: Update) -> bool:
        """Check auth and send denial if unauthorized."""
        if not self._is_authorized(update):
            await update.message.reply_text("Unauthorized. Your chat ID is not in the allowed list.")
            logger.warning(f"Unauthorized access attempt from chat_id={update.effective_chat.id}")
            return False
        return True

    def _connect_ipc(self):
        """Connect to the Go daemon IPC server."""
        try:
            self.ipc_client = IPCClient()
            if self.ipc_client.connect(timeout=5, retry_count=3):
                # Register handlers for events we want to forward to Telegram
                self.ipc_client.register_handler(MessageType.COMMIT_TRIGGER, self._on_ipc_event)
                self.ipc_client.register_handler(MessageType.TIMER_TRIGGER, self._on_ipc_event)
                self.ipc_client.register_handler(MessageType.WEBHOOK_EVENT, self._on_ipc_event)
                self.ipc_client.register_handler(MessageType.REPORT_TRIGGER, self._on_ipc_event)
                self.ipc_client.start_listening()
                logger.info("Connected to DevTrack IPC server")
                return True
            else:
                logger.warning("Failed to connect to IPC server -- bot will run without live events")
                return False
        except Exception as e:
            logger.warning(f"IPC connection failed: {e} -- bot will run without live events")
            return False

    def _on_ipc_event(self, message: IPCMessage):
        """Handle IPC events from Go daemon -- bridge to asyncio."""
        if self._loop is None:
            return
        asyncio.run_coroutine_threadsafe(
            self._broadcast_event(message),
            self._loop
        )

    async def _broadcast_event(self, message: IPCMessage):
        """Send IPC event to all authorized Telegram chats."""
        text = _format_ipc_event(message)
        if not text:
            return
        for chat_id in self.allowed_chat_ids:
            try:
                await self.app.bot.send_message(chat_id=chat_id, text=text, parse_mode="Markdown")
            except Exception as e:
                logger.error(f"Failed to send notification to {chat_id}: {e}")

    async def start(self):
        """Start the bot."""
        self.app = Application.builder().token(self.token).build()
        self._loop = asyncio.get_event_loop()

        # Register command handlers
        from backend.telegram.handlers import register_handlers
        register_handlers(self.app, self)

        # Connect IPC in background thread
        threading.Thread(target=self._connect_ipc, daemon=True).start()

        logger.info("Starting Telegram bot...")
        await self.app.initialize()
        await self.app.start()
        await self.app.updater.start_polling(drop_pending_updates=True)
        logger.info("Telegram bot started")

    async def stop(self):
        """Stop the bot."""
        if self.ipc_client:
            self.ipc_client.stop_listening()
            self.ipc_client.disconnect()
        if self.app:
            await self.app.updater.stop()
            await self.app.stop()
            await self.app.shutdown()
        logger.info("Telegram bot stopped")

    def run(self):
        """Run the bot (blocking). For use as main entry point."""
        asyncio.run(self._run_forever())

    async def _run_forever(self):
        """Start and run until interrupted."""
        await self.start()
        try:
            # Keep running until interrupted
            stop_event = asyncio.Event()
            await stop_event.wait()
        except (KeyboardInterrupt, SystemExit):
            pass
        finally:
            await self.stop()


def _format_ipc_event(message: IPCMessage) -> str:
    """Format an IPC event for Telegram display. Returns empty string to suppress."""
    data = message.data or {}
    from backend.config import get_telegram_notify_commits, get_telegram_notify_triggers, get_telegram_notify_health
    if message.type == MessageType.COMMIT_TRIGGER:
        if not get_telegram_notify_commits():
            return ""
        branch = data.get("branch", "")
        msg = data.get("message", "")
        text = "*Commit detected*\n"
        if branch:
            text += f"Branch: `{branch}`\n"
        if msg:
            if len(msg) > 200:
                msg = msg[:200] + "..."
            text += f"Message: {msg}\n"
        return text
    elif message.type == MessageType.TIMER_TRIGGER:
        if not get_telegram_notify_triggers():
            return ""
        return "*Timer trigger fired* -- work update prompt sent"
    elif message.type == MessageType.REPORT_TRIGGER:
        if not get_telegram_notify_triggers():
            return ""
        return "*Report generation triggered*"
    elif message.type == MessageType.WEBHOOK_EVENT:
        if not get_telegram_notify_health():
            return ""
        source = data.get("source", "unknown")
        event = data.get("event_type", "unknown")
        title = data.get("title", "")
        return f"*Webhook: {source}* -- {event}\n{title}"
    return ""
