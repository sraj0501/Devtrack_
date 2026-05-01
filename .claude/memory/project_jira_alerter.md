---
name: Jira Alerter
description: Async Jira Cloud poller for assigned issues, new comments, and status changes — Track C of the Ticket Alerter feature
type: feature
---

# Jira Alerter (Track C)

**Shipped**: April 4, 2026
**Commits**: `a6aef67 test: 26 tests for JiraAlerter` + implementation commit

## Why

GitHub and Azure DevOps alerters (Tracks A/B) were shipped earlier. Track C adds Jira Cloud support so developers on Jira-based teams get the same OS/terminal notifications for ticket events.

## File

`backend/alerters/jira_alerter.py` — `JiraAlerter` class (async context manager)

## Event Types

| Event | Poll Method | JQL / API |
|---|---|---|
| `assigned` | `_poll_assigned` | `assignee changed to currentUser() AFTER "..."` |
| `comment` | `_poll_comments` | `get_my_issues` then per-issue `get_issue_comments` |
| `status_change` | `_poll_status_changes` | `assignee = currentUser() AND status changed AFTER "..."` + changelog |

### First-Run Guards

`_poll_assigned` and `_poll_status_changes` both return `[]` immediately when `last_checked` is `None` (prevents a firehose of stale events on first run). `_poll_comments` has no first-run guard (comments are filtered by creation time).

### Own-Comment Filtering

Comments authored by the local user are skipped. User identity is resolved from `JIRA_EMAIL` → `EMAIL` env var fallback.

## Configuration

```
ALERT_JIRA_ENABLED=true           # Default true; set false to disable entirely
JIRA_URL=https://yourorg.atlassian.net
JIRA_EMAIL=you@yourorg.com
JIRA_API_TOKEN=...

# Inherits shared alert flags:
ALERT_NOTIFY_ASSIGNED=true
ALERT_NOTIFY_COMMENTS=true
ALERT_NOTIFY_STATUS_CHANGES=true
```

## Usage Pattern

```python
async with JiraAlerter() as alerter:
    if alerter.is_configured():
        notifications = await alerter.poll(last_checked=last_dt)
```

Returns a list of notification dicts matching the MongoDB schema (source="jira", event_type, ticket_id, title, summary, url, timestamp, read=False, dismissed=False, raw).

## Tests

`backend/tests/test_jira_alerter.py` — 26 pytest tests, all mocked (no network):
- Disabled flag short-circuits
- Unconfigured client returns empty list
- Assigned: first-run guard, normal poll, dedup (no double-counting)
- Comments: new comment detected, own comment skipped, stale comment ignored
- Status change: first-run guard, old→new status emitted, stale entry skipped

## Integration Status

`JiraAlerter` is fully implemented and tested. It is NOT yet wired into `backend/alert_poller.py` — that integration (`_poll_jira()` method in the main poller) is the remaining step to make it live.

## How to apply

To wire it in: add a `_poll_jira` method to `AlertPoller` in `backend/alert_poller.py` mirroring `_poll_github` / `_poll_azure`, then call it in the main poll loop. The `alert_state` collection key should be `"jira:{user_email}"`.
