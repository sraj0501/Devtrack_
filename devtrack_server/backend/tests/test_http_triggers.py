"""
CS-1 integration tests: HTTP trigger endpoints in webhook_server.py.

Tests cover:
  - /trigger/commit   — process_commit path
  - /trigger/timer    — process_timer path
  - /trigger/ping     — liveness
  - /trigger/shutdown — graceful stop signal
  - /health           — health check
  - Auth middleware   — X-DevTrack-API-Key enforcement
  - TriggerProcessor  — unit tests with mocked components
"""
from __future__ import annotations

import os
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch, AsyncMock

import pytest

# Ensure project root is on sys.path
_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Module-level patches — applied once before any test in this module imports
# the webhook_server app.  Patches _init_components so the lazy singleton
# creation inside request handlers is instantaneous (no spaCy / Azure init).
# We do NOT use TestClient as a context manager because the lifespan calls
# asyncio.to_thread(TriggerProcessor.get) which hangs in a test event loop.
# ---------------------------------------------------------------------------

@pytest.fixture(scope="module", autouse=True)
def _patch_slow_startup():
    """Stub blocking lifespan calls for the whole test module.

    Starlette TestClient runs the app lifespan on the first request even when
    the client is not used as a context manager. Two things in the lifespan
    block in test environments:

      1. TriggerProcessor._init_components — loads spaCy, Azure SDKs, etc.
      2. _ensure_gitlab_webhooks        — makes outbound HTTP calls to GitLab.

    Both are patched here so any test request returns immediately.
    """
    noop = AsyncMock(return_value=None)
    with (
        patch("backend.webhook_server.TriggerProcessor._init_components"),
        patch("backend.webhook_server._ensure_gitlab_webhooks", new=noop),
    ):
        yield


@pytest.fixture(autouse=True)
def clear_trigger_processor():
    """Reset TriggerProcessor singleton between tests."""
    from backend.webhook_server import TriggerProcessor
    TriggerProcessor._instance = None
    yield
    TriggerProcessor._instance = None


@pytest.fixture()
def client(monkeypatch):
    """FastAPI TestClient — no lifespan (avoids asyncio.to_thread hang)."""
    monkeypatch.delenv("DEVTRACK_API_KEY", raising=False)
    monkeypatch.setenv("DEVTRACK_TLS", "false")
    from fastapi.testclient import TestClient
    from backend.webhook_server import app
    # Do NOT use as context manager — that triggers the lifespan.
    return TestClient(app, raise_server_exceptions=True)


@pytest.fixture()
def client_with_key(monkeypatch):
    """FastAPI TestClient with DEVTRACK_API_KEY=test-key — no lifespan."""
    monkeypatch.setenv("DEVTRACK_API_KEY", "test-key")
    monkeypatch.setenv("DEVTRACK_TLS", "false")
    from fastapi.testclient import TestClient
    from backend.webhook_server import app
    return TestClient(app, raise_server_exceptions=True)


# ---------------------------------------------------------------------------
# Helper: build a bare TriggerProcessor with all components set to None
# ---------------------------------------------------------------------------

def _bare_processor():
    from backend.webhook_server import TriggerProcessor
    proc = TriggerProcessor.__new__(TriggerProcessor)
    proc.nlp_parser = None
    proc.description_enhancer = None
    proc.azure_client = None
    proc.gitlab_client = None
    proc.github_client = None
    proc.workspace_router = None
    proc.task_matcher = None
    return proc


# ---------------------------------------------------------------------------
# /health
# ---------------------------------------------------------------------------

class TestHealthEndpoint:
    def test_health_returns_200(self, client):
        resp = client.get("/health")
        assert resp.status_code == 200

    def test_health_response_has_status(self, client):
        data = client.get("/health").json()
        assert "status" in data


# ---------------------------------------------------------------------------
# Auth middleware
# ---------------------------------------------------------------------------

