"""
queue_status_store.py — Read-only status-count access to Go-owned queue tables.

Used by backend/telegram/handlers.py's ``/queue`` (message_queue) and
``/commits`` (deferred_commits) commands, which previously each had their
own bespoke ``sqlite3.connect()`` copy of the same
"SELECT status, COUNT(*) ... GROUP BY status" query.

Boundary rule
-------------
``message_queue`` and ``deferred_commits`` are Go-owned tables (created and
written by the Go daemon — see ``devtrack_client/internal/db/database.go``).
This module never defines a SQLAlchemy ``Table`` for either and never runs
DDL against them.

  SQLite mode     (POSTGRES_URL unset) — reads go through
    ``backend.db.engine.get_engine()`` via SQLAlchemy ``text()``.
  PostgreSQL mode (POSTGRES_URL set)   — Go never speaks Postgres (decided
    2026-07-13), so neither table exists there and there is no Go
    internal-HTTP endpoint exposing either. ``get_status_counts`` fails
    closed immediately in this mode (empty list, never raise — see the
    PostgreSQL Backend epic, ``Data/agent_logs/project_board.md``).
"""
from __future__ import annotations

import logging
from typing import List, Tuple

from sqlalchemy import text

from backend.db.engine import get_engine, is_postgres

logger = logging.getLogger("devtrack.queue_status_store")

_ALLOWED_TABLES = {"message_queue", "deferred_commits"}


def get_status_counts(table: str) -> List[Tuple[str, int]]:
    """Return [(status, count), ...] for the given Go-owned table.

    ``table`` must be one of _ALLOWED_TABLES (never derived from user input
    in any caller). Returns ``[]`` on any DB error, or immediately in
    PostgreSQL mode.
    """
    if table not in _ALLOWED_TABLES:
        raise ValueError(f"get_status_counts: unsupported table {table!r}")
    if is_postgres():
        logger.debug(
            "queue_status_store: get_status_counts(%s) is a no-op in "
            "PostgreSQL mode — %s is a Go-owned SQLite-only table with no "
            "Postgres equivalent yet",
            table, table,
        )
        return []
    try:
        with get_engine().connect() as conn:
            rows = conn.execute(
                text(f"SELECT status, COUNT(*) FROM {table} GROUP BY status")
            ).fetchall()
            return [(r[0], r[1]) for r in rows]
    except Exception as e:
        logger.debug("Could not read %s: %s", table, e)
        return []
