"""
Tests for EmailReporter.get_daily_activities() — Postgres backend epic (TASK-112).

``task_updates`` is a Go-owned table (see the boundary-rule docstring at the
top of backend/email_reporter.py and devtrack_client/internal/db/database.go).
This module never defines a SQLAlchemy Table for it — reads go through
backend.db.engine.get_engine() in SQLite mode, and fail closed (empty list,
never raise) in PostgreSQL mode.
"""
from __future__ import annotations

import sqlite3
import sys
from datetime import datetime
from pathlib import Path

import pytest

_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


@pytest.fixture()
def isolated_engine(tmp_path, monkeypatch):
    """Point DATABASE_DIR at a fresh temp directory and reset the shared
    SQLAlchemy engine singleton (same shape as test_eod_narrative.py's
    isolated_engine / test_skill_detector.py's / test_server_tui.py's)."""
    from backend.db.engine import reset_engine

    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    yield tmp_path
    reset_engine()


def _make_task_updates_db(db_dir: Path, rows: list[dict], with_optional_columns: bool = True) -> None:
    """Create/populate the ``task_updates`` table by hand — it's Go-owned,
    schema mirrored from devtrack_client/internal/db/database.go."""
    db_path = db_dir / "devtrack.db"
    conn = sqlite3.connect(str(db_path))
    extra_cols = ", time_estimate REAL, source TEXT" if with_optional_columns else ""
    conn.execute(
        f"""
        CREATE TABLE IF NOT EXISTS task_updates (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            response_id INTEGER,
            timestamp DATETIME NOT NULL,
            project TEXT NOT NULL,
            ticket_id TEXT NOT NULL,
            update_text TEXT,
            status TEXT
            {extra_cols}
        )
        """
    )
    for r in rows:
        if with_optional_columns:
            conn.execute(
                "INSERT INTO task_updates (timestamp, project, ticket_id, status, update_text, time_estimate, source) "
                "VALUES (?,?,?,?,?,?,?)",
                (
                    r["timestamp"], r["project"], r["ticket_id"], r["status"],
                    r["update_text"], r.get("time_estimate", 0), r.get("source", "manual"),
                ),
            )
        else:
            conn.execute(
                "INSERT INTO task_updates (timestamp, project, ticket_id, status, update_text) VALUES (?,?,?,?,?)",
                (r["timestamp"], r["project"], r["ticket_id"], r["status"], r["update_text"]),
            )
    conn.commit()
    conn.close()


TODAY = datetime.now().replace(hour=12, minute=0, second=0, microsecond=0)

_ROWS = [
    {"timestamp": TODAY.replace(hour=9).isoformat(), "project": "proj-a", "ticket_id": "PROJ-1",
     "status": "in_progress", "update_text": "started work", "time_estimate": 1.5, "source": "commit"},
    {"timestamp": TODAY.replace(hour=11).isoformat(), "project": "proj-a", "ticket_id": "PROJ-2",
     "status": "done", "update_text": "finished feature", "time_estimate": 2.0, "source": "manual"},
]


class TestGetDailyActivitiesSQLite:
    def test_reads_rows_with_optional_columns(self, isolated_engine):
        _make_task_updates_db(isolated_engine, _ROWS, with_optional_columns=True)
        from backend.email_reporter import EmailReporter
        reporter = EmailReporter()
        activities = reporter.get_daily_activities(TODAY)
        assert len(activities) == 2
        assert activities[0].ticket_id == "PROJ-1"
        assert activities[0].time_spent == 1.5
        assert activities[1].source == "manual"

    def test_reads_rows_without_optional_columns(self, isolated_engine):
        _make_task_updates_db(isolated_engine, _ROWS, with_optional_columns=False)
        from backend.email_reporter import EmailReporter
        reporter = EmailReporter()
        activities = reporter.get_daily_activities(TODAY)
        assert len(activities) == 2
        assert activities[0].time_spent == 0.0
        assert activities[0].source == "manual"

    def test_no_rows_returns_empty_list(self, isolated_engine):
        _make_task_updates_db(isolated_engine, [], with_optional_columns=True)
        from backend.email_reporter import EmailReporter
        reporter = EmailReporter()
        activities = reporter.get_daily_activities(TODAY)
        assert activities == []

    def test_missing_db_never_raises(self, isolated_engine):
        from backend.email_reporter import EmailReporter
        reporter = EmailReporter()
        activities = reporter.get_daily_activities(TODAY)
        assert activities == []


class TestGetDailyActivitiesPostgresMode:
    def test_fails_closed_in_postgres_mode(self, isolated_engine, monkeypatch):
        _make_task_updates_db(isolated_engine, _ROWS, with_optional_columns=True)
        monkeypatch.setenv("POSTGRES_URL", "postgresql://fake/fake")
        from backend.db.engine import reset_engine
        reset_engine()
        try:
            from backend.email_reporter import EmailReporter
            reporter = EmailReporter()
            activities = reporter.get_daily_activities(TODAY)
            assert activities == []
        finally:
            monkeypatch.delenv("POSTGRES_URL", raising=False)
            reset_engine()

    def test_never_touches_engine_in_postgres_mode(self, isolated_engine, monkeypatch):
        monkeypatch.setenv("POSTGRES_URL", "postgresql://fake/fake")
        from backend.db.engine import reset_engine
        reset_engine()
        try:
            import backend.email_reporter as er_mod
            called = {"get_engine": False}

            def _boom():
                called["get_engine"] = True
                raise AssertionError("get_engine() must not be called in Postgres mode")

            monkeypatch.setattr(er_mod, "get_engine", _boom)
            reporter = er_mod.EmailReporter()
            activities = reporter.get_daily_activities(TODAY)
            assert activities == []
            assert called["get_engine"] is False
        finally:
            monkeypatch.delenv("POSTGRES_URL", raising=False)
            reset_engine()
