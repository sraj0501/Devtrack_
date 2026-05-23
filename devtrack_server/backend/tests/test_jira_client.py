"""
Tests for the Jira integration module.

No network calls are made — all external calls are mocked.
"""

import pytest
from unittest.mock import MagicMock, patch


# ---------------------------------------------------------------------------
# Fixtures / helpers
# ---------------------------------------------------------------------------

def _make_raw_issue(
    id="10001",
    key="PROJ-1",
    summary="Fix the login bug",
    description=None,
    status_name="In Progress",
    assignee_display="Alice Smith",
    assignee_email="alice@example.com",
    issue_type="Bug",
    priority="High",
    labels=None,
):
    """Return a raw Jira API issue dict."""
    return {
        "id": id,
        "key": key,
        "fields": {
            "summary": summary,
            "description": description,
            "status": {"name": status_name},
            "assignee": {
                "displayName": assignee_display,
                "emailAddress": assignee_email,
            },
            "issuetype": {"name": issue_type},
            "priority": {"name": priority},
            "labels": labels or [],
        },
    }


def _make_adf_description(text_parts):
    """Build a minimal ADF document with paragraph nodes."""
    return {
        "type": "doc",
        "content": [
            {
                "type": "paragraph",
                "content": [{"type": "text", "text": t} for t in text_parts],
            }
        ],
    }


# ---------------------------------------------------------------------------
# JiraClient.is_configured()
# ---------------------------------------------------------------------------

class TestIsConfigured:
    def test_returns_false_when_no_credentials(self):
        from backend.jira.client import JiraClient
        client = JiraClient(base_url="", email="", api_token="", project_key="")
        assert client.is_configured() is False

    def test_returns_false_when_missing_url(self):
        from backend.jira.client import JiraClient
        client = JiraClient(base_url="", email="me@example.com", api_token="token123", project_key="PROJ")
        assert client.is_configured() is False

    def test_returns_false_when_missing_email(self):
        from backend.jira.client import JiraClient
        client = JiraClient(base_url="https://org.atlassian.net", email="", api_token="token123", project_key="PROJ")
        assert client.is_configured() is False

    def test_returns_false_when_missing_token(self):
        from backend.jira.client import JiraClient
        client = JiraClient(base_url="https://org.atlassian.net", email="me@example.com", api_token="", project_key="PROJ")
        assert client.is_configured() is False

    def test_returns_true_when_all_credentials_present(self):
        from backend.jira.client import JiraClient
        client = JiraClient(
            base_url="https://org.atlassian.net",
            email="me@example.com",
            api_token="token123",
            project_key="PROJ",
        )
        assert client.is_configured() is True


# ---------------------------------------------------------------------------
# JiraClient.get_my_issues() — no network
# ---------------------------------------------------------------------------

class TestGetMyIssues:
    def _unconfigured_client(self):
        from backend.jira.client import JiraClient
        return JiraClient(base_url="", email="", api_token="", project_key="")

    def _configured_client(self):
        from backend.jira.client import JiraClient
        return JiraClient(
            base_url="https://org.atlassian.net",
            email="me@example.com",
            api_token="token123",
            project_key="PROJ",
        )

    def test_returns_empty_list_when_not_configured(self):
        client = self._unconfigured_client()
        assert client.get_my_issues() == []

    def test_returns_empty_list_when_no_email(self):
        from backend.jira.client import JiraClient
        # configured URL + token but no email
        client = JiraClient(base_url="https://org.atlassian.net", email="", api_token="token123", project_key="PROJ")
        result = client.get_my_issues()
        assert result == []

    def test_returns_issues_on_success(self):
        client = self._configured_client()
        raw = _make_raw_issue()
        mock_response = {"issues": [raw]}

        with patch("requests.get") as mock_get:
            mock_resp = MagicMock()
            mock_resp.json.return_value = mock_response
            mock_resp.raise_for_status.return_value = None
            mock_get.return_value = mock_resp

            issues = client.get_my_issues()

        assert len(issues) == 1
        assert issues[0].key == "PROJ-1"
        assert issues[0].summary == "Fix the login bug"

    def test_returns_empty_on_api_failure(self):
        client = self._configured_client()

        with patch("requests.get") as mock_get:
            mock_get.side_effect = Exception("Connection refused")
            issues = client.get_my_issues()

        assert issues == []

    def test_status_filter_passed_in_jql(self):
        client = self._configured_client()
        mock_response = {"issues": []}

        with patch("requests.get") as mock_get:
            mock_resp = MagicMock()
            mock_resp.json.return_value = mock_response
            mock_resp.raise_for_status.return_value = None
            mock_get.return_value = mock_resp

            client.get_my_issues(status_filter=["In Progress", "To Do"])

        call_kwargs = mock_get.call_args
        jql = call_kwargs.kwargs.get("params", {}).get("jql", "")
        assert "In Progress" in jql
        assert "To Do" in jql


