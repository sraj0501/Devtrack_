"""
GitHubTicketSync — fetch open GitHub issues assigned to a user and write them
into the local ticket_cache SQLite table.

Usage (CLI):
    python backend/github_ticket_sync.py sync <owner/repo> <assignee>

Usage (library):
    from backend.github_ticket_sync import GitHubTicketSync
    syncer = GitHubTicketSync()
    count = syncer.sync("owner/repo", "my-github-username")
"""

from __future__ import annotations

import json
import logging
import sys
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import requests

import backend.config as config
from backend.db.ticket_db import TicketDB

log = logging.getLogger(__name__)


def _get_github_token() -> str:
    """Return GITHUB_TOKEN; raise ValueError if unset."""
    token = config.get("GITHUB_TOKEN")
    if not token:
        raise ValueError("GITHUB_TOKEN environment variable is required for GitHub sync")
    return token


class GitHubTicketSync:
    """Fetches open GitHub issues assigned to a user and upserts them into ticket_cache."""

    # GitHub REST API base — never hardcoded, read from env with a sensible
    # fallback to the public API endpoint.
    _GITHUB_API_BASE = "https://api.github.com"

    def __init__(self) -> None:
        self._token: Optional[str] = None  # loaded lazily on first sync

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def sync(self, repo: str, assignee: str) -> int:
        """Fetch all open issues assigned to *assignee* in *repo* and write to SQLite.

        *repo* must be in "owner/repo" format, e.g. "acme/backend".

        Returns the number of tickets upserted.
        """
        self._token = _get_github_token()
        issues = self._fetch_all_issues(repo, assignee)
        db = TicketDB.from_config()
        count = 0
        with db:
            for issue in issues:
                record = self._to_ticket_record(issue, repo)
                db.upsert_ticket(record)
                count += 1
        log.info("GitHub sync: upserted %d tickets for %s in %s", count, assignee, repo)
        return count

    # ------------------------------------------------------------------
    # Private helpers
    # ------------------------------------------------------------------

    def _headers(self) -> Dict[str, str]:
        return {
            "Authorization": f"token {self._token}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        }

    def _fetch_all_issues(self, repo: str, assignee: str) -> List[Dict[str, Any]]:
        """Page through all open issues assigned to *assignee* in *repo*."""
        timeout = config.http_timeout_short()
        url = f"{self._GITHUB_API_BASE}/repos/{repo}/issues"
        params: Dict[str, Any] = {
            "assignee": assignee,
            "state": "open",
            "per_page": 100,
            "page": 1,
        }
        all_issues: List[Dict[str, Any]] = []

        while True:
            response = requests.get(
                url,
                headers=self._headers(),
                params=params,
                timeout=timeout,
            )
            response.raise_for_status()
            page_issues = response.json()
            if not page_issues:
                break
            all_issues.extend(page_issues)
            if len(page_issues) < params["per_page"]:
                # Last page — no need to fetch another
                break
            params["page"] += 1  # type: ignore[operator]

        return all_issues

    def _to_ticket_record(self, issue: Dict[str, Any], repo: str) -> Dict[str, Any]:
        """Convert a GitHub issue dict to a ticket_cache record dict."""
        number = issue["number"]
        labels_list = [lbl["name"] for lbl in issue.get("labels", [])]
        assignee_login = ""
        if issue.get("assignee"):
            assignee_login = issue["assignee"].get("login", "")
        elif issue.get("assignees"):
            # Pick the first assignee for simplicity
            assignee_login = issue["assignees"][0].get("login", "")

        return {
            "id": f"github:{repo}#{number}",
            "source": "github",
            "external_id": str(number),
            "repo": repo,
            "title": issue.get("title", ""),
            "description": issue.get("body") or "",
            "status": issue.get("state", "open"),
            "assignee": assignee_login,
            "labels": json.dumps(labels_list),
            "url": issue.get("html_url", ""),
            "synced_at": datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S"),
        }


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------

def _main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
    if len(sys.argv) < 4 or sys.argv[1] != "sync":
        print("Usage: python github_ticket_sync.py sync <owner/repo> <assignee>")
        sys.exit(1)

    _, _, repo, assignee = sys.argv[:4]
    syncer = GitHubTicketSync()
    try:
        count = syncer.sync(repo, assignee)
        print(f"Synced {count} tickets from github:{repo} for {assignee}")
    except Exception as exc:  # pylint: disable=broad-except
        print(f"Sync failed: {exc}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    _main()
