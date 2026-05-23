"""
GitHub sync utilities.

Provides a simple sync layer that fetches issues from GitHub
and reports status. State is stored in Data/github/sync_state.json for
offline reference and audit trail.

No dependency on ProjectManager — designed to work standalone.
"""

import json
import logging
import os
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Dict, List, Optional

from backend.github.client import GitHubClient, GitHubIssue

logger = logging.getLogger(__name__)


def _state_file() -> Path:
    """Return path to local sync state file."""
    from backend.config import get_data_dir
    data_dir = get_data_dir() or "Data"
    path = Path(data_dir) / "github" / "sync_state.json"
    path.parent.mkdir(parents=True, exist_ok=True)
    return path


def _load_state() -> Dict:
    f = _state_file()
    if f.exists():
        try:
            return json.loads(f.read_text())
        except Exception:
            pass
    return {"last_sync": None, "issues": {}}


def _save_state(state: Dict) -> None:
    _state_file().write_text(json.dumps(state, indent=2, default=str))


# ---------------------------------------------------------------------------
# GitHubSync — simple fetch + report, no ProjectManager dependency
# ---------------------------------------------------------------------------

class GitHubSync:
    """Fetch GitHub issues and maintain a local sync state."""

    def __init__(self, client: Optional[GitHubClient] = None) -> None:
        self.client = client or GitHubClient()

    # -- public API ---------------------------------------------------------

    async def sync(self, full: bool = True, hours: Optional[int] = None) -> Dict:
        """Fetch issues and persist state.

        Args:
            full:  If True (default), clear existing cache and do a complete resync.
            hours: If set, only fetch issues updated in the last N hours and merge
                   into the existing cache. Overrides `full`.
        """
        updated_after: Optional[datetime] = None
        existing: Dict = {}

        if hours is not None:
            # Windowed sync — merge into existing cache
            updated_after = datetime.now(timezone.utc) - timedelta(hours=hours)
            existing = _load_state().get("issues", {})
            logger.info(f"GitHub sync: windowed mode, last {hours}h (since {updated_after.isoformat()})")
        elif full:
            # Full sync — start fresh
            logger.info("GitHub sync: full mode, clearing existing cache")
        else:
            # Incremental default: read window from env
            from backend.config import get_github_sync_window_hours
            window_hours = get_github_sync_window_hours()
            if window_hours > 0:
                updated_after = datetime.now(timezone.utc) - timedelta(hours=window_hours)
                existing = _load_state().get("issues", {})
                logger.info(f"GitHub sync: env window {window_hours}h")
            else:
                logger.info("GitHub sync: full mode (GITHUB_SYNC_WINDOW_HOURS=0)")

        items = await self.client.get_my_issues(
            state="open",
            updated_after=updated_after,
        )

        now = datetime.now(timezone.utc).isoformat()
        fetched_records = {
            str(issue.id): {
                "id": issue.id,
                "number": issue.number,
                "title": issue.title,
                "body": issue.body,
                "state": issue.state,
                "labels": issue.labels,
                "assignees": issue.assignees,
                "milestone_title": issue.milestone,
                "url": issue.html_url,
                "created_at": issue.created_at,
                "updated_at": issue.updated_at,
                "synced_at": now,
            }
            for issue in items
        }

        # Merge: fetched records overwrite existing ones
        merged = {**existing, **fetched_records}

        from backend.config import get_github_sync_window_hours as _gh_win
        mode = f"windowed:{hours}h" if hours else ("full" if not existing else f"windowed:{_gh_win()}h" if updated_after else "full")
        state = {
            "last_sync": now,
            "sync_mode": mode,
            "base_url": self.client._base_url,
            "repo": f"{self.client._owner}/{self.client._repo}",
            "issues": merged,
        }
        _save_state(state)
        logger.info(f"GitHub sync complete: {len(items)} fetched, {len(merged)} total in cache")

        by_state: Dict[str, List[GitHubIssue]] = {}
        for issue in items:
            by_state.setdefault(issue.state, []).append(issue)

        return {
            "total": len(merged),
            "fetched": len(items),
            "by_state": {s: len(v) for s, v in by_state.items()},
            "items": items,
            "last_sync": now,
            "state_file": str(_state_file()),
            "mode": mode,
        }

    async def get_issue(self, number: int) -> Optional[GitHubIssue]:
        """Fetch a single issue by its number."""
        return await self.client.get_issue(number)

    async def add_comment(self, number: int, body: str) -> bool:
        return await self.client.add_comment(number, body)

    async def close_issue(self, number: int) -> bool:
        return await self.client.close_issue(number)

    def get_cached(self) -> Dict:
        """Return locally cached state without hitting the API."""
        return _load_state()
