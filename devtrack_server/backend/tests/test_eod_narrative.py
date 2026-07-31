"""
Tests for DailyReportGenerator.generate_eod_narrative() — TASK-076.

Five test classes as specified by the task acceptance criteria:

  1. LLM available (mocked provider) — returns multi-section string mentioning ticket IDs.
  2. LLM unavailable — falls back to plain-text bullet list; no exception raised.
  3. Empty commit history — returns "No commits recorded" string, never raises.
  4. inject_style called with context_type="report" (mock/spy assertion).
  5. Unlinked commits (ticket_id = "") — appear under "Other commits" section, not silently dropped.

TASK-112 (Postgres backend epic) note: ``_query_commit_rows`` (the method
backing this narrative) now reads the Go-owned ``triggers`` table through
``backend.db.engine.get_engine()`` instead of a bespoke ``sqlite3.connect()``
— see the boundary-rule docstring on that method in
``backend/daily_report_generator.py``. get_engine() is a process-wide
singleton resolved from DATABASE_DIR/database_path(), so an arbitrary
``db_path=...`` passed straight to ``DailyReportGenerator`` is silently
ignored for these reads. Tests below use the DATABASE_DIR + reset_engine()
isolated-engine fixture already established in test_skill_detector.py /
test_server_tui.py / test_work_tracker.py / test_queue_gateway.py instead.
"""
from __future__ import annotations

import sqlite3
import sys
from datetime import date
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

# Ensure the project root is on sys.path so backend.* imports work.
_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

TODAY = date.today().isoformat()

_LINKED_ROWS = [
    {"ticket_id": "PROJ-1", "commit_message": "fix: resolve login bug", "commit_hash": "aaa", "timestamp": f"{TODAY} 09:00:00"},
    {"ticket_id": "PROJ-1", "commit_message": "fix: add unit test for login", "commit_hash": "bbb", "timestamp": f"{TODAY} 10:00:00"},
    {"ticket_id": "PROJ-2", "commit_message": "feat: add dashboard widget", "commit_hash": "ccc", "timestamp": f"{TODAY} 11:00:00"},
]

_UNLINKED_ROWS = [
    {"ticket_id": "", "commit_message": "chore: update README", "commit_hash": "ddd", "timestamp": f"{TODAY} 12:00:00"},
    {"ticket_id": "unlinked", "commit_message": "chore: bump version", "commit_hash": "eee", "timestamp": f"{TODAY} 13:00:00"},
]

_ALL_ROWS = _LINKED_ROWS + _UNLINKED_ROWS


@pytest.fixture()
def isolated_engine(tmp_path, monkeypatch):
    """Point DATABASE_DIR at a fresh temp directory and reset the shared
    SQLAlchemy engine singleton so DailyReportGenerator's engine-based
    ``triggers`` reads hit an isolated SQLite file instead of whatever
    engine a prior test built (same fixture shape as
    test_skill_detector.py's isolated_engine / test_server_tui.py's
    isolated_stats_engine).

    Yields the temp directory; the DB file itself is ``devtrack.db`` inside
    it (backend.config.database_path()'s default filename).
    """
    from backend.db.engine import reset_engine

    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    yield tmp_path
    reset_engine()


