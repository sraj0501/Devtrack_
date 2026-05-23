"""
GitLab sync utilities.

Provides a simple sync layer that fetches issues from GitLab
and reports status. State is persisted in SQLite (devtrack.db) via
backend.db.platform_store.

No dependency on ProjectManager — designed to work standalone.
"""

import logging
import os
from datetime import datetime, timedelta, timezone
from typing import Dict, List, Optional

from backend.gitlab.client import GitLabClient, GitLabIssue
from backend.db.platform_store import (
    clear_sync_items,
    load_sync_items,
    load_sync_meta,
    save_sync_items,
)

logger = logging.getLogger(__name__)

_PLATFORM = "gitlab"


# ---------------------------------------------------------------------------
# GitLabSync — simple fetch + report, no ProjectManager dependency
# ---------------------------------------------------------------------------

class GitLabSync:
    """Fetch GitLab issues and maintain a local sync state."""

    def __init__(self, client: Optional[GitLabClient] = None) -> None:
        self.client = client or GitLabClient()

    # -- public API ---------------------------------------------------------

    async def sync(self, full: bool = True, hours: Optional[int] = None) -> Dict:
        """Fetch issues and persist state.

        Args:
            full:  If True (default), clear existing cache and do a complete resync.
            hours: If set, only fetch issues updated in the last N hours and merge
                   into the existing cache. Overrides `full`.
        """
        changed_after: Optional[datetime] = None
        existing: Dict = {}

        if hours is not None:
            # Windowed sync — merge into existing cache
            changed_after = datetime.now(timezone.utc) - timedelta(hours=hours)
            existing = load_sync_items(_PLATFORM)
            logger.info(f"GitLab sync: windowed mode, last {hours}h (since {changed_after.isoformat()})")
        elif full:
            # Full sync — start fresh
            logger.info("GitLab sync: full mode, clearing existing cache")
            clear_sync_items(_PLATFORM)
        else:
            # Incremental default: read window from env
            from backend.config import get_gitlab_sync_window_hours
            window_hours = get_gitlab_sync_window_hours()
            if window_hours > 0:
                changed_after = datetime.now(timezone.utc) - timedelta(hours=window_hours)
                existing = load_sync_items(_PLATFORM)
                logger.info(f"GitLab sync: env window {window_hours}h")
            else:
                logger.info("GitLab sync: full mode (GITLAB_SYNC_WINDOW_HOURS=0)")
                clear_sync_items(_PLATFORM)

        items = await self.client.get_my_issues(
            state="opened",
            max_results=200,
            updated_after=changed_after,
        )

        now = datetime.now(timezone.utc).isoformat()
        fetched_records = {
            str(issue.id): {
                "id": issue.id,
                "iid": issue.iid,
                "project_id": issue.project_id,
                "title": issue.title,
                "description": issue.description,
                "state": issue.state,
                "labels": issue.labels,
                "assignee": issue.assignee,
                "milestone_title": issue.milestone_title,
                "milestone_id": issue.milestone_id,
                "due_date": issue.due_date,
                "url": issue.url,
                "created_at": issue.created_at,
                "updated_at": issue.updated_at,
                "synced_at": now,
            }
            for issue in items
        }

        # Merge: fetched records overwrite existing ones
        merged = {**existing, **fetched_records}

        from backend.config import get_gitlab_sync_window_hours as _gl_win
        mode = f"windowed:{hours}h" if hours else ("full" if not existing else f"windowed:{_gl_win()}h" if changed_after else "full")
        save_sync_items(_PLATFORM, merged, now)
        logger.info(f"GitLab sync complete: {len(items)} fetched, {len(merged)} total in cache")

        by_state: Dict[str, List[GitLabIssue]] = {}
        for issue in items:
            by_state.setdefault(issue.state, []).append(issue)

        return {
            "total": len(merged),
            "fetched": len(items),
            "by_state": {s: len(v) for s, v in by_state.items()},
            "items": items,
            "last_sync": now,
            "mode": mode,
        }

    async def get_issue(self, project_id: int, issue_iid: int) -> Optional[GitLabIssue]:
        """Fetch a single issue by project ID and project-local iid."""
        return await self.client.get_issue(project_id, issue_iid)

    async def add_comment(self, project_id: int, issue_iid: int, body: str) -> bool:
        return await self.client.add_comment(project_id, issue_iid, body)

    async def close_issue(self, project_id: int, issue_iid: int) -> bool:
        return await self.client.close_issue(project_id, issue_iid)

    def get_cached(self) -> Dict:
        """Return locally cached state without hitting the API."""
        return {
            "last_sync": load_sync_meta(_PLATFORM),
            "issues": load_sync_items(_PLATFORM),
        }
