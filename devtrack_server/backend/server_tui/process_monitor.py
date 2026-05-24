"""
Process monitor — discovers and tracks DevTrack server processes via psutil.

Each "managed process" is identified by a cmdline pattern rather than a stored PID
so the monitor works even after restarts and doesn't need to own the processes.
"""
from __future__ import annotations

import subprocess
import sys
from dataclasses import dataclass, field
from typing import Optional

import psutil


@dataclass
class ProcessInfo:
    name: str           # display name
    pattern: str        # substring matched against cmdline
    pid: Optional[int] = None
    status: str = "stopped"   # running | stopped | sleeping | zombie
    cpu_percent: float = 0.0
    mem_mb: float = 0.0
    restart_cmd: list[str] = field(default_factory=list)

    @property
    def running(self) -> bool:
        return self.status not in ("stopped", "zombie", "dead")


# Processes the TUI knows about, in display order.
# `pattern` is matched against the full space-joined cmdline string.
MANAGED_PROCESSES: list[dict] = [
    {
        "name": "python_bridge",
        "pattern": "python_bridge.py",
        "restart_cmd": [sys.executable, "python_bridge.py"],
    },
    {
        "name": "webhook_server",
        "pattern": "webhook_server",
        "restart_cmd": [sys.executable, "-m", "uvicorn", "backend.webhook_server:app",
                        "--host", "0.0.0.0", "--port", "8089"],
    },
    {
        "name": "telegram_bot",
        "pattern": "backend.telegram",
        "restart_cmd": [sys.executable, "-m", "backend.telegram"],
    },
    {
        "name": "alert_poller",
        "pattern": "alert_poller",
        "restart_cmd": [sys.executable, "-m", "backend.alert_poller"],
    },
]


class ProcessMonitor:
    """Snapshot current state of all managed DevTrack processes."""

    def __init__(self) -> None:
        self._procs: dict[str, ProcessInfo] = {
            d["name"]: ProcessInfo(
                name=d["name"],
                pattern=d["pattern"],
                restart_cmd=d["restart_cmd"],
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

        for proc in psutil.process_iter(["pid", "name", "cmdline", "status", "cpu_percent",
                                          "memory_info"]):
            try:
                cmdline = " ".join(proc.info["cmdline"] or [])
                if not cmdline:
                    continue
                for info in self._procs.values():
                    if info.pattern in cmdline:
                        info.pid = proc.info["pid"]
                        info.status = proc.info["status"] or "running"
                        info.cpu_percent = proc.info["cpu_percent"] or 0.0
                        mem = proc.info["memory_info"]
                        info.mem_mb = (mem.rss / 1024 / 1024) if mem else 0.0
                        break
            except (psutil.NoSuchProcess, psutil.AccessDenied):
                continue

    def get(self, name: str) -> Optional[ProcessInfo]:
        return self._procs.get(name)

    def restart(self, name: str) -> bool:
        """Kill existing process (if any) and spawn a fresh one. Returns True on success."""
        info = self._procs.get(name)
        if not info:
            return False

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
