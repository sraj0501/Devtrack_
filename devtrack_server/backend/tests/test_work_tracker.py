"""
Tests for backend/work_tracker/ — session store, EOD report generator, EOD emailer.

Uses an in-memory SQLite database so no real DevTrack DB is needed.
"""

import asyncio
import json
import sqlite3
import tempfile
from datetime import date, datetime, timedelta
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest


# ---------------------------------------------------------------------------
# Helpers: build an in-memory DB with the work_sessions schema
# ---------------------------------------------------------------------------

_SCHEMA = """
CREATE TABLE IF NOT EXISTS work_sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at       TEXT NOT NULL,
    ended_at         TEXT,
    ticket_ref       TEXT,
    repo_path        TEXT,
    workspace_name   TEXT,
    description      TEXT,
    commits          TEXT DEFAULT '[]',
    duration_minutes INTEGER,
    adjusted_minutes INTEGER,
    auto_stopped     INTEGER DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);
"""


def _make_db(path: str) -> sqlite3.Connection:
    conn = sqlite3.connect(path)
    conn.row_factory = sqlite3.Row
    conn.execute(_SCHEMA)
    conn.commit()
    return conn


def _insert_session(conn, started_at, ended_at=None, ticket_ref="",
                    duration_minutes=None, adjusted_minutes=None, commits="[]"):
    cur = conn.execute(
        """
        INSERT INTO work_sessions
          (started_at, ended_at, ticket_ref, commits, duration_minutes, adjusted_minutes)
        VALUES (?, ?, ?, ?, ?, ?)
        """,
        (started_at, ended_at, ticket_ref, commits, duration_minutes, adjusted_minutes),
    )
    conn.commit()
    return cur.lastrowid


# ---------------------------------------------------------------------------
# WorkSessionStore tests
# ---------------------------------------------------------------------------

class TestWorkSessionStore:
    @pytest.fixture(autouse=True)
    def setup(self, tmp_path):
        self.db_path = str(tmp_path / "test.db")
        conn = _make_db(self.db_path)
        conn.close()

        # Patch _db_path so the store hits our temp file
        with patch("backend.work_tracker.session_store._db_path", return_value=self.db_path):
            from backend.work_tracker.session_store import WorkSessionStore
            self.store = WorkSessionStore()
            yield

    def test_get_active_session_none_when_empty(self):
        assert self.store.get_active_session() is None

    def test_get_active_session_returns_open_session(self):
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        _insert_session(conn, started_at="2026-03-28 09:00:00", ticket_ref="AUTH-42")
        conn.close()

        session = self.store.get_active_session()
        assert session is not None
        assert session["ticket_ref"] == "AUTH-42"
        assert session["ended_at"] is None

    def test_get_active_session_ignores_closed_sessions(self):
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        _insert_session(conn, "2026-03-28 09:00:00", ended_at="2026-03-28 11:00:00", duration_minutes=120)
        conn.close()

        assert self.store.get_active_session() is None

    def test_append_commit_to_active_session(self):
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        sid = _insert_session(conn, "2026-03-28 09:00:00")
        conn.close()

        self.store.append_commit(sid, "abc123")
        self.store.append_commit(sid, "def456")

        conn = sqlite3.connect(self.db_path)
        row = conn.execute("SELECT commits FROM work_sessions WHERE id = ?", (sid,)).fetchone()
        conn.close()
        commits = json.loads(row[0])
        assert "abc123" in commits
        assert "def456" in commits

    def test_append_commit_no_duplicates(self):
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        sid = _insert_session(conn, "2026-03-28 09:00:00")
        conn.close()

        self.store.append_commit(sid, "abc123")
        self.store.append_commit(sid, "abc123")  # duplicate

        conn = sqlite3.connect(self.db_path)
        row = conn.execute("SELECT commits FROM work_sessions WHERE id = ?", (sid,)).fetchone()
        conn.close()
        assert json.loads(row[0]).count("abc123") == 1

    def test_end_session_sets_ended_at_and_duration(self):
        from datetime import timezone
        started = (datetime.now(timezone.utc).replace(tzinfo=None) - timedelta(minutes=30)).strftime("%Y-%m-%d %H:%M:%S")
        conn = _make_db(self.db_path)
        sid = _insert_session(conn, started)
        conn.close()

        self.store.end_session(sid)

        conn = _make_db(self.db_path)
        row = conn.execute("SELECT ended_at, duration_minutes FROM work_sessions WHERE id = ?", (sid,)).fetchone()
        conn.close()
        assert row["ended_at"] is not None
        assert row["duration_minutes"] is not None
        assert row["duration_minutes"] >= 29  # ~30 min, allow rounding

    def test_adjust_time_sets_adjusted_minutes(self):
        conn = _make_db(self.db_path)
        sid = _insert_session(conn, "2026-03-28 09:00:00", ended_at="2026-03-28 11:00:00", duration_minutes=120)
        conn.close()

        self.store.adjust_time(sid, 90)

        conn = _make_db(self.db_path)
        row = conn.execute("SELECT adjusted_minutes, duration_minutes FROM work_sessions WHERE id = ?", (sid,)).fetchone()
        conn.close()
        assert row["adjusted_minutes"] == 90
        assert row["duration_minutes"] == 120  # original preserved

    def test_effective_duration_uses_adjusted_when_set(self):
        session = {"duration_minutes": 120, "adjusted_minutes": 90}
        assert self.store.effective_duration(session) == 90

    def test_effective_duration_falls_back_to_auto(self):
        session = {"duration_minutes": 120, "adjusted_minutes": None}
        assert self.store.effective_duration(session) == 120

    def test_effective_duration_zero_when_both_null(self):
        session = {"duration_minutes": None, "adjusted_minutes": None}
        assert self.store.effective_duration(session) == 0

    def test_get_sessions_for_date_returns_correct_date(self):
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        _insert_session(conn, "2026-03-28 09:00:00", ended_at="2026-03-28 11:00:00", duration_minutes=120)
        _insert_session(conn, "2026-03-27 14:00:00", ended_at="2026-03-27 16:00:00", duration_minutes=120)
        conn.close()

        sessions = self.store.get_sessions_for_date("2026-03-28")
        assert len(sessions) == 1
        assert sessions[0]["started_at"] == "2026-03-28 09:00:00"

    def test_get_sessions_for_date_empty_when_no_match(self):
        sessions = self.store.get_sessions_for_date("2099-01-01")
        assert sessions == []


