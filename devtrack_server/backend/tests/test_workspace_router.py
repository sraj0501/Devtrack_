"""
Tests for backend.workspace_router.WorkspaceRouter.
"""
import asyncio
import pytest
from unittest.mock import AsyncMock, MagicMock, patch


from backend.workspace_router import WorkspaceRouter


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_router(azure=None, gitlab=None, github=None) -> WorkspaceRouter:
    return WorkspaceRouter(azure_client=azure, gitlab_client=gitlab, github_client=github)


def _stub_call_sync(return_value):
    """Patch WorkspaceRouter._call_sync to return a fixed RouteResult."""
    return patch.object(WorkspaceRouter, "_call_sync", return_value=return_value)


# ---------------------------------------------------------------------------
# pm_platform="none" — always skip
# ---------------------------------------------------------------------------

def test_route_none_platform():
    """pm_platform='none' returns (None, None) regardless of clients."""
    router = make_router(azure=MagicMock(), gitlab=MagicMock(), github=MagicMock())
    result = router.route("none", "some work", "TICKET-1", "in_progress")
    assert result == (None, None)


def test_route_none_platform_case_insensitive():
    """pm_platform='NONE' is also treated as none."""
    router = make_router(azure=MagicMock())
    result = router.route("NONE", "some work", "TICKET-1", "in_progress")
    assert result == (None, None)


# ---------------------------------------------------------------------------
# pm_platform="" — falls back to priority chain
# ---------------------------------------------------------------------------

def test_route_empty_platform_uses_priority_chain():
    """Empty pm_platform delegates to _route_priority_chain."""
    router = make_router()
    with patch.object(router, "_route_priority_chain", return_value=(99, "azure")) as mock_chain:
        result = router.route("", "desc", "T-1", "done")
    mock_chain.assert_called_once()
    assert result == (99, "azure")


def test_route_whitespace_platform_uses_priority_chain():
    """Whitespace-only pm_platform is treated as empty."""
    router = make_router()
    with patch.object(router, "_route_priority_chain", return_value=(None, None)) as mock_chain:
        result = router.route("   ", "desc", "T-1", "done")
    mock_chain.assert_called_once()
    assert result == (None, None)


# ---------------------------------------------------------------------------
# Direct routing — no client configured
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("platform", ["azure", "gitlab", "github"])
def test_route_no_client_returns_none(platform):
    """When pm_platform is set but the matching client is None, return (None, None)."""
    router = make_router()  # no clients
    result = router.route(platform, "desc", "T-1", "in_progress")
    assert result == (None, None)


# ---------------------------------------------------------------------------
# Individual alias tests kept for clarity
# ---------------------------------------------------------------------------

def test_route_azure_no_client():
    """pm_platform='azure' with no azure_client returns (None, None)."""
    router = make_router(gitlab=MagicMock(), github=MagicMock())
    result = router.route("azure", "desc", "T-1", "in_progress")
    assert result == (None, None)


def test_route_gitlab_no_client():
    """pm_platform='gitlab' with no gitlab_client returns (None, None)."""
    router = make_router(azure=MagicMock(), github=MagicMock())
    result = router.route("gitlab", "desc", "T-1", "in_progress")
    assert result == (None, None)


def test_route_github_no_client():
    """pm_platform='github' with no github_client returns (None, None)."""
    router = make_router(azure=MagicMock(), gitlab=MagicMock())
    result = router.route("github", "desc", "T-1", "in_progress")
    assert result == (None, None)


# ---------------------------------------------------------------------------
# Unknown platform — falls back to priority chain
# ---------------------------------------------------------------------------

def test_route_unknown_platform_falls_back():
    """An unrecognised pm_platform logs a warning and falls back to priority chain."""
    router = make_router()
    with patch.object(router, "_route_priority_chain", return_value=(None, None)) as mock_chain:
        result = router.route("unknown_xyz", "desc", "T-1", "done")
    mock_chain.assert_called_once()
    assert result == (None, None)


# ---------------------------------------------------------------------------
# Jira — not yet implemented
# ---------------------------------------------------------------------------

def test_route_jira_not_implemented():
    """pm_platform='jira' returns (None, None) (not yet implemented)."""
    router = make_router(azure=MagicMock(), gitlab=MagicMock(), github=MagicMock())
    result = router.route("jira", "desc", "T-1", "in_progress")
    assert result == (None, None)


# ---------------------------------------------------------------------------
# Priority chain ordering
# ---------------------------------------------------------------------------