# ---------------------------------------------------------------------------
# JiraClient.get_issue()
# ---------------------------------------------------------------------------

class TestGetIssue:
    def _configured_client(self):
        from backend.jira.client import JiraClient
        return JiraClient(
            base_url="https://org.atlassian.net",
            email="me@example.com",
            api_token="token123",
            project_key="PROJ",
        )

    def test_returns_none_when_not_configured(self):
        from backend.jira.client import JiraClient
        client = JiraClient(base_url="", email="", api_token="", project_key="")
        assert client.get_issue("PROJ-1") is None

    def test_returns_issue_on_success(self):
        client = self._configured_client()
        raw = _make_raw_issue(key="PROJ-42", summary="Add OAuth support")

        with patch("requests.get") as mock_get:
            mock_resp = MagicMock()
            mock_resp.json.return_value = raw
            mock_resp.raise_for_status.return_value = None
            mock_get.return_value = mock_resp

            issue = client.get_issue("PROJ-42")

        assert issue is not None
        assert issue.key == "PROJ-42"
        assert issue.summary == "Add OAuth support"

    def test_returns_none_on_api_error(self):
        client = self._configured_client()

        with patch("requests.get") as mock_get:
            mock_get.side_effect = Exception("404 Not Found")
            result = client.get_issue("PROJ-999")

        assert result is None


# ---------------------------------------------------------------------------
# JiraClient._parse_issue() — field mapping
# ---------------------------------------------------------------------------

class TestParseIssue:
    def _client(self):
        from backend.jira.client import JiraClient
        return JiraClient(
            base_url="https://org.atlassian.net",
            email="me@example.com",
            api_token="token123",
            project_key="PROJ",
        )

    def test_maps_all_basic_fields(self):
        client = self._client()
        raw = _make_raw_issue(
            id="10002",
            key="PROJ-2",
            summary="Update API docs",
            status_name="To Do",
            assignee_display="Bob Jones",
            issue_type="Task",
            priority="Medium",
            labels=["docs", "api"],
        )

        issue = client._parse_issue(raw)

        assert issue.id == "10002"
        assert issue.key == "PROJ-2"
        assert issue.summary == "Update API docs"
        assert issue.status == "To Do"
        assert issue.assignee == "Bob Jones"
        assert issue.issue_type == "Task"
        assert issue.priority == "Medium"
        assert issue.labels == ["docs", "api"]

    def test_url_constructed_correctly(self):
        client = self._client()
        raw = _make_raw_issue(key="PROJ-5")
        issue = client._parse_issue(raw)
        assert issue.url == "https://org.atlassian.net/browse/PROJ-5"

    def test_handles_missing_assignee(self):
        client = self._client()
        raw = _make_raw_issue()
        raw["fields"]["assignee"] = None
        issue = client._parse_issue(raw)
        assert issue.assignee == ""

    def test_handles_missing_status(self):
        client = self._client()
        raw = _make_raw_issue()
        raw["fields"]["status"] = None
        issue = client._parse_issue(raw)
        assert issue.status == "Unknown"

    def test_handles_empty_labels(self):
        client = self._client()
        raw = _make_raw_issue(labels=[])
        issue = client._parse_issue(raw)
        assert issue.labels == []


# ---------------------------------------------------------------------------
# JiraClient._extract_description() and _adf_to_text()
# ---------------------------------------------------------------------------

class TestDescriptionExtraction:
    def _client(self):
        from backend.jira.client import JiraClient
        return JiraClient(
            base_url="https://org.atlassian.net",
            email="me@example.com",
            api_token="token123",
            project_key="PROJ",
        )

    def test_none_returns_empty_string(self):
        client = self._client()
        assert client._extract_description(None) == ""

    def test_plain_string_returned_as_is(self):
        client = self._client()
        assert client._extract_description("Simple description") == "Simple description"

    def test_adf_single_text_node(self):
        client = self._client()
        adf = {"type": "text", "text": "Hello world"}
        assert client._adf_to_text(adf) == "Hello world"

    def test_adf_paragraph_with_multiple_text_nodes(self):
        client = self._client()
        adf = _make_adf_description(["Fix the bug", "in login flow"])
        result = client._extract_description(adf)
        assert "Fix the bug" in result
        assert "login flow" in result

    def test_adf_empty_document_returns_empty(self):
        client = self._client()
        adf = {"type": "doc", "content": []}
        result = client._extract_description(adf)
        assert result == ""

    def test_nested_adf_extracts_all_text(self):
        client = self._client()
        adf = {
            "type": "doc",
            "content": [
                {
                    "type": "paragraph",
                    "content": [{"type": "text", "text": "First paragraph"}],
                },
                {
                    "type": "paragraph",
                    "content": [{"type": "text", "text": "Second paragraph"}],
                },
            ],
        }
        result = client._extract_description(adf)
        assert "First paragraph" in result
        assert "Second paragraph" in result
