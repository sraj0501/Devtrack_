"""
Tests for TASK-077 — EOD report queue integration.

Four test classes as specified by the task acceptance criteria:

  1. /reports/eod stages an eod_report action (mock _queue_gateway.stage, assert called).
  2. _execute_pm_action with eod_report + non-empty email calls EmailReporter.send_text_report.
  3. _execute_pm_action with empty email returns {"status": "posted"} without raising.
  4. get_eod_report_confidence() returns 0.88 when EOD_REPORT_CONFIDENCE is unset.
"""
from __future__ import annotations

import json
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch, AsyncMock

import pytest

# Ensure the project root is on sys.path so backend.* imports work.
_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Module-level patches — mirrors the pattern in test_http_triggers.py
# ---------------------------------------------------------------------------

@pytest.fixture(scope="module", autouse=True)
def _patch_slow_startup():
    """Stub blocking lifespan calls for the whole test module.

    Starlette TestClient runs the app lifespan on the first request even when
    the client is not used as a context manager. Two things in the lifespan
    block in test environments:
      1. TriggerProcessor._init_components — loads spaCy, Azure SDKs, etc.
      2. _ensure_gitlab_webhooks          — makes outbound HTTP calls to GitLab.
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
    """FastAPI TestClient — no API key, no lifespan hang."""
    monkeypatch.delenv("DEVTRACK_API_KEY", raising=False)
    monkeypatch.setenv("DEVTRACK_TLS", "false")
    from fastapi.testclient import TestClient
    from backend.webhook_server import app
    return TestClient(app, raise_server_exceptions=True)


# ---------------------------------------------------------------------------
# Helper: build a bare TriggerProcessor for unit tests
# ---------------------------------------------------------------------------

def _bare_processor():
    from backend.webhook_server import TriggerProcessor
    proc = TriggerProcessor.__new__(TriggerProcessor)
    proc.nlp_parser = None
    proc.description_enhancer = None
    proc.azure_client = None
    proc.gitlab_client = None
    proc.github_client = None
    proc.workspace_router = MagicMock()
    proc.task_matcher = None
    proc._queue_gateway = None
    return proc


# ---------------------------------------------------------------------------
# 1. /reports/eod stages an eod_report queue action
# ---------------------------------------------------------------------------

class TestEODEndpointStagesAction:
    """/reports/eod must call queue_gateway.stage() with action_type='eod_report'."""

    def test_stage_called_with_eod_report_action_type(self, client):
        """When the gateway is available, stage() is called with action_type='eod_report'."""
        mock_stage = MagicMock(return_value=42)
        mock_gw = MagicMock()
        mock_gw.stage = mock_stage

        fake_narrative = "EOD Report — 2026-06-17\n\nPROJ-1: Fixed auth bug."

        with (
            patch("backend.webhook_server._get_queue_gateway", return_value=mock_gw),
            patch(
                "backend.webhook_server._daily_report_generator_available",
                True,
            ),
            patch(
                "backend.webhook_server._DailyReportGenerator"
            ) as mock_gen_cls,
        ):
            mock_gen = MagicMock()
            mock_gen.generate_eod_narrative.return_value = fake_narrative
            mock_gen_cls.return_value = mock_gen

            resp = client.post("/reports/eod", json={"email": "dev@example.com"})

        assert resp.status_code == 200
        data = resp.json()
        assert data["success"] is True
        assert data["action_id"] == 42

        # Verify stage() was called with the correct action_type
        mock_stage.assert_called_once()
        call_kwargs = mock_stage.call_args
        # stage() can be called positionally or via kwargs
        args, kwargs = call_kwargs
        all_kwargs = {**dict(zip(
            ["action_type", "target", "platform", "workspace", "payload",
             "confidence", "is_new_action_type"],
            args,
        )), **kwargs}
        assert all_kwargs.get("action_type") == "eod_report"
        assert all_kwargs.get("target") == "developer"
        assert all_kwargs.get("platform") == "email"
        payload = all_kwargs.get("payload", {})
        assert payload.get("narrative") == fake_narrative
        assert payload.get("email") == "dev@example.com"

    def test_stage_not_called_when_gateway_unavailable(self, client):
        """When _get_queue_gateway returns None, the endpoint still succeeds."""
        fake_narrative = "No commits recorded today."
        with (
            patch("backend.webhook_server._get_queue_gateway", return_value=None),
            patch(
                "backend.webhook_server._daily_report_generator_available",
                False,
            ),
        ):
            resp = client.post("/reports/eod", json={})

        assert resp.status_code == 200
        data = resp.json()
        assert data["success"] is True
        # action_id is None when gateway is unavailable
        assert data.get("action_id") is None

    def test_confidence_comes_from_config_accessor(self, client, monkeypatch):
        """Confidence passed to stage() comes from get_eod_report_confidence(), not a literal."""
        monkeypatch.setenv("EOD_REPORT_CONFIDENCE", "0.75")

        mock_gw = MagicMock()
        mock_gw.stage = MagicMock(return_value=99)

        with (
            patch("backend.webhook_server._get_queue_gateway", return_value=mock_gw),
            patch("backend.webhook_server._daily_report_generator_available", False),
        ):
            resp = client.post("/reports/eod", json={})

        assert resp.status_code == 200
        args, kwargs = mock_gw.stage.call_args
        all_kwargs = {**dict(zip(
            ["action_type", "target", "platform", "workspace", "payload",
             "confidence", "is_new_action_type"],
            args,
        )), **kwargs}
        # Should use 0.75 from env, not the literal default 0.88
        assert abs(all_kwargs.get("confidence", 0) - 0.75) < 0.001


# ---------------------------------------------------------------------------
# 2. _execute_pm_action with eod_report + non-empty email calls send_text_report
# ---------------------------------------------------------------------------

class TestExecutePMActionEODReport:
    """Unit tests for the eod_report branch of _execute_pm_action."""

    def _make_action(self, email: str, narrative: str = "EOD content") -> dict:
        """Build a pending_actions row dict for an eod_report action."""
        return {
            "id": 1,
            "action_type": "eod_report",
            "target": "developer",
            "platform": "email",
            "workspace": "all",
            "payload": json.dumps({
                "narrative": narrative,
                "email": email,
                "date": "2026-06-17",
            }),
        }

    def test_send_text_report_called_with_non_empty_email(self):
        """When email is non-empty, EmailReporter.send_text_report must be called."""
        proc = _bare_processor()
        action = self._make_action(email="dev@example.com", narrative="EOD narrative")

        mock_reporter = MagicMock()
        with patch(
            "backend.webhook_server.EmailReporter",
            return_value=mock_reporter,
            create=True,
        ):
            # The import inside _execute_pm_action is local; patch the module attribute
            with patch(
                "backend.webhook_server._execute_pm_action_eod_reporter",
                create=True,
            ):
                pass
            # Patch the import directly in the method
            import backend.email_reporter as _er_mod
            original_cls = _er_mod.EmailReporter
            _er_mod.EmailReporter = MagicMock(return_value=mock_reporter)
            try:
                result = proc._execute_pm_action(action)
            finally:
                _er_mod.EmailReporter = original_cls

        mock_reporter.send_text_report.assert_called_once_with("EOD narrative", "dev@example.com")
        assert result["status"] == "posted"
        assert result["delivered_to"] == "dev@example.com"

    def test_send_text_report_not_called_with_empty_email(self):
        """When email is empty, send_text_report must NOT be called — returns posted."""
        proc = _bare_processor()
        action = self._make_action(email="", narrative="EOD narrative")

        import backend.email_reporter as _er_mod
        original_cls = _er_mod.EmailReporter
        mock_reporter = MagicMock()
        _er_mod.EmailReporter = MagicMock(return_value=mock_reporter)
        try:
            result = proc._execute_pm_action(action)
        finally:
            _er_mod.EmailReporter = original_cls

        mock_reporter.send_text_report.assert_not_called()
        assert result["status"] == "posted"
        assert result["delivered_to"] == "none"

    def test_eod_report_action_never_raises(self):
        """Even if EmailReporter.send_text_report raises, _execute_pm_action returns posted."""
        proc = _bare_processor()
        action = self._make_action(email="dev@example.com")

        import backend.email_reporter as _er_mod
        original_cls = _er_mod.EmailReporter
        mock_reporter = MagicMock()
        mock_reporter.send_text_report.side_effect = RuntimeError("SMTP connection refused")
        _er_mod.EmailReporter = MagicMock(return_value=mock_reporter)
        try:
            # Must not raise
            result = proc._execute_pm_action(action)
        finally:
            _er_mod.EmailReporter = original_cls

        # Returns posted regardless (Non-Negotiable #8)
        assert result["status"] == "posted"


# ---------------------------------------------------------------------------
# 3. _execute_pm_action with empty email returns {"status": "posted"} without raising
# ---------------------------------------------------------------------------

class TestExecutePMActionEmptyEmail:
    """Verify the empty-email path explicitly."""

    def test_empty_email_returns_posted_without_sending(self):
        """Empty email: result contains status=posted, delivered_to=none, no exception."""
        proc = _bare_processor()
        action = {
            "id": 7,
            "action_type": "eod_report",
            "target": "developer",
            "platform": "email",
            "workspace": "all",
            "payload": json.dumps({
                "narrative": "Some narrative",
                "email": "",
                "date": "2026-06-17",
            }),
        }
        result = proc._execute_pm_action(action)
        assert result == {"status": "posted", "delivered_to": "none"}

    def test_missing_email_key_returns_posted(self):
        """Payload without 'email' key behaves same as empty email."""
        proc = _bare_processor()
        action = {
            "id": 8,
            "action_type": "eod_report",
            "target": "developer",
            "platform": "email",
            "workspace": "all",
            "payload": json.dumps({
                "narrative": "Some narrative",
                "date": "2026-06-17",
            }),
        }
        result = proc._execute_pm_action(action)
        assert result["status"] == "posted"
        assert result["delivered_to"] == "none"


# ---------------------------------------------------------------------------
# 4. get_eod_report_confidence() returns 0.88 when EOD_REPORT_CONFIDENCE unset
# ---------------------------------------------------------------------------

class TestGetEODReportConfidence:
    """Unit tests for the config.get_eod_report_confidence() accessor."""

    def test_default_value_is_0_88(self, monkeypatch):
        """Returns 0.88 when EOD_REPORT_CONFIDENCE is not set."""
        monkeypatch.delenv("EOD_REPORT_CONFIDENCE", raising=False)
        from backend.config import get_eod_report_confidence
        result = get_eod_report_confidence()
        assert abs(result - 0.88) < 1e-9, f"Expected 0.88, got {result}"

    def test_custom_value_from_env(self, monkeypatch):
        """Returns parsed float when EOD_REPORT_CONFIDENCE is set."""
        monkeypatch.setenv("EOD_REPORT_CONFIDENCE", "0.95")
        from backend.config import get_eod_report_confidence
        result = get_eod_report_confidence()
        assert abs(result - 0.95) < 1e-9, f"Expected 0.95, got {result}"

    def test_returns_float_type(self, monkeypatch):
        """Return type is float, not str."""
        monkeypatch.delenv("EOD_REPORT_CONFIDENCE", raising=False)
        from backend.config import get_eod_report_confidence
        result = get_eod_report_confidence()
        assert isinstance(result, float)
