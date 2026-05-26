"""
Azure DevOps alert poller for DevTrack.

Polls Azure DevOps for events relevant to the authenticated user:
  - Work items newly assigned to me
  - New comments on work items I own/am assigned to
  - State changes on work items assigned to me

Uses backend/azure/client.py (AzureDevOpsClient) for base API calls.
State tracking is delegated to MongoAlertsStore via the caller (alert_poller).
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import backend.config as cfg

logger = logging.getLogger(__name__)


def _is_enabled() -> bool:
    return cfg.get_bool("ALERT_AZURE_ENABLED", True)


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
        "source": "azure",
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
    """Parse an ISO 8601 string from Azure DevOps API into a timezone-aware datetime."""
    if not val:
        return None
    try:
        # Azure uses format like "2024-01-15T10:30:00.000Z" or "+00:00"
        val = val.rstrip("Z")
        if "." in val:
            # Truncate microseconds to 6 digits max
            date_part, frac = val.rsplit(".", 1)
            frac = frac[:6]
            val = f"{date_part}.{frac}"
        return datetime.fromisoformat(val).replace(tzinfo=timezone.utc)
    except Exception:
        return None


def _display_name(val: Any) -> str:
    """Extract displayName from an Azure identity field (dict or string)."""
    if isinstance(val, dict):
        return val.get("displayName", "") or val.get("uniqueName", "")
    return str(val) if val else ""


# ---------------------------------------------------------------------------
# Poller class
# ---------------------------------------------------------------------------

class AzureAlerter:
    """
    Polls Azure DevOps for events and returns new notification dicts.

    Usage::

        alerter = AzureAlerter()
        async with alerter:
            notifications = await alerter.poll(last_checked=dt)
    """

    def __init__(self) -> None:
        self._client = None
        self._me: Optional[str] = None  # Current user display name or email

    async def __aenter__(self) -> "AzureAlerter":
        try:
            from backend.azure.client import AzureDevOpsClient
            self._client = AzureDevOpsClient()
        except ImportError as e:
            logger.warning(f"AzureDevOpsClient not available: {e}")
        return self

    async def __aexit__(self, *args: Any) -> None:
        if self._client:
            await self._client.close()

    def is_configured(self) -> bool:
        return self._client is not None and self._client.is_configured()

    async def _ensure_me(self) -> Optional[str]:
        """
        Determine the current user's display name or email for filtering.

        Priority:
        1. Cached value
        2. AZURE_EMAIL env var
        3. EMAIL env var
        4. Azure profile API (https://app.vssps.visualstudio.com/_apis/profile/profiles/me)
        """
        if self._me:
            return self._me
        me = cfg.get("AZURE_EMAIL") or cfg.get("EMAIL")
        if me:
            self._me = me
            return self._me
        try:
            profile_url = "https://app.vssps.visualstudio.com/_apis/profile/profiles/me"
            data = await self._client._get(profile_url, params={"api-version": "7.0"})
            if data:
                self._me = data.get("emailAddress") or data.get("displayName")
        except Exception as e:
            logger.debug(f"Could not fetch Azure profile: {e}")
        return self._me

    async def poll(
        self,
        last_checked: Optional[datetime] = None,
    ) -> List[Dict[str, Any]]:
        """
        Poll all enabled Azure DevOps event types since ``last_checked``.

        Returns a list of notification dicts ready for insertion into MongoDB.
        """
        if not _is_enabled():
            logger.debug("Azure alerter disabled (ALERT_AZURE_ENABLED=false)")
            return []

        if not self.is_configured():
            logger.warning(
                "Azure alerter: client not configured. "
                "Set AZURE_ORGANIZATION and AZURE_DEVOPS_PAT in .env"
            )
            return []

        notifications: List[Dict[str, Any]] = []
        me = await self._ensure_me()

        # Fetch work items assigned to me that changed since last_checked
        # (shared across all poll methods to avoid redundant WIQL calls)
        changed_items = []
        try:
            changed_items = await self._client.get_my_work_items(
                changed_after=last_checked
            )
        except Exception as e:
            logger.warning(f"Azure alerter: error fetching work items: {e}")
            return []

        # --- Newly assigned work items --------------------------------------
        if cfg.get_bool("ALERT_NOTIFY_ASSIGNED", True):
            try:
                assigned = await self._poll_assigned(last_checked, changed_items)
                notifications.extend(assigned)
            except Exception as e:
                logger.warning(f"Azure alerter: error polling assigned items: {e}")

        # --- Comments on my work items -------------------------------------
        if cfg.get_bool("ALERT_NOTIFY_COMMENTS", True):
            try:
                comments = await self._poll_comments(last_checked, changed_items, me)
                notifications.extend(comments)
            except Exception as e:
                logger.warning(f"Azure alerter: error polling comments: {e}")

        # --- State changes on my work items --------------------------------
        if cfg.get_bool("ALERT_NOTIFY_STATUS_CHANGES", True):
            try:
                state_changes = await self._poll_state_changes(
                    last_checked, changed_items, me
                )
                notifications.extend(state_changes)
            except Exception as e:
                logger.warning(f"Azure alerter: error polling state changes: {e}")

        logger.info(
            f"Azure alerter: found {len(notifications)} new events "
            f"(since {last_checked})"
        )
        return notifications

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    async def _get_item_updates(
        self,
        work_item_id: int,
        since: Optional[datetime],
    ) -> List[Dict[str, Any]]:
        """Fetch work item revision history, filtered to updates after ``since``."""
        url = self._client._org_url(f"_apis/wit/workitems/{work_item_id}/updates")
        data = await self._client._get(url)
        if not data:
            return []
        updates = data.get("value", [])
        if not since:
            return updates
        result = []
        for update in updates:
            revised = _parse_dt(update.get("revisedDate"))
            if revised and revised > since:
                result.append(update)
        return result

    async def _get_item_comments(
        self,
        work_item_id: int,
        since: Optional[datetime],
    ) -> List[Dict[str, Any]]:
        """Fetch comments for a work item, filtered to those after ``since``."""
        url = self._client._project_url(
            f"_apis/wit/workItems/{work_item_id}/comments"
        )
        data = await self._client._get(url, params={"api-version": "7.1-preview.3"})
        if not data:
            return []
        comments = data.get("comments", [])
        if not since:
            return comments
        result = []
        for comment in comments:
            created = _parse_dt(comment.get("createdDate"))
            if created and created > since:
                result.append(comment)
        return result

    # ------------------------------------------------------------------
    # Assigned work items
    # ------------------------------------------------------------------

    async def _poll_assigned(
        self,
        since: Optional[datetime],
        changed_items: list,
    ) -> List[Dict[str, Any]]:
        """
        Detect work items newly assigned to the current user since ``since``.

        Checks each recently-changed item's revision history for an AssignedTo
        field change targeting the current user.
        """
        results = []
        for item in changed_items:
            # If no since boundary, report items created recently (first-run guard)
            if not since:
                continue

            updates = await self._get_item_updates(item.id, since)
            for update in updates:
                fields = update.get("fields", {})
                if "System.AssignedTo" not in fields:
                    continue
                assignment = fields["System.AssignedTo"]
                new_val = assignment.get("newValue")
                new_name = _display_name(new_val)
                if not new_name:
                    continue

                revised_dt = _parse_dt(update.get("revisedDate"))
                ticket_id = f"{self._client._org}#{item.id}"
                summary = f"Assigned to {new_name}"
                notif = _make_notification(
                    event_type="assigned",
                    ticket_id=ticket_id,
                    title=item.title,
                    summary=summary,
                    url=item.url,
                    timestamp=revised_dt or datetime.now(tz=timezone.utc),
                    raw={
                        "work_item_id": item.id,
                        "assigned_to": new_name,
                        "state": item.state,
                    },
                )
                results.append(notif)
                break  # One "assigned" notification per item per poll cycle

        return results

    # ------------------------------------------------------------------
    # Comments
    # ------------------------------------------------------------------

    async def _poll_comments(
        self,
        since: Optional[datetime],
        changed_items: list,
        me: Optional[str],
    ) -> List[Dict[str, Any]]:
        """
        Fetch new comments on work items assigned to the current user.

        Skips comments authored by the current user (``me`` identity string).
        """
        results = []
        for item in changed_items:
            comments = await self._get_item_comments(item.id, since)
            for comment in comments:
                author_raw = comment.get("createdBy", {})
                author = _display_name(author_raw)

                # Skip own comments
                if me and me.lower() in (author.lower(), author_raw.get("uniqueName", "").lower()):
                    continue

                created_dt = _parse_dt(comment.get("createdDate"))
                text = (comment.get("text") or "")
                # Strip HTML tags (Azure comments can contain HTML)
                import re
                text = re.sub(r"<[^>]+>", "", text)[:120].strip()

                ticket_id = f"{self._client._org}#{item.id}"
                summary = f"{author}: {text}" if author else text
                notif = _make_notification(
                    event_type="comment",
                    ticket_id=ticket_id,
                    title=item.title,
                    summary=summary,
                    url=item.url,
                    timestamp=created_dt or datetime.now(tz=timezone.utc),
                    raw={
                        "work_item_id": item.id,
                        "comment_id": comment.get("id"),
                        "author": author,
                    },
                )
                results.append(notif)

        return results

    # ------------------------------------------------------------------
    # State changes
    # ------------------------------------------------------------------

    async def _poll_state_changes(
        self,
        since: Optional[datetime],
        changed_items: list,
        me: Optional[str],
    ) -> List[Dict[str, Any]]:
        """
        Detect state changes on work items assigned to the current user.

        Uses the work item updates API to find revisions where System.State
        changed after ``since``.
        """
        if not since:
            return []

        results = []
        for item in changed_items:
            updates = await self._get_item_updates(item.id, since)
            for update in updates:
                fields = update.get("fields", {})
                if "System.State" not in fields:
                    continue
                state_field = fields["System.State"]
                old_state = state_field.get("oldValue", "")
                new_state = state_field.get("newValue", "")
                if old_state == new_state:
                    continue

                # Who changed it?
                revised_by_raw = update.get("revisedBy", {})
                changed_by = _display_name(revised_by_raw)
                revised_dt = _parse_dt(update.get("revisedDate"))

                ticket_id = f"{self._client._org}#{item.id}"
                summary = f"State: {old_state} → {new_state}"
                if changed_by:
                    summary += f" (by {changed_by})"
                notif = _make_notification(
                    event_type="status_change",
                    ticket_id=ticket_id,
                    title=item.title,
                    summary=summary,
                    url=item.url,
                    timestamp=revised_dt or datetime.now(tz=timezone.utc),
                    raw={
                        "work_item_id": item.id,
                        "old_state": old_state,
                        "new_state": new_state,
                        "changed_by": changed_by,
                    },
                )
                results.append(notif)
                break  # One state-change notification per item per poll cycle

        return results