# ---------------------------------------------------------------------------
# EODReport tests
# ---------------------------------------------------------------------------

class TestEODReport:
    def test_total_str_minutes_only(self):
        from backend.work_tracker.eod_report_generator import EODReport
        r = EODReport(date="2026-03-28", total_minutes=45)
        assert r.total_str() == "45m"

    def test_total_str_hours_and_minutes(self):
        from backend.work_tracker.eod_report_generator import EODReport
        r = EODReport(date="2026-03-28", total_minutes=90)
        assert r.total_str() == "1h 30m"

    def test_as_text_contains_date(self):
        from backend.work_tracker.eod_report_generator import EODReport
        r = EODReport(date="2026-03-28", total_minutes=60)
        text = r.as_text()
        assert "2026-03-28" in text
        assert "1h 0m" in text

    def test_as_text_shows_sessions(self):
        from backend.work_tracker.eod_report_generator import EODReport, SessionSummary
        r = EODReport(
            date="2026-03-28",
            total_minutes=120,
            sessions=[
                SessionSummary(
                    session_id=1, ticket_ref="AUTH-42", duration_minutes=120,
                    adjusted=False, commits=["abc"], started_at="2026-03-28 09:00:00",
                    ended_at="2026-03-28 11:00:00", description="",
                )
            ],
        )
        text = r.as_text()
        assert "AUTH-42" in text
        assert "2h 0m" in text

    def test_as_text_shows_achievements(self):
        from backend.work_tracker.eod_report_generator import EODReport
        r = EODReport(date="2026-03-28", total_minutes=60, achievements=["Fixed the bug"])
        assert "Fixed the bug" in r.as_text()

    def test_adjusted_session_shown(self):
        from backend.work_tracker.eod_report_generator import EODReport, SessionSummary
        r = EODReport(
            date="2026-03-28",
            total_minutes=90,
            sessions=[
                SessionSummary(
                    session_id=2, ticket_ref="PROJ-7", duration_minutes=90,
                    adjusted=True, commits=[], started_at="2026-03-28 13:00:00",
                    ended_at="2026-03-28 14:30:00", description="",
                )
            ],
        )
        assert "[adjusted]" in r.as_text()


# ---------------------------------------------------------------------------
# EODReportGenerator tests (async)
# ---------------------------------------------------------------------------

