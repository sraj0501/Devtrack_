"""
Tests for TASK-073: state_transition queue action staging and execution.

Covers:
  1. state_transition is staged when: resolved_ticket_id + is_first_commit_for_ticket=True
     + known platform with a non-empty in-progress state mapping
  2. state_transition is NOT staged when: is_first_commit_for_ticket=False
  3. state_transition is NOT staged when: unknown/unsupported platform (empty state string)
  4. state_transition is NOT staged when: ticket unresolved (no ticket_id)
  5. _execute_pm_action routes state_transition to workspace_router.route() with mapped
     status and no comment text required
  6. _execute_pm_action with unknown action_type logs warning and marks complete
"""
from __future__ import annotations

import sys
from pathlib import Path
from unittest.mock import MagicMock, patch, call

import pytest

_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

COMMIT_PAYLOAD_BASE = {
    "commit_hash": "deadbeef1234",
    "commit_message": "feat: add user auth",
    "repo_path": "/repo/myapp",
    "author": "dev@example.com",
    "branch": "feat/PROJ-42-auth",
    "pm_platform": "azure",
    "pm_project": "",
    "workspace_name": "myapp",
}


def _bare_processor():
    """Build a TriggerProcessor with all components set to None."""
    from backend.webhook_server import TriggerProcessor
    proc = TriggerProcessor.__new__(TriggerProcessor)
    proc.nlp_parser = None
    proc.description_enhancer = None
    proc.azure_client = None
    proc.gitlab_client = None
    proc.github_client = None
    proc.workspace_router = None
    proc.task_matcher = None
    proc._queue_gateway = None
    return proc


def _processor_with_gateway(stage_side_effect=None):
    """Return a processor with a workspace_router and a mock queue gateway."""
    proc = _bare_processor()
    proc.workspace_router = MagicMock()

    mock_gateway = MagicMock()
    # By default stage() returns incrementing IDs: first call → 1, second → 2
    if stage_side_effect is not None:
        mock_gateway.stage.side_effect = stage_side_effect
    else:
        mock_gateway.stage.side_effect = [1, 2, 3, 4]

    proc._queue_gateway = mock_gateway
    return proc, mock_gateway


# ---------------------------------------------------------------------------
# 1. state_transition staged on first commit for a known platform
# ---------------------------------------------------------------------------

class TestStateTransitionStaged:
    """When is_first_commit_for_ticket=True and platform has a known mapping,
    a second stage() call with action_type='state_transition' must be made."""

    def test_two_stage_calls_on_first_azure_commit(self):
        proc, mock_gw = _processor_with_gateway()

        payload = {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "PROJ-42",
            "is_first_commit_for_ticket": True,
            "pm_platform": "azure",
        }
        result = proc.process_commit(payload)

        assert mock_gw.stage.call_count == 2, (
            f"Expected 2 stage() calls (post_comment + state_transition), "
            f"got {mock_gw.stage.call_count}"
        )

        # First call must be post_comment
        _, first_kwargs = mock_gw.stage.call_args_list[0]
        assert first_kwargs["action_type"] == "post_comment"
        assert first_kwargs["target"] == "PROJ-42"
        assert first_kwargs["confidence"] == 0.85

        # Second call must be state_transition
        _, second_kwargs = mock_gw.stage.call_args_list[1]
        assert second_kwargs["action_type"] == "state_transition"
        assert second_kwargs["target"] == "PROJ-42"
        assert second_kwargs["confidence"] == 0.90
        assert second_kwargs["payload"]["new_state"] == "Active"  # Azure
        assert second_kwargs["payload"]["ticket_id"] == "PROJ-42"

    def test_state_transition_in_actions_list(self):
        proc, mock_gw = _processor_with_gateway()
        payload = {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "PROJ-42",
            "is_first_commit_for_ticket": True,
            "pm_platform": "azure",
        }
        result = proc.process_commit(payload)

        assert any("state_transition" in a for a in result["actions"]), (
            f"state_transition not in actions: {result['actions']}"
        )

    def test_state_transition_staged_for_jira(self):
        proc, mock_gw = _processor_with_gateway()
        payload = {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "PROJ-10",
            "is_first_commit_for_ticket": True,
            "pm_platform": "jira",
        }
        proc.process_commit(payload)

        assert mock_gw.stage.call_count == 2
        _, st_kwargs = mock_gw.stage.call_args_list[1]
        assert st_kwargs["action_type"] == "state_transition"
        assert st_kwargs["payload"]["new_state"] == "In Progress"


