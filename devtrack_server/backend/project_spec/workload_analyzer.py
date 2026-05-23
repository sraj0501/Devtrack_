"""
Analyze current workload for a list of developers by querying their PM platform.

Computes available_days to the project deadline based on:
  - Working days remaining to deadline
  - Days already committed to active work items (estimated or default)

All methods are async. Configuration is read from env via platform clients.
"""

import logging
from dataclasses import dataclass, field
from datetime import date, datetime, timedelta
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)

# Default story-point-to-days ratio when no explicit estimate is available
_DAYS_PER_STORY_POINT = 0.5
# Default days to assume per active work item when no story points are present
_DEFAULT_DAYS_PER_ITEM = 2.0
# Working days per week
_WORK_DAYS_PER_WEEK = 5


def _working_days(start: date, end: date) -> float:
    """Count working days (Mon–Fri) between start (inclusive) and end (inclusive)."""
    if end <= start:
        return 0.0
    total = 0
    current = start
    while current <= end:
        if current.weekday() < 5:  # 0=Mon … 4=Fri
            total += 1
        current += timedelta(days=1)
    return float(total)


@dataclass
class AssignedItem:
    """A single active work item assigned to a developer."""

    item_id: str
    title: str
    state: str
    story_points: Optional[float] = None
    estimated_days_remaining: float = _DEFAULT_DAYS_PER_ITEM

    def to_dict(self) -> Dict[str, Any]:
        return {
            "item_id": self.item_id,
            "title": self.title,
            "state": self.state,
            "story_points": self.story_points,
            "estimated_days_remaining": self.estimated_days_remaining,
        }


@dataclass
class DeveloperLoad:
    """Computed workload for one developer."""

    name: str
    platform_user_id: str
    current_assignments: List[AssignedItem] = field(default_factory=list)
    committed_days: float = 0.0
    available_days: float = 0.0
    first_available_sprint: int = 1
    capacity_source: str = "auto"  # "auto" | "override"

    def to_dict(self) -> Dict[str, Any]:
        return {
            "name": self.name,
            "platform_user_id": self.platform_user_id,
            "current_assignments": [a.to_dict() for a in self.current_assignments],
            "committed_days": round(self.committed_days, 1),
            "available_days": round(self.available_days, 1),
            "first_available_sprint": self.first_available_sprint,
            "capacity_source": self.capacity_source,
        }


@dataclass
class WorkloadSnapshot:
    """Aggregated workload snapshot for all team members."""

    pulled_at: str  # ISO datetime string
    developers: List[DeveloperLoad] = field(default_factory=list)
    sprint_length_days: int = 10  # defaults to 2-week sprints (10 working days)

    def total_available_days(self) -> Dict[str, float]:
        return {d.name: d.available_days for d in self.developers}

    def team_total(self) -> float:
        return sum(d.available_days for d in self.developers)

    def to_dict(self) -> Dict[str, Any]:
        return {
            "pulled_at": self.pulled_at,
            "developers": [d.to_dict() for d in self.developers],
        }

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "WorkloadSnapshot":
        snap = cls(pulled_at=d.get("pulled_at", ""), sprint_length_days=d.get("sprint_length_days", 10))
        for dev_d in d.get("developers", []):
            items = [
                AssignedItem(
                    item_id=a.get("item_id", ""),
                    title=a.get("title", ""),
                    state=a.get("state", ""),
                    story_points=a.get("story_points"),
                    estimated_days_remaining=a.get("estimated_days_remaining", _DEFAULT_DAYS_PER_ITEM),
                )
                for a in dev_d.get("current_assignments", [])
            ]
            snap.developers.append(DeveloperLoad(
                name=dev_d.get("name", ""),
                platform_user_id=dev_d.get("platform_user_id", ""),
                current_assignments=items,
                committed_days=dev_d.get("committed_days", 0.0),
                available_days=dev_d.get("available_days", 0.0),
                first_available_sprint=dev_d.get("first_available_sprint", 1),
                capacity_source=dev_d.get("capacity_source", "auto"),
            ))
        return snap


