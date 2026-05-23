"""
DevTrack Telemetry — opt-in usage analytics.

Rules:
  - Completely no-op if user is not logged in
  - Completely no-op if session.telemetry_enabled is False
  - Never collects: code, file contents, commit messages, credentials
  - Fires async so it never blocks CLI commands
  - Batches events and flushes on daemon shutdown or every FLUSH_INTERVAL_SECS

Usage:
    from backend.telemetry import record

    record("command.run", command="devtrack start")
    record("feature.llm_enhance", provider="ollama")
    record("error", code="IPC_TIMEOUT")
"""

from __future__ import annotations

import os
import threading
import time
from datetime import datetime, timezone
from typing import Any, Optional

# ── Config ────────────────────────────────────────────────────────────────────

FLUSH_INTERVAL_SECS = 300   # 5 minutes
MAX_BATCH_SIZE = 50


# ── Internal state ────────────────────────────────────────────────────────────

_event_queue: list[dict] = []
_queue_lock = threading.Lock()
_flush_thread: Optional[threading.Thread] = None
_shutdown = threading.Event()


# ── Helpers ───────────────────────────────────────────────────────────────────

def _is_enabled() -> bool:
    """Return True only if user is logged in with telemetry opt-in."""
    try:
        from backend.auth.session import get_session
        session = get_session()
        return session is not None and session.telemetry_enabled
    except Exception:
        return False


def _api_url() -> Optional[str]:
    from backend.config import get_devtrack_api_url
    return get_devtrack_api_url() or None


def _devtrack_version() -> str:
    try:
        from importlib.metadata import version
        return version("devtrack")
    except Exception:
        from backend.config import get_devtrack_version
        return get_devtrack_version()


# ── Public API ────────────────────────────────────────────────────────────────

def record(event_type: str, **props: Any) -> None:
    """
    Queue a telemetry event. No-op if telemetry is disabled.

    Args:
        event_type: dot-separated event name (e.g. "command.run", "feature.nlp")
        **props:    additional safe, non-PII properties
    """
    if not _is_enabled():
        return

    # Strip any accidentally passed sensitive keys
    _BLOCKED = {"token", "password", "key", "secret", "credential", "message", "content", "diff"}
    safe_props = {k: v for k, v in props.items() if k.lower() not in _BLOCKED}

    event = {
        "type": event_type,
        "ts": datetime.now(timezone.utc).isoformat(),
        "version": _devtrack_version(),
        **safe_props,
    }

    with _queue_lock:
        _event_queue.append(event)
        if len(_event_queue) >= MAX_BATCH_SIZE:
            _flush_sync()


def flush() -> None:
    """Flush pending events to the telemetry endpoint synchronously."""
    if not _is_enabled():
        return
    with _queue_lock:
        _flush_sync()


def _flush_sync() -> None:
    """Must be called with _queue_lock held."""
    global _event_queue
    if not _event_queue or not _api_url():
        _event_queue = []
        return

    batch = list(_event_queue)
    _event_queue = []

    # Fire and forget in a daemon thread so we don't block
    threading.Thread(
        target=_send_batch,
        args=(batch,),
        daemon=True,
        name="devtrack-telemetry-flush",
    ).start()


def _send_batch(events: list[dict]) -> None:
    """Send a batch of events to the telemetry endpoint. Silently drops on error."""
    try:
        import httpx
        session_email = ""
        try:
            from backend.auth.session import get_session
            s = get_session()
            if s:
                session_email = s.user_id  # opaque ID, not email
        except Exception:
            pass

        httpx.post(
            f"{_api_url()}/telemetry/batch",
            json={"user_id": session_email, "events": events},
            timeout=5,
        )
    except Exception:
        pass  # Telemetry failures must never surface to the user


# ── Background flush thread ───────────────────────────────────────────────────

def start_background_flush() -> None:
    """Start the background flush thread. Call once at daemon startup."""
    global _flush_thread
    if _flush_thread and _flush_thread.is_alive():
        return
    _shutdown.clear()
    _flush_thread = threading.Thread(
        target=_flush_loop,
        daemon=True,
        name="devtrack-telemetry",
    )
    _flush_thread.start()


def stop_background_flush() -> None:
    """Signal the flush thread to stop and do a final flush."""
    _shutdown.set()
    flush()


def _flush_loop() -> None:
    while not _shutdown.wait(timeout=FLUSH_INTERVAL_SECS):
        flush()
