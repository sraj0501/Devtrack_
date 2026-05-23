"""
Alert notifier for DevTrack.

Delivers notifications via:
  - macOS OS notification: osascript
  - Terminal: formatted print to stdout

Respects ALERT_NOTIFY_* env vars from backend.config.
"""

from __future__ import annotations

import logging
import platform
import shutil
import subprocess
import sys
from datetime import datetime
from typing import Any, Dict, Optional

import backend.config as cfg

logger = logging.getLogger(__name__)
log = logging.getLogger(__name__)


def _should_notify(event_type: str) -> bool:
    """Return True if this event_type is enabled via config."""
    event_type = event_type.lower()
    if event_type == "assigned":
        return cfg.get_bool("ALERT_NOTIFY_ASSIGNED", True)
    if event_type == "comment":
        return cfg.get_bool("ALERT_NOTIFY_COMMENTS", True)
    if event_type in ("status_change", "status-change"):
        return cfg.get_bool("ALERT_NOTIFY_STATUS_CHANGES", True)
    if event_type in ("review_requested", "review-requested"):
        return cfg.get_bool("ALERT_NOTIFY_REVIEW_REQUESTED", True)
    # Default: allow unknown types through
    return True


def _os_notify(title: str, subtitle: str, message: str) -> bool:
    """
    Send a macOS notification via osascript.

    Returns True on success, False if osascript not available or failed.
    """
    if not shutil.which("osascript"):
        return False
    script = (
        f'display notification {_osa_quote(message)} '
        f'with title {_osa_quote(title)} '
        f'subtitle {_osa_quote(subtitle)}'
    )
    try:
        subprocess.run(
            ["osascript", "-e", script],
            check=True,
            capture_output=True,
            timeout=5,
        )
        return True
    except Exception as e:
        logger.debug(f"osascript notification failed: {e}")
        return False


def _osa_quote(text: str) -> str:
    """Wrap a string in AppleScript double-quotes, escaping backslash and quote."""
    escaped = text.replace("\\", "\\\\").replace('"', '\\"')
    return f'"{escaped}"'


def _terminal_notify(notification: Dict[str, Any]) -> None:
    """Print a formatted notification to stdout."""
    source = notification.get("source", "").upper()
    event_type = notification.get("event_type", "")
    title = notification.get("title", "")
    summary = notification.get("summary", "")
    url = notification.get("url", "")
    ts = notification.get("timestamp")

    ts_str = ""
    if isinstance(ts, datetime):
        ts_str = ts.strftime("%H:%M")
    elif isinstance(ts, str):
        ts_str = ts[:16]

    icon = _event_icon(event_type)
    print(f"\n{icon} [{source}] {event_type.upper()}")
    print(f"  {title}")
    if summary:
        print(f"  {summary}")
    if url:
        print(f"  {url}")
    if ts_str:
        print(f"  {ts_str}")


def _event_icon(event_type: str) -> str:
    icons = {
        "assigned": "-->",
        "comment": "  [C]",
        "review_requested": " [R]",
        "status_change": " [S]",
    }
    return icons.get(event_type.lower(), " [!]")


def notify(notification: Dict[str, Any]) -> None:
    """
    Deliver a single notification via all enabled channels.

    ``notification`` should be a dict matching the notifications collection schema:
    {
        source, event_type, ticket_id, title, summary, url,
        timestamp, read, dismissed, raw
    }
    """
    event_type = notification.get("event_type", "")
    if not _should_notify(event_type):
        logger.debug(f"Notification suppressed by config: {event_type}")
        return

    title = notification.get("title", "DevTrack Alert")
    summary = notification.get("summary", "")
    source = notification.get("source", "").upper()

    if cfg.get_bool("ALERT_NOTIFY_TERMINAL", True):
        _terminal_notify(notification)

    if cfg.get_bool("ALERT_NOTIFY_OS", True):
        subtitle = f"[{source}] {event_type.replace('_', ' ').title()}"
        _os_notify("DevTrack", subtitle, f"{title}: {summary}" if summary else title)


def notify_many(notifications: list) -> None:
    """Deliver multiple notifications."""
    for n in notifications:
        try:
            notify(n)
        except Exception as e:
            logger.warning(f"Failed to deliver notification: {e}")


# ---------------------------------------------------------------------------
# Cross-platform notification class (TASK-023)
# ---------------------------------------------------------------------------


class AlertNotifier:
    """Cross-platform desktop notification dispatcher.

    Dispatch order:
    - macOS:   osascript -> plyer fallback
    - Linux:   notify-send -> plyer fallback
    - Windows: plyer -> PowerShell Toast fallback
    """

    def notify(self, title: str, message: str, url: str = "") -> bool:
        system = platform.system()
        try:
            if system == "Darwin":
                return self._notify_macos(title, message, url)
            elif system == "Windows":
                return self._notify_windows(title, message)
            else:
                return self._notify_linux(title, message)
        except Exception as exc:
            log.warning("Notification delivery failed: %s", exc)
            return False

    def _notify_macos(self, title: str, message: str, url: str) -> bool:
        script = f'display notification "{message}" with title "{title}"'
        result = subprocess.run(
            ["osascript", "-e", script],
            capture_output=True,
            timeout=self._timeout(),
        )
        if result.returncode == 0:
            return True
        return self._notify_plyer(title, message)

    def _notify_linux(self, title: str, message: str) -> bool:
        result = subprocess.run(
            ["notify-send", title, message],
            capture_output=True,
            timeout=self._timeout(),
        )
        if result.returncode == 0:
            return True
        return self._notify_plyer(title, message)

    def _notify_windows(self, title: str, message: str) -> bool:
        if self._notify_plyer(title, message):
            return True
        return self._notify_windows_powershell(title, message)

    def _notify_plyer(self, title: str, message: str) -> bool:
        try:
            from plyer import notification as plyer_notification  # type: ignore
            plyer_notification.notify(title=title, message=message, timeout=5)
            return True
        except ImportError:
            return False
        except Exception as exc:
            log.debug("plyer notification failed: %s", exc)
            return False

    def _notify_windows_powershell(self, title: str, message: str) -> bool:
        ps_script = (
            "[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, "
            "ContentType = WindowsRuntime] | Out-Null; "
            "$template = [Windows.UI.Notifications.ToastNotificationManager]"
            "::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02); "
            f'$template.SelectSingleNode(\"//text[@id=\'1\']\").InnerText = \"{title}\"; '
            f'$template.SelectSingleNode(\"//text[@id=\'2\']\").InnerText = \"{message}\"; '
            "$toast = [Windows.UI.Notifications.ToastNotification]::new($template); "
            "[Windows.UI.Notifications.ToastNotificationManager]"
            "::CreateToastNotifier('DevTrack').Show($toast)"
        )
        result = subprocess.run(
            ["powershell", "-NonInteractive", "-Command", ps_script],
            capture_output=True,
            timeout=self._timeout(),
        )
        return result.returncode == 0

    def _timeout(self) -> int:
        import backend.config as _cfg
        return _cfg.http_timeout_short()
