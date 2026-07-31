"""
Tests for backend/vacation/auto_responder.py's get_vacation_state() /
is_vacation_active() (TASK-112 port, module 6 of 15).

auto_responder.py reads the Go-owned ``vacation_mode`` table through
backend.db.engine.get_engine() (SQLite mode) rather than a bespoke
sqlite3.connect(). These tests exercise that path against a real,
temp-directory SQLite DB via an isolated engine (same fixture pattern as
test_skill_detector.py / test_voice_add_status.py's isolated_dialectic_engine),
plus the PostgreSQL-mode fail-closed path via a monkeypatched is_postgres().

Only get_vacation_state()/is_vacation_active() are covered here — handle(),
_generate_update(), _submit_update(), etc. are LLM/async logic out of scope
for this port and are untouched by it.
"""
from __future__ import annotations

import sqlite3
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest.mock import patch

import pytest

from backend.db.engine import reset_engine


# ---------------------------------------------------------------------------
# Fixture: isolated SQLAlchemy engine pointed at a fresh temp DB directory
# ---------------------------------------------------------------------------

@pytest.fixture()
def isolated_vacation_engine(tmp_path: Path, monkeypatch):
    """Point DATABASE_DIR at a fresh temp directory and reset the shared
    SQLAlchemy engine singleton so get_vacation_state()'s engine-based reads
    hit an isolated SQLite file instead of whatever engine a prior test built.
    Yields the temp directory; the DB file itself is `devtrack.db` inside it
    (backend.config.database_path()'s default filename).
    """
    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    yield tmp_path
    reset_engine()


def _make_vacation_db(
    db_path: Path,
    enabled: int = 1,
    enabled_at: str = "2026-07-01T00:00:00Z",
    until: str = "",
    confidence_threshold: float = 0.7,
    auto_submit: int = 1,
) -> None:
    """Create a vacation_mode table + single row matching the Go client's
    exact schema (devtrack_client/internal/db/database.go):

        CREATE TABLE IF NOT EXISTS vacation_mode (
            id                   INTEGER PRIMARY KEY CHECK (id = 1),
            enabled              INTEGER NOT NULL DEFAULT 0,
            enabled_at           TEXT,
            until                TEXT,
            confidence_threshold REAL    NOT NULL DEFAULT 0.7,
            auto_submit          INTEGER NOT NULL DEFAULT 1
        );
    """
    conn = sqlite3.connect(str(db_path))
    conn.execute(
        """
        CREATE TABLE vacation_mode (
            id                   INTEGER PRIMARY KEY CHECK (id = 1),
            enabled              INTEGER NOT NULL DEFAULT 0,
            enabled_at           TEXT,
            until                TEXT,
            confidence_threshold REAL    NOT NULL DEFAULT 0.7,
            auto_submit          INTEGER NOT NULL DEFAULT 1
        )
        """
    )
    conn.execute(
        "INSERT INTO vacation_mode (id, enabled, enabled_at, until, confidence_threshold, auto_submit)"
        " VALUES (1, ?, ?, ?, ?, ?)",
        (enabled, enabled_at, until, confidence_threshold, auto_submit),
    )
    conn.commit()
    conn.close()


# ---------------------------------------------------------------------------
# get_vacation_state() -- real DB
# ---------------------------------------------------------------------------

