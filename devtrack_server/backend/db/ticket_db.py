"""
TicketDB — SQLAlchemy helper for ticket_cache and pm_update_queue tables.

Runs against SQLite (local) or PostgreSQL (multi-user), selected via POSTGRES_URL.
All callers use TicketDB.from_config() or TicketDB() with the shared engine.
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import List, Optional

from sqlalchemy import Column, Table, Text, select
from sqlalchemy.engine import Engine

from backend.db.engine import get_engine, metadata, upsert

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Table definitions
# ---------------------------------------------------------------------------

ticket_cache_table = Table(
    "ticket_cache", metadata,
    Column("id",          Text, primary_key=True),
    Column("source",      Text, nullable=False),
    Column("external_id", Text, nullable=False),
    Column("repo",        Text),
    Column("title",       Text, nullable=False),
    Column("description", Text),
    Column("status",      Text),
    Column("assignee",    Text),
    Column("labels",      Text),
    Column("url",         Text),
    Column("synced_at",   Text, nullable=False),
    Column("created_at",  Text, nullable=False),
)

pm_update_queue_table = Table(
    "pm_update_queue", metadata,
    Column("id",          Text, primary_key=True),
    Column("ticket_id",   Text, nullable=False),
    Column("source",      Text, nullable=False),
    Column("update_json", Text, nullable=False),
    Column("queued_at",   Text, nullable=False),
    Column("processed",   Text),
)

_schema_done: bool = False


def _init(engine: Optional[Engine] = None) -> Engine:
    global _schema_done
    eng = engine or get_engine()
    if not _schema_done:
        metadata.create_all(eng, tables=[ticket_cache_table, pm_update_queue_table])
        _schema_done = True
    return eng


def _now_str() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")


# ---------------------------------------------------------------------------
# TicketDB class (context manager; same public interface as before)
# ---------------------------------------------------------------------------

class TicketDB:
    """SQLAlchemy-backed wrapper for ticket_cache / pm_update_queue."""

    def __init__(self, engine: Optional[Engine] = None) -> None:
        self._engine = engine

    @classmethod
    def from_config(cls) -> "TicketDB":
        """Create a TicketDB using the shared engine (SQLite or PostgreSQL)."""
        return cls()

    def __enter__(self) -> "TicketDB":
        return self

    def __exit__(self, *_) -> None:
        pass  # engine connections are managed per-operation

    # ------------------------------------------------------------------
    # ticket_cache
    # ------------------------------------------------------------------

    def upsert_ticket(self, record: dict) -> None:
        """Insert or update a ticket_cache row.

        Required keys: id, source, external_id, title.
        Optional keys: repo, description, status, assignee, labels, url, synced_at.
        created_at is set on first insert and never updated.
        """
        synced_at = record.get("synced_at")
        if isinstance(synced_at, datetime):
            synced_at = synced_at.strftime("%Y-%m-%d %H:%M:%S")

        now = _now_str()
        row = {
            "id":          record["id"],
            "source":      record["source"],
            "external_id": str(record["external_id"]),
            "repo":        record.get("repo") or "",
            "title":       record["title"],
            "description": record.get("description") or "",
            "status":      record.get("status") or "",
            "assignee":    record.get("assignee") or "",
            "labels":      record.get("labels") or "[]",
            "url":         record.get("url") or "",
            "synced_at":   synced_at or now,
            "created_at":  now,
        }
        update_cols = {k: v for k, v in row.items() if k not in ("id", "created_at")}
        stmt = (
            upsert(ticket_cache_table)
            .values(**row)
            .on_conflict_do_update(index_elements=["id"], set_=update_cols)
        )
        eng = _init(self._engine)
        with eng.begin() as conn:
            conn.execute(stmt)

    def get_tickets_by_assignee(self, assignee: str) -> List[dict]:
        """Return all cached tickets for *assignee*, most-recently synced first."""
        eng = _init(self._engine)
        with eng.connect() as conn:
            rows = conn.execute(
                select(ticket_cache_table)
                .where(ticket_cache_table.c.assignee == assignee)
                .order_by(ticket_cache_table.c.synced_at.desc())
            ).mappings().all()
        return [dict(r) for r in rows]

    def get_ticket_by_id(self, ticket_id: str) -> Optional[dict]:
        """Return a single cached ticket dict or None."""
        eng = _init(self._engine)
        with eng.connect() as conn:
            row = conn.execute(
                select(ticket_cache_table).where(ticket_cache_table.c.id == ticket_id)
            ).mappings().first()
        return dict(row) if row else None

    def get_all_tickets(self) -> List[dict]:
        """Return all rows from ticket_cache."""
        eng = _init(self._engine)
        with eng.connect() as conn:
            rows = conn.execute(
                select(ticket_cache_table).order_by(ticket_cache_table.c.synced_at.desc())
            ).mappings().all()
        return [dict(r) for r in rows]

    def clear_by_source(self, source: str, repo: Optional[str] = None) -> int:
        """Delete all ticket_cache rows for a source (and optionally a repo).

        Used by force-refresh: Go drops everything for a source then re-inserts
        the full fresh list.  Returns the number of rows deleted.
        """
        eng = _init(self._engine)
        with eng.begin() as conn:
            stmt = ticket_cache_table.delete().where(
                ticket_cache_table.c.source == source
            )
            if repo:
                stmt = stmt.where(ticket_cache_table.c.repo == repo)
            result = conn.execute(stmt)
        return result.rowcount
