"""
GitLab REST API client (async).

Provides issue management, milestone (sprint) listing, project access,
and assignment polling. Uses aiohttp for all HTTP calls.

Configuration (via .env):
    GITLAB_URL        = https://gitlab.com  (or self-hosted base URL)
    GITLAB_PAT        = your_personal_access_token
    GITLAB_PROJECT_ID = 12345  (optional; used for project-scoped ops)
"""

import logging
import os
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

import aiohttp

logger = logging.getLogger(__name__)


def _load_env() -> None:
    here = Path(__file__).resolve()
    for parent in [here, *here.parents]:
        env_file = parent / ".env"
        if env_file.exists():
            try:
                from dotenv import load_dotenv
                load_dotenv(env_file, override=False)
            except ImportError:
                pass
            return


_load_env()


def _env(key: str, default: str = "") -> str:
    from backend.config import get
    return get(key, default)


def _env_int(key: str, default: int = 0) -> int:
    from backend.config import get_int
    return get_int(key, default)


# ---------------------------------------------------------------------------
# Data model
# ---------------------------------------------------------------------------

@dataclass
class GitLabIssue:
    """Represents a GitLab issue."""

    id: int                          # global issue id (stable, use for seen-sets)
    iid: int                         # project-local issue number (#42 in the UI)
    project_id: int
    title: str
    description: str
    state: str                       # "opened" | "closed"
    labels: List[str] = field(default_factory=list)
    assignee: str = ""               # display name of first assignee
    milestone_title: str = ""        # empty if no milestone
    milestone_id: Optional[int] = None
    due_date: Optional[str] = None   # "YYYY-MM-DD" or None
    url: str = ""
    created_at: str = ""
    updated_at: str = ""


# ---------------------------------------------------------------------------
# Client
# ---------------------------------------------------------------------------

