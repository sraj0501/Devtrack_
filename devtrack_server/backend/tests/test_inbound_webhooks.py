"""
Integration tests for inbound webhook endpoints in backend/webhook_server.py.

Covers the four /webhooks/* routes that receive external events:
  - /webhooks/azure-devops  — Basic auth, 4 Azure event types
  - /webhooks/github        — HMAC-SHA256 signature, placeholder handler
  - /webhooks/gitlab        — X-Gitlab-Token, 3 event types + unknown
  - /webhooks/jira          — No auth, placeholder handler

Patterns follow backend/tests/test_http_triggers.py:
  - Module-scoped _patch_slow_startup stubs the lifespan (no spaCy / GitLab calls)
  - Function-scoped fixture resets the cached WebhookEventHandler singleton
  - TestClient NOT used as context manager (avoids asyncio.to_thread hang)
  - WebhookNotifier.notify and _send_ipc_event are patched to AsyncMock
    so no OS notifications or IPC calls happen during tests
"""
from __future__ import annotations

import base64
import hashlib
import hmac
import sys
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest

# Ensure project root is on sys.path
_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Module-level patches — applied before any import of webhook_server.app
# ---------------------------------------------------------------------------

@pytest.fixture(scope="module", autouse=True)
def _patch_slow_startup():
    """Stub blocking lifespan calls for the entire test module.

    Starlette TestClient triggers the app lifespan on the first request.
    Two things block in test environments:
      1. TriggerProcessor._init_components — loads spaCy, Azure SDKs, etc.
      2. _ensure_gitlab_webhooks        — makes outbound HTTP calls to GitLab.
    Both are patched so any test request returns immediately.
    """
    noop = AsyncMock(return_value=None)
    with (
        patch("backend.webhook_server.TriggerProcessor._init_components"),
        patch("backend.webhook_server._ensure_gitlab_webhooks", new=noop),
    ):
        yield


@pytest.fixture(autouse=True)
def _reset_handler():
    """Reset the cached WebhookEventHandler singleton between tests."""
    import backend.webhook_server as ws
    ws._handler = None
    yield
    ws._handler = None


@pytest.fixture()
def client(monkeypatch):
    """TestClient with no auth env vars set (dev/open mode for all sources).

    WebhookNotifier.notify is patched to a no-op AsyncMock so no osascript
    calls happen.  _send_ipc_event is also patched to prevent IPC import.
    """
    monkeypatch.delenv("WEBHOOK_AZURE_USERNAME", raising=False)
    monkeypatch.delenv("WEBHOOK_AZURE_PASSWORD", raising=False)
    monkeypatch.delenv("WEBHOOK_GITHUB_SECRET", raising=False)
    monkeypatch.delenv("WEBHOOK_GITLAB_SECRET", raising=False)
    monkeypatch.setenv("DEVTRACK_TLS", "false")

    with (
        patch(
            "backend.webhook_handlers.WebhookEventHandler._send_ipc_event",
            new=AsyncMock(return_value=None),
        ),
        patch(
            "backend.webhook_notifier.WebhookNotifier.notify",
            new=AsyncMock(return_value=None),
        ),
    ):
        from fastapi.testclient import TestClient
        from backend.webhook_server import app
        yield TestClient(app, raise_server_exceptions=True)


# ---------------------------------------------------------------------------
# Auth fixture helpers
# ---------------------------------------------------------------------------

def _basic_auth_header(username: str, password: str) -> str:
    creds = base64.b64encode(f"{username}:{password}".encode()).decode()
    return f"Basic {creds}"


def _github_sig(secret: str, body: bytes) -> str:
    """Compute the correct X-Hub-Signature-256 header value."""
    digest = hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
    return f"sha256={digest}"


# ---------------------------------------------------------------------------
# Reusable payloads
# ---------------------------------------------------------------------------

_AZURE_WORK_ITEM_UPDATED = {
    "eventType": "workitem.updated",
    "resource": {
        "workItemId": 101,
        "revisedBy": {"displayName": "Alice"},
        "fields": {
            "System.State": {"oldValue": "Active", "newValue": "Resolved"},
            "System.AssignedTo": {"oldValue": "Bob", "newValue": "Alice"},
        },
    },
}

_AZURE_WORK_ITEM_COMMENTED = {
    "eventType": "workitem.commented",
    "resource": {
        "workItemId": 202,
        "revisedBy": {"displayName": "Bob"},
        "comment": "Looks good, deploying now.",
    },
}

_AZURE_WORK_ITEM_CREATED = {
    "eventType": "workitem.created",
    "resource": {
        "id": 303,
        "fields": {
            "System.Title": "Add login page",
            "System.AssignedTo": {"displayName": "Carol"},
        },
    },
}

