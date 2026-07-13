"""
Tests for ReviewClassifier (backend/review_classifier.py) and the
POST /review/classify endpoint in webhook_server.py.

Tests cover:
  1. Auto-fixable path: mocked LLM returns well-formed JSON with
     classification="auto_fixable" → classify() returns correct dict.
  2. LLM failure fallback: LLM raises → classify() returns needs_human
     without raising.
  3. POST /review/classify: 200 with valid body; 401/403 when API key
     is set but missing/wrong.
"""
from __future__ import annotations

import json
import os
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

# Ensure project root is on sys.path
_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Unit tests — ReviewClassifier
# ---------------------------------------------------------------------------

class TestReviewClassifierAutoFixable:
    """Mocked LLM returns auto_fixable JSON — classify() should pass it through."""

    def _make_classifier_with_mock_provider(self, llm_response: str):
        from backend.review_classifier import ReviewClassifier
        mock_provider = MagicMock()
        mock_provider.complete.return_value = llm_response
        return ReviewClassifier(provider=mock_provider)

    def test_returns_dict_with_all_three_keys(self):
        llm_json = json.dumps({
            "classification": "auto_fixable",
            "reason": "Formatting issue — trailing whitespace.",
            "fix_hint": "Remove trailing whitespace on line 42.",
        })
        classifier = self._make_classifier_with_mock_provider(llm_json)
        result = classifier.classify(
            comment_body="Remove trailing whitespace",
            pr_title="feat: add user authentication",
            platform="github",
        )
        assert "classification" in result
        assert "reason" in result
        assert "fix_hint" in result

    def test_classification_is_auto_fixable(self):
        llm_json = json.dumps({
            "classification": "auto_fixable",
            "reason": "Naming convention violation.",
            "fix_hint": "Rename variable 'x' to 'userID'.",
        })
        classifier = self._make_classifier_with_mock_provider(llm_json)
        result = classifier.classify(
            comment_body="Please rename x to userID per our naming conventions.",
            pr_title="fix: auth token handling",
            platform="github",
        )
        assert result["classification"] == "auto_fixable"
        assert result["fix_hint"] != ""

    def test_needs_human_classification(self):
        llm_json = json.dumps({
            "classification": "needs_human",
            "reason": "Architectural concern about coupling.",
            "fix_hint": "",
        })
        classifier = self._make_classifier_with_mock_provider(llm_json)
        result = classifier.classify(
            comment_body="This module is doing too many things, consider SRP.",
            pr_title="refactor: modularise auth",
            platform="github",
        )
        assert result["classification"] == "needs_human"
        assert result["fix_hint"] == ""

    def test_strips_markdown_code_fences(self):
        """LLM sometimes wraps JSON in code fences."""
        llm_json = "```json\n" + json.dumps({
            "classification": "auto_fixable",
            "reason": "Import ordering.",
            "fix_hint": "Sort imports alphabetically.",
        }) + "\n```"
        classifier = self._make_classifier_with_mock_provider(llm_json)
        result = classifier.classify("Sort your imports", "PR", "github")
        assert result["classification"] == "auto_fixable"


class TestReviewClassifierLLMFailure:
    """LLM raises or returns garbage → classify() falls back to needs_human."""

    def test_llm_raises_returns_needs_human(self):
        from backend.review_classifier import ReviewClassifier
        mock_provider = MagicMock()
        mock_provider.complete.side_effect = RuntimeError("Ollama is down")
        classifier = ReviewClassifier(provider=mock_provider)

        result = classifier.classify("Fix the indentation", "PR title", "github")

        assert result["classification"] == "needs_human"
        assert result["reason"] == "LLM unavailable."
        assert result["fix_hint"] == ""

    def test_llm_raises_does_not_raise(self):
        """classify() must never propagate the LLM exception to the caller."""
        from backend.review_classifier import ReviewClassifier
        mock_provider = MagicMock()
        mock_provider.complete.side_effect = Exception("network timeout")
        classifier = ReviewClassifier(provider=mock_provider)

        # Should not raise
        result = classifier.classify("any comment", "any PR", "github")
        assert isinstance(result, dict)

    def test_invalid_json_returns_needs_human(self):
        from backend.review_classifier import ReviewClassifier
        mock_provider = MagicMock()
        mock_provider.complete.return_value = "this is not json"
        classifier = ReviewClassifier(provider=mock_provider)

        result = classifier.classify("some comment", "some PR", "github")
        assert result["classification"] == "needs_human"

    def test_empty_llm_response_returns_needs_human(self):
        from backend.review_classifier import ReviewClassifier
        mock_provider = MagicMock()
        mock_provider.complete.return_value = ""
        classifier = ReviewClassifier(provider=mock_provider)

        result = classifier.classify("some comment", "some PR", "github")
        assert result["classification"] == "needs_human"

    def test_unknown_classification_value_defaults_to_needs_human(self):
        from backend.review_classifier import ReviewClassifier
        mock_provider = MagicMock()
        mock_provider.complete.return_value = json.dumps({
            "classification": "maybe_fixable",
            "reason": "Not sure",
            "fix_hint": "",
        })
        classifier = ReviewClassifier(provider=mock_provider)

        result = classifier.classify("some comment", "some PR", "github")
        assert result["classification"] == "needs_human"


