"""
Tests for backend/admin/user_manager.py

Uses an in-memory / tmp SQLite DB via the DATABASE_DIR env override so
these tests never touch the real admin.db.

Covers:
  - init_db / ensure_default_admin
  - User CRUD: create, get, list, delete, update_password, update_role
  - API key lifecycle: create, list, revoke
  - Audit log: log_action, get_audit_log
"""
from __future__ import annotations

import os
import pytest


# ---------------------------------------------------------------------------
# Fixture: isolated temp DB for every test
# ---------------------------------------------------------------------------

@pytest.fixture(autouse=True)
def tmp_admin_db(tmp_path, monkeypatch):
    """Point DATABASE_DIR at a fresh temp directory each test."""
    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    # Force re-import so _admin_db_path() picks up the new env var.
    import importlib
    import backend.admin.user_manager as um
    importlib.reload(um)
    um.init_db()
    yield um


# ---------------------------------------------------------------------------
# init_db / ensure_default_admin
# ---------------------------------------------------------------------------

class TestInitDb:
    def test_tables_created(self, tmp_admin_db):
        import sqlite3
        db_path = tmp_admin_db._admin_db_path()
        con = sqlite3.connect(str(db_path))
        tables = {r[0] for r in con.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()}
        con.close()
        assert "admin_users" in tables
        assert "admin_api_keys" in tables
        assert "audit_log" in tables

    def test_idempotent(self, tmp_admin_db):
        """Calling init_db twice doesn't raise."""
        tmp_admin_db.init_db()

    def test_ensure_default_admin_creates_when_empty(self, tmp_admin_db):
        tmp_admin_db.ensure_default_admin("admin", "pass123")
        user = tmp_admin_db.get_user("admin")
        assert user is not None
        assert user.role == "admin"

    def test_ensure_default_admin_skips_when_users_exist(self, tmp_admin_db):
        tmp_admin_db.create_user("existing", "pass", "viewer")
        tmp_admin_db.ensure_default_admin("admin", "pass123")
        # "admin" user should NOT have been created
        assert tmp_admin_db.get_user("admin") is None


# ---------------------------------------------------------------------------
# User CRUD
# ---------------------------------------------------------------------------