_AZURE_WORK_ITEM_DELETED = {
    "eventType": "workitem.deleted",
    "resource": {"id": 404},
}

_GITLAB_ISSUE_HOOK = {
    "object_attributes": {
        "iid": 7,
        "title": "Fix navbar overflow",
        "state": "opened",
        "action": "open",
        "url": "https://gitlab.example.com/proj/issues/7",
    },
    "project": {"name": "my-app"},
    "assignees": [{"name": "Dev User"}],
}

_GITLAB_MR_HOOK = {
    "object_attributes": {
        "iid": 12,
        "title": "Feature: dark mode",
        "state": "opened",
        "action": "open",
        "target_branch": "main",
    },
    "project": {"name": "my-app"},
    "assignees": [{"name": "Dev User"}],
}

_GITLAB_NOTE_HOOK = {
    "object_attributes": {"note": "LGTM, merging."},
    "user": {"name": "Reviewer"},
    "noteable_type": "MergeRequest",
}

_JIRA_ISSUE_UPDATED = {
    "webhookEvent": "jira:issue_updated",
    "issue": {"id": "10001", "key": "PROJ-42"},
}


# ===========================================================================
# Group 1 — /webhooks/azure-devops
# ===========================================================================

class TestAzureWebhookAuth:
    """Basic auth enforcement on /webhooks/azure-devops."""

    def test_no_auth_configured_allows_request(self, client):
        """When WEBHOOK_AZURE_USERNAME/PASSWORD are unset, all requests pass."""
        resp = client.post("/webhooks/azure-devops", json=_AZURE_WORK_ITEM_UPDATED)
        assert resp.status_code == 200

    def test_missing_auth_header_rejected_when_configured(self, client, monkeypatch):
        monkeypatch.setenv("WEBHOOK_AZURE_USERNAME", "svcuser")
        monkeypatch.setenv("WEBHOOK_AZURE_PASSWORD", "s3cr3t")
        resp = client.post("/webhooks/azure-devops", json=_AZURE_WORK_ITEM_UPDATED)
        assert resp.status_code == 401

    def test_wrong_credentials_rejected(self, client, monkeypatch):
        monkeypatch.setenv("WEBHOOK_AZURE_USERNAME", "svcuser")
        monkeypatch.setenv("WEBHOOK_AZURE_PASSWORD", "s3cr3t")
        resp = client.post(
            "/webhooks/azure-devops",
            json=_AZURE_WORK_ITEM_UPDATED,
            headers={"Authorization": _basic_auth_header("svcuser", "wrongpass")},
        )
        assert resp.status_code == 403

    def test_correct_credentials_accepted(self, client, monkeypatch):
        monkeypatch.setenv("WEBHOOK_AZURE_USERNAME", "svcuser")
        monkeypatch.setenv("WEBHOOK_AZURE_PASSWORD", "s3cr3t")
        resp = client.post(
            "/webhooks/azure-devops",
            json=_AZURE_WORK_ITEM_UPDATED,
            headers={"Authorization": _basic_auth_header("svcuser", "s3cr3t")},
        )
        assert resp.status_code == 200

    def test_malformed_auth_header_rejected(self, client, monkeypatch):
        monkeypatch.setenv("WEBHOOK_AZURE_USERNAME", "svcuser")
        monkeypatch.setenv("WEBHOOK_AZURE_PASSWORD", "s3cr3t")
        resp = client.post(
            "/webhooks/azure-devops",
            json=_AZURE_WORK_ITEM_UPDATED,
            headers={"Authorization": "Basic not-valid-base64!!!"},
        )
        assert resp.status_code == 401


