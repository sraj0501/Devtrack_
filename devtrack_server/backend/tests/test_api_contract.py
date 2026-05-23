"""
HTTP API contract tests — devtrack_server side.

Validates that the server accepts the request shapes and returns the response
shapes defined in docs/HTTP_API.md.

These tests use FastAPI's TestClient (Starlette). They do NOT import any Go code.
All assertions are on the Python side of the HTTP boundary only.

# TODO: extend with /trigger/commit shape test (TASK-046 or later)
# TODO: extend with /trigger/timer shape test
# TODO: extend with /trigger/boardroom shape test
"""
from __future__ import annotations

import sys
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest

# Ensure project root is on sys.path so backend.* imports resolve.
_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Module-level patches applied before any import of the FastAPI app.
# These mirror the pattern established in test_http_triggers.py to avoid
# blocking lifespan calls (spaCy, Azure SDK init, GitLab HTTP calls).
# ---------------------------------------------------------------------------

@pytest.fixture(scope="module", autouse=True)
def _patch_slow_startup():
    """Stub blocking lifespan calls so TestClient requests return immediately."""
    noop = AsyncMock(return_value=None)
    with (
        patch("backend.webhook_server.TriggerProcessor._init_components"),
        patch("backend.webhook_server._ensure_gitlab_webhooks", new=noop),
    ):
        yield


@pytest.fixture(autouse=True)
def _reset_trigger_processor():
    """Reset TriggerProcessor singleton between tests."""
    from backend.webhook_server import TriggerProcessor
    TriggerProcessor._instance = None
    yield
    TriggerProcessor._instance = None


@pytest.fixture()
def client(monkeypatch):
    """FastAPI TestClient — no API key required (dev mode)."""
    monkeypatch.delenv("DEVTRACK_API_KEY", raising=False)
    monkeypatch.setenv("DEVTRACK_TLS", "false")
    from fastapi.testclient import TestClient
    from backend.webhook_server import app
    # Do NOT use as context manager — triggers lifespan which hangs in tests.
    return TestClient(app, raise_server_exceptions=True)


# ---------------------------------------------------------------------------
# GET /health
# Spec: docs/HTTP_API.md § GET /health
# Expected: HTTP 200, body has {"status": "ok", "service": "devtrack-webhooks"}
# ---------------------------------------------------------------------------

class TestHealthContract:
    def test_returns_200(self, client):
        resp = client.get("/health")
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}"

    def test_body_has_status_ok(self, client):
        data = client.get("/health").json()
        assert "status" in data, "Response missing 'status' field"
        assert data["status"] == "ok", f"Expected status='ok', got {data['status']!r}"

    def test_body_has_service_field(self, client):
        data = client.get("/health").json()
        assert "service" in data, "Response missing 'service' field"

    def test_service_is_devtrack_webhooks(self, client):
        data = client.get("/health").json()
        assert data["service"] == "devtrack-webhooks"


# ---------------------------------------------------------------------------
# GET /version
# Spec: docs/HTTP_API.md § GET /version
# Expected: HTTP 200, body has {"version": "...", "service": "devtrack-webhooks"}
# ---------------------------------------------------------------------------

class TestVersionContract:
    def test_returns_200(self, client):
        resp = client.get("/version")
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}"

    def test_body_has_version_field(self, client):
        data = client.get("/version").json()
        assert "version" in data, "Response missing 'version' field"

    def test_version_is_string(self, client):
        data = client.get("/version").json()
        assert isinstance(data["version"], str), "Expected 'version' to be a string"

    def test_body_has_service_field(self, client):
        data = client.get("/version").json()
        assert "service" in data, "Response missing 'service' field"


# ---------------------------------------------------------------------------
# GET /status
# Spec: docs/HTTP_API.md § GET /status
# Expected: HTTP 200, body has service + boolean feature flags
# ---------------------------------------------------------------------------

class TestStatusContract:
    def test_returns_200(self, client):
        resp = client.get("/status")
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}"

    def test_body_has_service(self, client):
        data = client.get("/status").json()
        assert "service" in data

    def test_body_has_boolean_flags(self, client):
        data = client.get("/status").json()
        for flag in ("azure_devops", "webhook_enabled", "notify_os", "notify_terminal"):
            assert flag in data, f"Response missing '{flag}' field"
            assert isinstance(data[flag], bool), f"Expected '{flag}' to be bool"


# ---------------------------------------------------------------------------
# POST /trigger/ping
# Spec: docs/HTTP_API.md § POST /trigger/ping
# Expected: HTTP 200, body {"status": "ok", "pong": true}
# ---------------------------------------------------------------------------

class TestPingContract:
    def test_returns_200(self, client):
        resp = client.post("/trigger/ping", json={})
        assert resp.status_code == 200

    def test_body_status_ok(self, client):
        data = client.post("/trigger/ping", json={}).json()
        assert data.get("status") == "ok"

    def test_body_pong_true(self, client):
        data = client.post("/trigger/ping", json={}).json()
        assert data.get("pong") is True


# ---------------------------------------------------------------------------
# POST /trigger/commit
# Spec: docs/HTTP_API.md § POST /trigger/commit
# Expected: HTTP 200, body {"status": "ok", "actions": [...], "commit_hash": "..."}
# ---------------------------------------------------------------------------

# Canonical example request from docs/HTTP_API.md
COMMIT_EXAMPLE = {
    "commit_hash":    "abc123def456",
    "commit_message": "fix: resolve login timeout issue",
    "repo_path":      "/home/user/myproject",
    "author":         "dev@example.com",
    "timestamp":      "2026-05-24T14:00:00Z",
    "files_changed":  ["backend/auth.py", "backend/config.py"],
    "branch":         "fix/login-timeout",
    "workspace_name": "myproject",
    "pm_platform":    "github",
    "pm_project":     "myorg/myproject",
}