class TestGetVacationStateUnit:
    """Unit tests for get_vacation_state() (SQLite mode)."""

    def test_happy_path_returns_vacation_state(
        self, isolated_vacation_engine: Path
    ) -> None:
        """A real vacation_mode row is read back correctly."""
        from backend.vacation.auto_responder import get_vacation_state, VacationState

        db_path = isolated_vacation_engine / "devtrack.db"
        _make_vacation_db(
            db_path,
            enabled=1,
            enabled_at="2026-07-01T00:00:00Z",
            until="2026-08-01",
            confidence_threshold=0.75,
            auto_submit=0,
        )

        state = get_vacation_state()

        assert state is not None
        assert isinstance(state, VacationState)
        assert state.enabled is True
        assert state.enabled_at == "2026-07-01T00:00:00Z"
        assert state.until == "2026-08-01"
        assert state.confidence_threshold == 0.75
        assert state.auto_submit is False

    def test_disabled_row_returns_state_with_enabled_false(
        self, isolated_vacation_engine: Path
    ) -> None:
        """A row with enabled=0 still returns a VacationState, just disabled."""
        from backend.vacation.auto_responder import get_vacation_state

        db_path = isolated_vacation_engine / "devtrack.db"
        _make_vacation_db(db_path, enabled=0)

        state = get_vacation_state()

        assert state is not None
        assert state.enabled is False

    def test_nonexistent_db_returns_none(
        self, isolated_vacation_engine: Path
    ) -> None:
        """get_vacation_state() with a nonexistent DB file returns None without raising."""
        from backend.vacation.auto_responder import get_vacation_state

        # isolated_vacation_engine points DATABASE_DIR at a fresh tmp_path
        # with no devtrack.db file in it yet.
        state = get_vacation_state()

        assert state is None

    def test_missing_table_returns_none(
        self, isolated_vacation_engine: Path
    ) -> None:
        """get_vacation_state() returns None when the DB exists but the
        vacation_mode table does not (e.g. an old client DB predating the
        vacation-mode feature)."""
        from backend.vacation.auto_responder import get_vacation_state

        db_path = isolated_vacation_engine / "devtrack.db"
        conn = sqlite3.connect(str(db_path))
        conn.execute("CREATE TABLE other_table (id INTEGER PRIMARY KEY)")
        conn.commit()
        conn.close()

        state = get_vacation_state()

        assert state is None

    def test_missing_row_returns_none(
        self, isolated_vacation_engine: Path
    ) -> None:
        """get_vacation_state() returns None when the table exists but has no id=1 row."""
        from backend.vacation.auto_responder import get_vacation_state

        db_path = isolated_vacation_engine / "devtrack.db"
        conn = sqlite3.connect(str(db_path))
        conn.execute(
            """
            CREATE TABLE vacation_mode (
                id                   INTEGER PRIMARY KEY CHECK (id = 1),
                enabled              INTEGER NOT NULL DEFAULT 0,
                enabled_at           TEXT,
                until                TEXT,
                confidence_threshold REAL    NOT NULL DEFAULT 0.7,
                auto_submit          INTEGER NOT NULL DEFAULT 1
            )
            """
        )
        conn.commit()
        conn.close()

        state = get_vacation_state()

        assert state is None


# ---------------------------------------------------------------------------
# is_vacation_active()
# ---------------------------------------------------------------------------

class TestIsVacationActive:
    """is_vacation_active() combines get_vacation_state() with expiry logic."""

    def test_true_when_enabled_and_not_expired(
        self, isolated_vacation_engine: Path
    ) -> None:
        """enabled=1, until in the future -> True."""
        from backend.vacation.auto_responder import is_vacation_active

        future = (datetime.now(timezone.utc).date() + timedelta(days=30)).isoformat()
        db_path = isolated_vacation_engine / "devtrack.db"
        _make_vacation_db(db_path, enabled=1, until=future)

        assert is_vacation_active() is True

    def test_false_when_disabled(self, isolated_vacation_engine: Path) -> None:
        """enabled=0 -> False regardless of until."""
        from backend.vacation.auto_responder import is_vacation_active

        future = (datetime.now(timezone.utc).date() + timedelta(days=30)).isoformat()
        db_path = isolated_vacation_engine / "devtrack.db"
        _make_vacation_db(db_path, enabled=0, until=future)

        assert is_vacation_active() is False

    def test_false_when_until_date_has_passed(
        self, isolated_vacation_engine: Path
    ) -> None:
        """enabled=1, until in the past -> False (expired)."""
        from backend.vacation.auto_responder import is_vacation_active

        past = (datetime.now(timezone.utc).date() - timedelta(days=1)).isoformat()
        db_path = isolated_vacation_engine / "devtrack.db"
        _make_vacation_db(db_path, enabled=1, until=past)

        assert is_vacation_active() is False

    def test_true_on_exact_until_date(self, isolated_vacation_engine: Path) -> None:
        """enabled=1, until == today -> True (whole until-day is valid)."""
        from backend.vacation.auto_responder import is_vacation_active

        today = datetime.now(timezone.utc).date().isoformat()
        db_path = isolated_vacation_engine / "devtrack.db"
        _make_vacation_db(db_path, enabled=1, until=today)

        assert is_vacation_active() is True

    def test_true_when_until_is_empty_indefinite(
        self, isolated_vacation_engine: Path
    ) -> None:
        """enabled=1, until empty -> indefinite vacation, always True."""
        from backend.vacation.auto_responder import is_vacation_active

        db_path = isolated_vacation_engine / "devtrack.db"
        _make_vacation_db(db_path, enabled=1, until="")

        assert is_vacation_active() is True

    def test_false_when_no_state_at_all(
        self, isolated_vacation_engine: Path
    ) -> None:
        """No DB / no row -> get_vacation_state() returns None -> False."""
        from backend.vacation.auto_responder import is_vacation_active

        assert is_vacation_active() is False


