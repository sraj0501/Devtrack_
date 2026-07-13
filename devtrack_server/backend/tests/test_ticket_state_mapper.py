"""
Tests for ticket_state_mapper.py (TASK-073).

Covers:
  - Known platforms return expected state strings
  - Unknown/empty platform returns ""
  - Lookup is case-insensitive
"""
from __future__ import annotations

import sys
from pathlib import Path

import pytest

_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


from backend.ticket_state_mapper import in_progress_state_for, PLATFORM_IN_PROGRESS_STATE


class TestKnownPlatforms:
    """Known platforms must return a non-empty state string."""

    def test_azure_returns_active(self):
        assert in_progress_state_for("azure") == "Active"

    def test_jira_returns_in_progress(self):
        assert in_progress_state_for("jira") == "In Progress"

    def test_github_returns_empty(self):
        # GitHub Issues has no native in-progress state in this codebase
        assert in_progress_state_for("github") == ""

    def test_gitlab_returns_empty(self):
        # GitLab issues use open/closed binary state; no in-progress API state
        assert in_progress_state_for("gitlab") == ""


class TestUnknownPlatforms:
    """Unknown or empty platform must return ''."""

    def test_unknown_platform_returns_empty(self):
        assert in_progress_state_for("bitbucket") == ""

    def test_empty_string_returns_empty(self):
        assert in_progress_state_for("") == ""

    def test_none_like_empty_returns_empty(self):
        # Callers may pass None; coerce to ""
        assert in_progress_state_for(None) == ""  # type: ignore[arg-type]

    def test_arbitrary_string_returns_empty(self):
        assert in_progress_state_for("asana") == ""


class TestCaseInsensitivity:
    """Lookup must be case-insensitive."""

    def test_azure_uppercase(self):
        assert in_progress_state_for("AZURE") == "Active"

    def test_azure_mixed_case(self):
        assert in_progress_state_for("Azure") == "Active"

    def test_jira_uppercase(self):
        assert in_progress_state_for("JIRA") == "In Progress"

    def test_github_uppercase(self):
        # Still "" — just confirms case-insensitivity applies here too
        assert in_progress_state_for("GITHUB") == ""


class TestMappingDictIntegrity:
    """The PLATFORM_IN_PROGRESS_STATE dict must contain the four expected keys."""

    def test_all_expected_keys_present(self):
        expected = {"azure", "github", "gitlab", "jira"}
        assert expected <= set(PLATFORM_IN_PROGRESS_STATE.keys()), (
            f"Missing keys: {expected - set(PLATFORM_IN_PROGRESS_STATE.keys())}"
        )