def _make_db_with_rows(db_dir: Path, rows: list[dict]) -> None:
    """Create/populate the ``triggers`` table inside db_dir/devtrack.db.

    ``triggers`` is a Go-owned table (see the boundary-rule docstring on
    ``DailyReportGenerator._query_commit_rows``) — tests create its schema
    by hand, same as test_skill_detector.py does for ``corrections``.
    """
    db_path = db_dir / "devtrack.db"
    conn = sqlite3.connect(str(db_path))
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS triggers (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            trigger_type TEXT NOT NULL,
            ticket_id TEXT DEFAULT '',
            commit_message TEXT DEFAULT '',
            commit_hash TEXT DEFAULT '',
            timestamp DATETIME DEFAULT (datetime('now'))
        )
        """
    )
    for r in rows:
        conn.execute(
            "INSERT INTO triggers (trigger_type, ticket_id, commit_message, commit_hash, timestamp) VALUES (?,?,?,?,?)",
            ("commit", r.get("ticket_id", ""), r.get("commit_message", ""), r.get("commit_hash", ""), r.get("timestamp", TODAY)),
        )
    conn.commit()
    conn.close()


def _make_mock_provider(response: str) -> MagicMock:
    provider = MagicMock()
    provider.generate.return_value = response
    return provider


def _config_patches():
    """Context managers that stub out backend.config calls used inside generate()."""
    from contextlib import ExitStack
    stack = ExitStack()
    stack.enter_context(patch("backend.config.http_timeout", return_value=30))
    stack.enter_context(patch("backend.config.report_llm_temperature", return_value=0.3))
    stack.enter_context(patch("backend.config.report_llm_max_tokens", return_value=300))
    return stack


# ---------------------------------------------------------------------------
# 1. LLM available — multi-section string with ticket IDs
# ---------------------------------------------------------------------------

class TestEODNarrativeLLMAvailable:
    """When the LLM is reachable the report should contain per-ticket sections."""

    def test_returns_string(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _LINKED_ROWS)
        mock_provider = _make_mock_provider("I fixed the login issue and resolved the bug.")
        with _config_patches():
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            result = gen.generate_eod_narrative(target_date=TODAY)
        assert isinstance(result, str)
        assert result.strip()

    def test_contains_ticket_ids(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _LINKED_ROWS)
        mock_provider = _make_mock_provider("Completed work on the ticket.")
        with _config_patches():
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            result = gen.generate_eod_narrative(target_date=TODAY)
        assert "PROJ-1" in result, f"Expected PROJ-1 in report; got:\n{result}"
        assert "PROJ-2" in result, f"Expected PROJ-2 in report; got:\n{result}"

    def test_header_contains_date(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _LINKED_ROWS)
        mock_provider = _make_mock_provider("Done.")
        with _config_patches():
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            result = gen.generate_eod_narrative(target_date=TODAY)
        assert TODAY in result, f"Report header should contain target_date; got:\n{result}"

    def test_llm_called_for_each_ticket(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _LINKED_ROWS)
        mock_provider = _make_mock_provider("Narrative paragraph.")
        with _config_patches():
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            gen.generate_eod_narrative(target_date=TODAY)
        # Two distinct ticket IDs → two LLM calls
        assert mock_provider.generate.call_count == 2, (
            f"Expected 2 LLM calls (one per ticket); got {mock_provider.generate.call_count}"
        )


# ---------------------------------------------------------------------------
# 2. LLM unavailable — plain-text bullet fallback, no exception
# ---------------------------------------------------------------------------

class TestEODNarrativeLLMUnavailable:
    """When the LLM raises the report should contain raw commit messages as bullets."""

    def test_no_exception_propagates(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _LINKED_ROWS)
        mock_provider = MagicMock()
        mock_provider.generate.side_effect = ConnectionError("Ollama unreachable")
        with _config_patches():
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            # Must not raise
            result = gen.generate_eod_narrative(target_date=TODAY)
        assert isinstance(result, str)

    def test_fallback_contains_commit_messages(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _LINKED_ROWS)
        mock_provider = MagicMock()
        mock_provider.generate.side_effect = RuntimeError("model not loaded")
        with _config_patches():
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            result = gen.generate_eod_narrative(target_date=TODAY)
        # The fallback uses bullet lists of the raw commit messages
        assert "fix: resolve login bug" in result or "fix: add unit test for login" in result, (
            f"Fallback should contain raw commit messages; got:\n{result}"
        )

    def test_fallback_result_not_empty(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _LINKED_ROWS)
        mock_provider = MagicMock()
        mock_provider.generate.side_effect = OSError("connection refused")
        with _config_patches():
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            result = gen.generate_eod_narrative(target_date=TODAY)
        assert result.strip(), "Fallback result must be a non-empty string"


# ---------------------------------------------------------------------------
# 3. Empty commit history — returns "No commits recorded", never raises
# ---------------------------------------------------------------------------

class TestEODNarrativeEmptyHistory:
    """When there are no commits for today the method should return a safe message."""

    def test_no_exception_on_empty_db(self, isolated_engine):
        _make_db_with_rows(isolated_engine, [])  # No rows at all
        from backend.daily_report_generator import DailyReportGenerator
        gen = DailyReportGenerator()
        # Must not raise
        result = gen.generate_eod_narrative(target_date=TODAY)
        assert isinstance(result, str)

    def test_returns_no_commits_message(self, isolated_engine):
        _make_db_with_rows(isolated_engine, [])
        from backend.daily_report_generator import DailyReportGenerator
        gen = DailyReportGenerator()
        result = gen.generate_eod_narrative(target_date=TODAY)
        assert "No commits recorded" in result, (
            f"Expected 'No commits recorded' in empty-day result; got: {result!r}"
        )

    def test_no_commits_on_different_date(self, isolated_engine):
        """Rows exist but for a different date — should still return empty-day message."""
        _make_db_with_rows(isolated_engine, [
            {"ticket_id": "PROJ-1", "commit_message": "old commit", "commit_hash": "fff", "timestamp": "2020-01-01 09:00:00"},
        ])
        from backend.daily_report_generator import DailyReportGenerator
        gen = DailyReportGenerator()
        result = gen.generate_eod_narrative(target_date=TODAY)
        assert "No commits recorded" in result


# ---------------------------------------------------------------------------
# 4. inject_style called with context_type="report"
# ---------------------------------------------------------------------------

class TestEODNarrativeInjectStyle:
    """inject_style must be called with context_type='report' for each ticket section."""

    def test_inject_style_called_with_report_context(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _LINKED_ROWS)
        mock_provider = _make_mock_provider("Done.")
        with (
            _config_patches(),
            patch(
                "backend.daily_report_generator._inject_style",
                wraps=lambda p, context_type="general", query_text=None: p,
            ) as mock_inject,
        ):
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            gen.generate_eod_narrative(target_date=TODAY)

        assert mock_inject.called, "_inject_style must have been called at least once"
        # All calls should use context_type="report"
        for call_args in mock_inject.call_args_list:
            _, kwargs = call_args
            assert kwargs.get("context_type") == "report", (
                f"_inject_style must always be called with context_type='report'; "
                f"got: {kwargs.get('context_type')!r}"
            )

    def test_inject_style_called_once_per_ticket(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _LINKED_ROWS)
        mock_provider = _make_mock_provider("Done.")
        with (
            _config_patches(),
            patch(
                "backend.daily_report_generator._inject_style",
                wraps=lambda p, context_type="general", query_text=None: p,
            ) as mock_inject,
        ):
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            gen.generate_eod_narrative(target_date=TODAY)

        # _LINKED_ROWS has 2 distinct ticket IDs → 2 inject_style calls
        assert mock_inject.call_count == 2, (
            f"Expected 2 inject_style calls (one per ticket); got {mock_inject.call_count}"
        )


# ---------------------------------------------------------------------------
# 5. Unlinked commits appear under "Other commits" section
# ---------------------------------------------------------------------------

class TestEODNarrativeUnlinkedCommits:
    """Commits with empty or 'unlinked' ticket_id must appear under 'Other commits'."""

    def test_other_commits_section_present(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _UNLINKED_ROWS)
        mock_provider = _make_mock_provider("Done.")
        with _config_patches():
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            result = gen.generate_eod_narrative(target_date=TODAY)
        assert "Other commits" in result, (
            f"Expected 'Other commits' section for unlinked rows; got:\n{result}"
        )

    def test_unlinked_commits_not_silently_dropped(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _UNLINKED_ROWS)
        mock_provider = _make_mock_provider("Done.")
        with _config_patches():
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            result = gen.generate_eod_narrative(target_date=TODAY)
        # The actual commit messages from unlinked rows must be in the output
        assert "update README" in result or "bump version" in result, (
            f"Unlinked commit messages must appear in report; got:\n{result}"
        )

    def test_mixed_linked_and_unlinked(self, isolated_engine):
        _make_db_with_rows(isolated_engine, _ALL_ROWS)
        mock_provider = _make_mock_provider("Narrative for this ticket.")
        with _config_patches():
            from backend.daily_report_generator import DailyReportGenerator
            gen = DailyReportGenerator(provider=mock_provider)
            result = gen.generate_eod_narrative(target_date=TODAY)
        assert "PROJ-1" in result
        assert "PROJ-2" in result
        assert "Other commits" in result

    def test_unlinked_string_ticket_id_also_bucketed(self, isolated_engine):
        """The literal string 'unlinked' should go into Other commits, not its own section."""
        _make_db_with_rows(isolated_engine, [
            {"ticket_id": "unlinked", "commit_message": "chore: cleanup", "commit_hash": "ggg", "timestamp": f"{TODAY} 08:00:00"},
        ])
        from backend.daily_report_generator import DailyReportGenerator
        gen = DailyReportGenerator()
        result = gen.generate_eod_narrative(target_date=TODAY)
        assert "Other commits" in result
        # The section header should NOT be "## unlinked"
        assert "## unlinked" not in result