class TestAuthMiddleware:
    def test_no_key_required_when_env_unset(self, client):
        """When DEVTRACK_API_KEY is not set, any request is accepted."""
        resp = client.post("/trigger/ping", json={})
        assert resp.status_code == 200

    def test_missing_key_rejected_when_env_set(self, client_with_key):
        resp = client_with_key.post("/trigger/ping", json={})
        assert resp.status_code == 403

    def test_wrong_key_rejected(self, client_with_key):
        resp = client_with_key.post(
            "/trigger/ping", json={},
            headers={"X-DevTrack-API-Key": "wrong-key"},
        )
        assert resp.status_code == 403

    def test_correct_key_accepted(self, client_with_key):
        resp = client_with_key.post(
            "/trigger/ping", json={},
            headers={"X-DevTrack-API-Key": "test-key"},
        )
        assert resp.status_code == 200


# ---------------------------------------------------------------------------
# /trigger/ping
# ---------------------------------------------------------------------------

class TestPingEndpoint:
    def test_ping_returns_pong(self, client):
        data = client.post("/trigger/ping", json={}).json()
        assert data.get("pong") is True
        assert data.get("status") == "ok"


# ---------------------------------------------------------------------------
# /trigger/commit
# ---------------------------------------------------------------------------

COMMIT_PAYLOAD = {
    "commit_hash": "abc123def456",
    "commit_message": "fix: resolve login timeout issue",
    "repo_path": "/tmp/repo",
    "author": "dev@example.com",
    "branch": "main",
    "pm_platform": "",
    "pm_project": "",
}


class TestCommitTriggerEndpoint:
    def _mock_process_commit(self, actions=None):
        from backend.webhook_server import TriggerProcessor
        return patch.object(
            TriggerProcessor,
            "process_commit",
            return_value={"actions": actions or [], "commit_hash": "abc123def456"},
        )

    def test_commit_returns_200(self, client):
        with self._mock_process_commit():
            resp = client.post("/trigger/commit", json=COMMIT_PAYLOAD)
        assert resp.status_code == 200

    def test_commit_response_has_status_ok(self, client):
        with self._mock_process_commit(actions=["pm_sync:github"]):
            data = client.post("/trigger/commit", json=COMMIT_PAYLOAD).json()
        assert data.get("status") == "ok"

    def test_commit_response_includes_actions(self, client):
        with self._mock_process_commit(actions=["pm_sync:github"]):
            data = client.post("/trigger/commit", json=COMMIT_PAYLOAD).json()
        assert "actions" in data

    def test_commit_accepts_minimal_payload(self, client):
        with self._mock_process_commit():
            resp = client.post("/trigger/commit", json={"commit_hash": "deadbeef"})
        assert resp.status_code == 200


# ---------------------------------------------------------------------------
# /trigger/timer
# ---------------------------------------------------------------------------

TIMER_PAYLOAD = {
    "interval_mins": 60,
    "trigger_count": 3,
    "pm_platform": "github",
    "workspace_name": "my-project",
}


class TestTimerTriggerEndpoint:
    def _mock_process_timer(self, status="accepted"):
        from backend.webhook_server import TriggerProcessor
        return patch.object(
            TriggerProcessor,
            "process_timer",
            return_value={
                "status": status,
                "trigger_count": 3,
                "prompt_channel": "none",
                "active_session": False,
            },
        )

    def test_timer_returns_200(self, client):
        with self._mock_process_timer():
            resp = client.post("/trigger/timer", json=TIMER_PAYLOAD)
        assert resp.status_code == 200

    def test_timer_response_has_status(self, client):
        with self._mock_process_timer():
            data = client.post("/trigger/timer", json=TIMER_PAYLOAD).json()
        assert data.get("status") in ("accepted", "vacation_auto")

    def test_timer_accepts_minimal_payload(self, client):
        with self._mock_process_timer():
            resp = client.post("/trigger/timer", json={})
        assert resp.status_code == 200


# ---------------------------------------------------------------------------
# /trigger/workspace_reload
# ---------------------------------------------------------------------------

class TestWorkspaceReloadEndpoint:
    def test_reload_returns_ok(self, client):
        resp = client.post("/trigger/workspace_reload", json={})
        assert resp.status_code == 200
        assert resp.json().get("status") == "ok"


# ---------------------------------------------------------------------------
# /trigger/work_session_start and /trigger/work_session_stop
# ---------------------------------------------------------------------------

