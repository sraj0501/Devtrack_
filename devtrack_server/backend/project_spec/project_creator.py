"""
Create PM artifacts (sprints, epics, stories) from an approved ProjectSpec.

Supports Azure DevOps, GitHub, GitLab. All operations are async.
Dependencies between stories are respected via topological sort.

Configuration via env vars (same as platform clients).
"""

import asyncio
import logging
from typing import Any, Callable, Dict, List, Optional

logger = logging.getLogger(__name__)


def _topo_sort(stories: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    """Topological sort of stories by depends_on field.

    Stories with no dependencies come first.
    Cycles are broken by falling back to original order.
    """
    id_to_story = {s["id"]: s for s in stories}
    visited: set = set()
    order: List[Dict[str, Any]] = []

    def visit(story_id: str, stack: set) -> None:
        if story_id in visited:
            return
        if story_id in stack:
            return  # cycle guard
        stack.add(story_id)
        s = id_to_story.get(story_id)
        if not s:
            return
        for dep in s.get("depends_on", []):
            visit(dep, stack)
        stack.discard(story_id)
        visited.add(story_id)
        order.append(s)

    for s in stories:
        visit(s["id"], set())
    return order


class ProjectCreator:
    """Create PM tool artifacts from an approved ProjectSpec."""

    def __init__(self, on_progress: Optional[Callable[[str], Any]] = None):
        """
        Args:
            on_progress: Optional async or sync callback called with status strings.
        """
        self._on_progress = on_progress

    async def create(self, spec: Any, dry_run: bool = False) -> Dict[str, Any]:
        """Create all artifacts defined in a spec.

        Args:
            spec:    ProjectSpec instance.
            dry_run: If True, log actions without creating anything.

        Returns a summary dict with created item IDs and any errors.
        """
        platform = (spec.pm_platform or "").lower().strip()
        results: Dict[str, Any] = {
            "sprints": [],
            "stories": [],
            "errors": [],
        }

        if platform == "azure":
            await self._create_azure(spec, results, dry_run)
        elif platform == "github":
            await self._create_github(spec, results, dry_run)
        elif platform == "gitlab":
            await self._create_gitlab(spec, results, dry_run)
        else:
            results["errors"].append(f"Unknown platform: {platform}")

        return results

    async def _notify(self, msg: str) -> None:
        if self._on_progress:
            if asyncio.iscoroutinefunction(self._on_progress):
                await self._on_progress(msg)
            else:
                self._on_progress(msg)

    # -- Azure DevOps -------------------------------------------------------

    async def _create_azure(self, spec: Any, results: Dict, dry_run: bool) -> None:
        from backend.azure.client import AzureDevOpsClient
        client = AzureDevOpsClient()
        if not client.is_configured():
            results["errors"].append("Azure not configured")
            return
        try:
            # 1. Create iterations (sprints)
            sprint_paths: Dict[int, str] = {}
            for sprint in spec.sprints:
                num = sprint.get("number", 0)
                name = sprint.get("name", f"Sprint {num}")
                start = sprint.get("start")
                end = sprint.get("end")
                await self._notify(f"Creating sprint: {name}")
                if not dry_run:
                    result = await client.create_iteration(name, start_date=start, end_date=end)
                    if result:
                        # Build iteration path: Project\SprintName
                        proj = client._project or ""
                        sprint_paths[num] = f"{proj}\\{name}"
                        results["sprints"].append({"number": num, "name": name, "id": result.get("id")})
                    else:
                        results["errors"].append(f"Failed to create sprint: {name}")
                else:
                    sprint_paths[num] = f"(dry_run)\\{name}"
                    results["sprints"].append({"number": num, "name": name, "id": "dry_run"})

            # 2. Create stories (topologically sorted)
            all_stories = []
            feature_story_map: Dict[str, int] = {}  # feature_id -> epic work item id

            for feature in spec.features:
                # Create a Feature / Epic work item as parent
                feature_title = feature.get("title", "")
                await self._notify(f"Creating feature: {feature_title}")
                epic_id: Optional[int] = None
                if not dry_run:
                    epic = await client.create_work_item(
                        title=feature_title,
                        description=feature.get("description", ""),
                        work_item_type="Feature",
                    )
                    if epic:
                        epic_id = epic.id
                        feature_story_map[feature["id"]] = epic_id

                stories = feature.get("stories", [])
                for s in stories:
                    s["_feature_id"] = feature["id"]
                    s["_epic_id"] = epic_id
                all_stories.extend(stories)

            sorted_stories = _topo_sort(all_stories)
            created_ids: Dict[str, int] = {}
            for story in sorted_stories:
                title = story.get("title", "")
                sprint_num = story.get("sprint", 1)
                iter_path = sprint_paths.get(sprint_num)
                sp = story.get("story_points")
                assignee = story.get("assigned_to") or None
                epic_id = story.get("_epic_id")

                await self._notify(f"Creating story: {title}")
                if not dry_run:
                    wi = await client.create_work_item_with_story_points(
                        title=title,
                        story_points=float(sp) if sp is not None else None,
                        description="\n".join(story.get("acceptance_criteria", [])),
                        work_item_type="User Story",
                        parent_id=epic_id,
                        iteration_path=iter_path,
                        assigned_to=assignee,
                    )
                    if wi:
                        created_ids[story["id"]] = wi.id
                        results["stories"].append({"id": story["id"], "work_item_id": wi.id, "title": title})
                    else:
                        results["errors"].append(f"Failed to create story: {title}")
                else:
                    results["stories"].append({"id": story["id"], "work_item_id": "dry_run", "title": title})
        finally:
            await client.close()

    # -- GitHub -------------------------------------------------------------

    async def _create_github(self, spec: Any, results: Dict, dry_run: bool) -> None:
        from backend.github.client import GitHubClient
        client = GitHubClient()
        if not client.is_configured():
            results["errors"].append("GitHub not configured")
            return
        try:
            # 1. Create milestones (sprints)
            milestone_numbers: Dict[int, int] = {}
            for sprint in spec.sprints:
                num = sprint.get("number", 0)
                name = sprint.get("name", f"Sprint {num}")
                due_on = sprint.get("end")
                await self._notify(f"Creating milestone: {name}")
                if not dry_run:
                    ms = await client.create_milestone(title=name, due_on=due_on,
                                                       description=sprint.get("goal", ""))
                    if ms:
                        milestone_numbers[num] = ms.get("number")
                        results["sprints"].append({"number": num, "name": name, "milestone_number": ms.get("number")})
                    else:
                        results["errors"].append(f"Failed to create milestone: {name}")
                else:
                    milestone_numbers[num] = 0
                    results["sprints"].append({"number": num, "name": name, "milestone_number": "dry_run"})

            # 2. Collect and sort stories
            all_stories = []
            for feature in spec.features:
                feature_label = feature.get("title", "")
                for s in feature.get("stories", []):
                    s["_feature_label"] = feature_label
                    all_stories.append(s)
            sorted_stories = _topo_sort(all_stories)

            for story in sorted_stories:
                title = story.get("title", "")
                sprint_num = story.get("sprint", 1)
                ms_number = milestone_numbers.get(sprint_num)
                assignees = []
                if story.get("assigned_to"):
                    # GitHub uses login names; use platform_user_id if email was stored
                    assignees = [story["assigned_to"]]
                labels = [story.get("_feature_label", "")] if story.get("_feature_label") else []
                body = "\n".join(story.get("acceptance_criteria", []))
                await self._notify(f"Creating issue: {title}")
                if not dry_run:
                    issue = await client.create_issue(
                        title=title, body=body,
                        labels=[l for l in labels if l],
                        assignees=assignees,
                        milestone=ms_number,
                    )
                    if issue:
                        results["stories"].append({"id": story["id"], "issue_number": issue.number, "title": title})
                    else:
                        results["errors"].append(f"Failed to create issue: {title}")
                else:
                    results["stories"].append({"id": story["id"], "issue_number": "dry_run", "title": title})
        finally:
            await client.close()

    # -- GitLab -------------------------------------------------------------

    async def _create_gitlab(self, spec: Any, results: Dict, dry_run: bool) -> None:
        from backend.gitlab.client import GitLabClient
        client = GitLabClient()
        if not client.is_configured():
            results["errors"].append("GitLab not configured")
            return
        try:
            # 1. Create milestones (sprints)
            milestone_ids: Dict[int, int] = {}
            for sprint in spec.sprints:
                num = sprint.get("number", 0)
                name = sprint.get("name", f"Sprint {num}")
                await self._notify(f"Creating milestone: {name}")
                if not dry_run:
                    ms = await client.create_milestone(
                        title=name,
                        start_date=sprint.get("start"),
                        due_date=sprint.get("end"),
                        description=sprint.get("goal", ""),
                    )
                    if ms:
                        milestone_ids[num] = ms.get("id")
                        results["sprints"].append({"number": num, "name": name, "milestone_id": ms.get("id")})
                    else:
                        results["errors"].append(f"Failed to create milestone: {name}")
                else:
                    milestone_ids[num] = 0
                    results["sprints"].append({"number": num, "name": name, "milestone_id": "dry_run"})

            # 2. Create issues
            all_stories = []
            for feature in spec.features:
                labels = [feature.get("title", "")]
                for s in feature.get("stories", []):
                    s["_labels"] = [l for l in labels if l]
                    all_stories.append(s)
            sorted_stories = _topo_sort(all_stories)

            for story in sorted_stories:
                title = story.get("title", "")
                sprint_num = story.get("sprint", 1)
                ms_id = milestone_ids.get(sprint_num)
                body = "\n".join(story.get("acceptance_criteria", []))
                await self._notify(f"Creating issue: {title}")
                if not dry_run:
                    issue = await client.create_issue(
                        title=title,
                        description=body,
                        labels=story.get("_labels", []),
                        milestone_id=ms_id,
                    )
                    if issue:
                        results["stories"].append({"id": story["id"], "iid": issue.iid, "title": title})
                    else:
                        results["errors"].append(f"Failed to create issue: {title}")
                else:
                    results["stories"].append({"id": story["id"], "iid": "dry_run", "title": title})
        finally:
            await client.close()