class WorkloadAnalyzer:
    """Fetch and compute developer workload from the PM platform."""

    def __init__(self, sprint_length_days: int = 10):
        self._sprint_length = sprint_length_days

    async def analyze(
        self,
        developers: List[Any],   # List[Developer] from developer_roster
        platform: str,
        deadline: Optional[date] = None,
    ) -> WorkloadSnapshot:
        """Fetch active items for each developer and compute available capacity.

        Args:
            developers: List of Developer objects (from DeveloperRoster).
            platform:   "azure" | "github" | "gitlab"
            deadline:   Project deadline — used to compute available_days.
                        Defaults to 90 days from today if not provided.
        """
        today = date.today()
        end_date = deadline or (today + timedelta(days=90))
        total_working_days = _working_days(today, end_date)

        snap = WorkloadSnapshot(
            pulled_at=datetime.utcnow().isoformat() + "Z",
            sprint_length_days=self._sprint_length,
        )

        platform = (platform or "").lower().strip()
        for dev in developers:
            # Respect PM-provided capacity override
            if dev.capacity_override and dev.capacity_override.get("available_days") is not None:
                load = DeveloperLoad(
                    name=dev.name,
                    platform_user_id=dev.platform_user_id,
                    committed_days=0.0,
                    available_days=float(dev.capacity_override["available_days"]),
                    first_available_sprint=1,
                    capacity_source="override",
                )
                snap.developers.append(load)
                continue

            try:
                assignments = await self._fetch_assignments(dev.platform_user_id, platform)
            except Exception as e:
                logger.warning(f"Could not fetch assignments for {dev.name}: {e}")
                assignments = []

            committed = sum(a.estimated_days_remaining for a in assignments)
            available = max(0.0, total_working_days - committed)
            first_sprint = max(1, int(committed / self._sprint_length) + 1)

            snap.developers.append(DeveloperLoad(
                name=dev.name,
                platform_user_id=dev.platform_user_id,
                current_assignments=assignments,
                committed_days=round(committed, 1),
                available_days=round(available, 1),
                first_available_sprint=first_sprint,
                capacity_source="auto",
            ))

        return snap

    # -- platform fetchers --------------------------------------------------

    async def _fetch_assignments(self, user_id: str, platform: str) -> List[AssignedItem]:
        if platform == "azure":
            return await self._azure_assignments(user_id)
        elif platform == "github":
            return await self._github_assignments(user_id)
        elif platform == "gitlab":
            return await self._gitlab_assignments(user_id)
        return []

    async def _azure_assignments(self, user_email: str) -> List[AssignedItem]:
        from backend.azure.client import AzureDevOpsClient
        client = AzureDevOpsClient()
        if not client.is_configured():
            return []
        try:
            items = await client.get_work_items_for_user(user_email)
            result = []
            for wi in items:
                sp = None
                # Story points live in fields dict on the raw response; our dataclass
                # doesn't carry them, so estimate from type
                days = _DEFAULT_DAYS_PER_ITEM
                result.append(AssignedItem(
                    item_id=str(wi.id),
                    title=wi.title,
                    state=wi.state,
                    story_points=sp,
                    estimated_days_remaining=days,
                ))
            return result
        finally:
            await client.close()

    async def _github_assignments(self, username: str) -> List[AssignedItem]:
        from backend.github.client import GitHubClient
        client = GitHubClient()
        if not client.is_configured():
            return []
        try:
            issues = await client.get_issues_for_user(username, state="open")
            return [
                AssignedItem(
                    item_id=str(issue.number),
                    title=issue.title,
                    state=issue.state,
                    story_points=None,
                    estimated_days_remaining=_DEFAULT_DAYS_PER_ITEM,
                )
                for issue in issues
            ]
        finally:
            await client.close()

    async def _gitlab_assignments(self, username: str) -> List[AssignedItem]:
        from backend.gitlab.client import GitLabClient
        client = GitLabClient()
        if not client.is_configured():
            return []
        try:
            issues = await client.get_issues_for_user(username, state="opened")
            return [
                AssignedItem(
                    item_id=str(issue.iid),
                    title=issue.title,
                    state=issue.state,
                    story_points=None,
                    estimated_days_remaining=_DEFAULT_DAYS_PER_ITEM,
                )
                for issue in issues
            ]
        finally:
            await client.close()
