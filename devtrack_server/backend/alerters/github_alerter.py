"""
GitHub alert poller for DevTrack.

Polls GitHub for events relevant to the authenticated user:
  - Issues/PRs assigned to me (new assignments since last_checked)
  - Comments on issues/PRs where I am the author or assignee
  - Review requests on pull requests

Uses backend/github/client.py (GitHubClient) for all API calls.
State tracking is delegated to MongoAlertsStore via the caller (alert_poller).
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import backend.config as cfg

logger = logging.getLogger(__name__)


def _is_enabled() -> bool:
    return cfg.get_bool("ALERT_GITHUB_ENABLED", True)


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
        "source": "github",
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
    """Parse an ISO 8601 string from GitHub API into a timezone-aware datetime."""
    if not val:
        return None
    try:
        # GitHub uses Z suffix
        val = val.rstrip("Z") + "+00:00"
        return datetime.fromisoformat(val)
    except Exception:
        return None


# ---------------------------------------------------------------------------
# Poller class
# ---------------------------------------------------------------------------

class GitHubAlerter:
    """
    Polls GitHub for events and returns new notification dicts.

    Usage::

        alerter = GitHubAlerter()
        async with alerter:
            notifications = await alerter.poll(last_checked=dt)
    """

    def __init__(self) -> None:
        self._client = None
        self._login: Optional[str] = None

    async def __aenter__(self) -> "GitHubAlerter":
        try:
            from backend.github.client import GitHubClient
            self._client = GitHubClient()
        except ImportError as e:
            logger.warning(f"GitHubClient not available: {e}")
        return self

    async def __aexit__(self, *args: Any) -> None:
        if self._client:
            await self._client.close()

    def is_configured(self) -> bool:
        return self._client is not None and self._client.is_configured()

    async def _ensure_login(self) -> Optional[str]:
        if self._login:
            return self._login
        if not self._client:
            return None
        user = await self._client.get_current_user()
        if user:
            self._login = user.get("login")
        return self._login

    async def poll(
        self,
        last_checked: Optional[datetime] = None,
    ) -> List[Dict[str, Any]]:
        """
        Poll all enabled GitHub event types since ``last_checked``.

        Returns a list of notification dicts ready for insertion into MongoDB.
        """
        if not _is_enabled():
            logger.debug("GitHub alerter disabled (ALERT_GITHUB_ENABLED=false)")
            return []

        if not self.is_configured():
            logger.warning(
                "GitHub alerter: client not configured. "
                "Set GITHUB_TOKEN, GITHUB_OWNER, GITHUB_REPO in .env"
            )
            return []

        notifications: List[Dict[str, Any]] = []
        login = await self._ensure_login()
        if not login:
            logger.warning("GitHub alerter: could not determine current user login")
            return []

        owner = cfg.github_owner()
        repo = cfg.github_repo()

        # --- Assigned issues -------------------------------------------------
        if cfg.get_bool("ALERT_NOTIFY_ASSIGNED", True):
            try:
                assigned = await self._poll_assigned(last_checked, login, owner, repo)
                notifications.extend(assigned)
            except Exception as e:
                logger.warning(f"GitHub alerter: error polling assigned issues: {e}")

        # --- Comments on my issues/PRs --------------------------------------
        if cfg.get_bool("ALERT_NOTIFY_COMMENTS", True):
            try:
                comments = await self._poll_comments(last_checked, login, owner, repo)
                notifications.extend(comments)
            except Exception as e:
                logger.warning(f"GitHub alerter: error polling comments: {e}")

        # --- Review requests ------------------------------------------------
        if cfg.get_bool("ALERT_NOTIFY_REVIEW_REQUESTED", True):
            try:
                reviews = await self._poll_review_requests(
                    last_checked, login, owner, repo
                )
                notifications.extend(reviews)
            except Exception as e:
                logger.warning(f"GitHub alerter: error polling review requests: {e}")

        logger.info(
            f"GitHub alerter: found {len(notifications)} new events "
            f"(since {last_checked})"
        )
        return notifications

    # ------------------------------------------------------------------
    # Assigned issues
    # ------------------------------------------------------------------

    async def _poll_assigned(
        self,
        since: Optional[datetime],
        login: str,
        owner: str,
        repo: str,
    ) -> List[Dict[str, Any]]:
        """Fetch issues/PRs assigned to the authenticated user since ``since``."""
        issues = await self._client.get_my_issues(state="open", updated_after=since)
        results = []
        for issue in issues:
            created_dt = _parse_dt(issue.created_at)
            # Only report newly created (since last_checked), not just updated
            if since and created_dt and created_dt <= since:
                continue

            ticket_id = f"{owner}/{repo}#{issue.number}"
            is_pr = bool(issue.html_url and "/pull/" in issue.html_url)
            item_type = "PR" if is_pr else "Issue"
            summary = f"You were assigned to {item_type} #{issue.number}"
            notif = _make_notification(
                event_type="assigned",
                ticket_id=ticket_id,
                title=issue.title,
                summary=summary,
                url=issue.html_url,
                timestamp=created_dt or datetime.now(tz=timezone.utc),
                raw={"number": issue.number, "state": issue.state},
            )
            results.append(notif)
        return results

    # ------------------------------------------------------------------
    # Comments
    # ------------------------------------------------------------------

    async def _poll_comments(
        self,
        since: Optional[datetime],
        login: str,
        owner: str,
        repo: str,
    ) -> List[Dict[str, Any]]:
        """
        Fetch comments on issues where the authenticated user is involved.

        Uses GET /repos/{owner}/{repo}/issues/comments?since=...&sort=created
        to get all new comments in the repo, then filters for issues/PRs where
        the current user is author or assignee.
        """
        url = self._client._api(
            f"/repos/{owner}/{repo}/issues/comments"
        )
        params: Dict[str, Any] = {
            "sort": "created",
            "direction": "asc",
            "per_page": 100,
        }
        if since:
            params["since"] = since.isoformat().replace("+00:00", "Z")

        results = []
        while url:
            data, headers = await self._client._get_with_headers(url, params=params)
            if not isinstance(data, list):
                break
            for comment in data:
                # Skip comments made by the current user themselves
                commenter = (comment.get("user") or {}).get("login", "")
                if commenter == login:
                    continue

                # Check if the issue URL involves our user
                issue_url = comment.get("issue_url", "")
                if not issue_url:
                    continue

                # Fetch issue to check authorship/assignment
                issue_data = await self._client._get(issue_url)
                if not issue_data:
                    continue

                is_involved = (
                    (issue_data.get("user") or {}).get("login") == login
                    or any(
                        a.get("login") == login
                        for a in (issue_data.get("assignees") or [])
                    )
                )
                if not is_involved:
                    continue

                number = issue_data.get("number", 0)
                ticket_id = f"{owner}/{repo}#{number}"
                comment_body = (comment.get("body") or "")[:120]
                summary = f"{commenter}: {comment_body}"
                comment_dt = _parse_dt(comment.get("created_at"))
                notif = _make_notification(
                    event_type="comment",
                    ticket_id=ticket_id,
                    title=issue_data.get("title", f"Issue #{number}"),
                    summary=summary,
                    url=comment.get("html_url", ""),
                    timestamp=comment_dt or datetime.now(tz=timezone.utc),
                    raw={
                        "comment_id": comment.get("id"),
                        "commenter": commenter,
                        "number": number,
                    },
                )
                results.append(notif)

            url = self._client._parse_next_link(headers.get("Link", ""))
            params = {}

        return results

    # ------------------------------------------------------------------
    # Review requests
    # ------------------------------------------------------------------

    async def _poll_review_requests(
        self,
        since: Optional[datetime],
        login: str,
        owner: str,
        repo: str,
    ) -> List[Dict[str, Any]]:
        """
        Fetch open pull requests with review requested from the current user.

        Uses GET /repos/{owner}/{repo}/pulls?state=open and filters for PRs
        where the current user is in the requested_reviewers list.
        For delta tracking we check the ``created_at`` field.
        """
        url = self._client._api(f"/repos/{owner}/{repo}/pulls")
        params: Dict[str, Any] = {
            "state": "open",
            "per_page": 100,
        }

        results = []
        while url:
            data, headers = await self._client._get_with_headers(url, params=params)
            if not isinstance(data, list):
                break
            for pr in data:
                reviewers = [
                    r.get("login")
                    for r in (pr.get("requested_reviewers") or [])
                ]
                if login not in reviewers:
                    continue

                created_dt = _parse_dt(pr.get("created_at"))
                if since and created_dt and created_dt <= since:
                    continue

                number = pr.get("number", 0)
                ticket_id = f"{owner}/{repo}#{number}"
                author = (pr.get("user") or {}).get("login", "unknown")
                summary = f"Review requested by {author}"
                notif = _make_notification(
                    event_type="review_requested",
                    ticket_id=ticket_id,
                    title=pr.get("title", f"PR #{number}"),
                    summary=summary,
                    url=pr.get("html_url", ""),
                    timestamp=created_dt or datetime.now(tz=timezone.utc),
                    raw={"number": number, "author": author},
                )
                results.append(notif)

            url = self._client._parse_next_link(headers.get("Link", ""))
            params = {}

        return results
