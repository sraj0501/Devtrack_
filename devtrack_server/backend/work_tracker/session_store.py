"""
Async SQLite interface for the work_sessions table.

The table is managed by the Go daemon (database.go) which creates it on startup.
This module provides a read/write interface for the Python layer so commit
auto-linking and EOD report generation can access session data.
"""

import json
import logging
import sqlite3
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)


def _db_path() -> str:
    try:
        from backend.config import database_path
        return str(database_path())
    except Exception:
        from backend.config import get_project_root
        import os
        root = get_project_root() or "."
        return os.path.join(root, "Data", "db", "devtrack.db")


def _connect() -> sqlite3.Connection:
    conn = sqlite3.connect(_db_path())
    conn.row_factory = sqlite3.Row
    return conn


class WorkSessionStore:
    """Sync-friendly wrapper around work_sessions SQLite table.

    Although DevTrack's Python layer is async, sqlite3 is not natively async.
    All methods run synchronous SQLite calls directly. Callers that need true
    async behaviour should wrap calls with ``asyncio.to_thread``.
    """

    # ------------------------------------------------------------------
    # Read helpers
    # ------------------------------------------------------------------

    def get_active_session(self) -> Optional[Dict[str, Any]]:
        """Return the most-recently started open session, or None."""
        try:
            conn = _connect()
            row = conn.execute(
                """
                SELECT * FROM work_sessions
                WHERE ended_at IS NULL
                ORDER BY started_at DESC
                LIMIT 1
                """
            ).fetchone()
            conn.close()
            return dict(row) if row else None
        except Exception as e:
            logger.debug(f"WorkSessionStore.get_active_session error: {e}")
            return None

    def get_sessions_for_date(self, date: str) -> List[Dict[str, Any]]:
        """Return all sessions whose started_at date matches YYYY-MM-DD.

        Includes both completed and still-active sessions.
        """
        try:
            conn = _connect()
            rows = conn.execute(
                "SELECT * FROM work_sessions WHERE date(started_at) = ? ORDER BY started_at ASC",
                (date,),
            ).fetchall()
            conn.close()
            return [dict(r) for r in rows]
        except Exception as e:
            logger.debug(f"WorkSessionStore.get_sessions_for_date error: {e}")
            return []

    # ------------------------------------------------------------------
    # Write helpers
    # ------------------------------------------------------------------

    def append_commit(self, session_id: int, commit_hash: str) -> None:
        """Append *commit_hash* to the JSON commits array of a session."""
        try:
            conn = _connect()
            row = conn.execute(
                "SELECT commits FROM work_sessions WHERE id = ?", (session_id,)
            ).fetchone()
            if not row:
                conn.close()
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
                "UPDATE work_sessions SET commits = ? WHERE id = ?",
                (json.dumps(commits), session_id),
            )
            conn.commit()
            conn.close()
        except Exception as e:
            logger.debug(f"WorkSessionStore.append_commit error: {e}")

    def end_session(self, session_id: int) -> None:
        """Mark a session as ended, computing duration from started_at."""
        try:
            conn = _connect()
            row = conn.execute(
                "SELECT started_at FROM work_sessions WHERE id = ?", (session_id,)
            ).fetchone()
            if not row:
                conn.close()
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
                "UPDATE work_sessions SET ended_at = ?, duration_minutes = ? WHERE id = ?",
                (ended_at, duration_mins, session_id),
            )
            conn.commit()
            conn.close()
        except Exception as e:
            logger.debug(f"WorkSessionStore.end_session error: {e}")

    def adjust_time(self, session_id: int, adjusted_minutes: int) -> None:
        """Set user-overridden time. Auto-measured duration_minutes is preserved."""
        try:
            conn = _connect()
            conn.execute(
                "UPDATE work_sessions SET adjusted_minutes = ? WHERE id = ?",
                (adjusted_minutes, session_id),
            )
            conn.commit()
            conn.close()
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
