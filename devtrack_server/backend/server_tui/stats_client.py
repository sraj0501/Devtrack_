"""
DevTrack Server TUI — trigger throughput stats client.

Boundary rule: Python must NOT read Go-owned SQLite tables directly in
PostgreSQL mode (separate-machine constraint). Two backends are provided:

  SQLite mode  (POSTGRES_URL unset)   — query the shared devtrack.db directly.
  PostgreSQL mode (POSTGRES_URL set)  — call GET /internal/stats on the Go
                                        daemon's internal HTTP server.

If either backend is unavailable (daemon not running, DB missing, etc.) a
zero-valued TriggerStats is returned — no crash.

Note this module's SQLite-mode split is *not* the fail-closed pattern used by
``skill_detector.py``/``dialectic_status.py`` — ``triggers`` is a Go-owned
table (see ``backend.db.engine``'s module docstring: Go-owned tables are
NEVER touched by Python in PostgreSQL mode), so PostgreSQL mode here calls the
Go daemon's internal HTTP endpoint instead of reading SQLite directly. Only
the SQLite-mode internals (``_query_sqlite``) go through
``backend.db.engine.get_engine()`` via SQLAlchemy ``text()`` instead of a
bespoke ``sqlite3.connect()`` — the PostgreSQL/HTTP branch is untouched.
"""
from __future__ import annotations

import logging
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Optional

from sqlalchemy import text
from sqlalchemy.exc import OperationalError

logger = logging.getLogger(__name__)


@dataclass
class TriggerStats:
    """Snapshot of trigger throughput for the last 24 hours / today."""

    triggers_today: int = 0
    commits_today: int = 0
    last_trigger: str = "—"   # "HH:MM" or "—" if none
    errors_24h: int = 0


# ---------------------------------------------------------------------------
# Public entry point
# ---------------------------------------------------------------------------

def get_trigger_stats() -> TriggerStats:
    """Return a TriggerStats snapshot; never raises."""
    from backend.config import postgres_url
    try:
        if postgres_url():
            return _stats_from_go_http()
        else:
            return _stats_from_sqlite()
    except Exception as exc:
        logger.debug("get_trigger_stats: unexpected error: %s", exc)
        return TriggerStats()


# ---------------------------------------------------------------------------
# Backend: Go internal HTTP (PostgreSQL / separate-machine mode)
# ---------------------------------------------------------------------------

def _go_internal_base_url() -> str:
    """URL for the Go daemon's internal HTTP server."""
    import os
    host = os.getenv("IPC_HOST", "127.0.0.1")
    port = os.getenv("DEVTRACK_SERVER_HTTP_PORT", "35894")
    return f"http://{host}:{port}"


def _stats_from_go_http() -> TriggerStats:
    """Call GET /internal/stats on the Go daemon.  Falls back to zeros."""
    try:
        import urllib.request, json as _json
        url = f"{_go_internal_base_url()}/internal/stats"
        with urllib.request.urlopen(url, timeout=2) as resp:
            data = _json.loads(resp.read())
        return TriggerStats(
            triggers_today=int(data.get("triggers_today", 0)),
            commits_today=int(data.get("commits_today", 0)),
            last_trigger=str(data.get("last_trigger", "—")),
            errors_24h=int(data.get("errors_24h", 0)),
        )
    except Exception as exc:
        logger.debug("stats_from_go_http: unavailable (%s) — returning zeros", exc)
        return TriggerStats()


# ---------------------------------------------------------------------------
# Backend: direct SQLite read (local / single-machine mode only)
# ---------------------------------------------------------------------------

def _db_path() -> Optional[Path]:
    try:
        from backend.config import database_path
        return database_path()
    except Exception:
        return None


def _stats_from_sqlite() -> TriggerStats:
    path = _db_path()
    if path is None or not path.exists():
        return TriggerStats()
    try:
        return _query_sqlite()
    except Exception as exc:
        logger.debug("stats_from_sqlite: %s", exc)
        return TriggerStats()


def _query_sqlite() -> TriggerStats:
    """Query the ``triggers`` table via the shared SQLAlchemy engine.

    ``triggers`` is a Go-owned table living in the same devtrack.db file the
    dual-dialect engine already points at in SQLite mode, so this reads
    through ``backend.db.engine.get_engine()`` instead of a bespoke
    ``sqlite3.connect()`` — same pattern as ``dialectic_status.py`` and
    ``skill_detector.py``. Each of the four stats is queried independently
    and defaults to its zero-value if that specific query fails (e.g. the
    table does not exist yet).
    """
    from backend.db.engine import get_engine

    now_utc = datetime.now(timezone.utc)
    today_str = now_utc.strftime("%Y-%m-%d")
    cutoff_24h = (now_utc - timedelta(hours=24)).strftime("%Y-%m-%d %H:%M:%S")
    error_cutoff = (now_utc - timedelta(minutes=5)).strftime("%Y-%m-%d %H:%M:%S")

    with get_engine().connect() as conn:
        triggers_today: int = 0
        try:
            row = conn.execute(
                text("SELECT COUNT(*) FROM triggers WHERE date(timestamp) = :today"),
                {"today": today_str},
            ).fetchone()
            triggers_today = int(row[0]) if row else 0
        except OperationalError:
            pass

        commits_today: int = 0
        try:
            row = conn.execute(
                text(
                    "SELECT COUNT(*) FROM triggers "
                    "WHERE trigger_type = 'commit' AND date(timestamp) = :today"
                ),
                {"today": today_str},
            ).fetchone()
            commits_today = int(row[0]) if row else 0
        except OperationalError:
            pass

        last_trigger: str = "—"
        try:
            row = conn.execute(
                text("SELECT timestamp FROM triggers ORDER BY timestamp DESC LIMIT 1")
            ).fetchone()
            if row and row[0]:
                ts_raw: str = row[0]
                for fmt in ("%Y-%m-%dT%H:%M:%SZ", "%Y-%m-%dT%H:%M:%S", "%Y-%m-%d %H:%M:%S"):
                    try:
                        dt = datetime.strptime(ts_raw[:19], fmt[:len(fmt)])
                        last_trigger = dt.strftime("%H:%M")
                        break
                    except ValueError:
                        continue
        except OperationalError:
            pass

        errors_24h: int = 0
        try:
            row = conn.execute(
                text(
                    """SELECT COUNT(*) FROM triggers
                       WHERE processed = 0
                         AND timestamp >= :cutoff_24h
                         AND timestamp <= :error_cutoff"""
                ),
                {"cutoff_24h": cutoff_24h, "error_cutoff": error_cutoff},
            ).fetchone()
            errors_24h = int(row[0]) if row else 0
        except OperationalError:
            pass

    return TriggerStats(
        triggers_today=triggers_today,
        commits_today=commits_today,
        last_trigger=last_trigger,
        errors_24h=errors_24h,
    )

_query_stats = _query_sqlite  # backwards-compatible alias used by tests
