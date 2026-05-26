"""DevTrack Slack bot — lifecycle management and command dispatch.

Runs in Socket Mode (no public URL required) using slack_bolt.
All /devtrack <subcommand> slash commands are routed through a single
registered command; the subcommand text is parsed and dispatched here.
"""
from __future__ import annotations

import logging
from typing import Set

logger = logging.getLogger(__name__)


class SlackBot:
    """Manages the Slack App lifecycle and command dispatch."""

    def __init__(
        self,
        bot_token: str,
        app_token: str,
        allowed_channel_ids: Set[str] | None = None,
    ) -> None:
        from slack_bolt import App
        from slack_bolt.adapter.socket_mode import SocketModeHandler

        self.allowed_channel_ids: Set[str] = allowed_channel_ids or set()
        self.app = App(token=bot_token)
        self._handler = SocketModeHandler(self.app, app_token)
        self._register()

    # ------------------------------------------------------------------
    # Auth
    # ------------------------------------------------------------------

    def is_allowed(self, channel_id: str) -> bool:
        """Return True if channel_id is in the allow-list (or list is empty = dev mode)."""
        if not self.allowed_channel_ids:
            return True
        return channel_id in self.allowed_channel_ids

    # ------------------------------------------------------------------
    # Registration
    # ------------------------------------------------------------------

    def _register(self) -> None:
        from backend.slack.handlers import dispatch

        @self.app.command("/devtrack")
        def handle_devtrack(ack, command, respond):
            ack()
            channel = command.get("channel_id", "")
            if not self.is_allowed(channel):
                respond(":lock: This channel is not authorised to use DevTrack.")
                return
            text = command.get("text", "").strip()
            dispatch(text, respond, self)

        @self.app.event("app_mention")
        def handle_mention(event, say):
            """Handle @devtrack mentions in channels."""
            channel = event.get("channel", "")
            if not self.is_allowed(channel):
                return
            # Strip the mention text (e.g. "<@UBOT> workstatus")
            text = event.get("text", "")
            parts = text.split(None, 1)
            subtext = parts[1].strip() if len(parts) > 1 else "help"
            dispatch(subtext, say, self)

    # ------------------------------------------------------------------
    # Lifecycle
    # ------------------------------------------------------------------

    def run(self) -> None:
        """Start the Socket Mode handler (blocks)."""
        logger.info("Slack bot connected — listening for /devtrack commands")
        self._handler.start()

    # ------------------------------------------------------------------
    # Proactive notification helpers
    # ------------------------------------------------------------------

    def send_to_all_channels(self, text: str, blocks=None) -> None:
        """Post a message to every allowed channel (fire-and-forget)."""
        channels = self.allowed_channel_ids
        if not channels:
            logger.debug("No allowed channels configured — proactive notification skipped")
            return
        for channel in channels:
            try:
                kwargs: dict = {"channel": channel, "text": text}
                if blocks:
                    kwargs["blocks"] = blocks
                self.app.client.chat_postMessage(**kwargs)
            except Exception as e:
                logger.warning(f"Failed to post to channel {channel}: {e}")
