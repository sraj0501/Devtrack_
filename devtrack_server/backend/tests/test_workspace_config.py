"""
Smoke tests for WorkspaceRouter import and basic instantiation.

Python does not read workspaces.yaml directly — workspace routing info
arrives via IPC from the Go daemon.  These tests verify the module is
importable and that WorkspaceRouter behaves correctly with various client
configurations, without touching any external service.
"""
import pytest
from unittest.mock import MagicMock

from backend.workspace_router import WorkspaceRouter


# ---------------------------------------------------------------------------
# 1. Import smoke test
# ---------------------------------------------------------------------------

def test_workspace_router_import():
    """WorkspaceRouter can be imported without any external dependencies."""
    assert WorkspaceRouter is not None


# ---------------------------------------------------------------------------
# 2. Instantiation — no clients
# ---------------------------------------------------------------------------

def test_workspace_router_instantiation_no_clients():
    """WorkspaceRouter can be created with no clients (all default to None)."""
    router = WorkspaceRouter()
    assert router.azure_client is None
    assert router.gitlab_client is None
    assert router.github_client is None


# ---------------------------------------------------------------------------
# 3. Instantiation — with mock clients
# ---------------------------------------------------------------------------

def test_workspace_router_instantiation_with_clients():
    """WorkspaceRouter stores the clients it is given."""
    azure = MagicMock(name="azure_client")
    gitlab = MagicMock(name="gitlab_client")
    github = MagicMock(name="github_client")

    router = WorkspaceRouter(azure_client=azure, gitlab_client=gitlab, github_client=github)

    assert router.azure_client is azure
    assert router.gitlab_client is gitlab
    assert router.github_client is github


# ---------------------------------------------------------------------------
# 4. route() always returns a 2-tuple
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("platform", [
    "none",
    "azure",
    "gitlab",
    "github",
    "jira",
    "unknown_platform",
    "",
])
def test_route_result_is_tuple(platform):
    """route() always returns a 2-tuple regardless of platform or client state."""
    router = WorkspaceRouter()
    result = router.route(platform, "some description", "T-1", "in_progress")
    assert isinstance(result, tuple)
    assert len(result) == 2