# ---------------------------------------------------------------------------
# 2. state_transition NOT staged when is_first_commit_for_ticket=False
# ---------------------------------------------------------------------------

class TestStateTransitionNotStagedOnSubsequentCommit:
    """When is_first_commit_for_ticket is False (or absent), only post_comment
    should be staged — no state_transition."""

    def test_only_post_comment_on_second_commit(self):
        proc, mock_gw = _processor_with_gateway()
        payload = {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "PROJ-42",
            "is_first_commit_for_ticket": False,
            "pm_platform": "azure",
        }
        proc.process_commit(payload)

        assert mock_gw.stage.call_count == 1
        _, kwargs = mock_gw.stage.call_args
        assert kwargs["action_type"] == "post_comment"

    def test_only_post_comment_when_flag_absent(self):
        """is_first_commit_for_ticket absent from payload (False by default)."""
        proc, mock_gw = _processor_with_gateway()
        payload = {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "PROJ-42",
            "pm_platform": "azure",
            # is_first_commit_for_ticket intentionally omitted
        }
        proc.process_commit(payload)

        assert mock_gw.stage.call_count == 1
        _, kwargs = mock_gw.stage.call_args
        assert kwargs["action_type"] == "post_comment"


# ---------------------------------------------------------------------------
# 3. GitHub/GitLab in-progress: label convention (TASK-129); unknown platforms skip
# ---------------------------------------------------------------------------

class TestInProgressLabelConvention:
    """GitHub and GitLab have no native in-progress state — the first commit
    stages a label:<name> transition instead (default 'in-progress',
    workspace-configurable, 'none' opts out). Unknown platforms stage nothing."""

    def _first_commit_payload(self, platform, **extra):
        return {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "PROJ-42",
            "is_first_commit_for_ticket": True,
            "pm_platform": platform,
            **extra,
        }

    def test_github_stages_default_label(self):
        proc, mock_gw = _processor_with_gateway()
        proc.process_commit(self._first_commit_payload("github"))

        assert mock_gw.stage.call_count == 2
        _, kwargs = mock_gw.stage.call_args_list[1]
        assert kwargs["action_type"] == "state_transition"
        assert kwargs["payload"]["new_state"] == "label:in-progress"

    def test_gitlab_stages_configured_label(self):
        proc, mock_gw = _processor_with_gateway()
        proc.process_commit(
            self._first_commit_payload("gitlab", pm_in_progress_label="doing")
        )

        assert mock_gw.stage.call_count == 2
        _, kwargs = mock_gw.stage.call_args_list[1]
        assert kwargs["payload"]["new_state"] == "label:doing"

    def test_label_none_opts_out(self):
        proc, mock_gw = _processor_with_gateway()
        proc.process_commit(
            self._first_commit_payload("github", pm_in_progress_label="none")
        )

        assert mock_gw.stage.call_count == 1
        _, kwargs = mock_gw.stage.call_args
        assert kwargs["action_type"] == "post_comment"

    def test_unknown_platform_no_state_transition(self):
        proc, mock_gw = _processor_with_gateway()
        proc.process_commit(self._first_commit_payload("bitbucket"))

        assert mock_gw.stage.call_count == 1
        _, kwargs = mock_gw.stage.call_args
        assert kwargs["action_type"] == "post_comment"

    def test_empty_platform_no_state_transition(self):
        proc, mock_gw = _processor_with_gateway()
        payload = {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "PROJ-42",
            "is_first_commit_for_ticket": True,
            "pm_platform": "",
        }
        proc.process_commit(payload)
        # Only post_comment
        assert mock_gw.stage.call_count == 1


# ---------------------------------------------------------------------------
# 4. state_transition NOT staged when ticket is unresolved
# ---------------------------------------------------------------------------

class TestStateTransitionNotStagedWhenUnlinked:
    """When resolved_ticket_id is absent or empty, no queue actions at all."""

    def test_no_stage_when_ticket_absent(self):
        proc, mock_gw = _processor_with_gateway()
        payload = {
            **COMMIT_PAYLOAD_BASE,
            "is_first_commit_for_ticket": True,
            "pm_platform": "azure",
            # ticket_id intentionally absent
        }
        proc.process_commit(payload)
        mock_gw.stage.assert_not_called()

    def test_no_stage_when_ticket_empty(self):
        proc, mock_gw = _processor_with_gateway()
        payload = {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "",
            "is_first_commit_for_ticket": True,
            "pm_platform": "azure",
        }
        proc.process_commit(payload)
        mock_gw.stage.assert_not_called()


