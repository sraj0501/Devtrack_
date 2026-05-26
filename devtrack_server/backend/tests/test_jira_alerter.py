"""
Tests for backend.alerters.jira_alerter.JiraAlerter.

No real network calls — all JiraClient methods are mocked.
"""

from __future__ import annotations

import asyncio
import pytest
from datetime import datetime, timezone, timedelta
from unittest.mock import MagicMock, patch, AsyncMock


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

BASE_DT = datetime(2026, 4, 1, 10, 0, 0, tzinfo=timezone.utc)
AFTER_DT = BASE_DT + timedelta(hours=1)  # "newer than last_checked"


def _make_issue(key="PROJ-1", summary="Fix login", status="In Progress", url=None):
    """Return a mock JiraIssue-like object."""
    m = MagicMock()
    m.key = key
    m.summary = summary
    m.status = status
    m.url = url or f"https://jira.example.com/browse/{key}"
    return m


def _make_comment(comment_id="c1", author_email="bob@example.com",
                  author_name="Bob", body_text="Looks good",
                  created=None):
    created = created or AFTER_DT.isoformat()
    return {
        "id": comment_id,
        "author": {"emailAddress": author_email, "displayName": author_name},
        "body_text": body_text,
        "created": created,
    }


def _make_changelog_entry(old_status="Open", new_status="In Progress",
                           author_name="Alice", created=None):
    created = created or AFTER_DT.isoformat()
    return {
        "created": created,
        "author": {"displayName": author_name},
        "items": [
            {"field": "status", "fromString": old_status, "toString": new_status}
        ],
    }


# ---------------------------------------------------------------------------
# Fixture: JiraAlerter with a mocked client
# ---------------------------------------------------------------------------

@pytest.fixture
def alerter_with_client():
    """Return a JiraAlerter whose internal _client is fully mocked."""
    from backend.alerters.jira_alerter import JiraAlerter

    alerter = JiraAlerter()
    alerter._client = MagicMock()
    alerter._client.is_configured.return_value = True
    # Default: _search and get_my_issues return empty lists
    alerter._client._search.return_value = []
    alerter._client.get_my_issues.return_value = []
    alerter._client.get_issue_comments.return_value = []
    alerter._client.get_issue_changelog.return_value = []
    alerter._me_email = "me@example.com"
    return alerter


# ---------------------------------------------------------------------------
# _make_notification / _parse_dt helpers
# ---------------------------------------------------------------------------

def test_make_notification_structure():
    from backend.alerters.jira_alerter import _make_notification
    notif = _make_notification(
        event_type="assigned",
        ticket_id="PROJ-1",
        title="Fix login",
        summary="Assigned to you: PROJ-1",
        url="https://jira.example.com/browse/PROJ-1",
        timestamp=BASE_DT,
        raw={"key": "PROJ-1"},
    )
    assert notif["source"] == "jira"
    assert notif["event_type"] == "assigned"
    assert notif["ticket_id"] == "PROJ-1"
    assert notif["read"] is False
    assert notif["dismissed"] is False
    assert notif["timestamp"].tzinfo is not None


def test_make_notification_naive_dt_gets_utc():
    from backend.alerters.jira_alerter import _make_notification
    naive = datetime(2026, 4, 1, 12, 0, 0)  # no tzinfo
    notif = _make_notification("comment", "X-1", "T", "S", "u", naive, {})
    assert notif["timestamp"].tzinfo is not None


def test_parse_dt_iso_z():
    from backend.alerters.jira_alerter import _parse_dt
    dt = _parse_dt("2026-04-01T10:00:00.000Z")
    assert dt is not None
    assert dt.tzinfo is not None


def test_parse_dt_iso_offset():
    from backend.alerters.jira_alerter import _parse_dt
    dt = _parse_dt("2026-04-01T10:00:00.000+0000")
    assert dt is not None
    assert dt.tzinfo is not None


def test_parse_dt_none():
    from backend.alerters.jira_alerter import _parse_dt
    assert _parse_dt(None) is None
    assert _parse_dt("") is None


def test_parse_dt_invalid():
    from backend.alerters.jira_alerter import _parse_dt
    assert _parse_dt("not-a-date") is None


# ---------------------------------------------------------------------------
# is_configured
# ---------------------------------------------------------------------------

def test_is_configured_true(alerter_with_client):
    assert alerter_with_client.is_configured() is True


def test_is_configured_false_no_client():
    from backend.alerters.jira_alerter import JiraAlerter
    a = JiraAlerter()
    a._client = None
    assert a.is_configured() is False


