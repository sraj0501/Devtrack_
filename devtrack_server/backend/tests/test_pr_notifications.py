"""
TASK-096 tests: _execute_pm_action handling for pr_approved_notify and pr_escalation.

Verifies that:
  - pr_approved_notify returns {"status": "posted"} without calling any PM API
  - pr_escalation returns {"status": "posted"} without calling any PM API
  - Neither action type raises an exception under any payload shape
"""
from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _bare_processor():
    """Build a TriggerProcessor with no real components wired in."""
    from backend.webhook_server import TriggerProcessor
    proc = TriggerProcessor.__new__(TriggerProcessor)
    proc.llm_task_parser = None
    proc.description_enhancer = None
    proc.azure_client = None
    proc.gitlab_client = None
    proc.github_client = None
    proc.workspace_router = None
    proc.task_matcher = None
    return proc


def _action(action_type: str, payload: dict | None = None) -> dict:
    """Build a minimal pending_actions dict for _execute_pm_action."""
    return {
        "id": 999,
        "action_type": action_type,
        "target": "github:PR #42",
        "platform": "github",
        "workspace": "test-ws",
        "payload": json.dumps(payload or {}),
    }


# ---------------------------------------------------------------------------
# pr_approved_notify
# ---------------------------------------------------------------------------

class TestPRApprovedNotify:
    def test_returns_posted(self):
        """pr_approved_notify should return {"status": "posted"}."""
        proc = _bare_processor()
        result = proc._execute_pm_action(
            _action("pr_approved_notify", {"pr_title": "feat: add login", "pr_id": "42", "fixes_applied": 2})
        )
        assert result == {"status": "posted"}

    def test_no_workspace_router_required(self):
        """pr_approved_notify must NOT call workspace_router (it is None here)."""
        proc = _bare_processor()
        proc.workspace_router = None
        # If this raises AttributeError / TypeError, the router was called.
        result = proc._execute_pm_action(_action("pr_approved_notify"))
        assert result["status"] == "posted"

    def test_empty_payload_does_not_raise(self):
        """An empty payload dict should be handled gracefully."""
        proc = _bare_processor()
        result = proc._execute_pm_action(_action("pr_approved_notify", {}))
        assert result["status"] == "posted"

    def test_missing_payload_field_does_not_raise(self):
        """payload with no pr_title should not raise."""
        proc = _bare_processor()
        result = proc._execute_pm_action(_action("pr_approved_notify", {"fixes_applied": 0}))
        assert result["status"] == "posted"


# ---------------------------------------------------------------------------
# pr_escalation
# ---------------------------------------------------------------------------

class TestPREscalation:
    def test_returns_posted(self):
        """pr_escalation should return {"status": "posted"}."""
        proc = _bare_processor()
        result = proc._execute_pm_action(
            _action("pr_escalation", {
                "pr_title": "feat: add login",
                "pr_id": "19",
                "blocker_reason": "architecture question",
                "comment_url": "https://github.com/org/repo/pull/19#discussion_r1",
                "fixes_applied": 1,
            })
        )
        assert result == {"status": "posted"}

    def test_no_workspace_router_required(self):
        """pr_escalation must NOT call workspace_router (notification-only)."""
        proc = _bare_processor()
        proc.workspace_router = None
        result = proc._execute_pm_action(_action("pr_escalation"))
        assert result["status"] == "posted"

    def test_empty_payload_does_not_raise(self):
        proc = _bare_processor()
        result = proc._execute_pm_action(_action("pr_escalation", {}))
        assert result["status"] == "posted"

    def test_missing_blocker_does_not_raise(self):
        """Missing blocker_reason should be handled with a graceful empty string."""
        proc = _bare_processor()
        result = proc._execute_pm_action(_action("pr_escalation", {"pr_id": "19"}))
        assert result["status"] == "posted"
