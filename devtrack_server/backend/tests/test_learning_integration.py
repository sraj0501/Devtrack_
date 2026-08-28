"""
Tests for learning_integration.py's _load_last_collected()/_save_last_collected()
— Postgres backend epic (TASK-112).

These delegate to backend.db.learning_store.load_last_collected()/
save_last_collected(), which is already dual-dialect via backend.db.engine
(learning_sync_state is a Python-owned table, no Go involvement). Before
this fix, _load_last_collected() imported a since-removed
learning_store._db_path() helper, so it always silently returned None
(the ImportError was swallowed) regardless of what was actually persisted.
"""
from __future__ import annotations

import sys
from datetime import datetime, timezone
from pathlib import Path

import pytest

_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


@pytest.fixture()
def isolated_engine(tmp_path, monkeypatch):
    """learning_store also caches a module-level ``_schema_done`` flag (same
    gap as report_store.py/test_daily_report_generator.py) — reset it
    alongside reset_engine() or create_all() gets skipped on the fresh file."""
    from backend.db.engine import reset_engine
    import backend.db.learning_store as learning_store

    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    reset_engine()
    learning_store._schema_done = False
    yield tmp_path
    reset_engine()
    learning_store._schema_done = False


class TestLoadSaveLastCollected:
    def test_round_trip(self, isolated_engine):
        from backend.learning_integration import _load_last_collected, _save_last_collected

        assert _load_last_collected("dev@example.com") is None

        ts = datetime(2026, 7, 31, 12, 0, 0, tzinfo=timezone.utc)
        _save_last_collected(ts, user_email="dev@example.com")

        loaded = _load_last_collected("dev@example.com")
        assert loaded is not None
        assert loaded.replace(tzinfo=None) == ts.replace(tzinfo=None)

    def test_different_users_are_isolated(self, isolated_engine):
        from backend.learning_integration import _load_last_collected, _save_last_collected

        ts = datetime(2026, 7, 31, 9, 0, 0, tzinfo=timezone.utc)
        _save_last_collected(ts, user_email="a@example.com")

        assert _load_last_collected("b@example.com") is None
        assert _load_last_collected("a@example.com") is not None

    def test_empty_user_email_never_raises(self, isolated_engine):
        from backend.learning_integration import _load_last_collected

        assert _load_last_collected("") is None
