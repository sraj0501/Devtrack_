"""
Tests for backend/admin/routes.py  (HTTP-level coverage via starlette TestClient)

All tests use a fresh in-memory SQLite database isolated to tmp_path via the
DATABASE_DIR env override — no writes to the real Data/ directory.

The ServerSnapshot returned by get_snapshot() is mocked for every test so that
no live processes, psutil calls, or health HTTP checks are required.

Naming convention: TestXxx classes group routes by feature area.
"""
from __future__ import annotations

import importlib
from typing import Generator
from unittest.mock import MagicMock, patch

import pytest
from starlette.testclient import TestClient


# ---------------------------------------------------------------------------
# Helpers / stubs
# ---------------------------------------------------------------------------

def _make_snapshot():
    """Return a minimal ServerSnapshot with no processes and no services."""
    from backend.admin.server_status import ServerSnapshot
    return ServerSnapshot(
        processes=[],
        services=[],
        llm_provider="ollama",
        llm_model="llama3",
        webhook_port=8089,
        admin_port=8090,
    )


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture()
def db_dir(tmp_path, monkeypatch) -> Generator:
    """Isolate DATABASE_DIR and DATA_DIR to tmp_path for every test.

    Also sets ADMIN_USERNAME / ADMIN_PASSWORD so check_credentials() (which reads
    from env vars, not the DB) can authenticate in login route tests.
    """
    monkeypatch.setenv("DATABASE_DIR", str(tmp_path))
    monkeypatch.setenv("DATA_DIR", str(tmp_path))
    monkeypatch.setenv("ADMIN_USERNAME", "admin")
    monkeypatch.setenv("ADMIN_PASSWORD", "adminpass")

    # Reload user_manager so _admin_db_path() picks up the new DATABASE_DIR.
    import backend.admin.user_manager as um
    importlib.reload(um)
    um.init_db()
    um.ensure_default_admin("admin", "adminpass")
    yield um


@pytest.fixture()
def client(db_dir) -> TestClient:
    """Return a TestClient wired to the admin FastAPI app.

    get_snapshot is patched so no live process checks run.
    ADMIN_USERNAME / ADMIN_PASSWORD env vars are already set by db_dir fixture.
    """
    with patch("backend.admin.routes.get_snapshot", return_value=_make_snapshot()):
        from backend.admin.app import app
        yield TestClient(app, raise_server_exceptions=True)


@pytest.fixture()
def auth_cookies() -> dict:
    """Return a cookie dict with a valid session for user 'admin'."""
    from backend.admin.auth import COOKIE_NAME, create_token
    return {COOKIE_NAME: create_token("admin")}


# ---------------------------------------------------------------------------
# TestLogin — GET /admin/login, POST /admin/login
# ---------------------------------------------------------------------------