# ---------------------------------------------------------------------------
# PostgreSQL-mode boundary-rule behaviour
# ---------------------------------------------------------------------------

class TestSetVacationState:
    """set_vacation_state() -- the only writer, added for the Slack/Telegram
    /devtrack vacation on|off commands' Postgres port (TASK-112, module 10)."""

    def test_enable_writes_row(self, isolated_vacation_engine: Path) -> None:
        from backend.vacation.auto_responder import set_vacation_state, get_vacation_state

        db_path = isolated_vacation_engine / "devtrack.db"
        _make_vacation_db(db_path, enabled=0)

        ok = set_vacation_state(True, until="2026-09-01", confidence_threshold=0.9, auto_submit=False)

        assert ok is True
        state = get_vacation_state()
        assert state.enabled is True
        assert state.until == "2026-09-01"
        assert state.confidence_threshold == 0.9
        assert state.auto_submit is False

    def test_disable_writes_row(self, isolated_vacation_engine: Path) -> None:
        from backend.vacation.auto_responder import set_vacation_state, get_vacation_state

        db_path = isolated_vacation_engine / "devtrack.db"
        _make_vacation_db(db_path, enabled=1)

        ok = set_vacation_state(False)

        assert ok is True
        assert get_vacation_state().enabled is False

    def test_missing_row_returns_false(self, isolated_vacation_engine: Path) -> None:
        """UPDATE ... WHERE id=1 against a table with no row updates nothing --
        matches the original raw-sqlite3 code's behavior (no INSERT fallback)."""
        from backend.vacation.auto_responder import set_vacation_state

        db_path = isolated_vacation_engine / "devtrack.db"
        conn = sqlite3.connect(str(db_path))
        conn.execute(
            """
            CREATE TABLE vacation_mode (
                id                   INTEGER PRIMARY KEY CHECK (id = 1),
                enabled              INTEGER NOT NULL DEFAULT 0,
                enabled_at           TEXT,
                until                TEXT,
                confidence_threshold REAL    NOT NULL DEFAULT 0.7,
                auto_submit          INTEGER NOT NULL DEFAULT 1
            )
            """
        )
        conn.commit()
        conn.close()

        assert set_vacation_state(True) is False

    def test_nonexistent_db_returns_false_never_raises(
        self, isolated_vacation_engine: Path
    ) -> None:
        from backend.vacation.auto_responder import set_vacation_state

        assert set_vacation_state(True) is False

    def test_postgres_mode_returns_false_without_touching_engine(self) -> None:
        from backend.vacation.auto_responder import set_vacation_state

        with patch("backend.vacation.auto_responder.is_postgres", return_value=True), \
             patch("backend.vacation.auto_responder.get_engine") as mock_get_engine:
            result = set_vacation_state(True)

        mock_get_engine.assert_not_called()
        assert result is False


class TestVacationStatePostgresMode:
    """PostgreSQL-mode boundary-rule behaviour: vacation_mode is a Go-owned
    SQLite-only table not yet exposed over HTTP, so get_vacation_state() must
    fail closed (return None) without touching the engine or raising."""

    def test_get_vacation_state_returns_none_without_touching_engine(self) -> None:
        from backend.vacation.auto_responder import get_vacation_state

        with patch("backend.vacation.auto_responder.is_postgres", return_value=True), \
             patch("backend.vacation.auto_responder.get_engine") as mock_get_engine:
            result = get_vacation_state()

        mock_get_engine.assert_not_called()
        assert result is None

    def test_is_vacation_active_false_in_postgres_mode(self) -> None:
        from backend.vacation.auto_responder import is_vacation_active

        with patch("backend.vacation.auto_responder.is_postgres", return_value=True), \
             patch("backend.vacation.auto_responder.get_engine") as mock_get_engine:
            result = is_vacation_active()

        mock_get_engine.assert_not_called()
        assert result is False
