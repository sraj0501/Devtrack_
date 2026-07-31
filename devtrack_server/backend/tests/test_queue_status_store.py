"""
Tests for backend/queue_status_store.py -- Postgres backend epic (TASK-112).

message_queue and deferred_commits are Go-owned tables (see
devtrack_client/internal/db/database.go). get_status_counts() reads via
backend.db.engine.get_engine() in SQLite mode and fails closed (empty list,
never raises) in Postgres mode -- same pattern as dialectic_status.py.
"""
from __future__ import annotations

import sqlite3
from pathlib import Path
from unittest.mock import patch

import pytest

from backend.db.engine import reset_engine


@pytest.fixture()
def isolated_engine(tmp_path: Path, monkeypatch):
    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    yield tmp_path
    reset_engine()


def _make_table(db_path: Path, table: str, statuses: list[str]) -> None:
    conn = sqlite3.connect(str(db_path))
    conn.execute(f"CREATE TABLE {table} (id INTEGER PRIMARY KEY AUTOINCREMENT, status TEXT NOT NULL)")
    for s in statuses:
        conn.execute(f"INSERT INTO {table} (status) VALUES (?)", (s,))
    conn.commit()
    conn.close()


class TestGetStatusCounts:
    def test_message_queue_counts(self, isolated_engine: Path):
        from backend.queue_status_store import get_status_counts

        db_path = isolated_engine / "devtrack.db"
        _make_table(db_path, "message_queue", ["pending", "pending", "sent"])

        rows = dict(get_status_counts("message_queue"))
        assert rows == {"pending": 2, "sent": 1}

    def test_deferred_commits_counts(self, isolated_engine: Path):
        from backend.queue_status_store import get_status_counts

        db_path = isolated_engine / "devtrack.db"
        _make_table(db_path, "deferred_commits", ["queued"])

        rows = dict(get_status_counts("deferred_commits"))
        assert rows == {"queued": 1}

    def test_nonexistent_db_returns_empty_list(self, isolated_engine: Path):
        from backend.queue_status_store import get_status_counts

        assert get_status_counts("message_queue") == []

    def test_unsupported_table_raises_value_error(self, isolated_engine: Path):
        from backend.queue_status_store import get_status_counts

        with pytest.raises(ValueError):
            get_status_counts("some_other_table")


class TestGetStatusCountsPostgresMode:
    def test_fails_closed_without_touching_engine(self):
        from backend.queue_status_store import get_status_counts

        with patch("backend.queue_status_store.is_postgres", return_value=True), \
             patch("backend.queue_status_store.get_engine") as mock_get_engine:
            result = get_status_counts("message_queue")

        mock_get_engine.assert_not_called()
        assert result == []
