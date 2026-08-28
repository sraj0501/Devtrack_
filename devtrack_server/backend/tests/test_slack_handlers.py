"""
Tests for backend/slack/handlers.py's cmd_health/cmd_workstart/cmd_vacation
-- Postgres backend epic (TASK-112, module 10 of 15).

These were the module's three raw-sqlite3 call sites. All now delegate to
shared, already-ported helpers:
  - cmd_health    -> backend.health_snapshots_store.get_recent_snapshots()
  - cmd_workstart -> backend.work_tracker.session_store.WorkSessionStore.start_session()
  - cmd_vacation  -> backend.vacation.auto_responder.get_vacation_state()/set_vacation_state()

No raw sqlite3 remains in slack/handlers.py.
"""
from __future__ import annotations

import sqlite3
from pathlib import Path
from unittest.mock import MagicMock

import pytest

from backend.db.engine import reset_engine


@pytest.fixture()
def isolated_engine(tmp_path: Path, monkeypatch):
    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    yield tmp_path
    reset_engine()


def _make_work_sessions_table(db_path: Path) -> None:
    conn = sqlite3.connect(str(db_path))
    conn.execute(
        """
        CREATE TABLE work_sessions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            started_at TEXT NOT NULL,
            ended_at TEXT,
            ticket_ref TEXT,
            commits TEXT DEFAULT '[]',
            duration_minutes INTEGER,
            adjusted_minutes INTEGER
        )
        """
    )
    conn.commit()
    conn.close()


def _make_vacation_table(db_path: Path, enabled: int = 0) -> None:
    conn = sqlite3.connect(str(db_path))
    conn.execute(
        """
        CREATE TABLE vacation_mode (
            id INTEGER PRIMARY KEY CHECK (id = 1),
            enabled INTEGER NOT NULL DEFAULT 0,
            enabled_at TEXT,
            until TEXT,
            confidence_threshold REAL NOT NULL DEFAULT 0.7,
            auto_submit INTEGER NOT NULL DEFAULT 1
        )
        """
    )
    conn.execute(
        "INSERT INTO vacation_mode (id, enabled, enabled_at, until, confidence_threshold, auto_submit) "
        "VALUES (1, ?, '', '', 0.7, 1)",
        (enabled,),
    )
    conn.commit()
    conn.close()


class TestCmdHealth:
    def test_no_data_message(self, isolated_engine: Path):
        from backend.slack.handlers import cmd_health

        respond = MagicMock()
        cmd_health("", respond, MagicMock())
        respond.assert_called_once_with("No health data available yet.")

    def test_reports_latest_per_service(self, isolated_engine: Path):
        from backend.slack.handlers import cmd_health

        db_path = isolated_engine / "devtrack.db"
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
        conn.execute(
            "INSERT INTO health_snapshots (service, status, latency_ms, details, checked_at) VALUES (?,?,?,?,?)",
            ("ollama", "up", 12, "", "2026-07-31T09:00:00"),
        )
        conn.commit()
        conn.close()

        respond = MagicMock()
        cmd_health("", respond, MagicMock())
        msg = respond.call_args[0][0]
        assert "ollama" in msg
        assert "up" in msg


class TestCmdWorkstart:
    def test_starts_session(self, isolated_engine: Path):
        from backend.slack.handlers import cmd_workstart

        db_path = isolated_engine / "devtrack.db"
        _make_work_sessions_table(db_path)

        respond = MagicMock()
        cmd_workstart("PROJ-1", respond, MagicMock())
        msg = respond.call_args[0][0]
        assert "started" in msg.lower()
        assert "PROJ-1" in msg

    def test_warns_if_already_active(self, isolated_engine: Path):
        from backend.slack.handlers import cmd_workstart

        db_path = isolated_engine / "devtrack.db"
        _make_work_sessions_table(db_path)

        respond = MagicMock()
        cmd_workstart("", respond, MagicMock())
        respond.reset_mock()
        cmd_workstart("", respond, MagicMock())
        msg = respond.call_args[0][0]
        assert "already active" in msg.lower()


class TestCmdVacation:
    def test_status_off(self, isolated_engine: Path):
        from backend.slack.handlers import cmd_vacation

        db_path = isolated_engine / "devtrack.db"
        _make_vacation_table(db_path, enabled=0)

        respond = MagicMock()
        cmd_vacation("status", respond, MagicMock())
        msg = respond.call_args[0][0]
        assert "OFF" in msg

    def test_on_then_status(self, isolated_engine: Path):
        from backend.slack.handlers import cmd_vacation

        db_path = isolated_engine / "devtrack.db"
        _make_vacation_table(db_path, enabled=0)

        respond = MagicMock()
        cmd_vacation("on --threshold 0.9", respond, MagicMock())
        msg = respond.call_args[0][0]
        assert "ON" in msg

        respond.reset_mock()
        cmd_vacation("status", respond, MagicMock())
        msg = respond.call_args[0][0]
        assert "ON" in msg
        assert "90%" in msg

    def test_off(self, isolated_engine: Path):
        from backend.slack.handlers import cmd_vacation

        db_path = isolated_engine / "devtrack.db"
        _make_vacation_table(db_path, enabled=1)

        respond = MagicMock()
        cmd_vacation("off", respond, MagicMock())
        msg = respond.call_args[0][0]
        assert "OFF" in msg


class TestSlackHandlersPostgresMode:
    """None of the three ported commands should ever raise in Postgres mode --
    they should surface a clear, user-facing "not available yet" message."""

    def test_health_no_data_in_postgres_mode(self, isolated_engine: Path, monkeypatch):
        monkeypatch.setenv("POSTGRES_URL", "postgresql://fake/fake")
        reset_engine()
        try:
            from backend.slack.handlers import cmd_health
            respond = MagicMock()
            cmd_health("", respond, MagicMock())
            respond.assert_called_once_with("No health data available yet.")
        finally:
            monkeypatch.delenv("POSTGRES_URL", raising=False)
            reset_engine()

    def test_workstart_reports_unavailable_in_postgres_mode(self, isolated_engine: Path, monkeypatch):
        monkeypatch.setenv("POSTGRES_URL", "postgresql://fake/fake")
        reset_engine()
        try:
            from backend.slack.handlers import cmd_workstart
            respond = MagicMock()
            cmd_workstart("", respond, MagicMock())
            msg = respond.call_args[0][0]
            assert "not available" in msg.lower()
        finally:
            monkeypatch.delenv("POSTGRES_URL", raising=False)
            reset_engine()

    def test_vacation_on_reports_unavailable_in_postgres_mode(self, isolated_engine: Path, monkeypatch):
        monkeypatch.setenv("POSTGRES_URL", "postgresql://fake/fake")
        reset_engine()
        try:
            from backend.slack.handlers import cmd_vacation
            respond = MagicMock()
            cmd_vacation("on", respond, MagicMock())
            msg = respond.call_args[0][0]
            assert "not available" in msg.lower()
        finally:
            monkeypatch.delenv("POSTGRES_URL", raising=False)
            reset_engine()
