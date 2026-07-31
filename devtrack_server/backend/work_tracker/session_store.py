"""
Interface for the work_sessions table (Go-owned).

The table is created and written by the Go daemon (database.go).
This module provides read/write access for the Python layer.

Boundary rule
-------------
work_sessions is a Go-owned table.  In PostgreSQL mode (POSTGRES_URL set),
Python and Go may run on different machines — Python MUST NOT open the Go
SQLite file directly.

  SQLite mode  (POSTGRES_URL unset)  — all methods use the shared SQLAlchemy
    engine (``backend.db.engine.get_engine()``) instead of a bespoke
    ``sqlite3.connect()``, but still read/write the same devtrack.db file.
  PostgreSQL mode (POSTGRES_URL set) — read methods call Go's internal HTTP
    endpoints where one exists (``get_active_session`` →
    GET /internal/sessions/active). Where no such endpoint exists yet
    (``get_sessions_for_date`` — Go only exposes the single active session
    over HTTP today, not a date-range query) the method fails closed and
    returns ``[]`` rather than silently reading a local SQLite file that may
    not reflect Go's real state on a separate machine — same pattern as
    ``dialectic_status.py``/``skill_detector.py`` for tables/queries with no
    HTTP equivalent. Write methods (append_commit, end_session, adjust_time)
    log a debug message and no-op; they require Go API support to work in
    multi-machine mode (deferred to a later phase).
"""

import json
import logging
import os
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from sqlalchemy import text

logger = logging.getLogger(__name__)


def _is_postgres_mode() -> bool:
    try:
        from backend.config import postgres_url
        return postgres_url() is not None
    except Exception:
        return False


def _go_internal_base_url() -> str:
    host = os.getenv("IPC_HOST", "127.0.0.1")
    port = os.getenv("DEVTRACK_SERVER_HTTP_PORT", "35894")
    return f"http://{host}:{port}"