class TestAzureWebhookPayloads:
    """Event type routing and response shapes for Azure DevOps webhooks."""

    def test_workitem_updated_returns_processed(self, client):
        resp = client.post("/webhooks/azure-devops", json=_AZURE_WORK_ITEM_UPDATED)
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "processed"

    def test_workitem_updated_includes_work_item_id(self, client):
        resp = client.post("/webhooks/azure-devops", json=_AZURE_WORK_ITEM_UPDATED)
        assert resp.json()["work_item_id"] == 101

    def test_workitem_updated_changes_count_matches_fields(self, client):
        resp = client.post("/webhooks/azure-devops", json=_AZURE_WORK_ITEM_UPDATED)
        # The payload has 2 fields in the fields dict
        assert resp.json()["changes"] == 2

    def test_workitem_commented_returns_processed(self, client):
        resp = client.post("/webhooks/azure-devops", json=_AZURE_WORK_ITEM_COMMENTED)
        assert resp.status_code == 200
        assert resp.json()["status"] == "processed"

    def test_workitem_commented_includes_work_item_id(self, client):
        resp = client.post("/webhooks/azure-devops", json=_AZURE_WORK_ITEM_COMMENTED)
        assert resp.json()["work_item_id"] == 202

    def test_workitem_created_returns_processed(self, client):
        resp = client.post("/webhooks/azure-devops", json=_AZURE_WORK_ITEM_CREATED)
        assert resp.status_code == 200
        assert resp.json()["status"] == "processed"

    def test_workitem_created_includes_work_item_id(self, client):
        resp = client.post("/webhooks/azure-devops", json=_AZURE_WORK_ITEM_CREATED)
        assert resp.json()["work_item_id"] == 303

    def test_workitem_deleted_returns_processed(self, client):
        resp = client.post("/webhooks/azure-devops", json=_AZURE_WORK_ITEM_DELETED)
        assert resp.status_code == 200
        assert resp.json()["status"] == "processed"

    def test_unknown_event_type_returns_ignored(self, client):
        payload = {"eventType": "build.completed", "resource": {}}
        resp = client.post("/webhooks/azure-devops", json=payload)
        assert resp.status_code == 200
        assert resp.json()["status"] == "ignored"

    def test_missing_event_type_returns_400(self, client):
        resp = client.post("/webhooks/azure-devops", json={"resource": {}})
        assert resp.status_code == 400

    def test_invalid_json_returns_400(self, client):
        resp = client.post(
            "/webhooks/azure-devops",
            content=b"not-json",
            headers={"Content-Type": "application/json"},
        )
        assert resp.status_code == 400


class TestAzureHandlerUnit:
    """Unit-level tests that call WebhookEventHandler directly (no HTTP)."""

    @pytest.fixture()
    def handler(self):
        from backend.webhook_handlers import WebhookEventHandler
        notifier = AsyncMock()
        h = WebhookEventHandler(ipc_client=None, notifier=notifier)
        return h

    @pytest.mark.asyncio
    async def test_workitem_updated_calls_notifier(self, handler):
        resource = {
            "workItemId": 11,
            "revisedBy": {"displayName": "Dev"},
            "fields": {"System.State": {"oldValue": "New", "newValue": "Active"}},
        }
        with patch.object(handler, "_send_ipc_event", new=AsyncMock()):
            await handler.handle_azure_event("workitem.updated", resource, {})
        handler.notifier.notify.assert_called_once()

    @pytest.mark.asyncio
    async def test_workitem_commented_returns_correct_work_item_id(self, handler):
        resource = {
            "workItemId": 55,
            "revisedBy": {"displayName": "Tester"},
            "comment": "Looks good",
        }
        with patch.object(handler, "_send_ipc_event", new=AsyncMock()):
            result = await handler.handle_azure_event("workitem.commented", resource, {})
        assert result["work_item_id"] == 55

    @pytest.mark.asyncio
    async def test_unknown_azure_event_type_returns_ignored(self, handler):
        result = await handler.handle_azure_event("unknown.event", {}, {})
        assert result["status"] == "ignored"
        assert "unhandled event type" in result["reason"]


# ===========================================================================
# Group 2 — /webhooks/github
# ===========================================================================

class TestGitHubWebhookAuth:
    """HMAC-SHA256 signature enforcement on /webhooks/github."""

    def test_no_secret_configured_allows_request(self, client):
        """When WEBHOOK_GITHUB_SECRET is unset, all requests pass."""
        resp = client.post(
            "/webhooks/github",
            json={"action": "opened"},
            headers={"X-GitHub-Event": "pull_request"},
        )
        assert resp.status_code == 200

    def test_missing_signature_rejected_when_secret_configured(self, client, monkeypatch):
        monkeypatch.setenv("WEBHOOK_GITHUB_SECRET", "mysecret")
        resp = client.post(
            "/webhooks/github",
            json={"action": "opened"},
            headers={"X-GitHub-Event": "pull_request"},
        )
        assert resp.status_code == 401

    def test_wrong_signature_rejected(self, client, monkeypatch):
        monkeypatch.setenv("WEBHOOK_GITHUB_SECRET", "mysecret")
        resp = client.post(
            "/webhooks/github",
            content=b'{"action":"opened"}',
            headers={
                "Content-Type": "application/json",
                "X-GitHub-Event": "pull_request",
                "X-Hub-Signature-256": "sha256=deadbeef",
            },
        )
        assert resp.status_code == 403

    def test_correct_signature_accepted(self, client, monkeypatch):
        monkeypatch.setenv("WEBHOOK_GITHUB_SECRET", "mysecret")
        body = b'{"action":"opened"}'
        sig = _github_sig("mysecret", body)
        resp = client.post(
            "/webhooks/github",
            content=body,
            headers={
                "Content-Type": "application/json",
                "X-GitHub-Event": "push",
                "X-Hub-Signature-256": sig,
            },
        )
        assert resp.status_code == 200

    def test_signature_prefix_missing_rejected(self, client, monkeypatch):
        """Signature without 'sha256=' prefix is rejected."""
        monkeypatch.setenv("WEBHOOK_GITHUB_SECRET", "mysecret")
        body = b'{"action":"opened"}'
        raw_digest = hmac.new(b"mysecret", body, hashlib.sha256).hexdigest()
        resp = client.post(
            "/webhooks/github",
            content=body,
            headers={
                "Content-Type": "application/json",
                "X-GitHub-Event": "push",
                "X-Hub-Signature-256": raw_digest,  # missing "sha256=" prefix
            },
        )
        assert resp.status_code == 401


