"""
DevTrack Server TUI — trigger throughput stats client.

Queries the SQLite database for trigger activity and returns a TriggerStats
dataclass.  All queries degrade gracefully: if the DB file does not exist or
the ``triggers`` table has no rows the function returns zero-valued stats
instead of raising.
"""
from __future__ import annotations

import sqlite3
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Optional


@dataclass
class TriggerStats:
    """Snapshot of trigger throughput for the last 24 hours / today."""

    triggers_today: int = 0
    commits_today: int = 0
    last_trigger: str = "—"   # HH:MM string, or "—" if none
    errors_24h: int = 0


def _db_path() -> Optional[Path]:
    """Return the SQLite DB path using backend.config, or None if unavailable."""
    try:
        from backend.config import database_path  # type: ignore[import]
        return database_path()
    except Exception:
        return None


def get_trigger_stats() -> TriggerStats:
    """Query the ``triggers`` table and return a populated :class:`TriggerStats`.

    Returns a zero-valued instance (no crash) when:

    - The ``backend.config`` module cannot be imported.
    - The DB file does not exist.
    - The ``triggers`` table is absent.
    - Any SQL query fails.
    """
    path = _db_path()
    if path is None or not path.exists():
        return TriggerStats()

    try:
        return _query_stats(path)
    except Exception:
        return TriggerStats()


def _query_stats(path: Path) -> TriggerStats:
    """Execute the SQL queries against *path* and build a :class:`TriggerStats`."""
    now_utc = datetime.now(timezone.utc)
    today_str = now_utc.strftime("%Y-%m-%d")
    cutoff_24h = (now_utc - timedelta(hours=24)).strftime("%Y-%m-%d %H:%M:%S")
    # Triggers that are still unprocessed after 5 minutes are considered errors.
    error_cutoff = (now_utc - timedelta(minutes=5)).strftime("%Y-%m-%d %H:%M:%S")

    with sqlite3.connect(str(path)) as conn:
        conn.row_factory = sqlite3.Row
        cur = conn.cursor()

        # --- triggers today (all types) ---
        triggers_today: int = 0
        try:
            row = cur.execute(
                "SELECT COUNT(*) FROM triggers WHERE date(timestamp) = ?",
                (today_str,),
            ).fetchone()
            triggers_today = int(row[0]) if row else 0
        except sqlite3.OperationalError:
            pass

        # --- commits today (trigger_type = 'commit') ---
        commits_today: int = 0
        try:
            row = cur.execute(
                "SELECT COUNT(*) FROM triggers WHERE trigger_type = 'commit' AND date(timestamp) = ?",
                (today_str,),
            ).fetchone()
            commits_today = int(row[0]) if row else 0
        except sqlite3.OperationalError:
            pass

        # --- last trigger timestamp ---
        last_trigger: str = "—"
        try:
            row = cur.execute(
                "SELECT timestamp FROM triggers ORDER BY timestamp DESC LIMIT 1"
            ).fetchone()
            if row and row[0]:
                ts_raw: str = row[0]
                # Timestamps stored by Go as "2006-01-02T15:04:05Z" or "2006-01-02 15:04:05"
                for fmt in ("%Y-%m-%dT%H:%M:%SZ", "%Y-%m-%dT%H:%M:%S", "%Y-%m-%d %H:%M:%S"):
                    try:
                        dt = datetime.strptime(ts_raw[:19], fmt[:len(fmt)])
                        last_trigger = dt.strftime("%H:%M")
                        break
                    except ValueError:
                        continue
        except sqlite3.OperationalError:
            pass

        # --- errors in last 24h: unprocessed triggers older than 5 minutes ---
        errors_24h: int = 0
        try:
            row = cur.execute(
                """SELECT COUNT(*) FROM triggers
                   WHERE processed = 0
                     AND timestamp >= ?
                     AND timestamp <= ?""",
                (cutoff_24h, error_cutoff),
            ).fetchone()
            errors_24h = int(row[0]) if row else 0
        except sqlite3.OperationalError:
            pass

    return TriggerStats(
        triggers_today=triggers_today,
        commits_today=commits_today,
        last_trigger=last_trigger,
        errors_24h=errors_24h,
    )
