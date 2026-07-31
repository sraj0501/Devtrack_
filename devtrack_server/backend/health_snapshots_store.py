"""
health_snapshots_store.py — Read-only access to the Go-owned health_snapshots table.

Shared by backend/slack/handlers.py and backend/telegram/handlers.py's
``/devtrack health`` command (both previously had their own bespoke
``sqlite3.connect()`` copies of this query).

Boundary rule
-------------
``health_snapshots`` is a Go-owned table (created and written by the Go
daemon — see ``devtrack_client/internal/db/database.go``). This module
never defines a SQLAlchemy ``Table`` for it and never runs DDL against it.

  SQLite mode     (POSTGRES_URL unset) — reads go through
    ``backend.db.engine.get_engine()`` via SQLAlchemy ``text()``.
  PostgreSQL mode (POSTGRES_URL set)   — Go never speaks Postgres (decided
    2026-07-13), so ``health_snapshots`` doesn't exist there and there is no
    Go internal-HTTP endpoint exposing it yet. ``get_recent_snapshots`` fails
    closed immediately in this mode (empty list, never raise — see the
    PostgreSQL Backend epic, ``Data/agent_logs/project_board.md``).
"""
from __future__ import annotations

import logging
from typing import Dict, List

from sqlalchemy import text

from backend.db.engine import get_engine, is_postgres

logger = logging.getLogger("devtrack.health_snapshots_store")


def get_recent_snapshots(limit: int = 20) -> List[Dict]:
    """Return the most recent ``limit`` health_snapshot rows, newest first.

    Each row: {"service", "status", "latency_ms", "details", "checked_at"}.
    Returns ``[]`` on any DB error, or immediately in PostgreSQL mode.
    """
    if is_postgres():
        logger.debug(
            "health_snapshots_store: get_recent_snapshots is a no-op in "
            "PostgreSQL mode — health_snapshots is a Go-owned SQLite-only "
            "table with no Postgres equivalent yet"
        )
        return []
    try:
        with get_engine().connect() as conn:
            rows = conn.execute(
                text(
                    "SELECT service, status, latency_ms, details, checked_at "
                    "FROM health_snapshots "
                    "ORDER BY checked_at DESC LIMIT :limit"
                ),
                {"limit": limit},
            ).mappings().all()
            return [dict(r) for r in rows]
    except Exception as e:
        logger.debug("Could not read health_snapshots: %s", e)
        return []
