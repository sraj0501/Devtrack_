"""
Process monitor — discovers and tracks DevTrack server processes via psutil.

Only server-side processes are tracked here. Client-side components
(telegram_bot, alert_poller) are Go-native goroutines that belong to whichever
client(s) are connected — they are visible via `devtrack status` on each client,
not here. Listing them server-side is misleading in multi-client deployments
because we can only see the co-located client (if any), not remote clients.
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


# Server-side processes only — client goroutines (telegram_bot, alert_poller)
# are intentionally excluded: they belong to connected clients, not this server.
MANAGED_PROCESSES: list[dict] = [
    {
        "name": "webhook_server",
        "pattern": "webhook_server",
        "restart_cmd": [sys.executable, "-m", "uvicorn", "backend.webhook_server:app",
                        "--host", "0.0.0.0", "--port", "8089"],
    },
]


class ProcessMonitor:
    """Snapshot current state of all managed DevTrack processes."""

    def __init__(self) -> None:
        self._procs: dict[str, ProcessInfo] = {
            d["name"]: ProcessInfo(
                name=d["name"],
                pattern=d.get("pattern", ""),
                restart_cmd=d.get("restart_cmd", []),
            )
            for d in MANAGED_PROCESSES
        }

    @property
    def processes(self) -> list[ProcessInfo]:
        return list(self._procs.values())

    def refresh(self) -> None:
        """Walk running processes and update stored state."""
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
                    if not info.pattern:
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

    def get(self, name: str) -> Optional[ProcessInfo]:
        return self._procs.get(name)

    def restart(self, name: str) -> bool:
        """Kill existing process (if any) and spawn a fresh one. Returns True on success."""
        info = self._procs.get(name)
        if not info or not info.restart_cmd:
            return False

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
