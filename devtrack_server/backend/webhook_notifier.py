"""
DevTrack Webhook Notifier

Handles notification delivery when webhook events arrive from external systems.
Supports macOS OS notifications and terminal output.
"""

import asyncio
import logging
import platform
from typing import Optional

logger = logging.getLogger(__name__)

try:
    from backend import config
except ImportError:
    config = None


class WebhookNotifier:
    """Delivers notifications for incoming webhook events."""

    def __init__(self, notify_os: Optional[bool] = None, notify_terminal: Optional[bool] = None):
        if notify_os is not None:
            self._notify_os_enabled = notify_os
        elif config:
            self._notify_os_enabled = config.get_bool("WEBHOOK_NOTIFY_OS", True)
        else:
            self._notify_os_enabled = True

        if notify_terminal is not None:
            self._notify_terminal_enabled = notify_terminal
        elif config:
            self._notify_terminal_enabled = config.get_bool("WEBHOOK_NOTIFY_TERMINAL", True)
        else:
            self._notify_terminal_enabled = True

        self._is_macos = platform.system() == "Darwin"

    async def notify(self, title: str, body: str, source: str = "azure") -> None:
        """Send notification via all configured channels."""
        tasks = []
        if self._notify_terminal_enabled:
            tasks.append(self._notify_terminal(title, body, source))
        if self._notify_os_enabled and self._is_macos:
            tasks.append(self._notify_os(title, body))
        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)

    async def _notify_terminal(self, title: str, body: str, source: str) -> None:
        """Print formatted notification to terminal with bell."""
        tag = source.upper()
        # Bell char + colored output
        print(f"\a\033[1;36m[{tag}]\033[0m \033[1m{title}\033[0m: {body}")

    async def _notify_os(self, title: str, body: str) -> None:
        """Send macOS notification via osascript."""
        if not self._is_macos:
            return
        # Escape double quotes for AppleScript
        safe_title = title.replace('"', '\\"')
        safe_body = body.replace('"', '\\"')
        script = f'display notification "{safe_body}" with title "DevTrack: {safe_title}"'
        try:
            proc = await asyncio.create_subprocess_exec(
                "osascript", "-e", script,
                stdout=asyncio.subprocess.DEVNULL,
                stderr=asyncio.subprocess.DEVNULL,
            )
            await proc.wait()
        except Exception as e:
            logger.warning(f"Failed to send OS notification: {e}")