def test_priority_chain_tries_azure_first():
    """Priority chain calls azure first; when it returns a match we don't try others."""
    azure_client = MagicMock()
    gitlab_client = MagicMock()
    github_client = MagicMock()
    router = make_router(azure=azure_client, gitlab=gitlab_client, github=github_client)

    # Patch config so all platforms appear enabled
    with patch("backend.config.is_azure_sync_enabled", return_value=True), \
         patch("backend.config.is_gitlab_sync_enabled", return_value=True), \
         patch("backend.config.is_github_sync_enabled", return_value=True), \
         patch.object(router, "_call_sync", side_effect=[
             (42, "azure"),   # azure succeeds
             (99, "gitlab"),  # should never be reached
             (77, "github"),  # should never be reached
         ]) as mock_call:
        result = router._route_priority_chain("desc", "T-1", "done", "", None, None)

    assert result == (42, "azure")
    assert mock_call.call_count == 1
    assert mock_call.call_args[0][0] == "azure"


def test_priority_chain_skips_to_gitlab_when_azure_unavailable():
    """Priority chain skips azure (no client) and tries gitlab."""
    gitlab_client = MagicMock()
    github_client = MagicMock()
    router = make_router(gitlab=gitlab_client, github=github_client)

    with patch("backend.config.is_gitlab_sync_enabled", return_value=True), \
         patch("backend.config.is_github_sync_enabled", return_value=True), \
         patch.object(router, "_call_sync", side_effect=[
             (55, "gitlab"),
             (77, "github"),  # should not be reached
         ]) as mock_call:
        result = router._route_priority_chain("desc", "T-1", "done", "", None, None)

    assert result == (55, "gitlab")
    assert mock_call.call_count == 1
    assert mock_call.call_args[0][0] == "gitlab"


def test_priority_chain_skips_to_github_when_azure_gitlab_unavailable():
    """Priority chain skips azure and gitlab (no clients) and tries github."""
    github_client = MagicMock()
    router = make_router(github=github_client)

    with patch("backend.config.is_github_sync_enabled", return_value=True), \
         patch.object(router, "_call_sync", return_value=(33, "github")) as mock_call:
        result = router._route_priority_chain("desc", "T-1", "done", "", None, None)

    assert result == (33, "github")
    assert mock_call.call_count == 1
    assert mock_call.call_args[0][0] == "github"


def test_priority_chain_returns_none_when_all_disabled():
    """Priority chain returns (None, None) when no platform is enabled."""
    router = make_router(azure=MagicMock(), gitlab=MagicMock(), github=MagicMock())

    with patch("backend.config.is_azure_sync_enabled", return_value=False), \
         patch("backend.config.is_gitlab_sync_enabled", return_value=False), \
         patch("backend.config.is_github_sync_enabled", return_value=False):
        result = router._route_priority_chain("desc", "T-1", "done", "", None, None)

    assert result == (None, None)


# ---------------------------------------------------------------------------
# Azure routing — correct method called
# ---------------------------------------------------------------------------

def test_route_azure_calls_correct_method():
    """_route_azure delegates to _call_sync with platform='azure'."""
    azure_client = MagicMock()
    router = make_router(azure=azure_client)

    with patch("backend.config.is_azure_sync_enabled", return_value=True), \
         patch.object(router, "_call_sync", return_value=(7, "azure")) as mock_call:
        result = router._route_azure("did some work", "T-1", "done", None, None)

    assert result == (7, "azure")
    mock_call.assert_called_once()
    call_args = mock_call.call_args[0]
    assert call_args[0] == "azure"
    assert call_args[1] is azure_client


# ---------------------------------------------------------------------------
# GitLab pm_project parsing
# ---------------------------------------------------------------------------

def test_gitlab_pm_project_parsed_as_int():
    """A numeric pm_project string is converted to int and passed to _call_sync."""
    gitlab_client = MagicMock()
    router = make_router(gitlab=gitlab_client)

    captured = {}

    def capture_call_sync(platform, client, description, ticket_id, status,
                          commit_info, task_matcher, project_id, overrides=None):
        captured["project_id"] = project_id
        return (42, "gitlab")

    with patch("backend.config.is_gitlab_sync_enabled", return_value=True), \
         patch.object(router, "_call_sync", side_effect=capture_call_sync):
        router._route_gitlab("desc", "T-1", "done", "42", None, None)

    assert captured["project_id"] == 42


