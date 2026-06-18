"""
Tests for DialecticReasoner and the POST /dialectic/infer endpoint.

Coverage:
  1. DialecticReasoner.reason() returns [] when LLM is mocked to raise.
  2. DialecticReasoner.reason() returns a list of dicts with correct keys
     when a mock LLM returns well-formed JSON.
  3. POST /dialectic/infer with valid body returns 200 and {"inferences": [...]}.
  4. POST /dialectic/infer without auth header returns 401 when API key is set.
"""
from __future__ import annotations

import json
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch, AsyncMock

import pytest

_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Fixtures for the webhook_server app (reuse the pattern from test_http_triggers.py)
# ---------------------------------------------------------------------------

@pytest.fixture(scope="module", autouse=True)
def _patch_slow_startup():
    """Stub blocking lifespan calls for the whole test module."""
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
    """FastAPI TestClient — no DEVTRACK_API_KEY set (open auth mode)."""
    monkeypatch.delenv("DEVTRACK_API_KEY", raising=False)
    monkeypatch.setenv("DEVTRACK_TLS", "false")
    from fastapi.testclient import TestClient
    from backend.webhook_server import app
    return TestClient(app, raise_server_exceptions=True)


@pytest.fixture()
def client_with_key(monkeypatch):
    """FastAPI TestClient with DEVTRACK_API_KEY=test-key enforced."""
    monkeypatch.setenv("DEVTRACK_API_KEY", "test-key")
    monkeypatch.setenv("DEVTRACK_TLS", "false")
    from fastapi.testclient import TestClient
    from backend.webhook_server import app
    return TestClient(app, raise_server_exceptions=True)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_VALID_BODY = {
    "interaction_type": "approval",
    "context_type": "comment",
    "before_text": "Fixed the null pointer issue in login flow.",
    "after_text": "Fixed the null pointer issue in login flow.",
    "metadata": {"action_id": 42, "workspace": "myproject"},
}

_GOOD_JSON_RESPONSE = json.dumps({
    "inferences": [
        {
            "subject": "commit tone",
            "inference": "Developer uses short, imperative sentences.",
            "confidence": 0.80,
        },
        {
            "subject": "ticket referencing",
            "inference": "Developer consistently references ticket IDs in messages.",
            "confidence": 0.72,
        },
    ]
})


# ---------------------------------------------------------------------------
# Unit tests: DialecticReasoner
# ---------------------------------------------------------------------------

class TestDialecticReasoner:
    """Unit-level tests that do not touch the HTTP layer."""

    def test_returns_empty_list_when_llm_raises(self, monkeypatch):
        """reason() must return [] and never raise when the LLM fails."""
        from backend.dialectic_reasoner import DialecticReasoner

        reasoner = DialecticReasoner()

        # Both Hermes3 path and fallback path raise.
        monkeypatch.setattr(reasoner, "_call_hermes3", lambda _: (_ for _ in ()).throw(RuntimeError("ollama down")))
        monkeypatch.setattr(reasoner, "_call_fallback", lambda _: (_ for _ in ()).throw(RuntimeError("fallback down")))

        result = reasoner.reason(
            interaction_type="commit",
            context_type="commit",
            before_text="feat: add retry logic",
            after_text="feat: add retry logic",
            metadata={},
        )
        assert result == [], "reason() must return [] on LLM failure, never raise"

    def test_returns_empty_list_when_both_llms_return_none(self, monkeypatch):
        """reason() returns [] when both Hermes3 and fallback return None."""
        from backend.dialectic_reasoner import DialecticReasoner

        reasoner = DialecticReasoner()
        monkeypatch.setattr(reasoner, "_call_hermes3", lambda _: None)
        monkeypatch.setattr(reasoner, "_call_fallback", lambda _: None)

        result = reasoner.reason(
            interaction_type="approval",
            context_type="comment",
            before_text="",
            after_text="Good fix.",
            metadata={},
        )
        assert result == []

    def test_well_formed_json_returns_correct_structure(self, monkeypatch):
        """With a mock LLM returning well-formed JSON, reason() returns a list of dicts."""
        from backend.dialectic_reasoner import DialecticReasoner

        reasoner = DialecticReasoner()
        # Mock Hermes3 to return our good JSON.
        monkeypatch.setattr(reasoner, "_call_hermes3", lambda _: _GOOD_JSON_RESPONSE)

        result = reasoner.reason(
            interaction_type="approval",
            context_type="comment",
            before_text="Fixed null check.",
            after_text="Fixed null check.",
            metadata={"action_id": 1},
        )

        assert isinstance(result, list)
        assert len(result) == 2

        for item in result:
            assert "subject" in item, f"Missing 'subject' key: {item}"
            assert "inference" in item, f"Missing 'inference' key: {item}"
            assert "confidence" in item, f"Missing 'confidence' key: {item}"
            assert isinstance(item["subject"], str) and item["subject"]
            assert isinstance(item["inference"], str) and item["inference"]
            assert 0.0 <= item["confidence"] <= 1.0

    def test_fallback_used_when_hermes3_unavailable(self, monkeypatch):
        """Falls back to configured LLM chain when Hermes3 returns None."""
        from backend.dialectic_reasoner import DialecticReasoner

        reasoner = DialecticReasoner()
        monkeypatch.setattr(reasoner, "_call_hermes3", lambda _: None)  # unavailable
        monkeypatch.setattr(reasoner, "_call_fallback", lambda _: _GOOD_JSON_RESPONSE)

        result = reasoner.reason(
            interaction_type="rejection",
            context_type="task",
            before_text="Wrong tone.",
            after_text="",
            metadata={},
        )
        assert len(result) == 2

    def test_confidence_clamped_to_valid_range(self, monkeypatch):
        """Confidence values outside [0.0, 1.0] are clamped, not rejected."""
        from backend.dialectic_reasoner import DialecticReasoner

        bad_conf_json = json.dumps({
            "inferences": [
                {"subject": "style", "inference": "Likes short commits.", "confidence": 1.5},
                {"subject": "length", "inference": "Prefers brief messages.", "confidence": -0.3},
            ]
        })

        reasoner = DialecticReasoner()
        monkeypatch.setattr(reasoner, "_call_hermes3", lambda _: bad_conf_json)

        result = reasoner.reason("commit", "commit", "msg", "msg", {})
        assert all(0.0 <= r["confidence"] <= 1.0 for r in result)

    def test_malformed_json_returns_empty(self, monkeypatch):
        """Malformed JSON from LLM returns [] gracefully."""
        from backend.dialectic_reasoner import DialecticReasoner

        reasoner = DialecticReasoner()
        monkeypatch.setattr(reasoner, "_call_hermes3", lambda _: "not json at all")
        monkeypatch.setattr(reasoner, "_call_fallback", lambda _: None)

        result = reasoner.reason("commit", "commit", "", "", {})
        assert result == []

    def test_no_os_getenv_in_module(self):
        """Verify dialectic_reasoner.py uses backend.config, not os.getenv directly."""
        import ast, inspect
        from backend import dialectic_reasoner
        source = inspect.getsource(dialectic_reasoner)
        # os.getenv must not appear in the module source.
        assert "os.getenv" not in source, (
            "dialectic_reasoner.py must not call os.getenv — use backend.config"
        )


