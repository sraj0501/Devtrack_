"""
Tests for backend/telegram/handlers.py's _cmd_queue/_cmd_commits/_cmd_health/
_cmd_workstart/_cmd_vacation -- Postgres backend epic (TASK-112, module 11 of 15).

These were the module's six raw-sqlite3 call sites (one function, _get_db_path,
was dead code after the port and removed). All now delegate to shared,
already-ported helpers -- the same three tables/shapes slack/handlers.py's
port (TASK-136) already fixed, plus two new Go-owned status-count tables:

  - _cmd_queue/_cmd_commits -> new backend.queue_status_store.get_status_counts()
  - _cmd_health             -> backend.health_snapshots_store.get_latest_snapshot_per_service()
  - _cmd_workstart          -> backend.work_tracker.session_store.WorkSessionStore.start_session()
  - _cmd_vacation on/off    -> backend.vacation.auto_responder.get_vacation_state()/set_vacation_state()

No raw sqlite3 remains in telegram/handlers.py.
"""
from __future__ import annotations

import sqlite3
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest

from backend.db.engine import reset_engine


@pytest.fixture()
def isolated_engine(tmp_path: Path, monkeypatch):
    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    yield tmp_path
    reset_engine()


def _make_update():
    update = MagicMock()
    update.message.reply_text = AsyncMock()
    return update


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


class TestCmdQueueCommits:
    @pytest.mark.asyncio
    async def test_queue_empty(self, isolated_engine: Path):
        from backend.telegram.handlers import _cmd_queue

        update = _make_update()
        await _cmd_queue(update, MagicMock(), MagicMock())
        update.message.reply_text.assert_awaited_once_with("Message queue is empty")

    @pytest.mark.asyncio
    async def test_queue_reports_counts(self, isolated_engine: Path):
        from backend.telegram.handlers import _cmd_queue

        db_path = isolated_engine / "devtrack.db"
        conn = sqlite3.connect(str(db_path))
        conn.execute("CREATE TABLE message_queue (id INTEGER PRIMARY KEY, status TEXT NOT NULL)")
        conn.execute("INSERT INTO message_queue (status) VALUES ('pending')")
        conn.commit()
        conn.close()

        update = _make_update()
        await _cmd_queue(update, MagicMock(), MagicMock())
        msg = update.message.reply_text.call_args[0][0]
        assert "pending" in msg

    @pytest.mark.asyncio
    async def test_commits_empty(self, isolated_engine: Path):
        from backend.telegram.handlers import _cmd_commits

        update = _make_update()
        await _cmd_commits(update, MagicMock(), MagicMock())
        update.message.reply_text.assert_awaited_once_with("No deferred commits")


class TestCmdHealth:
    @pytest.mark.asyncio
    async def test_no_data(self, isolated_engine: Path):
        from backend.telegram.handlers import _cmd_health

        update = _make_update()
        await _cmd_health(update, MagicMock(), MagicMock())
        update.message.reply_text.assert_awaited_once_with("No health data available yet")

    @pytest.mark.asyncio
    async def test_reports_latest_per_service(self, isolated_engine: Path):
        from backend.telegram.handlers import _cmd_health

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

        update = _make_update()
        await _cmd_health(update, MagicMock(), MagicMock())
        msg = update.message.reply_text.call_args[0][0]
        assert "Ollama" in msg or "ollama" in msg.lower()
        assert "UP" in msg


class TestCmdWorkstart:
    @pytest.mark.asyncio
    async def test_starts_session(self, isolated_engine: Path):
        from backend.telegram.handlers import _cmd_workstart

        db_path = isolated_engine / "devtrack.db"
        _make_work_sessions_table(db_path)

        update = _make_update()
        context = MagicMock()
        context.args = ["PROJ-1"]
        await _cmd_workstart(update, context, MagicMock())
        msg = update.message.reply_text.call_args[0][0]
        assert "started" in msg.lower()
        assert "PROJ-1" in msg

    @pytest.mark.asyncio
    async def test_warns_if_already_active(self, isolated_engine: Path):
        from backend.telegram.handlers import _cmd_workstart

        db_path = isolated_engine / "devtrack.db"
        _make_work_sessions_table(db_path)

        context = MagicMock()
        context.args = []
        await _cmd_workstart(_make_update(), context, MagicMock())

        update2 = _make_update()
        await _cmd_workstart(update2, context, MagicMock())
        msg = update2.message.reply_text.call_args[0][0]
        assert "already active" in msg.lower()


