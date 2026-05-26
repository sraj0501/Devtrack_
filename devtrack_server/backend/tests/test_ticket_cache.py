"""
Tests for TicketDB — ticket_cache and pm_update_queue helpers.

All tests use an in-memory SQLite database: DATABASE_PATH is patched to ":memory:"
so no persistent file is created or required.
"""

from __future__ import annotations

import sqlite3
from datetime import datetime, timezone
from typing import Generator
from unittest.mock import patch

import pytest

# ---------------------------------------------------------------------------
# Helpers to bootstrap the schema in the in-memory DB
# ---------------------------------------------------------------------------

_SCHEMA = """
CREATE TABLE IF NOT EXISTS ticket_cache (
    id          TEXT PRIMARY KEY,
    source      TEXT NOT NULL,
    external_id TEXT NOT NULL,
    repo        TEXT,
    title       TEXT NOT NULL,
    description TEXT,
    status      TEXT,
    assignee    TEXT,
    labels      TEXT,
    url         TEXT,
    synced_at   DATETIME NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pm_update_queue (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_id   TEXT NOT NULL,
    action      TEXT NOT NULL,
    payload     TEXT NOT NULL,
    commit_hash TEXT,
    status      TEXT DEFAULT 'pending',
    attempts    INTEGER DEFAULT 0,
    last_error  TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    sent_at     DATETIME
);
"""


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture()
def mem_db_path(tmp_path) -> Generator[str, None, None]:
    """Create a temporary on-disk SQLite database with the ticket schema.

    Using a real file (not ':memory:') avoids connection-sharing issues
    between TicketDB and the setup connection.  The file is cleaned up by
    pytest's tmp_path fixture.
    """
    db_file = str(tmp_path / "test_tickets.db")
    conn = sqlite3.connect(db_file)
    conn.executescript(_SCHEMA)
    conn.commit()
    conn.close()
    yield db_file


@pytest.fixture()
def ticket_db(mem_db_path) -> Generator:
    """Return a TicketDB instance pointed at the in-memory schema.

    DATABASE_PATH is patched so TicketDB.from_config() uses our temp file.
    """
    # Patch backend.config.get to return our temp path for DATABASE_PATH
    original_get = None

    import backend.config as cfg

    original_get = cfg.get

    def patched_get(key: str, default=None) -> str:  # type: ignore[override]
        if key == "DATABASE_PATH":
            return mem_db_path
        return original_get(key, default)

    with patch("backend.config.get", side_effect=patched_get):
        # Also patch database_path() to return the same path
        from pathlib import Path
        with patch("backend.config.database_path", return_value=Path(mem_db_path)):
            from backend.db.ticket_db import TicketDB
            db = TicketDB.from_config()
            yield db
            db.close()


# ---------------------------------------------------------------------------
# TicketDB.upsert_ticket tests
# ---------------------------------------------------------------------------

class TestUpsertTicket:
    def test_insert_new_ticket(self, ticket_db):
        """upsert_ticket inserts a row that can be retrieved."""
        now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
        ticket_db.upsert_ticket({
            "id": "github:owner/repo#1",
            "source": "github",
            "external_id": "1",
            "repo": "owner/repo",
            "title": "Fix the bug",
            "description": "A very bad bug.",
            "status": "open",
            "assignee": "alice",
            "labels": '["bug"]',
            "url": "https://github.com/owner/repo/issues/1",
            "synced_at": now,
        })
        result = ticket_db.get_ticket_by_id("github:owner/repo#1")
        assert result is not None
        assert result["title"] == "Fix the bug"
        assert result["assignee"] == "alice"

    def test_update_on_re_insert(self, ticket_db):
        """upsert_ticket updates an existing row when the same id is inserted again."""
        now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
        base = {
            "id": "github:owner/repo#2",
            "source": "github",
            "external_id": "2",
            "title": "Original title",
            "status": "open",
            "assignee": "bob",
            "synced_at": now,
        }
        ticket_db.upsert_ticket(base)

        updated = dict(base, title="Updated title", status="closed")
        ticket_db.upsert_ticket(updated)

        result = ticket_db.get_ticket_by_id("github:owner/repo#2")
        assert result is not None
        assert result["title"] == "Updated title"
        assert result["status"] == "closed"

    def test_upsert_preserves_created_at(self, ticket_db):
        """Re-inserting a ticket does not change the original created_at value."""
        now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
        ticket_db.upsert_ticket({
            "id": "github:owner/repo#3",
            "source": "github",
            "external_id": "3",
            "title": "Title",
            "status": "open",
            "assignee": "carol",
            "synced_at": now,
        })
        first = ticket_db.get_ticket_by_id("github:owner/repo#3")
        first_created = first["created_at"]

        # Upsert again with a different synced_at
        ticket_db.upsert_ticket({
            "id": "github:owner/repo#3",
            "source": "github",
            "external_id": "3",
            "title": "New title",
            "status": "open",
            "assignee": "carol",
            "synced_at": now,
        })
        second = ticket_db.get_ticket_by_id("github:owner/repo#3")
        # created_at must be the same as the first insert
        assert second["created_at"] == first_created


# ---------------------------------------------------------------------------
# TicketDB.get_tickets_by_assignee tests
# ---------------------------------------------------------------------------

class TestGetTicketsByAssignee:
    def _insert(self, db, ticket_id: str, assignee: str) -> None:
        now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
        db.upsert_ticket({
            "id": ticket_id,
            "source": "github",
            "external_id": ticket_id.split("#")[-1],
            "title": f"Ticket {ticket_id}",
            "status": "open",
            "assignee": assignee,
            "synced_at": now,
        })

    def test_returns_only_assigned_tickets(self, ticket_db):
        """get_tickets_by_assignee returns only rows matching the given assignee."""
        self._insert(ticket_db, "github:repo#10", "alice")
        self._insert(ticket_db, "github:repo#11", "alice")
        self._insert(ticket_db, "github:repo#12", "bob")

        alice_tickets = ticket_db.get_tickets_by_assignee("alice")
        assert len(alice_tickets) == 2
        ids = {t["id"] for t in alice_tickets}
        assert ids == {"github:repo#10", "github:repo#11"}

    def test_returns_empty_for_unknown_assignee(self, ticket_db):
        """get_tickets_by_assignee returns an empty list for an unknown assignee."""
        self._insert(ticket_db, "github:repo#20", "dave")
        result = ticket_db.get_tickets_by_assignee("nobody")
        assert result == []

    def test_returns_all_assignee_tickets(self, ticket_db):
        """get_tickets_by_assignee returns all tickets, not just the first one."""
        for i in range(5):
            self._insert(ticket_db, f"github:repo#3{i}", "eve")
        result = ticket_db.get_tickets_by_assignee("eve")
        assert len(result) == 5
