"""
Project Manager Agent.

Decomposes a problem statement into a work item hierarchy (Epic → Story/Feature → Task/Bug)
and creates items in the target platform (Azure DevOps, GitLab, or GitHub).

Usage:
    agent = PMAgent(platform="azure")
    plan = agent.decompose("Build a user auth system")
    print(agent.format_preview(plan))
    created, failed = await agent.create_all(plan)
"""

import json
import logging
import re
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, Tuple

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Data model
# ---------------------------------------------------------------------------

@dataclass
class WorkItemNode:
    """A single work item in the decomposed hierarchy."""
    level: int            # 0=Epic/Milestone, 1=Feature/Story, 2=Task/Bug/Issue
    item_type: str        # "Epic", "Feature", "User Story", "Bug", "Task", "milestone", "issue"
    title: str
    description: str
    labels: List[str] = field(default_factory=list)
    parent_index: Optional[int] = None   # index in flat list; None for top-level
    platform_id: Optional[str] = None    # filled after creation
    platform_url: Optional[str] = None   # filled after creation


@dataclass
class DecompositionPlan:
    """Full hierarchy returned by LLM decomposition."""
    problem_statement: str
    platform: str                        # "azure" | "gitlab" | "github"
    items: List[WorkItemNode]
    total_count: int
    epic_count: int
    story_count: int
    task_count: int


# ---------------------------------------------------------------------------
# PMAgent
# ---------------------------------------------------------------------------