class TestCmdVacation:
    @pytest.mark.asyncio
    async def test_status_off(self, isolated_engine: Path):
        from backend.telegram.handlers import _cmd_vacation

        db_path = isolated_engine / "devtrack.db"
        _make_vacation_table(db_path, enabled=0)

        update = _make_update()
        context = MagicMock()
        context.args = ["status"]
        await _cmd_vacation(update, context, MagicMock())
        msg = update.message.reply_text.call_args[0][0]
        assert "OFF" in msg

    @pytest.mark.asyncio
    async def test_on_then_status(self, isolated_engine: Path):
        from backend.telegram.handlers import _cmd_vacation

        db_path = isolated_engine / "devtrack.db"
        _make_vacation_table(db_path, enabled=0)

        update = _make_update()
        context = MagicMock()
        context.args = ["on", "--threshold", "0.9"]
        await _cmd_vacation(update, context, MagicMock())
        msg = update.message.reply_text.call_args[0][0]
        assert "ON" in msg

        update2 = _make_update()
        context2 = MagicMock()
        context2.args = ["status"]
        await _cmd_vacation(update2, context2, MagicMock())
        msg2 = update2.message.reply_text.call_args[0][0]
        assert "ON" in msg2
        assert "90%" in msg2

    @pytest.mark.asyncio
    async def test_off(self, isolated_engine: Path):
        from backend.telegram.handlers import _cmd_vacation

        db_path = isolated_engine / "devtrack.db"
        _make_vacation_table(db_path, enabled=1)

        update = _make_update()
        context = MagicMock()
        context.args = ["off"]
        await _cmd_vacation(update, context, MagicMock())
        msg = update.message.reply_text.call_args[0][0]
        assert "OFF" in msg


class TestTelegramHandlersPostgresMode:
    """None of the five ported commands should ever raise in Postgres mode --
    they should surface a clear message rather than crashing."""

    @pytest.mark.asyncio
    async def test_health_no_data_in_postgres_mode(self, isolated_engine: Path, monkeypatch):
        monkeypatch.setenv("POSTGRES_URL", "postgresql://fake/fake")
        reset_engine()
        try:
            from backend.telegram.handlers import _cmd_health
            update = _make_update()
            await _cmd_health(update, MagicMock(), MagicMock())
            update.message.reply_text.assert_awaited_once_with("No health data available yet")
        finally:
            monkeypatch.delenv("POSTGRES_URL", raising=False)
            reset_engine()

    @pytest.mark.asyncio
    async def test_workstart_reports_unavailable_in_postgres_mode(self, isolated_engine: Path, monkeypatch):
        monkeypatch.setenv("POSTGRES_URL", "postgresql://fake/fake")
        reset_engine()
        try:
            from backend.telegram.handlers import _cmd_workstart
            update = _make_update()
            context = MagicMock()
            context.args = []
            await _cmd_workstart(update, context, MagicMock())
            msg = update.message.reply_text.call_args[0][0]
            assert "not available" in msg.lower()
        finally:
            monkeypatch.delenv("POSTGRES_URL", raising=False)
            reset_engine()

    @pytest.mark.asyncio
    async def test_vacation_on_reports_unavailable_in_postgres_mode(self, isolated_engine: Path, monkeypatch):
        monkeypatch.setenv("POSTGRES_URL", "postgresql://fake/fake")
        reset_engine()
        try:
            from backend.telegram.handlers import _cmd_vacation
            update = _make_update()
            context = MagicMock()
            context.args = ["on"]
            await _cmd_vacation(update, context, MagicMock())
            msg = update.message.reply_text.call_args[0][0]
            assert "not available" in msg.lower()
        finally:
            monkeypatch.delenv("POSTGRES_URL", raising=False)
            reset_engine()
