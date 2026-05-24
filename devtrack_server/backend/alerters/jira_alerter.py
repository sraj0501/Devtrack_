"""
Jira alert poller for DevTrack.

Polls Jira Cloud for events relevant to the authenticated user:
  - Issues newly assigned to me (since last_checked)
  - New comments on issues I am assigned to
  - Status changes on issues assigned to me

Uses backend/jira/client.py (JiraClient) for all API calls.
State tracking is delegated to MongoAlertsStore via the caller (alert_poller).
"""

from __future__ import annotations

import asyncio
import logging
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import backend.config as cfg

logger = logging.getLogger(__name__)


def _is_enabled() -> bool:
    return cfg.get_bool("ALERT_JIRA_ENABLED", True)


# ---------------------------------------------------------------------------
# Event builders
# ---------------------------------------------------------------------------

def _make_notification(
    event_type: str,
    ticket_id: str,
    title: str,
    summary: str,
    url: str,
    timestamp: datetime,
    raw: Dict[str, Any],
) -> Dict[str, Any]:
    if timestamp.tzinfo is None:
        timestamp = timestamp.replace(tzinfo=timezone.utc)
    return {
        "source": "jira",
        "event_type": event_type,
        "ticket_id": ticket_id,
        "title": title,
        "summary": summary,
        "url": url,
        "timestamp": timestamp,
        "read": False,
        "dismissed": False,
        "raw": raw,
    }


def _parse_dt(val: Optional[str]) -> Optional[datetime]:
    """Parse an ISO 8601 string from the Jira API into a timezone-aware datetime.

    Jira uses formats like ``2023-10-13T14:25:00.000+0000`` or
    ``2023-10-13T14:25:00.000+00:00``.
    """
    if not val:
        return None
    try:
        import re
        val = val.strip()
        if val.endswith("Z"):
            val = val[:-1] + "+00:00"
        # Normalise +0000 → +00:00
        val = re.sub(r"([+-])(\d{2})(\d{2})$", r"\1\2:\3", val)
        return datetime.fromisoformat(val)
    except Exception:
        return None


def _jql_date(dt: datetime) -> str:
    """Format a datetime for use in a Jira JQL ``AFTER`` / ``>=`` clause."""
    return dt.strftime("%Y-%m-%d %H:%M")


# ---------------------------------------------------------------------------
# Poller class
# ---------------------------------------------------------------------------