class TestWorkSessionEndpoints:
    def test_session_start_returns_ok(self, client):
        resp = client.post(
            "/trigger/work_session_start",
            json={"session_id": 42, "ticket_ref": "GH-123"},
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data.get("status") == "ok"
        assert data.get("session_id") == 42

    def test_session_stop_returns_ok(self, client):
        resp = client.post(
            "/trigger/work_session_stop",
            json={"session_id": 42},
        )
        assert resp.status_code == 200
        assert resp.json().get("status") == "ok"


# ---------------------------------------------------------------------------
# TriggerProcessor.process_commit — unit tests
# ---------------------------------------------------------------------------

class TestTriggerProcessorCommit:
    def test_returns_dict_with_actions(self):
        proc = _bare_processor()
        result = proc.process_commit(COMMIT_PAYLOAD)
        assert isinstance(result, dict)
        assert "actions" in result
        assert "commit_hash" in result

    def test_includes_commit_hash(self):
        proc = _bare_processor()
        result = proc.process_commit({"commit_hash": "cafebabe1234", "commit_message": "chore: cleanup"})
        assert result["commit_hash"] == "cafebabe1234"

    def test_empty_payload_does_not_raise(self):
        proc = _bare_processor()
        result = proc.process_commit({})
        assert "actions" in result

    def test_calls_workspace_router_when_nlp_parses(self):
        proc = _bare_processor()
        mock_parser = MagicMock()
        mock_parser.parse.return_value = {
            "description": "fix login",
            "ticket_id": "GH-1",
            "status": "done",
        }
        proc.nlp_parser = mock_parser
        mock_router = MagicMock()
        proc.workspace_router = mock_router

        proc.process_commit(COMMIT_PAYLOAD)

        mock_router.route.assert_called_once()

    def test_skips_pm_sync_when_nlp_returns_none(self):
        proc = _bare_processor()
        mock_parser = MagicMock()
        mock_parser.parse.return_value = None
        proc.nlp_parser = mock_parser
        mock_router = MagicMock()
        proc.workspace_router = mock_router

        result = proc.process_commit(COMMIT_PAYLOAD)

        mock_router.route.assert_not_called()
        assert not any("pm_sync" in a for a in result["actions"])

    def test_skips_pm_sync_when_no_workspace_router(self):
        proc = _bare_processor()
        mock_parser = MagicMock()
        mock_parser.parse.return_value = {"description": "fix", "ticket_id": "X-1", "status": "done"}
        proc.nlp_parser = mock_parser
        # workspace_router stays None

        result = proc.process_commit(COMMIT_PAYLOAD)
        assert not any("pm_sync" in a for a in result["actions"])


# ---------------------------------------------------------------------------
# TriggerProcessor.process_timer — unit tests
# ---------------------------------------------------------------------------

class TestTriggerProcessorTimer:
    def test_returns_accepted_status(self):
        proc = _bare_processor()
        result = proc.process_timer(TIMER_PAYLOAD)
        assert result.get("status") in ("accepted", "vacation_auto")

    def test_includes_trigger_count(self):
        proc = _bare_processor()
        result = proc.process_timer({"trigger_count": 7})
        assert result.get("trigger_count") == 7

    def test_empty_payload_does_not_raise(self):
        proc = _bare_processor()
        result = proc.process_timer({})
        assert "status" in result

    def test_prompt_channel_none_when_no_integrations(self):
        proc = _bare_processor()
        with patch.dict("sys.modules", {
            "backend.telegram": None,
            "backend.telegram.notifier": None,
            "backend.slack": None,
            "backend.slack.notifier": None,
        }):
            result = proc.process_timer(TIMER_PAYLOAD)
        assert result.get("prompt_channel") == "none"

    def test_active_session_false_when_store_unavailable(self):
        proc = _bare_processor()
        with patch("backend.work_tracker.session_store.WorkSessionStore") as mock_store:
            mock_store.side_effect = Exception("db unavailable")
            result = proc.process_timer(TIMER_PAYLOAD)
        # Should not raise; active_session defaults to False
        assert result.get("active_session") is False