class TestGitHubWebhookPayloads:
    """Payload handling for GitHub webhooks."""

    def test_push_event_returns_200(self, client):
        resp = client.post(
            "/webhooks/github",
            json={"ref": "refs/heads/main", "commits": []},
            headers={"X-GitHub-Event": "push"},
        )
        assert resp.status_code == 200

    def test_github_placeholder_returns_ignored(self, client):
        """GitHub handler is a placeholder — current correct response is ignored."""
        data = client.post(
            "/webhooks/github",
            json={"action": "opened"},
            headers={"X-GitHub-Event": "issues"},
        ).json()
        assert data.get("status") == "ignored"
        assert "reason" in data

    def test_invalid_json_returns_400(self, client):
        resp = client.post(
            "/webhooks/github",
            content=b"not-json",
            headers={"Content-Type": "application/json"},
        )
        assert resp.status_code == 400

    def test_event_type_from_header(self, client):
        """The X-GitHub-Event header is read; unknown types still return 200."""
        resp = client.post(
            "/webhooks/github",
            json={"zen": "Keep it logically awesome."},
            headers={"X-GitHub-Event": "ping"},
        )
        assert resp.status_code == 200


# ===========================================================================
# Group 3 — /webhooks/gitlab
# ===========================================================================

class TestGitLabWebhookAuth:
    """X-Gitlab-Token enforcement on /webhooks/gitlab."""

    def test_no_secret_configured_allows_request(self, client):
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_ISSUE_HOOK,
            headers={"X-Gitlab-Event": "Issue Hook"},
        )
        assert resp.status_code == 200

    def test_missing_token_rejected_when_configured(self, client, monkeypatch):
        monkeypatch.setenv("WEBHOOK_GITLAB_SECRET", "gl-secret")
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_ISSUE_HOOK,
            headers={"X-Gitlab-Event": "Issue Hook"},
        )
        assert resp.status_code == 401

    def test_wrong_token_rejected(self, client, monkeypatch):
        monkeypatch.setenv("WEBHOOK_GITLAB_SECRET", "gl-secret")
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_ISSUE_HOOK,
            headers={
                "X-Gitlab-Event": "Issue Hook",
                "X-Gitlab-Token": "wrong-token",
            },
        )
        assert resp.status_code == 401

    def test_correct_token_accepted(self, client, monkeypatch):
        monkeypatch.setenv("WEBHOOK_GITLAB_SECRET", "gl-secret")
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_ISSUE_HOOK,
            headers={
                "X-Gitlab-Event": "Issue Hook",
                "X-Gitlab-Token": "gl-secret",
            },
        )
        assert resp.status_code == 200


