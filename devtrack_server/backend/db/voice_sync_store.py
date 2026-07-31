"""
voice_sync_store.py — Idempotency tracking for backend.voice_sync.VoiceSync.

``voice_synced_items`` is a Python-owned table (created and written only
here — no Go involvement, confirmed via repo-wide grep). Like
``db/voice_seed_store.py``, ``db/ticket_db.py``, and ``db/report_store.py``,
it gets a real SQLAlchemy ``Table`` on the shared
``backend.db.engine.metadata`` and works identically in SQLite and
PostgreSQL mode — no boundary-rule guard needed (see the PostgreSQL
Backend epic, ``Data/agent_logs/project_board.md``).
"""
from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Optional

from sqlalchemy import Column, Table, Text
from sqlalchemy.engine import Engine

from backend.db.engine import get_engine, metadata, upsert

logger = logging.getLogger("devtrack.voice_sync_store")

voice_synced_items_table = Table(
    "voice_synced_items", metadata,
    Column("platform",     Text, primary_key=True),
    Column("item_id",      Text, primary_key=True),
    Column("context_type", Text, primary_key=True),
    Column("synced_at",    Text, nullable=False),
)

_schema_done: bool = False
_own_tables = [voice_synced_items_table]


def _init(engine: Optional[Engine] = None) -> Engine:
    global _schema_done
    eng = engine or get_engine()
    if not _schema_done:
        metadata.create_all(eng, tables=_own_tables)
        _schema_done = True
    return eng


def is_already_synced(
    platform: str, item_id: str, context_type: str, engine: Optional[Engine] = None
) -> bool:
    """Return True if this (platform, item_id, context_type) is already tracked.

    Returns False on any DB error — never raises (VoiceSync degrades to
    re-embedding rather than crashing, same as before this port).
    """
    try:
        eng = _init(engine)
        with eng.connect() as conn:
            row = conn.execute(
                voice_synced_items_table.select().where(
                    (voice_synced_items_table.c.platform == platform)
                    & (voice_synced_items_table.c.item_id == item_id)
                    & (voice_synced_items_table.c.context_type == context_type)
                )
            ).first()
            return row is not None
    except Exception as e:
        logger.warning("voice_sync_store: could not check synced state: %s", e)
        return False


def mark_synced(
    platform: str, item_id: str, context_type: str, engine: Optional[Engine] = None
) -> None:
    """Record an item as synced. Never raises."""
    try:
        eng = _init(engine)
        synced_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
        with eng.begin() as conn:
            conn.execute(
                upsert(voice_synced_items_table)
                .values(platform=platform, item_id=item_id, context_type=context_type, synced_at=synced_at)
                .on_conflict_do_nothing()
            )
    except Exception as e:
        logger.warning("voice_sync_store: could not mark synced: %s", e)