class GitLabClient:
    """Async GitLab REST API client."""

    def __init__(
        self,
        base_url: Optional[str] = None,
        pat: Optional[str] = None,
        project_id: Optional[int] = None,
    ):
        raw_url = base_url or _env("GITLAB_URL", "https://gitlab.com")
        self._base_url = raw_url.rstrip("/")
        self._pat = pat or _env("GITLAB_PAT") or _env("GITLAB_API_KEY")
        self._project_id = project_id or _env_int("GITLAB_PROJECT_ID")
        self._timeout_secs = _env_int("HTTP_TIMEOUT", 30)
        self._session: Optional[aiohttp.ClientSession] = None
        self._user_id: Optional[int] = None  # cached after first /user call

    # -- lifecycle ----------------------------------------------------------

    def is_configured(self) -> bool:
        return bool(self._pat)

    async def _get_session(self) -> aiohttp.ClientSession:
        if self._session is None or self._session.closed:
            headers = {"Private-Token": self._pat, "Content-Type": "application/json"}
            timeout = aiohttp.ClientTimeout(total=self._timeout_secs)
            self._session = aiohttp.ClientSession(headers=headers, timeout=timeout)
        return self._session

    async def close(self) -> None:
        if self._session and not self._session.closed:
            await self._session.close()

    # -- URL helpers --------------------------------------------------------

    def _api(self, path: str) -> str:
        return f"{self._base_url}/api/v4/{path}"

    # -- low-level HTTP -----------------------------------------------------

    async def _get(self, url: str, params: Optional[Dict[str, Any]] = None) -> Optional[Any]:
        """GET request; returns parsed JSON or None on error."""
        session = await self._get_session()
        try:
            async with session.get(url, params=params) as resp:
                if resp.status == 200:
                    return await resp.json()
                logger.warning(f"GET {url} → {resp.status}: {await resp.text()}")
                return None
        except Exception as e:
            logger.warning(f"GET {url} failed: {e}")
            return None

    async def _get_with_headers(self, url: str, params: Optional[Dict] = None):
        """GET request; returns (json_body, response_headers) for pagination."""
        session = await self._get_session()
        try:
            async with session.get(url, params=params) as resp:
                if resp.status == 200:
                    return await resp.json(), dict(resp.headers)
                logger.warning(f"GET {url} → {resp.status}")
                return None, {}
        except Exception as e:
            logger.warning(f"GET {url} failed: {e}")
            return None, {}

    async def _post(self, url: str, json_body: Any = None) -> Optional[Dict]:
        session = await self._get_session()
        try:
            async with session.post(url, json=json_body) as resp:
                if resp.status in (200, 201):
                    return await resp.json()
                logger.warning(f"POST {url} → {resp.status}: {await resp.text()}")
                return None
        except Exception as e:
            logger.warning(f"POST {url} failed: {e}")
            return None

    async def _put(self, url: str, json_body: Any = None) -> Optional[Dict]:
        session = await self._get_session()
        try:
            async with session.put(url, json=json_body) as resp:
                if resp.status == 200:
                    return await resp.json()
                logger.warning(f"PUT {url} → {resp.status}: {await resp.text()}")
                return None
        except Exception as e:
            logger.warning(f"PUT {url} failed: {e}")
            return None

    # -- parsing ------------------------------------------------------------

    @staticmethod
    def _parse_issue(data: Dict[str, Any]) -> GitLabIssue:
        assignee_data = data.get("assignee") or {}
        assignee = assignee_data.get("name", "") if isinstance(assignee_data, dict) else ""

        milestone = data.get("milestone") or {}
        milestone_title = milestone.get("title", "") if isinstance(milestone, dict) else ""
        milestone_id = milestone.get("id") if isinstance(milestone, dict) else None

        labels = data.get("labels", [])

        return GitLabIssue(
            id=data.get("id", 0),
            iid=data.get("iid", 0),
            project_id=data.get("project_id", 0),
            title=data.get("title", ""),
            description=data.get("description", "") or "",
            state=data.get("state", ""),
            labels=labels if isinstance(labels, list) else [],
            assignee=assignee,
            milestone_title=milestone_title,
            milestone_id=milestone_id,
            due_date=data.get("due_date"),
            url=data.get("web_url", ""),
            created_at=data.get("created_at", ""),
            updated_at=data.get("updated_at", ""),
        )

    # -- read operations ----------------------------------------------------

    async def get_current_user(self) -> Optional[Dict[str, Any]]:
        """GET /user — returns id, username, name."""
        return await self._get(self._api("user"))

    async def _ensure_user_id(self) -> Optional[int]:
        """Fetch and cache current user's numeric ID."""
        if self._user_id is None:
            user = await self.get_current_user()
            if user:
                self._user_id = user.get("id")
        return self._user_id

    async def get_my_issues(
        self,
        state: str = "opened",
        max_results: int = 100,
        updated_after: Optional[datetime] = None,
    ) -> List[GitLabIssue]:
        """Fetch issues assigned to the current user, paginated."""
        user_id = await self._ensure_user_id()
        if not user_id:
            logger.warning("Could not determine current user ID")
            return []

        params: Dict[str, Any] = {
            "assignee_id": user_id,
            "state": state,
            "per_page": 100,
            "page": 1,
        }
        if updated_after:
            params["updated_after"] = updated_after.isoformat()

        results: List[GitLabIssue] = []
        while len(results) < max_results:
            data, headers = await self._get_with_headers(self._api("issues"), params=params)
            if not data:
                break
            for item in data:
                results.append(self._parse_issue(item))
                if len(results) >= max_results:
                    break
            next_page = headers.get("X-Next-Page", "")
            if not next_page:
                break
            params["page"] = int(next_page)

        return results

    async def get_issue(self, project_id: int, issue_iid: int) -> Optional[GitLabIssue]:
        """Fetch a single issue by project ID and project-local iid."""
        url = self._api(f"projects/{project_id}/issues/{issue_iid}")
        data = await self._get(url)
        return self._parse_issue(data) if data else None

    async def get_issue_by_global_id(self, issue_id: int) -> Optional[GitLabIssue]:
        """Fetch a single issue by global ID."""
        data = await self._get(self._api(f"issues/{issue_id}"))
        return self._parse_issue(data) if data else None

    async def get_projects(self, membership: bool = True) -> List[Dict[str, Any]]:
        """List projects the user is a member of."""
        data = await self._get(
            self._api("projects"),
            params={"membership": "true" if membership else "false", "per_page": 50},
        )
        return data if isinstance(data, list) else []

    async def get_milestones(self, project_id: Optional[int] = None) -> List[Dict[str, Any]]:
        """List milestones for a project (active milestones first)."""
        pid = project_id or self._project_id
        if not pid:
            return []
        data = await self._get(
            self._api(f"projects/{pid}/milestones"),
            params={"state": "active", "per_page": 50},
        )
        return data if isinstance(data, list) else []

    async def search_issues(self, query: str, max_results: int = 20) -> List[GitLabIssue]:
        """Search issues by title text, scoped to current user assignments."""
        params = {"search": query, "scope": "assigned_to_me", "per_page": max_results}
        data = await self._get(self._api("issues"), params=params)
        if not isinstance(data, list):
            return []
        return [self._parse_issue(item) for item in data]

    # -- write operations ---------------------------------------------------

    async def create_issue(
        self,
        title: str,
        description: str = "",
        project_id: Optional[int] = None,
        labels: Optional[List[str]] = None,
        milestone_id: Optional[int] = None,
        due_date: Optional[str] = None,
        assignee_ids: Optional[List[int]] = None,
    ) -> Optional[GitLabIssue]:
        """POST /projects/:id/issues"""
        pid = project_id or self._project_id
        if not pid:
            logger.error("create_issue requires GITLAB_PROJECT_ID or project_id arg")
            return None

        body: Dict[str, Any] = {"title": title}
        if description:
            body["description"] = description
        if labels:
            body["labels"] = ",".join(labels)
        if milestone_id:
            body["milestone_id"] = milestone_id
        if due_date:
            body["due_date"] = due_date
        if assignee_ids:
            body["assignee_ids"] = assignee_ids

        data = await self._post(self._api(f"projects/{pid}/issues"), json_body=body)
        if data:
            issue = self._parse_issue(data)
            logger.info(f"Created GitLab issue #{issue.iid}: {title}")
            return issue
        return None

    async def update_issue(
        self,
        project_id: int,
        issue_iid: int,
        fields: Dict[str, Any],
    ) -> bool:
        """PUT /projects/:id/issues/:iid — update arbitrary fields."""
        url = self._api(f"projects/{project_id}/issues/{issue_iid}")
        result = await self._put(url, json_body=fields)
        if result:
            logger.info(f"Updated GitLab issue !{issue_iid} in project {project_id}")
            return True
        return False

    async def close_issue(self, project_id: int, issue_iid: int) -> bool:
        return await self.update_issue(project_id, issue_iid, {"state_event": "close"})

    async def reopen_issue(self, project_id: int, issue_iid: int) -> bool:
        return await self.update_issue(project_id, issue_iid, {"state_event": "reopen"})

    async def add_comment(self, project_id: int, issue_iid: int, body: str) -> bool:
        """POST /projects/:id/issues/:iid/notes"""
        url = self._api(f"projects/{project_id}/issues/{issue_iid}/notes")
        result = await self._post(url, json_body={"body": body})
        if result:
            logger.info(f"Added comment to GitLab issue !{issue_iid}")
            return True
        return False

    # -- team & capacity operations (project planning) ----------------------

    async def list_project_members(
        self,
        project_id: Optional[int] = None,
        include_inherited: bool = True,
    ) -> List[Dict[str, Any]]:
        """List all members of a GitLab project.

        Returns list of dicts with id, username, name, state, access_level.
        Uses GET /projects/{id}/members/all (includes inherited group members).
        """
        pid = project_id or self._project_id
        if not pid:
            logger.warning("list_project_members: no project_id configured")
            return []

        endpoint = "members/all" if include_inherited else "members"
        results: List[Dict[str, Any]] = []
        page = 1
        while True:
            data, headers = await self._get_with_headers(
                self._api(f"projects/{pid}/{endpoint}"),
                params={"per_page": 100, "page": page},
            )
            if not isinstance(data, list) or not data:
                break
            for m in data:
                results.append({
                    "id": m.get("id", 0),
                    "username": m.get("username", ""),
                    "name": m.get("name", ""),
                    "state": m.get("state", ""),
                    "access_level": m.get("access_level", 0),
                })
            next_page = headers.get("X-Next-Page", "")
            if not next_page:
                break
            page = int(next_page)
        return results

    async def get_issues_for_user(
        self,
        username: str,
        state: str = "opened",
        project_id: Optional[int] = None,
        max_results: int = 100,
    ) -> List[GitLabIssue]:
        """Fetch open issues assigned to a specific user.

        Uses GET /projects/{id}/issues?assignee_username={X}&state=opened.
        Falls back to instance-level issues API if no project_id.
        """
        pid = project_id or self._project_id
        if pid:
            endpoint = f"projects/{pid}/issues"
        else:
            endpoint = "issues"

        params: Dict[str, Any] = {
            "assignee_username": username,
            "state": state,
            "per_page": 100,
            "page": 1,
        }
        results: List[GitLabIssue] = []
        while len(results) < max_results:
            data, headers = await self._get_with_headers(self._api(endpoint), params=params)
            if not isinstance(data, list) or not data:
                break
            for item in data:
                results.append(self._parse_issue(item))
                if len(results) >= max_results:
                    break
            next_page = headers.get("X-Next-Page", "")
            if not next_page:
                break
            params["page"] = int(next_page)
        return results

    async def create_milestone(
        self,
        title: str,
        due_date: Optional[str] = None,
        start_date: Optional[str] = None,
        description: str = "",
        project_id: Optional[int] = None,
    ) -> Optional[Dict[str, Any]]:
        """POST /projects/:id/milestones — create a sprint milestone.

        Args:
            title: Milestone title (e.g. "Sprint 1").
            due_date: "YYYY-MM-DD" end date.
            start_date: "YYYY-MM-DD" start date.
            description: Optional description.
            project_id: Project ID override.

        Returns the created milestone dict or None on failure.
        """
        pid = project_id or self._project_id
        if not pid:
            logger.error("create_milestone requires GITLAB_PROJECT_ID or project_id arg")
            return None

        body: Dict[str, Any] = {"title": title}
        if description:
            body["description"] = description
        if due_date:
            body["due_date"] = due_date
        if start_date:
            body["start_date"] = start_date

        data = await self._post(self._api(f"projects/{pid}/milestones"), json_body=body)
        if data:
            logger.info(f"Created GitLab milestone '{title}' (#{data.get('id')})")
        return data

    async def get_user_by_username(self, username: str) -> Optional[Dict[str, Any]]:
        """GET /users?username={username} — lookup user profile."""
        data = await self._get(self._api("users"), params={"username": username})
        if isinstance(data, list) and data:
            return data[0]
        return None

    # -- webhook management -------------------------------------------------

    async def ensure_webhook_current(
        self,
        project_id: int,
        webhook_url: str,
        secret: str = "",
    ) -> bool:
        """
        Ensure a project webhook pointing at *webhook_url* is registered.

        Idempotent: if a hook with the same URL already exists it is left
        unchanged; only missing or stale hooks are (re-)created.

        Args:
            project_id: GitLab numeric project ID.
            webhook_url: Full URL the webhook should POST to.
            secret: Optional webhook secret token (WEBHOOK_GITLAB_SECRET).

        Returns True if the hook is present after the call, False on error.
        """
        if not project_id or not webhook_url:
            logger.debug("ensure_webhook_current: project_id or webhook_url missing — skip")
            return False

        # List existing hooks
        hooks = await self._get(self._api(f"projects/{project_id}/hooks"))
        if not isinstance(hooks, list):
            logger.warning(f"GitLab: could not list hooks for project {project_id}")
            return False

        for hook in hooks:
            if hook.get("url") == webhook_url:
                logger.debug(f"GitLab: webhook already registered for project {project_id}")
                return True

        # Register missing hook
        body: Dict[str, Any] = {
            "url": webhook_url,
            "push_events": True,
            "issues_events": True,
            "merge_requests_events": True,
            "note_events": True,
            "confidential_issues_events": True,
            "confidential_note_events": True,
            "enable_ssl_verification": False,
        }
        if secret:
            body["token"] = secret

        result = await self._post(self._api(f"projects/{project_id}/hooks"), json_body=body)
        if result and result.get("id"):
            logger.info(
                f"GitLab: registered webhook #{result['id']} for project {project_id} → {webhook_url}"
            )
            return True

        logger.warning(f"GitLab: failed to register webhook for project {project_id}")
        return False