# ---------------------------------------------------------------------------
# 4b. TASK-126: merge-to-default stages a done transition
# ---------------------------------------------------------------------------

class TestMergeToDefaultStagesDone:
    """is_merge_to_default=True must stage a state_transition with the logical
    'done' state — and must NOT stage an in-progress transition, even when the
    merge is the first commit seen for the ticket."""

    def _merge_payload(self, platform="azure"):
        return {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "PROJ-42",
            "branch": "main",
            "commit_message": "Merge branch 'feat/PROJ-42-auth'",
            "is_merge_to_default": True,
            "is_first_commit_for_ticket": False,
            "pm_platform": platform,
        }

    def test_done_transition_staged_on_merge(self):
        proc, mock_gw = _processor_with_gateway()
        proc.process_commit(self._merge_payload())

        assert mock_gw.stage.call_count == 2
        _, second_kwargs = mock_gw.stage.call_args_list[1]
        assert second_kwargs["action_type"] == "state_transition"
        assert second_kwargs["payload"]["new_state"] == "done"
        assert second_kwargs["payload"]["ticket_id"] == "PROJ-42"
        assert second_kwargs["confidence"] == 0.90

    def test_done_staged_even_for_github(self):
        """Unlike in-progress (no GH state), done maps to closing the issue —
        the done transition must be staged for every platform."""
        proc, mock_gw = _processor_with_gateway()
        proc.process_commit(self._merge_payload(platform="github"))

        assert mock_gw.stage.call_count == 2
        _, second_kwargs = mock_gw.stage.call_args_list[1]
        assert second_kwargs["action_type"] == "state_transition"
        assert second_kwargs["payload"]["new_state"] == "done"

    def test_no_in_progress_when_merge_is_first_commit(self):
        proc, mock_gw = _processor_with_gateway()
        payload = self._merge_payload()
        payload["is_first_commit_for_ticket"] = True
        proc.process_commit(payload)

        transition_calls = [
            kwargs for _, kwargs in mock_gw.stage.call_args_list
            if kwargs["action_type"] == "state_transition"
        ]
        assert len(transition_calls) == 1
        assert transition_calls[0]["payload"]["new_state"] == "done"


# ---------------------------------------------------------------------------
# 5. _execute_pm_action routes state_transition to route_state_transition()
# ---------------------------------------------------------------------------

class TestExecutePmActionStateTransition:
    """_execute_pm_action with action_type='state_transition' must call
    workspace_router.route_state_transition() targeted by exact ticket_id
    (TASK-126) — never the fuzzy-matching route()."""

    def _action(self, new_state="Active", platform="azure", ticket_id="PROJ-42"):
        import json
        return {
            "id": 99,
            "action_type": "state_transition",
            "target": ticket_id,
            "platform": platform,
            "workspace": "myapp",
            "payload": json.dumps({
                "ticket_id": ticket_id,
                "new_state": new_state,
                "commit_info": {"hash": "abc123", "message": "feat: auth"},
            }),
        }

    def test_calls_direct_transition_with_new_state(self):
        proc = _bare_processor()
        mock_router = MagicMock()
        mock_router.route_state_transition.return_value = True
        proc.workspace_router = mock_router

        result = proc._execute_pm_action(self._action(new_state="Active"))

        assert result["status"] == "posted"
        mock_router.route_state_transition.assert_called_once()
        _, kwargs = mock_router.route_state_transition.call_args
        assert kwargs["new_state"] == "Active"
        assert kwargs["ticket_id"] == "PROJ-42"

    def test_fuzzy_route_never_used_for_state_transition(self):
        """The exact ticket_id is in hand — fuzzy route() must not run."""
        proc = _bare_processor()
        mock_router = MagicMock()
        mock_router.route_state_transition.return_value = True
        proc.workspace_router = mock_router

        proc._execute_pm_action(self._action())

        mock_router.route.assert_not_called()

    def test_returns_posted_on_success(self):
        proc = _bare_processor()
        mock_router = MagicMock()
        mock_router.route_state_transition.return_value = True
        proc.workspace_router = mock_router

        result = proc._execute_pm_action(self._action())
        assert result == {"status": "posted"}

    def test_returns_failed_when_not_applied(self):
        proc = _bare_processor()
        mock_router = MagicMock()
        mock_router.route_state_transition.return_value = False
        proc.workspace_router = mock_router

        result = proc._execute_pm_action(self._action())
        assert result["status"] == "failed"

    def test_returns_failed_when_router_raises(self):
        proc = _bare_processor()
        mock_router = MagicMock()
        mock_router.route_state_transition.side_effect = RuntimeError("Azure API error")
        proc.workspace_router = mock_router

        result = proc._execute_pm_action(self._action())
        assert result["status"] == "failed"
        assert "Azure API error" in result["error"]

    def test_returns_failed_when_no_workspace_router(self):
        proc = _bare_processor()
        # workspace_router stays None
        result = proc._execute_pm_action(self._action())
        assert result["status"] == "failed"