class TestGitLabWebhookPayloads:
    """Event type routing and response shapes for GitLab webhooks."""

    def test_issue_hook_returns_handled(self, client):
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_ISSUE_HOOK,
            headers={"X-Gitlab-Event": "Issue Hook"},
        )
        assert resp.status_code == 200
        assert resp.json()["handled"] is True

    def test_issue_hook_event_name_in_response(self, client):
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_ISSUE_HOOK,
            headers={"X-Gitlab-Event": "Issue Hook"},
        )
        assert resp.json()["event"] == "Issue Hook"

    def test_issue_hook_action_in_response(self, client):
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_ISSUE_HOOK,
            headers={"X-Gitlab-Event": "Issue Hook"},
        )
        assert resp.json()["action"] == "open"

    def test_merge_request_hook_returns_handled(self, client):
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_MR_HOOK,
            headers={"X-Gitlab-Event": "Merge Request Hook"},
        )
        assert resp.status_code == 200
        assert resp.json()["handled"] is True

    def test_merge_request_hook_event_name_in_response(self, client):
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_MR_HOOK,
            headers={"X-Gitlab-Event": "Merge Request Hook"},
        )
        assert resp.json()["event"] == "Merge Request Hook"

    def test_note_hook_returns_handled(self, client):
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_NOTE_HOOK,
            headers={"X-Gitlab-Event": "Note Hook"},
        )
        assert resp.status_code == 200
        assert resp.json()["handled"] is True

    def test_note_hook_event_name_in_response(self, client):
        resp = client.post(
            "/webhooks/gitlab",
            json=_GITLAB_NOTE_HOOK,
            headers={"X-Gitlab-Event": "Note Hook"},
        )
        assert resp.json()["event"] == "Note Hook"

    def test_unknown_event_type_returns_not_handled(self, client):
        resp = client.post(
            "/webhooks/gitlab",
            json={"object_attributes": {}},
            headers={"X-Gitlab-Event": "Pipeline Hook"},
        )
        assert resp.status_code == 200
        assert resp.json()["handled"] is False

    def test_missing_event_header_uses_unknown(self, client):
        """No X-Gitlab-Event header — defaults to 'unknown' → handled=False."""
        resp = client.post("/webhooks/gitlab", json={"object_attributes": {}})
        assert resp.status_code == 200
        assert resp.json()["handled"] is False

    def test_invalid_json_returns_400(self, client):
        resp = client.post(
            "/webhooks/gitlab",
            content=b"not-json",
            headers={"Content-Type": "application/json"},
        )
        assert resp.status_code == 400


class TestGitLabHandlerUnit:
    """Unit tests for GitLab handler methods called directly."""

    @pytest.fixture()
    def handler(self):
        from backend.webhook_handlers import WebhookEventHandler
        notifier = AsyncMock()
        h = WebhookEventHandler(ipc_client=None, notifier=notifier)
        return h

    @pytest.mark.asyncio
    async def test_issue_hook_calls_notifier(self, handler):
        with patch.object(handler, "_send_ipc_event", new=AsyncMock()):
            await handler.handle_gitlab_event("Issue Hook", _GITLAB_ISSUE_HOOK)
        handler.notifier.notify.assert_called_once()

    @pytest.mark.asyncio
    async def test_merge_request_hook_calls_notifier(self, handler):
        with patch.object(handler, "_send_ipc_event", new=AsyncMock()):
            await handler.handle_gitlab_event("Merge Request Hook", _GITLAB_MR_HOOK)
        handler.notifier.notify.assert_called_once()

    @pytest.mark.asyncio
    async def test_note_hook_calls_notifier(self, handler):
        with patch.object(handler, "_send_ipc_event", new=AsyncMock()):
            await handler.handle_gitlab_event("Note Hook", _GITLAB_NOTE_HOOK)
        handler.notifier.notify.assert_called_once()

    @pytest.mark.asyncio
    async def test_unknown_event_does_not_call_notifier(self, handler):
        with patch.object(handler, "_send_ipc_event", new=AsyncMock()):
            await handler.handle_gitlab_event("Unknown Hook", {})
        handler.notifier.notify.assert_not_called()


# ===========================================================================
# Group 4 — /webhooks/jira
# ===========================================================================

class TestJiraWebhookPayloads:
    """Payload handling for Jira webhooks (unauthenticated; placeholder handler)."""

    def test_issue_updated_returns_200(self, client):
        resp = client.post("/webhooks/jira", json=_JIRA_ISSUE_UPDATED)
        assert resp.status_code == 200

    def test_issue_updated_returns_ignored(self, client):
        """Jira handler is a placeholder — current correct response is ignored."""
        data = client.post("/webhooks/jira", json=_JIRA_ISSUE_UPDATED).json()
        assert data.get("status") == "ignored"

    def test_missing_webhook_event_defaults_to_unknown(self, client):
        """When webhookEvent key is absent, event_type is 'unknown' — still 200."""
        resp = client.post("/webhooks/jira", json={"issue": {"key": "PROJ-1"}})
        assert resp.status_code == 200
        assert resp.json().get("status") == "ignored"

    def test_reason_key_present_in_response(self, client):
        data = client.post("/webhooks/jira", json=_JIRA_ISSUE_UPDATED).json()
        assert "reason" in data

    def test_invalid_json_returns_400(self, client):
        resp = client.post(
            "/webhooks/jira",
            content=b"not-json",
            headers={"Content-Type": "application/json"},
        )
        assert resp.status_code == 400
