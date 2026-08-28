"""
Tests for backend/db/report_store.py and DailyReportGenerator's
`reports`-table methods — TASK-112 Postgres backend epic port.

``reports`` is Python-owned: ``DailyReportGenerator`` is its sole
reader/writer (confirmed via a repo-wide grep for ``FROM reports`` /
``INTO reports`` — nothing else touches this table). Unlike the Go-owned
tables ported elsewhere in this epic, ``report_store.reports_table`` is a
real SQLAlchemy ``Table`` registered on the shared ``backend.db.engine``
metadata, so it works in both SQLite and PostgreSQL mode. This file
exercises the SQLite path; the PostgreSQL path is smoke-tested in
``test_postgres_backend.py`` (skipped without a live POSTGRES_URL).

Covers:
  - report_store.insert_report / get_reports round-trip
  - report_store.mark_email_sent toggling the flag
  - report_store.get_content hit/miss
  - DailyReportGenerator.save_to_database + get_reports_from_database round-trip
  - DailyReportGenerator.update_report_email_status toggling the flag
  - DailyReportGenerator.get_report_content hit/miss
"""
from __future__ import annotations

from datetime import datetime

import pytest

from backend.db.engine import reset_engine


# ---------------------------------------------------------------------------
# Fixture: isolated SQLAlchemy engine pointed at a fresh temp DB directory
# ---------------------------------------------------------------------------

@pytest.fixture(autouse=True)
def isolated_report_engine(tmp_path, monkeypatch):
    """Point DATABASE_DIR at a fresh temp directory and reset the shared
    SQLAlchemy engine singleton so report_store's engine-based reads/writes
    hit an isolated SQLite file per test (same fixture shape as
    test_skill_detector.py / test_vacation_auto_responder.py /
    test_eod_narrative.py's isolated_engine).

    report_store also caches a module-level ``_schema_done`` flag (same
    lazy-init pattern as ticket_db.py/platform_store.py/learning_store.py) so
    a real running process only pays for ``metadata.create_all()`` once —
    that flag must be reset here too, or the second test in this file would
    silently skip table creation against its own fresh (schema-less) SQLite
    file and every query would fail with "no such table: reports".
    """
    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    import backend.db.report_store as report_store
    report_store._schema_done = False
    yield tmp_path
    reset_engine()
    report_store._schema_done = False


def _report_row(**overrides) -> dict:
    row = {
        "report_date": "2026-07-31",
        "report_type": "daily",
        "format": "text",
        "content": "Full report content here.",
        "summary": "Hours: 8.0 | Tasks: 3",
        "total_hours": 8.0,
        "task_count": 3,
        "completed_count": 2,
        "projects_count": 1,
        "ai_enhanced": True,
    }
    row.update(overrides)
    return row


# ---------------------------------------------------------------------------
# report_store — direct tests
# ---------------------------------------------------------------------------

class TestReportStore:
    def test_insert_and_get_reports_roundtrip(self):
        from backend.db import report_store

        report_id = report_store.insert_report(_report_row())
        assert report_id is not None

        rows = report_store.get_reports(cutoff_date="2026-01-01")
        assert len(rows) == 1
        row = rows[0]
        assert row["id"] == report_id
        assert row["report_type"] == "daily"
        assert row["format"] == "text"
        assert row["content"] == "Full report content here."
        assert row["total_hours"] == 8.0
        assert row["task_count"] == 3
        assert row["completed_count"] == 2
        assert row["projects_count"] == 1
        assert row["ai_enhanced"] == 1  # stored as 0/1, matches original schema
        assert row["email_sent"] == 0
        assert row["email_sent_at"] is None
        assert row["created_at"]  # stamped at insert time

    def test_get_reports_filters_by_report_type(self):
        from backend.db import report_store

        report_store.insert_report(_report_row(report_type="daily"))
        report_store.insert_report(_report_row(report_type="weekly"))

        daily_rows = report_store.get_reports(report_type="daily", cutoff_date="2026-01-01")
        weekly_rows = report_store.get_reports(report_type="weekly", cutoff_date="2026-01-01")
        all_rows = report_store.get_reports(cutoff_date="2026-01-01")

        assert len(daily_rows) == 1
        assert len(weekly_rows) == 1
        assert len(all_rows) == 2

    def test_get_reports_respects_cutoff_date(self):
        from backend.db import report_store

        report_store.insert_report(_report_row(report_date="2020-01-01"))
        report_store.insert_report(_report_row(report_date="2026-07-31"))

        rows = report_store.get_reports(cutoff_date="2026-01-01")
        assert len(rows) == 1
        assert rows[0]["report_date"] == "2026-07-31"

    def test_get_reports_respects_limit(self):
        from backend.db import report_store

        for _ in range(5):
            report_store.insert_report(_report_row())

        rows = report_store.get_reports(cutoff_date="2020-01-01", limit=2)
        assert len(rows) == 2

    def test_get_reports_empty_when_no_rows(self):
        from backend.db import report_store

        assert report_store.get_reports(cutoff_date="2020-01-01") == []

    def test_mark_email_sent_toggle(self):
        from backend.db import report_store

        report_id = report_store.insert_report(_report_row())

        assert report_store.mark_email_sent(report_id, True) is True
        row = next(
            r for r in report_store.get_reports(cutoff_date="2020-01-01") if r["id"] == report_id
        )
        assert row["email_sent"] == 1
        assert row["email_sent_at"] is not None

        assert report_store.mark_email_sent(report_id, False) is True
        row = next(
            r for r in report_store.get_reports(cutoff_date="2020-01-01") if r["id"] == report_id
        )
        assert row["email_sent"] == 0
        assert row["email_sent_at"] is None

    def test_mark_email_sent_unknown_id_still_returns_true(self):
        """Matches the original raw-SQL behaviour: an UPDATE ... WHERE id = ?
        that matches zero rows is not treated as a failure."""
        from backend.db import report_store

        assert report_store.mark_email_sent(999999, True) is True

    def test_get_content_hit(self):
        from backend.db import report_store

        report_id = report_store.insert_report(_report_row(content="specific content xyz"))
        assert report_store.get_content(report_id) == "specific content xyz"

    def test_get_content_miss(self):
        from backend.db import report_store

        assert report_store.get_content(999999) is None