# ---------------------------------------------------------------------------
# 6. _execute_pm_action with unknown action_type
# ---------------------------------------------------------------------------

class TestExecutePmActionUnknownType:
    """Unknown action_type must log a warning and return posted (never fail)."""

    def _unknown_action(self):
        import json
        return {
            "id": 77,
            "action_type": "mystery_action",
            "target": "PROJ-42",
            "platform": "azure",
            "workspace": "myapp",
            "payload": json.dumps({"ticket_id": "PROJ-42"}),
        }

    def test_returns_posted_for_unknown_type(self):
        proc = _bare_processor()
        mock_router = MagicMock()
        proc.workspace_router = mock_router

        result = proc._execute_pm_action(self._unknown_action())
        assert result["status"] == "posted"

    def test_workspace_router_not_called_for_unknown_type(self):
        proc = _bare_processor()
        mock_router = MagicMock()
        proc.workspace_router = mock_router

        proc._execute_pm_action(self._unknown_action())
        mock_router.route.assert_not_called()


# ---------------------------------------------------------------------------
# 7. TASK-128: extraction-strategy confidence + commit-language demotion
# ---------------------------------------------------------------------------

class TestExtractionConfidencePlumbing:
    """Go's ticket_confidence drives staged-action confidence; commit-language
    completion words stage a LOW-confidence done transition instead of
    auto-closing via the comment route."""

    def test_fallback_ticket_lands_in_review_tier(self):
        proc, mock_gw = _processor_with_gateway()
        payload = {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "PROJ-42",
            "ticket_confidence": 0.60,  # active-ticket fallback
            "is_first_commit_for_ticket": True,
            "pm_platform": "azure",
        }
        proc.process_commit(payload)

        _, first_kwargs = mock_gw.stage.call_args_list[0]
        assert first_kwargs["confidence"] == 0.60
        _, second_kwargs = mock_gw.stage.call_args_list[1]
        assert second_kwargs["action_type"] == "state_transition"
        assert second_kwargs["confidence"] == pytest.approx(0.65)

    def test_branch_ticket_keeps_high_confidence(self):
        proc, mock_gw = _processor_with_gateway()
        payload = {
            **COMMIT_PAYLOAD_BASE,
            "ticket_id": "PROJ-42",
            "ticket_confidence": 0.95,
            "is_first_commit_for_ticket": True,
            "pm_platform": "azure",
        }
        proc.process_commit(payload)

        _, first_kwargs = mock_gw.stage.call_args_list[0]
        assert first_kwargs["confidence"] == 0.95
        _, second_kwargs = mock_gw.stage.call_args_list[1]
        assert second_kwargs["confidence"] == pytest.approx(0.90)  # capped

    def test_commit_language_done_is_demoted(self):
        """'done'-word commits: status stripped from the comment payload and a
        separate 0.65-confidence done transition staged for explicit review."""
        proc, mock_gw = _processor_with_gateway()
        proc.nlp_parser = MagicMock()
        proc.nlp_parser.parse.return_value = {
            "status": "completed",
            "description": "finished the auth flow",
        }
        payload = {
            **COMMIT_PAYLOAD_BASE,
            "commit_message": "finished the auth flow",
            "ticket_id": "PROJ-42",
            "ticket_confidence": 0.95,
            "is_first_commit_for_ticket": False,
            "pm_platform": "azure",
        }
        proc.process_commit(payload)

        _, first_kwargs = mock_gw.stage.call_args_list[0]
        assert first_kwargs["action_type"] == "post_comment"
        assert first_kwargs["payload"]["status"] == ""  # never auto-closes via route()

        transition_calls = [
            kwargs for _, kwargs in mock_gw.stage.call_args_list
            if kwargs["action_type"] == "state_transition"
        ]
        assert len(transition_calls) == 1
        assert transition_calls[0]["payload"]["new_state"] == "done"
        assert transition_calls[0]["confidence"] == pytest.approx(0.65)
