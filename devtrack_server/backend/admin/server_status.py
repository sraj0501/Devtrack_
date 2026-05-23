"""
Server status — aggregates process info and health checks for the admin dashboard.
Bridges server_tui.process_monitor and server_tui.health_client.
"""
from __future__ import annotations

from dataclasses import dataclass
from typing import Optional

from backend.config import (
    llm_provider as _llm_provider,
    ollama_model as _ollama_model,
    openai_model as _openai_model,
    get_webhook_port as _webhook_port,
    get_admin_port as _admin_port,
)


@dataclass
class ProcessStatus:
    name: str
    running: bool
    pid: Optional[int]
    cpu_percent: float
    mem_mb: float
    status: str


@dataclass
class ServiceStatus:
    name: str
    url: str
    ok: bool
    detail: str


@dataclass
class ServerSnapshot:
    processes: list[ProcessStatus]
    services: list[ServiceStatus]
    llm_provider: str
    llm_model: str
    webhook_port: int
    admin_port: int


def get_snapshot() -> ServerSnapshot:
    processes: list[ProcessStatus] = []
    services: list[ServiceStatus] = []

    try:
        from backend.server_tui.process_monitor import ProcessMonitor
        mon = ProcessMonitor()
        mon.refresh()
        for p in mon.processes:
            processes.append(ProcessStatus(
                name=p.name,
                running=p.running,
                pid=p.pid,
                cpu_percent=p.cpu_percent,
                mem_mb=p.mem_mb,
                status=p.status,
            ))
    except Exception:
        pass

    try:
        from backend.server_tui.health_client import check_all
        for h in check_all():
            services.append(ServiceStatus(
                name=h.name, url=h.url, ok=h.ok, detail=h.detail
            ))
    except Exception:
        pass

    return ServerSnapshot(
        processes=processes,
        services=services,
        llm_provider=_llm_provider() or "—",
        llm_model=_ollama_model() or _openai_model() or "—",
        webhook_port=_webhook_port(),
        admin_port=_admin_port(),
    )