class TestLogin:
    def test_login_page_returns_200(self, client):
        r = client.get("/admin/login")
        assert r.status_code == 200
        assert b"<form" in r.content

    def test_login_page_contains_username_field(self, client):
        r = client.get("/admin/login")
        assert b"username" in r.content

    def test_post_valid_credentials_sets_cookie_and_redirects(self, client):
        r = client.post(
            "/admin/login",
            data={"username": "admin", "password": "adminpass"},
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert r.headers["location"] == "/admin/"
        from backend.admin.auth import COOKIE_NAME
        assert COOKIE_NAME in r.cookies

    def test_post_wrong_password_returns_401(self, client):
        r = client.post(
            "/admin/login",
            data={"username": "admin", "password": "wrongpass"},
            follow_redirects=False,
        )
        assert r.status_code == 401
        assert b"Invalid" in r.content

    def test_post_unknown_user_returns_401(self, client):
        r = client.post(
            "/admin/login",
            data={"username": "nobody", "password": "anything"},
            follow_redirects=False,
        )
        assert r.status_code == 401

    def test_post_empty_password_returns_401(self, client):
        r = client.post(
            "/admin/login",
            data={"username": "admin", "password": ""},
            follow_redirects=False,
        )
        assert r.status_code == 401


# ---------------------------------------------------------------------------
# TestLogout — GET /admin/logout
# ---------------------------------------------------------------------------

class TestLogout:
    def test_logout_clears_cookie_and_redirects(self, client, auth_cookies):
        r = client.get("/admin/logout", cookies=auth_cookies, follow_redirects=False)
        assert r.status_code == 303
        assert "/admin/login" in r.headers["location"]

    def test_logout_without_session_redirects_to_login(self, client):
        r = client.get("/admin/logout", follow_redirects=False)
        # require_auth raises 303 redirect when no cookie present
        assert r.status_code == 303
        assert "/admin/login" in r.headers["location"]


# ---------------------------------------------------------------------------
# TestDashboard — GET /admin/
# ---------------------------------------------------------------------------

class TestDashboard:
    def test_dashboard_authenticated_returns_200(self, client, auth_cookies):
        with patch("backend.admin.routes.get_snapshot", return_value=_make_snapshot()):
            r = client.get("/admin/", cookies=auth_cookies)
        assert r.status_code == 200

    def test_dashboard_unauthenticated_redirects_to_login(self, client):
        r = client.get("/admin/", follow_redirects=False)
        assert r.status_code == 303
        assert "/admin/login" in r.headers["location"]

    def test_dashboard_shows_admin_section(self, client, auth_cookies):
        with patch("backend.admin.routes.get_snapshot", return_value=_make_snapshot()):
            r = client.get("/admin/", cookies=auth_cookies)
        assert r.status_code == 200
        # The page should contain navigation markers
        assert b"Dashboard" in r.content or b"dashboard" in r.content

    def test_dashboard_uses_stats_refresh_interval_from_env(self, client, auth_cookies, monkeypatch):
        """Dashboard HTML must use the value from STATS_REFRESH_INTERVAL_SECONDS, not a hardcoded literal."""
        monkeypatch.setenv("STATS_REFRESH_INTERVAL_SECONDS", "42")
        with patch("backend.admin.routes.get_snapshot", return_value=_make_snapshot()):
            r = client.get("/admin/", cookies=auth_cookies)
        assert r.status_code == 200
        assert b"42s" in r.content


# ---------------------------------------------------------------------------
# TestUsers — GET/POST /admin/users
# ---------------------------------------------------------------------------

class TestUsers:
    def test_users_page_returns_200(self, client, auth_cookies):
        r = client.get("/admin/users", cookies=auth_cookies)
        assert r.status_code == 200

    def test_users_page_unauthenticated_redirects(self, client):
        r = client.get("/admin/users", follow_redirects=False)
        assert r.status_code == 303

    def test_users_page_shows_admin_user(self, client, auth_cookies):
        r = client.get("/admin/users", cookies=auth_cookies)
        assert r.status_code == 200
        assert b"admin" in r.content

    def test_create_user_redirects_and_user_exists(self, client, auth_cookies, db_dir):
        r = client.post(
            "/admin/users/create",
            data={"username": "newuser", "password": "pass1234", "role": "viewer"},
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert "/admin/users" in r.headers["location"]
        # Verify user was actually created in the DB
        user = db_dir.get_user("newuser")
        assert user is not None
        assert user.role == "viewer"

    def test_create_duplicate_user_redirects_with_error(self, client, auth_cookies):
        # "admin" user already exists from fixture
        r = client.post(
            "/admin/users/create",
            data={"username": "admin", "password": "pass1234", "role": "viewer"},
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert "error" in r.headers["location"].lower() or "already" in r.headers["location"]

    def test_delete_other_user(self, client, auth_cookies, db_dir):
        db_dir.create_user("todelete", "pass", "viewer")
        r = client.post(
            "/admin/users/todelete/delete",
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert db_dir.get_user("todelete") is None

    def test_cannot_delete_self(self, client, auth_cookies, db_dir):
        r = client.post(
            "/admin/users/admin/delete",
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        # User must still exist
        assert db_dir.get_user("admin") is not None
        # Flash message should contain an error indicator
        assert "error" in r.headers["location"].lower() or "cannot" in r.headers["location"].lower()


# ---------------------------------------------------------------------------
# TestApiKeys — GET/POST /admin/users/{username}/keys
# ---------------------------------------------------------------------------

class TestApiKeys:
    def test_api_keys_page_returns_200(self, client, auth_cookies):
        r = client.get("/admin/users/admin/keys", cookies=auth_cookies)
        assert r.status_code == 200

    def test_api_keys_page_unauthenticated_redirects(self, client):
        r = client.get("/admin/users/admin/keys", follow_redirects=False)
        assert r.status_code == 303

    def test_create_api_key_redirects_with_new_key_param(self, client, auth_cookies):
        r = client.post(
            "/admin/users/admin/keys/create",
            data={"label": "ci-token"},
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        location = r.headers["location"]
        assert "new_key=" in location

    def test_revoke_api_key(self, client, auth_cookies, db_dir):
        raw, key = db_dir.create_api_key("admin", "test-label")
        r = client.post(
            f"/admin/keys/{key.id}/revoke",
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        # Key should be gone
        remaining = db_dir.list_api_keys("admin")
        assert all(k.id != key.id for k in remaining)


# ---------------------------------------------------------------------------
# TestPasswordReset — POST /admin/users/{username}/reset-password (CS-3 TASK-015)
# ---------------------------------------------------------------------------

class TestPasswordReset:
    def test_reset_other_user_password(self, client, auth_cookies, db_dir):
        db_dir.create_user("alice", "oldpass", "viewer")
        r = client.post(
            "/admin/users/alice/reset-password",
            data={"new_password": "newpass123", "current_password": ""},
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert db_dir.verify_user_password("alice", "newpass123") is True

    def test_reset_own_password_requires_current_password(self, client, auth_cookies, db_dir):
        r = client.post(
            "/admin/users/admin/reset-password",
            data={"new_password": "new123", "current_password": ""},
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert "error" in r.headers["location"].lower()

    def test_reset_own_password_with_wrong_current_password(self, client, auth_cookies, db_dir):
        r = client.post(
            "/admin/users/admin/reset-password",
            data={"new_password": "new123", "current_password": "wrongcurrent"},
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert "error" in r.headers["location"].lower()

    def test_reset_password_empty_new_password_blocked(self, client, auth_cookies, db_dir):
        db_dir.create_user("alice", "oldpass", "viewer")
        r = client.post(
            "/admin/users/alice/reset-password",
            data={"new_password": "", "current_password": ""},
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code in (303, 422)  # redirect with error or FastAPI validation

    def test_reset_password_unauthenticated_redirects(self, client, db_dir):
        db_dir.create_user("alice", "oldpass", "viewer")
        r = client.post(
            "/admin/users/alice/reset-password",
            data={"new_password": "new123"},
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert "/admin/login" in r.headers["location"]


# ---------------------------------------------------------------------------
# TestTriggerStats — HTMX partial GET /admin/_partials/stats (CS-3 TASK-014)
# ---------------------------------------------------------------------------

class TestTriggerStats:
    def test_stats_partial_returns_200(self, client, auth_cookies):
        r = client.get("/admin/_partials/stats", cookies=auth_cookies)
        assert r.status_code == 200

    def test_stats_partial_unauthenticated_redirects(self, client):
        r = client.get("/admin/_partials/stats", follow_redirects=False)
        assert r.status_code == 303
        assert "/admin/login" in r.headers["location"]

    def test_stats_partial_returns_html_with_stats_or_unavailable(self, client, auth_cookies):
        r = client.get("/admin/_partials/stats", cookies=auth_cookies)
        assert r.status_code == 200
        # Either the 4-stat grid or the graceful-degrade message must appear
        assert (
            b"Triggers Today" in r.content
            or b"Trigger stats unavailable" in r.content
        )

    def test_dashboard_includes_stats_panel(self, client, auth_cookies):
        with patch("backend.admin.routes.get_snapshot", return_value=_make_snapshot()):
            r = client.get("/admin/", cookies=auth_cookies)
        assert r.status_code == 200
        assert b"Trigger Activity" in r.content


# ---------------------------------------------------------------------------
# TestLicensePage — GET /admin/license (CS-3 TASK-013)
# ---------------------------------------------------------------------------

class TestLicensePage:
    def test_license_page_returns_200(self, client, auth_cookies):
        r = client.get("/admin/license", cookies=auth_cookies)
        assert r.status_code == 200

    def test_license_page_unauthenticated_redirects(self, client):
        r = client.get("/admin/license", follow_redirects=False)
        assert r.status_code == 303
        assert "/admin/login" in r.headers["location"]

    def test_license_page_shows_tier_info(self, client, auth_cookies):
        r = client.get("/admin/license", cookies=auth_cookies)
        assert r.status_code == 200
        # Should contain tier label text somewhere on the page
        assert b"Personal" in r.content or b"Team" in r.content or b"Enterprise" in r.content

    def test_dashboard_shows_license_tier_card(self, client, auth_cookies):
        with patch("backend.admin.routes.get_snapshot", return_value=_make_snapshot()):
            r = client.get("/admin/", cookies=auth_cookies)
        assert r.status_code == 200
        assert b"License Tier" in r.content

    def test_license_nav_link_in_base(self, client, auth_cookies):
        r = client.get("/admin/license", cookies=auth_cookies)
        assert r.status_code == 200
        # The sidebar nav should contain a link to /admin/license
        assert b"/admin/license" in r.content


# ---------------------------------------------------------------------------
# TestUserRoleDisable — new CS-3 routes
# ---------------------------------------------------------------------------

class TestUserRoleDisable:
    def test_update_role_to_admin(self, client, auth_cookies, db_dir):
        db_dir.create_user("alice", "pass", "viewer")
        r = client.post(
            "/admin/users/alice/role",
            data={"role": "admin"},
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert db_dir.get_user("alice").role == "admin"

    def test_update_role_invalid_value_redirects_with_error(self, client, auth_cookies, db_dir):
        db_dir.create_user("alice", "pass", "viewer")
        r = client.post(
            "/admin/users/alice/role",
            data={"role": "superuser"},
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert "error" in r.headers["location"].lower()

    def test_disable_user(self, client, auth_cookies, db_dir):
        db_dir.create_user("alice", "pass", "viewer")
        r = client.post(
            "/admin/users/alice/disable",
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert db_dir.get_user("alice").disabled is True

    def test_cannot_disable_self(self, client, auth_cookies, db_dir):
        r = client.post(
            "/admin/users/admin/disable",
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert "error" in r.headers["location"].lower() or "cannot" in r.headers["location"].lower()
        assert db_dir.get_user("admin").disabled is False

    def test_enable_user(self, client, auth_cookies, db_dir):
        db_dir.create_user("alice", "pass", "viewer")
        db_dir.disable_user("alice")
        r = client.post(
            "/admin/users/alice/enable",
            cookies=auth_cookies,
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert db_dir.get_user("alice").disabled is False

    def test_role_update_unauthenticated_redirects(self, client, db_dir):
        db_dir.create_user("alice", "pass", "viewer")
        r = client.post(
            "/admin/users/alice/role",
            data={"role": "admin"},
            follow_redirects=False,
        )
        assert r.status_code == 303
        assert "/admin/login" in r.headers["location"]


# ---------------------------------------------------------------------------
# TestServerPage — GET /admin/server
# ---------------------------------------------------------------------------

class TestServerPage:
    def test_server_page_returns_200(self, client, auth_cookies):
        with patch("backend.admin.routes.get_snapshot", return_value=_make_snapshot()):
            r = client.get("/admin/server", cookies=auth_cookies)
        assert r.status_code == 200

    def test_server_page_unauthenticated_redirects(self, client):
        r = client.get("/admin/server", follow_redirects=False)
        assert r.status_code == 303

    def test_server_page_shows_llm_section(self, client, auth_cookies):
        with patch("backend.admin.routes.get_snapshot", return_value=_make_snapshot()):
            r = client.get("/admin/server", cookies=auth_cookies)
        assert r.status_code == 200
        # Server page renders LLM_PROVIDER in the config table
        assert b"LLM" in r.content or b"llm" in r.content.lower()


# ---------------------------------------------------------------------------
# TestAuditPage — GET /admin/audit
# ---------------------------------------------------------------------------

class TestAuditPage:
    def test_audit_page_returns_200(self, client, auth_cookies):
        r = client.get("/admin/audit", cookies=auth_cookies)
        assert r.status_code == 200

    def test_audit_page_unauthenticated_redirects(self, client):
        r = client.get("/admin/audit", follow_redirects=False)
        assert r.status_code == 303

    def test_audit_page_shows_login_event(self, client, auth_cookies, db_dir):
        # Write an audit entry directly so we don't depend on DB path resolution
        # across module reloads.
        db_dir.log_action("admin", "login", detail="test entry", ip="127.0.0.1")
        r = client.get("/admin/audit", cookies=auth_cookies)
        assert r.status_code == 200
        assert b"login" in r.content


# ---------------------------------------------------------------------------
# TestPartials — HTMX partial routes
# ---------------------------------------------------------------------------

class TestPartials:
    def test_partial_processes_returns_200(self, client, auth_cookies):
        with patch("backend.admin.routes.get_snapshot", return_value=_make_snapshot()):
            r = client.get("/admin/_partials/processes", cookies=auth_cookies)
        assert r.status_code == 200

    def test_partial_processes_unauthenticated_redirects(self, client):
        r = client.get("/admin/_partials/processes", follow_redirects=False)
        assert r.status_code == 303

    def test_partial_processes_returns_html_fragment(self, client, auth_cookies):
        with patch("backend.admin.routes.get_snapshot", return_value=_make_snapshot()):
            r = client.get("/admin/_partials/processes", cookies=auth_cookies)
        # Should return an HTML table body fragment (tr elements or "No processes" message)
        assert b"<tr>" in r.content or b"No processes" in r.content


# ---------------------------------------------------------------------------
# TestAuditLogLimitConfig — config accessor validation
# ---------------------------------------------------------------------------

class TestAuditLogLimitConfig:
    def test_get_audit_log_limit_raises_when_unset(self, monkeypatch):
        """get_audit_log_limit() must raise ValueError when AUDIT_LOG_LIMIT is not set."""
        monkeypatch.delenv("AUDIT_LOG_LIMIT", raising=False)
        import importlib
        import backend.config as cfg
        importlib.reload(cfg)
        with pytest.raises(ValueError, match="AUDIT_LOG_LIMIT"):
            cfg.get_audit_log_limit()

    def test_get_audit_log_limit_returns_value(self, monkeypatch):
        """get_audit_log_limit() returns the configured integer."""
        monkeypatch.setenv("AUDIT_LOG_LIMIT", "500")
        import importlib
        import backend.config as cfg
        importlib.reload(cfg)
        assert cfg.get_audit_log_limit() == 500

    def test_license_page_shows_configured_email(self, client, auth_cookies, monkeypatch):
        """License page must use LICENSE_CONTACT_EMAIL, not a hardcoded literal."""
        monkeypatch.setenv("LICENSE_CONTACT_EMAIL", "support@example.com")
        r = client.get("/admin/license", cookies=auth_cookies)
        assert r.status_code == 200
        assert b"support@example.com" in r.content