# ---------------------------------------------------------------------------
# Integration tests — POST /review/classify endpoint
# ---------------------------------------------------------------------------

@pytest.fixture(scope="module", autouse=True)
def _patch_slow_startup_for_review():
    """Stub blocking lifespan calls (same pattern as test_http_triggers.py)."""
    from unittest.mock import AsyncMock
    noop = AsyncMock(return_value=None)
    with (
        patch("backend.webhook_server.TriggerProcessor._init_components"),
        patch("backend.webhook_server._ensure_gitlab_webhooks", new=noop),
    ):
        yield


@pytest.fixture(autouse=True)
def _reset_trigger_processor():
    from backend.webhook_server import TriggerProcessor
    TriggerProcessor._instance = None
    yield
    TriggerProcessor._instance = None


@pytest.fixture()
def client(monkeypatch):
    """FastAPI TestClient — no API key set (dev mode: all requests pass auth)."""
    monkeypatch.delenv("DEVTRACK_API_KEY", raising=False)
    monkeypatch.setenv("DEVTRACK_TLS", "false")
    from fastapi.testclient import TestClient
    from backend.webhook_server import app
    return TestClient(app, raise_server_exceptions=True)


@pytest.fixture()
def client_with_key(monkeypatch):
    """FastAPI TestClient with DEVTRACK_API_KEY=test-key set."""
    monkeypatch.setenv("DEVTRACK_API_KEY", "test-key")
    monkeypatch.setenv("DEVTRACK_TLS", "false")
    from fastapi.testclient import TestClient
    from backend.webhook_server import app
    return TestClient(app, raise_server_exceptions=True)


class TestReviewClassifyEndpoint:
    """POST /review/classify endpoint tests."""

    def test_returns_200_with_valid_body(self, client):
        """Endpoint returns 200 with a classification response."""
        mock_result = {
            "classification": "auto_fixable",
            "reason": "Formatting issue.",
            "fix_hint": "Run gofmt.",
        }
        with patch("backend.review_classifier.ReviewClassifier.classify", return_value=mock_result):
            resp = client.post("/review/classify", json={
                "comment_body": "Please run gofmt on this file.",
                "pr_title": "feat: add pagination",
                "platform": "github",
            })
        assert resp.status_code == 200
        data = resp.json()
        assert "classification" in data
        assert "reason" in data
        assert "fix_hint" in data

    def test_endpoint_auth_guard_rejects_wrong_key(self, client_with_key):
        """When DEVTRACK_API_KEY is set, wrong key returns 403."""
        resp = client_with_key.post(
            "/review/classify",
            json={"comment_body": "x", "pr_title": "y", "platform": "github"},
            headers={"X-DevTrack-API-Key": "wrong-key"},
        )
        assert resp.status_code == 403

    def test_endpoint_auth_guard_accepts_correct_key(self, client_with_key):
        """Correct API key is accepted."""
        mock_result = {
            "classification": "needs_human",
            "reason": "Architecture concern.",
            "fix_hint": "",
        }
        with patch("backend.review_classifier.ReviewClassifier.classify", return_value=mock_result):
            resp = client_with_key.post(
                "/review/classify",
                json={"comment_body": "rethink this design", "pr_title": "PR", "platform": "github"},
                headers={"X-DevTrack-API-Key": "test-key"},
            )
        assert resp.status_code == 200

    def test_endpoint_no_auth_required_when_key_not_set(self, client):
        """When DEVTRACK_API_KEY is not set, no header needed."""
        mock_result = {
            "classification": "needs_human",
            "reason": "No LLM.",
            "fix_hint": "",
        }
        with patch("backend.review_classifier.ReviewClassifier.classify", return_value=mock_result):
            resp = client.post("/review/classify", json={
                "comment_body": "check this logic",
                "pr_title": "PR",
                "platform": "github",
            })
        assert resp.status_code == 200

    def test_endpoint_missing_key_returns_403(self, client_with_key):
        """When DEVTRACK_API_KEY is set, missing header returns 403."""
        resp = client_with_key.post(
            "/review/classify",
            json={"comment_body": "x", "pr_title": "y", "platform": "github"},
            # No X-DevTrack-API-Key header
        )
        assert resp.status_code == 403
