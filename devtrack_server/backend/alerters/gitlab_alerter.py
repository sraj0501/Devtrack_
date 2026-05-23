"""
GitLab alert poller for DevTrack.

Polls GitLab for events relevant to the authenticated user:
  - Issues newly assigned to me (since last_checked)
  - New notes (comments) on issues I am assigned to
  - Merge requests where a review is requested from me

Uses backend/gitlab/client.py (GitLabClient) for all API calls.
State tracking is delegated to MongoAlertsStore via the caller (alert_poller).
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import backend.config as cfg

logger = logging.getLogger(__name__)


def _is_enabled() -> bool:
    return cfg.get_bool("ALERT_GITLAB_ENABLED", True)


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
        "source": "gitlab",
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
    """Parse an ISO 8601 string from the GitLab API into a timezone-aware datetime.

    GitLab uses formats like ``2023-10-13T14:25:00.000Z`` or
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


def _ticket_id_from_url(url: str, iid: int, prefix: str = "#") -> str:
    """
    Extract a short ticket ID from a GitLab web URL.

    e.g. ``https://gitlab.com/mygroup/myproject/-/issues/42``
         → ``mygroup/myproject#42``
    """
    try:
        # Strip scheme + host, take the namespace/project part
        path = url.split("/-/")[0]            # "https://host/ns/proj"
        path = path.split("//", 1)[-1]        # "host/ns/proj"
        path = "/".join(path.split("/")[1:])  # "ns/proj"
        if path:
            return f"{path}{prefix}{iid}"
    except Exception:
        pass
    return f"{prefix}{iid}"


# ---------------------------------------------------------------------------
# Poller class
# ---------------------------------------------------------------------------