class WorkSessionStore:
    """Sync-friendly wrapper around work_sessions SQLite table.

    Although DevTrack's Python layer is async, the shared engine is used
    synchronously. All methods run synchronous DB calls directly. Callers
    that need true async behaviour should wrap calls with ``asyncio.to_thread``.
    """

    # ------------------------------------------------------------------
    # Read helpers
    # ------------------------------------------------------------------

    def get_active_session(self) -> Optional[Dict[str, Any]]:
        """Return the most-recently started open session, or None.

        In PostgreSQL mode calls the Go daemon's /internal/sessions/active endpoint.
        In SQLite mode reads the shared devtrack.db directly.
        """
        if _is_postgres_mode():
            return self._get_active_session_via_http()
        return self._get_active_session_sqlite()

    def _get_active_session_via_http(self) -> Optional[Dict[str, Any]]:
        """Call GET /internal/sessions/active on the Go daemon."""
        try:
            import urllib.request, json as _json
            url = f"{_go_internal_base_url()}/internal/sessions/active"
            with urllib.request.urlopen(url, timeout=2) as resp:
                data = _json.loads(resp.read())
            if not data.get("active"):
                return None
            return {k: v for k, v in data.items() if k != "active"}
        except Exception as exc:
            logger.debug("get_active_session (http): %s", exc)
            return None

    def _get_active_session_sqlite(self) -> Optional[Dict[str, Any]]:
        try:
            from backend.db.engine import get_engine
            with get_engine().connect() as conn:
                row = conn.execute(
                    text(
                        """
                        SELECT * FROM work_sessions
                        WHERE ended_at IS NULL
                        ORDER BY started_at DESC
                        LIMIT 1
                        """
                    )
                ).mappings().fetchone()
            return dict(row) if row else None
        except Exception as exc:
            logger.debug("get_active_session (sqlite): %s", exc)
            return None

    # ------------------------------------------------------------------
    # Write helpers
    # ------------------------------------------------------------------

    def start_session(self, ticket_ref: str = "") -> Optional[int]:
        """Insert a new open session and return its id, or None on failure.

        No-op in PostgreSQL mode — work_sessions is a Go-owned table and
        cannot be written directly when Python and Go are on separate
        machines (same limitation as append_commit/end_session/adjust_time).
        """
        if _is_postgres_mode():
            logger.debug("start_session: skipped in PostgreSQL mode (Go-owned table)")
            return None
        try:
            from backend.db.engine import get_engine
            with get_engine().connect() as conn:
                result = conn.execute(
                    text(
                        "INSERT INTO work_sessions (started_at, ticket_ref, commits) "
                        "VALUES (datetime('now'), :ticket_ref, '[]')"
                    ),
                    {"ticket_ref": ticket_ref},
                )
                conn.commit()
                return result.lastrowid
        except Exception as e:
            logger.debug(f"WorkSessionStore.start_session error: {e}")
            return None

    def get_sessions_for_date(self, date: str) -> List[Dict[str, Any]]:
        """Return all sessions whose started_at date matches YYYY-MM-DD.

        Includes both completed and still-active sessions.

        Fails closed (returns []) in PostgreSQL mode: work_sessions is a
        Go-owned table and Go's internal HTTP server only exposes the single
        active session (GET /internal/sessions/active), not a date-range
        query — there is no Postgres-safe way to serve this today. See the
        boundary-rule note at the top of this module.
        """
        if _is_postgres_mode():
            logger.debug(
                "get_sessions_for_date: fail-closed in PostgreSQL mode — "
                "work_sessions is a Go-owned table with no date-range HTTP "
                "endpoint (only /internal/sessions/active exists)"
            )
            return []
        try:
            from backend.db.engine import get_engine
            with get_engine().connect() as conn:
                rows = conn.execute(
                    text(
                        "SELECT * FROM work_sessions WHERE date(started_at) = :date "
                        "ORDER BY started_at ASC"
                    ),
                    {"date": date},
                ).mappings().all()
            return [dict(r) for r in rows]
        except Exception as e:
            logger.debug(f"WorkSessionStore.get_sessions_for_date error: {e}")
            return []

    # ------------------------------------------------------------------
    # Write helpers
    # ------------------------------------------------------------------

    def append_commit(self, session_id: int, commit_hash: str) -> None:
        """Append *commit_hash* to the JSON commits array of a session.

        No-op in PostgreSQL mode — work_sessions is a Go-owned table and cannot
        be written directly when Python and Go are on separate machines.
        """
        if _is_postgres_mode():
            logger.debug("append_commit: skipped in PostgreSQL mode (Go-owned table)")
            return
        try:
            from backend.db.engine import get_engine
            with get_engine().connect() as conn:
                row = conn.execute(
                    text("SELECT commits FROM work_sessions WHERE id = :id"),
                    {"id": session_id},
                ).mappings().fetchone()
                if not row:
                    return

                commits: List[str] = []
                raw = (row["commits"] or "[]").strip()
                try:
                    commits = json.loads(raw)
                except json.JSONDecodeError:
                    commits = []

                if commit_hash not in commits:
                    commits.append(commit_hash)

                conn.execute(
                    text("UPDATE work_sessions SET commits = :commits WHERE id = :id"),
                    {"commits": json.dumps(commits), "id": session_id},
                )
                conn.commit()
        except Exception as e:
            logger.debug(f"WorkSessionStore.append_commit error: {e}")

    def end_session(self, session_id: int) -> None:
        """Mark a session as ended, computing duration from started_at.

        No-op in PostgreSQL mode — work_sessions is a Go-owned table.
        """
        if _is_postgres_mode():
            logger.debug("end_session: skipped in PostgreSQL mode (Go-owned table)")
            return
        try:
            from backend.db.engine import get_engine
            with get_engine().connect() as conn:
                row = conn.execute(
                    text("SELECT started_at FROM work_sessions WHERE id = :id"),
                    {"id": session_id},
                ).mappings().fetchone()
                if not row:
                    return

                started_at = row["started_at"]
                try:
                    start = datetime.fromisoformat(started_at)
                except ValueError:
                    start = datetime.strptime(started_at, "%Y-%m-%d %H:%M:%S")
                now = datetime.now(timezone.utc).replace(tzinfo=None)
                duration_mins = max(0, int((now - start).total_seconds() / 60))
                ended_at = now.strftime("%Y-%m-%d %H:%M:%S")

                conn.execute(
                    text(
                        "UPDATE work_sessions SET ended_at = :ended_at, "
                        "duration_minutes = :duration_minutes WHERE id = :id"
                    ),
                    {"ended_at": ended_at, "duration_minutes": duration_mins, "id": session_id},
                )
                conn.commit()
        except Exception as e:
            logger.debug(f"WorkSessionStore.end_session error: {e}")

    def adjust_time(self, session_id: int, adjusted_minutes: int) -> None:
        """Set user-overridden time. Auto-measured duration_minutes is preserved.

        No-op in PostgreSQL mode — work_sessions is a Go-owned table and cannot
        be written directly when Python and Go are on separate machines (same
        boundary rule as append_commit/end_session; this guard was previously
        missing here — see TASK-112 module 4/15 port notes).
        """
        if _is_postgres_mode():
            logger.debug("adjust_time: skipped in PostgreSQL mode (Go-owned table)")
            return
        try:
            from backend.db.engine import get_engine
            with get_engine().connect() as conn:
                conn.execute(
                    text(
                        "UPDATE work_sessions SET adjusted_minutes = :adjusted_minutes "
                        "WHERE id = :id"
                    ),
                    {"adjusted_minutes": adjusted_minutes, "id": session_id},
                )
                conn.commit()
        except Exception as e:
            logger.debug(f"WorkSessionStore.adjust_time error: {e}")

    # ------------------------------------------------------------------
    # Effective duration helper
    # ------------------------------------------------------------------

    @staticmethod
    def effective_duration(session: Dict[str, Any]) -> int:
        """Return adjusted_minutes if set, else duration_minutes, else 0."""
        adj = session.get("adjusted_minutes")
        if adj is not None:
            return int(adj)
        dur = session.get("duration_minutes")
        return int(dur) if dur is not None else 0
