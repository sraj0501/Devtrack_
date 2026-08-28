"""
Tests for backend/db/voice_seed_store.py -- Postgres backend epic (TASK-112).

voice_seeded_commits is a Python-owned table (no Go involvement) on the
shared SQLAlchemy engine -- works identically in SQLite and PostgreSQL mode,
same convention as db/ticket_db.py and db/report_store.py.
"""
from __future__ import annotations

from pathlib import Path

import pytest

from backend.db.engine import reset_engine


@pytest.fixture()
def isolated_engine(tmp_path: Path, monkeypatch):
    import backend.db.voice_seed_store as vss

    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    vss._schema_done = False
    yield tmp_path
    reset_engine()
    vss._schema_done = False


class TestIsAlreadySeededMarkSeeded:
    def test_not_seeded_initially(self, isolated_engine: Path):
        from backend.db.voice_seed_store import is_already_seeded

        assert is_already_seeded("abc123", "/repo/path") is False

    def test_mark_then_check(self, isolated_engine: Path):
        from backend.db.voice_seed_store import is_already_seeded, mark_seeded

        mark_seeded("abc123", "/repo/path")

        assert is_already_seeded("abc123", "/repo/path") is True

    def test_different_repo_path_not_seeded(self, isolated_engine: Path):
        from backend.db.voice_seed_store import is_already_seeded, mark_seeded

        mark_seeded("abc123", "/repo/a")

        assert is_already_seeded("abc123", "/repo/b") is False

    def test_mark_seeded_idempotent(self, isolated_engine: Path):
        """Marking the same (hash, repo_path) twice must not raise."""
        from backend.db.voice_seed_store import is_already_seeded, mark_seeded

        mark_seeded("abc123", "/repo/path")
        mark_seeded("abc123", "/repo/path")

        assert is_already_seeded("abc123", "/repo/path") is True

    def test_latest_seeded_at(self, isolated_engine: Path):
        from backend.db.voice_seed_store import latest_seeded_at, mark_seeded

        assert latest_seeded_at() is None

        mark_seeded("abc123", "/repo/path")

        assert latest_seeded_at() is not None

    def test_latest_seeded_at_fails_closed(self, isolated_engine: Path):
        from unittest.mock import MagicMock

        from backend.db.voice_seed_store import latest_seeded_at

        broken_engine = MagicMock()
        broken_engine.connect.side_effect = RuntimeError("database unavailable")

        assert latest_seeded_at(broken_engine) is None

    def test_works_across_postgres_and_sqlite_without_boundary_guard(self, isolated_engine: Path):
        """Unlike Go-owned tables, this one has no is_postgres() fail-closed
        branch -- it's Python-owned, so it must work the same in both modes.
        This test just confirms no such guard silently no-ops it."""
        from backend.db import voice_seed_store

        assert not hasattr(voice_seed_store, "is_postgres")