def test_is_configured_false_client_not_configured(alerter_with_client):
    alerter_with_client._client.is_configured.return_value = False
    assert alerter_with_client.is_configured() is False


# ---------------------------------------------------------------------------
# poll — disabled / unconfigured
# ---------------------------------------------------------------------------

def test_poll_disabled_returns_empty(alerter_with_client):
    with patch("backend.alerters.jira_alerter.cfg.get_bool") as mock_get_bool:
        mock_get_bool.return_value = False  # ALERT_JIRA_ENABLED=false
        result = asyncio.run(alerter_with_client.poll(last_checked=BASE_DT))
    assert result == []


def test_poll_unconfigured_returns_empty():
    from backend.alerters.jira_alerter import JiraAlerter
    alerter = JiraAlerter()
    alerter._client = None
    result = asyncio.run(alerter.poll(last_checked=BASE_DT))
    assert result == []


# ---------------------------------------------------------------------------
# _poll_assigned
# ---------------------------------------------------------------------------

def test_poll_assigned_no_since_skips(alerter_with_client):
    result = asyncio.run(alerter_with_client._poll_assigned(since=None))
    assert result == []
    alerter_with_client._client._search.assert_not_called()


def test_poll_assigned_returns_notifications(alerter_with_client):
    issue = _make_issue("PROJ-2", "Deploy hotfix")
    alerter_with_client._client._search.return_value = [issue]

    result = asyncio.run(alerter_with_client._poll_assigned(since=BASE_DT))

    assert len(result) == 1
    notif = result[0]
    assert notif["event_type"] == "assigned"
    assert notif["ticket_id"] == "PROJ-2"
    assert notif["title"] == "Deploy hotfix"


def test_poll_assigned_empty(alerter_with_client):
    alerter_with_client._client._search.return_value = []
    result = asyncio.run(alerter_with_client._poll_assigned(since=BASE_DT))
    assert result == []


# ---------------------------------------------------------------------------
# _poll_comments
# ---------------------------------------------------------------------------

def test_poll_comments_new_comment(alerter_with_client):
    issue = _make_issue("PROJ-3", "Add dark mode")
    alerter_with_client._client.get_my_issues.return_value = [issue]
    comment = _make_comment(author_email="bob@example.com", author_name="Bob",
                            body_text="LGTM", created=AFTER_DT.isoformat())
    alerter_with_client._client.get_issue_comments.return_value = [comment]

    result = asyncio.run(alerter_with_client._poll_comments(since=BASE_DT))

    assert len(result) == 1
    assert result[0]["event_type"] == "comment"
    assert "Bob" in result[0]["summary"]


def test_poll_comments_skips_own_comment(alerter_with_client):
    issue = _make_issue("PROJ-4", "Search")
    alerter_with_client._client.get_my_issues.return_value = [issue]
    # Comment authored by "me" — should be skipped
    comment = _make_comment(author_email="me@example.com",  # same as _me_email
                            created=AFTER_DT.isoformat())
    alerter_with_client._client.get_issue_comments.return_value = [comment]

    result = asyncio.run(alerter_with_client._poll_comments(since=BASE_DT))
    assert result == []


def test_poll_comments_skips_old_comments(alerter_with_client):
    issue = _make_issue("PROJ-5", "Old bug")
    alerter_with_client._client.get_my_issues.return_value = [issue]
    # Comment created BEFORE last_checked
    old_dt = (BASE_DT - timedelta(hours=1)).isoformat()
    comment = _make_comment(author_email="bob@example.com", created=old_dt)
    alerter_with_client._client.get_issue_comments.return_value = [comment]

    result = asyncio.run(alerter_with_client._poll_comments(since=BASE_DT))
    assert result == []


def test_poll_comments_no_issues(alerter_with_client):
    alerter_with_client._client.get_my_issues.return_value = []
    result = asyncio.run(alerter_with_client._poll_comments(since=BASE_DT))
    assert result == []


def test_poll_comments_comment_fetch_error_skips(alerter_with_client):
    issue = _make_issue("PROJ-6", "Crasher")
    alerter_with_client._client.get_my_issues.return_value = [issue]
    alerter_with_client._client.get_issue_comments.side_effect = RuntimeError("API error")

    result = asyncio.run(alerter_with_client._poll_comments(since=BASE_DT))
    assert result == []  # error swallowed; no crash


# ---------------------------------------------------------------------------
# _poll_status_changes
# ---------------------------------------------------------------------------

def test_poll_status_changes_no_since_skips(alerter_with_client):
    result = asyncio.run(alerter_with_client._poll_status_changes(since=None))
    assert result == []
    alerter_with_client._client._search.assert_not_called()


