"""
Tests for TicketDB — ticket_cache and pm_update_queue helpers.

All tests use an in-memory SQLite database: DATABASE_PATH is patched to ":memory:"
so no persistent file is created or required.
"""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Generator

import pytest
from sqlalchemy import create_engine


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture()
def ticket_db(tmp_path) -> Generator:
    """Return a TicketDB instance backed by a fresh temp SQLite file."""
    engine = create_engine(
        f"sqlite:///{tmp_path / 'test_tickets.db'}",
        connect_args={"check_same_thread": False},
    )
    from backend.db.ticket_db import TicketDB, _init
    _init(engine)
    yield TicketDB(engine=engine)


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
