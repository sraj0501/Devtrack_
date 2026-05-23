"""
Platform sync cache — SQLite-backed store for external platform state.

Replaces the JSON files previously used by:
  - backend/azure/sync.py          → Data/azure/sync_state.json
  - backend/azure/assignment_poller.py → Data/azure/seen_assignments.json
  - backend/gitlab/sync.py         → Data/gitlab/sync_state.json
  - backend/gitlab/assignment_poller.py → Data/gitlab/seen_assignments.json
  - backend/github/ (similar pattern)
  - backend/jira/ (similar pattern)

All data lives in the main devtrack.db to keep a single DB file.
Tables are created on first use (Python-side, follows session_store.py pattern).
"""
from __future__ import annotations

import json
import sqlite3
from contextlib import contextmanager
from typing import Any, Dict, Optional, Set


# ---------------------------------------------------------------------------
# Connection helpers (mirrors session_store.py pattern)
# ---------------------------------------------------------------------------

def _db_path() -> str:
    try:
        from backend.config import database_path
        return str(database_path())
    except Exception:
        from backend.config import get_project_root
        root = get_project_root() or "."
        import os
        return os.path.join(root, "Data", "db", "devtrack.db")


@contextmanager
def _conn():
    con = sqlite3.connect(_db_path())
    con.row_factory = sqlite3.Row
    con.execute("PRAGMA journal_mode=WAL")
    try:
        yield con
        con.commit()
    finally:
        con.close()


# ---------------------------------------------------------------------------
# Schema init
# ---------------------------------------------------------------------------

def _ensure_schema() -> None:
    with _conn() as con:
        con.executescript("""
            CREATE TABLE IF NOT EXISTS platform_sync_cache (
                platform    TEXT NOT NULL,
                item_id     TEXT NOT NULL,
                data_json   TEXT NOT NULL DEFAULT '{}',
                synced_at   TEXT NOT NULL DEFAULT (datetime('now')),
                PRIMARY KEY (platform, item_id)
            );

            CREATE TABLE IF NOT EXISTS platform_sync_meta (
                platform    TEXT PRIMARY KEY,
                last_sync   TEXT
            );

            CREATE TABLE IF NOT EXISTS platform_seen_events (
                platform    TEXT NOT NULL,
                event_key   TEXT NOT NULL,
                seen_at     TEXT NOT NULL DEFAULT (datetime('now')),
                PRIMARY KEY (platform, event_key)
            );
        """)


_schema_done: bool = False


def _init() -> None:
    global _schema_done
    if not _schema_done:
        _ensure_schema()
        _schema_done = True


# ---------------------------------------------------------------------------
# Sync cache (work items / issues)
# ---------------------------------------------------------------------------

def load_sync_items(platform: str) -> Dict[str, Any]:
    """Return {item_id: item_dict} for all cached items of a platform."""
    _init()
    with _conn() as con:
        rows = con.execute(
            "SELECT item_id, data_json FROM platform_sync_cache WHERE platform=?",
            (platform,),
        ).fetchall()
    return {r["item_id"]: json.loads(r["data_json"]) for r in rows}


def load_sync_meta(platform: str) -> Optional[str]:
    """Return the last_sync ISO timestamp for a platform, or None."""
    _init()
    with _conn() as con:
        row = con.execute(
            "SELECT last_sync FROM platform_sync_meta WHERE platform=?", (platform,)
        ).fetchone()
    return row["last_sync"] if row else None


def save_sync_items(platform: str, items: Dict[str, Any], last_sync: Optional[str] = None) -> None:
    """Persist work items/issues for a platform.

    Each value in `items` must be JSON-serialisable.
    If `last_sync` is provided, the sync_meta row is updated too.
    """
    _init()
    with _conn() as con:
        # Upsert each item
        for item_id, data in items.items():
            con.execute(
                """INSERT INTO platform_sync_cache (platform, item_id, data_json, synced_at)
                   VALUES (?, ?, ?, datetime('now'))
                   ON CONFLICT(platform, item_id) DO UPDATE
                   SET data_json=excluded.data_json, synced_at=excluded.synced_at""",
                (platform, str(item_id), json.dumps(data, default=str)),
            )
        if last_sync is not None:
            con.execute(
                """INSERT INTO platform_sync_meta (platform, last_sync) VALUES (?, ?)
                   ON CONFLICT(platform) DO UPDATE SET last_sync=excluded.last_sync""",
                (platform, last_sync),
            )


def clear_sync_items(platform: str) -> None:
    """Delete all cached items for a platform (full resync)."""
    _init()
    with _conn() as con:
        con.execute("DELETE FROM platform_sync_cache WHERE platform=?", (platform,))
        con.execute(
            "INSERT INTO platform_sync_meta (platform, last_sync) VALUES (?, NULL) "
            "ON CONFLICT(platform) DO UPDATE SET last_sync=NULL",
            (platform,),
        )


# ---------------------------------------------------------------------------
# Seen events (assignment / comment dedup)
# ---------------------------------------------------------------------------

def load_seen_events(platform: str) -> Set[str]:
    """Return the set of event_keys already seen for a platform."""
    _init()
    with _conn() as con:
        rows = con.execute(
            "SELECT event_key FROM platform_seen_events WHERE platform=?", (platform,)
        ).fetchall()
    return {r["event_key"] for r in rows}


def mark_event_seen(platform: str, event_key: str) -> None:
    """Record an event_key as seen (idempotent)."""
    _init()
    with _conn() as con:
        con.execute(
            """INSERT OR IGNORE INTO platform_seen_events (platform, event_key)
               VALUES (?, ?)""",
            (platform, event_key),
        )


def mark_events_seen(platform: str, event_keys: Set[str]) -> None:
    """Bulk-record multiple event_keys as seen."""
    _init()
    with _conn() as con:
        con.executemany(
            "INSERT OR IGNORE INTO platform_seen_events (platform, event_key) VALUES (?, ?)",
            [(platform, k) for k in event_keys],
        )


