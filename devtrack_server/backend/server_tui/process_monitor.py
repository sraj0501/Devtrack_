"""
Process monitor — discovers and tracks DevTrack server processes via psutil,
with a fallback to the Go daemon's /internal/components API for Go-native
components (alert_poller, telegram_bot) that are no longer Python subprocesses.
"""
from __future__ import annotations

import subprocess
import sys
from dataclasses import dataclass, field
from typing import Optional

import psutil

try:
    import urllib.request
    import json as _json
    _urllib_available = True
except ImportError:
    _urllib_available = False


@dataclass
class ProcessInfo:
    name: str           # display name
    pattern: str        # substring matched against cmdline (empty for Go-native)
    pid: Optional[int] = None
    status: str = "stopped"   # running | stopped | sleeping | zombie | go-native
    cpu_percent: float = 0.0
    mem_mb: float = 0.0
    restart_cmd: list[str] = field(default_factory=list)
    go_native: bool = False   # True = managed by Go daemon, not a Python subprocess

    @property
    def running(self) -> bool:
        return self.status not in ("stopped", "zombie", "dead")


# Processes the TUI knows about, in display order.
# `pattern` is matched against the full space-joined cmdline string.
# Go-native entries have pattern="" and go_native=True — their state comes
# from GET http://127.0.0.1:35894/internal/components instead of psutil.
MANAGED_PROCESSES: list[dict] = [
    {
        "name": "webhook_server",
        "pattern": "webhook_server",
        "restart_cmd": [sys.executable, "-m", "uvicorn", "backend.webhook_server:app",
                        "--host", "0.0.0.0", "--port", "8089"],
    },
    {
        "name": "telegram_bot",
        "pattern": "",
        "restart_cmd": [],
        "go_native": True,
    },
    {
        "name": "alert_poller",
        "pattern": "",
        "restart_cmd": [],
        "go_native": True,
    },
]


def _query_go_components() -> dict:
    """Query the Go daemon's internal API for Go-native component states."""
    if not _urllib_available:
        return {}
    try:
        import os
        port = os.environ.get("DEVTRACK_SERVER_HTTP_PORT", "35894")
        url = f"http://127.0.0.1:{port}/internal/components"
        with urllib.request.urlopen(url, timeout=2) as resp:
            return _json.loads(resp.read())
    except Exception:
        return {}


class ProcessMonitor:
    """Snapshot current state of all managed DevTrack processes."""

    def __init__(self) -> None:
        self._procs: dict[str, ProcessInfo] = {
            d["name"]: ProcessInfo(
                name=d["name"],
                pattern=d.get("pattern", ""),
                restart_cmd=d.get("restart_cmd", []),
                go_native=d.get("go_native", False),
            )
            for d in MANAGED_PROCESSES
        }

    @property
    def processes(self) -> list[ProcessInfo]:
        return list(self._procs.values())

    def refresh(self) -> None:
        """Walk running processes and update stored state."""
        # Reset all
        for info in self._procs.values():
            info.pid = None
            info.status = "stopped"
            info.cpu_percent = 0.0
            info.mem_mb = 0.0

        # Update psutil-tracked processes
        for proc in psutil.process_iter(["pid", "name", "cmdline", "status", "cpu_percent",
                                          "memory_info"]):
            try:
                cmdline = " ".join(proc.info["cmdline"] or [])
                if not cmdline:
                    continue
                for info in self._procs.values():
                    if info.go_native or not info.pattern:
                        continue
                    if info.pattern in cmdline:
                        info.pid = proc.info["pid"]
                        info.status = proc.info["status"] or "running"
                        info.cpu_percent = proc.info["cpu_percent"] or 0.0
                        mem = proc.info["memory_info"]
                        info.mem_mb = (mem.rss / 1024 / 1024) if mem else 0.0
                        break
            except (psutil.NoSuchProcess, psutil.AccessDenied):
                continue

        # Update Go-native components from the daemon's internal API
        go_state = _query_go_components()
        for info in self._procs.values():
            if not info.go_native:
                continue
            comp = go_state.get(info.name, {})
            if comp.get("running"):
                info.status = "running"
            else:
                # Distinguish "daemon down" from "component disabled"
                info.status = "stopped"

    def get(self, name: str) -> Optional[ProcessInfo]:
        return self._procs.get(name)

    def restart(self, name: str) -> bool:
        """Kill existing process (if any) and spawn a fresh one. Returns True on success."""
        info = self._procs.get(name)
        if not info:
            return False

        if info.go_native:
            return False  # Go-native components are managed by the daemon

        # Kill existing
        if info.pid:
            try:
                p = psutil.Process(info.pid)
                p.terminate()
                try:
                    p.wait(timeout=5)
                except psutil.TimeoutExpired:
                    p.kill()
            except (psutil.NoSuchProcess, psutil.AccessDenied):
                pass

        if not info.restart_cmd:
            return False

        try:
            subprocess.Popen(
                info.restart_cmd,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                start_new_session=True,
            )
            return True
        except Exception:
            return False

    def stop(self, name: str) -> bool:
        """Terminate a process by name."""
        info = self._procs.get(name)
        if not info or not info.pid:
            return False
        if info.go_native:
            return False  # Go-native components are managed by the daemon
        try:
            p = psutil.Process(info.pid)
            p.terminate()
            try:
                p.wait(timeout=5)
            except psutil.TimeoutExpired:
                p.kill()
            return True
        except (psutil.NoSuchProcess, psutil.AccessDenied):
            return False