class PMAgent:
    """
    Decomposes a problem statement into a work item hierarchy and creates
    items in the target platform.

    Supported platforms: "azure" | "gitlab" | "github"
    """

    PLATFORM_TYPE_MAP = {
        "azure": {0: "Epic", 1: "Feature", 2: "Task"},
        "gitlab": {0: "milestone", 1: "issue", 2: "issue"},
        "github": {0: "milestone", 1: "issue", 2: "issue"},
    }

    def __init__(
        self,
        platform: str,
        provider=None,
        azure_client=None,
        gitlab_client=None,
        github_token: Optional[str] = None,
        project_context: Optional[str] = None,
        area_path: Optional[str] = None,
        iteration_path: Optional[str] = None,
        max_items_per_level: int = 10,
    ):
        if platform not in ("azure", "gitlab", "github"):
            raise ValueError(f"Unsupported platform: {platform}")
        self.platform = platform
        self._provider = provider
        self._azure_client = azure_client
        self._gitlab_client = gitlab_client
        self._github_token = github_token
        self.project_context = project_context or ""
        self.area_path = area_path
        self.iteration_path = iteration_path
        self.max_items_per_level = max_items_per_level

    def _get_provider(self):
        if self._provider is None:
            from backend.llm import get_provider
            self._provider = get_provider()
        return self._provider

    # -- decomposition -------------------------------------------------------

    def decompose(self, problem_statement: str) -> DecompositionPlan:
        """
        Call LLM to decompose problem_statement into a work item hierarchy.

        Returns a DecompositionPlan with items populated but platform_id=None.
        Does NOT make any platform API calls.

        Raises ValueError if LLM returns unparseable JSON after 2 retries.
        """
        prompt = self._build_decomposition_prompt(problem_statement)

        try:
            from backend.personalization import inject_style
            prompt = inject_style(prompt, context_type="task", query_text=problem_statement[:200])
        except Exception:
            pass  # personalization is optional

        try:
            from backend.llm.base import LLMOptions
        except ImportError:
            LLMOptions = None

        try:
            from backend.config import get_int
            timeout = get_int("HTTP_TIMEOUT_LONG")
        except Exception:
            timeout = 60

        options = LLMOptions(temperature=0.3, max_tokens=2000) if LLMOptions else None

        if options is not None:
            raw = self._get_provider().generate(
                prompt=prompt,
                options=options,
                timeout=timeout,
            )
        else:
            raw = self._get_provider().generate(prompt=prompt, timeout=timeout)

        if not raw:
            raise ValueError("LLM returned no response for decomposition")

        items = self._parse_decomposition(raw, problem_statement)
        return self._build_plan(problem_statement, items)

    def _build_decomposition_prompt(self, problem_statement: str) -> str:
        type_map = self.PLATFORM_TYPE_MAP[self.platform]

        if self.platform == "azure":
            level_desc = (
                "- level 0: type must be \"Epic\"\n"
                "- level 1: type must be one of \"Feature\", \"User Story\", \"Bug\"\n"
                "- level 2: type must be one of \"Task\", \"Bug\""
            )
        else:
            level_desc = (
                "- level 0: type must be \"milestone\" (this becomes a GitLab/GitHub Milestone)\n"
                "- level 1: type must be \"issue\" with labels like [\"story\"] or [\"epic\"]\n"
                "- level 2: type must be \"issue\" with labels like [\"task\"] or [\"bug\"]"
            )

        context_block = f"\nProject context: {self.project_context}\n" if self.project_context else ""
        cap = self.max_items_per_level

        return f"""You are a project manager. Decompose the following problem into a hierarchical work item plan for {self.platform}.
{context_block}
Problem: {problem_statement}

Rules:
{level_desc}
- Maximum {cap} items per level
- Each item needs: level (integer), type (string), title (short, actionable), description (1-2 sentences), labels (list), parent_index (integer index into items array, or null for top-level)
- parent_index must reference an item that comes BEFORE the current item in the array
- Items at level 1 must have a level-0 parent. Items at level 2 must have a level-1 parent.
- Respond with ONLY valid JSON, no explanatory text before or after.

Required JSON format:
{{
  "items": [
    {{
      "level": 0,
      "type": "{type_map[0]}",
      "title": "...",
      "description": "...",
      "labels": [],
      "parent_index": null
    }},
    {{
      "level": 1,
      "type": "{type_map[1]}",
      "title": "...",
      "description": "...",
      "labels": ["story"],
      "parent_index": 0
    }},
    {{
      "level": 2,
      "type": "{type_map[2]}",
      "title": "...",
      "description": "...",
      "labels": ["task"],
      "parent_index": 1
    }}
  ]
}}"""

    def _parse_decomposition(self, raw: str, problem_statement: str) -> List[WorkItemNode]:
        """Parse LLM JSON output into List[WorkItemNode]."""
        data = None

        # Try 1: direct parse
        try:
            data = json.loads(raw)
        except json.JSONDecodeError:
            pass

        # Try 2: extract first {...} block
        if data is None:
            match = re.search(r'\{.*\}', raw, re.DOTALL)
            if match:
                try:
                    data = json.loads(match.group(0))
                except json.JSONDecodeError:
                    pass

        if data is None:
            raise ValueError(f"LLM returned invalid JSON. Raw response (first 200 chars): {raw[:200]}")

        raw_items = data.get("items", [])
        if not isinstance(raw_items, list):
            raise ValueError("LLM JSON missing 'items' list")

        # Count items per level for cap enforcement
        level_counts: Dict[int, int] = {}
        result: List[WorkItemNode] = []

        for item in raw_items:
            if not isinstance(item, dict):
                continue
            level = item.get("level")
            if not isinstance(level, int) or level not in (0, 1, 2):
                continue

            # Enforce cap per level
            level_counts[level] = level_counts.get(level, 0) + 1
            if level_counts[level] > self.max_items_per_level:
                logger.warning(f"Skipping item at level {level} — max {self.max_items_per_level} reached")
                continue

            title = item.get("title", "").strip()
            if not title:
                continue

            parent_index = item.get("parent_index")
            if parent_index is not None:
                if not isinstance(parent_index, int) or parent_index >= len(result):
                    parent_index = None  # invalid reference — treat as root

            node = WorkItemNode(
                level=level,
                item_type=item.get("type", self.PLATFORM_TYPE_MAP[self.platform][level]),
                title=title,
                description=item.get("description", ""),
                labels=item.get("labels", []) if isinstance(item.get("labels"), list) else [],
                parent_index=parent_index,
            )
            result.append(node)

        if not result:
            raise ValueError("LLM returned no valid work items")

        return result

    def _build_plan(self, problem_statement: str, items: List[WorkItemNode]) -> DecompositionPlan:
        epic_count = sum(1 for i in items if i.level == 0)
        story_count = sum(1 for i in items if i.level == 1)
        task_count = sum(1 for i in items if i.level == 2)
        return DecompositionPlan(
            problem_statement=problem_statement,
            platform=self.platform,
            items=items,
            total_count=len(items),
            epic_count=epic_count,
            story_count=story_count,
            task_count=task_count,
        )

    def format_preview(self, plan: DecompositionPlan) -> str:
        """Format plan as a human-readable indented tree."""
        lines = []
        indent_per_level = "  "
        for node in plan.items:
            indent = indent_per_level * node.level
            lines.append(f"{indent}{node.item_type}: {node.title}")

        lines.append("")
        lines.append(
            f"{plan.total_count} items total: "
            f"{plan.epic_count} epic(s)/milestone(s), "
            f"{plan.story_count} story/feature(s), "
            f"{plan.task_count} task(s)/bug(s)"
        )
        return "\n".join(lines)

    # -- creation ------------------------------------------------------------

    async def create_all(
        self,
        plan: DecompositionPlan,
        on_progress=None,
    ) -> Tuple[List[WorkItemNode], List[Tuple[WorkItemNode, str]]]:
        """
        Create all items in the target platform, respecting parent-child order.

        Creates items level by level (level 0 first) so parent IDs are
        available when creating children.

        Returns:
            (created, failed) where:
              created: List[WorkItemNode] with platform_id and platform_url filled
              failed:  List[(WorkItemNode, error_msg)]
        """
        created: List[WorkItemNode] = []
        failed: List[Tuple[WorkItemNode, str]] = []

        # Process level by level to ensure parents exist before children
        for level in (0, 1, 2):
            level_items = [item for item in plan.items if item.level == level]
            for node in level_items:
                parent_node: Optional[WorkItemNode] = None
                if node.parent_index is not None:
                    parent_node = plan.items[node.parent_index]
                    # If parent failed (no platform_id), skip this child
                    if parent_node.platform_id is None:
                        err = f"parent '{parent_node.title[:40]}' creation failed"
                        failed.append((node, err))
                        if on_progress:
                            await on_progress(node, f"skipped — {err}")
                        continue

                try:
                    success = await self._create_item(node, parent_node)
                    if success:
                        created.append(node)
                        status = f"created {node.platform_id or ''}"
                    else:
                        failed.append((node, "platform returned no result"))
                        status = "failed"
                except Exception as e:
                    failed.append((node, str(e)))
                    status = f"error: {str(e)[:50]}"

                if on_progress:
                    await on_progress(node, status)

        return created, failed

    async def _create_item(self, node: WorkItemNode, parent_node: Optional[WorkItemNode]) -> bool:
        """Dispatch to the correct platform creator."""
        if self.platform == "azure":
            return await self._create_azure_item(node, parent_node)
        elif self.platform == "gitlab":
            return await self._create_gitlab_item(node, parent_node)
        elif self.platform == "github":
            return await self._create_github_item(node, parent_node)
        return False

    async def _create_azure_item(self, node: WorkItemNode, parent_node: Optional[WorkItemNode]) -> bool:
        """Create a single work item in Azure DevOps."""
        client = self._get_azure_client()

        parent_id = None
        if parent_node and parent_node.platform_id:
            try:
                parent_id = int(parent_node.platform_id)
            except (ValueError, TypeError):
                pass

        wi = await client.create_work_item(
            title=node.title,
            description=node.description,
            work_item_type=node.item_type,
            parent_id=parent_id,
            area_path=self.area_path,
            iteration_path=self.iteration_path,
        )
        if wi:
            node.platform_id = str(wi.id)
            node.platform_url = wi.url
            return True
        return False

    async def _create_gitlab_item(self, node: WorkItemNode, parent_node: Optional[WorkItemNode]) -> bool:
        """Create milestone (level 0) or issue (level 1/2) in GitLab."""
        client = self._get_gitlab_client()

        if node.level == 0:
            # Create as milestone
            url = client._api(f"projects/{client._project_id}/milestones")
            result = await client._post(url, json_body={"title": node.title, "description": node.description})
            if result:
                node.platform_id = str(result.get("id", ""))
                node.platform_url = result.get("web_url", "")
                return True
            return False
        else:
            # Create as issue; use parent's platform_id as milestone_id if level 0 parent
            milestone_id = None
            if parent_node and parent_node.level == 0 and parent_node.platform_id:
                try:
                    milestone_id = int(parent_node.platform_id)
                except (ValueError, TypeError):
                    pass

            labels = node.labels or []
            issue = await client.create_issue(
                title=node.title,
                description=node.description,
                labels=labels,
                milestone_id=milestone_id,
            )
            if issue:
                node.platform_id = str(issue.iid)
                node.platform_url = issue.url
                return True
            return False

    async def _create_github_item(self, node: WorkItemNode, parent_node: Optional[WorkItemNode]) -> bool:
        """Create milestone (level 0) or issue (level 1/2) in GitHub via REST."""
        import aiohttp
        import os

        from backend.config import get_github_token, get_github_owner, get_github_repo
        token = self._github_token or get_github_token()
        owner = get_github_owner()
        repo = get_github_repo()

        if not token or not owner or not repo:
            logger.error("GitHub creation requires GITHUB_TOKEN, GITHUB_OWNER, GITHUB_REPO")
            return False

        headers = {
            "Authorization": f"token {token}",
            "Accept": "application/vnd.github.v3+json",
        }
        base = f"https://api.github.com/repos/{owner}/{repo}"

        async with aiohttp.ClientSession(headers=headers) as session:
            if node.level == 0:
                # Create as milestone
                async with session.post(
                    f"{base}/milestones",
                    json={"title": node.title, "description": node.description},
                ) as resp:
                    if resp.status == 201:
                        data = await resp.json()
                        node.platform_id = str(data.get("number", ""))
                        node.platform_url = data.get("html_url", "")
                        return True
                    body = await resp.text()
                    logger.warning(f"GitHub milestone creation failed {resp.status}: {body[:200]}")
                    return False
            else:
                # Create as issue
                milestone_number = None
                if parent_node and parent_node.level == 0 and parent_node.platform_id:
                    try:
                        milestone_number = int(parent_node.platform_id)
                    except (ValueError, TypeError):
                        pass

                issue_body: Dict[str, Any] = {
                    "title": node.title,
                    "body": node.description,
                    "labels": node.labels or [],
                }
                if milestone_number:
                    issue_body["milestone"] = milestone_number

                async with session.post(f"{base}/issues", json=issue_body) as resp:
                    if resp.status == 201:
                        data = await resp.json()
                        node.platform_id = str(data.get("number", ""))
                        node.platform_url = data.get("html_url", "")
                        return True
                    body = await resp.text()
                    logger.warning(f"GitHub issue creation failed {resp.status}: {body[:200]}")
                    return False

    def _get_azure_client(self):
        """Lazy-init AzureDevOpsClient."""
        if self._azure_client is None:
            from backend.azure.client import AzureDevOpsClient
            self._azure_client = AzureDevOpsClient()
        return self._azure_client

    def _get_gitlab_client(self):
        """Lazy-init GitLabClient."""
        if self._gitlab_client is None:
            from backend.gitlab.client import GitLabClient
            self._gitlab_client = GitLabClient()
        return self._gitlab_client
