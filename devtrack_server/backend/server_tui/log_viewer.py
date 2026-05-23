"""
Log viewer — tails log files from LOG_DIR for display in the TUI.
"""
from __future__ import annotations

import os
from pathlib import Path
from typing import Optional


def _log_dir() -> Optional[Path]:
    from backend.config import log_dir, get_project_root
    try:
        return log_dir()
    except Exception:
        root = get_project_root()
        if root:
            return Path(root) / "Data" / "logs"
        return None


# Map display names → log file base names
LOG_FILES: dict[str, str] = {
    "python_bridge": "python_bridge.log",
    "webhook_server": "webhook_server.log",
    "telegram_bot":   "telegram.log",
    "alert_poller":   "alert_poller.log",
    "daemon":         "daemon.log",
}


def available_logs() -> dict[str, Path]:
    """Return {name: path} for log files that actually exist."""
    d = _log_dir()
    if not d:
        return {}
    result = {}
    for name, filename in LOG_FILES.items():
        p = d / filename
        if p.exists():
            result[name] = p
    return result


def tail(path: Path, lines: int = 200) -> list[str]:
    """Return the last `lines` lines of a file (efficient for large files)."""
    try:
        with open(path, "rb") as f:
            # Seek from end
            try:
                f.seek(0, 2)
                size = f.tell()
                block = min(size, lines * 120)  # rough estimate: 120 bytes/line
                f.seek(max(0, size - block))
                data = f.read()
            except OSError:
                data = f.read()
        text = data.decode("utf-8", errors="replace")
        all_lines = text.splitlines()
        return all_lines[-lines:]
    except (OSError, PermissionError):
        return []


class LogTailer:
    """Incrementally reads new lines appended to a log file."""

    def __init__(self, path: Path) -> None:
        self.path = path
        self._offset: int = 0
        # Start at end of file so we only show new lines
        try:
            self._offset = path.stat().st_size
        except OSError:
            self._offset = 0

    def read_new(self) -> list[str]:
        """Return any new lines since last call."""
        try:
            size = self.path.stat().st_size
        except OSError:
            return []
        if size < self._offset:
            # File was rotated/truncated
            self._offset = 0
        if size == self._offset:
            return []
        try:
            with open(self.path, "rb") as f:
                f.seek(self._offset)
                data = f.read(size - self._offset)
            self._offset = size
            return data.decode("utf-8", errors="replace").splitlines()
        except (OSError, PermissionError):
            return []
