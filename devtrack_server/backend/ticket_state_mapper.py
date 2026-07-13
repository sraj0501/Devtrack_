"""
ticket_state_mapper.py — Per-platform "in progress" state vocabulary (TASK-073).

Maps DevTrack's logical "start work" intent to each PM platform's actual state
vocabulary.  Called by process_commit when the first commit for a ticket is
detected (is_first_commit_for_ticket=True in the Go payload), to decide what
state string to request via workspace_router.route(status=...).

Platform research findings (checked before writing this module):
  - Azure DevOps (backend/azure/client.py):
      WorkItemState enum: "New" → "Active" → "Resolved" → "Closed"
      workspace_router uses update_work_item_state(id, state_str) for transitions.
      "Active" is the native in-progress state name in the ADO default process.

  - GitHub Issues (backend/github/client.py):
      GitHub Issues only has binary state: "open" or "closed".
      There is no native "In Progress" state; the convention is labels.
      This codebase uses config.get_github_sync_label() to set a label on
      create, but there is NO existing label-as-state handling for "in progress"
      in github/client.py or workspace_router.py.
      → We map GitHub to "" (empty) so callers skip the transition rather than
        inventing a parallel label-based mechanism that doesn't exist in this
        codebase. The state_transition action is simply not staged for GitHub.

  - GitLab Issues (backend/gitlab/client.py):
      GitLab issues are either "opened" or "closed". The board-column "doing"
      convention exists at the UI level but is not exposed as an API state.
      update_issue() accepts state_event="close" or "reopen", not arbitrary
      state strings. No label-as-state handling exists in gitlab/client.py or
      workspace_router.py for "in progress".
      → Same decision as GitHub: map to "" so callers skip the transition.

  - Jira:
      Standard Jira workflow: To Do → In Progress → Done.
      "In Progress" is the canonical state name for this transition.

NOTE: The existing auto-transition code in workspace_router.py lines 298/372/445
      only handles "done"/"completed"/"closed" — it does NOT handle "starting work".
      This module adds that missing piece for the platforms that support it (Azure,
      Jira). GitHub and GitLab return "" because no equivalent API operation exists
      in this codebase.
"""

from __future__ import annotations

# Maps each platform key (lowercase, matching workspace_router.py conventions) to
# the state string that signals "work has started on this ticket".
# GitHub and GitLab are explicitly set to "" — callers must skip the transition
# when this function returns "" (never guess or invent a state string).
PLATFORM_IN_PROGRESS_STATE: dict[str, str] = {
    "azure":  "Active",       # ADO default process: New → Active → Resolved → Closed
    "github": "",             # GitHub Issues has no native in-progress state; no existing
                              # label-as-state convention in this codebase — skip transition
    "gitlab": "",             # GitLab Issues: opened/closed only; no in-progress API state
    "jira":   "In Progress",  # Jira default workflow: To Do → In Progress → Done
}


def in_progress_state_for(platform: str) -> str:
    """Return the platform-specific state string for the "start work" transition.

    Args:
        platform: Platform key as used in workspaces.yaml / CommitTriggerData
                  (e.g. "azure", "github", "gitlab", "jira"). Case-insensitive.

    Returns:
        The state string to pass to workspace_router.route(status=...) when
        transitioning the ticket to "in progress".  Returns ``""`` for any
        unrecognized or unsupported platform — callers MUST skip the transition
        when this returns "", never guess.
    """
    return PLATFORM_IN_PROGRESS_STATE.get((platform or "").lower(), "")