class TestCommitTriggerContract:
    def test_returns_200(self, client):
        with patch("backend.webhook_server.TriggerProcessor.process_commit",
                   return_value={"actions": [], "commit_hash": "abc123def456"}):
            resp = client.post("/trigger/commit", json=COMMIT_EXAMPLE)
        assert resp.status_code == 200

    def test_response_has_status_ok(self, client):
        with patch("backend.webhook_server.TriggerProcessor.process_commit",
                   return_value={"actions": [], "commit_hash": "abc123def456"}):
            data = client.post("/trigger/commit", json=COMMIT_EXAMPLE).json()
        assert data.get("status") == "ok"

    def test_response_has_actions_list(self, client):
        with patch("backend.webhook_server.TriggerProcessor.process_commit",
                   return_value={"actions": ["pm_sync:github"], "commit_hash": "abc123def456"}):
            data = client.post("/trigger/commit", json=COMMIT_EXAMPLE).json()
        assert "actions" in data
        assert isinstance(data["actions"], list)

    def test_response_has_commit_hash(self, client):
        with patch("backend.webhook_server.TriggerProcessor.process_commit",
                   return_value={"actions": [], "commit_hash": "abc123def456"}):
            data = client.post("/trigger/commit", json=COMMIT_EXAMPLE).json()
        assert "commit_hash" in data

    def test_minimal_payload_accepted(self, client):
        """Contract allows omitting all optional fields; only commit_hash is used."""
        with patch("backend.webhook_server.TriggerProcessor.process_commit",
                   return_value={"actions": [], "commit_hash": "deadbeef"}):
            resp = client.post("/trigger/commit", json={"commit_hash": "deadbeef"})
        assert resp.status_code == 200


# ---------------------------------------------------------------------------
# POST /trigger/timer
# Spec: docs/HTTP_API.md § POST /trigger/timer
# Expected: HTTP 200, body has "status" and "trigger_count"
# ---------------------------------------------------------------------------

TIMER_EXAMPLE = {
    "timestamp":      "2026-05-24T14:00:00Z",
    "interval_mins":  60,
    "trigger_count":  3,
    "workspace_name": "myproject",
    "pm_platform":    "github",
}


class TestTimerTriggerContract:
    def _mock_timer(self):
        return patch("backend.webhook_server.TriggerProcessor.process_timer",
                     return_value={
                         "status":         "accepted",
                         "trigger_count":  3,
                         "prompt_channel": "none",
                         "active_session": False,
                     })

    def test_returns_200(self, client):
        with self._mock_timer():
            resp = client.post("/trigger/timer", json=TIMER_EXAMPLE)
        assert resp.status_code == 200

    def test_response_has_status(self, client):
        with self._mock_timer():
            data = client.post("/trigger/timer", json=TIMER_EXAMPLE).json()
        assert data.get("status") in ("accepted", "vacation_auto")

    def test_response_has_trigger_count(self, client):
        with self._mock_timer():
            data = client.post("/trigger/timer", json=TIMER_EXAMPLE).json()
        assert "trigger_count" in data

    def test_minimal_payload_accepted(self, client):
        """Contract allows empty body."""
        with self._mock_timer():
            resp = client.post("/trigger/timer", json={})
        assert resp.status_code == 200


# ---------------------------------------------------------------------------
# POST /trigger/workspace_reload
# Spec: docs/HTTP_API.md § POST /trigger/workspace_reload
# Expected: HTTP 200, body {"status": "ok", "message": "..."}
# ---------------------------------------------------------------------------

class TestWorkspaceReloadContract:
    def test_returns_200(self, client):
        resp = client.post("/trigger/workspace_reload", json={"source": "cli"})
        assert resp.status_code == 200

    def test_response_status_ok(self, client):
        data = client.post("/trigger/workspace_reload", json={"source": "cli"}).json()
        assert data.get("status") == "ok"

    def test_response_has_message(self, client):
        data = client.post("/trigger/workspace_reload", json={"source": "cli"}).json()
        assert "message" in data


# ---------------------------------------------------------------------------
# POST /trigger/work_session_start
# Spec: docs/HTTP_API.md § POST /trigger/work_session_start
# Expected: HTTP 200, body {"status": "ok", "session_id": <int>}
# ---------------------------------------------------------------------------

class TestWorkSessionStartContract:
    def test_returns_200(self, client):
        resp = client.post("/trigger/work_session_start",
                           json={"session_id": 42, "ticket_ref": "GH-123"})
        assert resp.status_code == 200

    def test_response_status_ok(self, client):
        data = client.post("/trigger/work_session_start",
                           json={"session_id": 42, "ticket_ref": "GH-123"}).json()
        assert data.get("status") == "ok"

    def test_response_echoes_session_id(self, client):
        data = client.post("/trigger/work_session_start",
                           json={"session_id": 42}).json()
        assert data.get("session_id") == 42


# ---------------------------------------------------------------------------
# POST /trigger/work_session_stop
# Spec: docs/HTTP_API.md § POST /trigger/work_session_stop
# Expected: HTTP 200, body {"status": "ok", "session_id": <int>}
# ---------------------------------------------------------------------------

class TestWorkSessionStopContract:
    def test_returns_200(self, client):
        resp = client.post("/trigger/work_session_stop", json={"session_id": 42})
        assert resp.status_code == 200

    def test_response_status_ok(self, client):
        data = client.post("/trigger/work_session_stop", json={"session_id": 42}).json()
        assert data.get("status") == "ok"