def test_poll_status_changes_returns_notification(alerter_with_client):
    issue = _make_issue("PROJ-7", "CI pipeline")
    alerter_with_client._client._search.return_value = [issue]
    entry = _make_changelog_entry("Open", "In Review", "Alice",
                                  created=AFTER_DT.isoformat())
    alerter_with_client._client.get_issue_changelog.return_value = [entry]

    result = asyncio.run(alerter_with_client._poll_status_changes(since=BASE_DT))

    assert len(result) == 1
    notif = result[0]
    assert notif["event_type"] == "status_change"
    assert "Open" in notif["summary"]
    assert "In Review" in notif["summary"]
    assert notif["raw"]["old_status"] == "Open"
    assert notif["raw"]["new_status"] == "In Review"


def test_poll_status_changes_skips_old_entries(alerter_with_client):
    issue = _make_issue("PROJ-8", "Cache issue")
    alerter_with_client._client._search.return_value = [issue]
    old_entry = _make_changelog_entry(
        created=(BASE_DT - timedelta(hours=1)).isoformat()
    )
    alerter_with_client._client.get_issue_changelog.return_value = [old_entry]

    result = asyncio.run(alerter_with_client._poll_status_changes(since=BASE_DT))
    assert result == []


def test_poll_status_changes_skips_non_status_fields(alerter_with_client):
    issue = _make_issue("PROJ-9", "Priority bump")
    alerter_with_client._client._search.return_value = [issue]
    entry = {
        "created": AFTER_DT.isoformat(),
        "author": {"displayName": "Alice"},
        "items": [{"field": "priority", "fromString": "Low", "toString": "High"}],
    }
    alerter_with_client._client.get_issue_changelog.return_value = [entry]

    result = asyncio.run(alerter_with_client._poll_status_changes(since=BASE_DT))
    assert result == []  # "priority" field ignored, only "status" matters


def test_poll_status_changes_changelog_error_skips(alerter_with_client):
    issue = _make_issue("PROJ-10", "Error issue")
    alerter_with_client._client._search.return_value = [issue]
    alerter_with_client._client.get_issue_changelog.side_effect = RuntimeError("fail")

    result = asyncio.run(alerter_with_client._poll_status_changes(since=BASE_DT))
    assert result == []


# ---------------------------------------------------------------------------
# poll — integration (all event types)
# ---------------------------------------------------------------------------

def test_poll_combines_all_events(alerter_with_client):
    """poll() aggregates assigned + comment + status_change events."""
    # assigned issue
    assigned_issue = _make_issue("PROJ-20", "Assign me")
    # comment issue
    comment_issue = _make_issue("PROJ-21", "Comment here")
    # status issue
    status_issue = _make_issue("PROJ-22", "Status change")

    comment = _make_comment(author_email="other@example.com",
                            created=AFTER_DT.isoformat())
    changelog_entry = _make_changelog_entry(created=AFTER_DT.isoformat())

    def _search_side_effect(jql):
        if "assignee changed" in jql:
            return [assigned_issue]
        if "status changed" in jql:
            return [status_issue]
        return []

    alerter_with_client._client._search.side_effect = _search_side_effect
    alerter_with_client._client.get_my_issues.return_value = [comment_issue]
    alerter_with_client._client.get_issue_comments.return_value = [comment]
    alerter_with_client._client.get_issue_changelog.return_value = [changelog_entry]

    with patch("backend.alerters.jira_alerter.cfg.get_bool", return_value=True):
        result = asyncio.run(alerter_with_client.poll(last_checked=BASE_DT))

    event_types = {n["event_type"] for n in result}
    assert "assigned" in event_types
    assert "comment" in event_types
    assert "status_change" in event_types


def test_poll_error_in_one_type_does_not_abort_others(alerter_with_client):
    """An error in _poll_assigned should not prevent comment/status polling."""
    comment_issue = _make_issue("PROJ-30", "Comments issue")
    comment = _make_comment(author_email="other@example.com",
                            created=AFTER_DT.isoformat())

    def bad_search(jql):
        if "assignee changed" in jql:
            raise RuntimeError("API down")
        return []  # status search returns empty

    alerter_with_client._client._search.side_effect = bad_search
    alerter_with_client._client.get_my_issues.return_value = [comment_issue]
    alerter_with_client._client.get_issue_comments.return_value = [comment]
    alerter_with_client._client.get_issue_changelog.return_value = []

    with patch("backend.alerters.jira_alerter.cfg.get_bool", return_value=True):
        result = asyncio.run(alerter_with_client.poll(last_checked=BASE_DT))

    # Should still get the comment notification
    assert any(n["event_type"] == "comment" for n in result)