# ---------------------------------------------------------------------------
# DailyReportGenerator — integration tests through report_store
# ---------------------------------------------------------------------------

def _make_enhanced_report(report_type_hint: str = "daily"):
    """Build a minimal EnhancedReport with no AI insights (is_ai_enhanced=False)
    so format_report(TERMINAL) needs no LLM/AI dependency."""
    from backend.daily_report_generator import EnhancedReport, ReportStyle
    from backend.email_reporter import DailyReport, ActivitySummary

    activity = ActivitySummary(
        timestamp=datetime(2026, 7, 31, 9, 0, 0),
        project="devtrack",
        ticket_id="PROJ-1",
        status="done",
        description="Fixed a bug",
        time_spent=2.0,
        source="commit",
    )
    base = DailyReport(
        date=datetime(2026, 7, 31),
        activities=[activity],
        total_hours=8.0,
        projects_worked=["devtrack"],
        tickets_updated=["PROJ-1"],
        completed_count=1,
        in_progress_count=0,
        blocked_count=0,
    )
    return EnhancedReport(
        base_report=base,
        ai_insights=None,
        generated_at=datetime(2026, 7, 31, 18, 0, 0),
        report_style=ReportStyle.PROFESSIONAL,
        is_ai_enhanced=False,
    )


class TestDailyReportGeneratorDbMethods:
    def test_save_to_database_and_get_reports_from_database_roundtrip(self):
        from backend.daily_report_generator import DailyReportGenerator, OutputFormat

        gen = DailyReportGenerator()
        report = _make_enhanced_report()

        report_id = gen.save_to_database(report, output_format=OutputFormat.TERMINAL, report_type="daily")
        assert report_id is not None

        rows = gen.get_reports_from_database(days=365)
        assert len(rows) == 1
        row = rows[0]
        assert row["id"] == report_id
        assert row["report_type"] == "daily"
        assert row["total_hours"] == 8.0
        assert row["completed_count"] == 1
        assert row["projects_count"] == 1
        assert row["ai_enhanced"] is False
        assert row["email_sent"] is False
        assert row["source"] == "database"
        assert row["date"] == datetime(2026, 7, 31)

    def test_get_reports_from_database_filters_by_type(self):
        from backend.daily_report_generator import DailyReportGenerator, OutputFormat

        gen = DailyReportGenerator()
        report = _make_enhanced_report()
        gen.save_to_database(report, output_format=OutputFormat.TERMINAL, report_type="daily")

        rows = gen.get_reports_from_database(report_type="weekly", days=365)
        assert rows == []

        rows = gen.get_reports_from_database(report_type="daily", days=365)
        assert len(rows) == 1

    def test_update_report_email_status_toggle(self):
        from backend.daily_report_generator import DailyReportGenerator, OutputFormat

        gen = DailyReportGenerator()
        report = _make_enhanced_report()
        report_id = gen.save_to_database(report, output_format=OutputFormat.TERMINAL, report_type="daily")

        assert gen.update_report_email_status(report_id, sent=True) is True
        row = next(r for r in gen.get_reports_from_database(days=365) if r["id"] == report_id)
        assert row["email_sent"] is True
        assert row["email_sent_at"] is not None

        assert gen.update_report_email_status(report_id, sent=False) is True
        row = next(r for r in gen.get_reports_from_database(days=365) if r["id"] == report_id)
        assert row["email_sent"] is False
        assert row["email_sent_at"] is None

    def test_get_report_content_hit_and_miss(self):
        from backend.daily_report_generator import DailyReportGenerator, OutputFormat

        gen = DailyReportGenerator()
        report = _make_enhanced_report()
        report_id = gen.save_to_database(report, output_format=OutputFormat.TERMINAL, report_type="daily")

        content = gen.get_report_content(report_id)
        assert content is not None
        assert isinstance(content, str)
        assert content.strip()

        assert gen.get_report_content(999999) is None
