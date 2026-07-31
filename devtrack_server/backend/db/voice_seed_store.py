"""
voice_seed_store.py — Idempotency tracking for backend.voice_seeder.VoiceSeeder.

``voice_seeded_commits`` is a Python-owned table (created and written only
here — no Go involvement, confirmed via repo-wide grep). Like
``db/ticket_db.py`` and ``db/report_store.py``, it gets a real SQLAlchemy
``Table`` on the shared ``backend.db.engine.metadata`` and works identically
in SQLite and PostgreSQL mode — no boundary-rule guard needed (see the
PostgreSQL Backend epic, ``Data/agent_logs/project_board.md``).
"""
from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Optional

from sqlalchemy import Column, Table, Text
from sqlalchemy.engine import Engine

from backend.db.engine import get_engine, metadata, upsert

logger = logging.getLogger("devtrack.voice_seed_store")

voice_seeded_commits_table = Table(
    "voice_seeded_commits", metadata,
    Column("hash",      Text, primary_key=True),
    Column("repo_path", Text, primary_key=True),
    Column("seeded_at", Text, nullable=False),
)

_schema_done: bool = False
_own_tables = [voice_seeded_commits_table]


def _init(engine: Optional[Engine] = None) -> Engine:
    global _schema_done
    eng = engine or get_engine()
    if not _schema_done:
        metadata.create_all(eng, tables=_own_tables)
        _schema_done = True
    return eng


def is_already_seeded(commit_hash: str, repo_path: str, engine: Optional[Engine] = None) -> bool:
    """Return True if this (hash, repo_path) pair is already tracked.

    Returns False on any DB error — never raises (VoiceSeeder degrades to
    re-embedding rather than crashing, same as before this port).
    """
    try:
        eng = _init(engine)
        with eng.connect() as conn:
            row = conn.execute(
                voice_seeded_commits_table.select().where(
                    (voice_seeded_commits_table.c.hash == commit_hash)
                    & (voice_seeded_commits_table.c.repo_path == repo_path)
                )
            ).first()
            return row is not None
    except Exception as e:
        logger.warning("voice_seed_store: could not check seeded state: %s", e)
        return False


def mark_seeded(commit_hash: str, repo_path: str, engine: Optional[Engine] = None) -> None:
    """Record a commit as seeded. Never raises."""
    try:
        eng = _init(engine)
        seeded_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
        with eng.begin() as conn:
            conn.execute(
                upsert(voice_seeded_commits_table)
                .values(hash=commit_hash, repo_path=repo_path, seeded_at=seeded_at)
                .on_conflict_do_nothing()
            )
    except Exception as e:
        logger.warning("voice_seed_store: could not mark seeded: %s", e)
