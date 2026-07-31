"""
Tests for backend/health_snapshots_store.py -- Postgres backend epic (TASK-112).

health_snapshots is a Go-owned table (devtrack_client/internal/db/database.go).
get_recent_snapshots() reads via backend.db.engine.get_engine() in SQLite
mode and fails closed (empty list, never raises) in Postgres mode -- same
pattern as dialectic_status.py/skill_detector.py.
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


def _make_health_db(db_path: Path, rows: list[tuple]) -> None:
    conn = sqlite3.connect(str(db_path))
    conn.execute(
        """
        CREATE TABLE health_snapshots (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            service TEXT NOT NULL,
            status TEXT NOT NULL,
            latency_ms INTEGER,
            details TEXT,
            checked_at DATETIME NOT NULL
        )
        """
    )
    for r in rows:
        conn.execute(
            "INSERT INTO health_snapshots (service, status, latency_ms, details, checked_at) VALUES (?,?,?,?,?)",
            r,
        )
    conn.commit()
    conn.close()


class TestGetRecentSnapshots:
    def test_returns_rows_newest_first(self, isolated_engine: Path):
        from backend.health_snapshots_store import get_recent_snapshots

        db_path = isolated_engine / "devtrack.db"
        _make_health_db(db_path, [
            ("ollama", "up", 12, "", "2026-07-31T09:00:00"),
            ("ollama", "down", None, "connection refused", "2026-07-31T10:00:00"),
            ("webhook_server", "up", 5, "", "2026-07-31T09:30:00"),
        ])

        rows = get_recent_snapshots(limit=20)

        assert len(rows) == 3
        assert rows[0]["service"] == "ollama"
        assert rows[0]["status"] == "down"

    def test_nonexistent_db_returns_empty_list(self, isolated_engine: Path):
        from backend.health_snapshots_store import get_recent_snapshots

        assert get_recent_snapshots() == []

    def test_missing_table_returns_empty_list(self, isolated_engine: Path):
        from backend.health_snapshots_store import get_recent_snapshots

        db_path = isolated_engine / "devtrack.db"
        conn = sqlite3.connect(str(db_path))
        conn.execute("CREATE TABLE other_table (id INTEGER PRIMARY KEY)")
        conn.commit()
        conn.close()

        assert get_recent_snapshots() == []


class TestGetRecentSnapshotsPostgresMode:
    def test_fails_closed_without_touching_engine(self):
        from backend.health_snapshots_store import get_recent_snapshots

        with patch("backend.health_snapshots_store.is_postgres", return_value=True), \
             patch("backend.health_snapshots_store.get_engine") as mock_get_engine:
            result = get_recent_snapshots()

        mock_get_engine.assert_not_called()
        assert result == []