def test_gitlab_pm_project_invalid_ignored():
    """A non-integer pm_project is silently ignored (project_id stays None)."""
    gitlab_client = MagicMock()
    router = make_router(gitlab=gitlab_client)

    captured = {}

    def capture_call_sync(platform, client, description, ticket_id, status,
                          commit_info, task_matcher, project_id, overrides=None):
        captured["project_id"] = project_id
        return (None, None)

    with patch("backend.config.is_gitlab_sync_enabled", return_value=True), \
         patch.object(router, "_call_sync", side_effect=capture_call_sync):
        router._route_gitlab("desc", "T-1", "done", "not-an-int", None, None)

    assert captured["project_id"] is None


# ---------------------------------------------------------------------------
# _route_priority_chain — all no-match falls through to (None, None)
# ---------------------------------------------------------------------------

def test_priority_chain_all_return_none():
    """If every platform returns (None, None), the chain returns (None, None)."""
    router = make_router(azure=MagicMock(), gitlab=MagicMock(), github=MagicMock())

    with patch("backend.config.is_azure_sync_enabled", return_value=True), \
         patch("backend.config.is_gitlab_sync_enabled", return_value=True), \
         patch("backend.config.is_github_sync_enabled", return_value=True), \
         patch.object(router, "_call_sync", return_value=(None, None)):
        result = router._route_priority_chain("desc", "T-1", "done", "", None, None)

    assert result == (None, None)


# ---------------------------------------------------------------------------
# Per-workspace overrides — Azure
# ---------------------------------------------------------------------------

def test_route_passes_azure_overrides_to_call_sync():
    """pm_assignee, pm_area_path, pm_iteration_path are forwarded to _call_sync."""
    azure_client = MagicMock()
    router = make_router(azure=azure_client)

    captured = {}

    def capture(platform, client, description, ticket_id, status,
                commit_info, task_matcher, project_id, overrides=None):
        captured.update(overrides or {})
        return (5, "azure")

    with patch("backend.config.is_azure_sync_enabled", return_value=True), \
         patch.object(router, "_call_sync", side_effect=capture):
        router.route(
            pm_platform="azure",
            description="fix login",
            ticket_id="",
            status="done",
            pm_assignee="dev@example.com",
            pm_iteration_path="MyProject\\Sprint 3",
            pm_area_path="MyProject\\Backend",
        )

    assert captured["pm_assignee"] == "dev@example.com"
    assert captured["pm_iteration_path"] == "MyProject\\Sprint 3"
    assert captured["pm_area_path"] == "MyProject\\Backend"


# ---------------------------------------------------------------------------
# Per-workspace overrides — GitHub
# ---------------------------------------------------------------------------

def test_route_passes_github_overrides_to_call_sync():
    """pm_assignee and pm_milestone are forwarded for GitHub routing."""
    github_client = MagicMock()
    router = make_router(github=github_client)

    captured = {}

    def capture(platform, client, description, ticket_id, status,
                commit_info, task_matcher, project_id, overrides=None):
        captured.update(overrides or {})
        return (10, "github")

    with patch("backend.config.is_github_sync_enabled", return_value=True), \
         patch.object(router, "_call_sync", side_effect=capture):
        router.route(
            pm_platform="github",
            description="add feature",
            ticket_id="",
            status="in_progress",
            pm_assignee="octocat",
            pm_milestone="7",
        )

    assert captured["pm_assignee"] == "octocat"
    assert captured["pm_milestone"] == "7"


# ---------------------------------------------------------------------------
# Per-workspace overrides — GitLab
# ---------------------------------------------------------------------------

def test_route_passes_gitlab_overrides_to_call_sync():
    """pm_assignee (user ID) and pm_milestone are forwarded for GitLab routing."""
    gitlab_client = MagicMock()
    router = make_router(gitlab=gitlab_client)

    captured = {}

    def capture(platform, client, description, ticket_id, status,
                commit_info, task_matcher, project_id, overrides=None):
        captured.update(overrides or {})
        return (20, "gitlab")

    with patch("backend.config.is_gitlab_sync_enabled", return_value=True), \
         patch.object(router, "_call_sync", side_effect=capture):
        router.route(
            pm_platform="gitlab",
            description="refactor auth",
            ticket_id="",
            status="done",
            pm_project="42",
            pm_assignee="123",
            pm_milestone="5",
        )

    assert captured["pm_assignee"] == "123"
    assert captured["pm_milestone"] == "5"


# ---------------------------------------------------------------------------
# Per-workspace overrides — none platform still skips
# ---------------------------------------------------------------------------

def test_overrides_ignored_for_none_platform():
    """pm_platform=none always returns (None, None) regardless of overrides."""
    router = make_router(azure=MagicMock())

    result = router.route(
        pm_platform="none",
        description="work done",
        ticket_id="",
        status="done",
        pm_assignee="someone",
        pm_iteration_path="Sprint 1",
    )

    assert result == (None, None)