class GitLabAlerter:
    """
    Polls GitLab for events and returns new notification dicts.

    Usage::

        alerter = GitLabAlerter()
        async with alerter:
            notifications = await alerter.poll(last_checked=dt)
    """

    def __init__(self) -> None:
        self._client = None
        self._user_id: Optional[int] = None
        self._username: Optional[str] = None

    async def __aenter__(self) -> "GitLabAlerter":
        try:
            from backend.gitlab.client import GitLabClient
            self._client = GitLabClient()
        except ImportError as e:
            logger.warning(f"GitLabClient not available: {e}")
        return self

    async def __aexit__(self, *args: Any) -> None:
        if self._client:
            await self._client.close()

    def is_configured(self) -> bool:
        return self._client is not None and self._client.is_configured()

    async def _ensure_user(self) -> bool:
        """Fetch and cache current user id and username. Returns True on success."""
        if self._user_id and self._username:
            return True
        if not self._client:
            return False
        user = await self._client.get_current_user()
        if not user:
            return False
        self._user_id = user.get("id")
        self._username = user.get("username", "")
        return bool(self._user_id)

    async def poll(
        self,
        last_checked: Optional[datetime] = None,
    ) -> List[Dict[str, Any]]:
        """
        Poll all enabled GitLab event types since ``last_checked``.

        Returns a list of notification dicts ready for insertion into MongoDB.
        """
        if not _is_enabled():
            logger.debug("GitLab alerter disabled (ALERT_GITLAB_ENABLED=false)")
            return []

        if not self.is_configured():
            logger.warning(
                "GitLab alerter: client not configured. "
                "Set GITLAB_PAT (and optionally GITLAB_URL) in .env"
            )
            return []

        if not await self._ensure_user():
            logger.warning("GitLab alerter: could not determine current user")
            return []

        notifications: List[Dict[str, Any]] = []

        # --- Newly assigned issues ------------------------------------------
        if cfg.get_bool("ALERT_NOTIFY_ASSIGNED", True):
            try:
                assigned = await self._poll_assigned(last_checked)
                notifications.extend(assigned)
            except Exception as e:
                logger.warning(f"GitLab alerter: error polling assigned issues: {e}")

        # --- Notes (comments) on my issues ----------------------------------
        if cfg.get_bool("ALERT_NOTIFY_COMMENTS", True):
            try:
                comments = await self._poll_comments(last_checked)
                notifications.extend(comments)
            except Exception as e:
                logger.warning(f"GitLab alerter: error polling comments: {e}")

        # --- MR review requests ---------------------------------------------
        if cfg.get_bool("ALERT_NOTIFY_REVIEW_REQUESTED", True):
            try:
                reviews = await self._poll_mr_review_requests(last_checked)
                notifications.extend(reviews)
            except Exception as e:
                logger.warning(
                    f"GitLab alerter: error polling MR review requests: {e}"
                )

        logger.info(
            f"GitLab alerter: found {len(notifications)} new events "
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

        Fetches open issues updated after ``since``, then filters to those
        whose ``created_at`` is after ``since`` (i.e. just created/assigned).
        First-run guard: skips silently when ``since`` is None.
        """
        if not since:
            return []

        issues = await self._client.get_my_issues(
            state="opened", updated_after=since
        )
        results = []
        for issue in issues:
            created_dt = _parse_dt(issue.created_at)
            if since and created_dt and created_dt <= since:
                continue

            ticket_id = _ticket_id_from_url(issue.url, issue.iid)
            notif = _make_notification(
                event_type="assigned",
                ticket_id=ticket_id,
                title=issue.title,
                summary=f"You were assigned to issue #{issue.iid}",
                url=issue.url,
                timestamp=created_dt or datetime.now(tz=timezone.utc),
                raw={"iid": issue.iid, "project_id": issue.project_id, "state": issue.state},
            )
            results.append(notif)

        return results

    # ------------------------------------------------------------------
    # Notes (comments)
    # ------------------------------------------------------------------

    async def _poll_comments(
        self,
        since: Optional[datetime],
    ) -> List[Dict[str, Any]]:
        """
        Fetch new notes on issues assigned to the current user.

        For each assigned open issue, retrieves notes created after ``since``
        that were not authored by the current user.
        """
        issues = await self._client.get_my_issues(
            state="opened", updated_after=since
        )

        results = []
        for issue in issues:
            try:
                notes = await self._fetch_issue_notes(
                    issue.project_id, issue.iid, since
                )
            except Exception as e:
                logger.warning(
                    f"GitLab alerter: error fetching notes for "
                    f"project {issue.project_id} issue #{issue.iid}: {e}"
                )
                continue

            ticket_id = _ticket_id_from_url(issue.url, issue.iid)
            for note in notes:
                author = (note.get("author") or {}).get("username", "")
                if author == self._username:
                    continue  # skip own notes

                body = (note.get("body") or "")[:120].strip()
                summary = f"{author}: {body}" if author else body
                note_dt = _parse_dt(note.get("created_at"))

                notif = _make_notification(
                    event_type="comment",
                    ticket_id=ticket_id,
                    title=issue.title,
                    summary=summary,
                    url=note.get("url") or issue.url,
                    timestamp=note_dt or datetime.now(tz=timezone.utc),
                    raw={
                        "note_id": note.get("id"),
                        "iid": issue.iid,
                        "project_id": issue.project_id,
                        "author": author,
                    },
                )
                results.append(notif)

        return results

    async def _fetch_issue_notes(
        self,
        project_id: int,
        issue_iid: int,
        since: Optional[datetime],
    ) -> List[Dict[str, Any]]:
        """
        GET /projects/:id/issues/:iid/notes?sort=asc&order_by=created_at

        Returns note dicts created after ``since``.
        """
        url = self._client._api(
            f"projects/{project_id}/issues/{issue_iid}/notes"
        )
        params: Dict[str, Any] = {
            "sort": "asc",
            "order_by": "created_at",
            "per_page": 100,
        }

        notes = []
        while url:
            data, headers = await self._client._get_with_headers(url, params=params)
            if not isinstance(data, list):
                break
            for note in data:
                if note.get("system"):
                    continue  # skip system notes (e.g. "assigned to X")
                note_dt = _parse_dt(note.get("created_at"))
                if since and note_dt and note_dt <= since:
                    continue
                notes.append(note)
            next_page = headers.get("X-Next-Page", "")
            if not next_page:
                break
            params = {"page": int(next_page), "per_page": 100}

        return notes

    # ------------------------------------------------------------------
    # MR review requests
    # ------------------------------------------------------------------

    async def _poll_mr_review_requests(
        self,
        since: Optional[datetime],
    ) -> List[Dict[str, Any]]:
        """
        Fetch open merge requests where the current user is a reviewer.

        Uses GET /merge_requests?reviewer_id=<user_id>&state=opened
        Filters to MRs whose ``created_at`` is after ``since``.
        """
        if not since:
            return []

        url = self._client._api("merge_requests")
        params: Dict[str, Any] = {
            "reviewer_id": self._user_id,
            "state": "opened",
            "per_page": 100,
        }
        if since:
            params["updated_after"] = since.isoformat()

        results = []
        while url:
            data, headers = await self._client._get_with_headers(url, params=params)
            if not isinstance(data, list):
                break
            for mr in data:
                created_dt = _parse_dt(mr.get("created_at"))
                if since and created_dt and created_dt <= since:
                    continue

                iid = mr.get("iid", 0)
                mr_url = mr.get("web_url", "")
                ticket_id = _ticket_id_from_url(mr_url, iid, prefix="!")
                author = (mr.get("author") or {}).get("username", "unknown")
                summary = f"Review requested by {author}"

                notif = _make_notification(
                    event_type="review_requested",
                    ticket_id=ticket_id,
                    title=mr.get("title", f"MR !{iid}"),
                    summary=summary,
                    url=mr_url,
                    timestamp=created_dt or datetime.now(tz=timezone.utc),
                    raw={
                        "iid": iid,
                        "project_id": mr.get("project_id"),
                        "author": author,
                    },
                )
                results.append(notif)

            next_page = headers.get("X-Next-Page", "")
            if not next_page:
                break
            params = {
                "reviewer_id": self._user_id,
                "state": "opened",
                "per_page": 100,
                "page": int(next_page),
            }

        return results
