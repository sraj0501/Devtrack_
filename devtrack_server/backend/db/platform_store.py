"""
Platform sync cache — SQLAlchemy-backed store for external platform state.

Replaces the JSON files previously used by azure/, gitlab/, github/, jira/ sync
modules.  Runs against SQLite (local) or PostgreSQL (multi-user), selected via
POSTGRES_URL.

Tables
------
platform_sync_cache   — work item / issue snapshots keyed by (platform, item_id)
platform_sync_meta    — last_sync timestamp per platform
platform_seen_events  — dedup set of event_keys per platform (assignment / comment)
"""
from __future__ import annotations

import json
import logging
from datetime import datetime, timezone
from typing import Any, Dict, Optional, Set

from sqlalchemy import Column, Table, Text, select
from sqlalchemy.engine import Engine

from backend.db.engine import ensure_tables, get_engine, metadata, upsert

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Table definitions
# ---------------------------------------------------------------------------

platform_sync_cache_table = Table(
    "platform_sync_cache", metadata,
    Column("platform",  Text, primary_key=True),
    Column("item_id",   Text, primary_key=True),
    Column("data_json", Text, nullable=False),
    Column("synced_at", Text, nullable=False),
)

platform_sync_meta_table = Table(
    "platform_sync_meta", metadata,
    Column("platform",  Text, primary_key=True),
    Column("last_sync", Text),
)

platform_seen_events_table = Table(
    "platform_seen_events", metadata,
    Column("platform",  Text, primary_key=True),
    Column("event_key", Text, primary_key=True),
    Column("seen_at",   Text, nullable=False),
)

# ---------------------------------------------------------------------------
# Schema init
# ---------------------------------------------------------------------------

_schema_done: bool = False
_own_tables = [platform_sync_cache_table, platform_sync_meta_table, platform_seen_events_table]


def _init(engine: Optional[Engine] = None) -> Engine:
    global _schema_done
    eng = engine or get_engine()
    if not _schema_done:
        ensure_tables(eng, tables=_own_tables)
        _schema_done = True
    return eng


def _now() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")


# ---------------------------------------------------------------------------
# Sync cache (work items / issues)
# ---------------------------------------------------------------------------

def load_sync_items(platform: str, engine: Optional[Engine] = None) -> Dict[str, Any]:
    """Return {item_id: item_dict} for all cached items of a platform."""
    eng = _init(engine)
    with eng.connect() as conn:
        rows = conn.execute(
            select(
                platform_sync_cache_table.c.item_id,
                platform_sync_cache_table.c.data_json,
            ).where(platform_sync_cache_table.c.platform == platform)
        ).all()
    return {r.item_id: json.loads(r.data_json) for r in rows}


def load_sync_meta(platform: str, engine: Optional[Engine] = None) -> Optional[str]:
    """Return the last_sync ISO timestamp for a platform, or None."""
    eng = _init(engine)
    with eng.connect() as conn:
        row = conn.execute(
            select(platform_sync_meta_table.c.last_sync).where(
                platform_sync_meta_table.c.platform == platform
            )
        ).first()
    return row.last_sync if row else None


def save_sync_items(
    platform: str,
    items: Dict[str, Any],
    last_sync: Optional[str] = None,
    engine: Optional[Engine] = None,
) -> None:
    """Persist work items/issues for a platform.

    Each value in `items` must be JSON-serialisable.
    If `last_sync` is provided the sync_meta row is updated too.
    """
    eng = _init(engine)
    now = _now()
    with eng.begin() as conn:
        for item_id, data in items.items():
            row = {
                "platform":  platform,
                "item_id":   str(item_id),
                "data_json": json.dumps(data, default=str),
                "synced_at": now,
            }
            stmt = (
                upsert(platform_sync_cache_table)
                .values(**row)
                .on_conflict_do_update(
                    index_elements=["platform", "item_id"],
                    set_={"data_json": row["data_json"], "synced_at": now},
                )
            )
            conn.execute(stmt)

        if last_sync is not None:
            meta_row = {"platform": platform, "last_sync": last_sync}
            conn.execute(
                upsert(platform_sync_meta_table)
                .values(**meta_row)
                .on_conflict_do_update(
                    index_elements=["platform"],
                    set_={"last_sync": last_sync},
                )
            )


def clear_sync_items(platform: str, engine: Optional[Engine] = None) -> None:
    """Delete all cached items for a platform (full resync)."""
    eng = _init(engine)
    with eng.begin() as conn:
        conn.execute(
            platform_sync_cache_table.delete().where(
                platform_sync_cache_table.c.platform == platform
            )
        )
        conn.execute(
            upsert(platform_sync_meta_table)
            .values(platform=platform, last_sync=None)
            .on_conflict_do_update(index_elements=["platform"], set_={"last_sync": None})
        )


# ---------------------------------------------------------------------------
# Seen events (assignment / comment dedup)
# ---------------------------------------------------------------------------

def load_seen_events(platform: str, engine: Optional[Engine] = None) -> Set[str]:
    """Return the set of event_keys already seen for a platform."""
    eng = _init(engine)
    with eng.connect() as conn:
        rows = conn.execute(
            select(platform_seen_events_table.c.event_key).where(
                platform_seen_events_table.c.platform == platform
            )
        ).all()
    return {r.event_key for r in rows}


def mark_event_seen(platform: str, event_key: str, engine: Optional[Engine] = None) -> None:
    """Record an event_key as seen (idempotent)."""
    eng = _init(engine)
    with eng.begin() as conn:
        conn.execute(
            upsert(platform_seen_events_table)
            .values(platform=platform, event_key=event_key, seen_at=_now())
            .on_conflict_do_nothing()
        )


def mark_events_seen(
    platform: str, event_keys: Set[str], engine: Optional[Engine] = None
) -> None:
    """Bulk-record multiple event_keys as seen."""
    if not event_keys:
        return
    eng = _init(engine)
    now = _now()
    with eng.begin() as conn:
        for key in event_keys:
            conn.execute(
                upsert(platform_seen_events_table)
                .values(platform=platform, event_key=key, seen_at=now)
                .on_conflict_do_nothing()
            )