class TestEODReportGenerator:
    @pytest.fixture(autouse=True)
    def setup(self, tmp_path):
        self.db_path = str(tmp_path / "test.db")
        conn = _make_db(self.db_path)
        _insert_session(
            conn, "2026-03-28 09:00:00",
            ended_at="2026-03-28 11:00:00",
            ticket_ref="AUTH-42",
            duration_minutes=120,
            commits='["abc123"]',
        )
        _insert_session(
            conn, "2026-03-28 13:00:00",
            ended_at="2026-03-28 14:30:00",
            ticket_ref="PROJ-88",
            duration_minutes=60,
            adjusted_minutes=90,
        )
        conn.close()

    def _make_generator(self, db_path):
        from backend.work_tracker.eod_report_generator import EODReportGenerator
        gen = EODReportGenerator(include_ai=False)

        # Patch the session store to use our temp DB
        orig_init = gen.__class__.__init__

        def patched_store():
            from backend.work_tracker.session_store import WorkSessionStore
            store = WorkSessionStore()
            return store

        return gen, patched_store

    def test_generate_sums_sessions(self):
        from backend.work_tracker.eod_report_generator import EODReportGenerator

        async def run():
            with patch("backend.work_tracker.session_store._db_path", return_value=self.db_path):
                gen = EODReportGenerator(include_ai=False)
                report = await gen.generate("2026-03-28")
            return report

        report = asyncio.run(run())
        assert len(report.sessions) == 2
        # adjusted_minutes=90 overrides duration_minutes=60 for PROJ-88
        assert report.total_minutes == 120 + 90

    def test_generate_no_sessions_returns_empty(self):
        from backend.work_tracker.eod_report_generator import EODReportGenerator

        async def run():
            with patch("backend.work_tracker.session_store._db_path", return_value=self.db_path):
                gen = EODReportGenerator(include_ai=False)
                report = await gen.generate("2099-01-01")
            return report

        report = asyncio.run(run())
        assert report.sessions == []
        assert report.total_minutes == 0
        assert report.ai_narrative is None

    def test_generate_commit_list_parsed(self):
        from backend.work_tracker.eod_report_generator import EODReportGenerator

        async def run():
            with patch("backend.work_tracker.session_store._db_path", return_value=self.db_path):
                gen = EODReportGenerator(include_ai=False)
                report = await gen.generate("2026-03-28")
            return report

        report = asyncio.run(run())
        auth_session = next(s for s in report.sessions if s.ticket_ref == "AUTH-42")
        assert "abc123" in auth_session.commits

    def test_generate_adjusted_flag_set(self):
        from backend.work_tracker.eod_report_generator import EODReportGenerator

        async def run():
            with patch("backend.work_tracker.session_store._db_path", return_value=self.db_path):
                gen = EODReportGenerator(include_ai=False)
                report = await gen.generate("2026-03-28")
            return report

        report = asyncio.run(run())
        proj_session = next(s for s in report.sessions if s.ticket_ref == "PROJ-88")
        assert proj_session.adjusted is True
        assert proj_session.duration_minutes == 90  # effective = adjusted


# ---------------------------------------------------------------------------
# EODEmailer tests
# ---------------------------------------------------------------------------

class TestEODEmailer:
    def test_send_no_recipient_returns_false(self):
        from backend.work_tracker.eod_report_generator import EODReport
        from backend.work_tracker.eod_emailer import EODEmailer

        async def run():
            emailer = EODEmailer()
            report = EODReport(date="2026-03-28", total_minutes=60)
            with patch.dict("os.environ", {}, clear=True):
                # No EOD_REPORT_EMAIL set, no recipient arg
                return await emailer.send(report, recipient=None)

        result = asyncio.run(run())
        assert result is False

    def test_send_uses_recipient_arg(self):
        from backend.work_tracker.eod_report_generator import EODReport
        from backend.work_tracker.eod_emailer import EODEmailer

        async def run():
            emailer = EODEmailer()
            # Patch _send_via_graph so we don't need real Graph credentials
            emailer._send_via_graph = AsyncMock(return_value=True)
            report = EODReport(date="2026-03-28", total_minutes=60)
            return await emailer.send(report, recipient="test@example.com")

        result = asyncio.run(run())
        assert result is True

    def test_send_uses_env_fallback(self):
        from backend.work_tracker.eod_report_generator import EODReport
        from backend.work_tracker.eod_emailer import EODEmailer

        async def run():
            emailer = EODEmailer()
            emailer._send_via_graph = AsyncMock(return_value=True)
            report = EODReport(date="2026-03-28", total_minutes=60)
            with patch.dict("os.environ", {"EOD_REPORT_EMAIL": "env@example.com"}):
                return await emailer.send(report)

        result = asyncio.run(run())
        assert result is True

    def test_graph_failure_returns_false(self):
        from backend.work_tracker.eod_report_generator import EODReport
        from backend.work_tracker.eod_emailer import EODEmailer

        async def run():
            emailer = EODEmailer()
            emailer._send_via_graph = AsyncMock(return_value=False)
            report = EODReport(date="2026-03-28", total_minutes=60)
            return await emailer.send(report, recipient="x@y.com")

        assert asyncio.run(run()) is False
