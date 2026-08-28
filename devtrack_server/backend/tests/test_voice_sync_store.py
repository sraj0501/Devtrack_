"""
Tests for backend/db/voice_sync_store.py -- Postgres backend epic (TASK-112).

voice_synced_items is a Python-owned table (no Go involvement) on the
shared SQLAlchemy engine -- works identically in SQLite and PostgreSQL
mode, same convention as db/voice_seed_store.py and db/report_store.py.
"""
from __future__ import annotations

from pathlib import Path

import pytest

from backend.db.engine import reset_engine


@pytest.fixture()
def isolated_engine(tmp_path: Path, monkeypatch):
    import backend.db.voice_sync_store as vss

    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    vss._schema_done = False
    yield tmp_path
    reset_engine()
    vss._schema_done = False


class TestIsAlreadySyncedMarkSynced:
    def test_not_synced_initially(self, isolated_engine: Path):
        from backend.db.voice_sync_store import is_already_synced

        assert is_already_synced("github", "github-pr-1", "description") is False

    def test_mark_then_check(self, isolated_engine: Path):
        from backend.db.voice_sync_store import is_already_synced, mark_synced

        mark_synced("github", "github-pr-1", "description")

        assert is_already_synced("github", "github-pr-1", "description") is True

    def test_distinct_context_type_not_synced(self, isolated_engine: Path):
        """The same (platform, item_id) with a different context_type is a
        distinct tracked row (a PR description and a comment on the same
        underlying item_id must not collide)."""
        from backend.db.voice_sync_store import is_already_synced, mark_synced

        mark_synced("github", "42", "description")

        assert is_already_synced("github", "42", "comment") is False

    def test_different_platform_not_synced(self, isolated_engine: Path):
        from backend.db.voice_sync_store import is_already_synced, mark_synced

        mark_synced("github", "42", "description")

        assert is_already_synced("gitlab", "42", "description") is False

    def test_mark_synced_idempotent(self, isolated_engine: Path):
        from backend.db.voice_sync_store import is_already_synced, mark_synced

        mark_synced("github", "42", "description")
        mark_synced("github", "42", "description")

        assert is_already_synced("github", "42", "description") is True

    def test_latest_synced_at(self, isolated_engine: Path):
        from backend.db.voice_sync_store import latest_synced_at, mark_synced

        assert latest_synced_at() is None

        mark_synced("github", "42", "description")

        assert latest_synced_at() is not None

    def test_latest_synced_at_fails_closed(self, isolated_engine: Path):
        from unittest.mock import MagicMock

        from backend.db.voice_sync_store import latest_synced_at

        broken_engine = MagicMock()
        broken_engine.connect.side_effect = RuntimeError("database unavailable")

        assert latest_synced_at(broken_engine) is None
