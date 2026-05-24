"""DevTrack Slack notifier — proactive notifications from webhook_server.py.

Called when timer triggers fire and no TUI is available.
The bot instance is retrieved via a module-level singleton so webhook_server
(a different process) can use it via direct function calls if co-located,
or the webhook server can skip this if Slack bot is a separate process.

In practice, webhook_server calls send_work_reminder() which posts to the
Slack channels configured in SLACK_ALLOWED_CHANNEL_IDS using the Bot Token
directly (no shared state needed).
"""
from __future__ import annotations

import logging
import os

logger = logging.getLogger(__name__)


def send_work_reminder(
    interval_mins: int = 0,
    trigger_count: int = 0,
    active_session: dict | None = None,
    pm_platform: str = "",
    workspace_name: str = "",
) -> bool:
    """Post a work reminder to all configured Slack channels.

    Returns True if at least one message was delivered.
    """
    from backend.config import get_slack_bot_token, get_slack_allowed_channel_ids
    bot_token = get_slack_bot_token()
    channel_ids = get_slack_allowed_channel_ids()
    if not bot_token or not channel_ids:
        return False

    if not channel_ids:
        return False

    lines = [":alarm_clock: *Work update reminder*"]
    if workspace_name:
        lines.append(f"Workspace: {workspace_name}")
    if pm_platform:
        lines.append(f"Platform: {pm_platform}")

    if active_session:
        sid = active_session.get("id", "?")
        ticket = active_session.get("ticket_ref", "")
        lines.append(f":green_circle: Active session #{sid}" + (f" — `{ticket}`" if ticket else ""))
        lines.append("Use `/devtrack workstop` to stop it or `/devtrack workreport` for an EOD summary.")
    else:
        lines.append("No active session. Use `/devtrack workstart [ticket-ref]` to start tracking.")

    text = "\n".join(lines)

    try:
        from slack_sdk import WebClient
        client = WebClient(token=bot_token)
        sent = False
        for channel in channel_ids:
            try:
                client.chat_postMessage(channel=channel, text=text)
                sent = True
            except Exception as e:
                logger.warning(f"Slack reminder to {channel} failed: {e}")
        return sent
    except ImportError:
        logger.debug("slack_sdk not installed — Slack reminder skipped")
        return False
    except Exception as e:
        logger.warning(f"Slack notifier error: {e}")
        return False