class JiraAlerter:
    """
    Polls Jira Cloud for events and returns new notification dicts.

    Usage::

        alerter = JiraAlerter()
        async with alerter:
            notifications = await alerter.poll(last_checked=dt)
    """

    def __init__(self) -> None:
        self._client = None
        self._me_email: Optional[str] = None
        self._me_name: Optional[str] = None

    async def __aenter__(self) -> "JiraAlerter":
        try:
            from backend.jira.client import JiraClient
            self._client = JiraClient()
        except ImportError as e:
            logger.warning(f"JiraClient not available: {e}")
        return self

    async def __aexit__(self, *args: Any) -> None:
        pass  # JiraClient is synchronous; no cleanup needed

    def is_configured(self) -> bool:
        return self._client is not None and self._client.is_configured()

    def _ensure_me(self) -> Optional[str]:
        """Return the configured JIRA_EMAIL (used to filter out own comments)."""
        if self._me_email:
            return self._me_email
        self._me_email = cfg.get("JIRA_EMAIL") or cfg.get("EMAIL") or ""
        return self._me_email or None

    async def poll(
        self,
        last_checked: Optional[datetime] = None,
    ) -> List[Dict[str, Any]]:
        """
        Poll all enabled Jira event types since ``last_checked``.

        Returns a list of notification dicts ready for insertion into MongoDB.
        """
        if not _is_enabled():
            logger.debug("Jira alerter disabled (ALERT_JIRA_ENABLED=false)")
            return []

        if not self.is_configured():
            logger.warning(
                "Jira alerter: client not configured. "
                "Set JIRA_URL, JIRA_EMAIL, JIRA_API_TOKEN in .env"
            )
            return []

        notifications: List[Dict[str, Any]] = []

        # --- Newly assigned issues ------------------------------------------
        if cfg.get_bool("ALERT_NOTIFY_ASSIGNED", True):
            try:
                assigned = await self._poll_assigned(last_checked)
                notifications.extend(assigned)
            except Exception as e:
                logger.warning(f"Jira alerter: error polling assigned issues: {e}")

        # --- Comments on my issues ------------------------------------------
        if cfg.get_bool("ALERT_NOTIFY_COMMENTS", True):
            try:
                comments = await self._poll_comments(last_checked)
                notifications.extend(comments)
            except Exception as e:
                logger.warning(f"Jira alerter: error polling comments: {e}")

        # --- Status changes on my issues ------------------------------------
        if cfg.get_bool("ALERT_NOTIFY_STATUS_CHANGES", True):
            try:
                state_changes = await self._poll_status_changes(last_checked)
                notifications.extend(state_changes)
            except Exception as e:
                logger.warning(f"Jira alerter: error polling status changes: {e}")

        logger.info(
            f"Jira alerter: found {len(notifications)} new events "
            f"(since {last_checked})"
        )
        return notifications

    # ------------------------------------------------------------------
    # Assigned issues
    # ------------------------------------------------------------------

    async def _poll_assigned(
        self,
        since: Optional[datetime],
    ) -> List[Dict[str, Any]]:
        """
        Detect issues newly assigned to the current user since ``since``.

        Uses JQL ``assignee changed to currentUser() AFTER "{since}"`` to find
        issues where the current user became the assignee after the last poll.
        First-run guard: skips silently when ``since`` is None.
        """
        if not since:
            return []

        jql = f'assignee changed to currentUser() AFTER "{_jql_date(since)}"'
        issues = await asyncio.to_thread(self._client._search, jql)

        results = []
        for issue in issues:
            notif = _make_notification(
                event_type="assigned",
                ticket_id=issue.key,
                title=issue.summary,
                summary=f"Assigned to you: {issue.key}",
                url=issue.url,
                timestamp=since,  # No better timestamp without changelog fetch
                raw={"key": issue.key, "status": issue.status},
            )
            results.append(notif)

        return results

    # ------------------------------------------------------------------
    # Comments
    # ------------------------------------------------------------------

    async def _poll_comments(
        self,
        since: Optional[datetime],
    ) -> List[Dict[str, Any]]:
        """
        Fetch new comments on issues assigned to the current user.

        Gets recently updated assigned issues, then checks each for comments
        created after ``since`` that were not authored by the current user.
        """
        me = self._ensure_me()
        issues = await asyncio.to_thread(
            self._client.get_my_issues, None, None, 50, since
        )

        results = []
        for issue in issues:
            try:
                comments = await asyncio.to_thread(
                    self._client.get_issue_comments, issue.key
                )
            except Exception as e:
                logger.warning(f"Jira alerter: error fetching comments for {issue.key}: {e}")
                continue

            for comment in comments:
                created_dt = _parse_dt(comment.get("created"))
                if since and (not created_dt or created_dt <= since):
                    continue

                author = comment.get("author", {})
                author_email = author.get("emailAddress", "")
                author_name = author.get("displayName", "unknown")

                # Skip own comments
                if me and me.lower() == author_email.lower():
                    continue

                body = (comment.get("body_text") or "")[:120].strip()
                summary = f"{author_name}: {body}" if author_name else body
                notif = _make_notification(
                    event_type="comment",
                    ticket_id=issue.key,
                    title=issue.summary,
                    summary=summary,
                    url=issue.url,
                    timestamp=created_dt or datetime.now(tz=timezone.utc),
                    raw={
                        "key": issue.key,
                        "comment_id": comment.get("id"),
                        "author_email": author_email,
                    },
                )
                results.append(notif)

        return results

    # ------------------------------------------------------------------
    # Status changes
    # ------------------------------------------------------------------

    async def _poll_status_changes(
        self,
        since: Optional[datetime],
    ) -> List[Dict[str, Any]]:
        """
        Detect status changes on issues assigned to the current user.

        Uses JQL ``assignee = currentUser() AND status changed AFTER "{since}"``
        then fetches the changelog to get old→new status details.
        First-run guard: skips silently when ``since`` is None.
        """
        if not since:
            return []

        jql = (
            f'assignee = currentUser() AND status changed AFTER "{_jql_date(since)}"'
        )
        issues = await asyncio.to_thread(self._client._search, jql)

        results = []
        for issue in issues:
            try:
                changelog = await asyncio.to_thread(
                    self._client.get_issue_changelog, issue.key
                )
            except Exception as e:
                logger.warning(
                    f"Jira alerter: error fetching changelog for {issue.key}: {e}"
                )
                continue

            for entry in changelog:
                entry_dt = _parse_dt(entry.get("created"))
                if not entry_dt or entry_dt <= since:
                    continue

                for item in entry.get("items", []):
                    if item.get("field") != "status":
                        continue

                    old_status = item.get("fromString", "")
                    new_status = item.get("toString", "")
                    if old_status == new_status:
                        continue

                    author_name = entry.get("author", {}).get("displayName", "")
                    summary = f"Status: {old_status} → {new_status}"
                    if author_name:
                        summary += f" (by {author_name})"

                    notif = _make_notification(
                        event_type="status_change",
                        ticket_id=issue.key,
                        title=issue.summary,
                        summary=summary,
                        url=issue.url,
                        timestamp=entry_dt,
                        raw={
                            "key": issue.key,
                            "old_status": old_status,
                            "new_status": new_status,
                            "changed_by": author_name,
                        },
                    )
                    results.append(notif)
                    break  # One status-change notification per changelog entry

        return results
