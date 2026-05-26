"""
Azure DevOps REST API client (async).

Provides bidirectional CRUD operations on work items, comments, iterations,
and projects. Uses aiohttp for all HTTP calls.

Configuration (via .env):
    AZURE_ORGANIZATION   = your_org
    AZURE_DEVOPS_PAT     = your_pat  (or AZURE_API_KEY)
    AZURE_PROJECT        = your_project
    AZURE_API_VERSION    = 7.1
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
    """Load .env by walking up from this file. Idempotent via override=False."""
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
class AzureWorkItem:
    """Represents an Azure DevOps work item."""

    id: int
    title: str
    description: str
    state: str
    assigned_to: str
    work_item_type: str  # Task, Bug, Product Backlog Item, …
    area_path: str
    iteration_path: str
    tags: List[str] = field(default_factory=list)
    url: str = ""
    parent_id: Optional[int] = None
    due_date: Optional[str] = None


# ---------------------------------------------------------------------------
# Client
# ---------------------------------------------------------------------------

class AzureDevOpsClient:
    """Async Azure DevOps REST API client."""

    BASE_URL = "https://dev.azure.com"

    def __init__(
        self,
        org: Optional[str] = None,
        project: Optional[str] = None,
        pat: Optional[str] = None,
    ):
        self._org = org or _env("AZURE_ORGANIZATION") or _env("ORGANIZATION")
        self._project = project or _env("AZURE_PROJECT") or _env("PROJECT")
        self._pat = pat or _env("AZURE_DEVOPS_PAT") or _env("AZURE_API_KEY")
        self._api_version = _env("AZURE_API_VERSION", "7.1")
        self._timeout_secs = _env_int("HTTP_TIMEOUT", 30)
        self._session: Optional[aiohttp.ClientSession] = None

    # -- lifecycle ----------------------------------------------------------

    def is_configured(self) -> bool:
        return bool(self._org and self._pat)

    async def _get_session(self) -> aiohttp.ClientSession:
        if self._session is None or self._session.closed:
            auth = aiohttp.BasicAuth(login="", password=self._pat)
            timeout = aiohttp.ClientTimeout(total=self._timeout_secs)
            self._session = aiohttp.ClientSession(auth=auth, timeout=timeout)
        return self._session

    async def close(self) -> None:
        if self._session and not self._session.closed:
            await self._session.close()

    # -- low-level HTTP helpers ---------------------------------------------

    def _url(self, path: str) -> str:
        return f"{self.BASE_URL}/{self._org}/{path}"

    def _org_url(self, path: str) -> str:
        """URL scoped to org (no project)."""
        return f"{self.BASE_URL}/{self._org}/{path}"

    def _project_url(self, path: str) -> str:
        """URL scoped to org/project."""
        return f"{self.BASE_URL}/{self._org}/{self._project}/{path}"

    def _api_params(self, extra: Optional[Dict[str, str]] = None) -> Dict[str, str]:
        params: Dict[str, str] = {"api-version": self._api_version}
        if extra:
            params.update(extra)
        return params

    async def _get(self, url: str, params: Optional[Dict[str, str]] = None) -> Optional[Dict[str, Any]]:
        session = await self._get_session()
        try:
            async with session.get(url, params=self._api_params(params)) as resp:
                if resp.status == 200:
                    return await resp.json()
                logger.warning(f"GET {url} returned {resp.status}: {await resp.text()}")
                return None
        except Exception as e:
            logger.warning(f"GET {url} failed: {e}")
            return None

    async def _post(
        self,
        url: str,
        json_body: Any = None,
        content_type: Optional[str] = None,
    ) -> Optional[Dict[str, Any]]:
        session = await self._get_session()
        headers: Dict[str, str] = {}
        if content_type:
            headers["Content-Type"] = content_type

        try:
            async with session.post(
                url,
                json=json_body,
                params=self._api_params(),
                headers=headers or None,
            ) as resp:
                if resp.status in (200, 201):
                    return await resp.json()
                logger.warning(f"POST {url} returned {resp.status}: {await resp.text()}")
                return None
        except Exception as e:
            logger.warning(f"POST {url} failed: {e}")
            return None

    async def _patch(self, url: str, json_body: Any = None) -> Optional[Dict[str, Any]]:
        session = await self._get_session()
        headers = {"Content-Type": "application/json-patch+json"}
        try:
            async with session.patch(
                url,
                json=json_body,
                params=self._api_params(),
                headers=headers,
            ) as resp:
                if resp.status == 200:
                    return await resp.json()
                logger.warning(f"PATCH {url} returned {resp.status}: {await resp.text()}")
                return None
        except Exception as e:
            logger.warning(f"PATCH {url} failed: {e}")
            return None

    # -- parsing helpers ----------------------------------------------------

    @staticmethod
    def _parse_work_item(data: Dict[str, Any]) -> AzureWorkItem:
        """Convert API response dict to AzureWorkItem."""
        fields = data.get("fields", {})
        assigned_raw = fields.get("System.AssignedTo", {})
        assigned_to = (
            assigned_raw.get("displayName", "")
            if isinstance(assigned_raw, dict)
            else str(assigned_raw)
        )
        tags_raw = fields.get("System.Tags", "")
        tags = [t.strip() for t in tags_raw.split(";") if t.strip()] if tags_raw else []

        # Parent link
        parent_id: Optional[int] = None
        for relation in data.get("relations", []):
            if relation.get("rel") == "System.LinkTypes.Hierarchy-Reverse":
                parent_url = relation.get("url", "")
                # URL ends with /{id}
                try:
                    parent_id = int(parent_url.rsplit("/", 1)[-1])
                except (ValueError, IndexError):
                    pass

        due_date = (
            fields.get("Microsoft.VSTS.Scheduling.TargetDate")
            or fields.get("Microsoft.VSTS.Scheduling.FinishDate")
        )
        # Trim to date portion only (Azure returns ISO datetime strings)
        if due_date and "T" in due_date:
            due_date = due_date.split("T")[0]

        return AzureWorkItem(
            id=data.get("id", 0),
            title=fields.get("System.Title", ""),
            description=fields.get("System.Description", ""),
            state=fields.get("System.State", ""),
            assigned_to=assigned_to,
            work_item_type=fields.get("System.WorkItemType", ""),
            area_path=fields.get("System.AreaPath", ""),
            iteration_path=fields.get("System.IterationPath", ""),
            tags=tags,
            url=data.get("_links", {}).get("html", {}).get("href", ""),
            parent_id=parent_id,
            due_date=due_date,
        )

    # -- read operations ----------------------------------------------------

    async def get_projects(self) -> List[Dict[str, Any]]:
        """List all projects in the organization."""
        url = self._org_url("_apis/projects")
        data = await self._get(url)
        if data:
            return data.get("value", [])
        return []

    async def get_my_work_items(
        self,
        states: Optional[List[str]] = None,
        types: Optional[List[str]] = None,
        max_results: int = 200,
        changed_after: Optional[datetime] = None,
    ) -> List[AzureWorkItem]:
        """Fetch work items assigned to the authenticated user via WIQL."""
        conditions = ["[System.AssignedTo] = @Me"]

        if states:
            state_list = ", ".join(f"'{s}'" for s in states)
            conditions.append(f"[System.State] IN ({state_list})")

        if types:
            type_list = ", ".join(f"'{t}'" for t in types)
            conditions.append(f"[System.WorkItemType] IN ({type_list})")

        if changed_after:
            ts = changed_after.strftime("%Y-%m-%dT%H:%M:%S.000Z")
            conditions.append(f"[System.ChangedDate] >= '{ts}'")

        where_clause = " AND ".join(conditions)
        wiql = (
            f"SELECT [System.Id] FROM WorkItems "
            f"WHERE {where_clause} "
            f"ORDER BY [System.ChangedDate] DESC"
        )

        url = self._project_url("_apis/wit/wiql")
        data = await self._post(url, json_body={"query": wiql, "$top": max_results})
        if not data:
            return []

        ids = [item["id"] for item in data.get("workItems", [])]
        if not ids:
            return []

        return await self.get_work_items_batch(ids[:max_results])

    async def get_work_item(self, work_item_id: int) -> Optional[AzureWorkItem]:
        """Fetch a single work item by ID."""
        url = self._org_url(f"_apis/wit/workitems/{work_item_id}")
        data = await self._get(url, params={"$expand": "relations"})
        if data:
            return self._parse_work_item(data)
        return None

    async def get_work_items_batch(self, ids: List[int]) -> List[AzureWorkItem]:
        """Fetch multiple work items by ID (batches of 200)."""
        results: List[AzureWorkItem] = []
        batch_size = 200  # Azure DevOps API limit

        for i in range(0, len(ids), batch_size):
            batch = ids[i : i + batch_size]
            csv_ids = ",".join(str(wid) for wid in batch)
            url = self._org_url("_apis/wit/workitems")
            data = await self._get(
                url,
                params={"ids": csv_ids, "$expand": "relations"},
            )
            if data:
                for item_data in data.get("value", []):
                    results.append(self._parse_work_item(item_data))

        return results

    async def search_work_items(self, query: str, max_results: int = 20) -> List[AzureWorkItem]:
        """Search work items by title text via WIQL."""
        safe_query = query.replace("'", "''")
        wiql = (
            f"SELECT [System.Id] FROM WorkItems "
            f"WHERE [System.Title] CONTAINS '{safe_query}' "
            f"ORDER BY [System.ChangedDate] DESC"
        )

        url = self._project_url("_apis/wit/wiql")
        data = await self._post(url, json_body={"query": wiql, "$top": max_results})
        if not data:
            return []

        ids = [item["id"] for item in data.get("workItems", [])]
        if not ids:
            return []

        return await self.get_work_items_batch(ids[:max_results])

    async def get_iterations(self) -> List[Dict[str, Any]]:
        """List iterations (sprints) for the project."""
        url = self._project_url("_apis/work/teamsettings/iterations")
        data = await self._get(url)
        if data:
            return data.get("value", [])
        return []

    async def get_areas(self) -> List[Dict[str, Any]]:
        """List area paths for the project."""
        url = self._project_url("_apis/wit/classificationnodes/areas")
        data = await self._get(url, params={"$depth": "5"})
        if data:
            children = data.get("children", [])
            return [data] + children
        return []

    # -- write operations ---------------------------------------------------

    async def create_work_item(
        self,
        title: str,
        description: str = "",
        work_item_type: str = "Task",
        parent_id: Optional[int] = None,
        area_path: Optional[str] = None,
        iteration_path: Optional[str] = None,
        tags: Optional[List[str]] = None,
        assigned_to: Optional[str] = None,
    ) -> Optional[AzureWorkItem]:
        """Create a new work item."""
        operations: List[Dict[str, Any]] = [
            {"op": "add", "path": "/fields/System.Title", "value": title},
        ]

        if description:
            operations.append({"op": "add", "path": "/fields/System.Description", "value": description})
        if area_path:
            operations.append({"op": "add", "path": "/fields/System.AreaPath", "value": area_path})
        if iteration_path:
            operations.append({"op": "add", "path": "/fields/System.IterationPath", "value": iteration_path})
        if tags:
            operations.append({"op": "add", "path": "/fields/System.Tags", "value": "; ".join(tags)})
        if assigned_to:
            operations.append({"op": "add", "path": "/fields/System.AssignedTo", "value": assigned_to})
        if parent_id is not None:
            parent_url = f"{self.BASE_URL}/{self._org}/_apis/wit/workitems/{parent_id}"
            operations.append({
                "op": "add",
                "path": "/relations/-",
                "value": {
                    "rel": "System.LinkTypes.Hierarchy-Reverse",
                    "url": parent_url,
                },
            })

        safe_type = work_item_type.replace(" ", "%20")
        url = self._project_url(f"_apis/wit/workitems/${safe_type}")
        data = await self._post(url, json_body=operations, content_type="application/json-patch+json")
        if data:
            wi = self._parse_work_item(data)
            logger.info(f"Created work item #{wi.id}: {title}")
            return wi
        return None

    async def update_work_item_fields(self, work_item_id: int, fields: Dict[str, Any]) -> bool:
        """Update arbitrary fields on a work item using JSON Patch."""
        operations = [
            {"op": "replace", "path": f"/fields/{field_path}", "value": value}
            for field_path, value in fields.items()
        ]
        url = self._org_url(f"_apis/wit/workitems/{work_item_id}")
        result = await self._patch(url, json_body=operations)
        if result:
            logger.info(f"Updated work item #{work_item_id} fields: {list(fields.keys())}")
            return True
        return False

    async def update_work_item_state(self, work_item_id: int, new_state: str) -> bool:
        """Transition a work item to a new state."""
        return await self.update_work_item_fields(work_item_id, {"System.State": new_state})

    async def add_comment(self, work_item_id: int, comment: str) -> bool:
        """Add a comment to a work item."""
        url = self._project_url(f"_apis/wit/workItems/{work_item_id}/comments")
        result = await self._post(url, json_body={"text": comment})
        if result:
            logger.info(f"Added comment to work item #{work_item_id}")
            return True
        return False

    # -- team & capacity operations (project planning) ----------------------

    async def list_team_members(
        self,
        project: Optional[str] = None,
        team: Optional[str] = None,
    ) -> List[Dict[str, Any]]:
        """List members of a project team.

        Returns a list of dicts with keys: id, displayName, uniqueName (email).
        Uses GET _apis/projects/{project}/teams/{team}/members.
        If team is None, fetches the default team for the project.
        """
        proj = project or self._project
        if not proj:
            logger.warning("list_team_members: no project configured")
            return []

        # Resolve default team name if not provided
        if not team:
            proj_data = await self._get(self._org_url(f"_apis/projects/{proj}"))
            if proj_data:
                default_team = proj_data.get("defaultTeam", {})
                team = default_team.get("name", proj + " Team")
            else:
                team = proj + " Team"

        url = self._org_url(f"_apis/projects/{proj}/teams/{team}/members")
        data = await self._get(url)
        if not data:
            return []
        members = []
        for m in data.get("value", []):
            identity = m.get("identity", m)
            members.append({
                "id": identity.get("id", ""),
                "displayName": identity.get("displayName", ""),
                "uniqueName": identity.get("uniqueName", ""),  # email
            })
        return members

    async def get_work_items_for_user(
        self,
        user_email: str,
        max_results: int = 100,
    ) -> List[AzureWorkItem]:
        """Fetch active work items assigned to a specific user (by email/UPN).

        Uses WIQL: AssignedTo = '{user_email}' AND State NOT IN (Done, Resolved, Closed).
        """
        safe_email = user_email.replace("'", "''")
        wiql = (
            f"SELECT [System.Id] FROM WorkItems "
            f"WHERE [System.AssignedTo] = '{safe_email}' "
            f"AND [System.State] NOT IN ('Done', 'Resolved', 'Closed', 'Removed') "
            f"ORDER BY [System.ChangedDate] DESC"
        )
        url = self._project_url("_apis/wit/wiql")
        data = await self._post(url, json_body={"query": wiql, "$top": max_results})
        if not data:
            return []
        ids = [item["id"] for item in data.get("workItems", [])]
        if not ids:
            return []
        return await self.get_work_items_batch(ids[:max_results])

    async def create_iteration(
        self,
        name: str,
        start_date: Optional[str] = None,
        end_date: Optional[str] = None,
        project: Optional[str] = None,
    ) -> Optional[Dict[str, Any]]:
        """Create a new iteration (sprint) in the project classification nodes.

        Args:
            name: Sprint name (e.g. "Sprint 1").
            start_date: ISO date string "YYYY-MM-DD".
            end_date: ISO date string "YYYY-MM-DD".
            project: Project name override.

        Returns the created iteration dict or None on failure.
        """
        proj = project or self._project
        if not proj:
            logger.warning("create_iteration: no project configured")
            return None

        body: Dict[str, Any] = {"name": name}
        if start_date or end_date:
            body["attributes"] = {}
            if start_date:
                body["attributes"]["startDate"] = f"{start_date}T00:00:00Z"
            if end_date:
                body["attributes"]["finishDate"] = f"{end_date}T00:00:00Z"

        url = self._org_url(f"_apis/wit/classificationNodes/{proj}/iterations")
        data = await self._post(url, json_body=body)
        if data:
            logger.info(f"Created iteration '{name}' in project '{proj}'")
        return data

    async def create_work_item_with_story_points(
        self,
        title: str,
        story_points: Optional[float] = None,
        description: str = "",
        work_item_type: str = "Task",
        parent_id: Optional[int] = None,
        area_path: Optional[str] = None,
        iteration_path: Optional[str] = None,
        tags: Optional[List[str]] = None,
        assigned_to: Optional[str] = None,
    ) -> Optional[AzureWorkItem]:
        """Create a work item with optional story points field."""
        operations: List[Dict[str, Any]] = [
            {"op": "add", "path": "/fields/System.Title", "value": title},
        ]
        if description:
            operations.append({"op": "add", "path": "/fields/System.Description", "value": description})
        if area_path:
            operations.append({"op": "add", "path": "/fields/System.AreaPath", "value": area_path})
        if iteration_path:
            operations.append({"op": "add", "path": "/fields/System.IterationPath", "value": iteration_path})
        if tags:
            operations.append({"op": "add", "path": "/fields/System.Tags", "value": "; ".join(tags)})
        if assigned_to:
            operations.append({"op": "add", "path": "/fields/System.AssignedTo", "value": assigned_to})
        if story_points is not None:
            operations.append({
                "op": "add",
                "path": "/fields/Microsoft.VSTS.Scheduling.StoryPoints",
                "value": story_points,
            })
        if parent_id is not None:
            parent_url = f"{self.BASE_URL}/{self._org}/_apis/wit/workitems/{parent_id}"
            operations.append({
                "op": "add",
                "path": "/relations/-",
                "value": {
                    "rel": "System.LinkTypes.Hierarchy-Reverse",
                    "url": parent_url,
                },
            })

        safe_type = work_item_type.replace(" ", "%20")
        url = self._project_url(f"_apis/wit/workitems/${safe_type}")
        data = await self._post(url, json_body=operations, content_type="application/json-patch+json")
        if data:
            wi = self._parse_work_item(data)
            logger.info(f"Created work item #{wi.id} ({story_points} pts): {title}")
            return wi
        return None