class TestUserCrud:
    def test_create_and_get_user(self, tmp_admin_db):
        user = tmp_admin_db.create_user("alice", "secret", "viewer")
        assert user.username == "alice"
        assert user.role == "viewer"

    def test_get_nonexistent_user_returns_none(self, tmp_admin_db):
        assert tmp_admin_db.get_user("nobody") is None

    def test_list_users_empty(self, tmp_admin_db):
        assert tmp_admin_db.list_users() == []

    def test_list_users_returns_all(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "a", "viewer")
        tmp_admin_db.create_user("bob", "b", "admin")
        names = {u.username for u in tmp_admin_db.list_users()}
        assert names == {"alice", "bob"}

    def test_delete_user(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "a", "viewer")
        result = tmp_admin_db.delete_user("alice")
        assert result is True
        assert tmp_admin_db.get_user("alice") is None

    def test_delete_nonexistent_returns_false(self, tmp_admin_db):
        assert tmp_admin_db.delete_user("nobody") is False

    def test_update_password(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "oldpass", "viewer")
        result = tmp_admin_db.update_password("alice", "newpass")
        assert result is True
        assert tmp_admin_db.verify_user_password("alice", "newpass") is True
        assert tmp_admin_db.verify_user_password("alice", "oldpass") is False

    def test_update_password_nonexistent_returns_false(self, tmp_admin_db):
        assert tmp_admin_db.update_password("nobody", "pw") is False

    def test_update_role(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        result = tmp_admin_db.update_role("alice", "admin")
        assert result is True
        user = tmp_admin_db.get_user("alice")
        assert user.role == "admin"

    def test_update_role_nonexistent_returns_false(self, tmp_admin_db):
        assert tmp_admin_db.update_role("nobody", "admin") is False

    def test_touch_last_login(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        user_before = tmp_admin_db.get_user("alice")
        assert user_before.last_login is None
        tmp_admin_db.touch_last_login("alice")
        user_after = tmp_admin_db.get_user("alice")
        assert user_after.last_login is not None

    def test_verify_user_password_correct(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "correct", "viewer")
        assert tmp_admin_db.verify_user_password("alice", "correct") is True

    def test_verify_user_password_wrong(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "correct", "viewer")
        assert tmp_admin_db.verify_user_password("alice", "wrong") is False

    def test_verify_user_password_nonexistent(self, tmp_admin_db):
        assert tmp_admin_db.verify_user_password("nobody", "pass") is False

    def test_duplicate_username_raises(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        import sqlite3
        with pytest.raises(sqlite3.IntegrityError):
            tmp_admin_db.create_user("alice", "pass2", "viewer")


# ---------------------------------------------------------------------------
# API key lifecycle
# ---------------------------------------------------------------------------

class TestApiKeys:
    def test_create_api_key_returns_raw_and_record(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        raw, key = tmp_admin_db.create_api_key("alice", "my-key")
        assert len(raw) > 16
        assert key.key_prefix == raw[:8]
        assert key.label == "my-key"

    def test_create_api_key_for_nonexistent_user_raises(self, tmp_admin_db):
        with pytest.raises(ValueError, match="User not found"):
            tmp_admin_db.create_api_key("nobody", "label")

    def test_list_api_keys_empty(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        assert tmp_admin_db.list_api_keys("alice") == []

    def test_list_api_keys_returns_all(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        tmp_admin_db.create_api_key("alice", "key-1")
        tmp_admin_db.create_api_key("alice", "key-2")
        keys = tmp_admin_db.list_api_keys("alice")
        assert len(keys) == 2
        labels = {k.label for k in keys}
        assert labels == {"key-1", "key-2"}

    def test_list_api_keys_nonexistent_user_returns_empty(self, tmp_admin_db):
        assert tmp_admin_db.list_api_keys("nobody") == []

    def test_revoke_api_key(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        _, key = tmp_admin_db.create_api_key("alice", "k")
        result = tmp_admin_db.revoke_api_key(key.id)
        assert result is True
        assert tmp_admin_db.list_api_keys("alice") == []

    def test_revoke_nonexistent_key_returns_false(self, tmp_admin_db):
        assert tmp_admin_db.revoke_api_key(99999) is False

    def test_raw_key_is_unique_each_call(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        raw1, _ = tmp_admin_db.create_api_key("alice", "a")
        raw2, _ = tmp_admin_db.create_api_key("alice", "b")
        assert raw1 != raw2

    def test_delete_user_cascades_api_keys(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        tmp_admin_db.create_api_key("alice", "k")
        tmp_admin_db.delete_user("alice")
        # After user deleted, listing keys for that user should be empty
        assert tmp_admin_db.list_api_keys("alice") == []


# ---------------------------------------------------------------------------
# Audit log
# ---------------------------------------------------------------------------

class TestAuditLog:
    def test_log_action_creates_entry(self, tmp_admin_db):
        tmp_admin_db.log_action("admin", "login", ip="127.0.0.1")
        entries = tmp_admin_db.get_audit_log()
        assert len(entries) == 1
        assert entries[0]["action"] == "login"
        assert entries[0]["username"] == "admin"

    def test_multiple_actions_logged(self, tmp_admin_db):
        tmp_admin_db.log_action("admin", "login")
        tmp_admin_db.log_action("admin", "create_user", detail="username=bob")
        tmp_admin_db.log_action("bob", "login")
        entries = tmp_admin_db.get_audit_log()
        assert len(entries) == 3

    def test_get_audit_log_ordered_newest_first(self, tmp_admin_db):
        tmp_admin_db.log_action("admin", "action_first")
        tmp_admin_db.log_action("admin", "action_second")
        entries = tmp_admin_db.get_audit_log()
        assert entries[0]["action"] == "action_second"
        assert entries[1]["action"] == "action_first"

    def test_get_audit_log_respects_limit(self, tmp_admin_db):
        for i in range(10):
            tmp_admin_db.log_action("admin", f"action_{i}")
        entries = tmp_admin_db.get_audit_log(limit=3)
        assert len(entries) == 3

    def test_log_action_detail_and_ip_stored(self, tmp_admin_db):
        tmp_admin_db.log_action("admin", "delete_user", detail="username=alice", ip="10.0.0.1")
        entry = tmp_admin_db.get_audit_log()[0]
        assert entry["detail"] == "username=alice"
        assert entry["ip"] == "10.0.0.1"

    def test_get_audit_log_empty(self, tmp_admin_db):
        assert tmp_admin_db.get_audit_log() == []


# ---------------------------------------------------------------------------
# disable_user / enable_user
# ---------------------------------------------------------------------------

class TestDisableEnable:
    def test_new_user_is_enabled_by_default(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        user = tmp_admin_db.get_user("alice")
        assert user.disabled is False

    def test_disable_user_sets_flag(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        result = tmp_admin_db.disable_user("alice")
        assert result is True
        user = tmp_admin_db.get_user("alice")
        assert user.disabled is True

    def test_enable_user_clears_flag(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        tmp_admin_db.disable_user("alice")
        result = tmp_admin_db.enable_user("alice")
        assert result is True
        user = tmp_admin_db.get_user("alice")
        assert user.disabled is False

    def test_disable_nonexistent_returns_false(self, tmp_admin_db):
        assert tmp_admin_db.disable_user("nobody") is False

    def test_enable_nonexistent_returns_false(self, tmp_admin_db):
        assert tmp_admin_db.enable_user("nobody") is False

    def test_disabled_flag_visible_in_list_users(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        tmp_admin_db.create_user("bob", "pass", "viewer")
        tmp_admin_db.disable_user("alice")
        users = {u.username: u for u in tmp_admin_db.list_users()}
        assert users["alice"].disabled is True
        assert users["bob"].disabled is False

    def test_enable_already_enabled_is_idempotent(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        # enable on an already-enabled user should not raise and should return True
        result = tmp_admin_db.enable_user("alice")
        assert result is True
        assert tmp_admin_db.get_user("alice").disabled is False

    def test_disable_already_disabled_is_idempotent(self, tmp_admin_db):
        tmp_admin_db.create_user("alice", "pass", "viewer")
        tmp_admin_db.disable_user("alice")
        result = tmp_admin_db.disable_user("alice")
        assert result is True
        assert tmp_admin_db.get_user("alice").disabled is True
