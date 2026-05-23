"""
Backlog dataclasses — BacklogItem and Sprint.

These are the in-memory representations used throughout the application.
Persistence is handled by backend.db.project_store.
"""

from __future__ import annotations

from dataclasses import dataclass, field, asdict
from datetime import datetime
from enum import Enum
from typing import Any, Dict, List, Optional


class ItemType(str, Enum):
    EPIC = "epic"
    STORY = "story"
    BUG = "bug"
    TASK = "task"


class ItemStatus(str, Enum):
    OPEN = "open"
    IN_PROGRESS = "in_progress"
    DONE = "done"
    WONT_DO = "wont_do"


class ItemPriority(str, Enum):
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"
    CRITICAL = "critical"


class SprintStatus(str, Enum):
    PLANNED = "planned"
    ACTIVE = "active"
    CLOSED = "closed"


@dataclass
class BacklogItem:
    """A single item in the project backlog (epic, story, bug, or task)."""

    id: str
    project_id: str
    title: str
    item_type: ItemType = ItemType.STORY
    status: ItemStatus = ItemStatus.OPEN
    priority: ItemPriority = ItemPriority.MEDIUM
    description: str = ""
    acceptance_criteria: str = ""
    story_points: Optional[int] = None
    labels: List[str] = field(default_factory=list)
    assigned_to: str = ""
    sprint_id: Optional[str] = None
    parent_id: Optional[str] = None      # for sub-tasks / child stories
    external_id: Optional[str] = None    # linked issue/work-item ID
    external_source: Optional[str] = None
    created_at: datetime = field(default_factory=datetime.utcnow)
    updated_at: datetime = field(default_factory=datetime.utcnow)

    def to_dict(self) -> Dict[str, Any]:
        d = asdict(self)
        d["item_type"] = self.item_type.value
        d["status"] = self.status.value
        d["priority"] = self.priority.value
        d["created_at"] = self.created_at.isoformat()
        d["updated_at"] = self.updated_at.isoformat()
        return d

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> BacklogItem:
        d = dict(d)
        d["item_type"] = ItemType(d.get("item_type", "story"))
        d["status"] = ItemStatus(d.get("status", "open"))
        d["priority"] = ItemPriority(d.get("priority", "medium"))
        for ts in ("created_at", "updated_at"):
            if isinstance(d.get(ts), str):
                d[ts] = datetime.fromisoformat(d[ts])
        return cls(**d)


@dataclass
class Sprint:
    """A time-boxed sprint belonging to a project."""

    id: str
    project_id: str
    name: str
    status: SprintStatus = SprintStatus.PLANNED
    goal: str = ""
    start_date: Optional[datetime] = None
    end_date: Optional[datetime] = None
    capacity_points: Optional[int] = None
    completed_points: int = 0
    created_at: datetime = field(default_factory=datetime.utcnow)
    updated_at: datetime = field(default_factory=datetime.utcnow)

    @property
    def velocity(self) -> Optional[float]:
        """Points completed per day; None if sprint has no defined dates."""
        if self.start_date and self.end_date and self.status == SprintStatus.CLOSED:
            days = max((self.end_date - self.start_date).days, 1)
            return round(self.completed_points / days, 2)
        return None

    def to_dict(self) -> Dict[str, Any]:
        d = asdict(self)
        d["status"] = self.status.value
        for ts in ("created_at", "updated_at", "start_date", "end_date"):
            val = getattr(self, ts)
            d[ts] = val.isoformat() if val else None
        return d

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> Sprint:
        d = dict(d)
        d["status"] = SprintStatus(d.get("status", "planned"))
        for ts in ("created_at", "updated_at"):
            if isinstance(d.get(ts), str):
                d[ts] = datetime.fromisoformat(d[ts])
        for ts in ("start_date", "end_date"):
            if isinstance(d.get(ts), str):
                d[ts] = datetime.fromisoformat(d[ts])
        return cls(**d)