# ---------------------------------------------------------------------------
# Integration tests: POST /dialectic/infer endpoint
# ---------------------------------------------------------------------------

class TestDialecticInferEndpoint:
    """HTTP-layer tests for POST /dialectic/infer."""

    def test_valid_body_returns_200_with_inferences(self, client, monkeypatch):
        """Valid body returns 200 and {"inferences": [...]}."""
        # Patch DialecticReasoner.reason to return a known list.
        fake_inferences = [
            {"subject": "tone", "inference": "Uses imperative mood.", "confidence": 0.8}
        ]
        with patch("backend.dialectic_reasoner.DialecticReasoner.reason", return_value=fake_inferences):
            resp = client.post("/dialectic/infer", json=_VALID_BODY)

        assert resp.status_code == 200
        data = resp.json()
        assert "inferences" in data
        assert data["inferences"] == fake_inferences

    def test_llm_failure_returns_200_with_empty_list(self, client, monkeypatch):
        """When LLM fails, endpoint returns 200 with {"inferences": []} (not an error)."""
        with patch("backend.dialectic_reasoner.DialecticReasoner.reason", return_value=[]):
            resp = client.post("/dialectic/infer", json=_VALID_BODY)

        assert resp.status_code == 200
        data = resp.json()
        assert data == {"inferences": []}

    def test_missing_auth_returns_401_when_key_configured(self, client_with_key):
        """Without X-DevTrack-API-Key the endpoint returns 401/403 when DEVTRACK_API_KEY is set."""
        with patch("backend.dialectic_reasoner.DialecticReasoner.reason", return_value=[]):
            resp = client_with_key.post("/dialectic/infer", json=_VALID_BODY)

        # The existing _verify_trigger_key returns 403 for wrong/missing key.
        assert resp.status_code in (401, 403)

    def test_valid_auth_returns_200(self, client_with_key):
        """With the correct X-DevTrack-API-Key the endpoint returns 200."""
        fake_inferences = [{"subject": "s", "inference": "i", "confidence": 0.5}]
        with patch("backend.dialectic_reasoner.DialecticReasoner.reason", return_value=fake_inferences):
            resp = client_with_key.post(
                "/dialectic/infer",
                json=_VALID_BODY,
                headers={"X-DevTrack-API-Key": "test-key"},
            )

        assert resp.status_code == 200
        assert resp.json()["inferences"] == fake_inferences

    def test_invalid_json_body_returns_400(self, client):
        """Non-JSON body returns HTTP 400."""
        resp = client.post(
            "/dialectic/infer",
            content=b"not json",
            headers={"Content-Type": "application/json"},
        )
        assert resp.status_code == 400

    def test_empty_body_returns_200_with_empty_inferences(self, client):
        """Empty but valid JSON body ({}) returns 200 (all fields optional)."""
        with patch("backend.dialectic_reasoner.DialecticReasoner.reason", return_value=[]):
            resp = client.post("/dialectic/infer", json={})
        assert resp.status_code == 200
        assert resp.json() == {"inferences": []}
