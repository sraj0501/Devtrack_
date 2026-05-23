"""
BacklogManager — CRUD for backlog items and sprints.

Persists to SQLite via backend.db.project_store.
AI story-point estimation is optional (degrades gracefully if LLM unavailable).

Usage
-----
    from backend.backlog_manager import BacklogManager

    bm = BacklogManager()

    # Backlog items
    item = bm.add_item(project_id="abc", title="User login", item_type="story")
    items = bm.list_items("abc", status="open")
    bm.update_item(item.id, story_points=3, status="in_progress")

    # Sprints
    sprint = bm.create_sprint("abc", "Sprint 1", capacity_points=20)
    bm.move_to_sprint(item.id, sprint.id)
    bm.close_sprint(sprint.id)   # tallies completed_points
"""

from __future__ import annotations

import logging
import uuid
from datetime import datetime
from typing import List, Optional

from backend.models.backlog import (
    BacklogItem,
    ItemPriority,
    ItemStatus,
    ItemType,
    Sprint,
    SprintStatus,
)
from backend.db import project_store

try:
    from backend.llm import get_provider
except Exception:  # pragma: no cover
    get_provider = None  # type: ignore

logger = logging.getLogger(__name__)

_PRIORITY_ORDER = {
    ItemPriority.CRITICAL: 4,
    ItemPriority.HIGH: 3,
    ItemPriority.MEDIUM: 2,
    ItemPriority.LOW: 1,
}


class BacklogManager:
    """Manages backlog items and sprints for a project."""

    # ------------------------------------------------------------------
    # Backlog items
    # ------------------------------------------------------------------

    def add_item(
        self,
        project_id: str,
        title: str,
        *,
        description: str = "",
        acceptance_criteria: str = "",
        item_type: str = "story",
        priority: str = "medium",
        story_points: Optional[int] = None,
        labels: Optional[List[str]] = None,
        assigned_to: str = "",
        sprint_id: Optional[str] = None,
        parent_id: Optional[str] = None,
    ) -> BacklogItem:
        item = BacklogItem(
            id=str(uuid.uuid4()),
            project_id=project_id,
            title=title,
            description=description,
            acceptance_criteria=acceptance_criteria,
            item_type=ItemType(item_type),
            priority=ItemPriority(priority),
            story_points=story_points,
            labels=labels or [],
            assigned_to=assigned_to,
            sprint_id=sprint_id,
            parent_id=parent_id,
        )
        project_store.save_item(item.to_dict())
        logger.info("BacklogManager: added item %s (%s) to project %s", item.id, item.title, project_id)
        return item

    def get_item(self, item_id: str) -> Optional[BacklogItem]:
        row = project_store.load_item(item_id)
        return BacklogItem.from_dict(row) if row else None

    def list_items(
        self,
        project_id: str,
        *,
        status: Optional[str] = None,
        sprint_id: Optional[str] = None,
        item_type: Optional[str] = None,
    ) -> List[BacklogItem]:
        rows = project_store.load_items(
            project_id,
            status=status,
            sprint_id=sprint_id,
            item_type=item_type,
        )
        items = [BacklogItem.from_dict(r) for r in rows]
        # Sort: critical → high → medium → low, then by created_at
        items.sort(
            key=lambda i: (_PRIORITY_ORDER.get(i.priority, 0), i.created_at),
            reverse=True,
        )
        return items

    def update_item(self, item_id: str, **changes) -> Optional[BacklogItem]:
        row = project_store.load_item(item_id)
        if not row:
            logger.warning("BacklogManager: item %s not found", item_id)
            return None
        item = BacklogItem.from_dict(row)
        _ENUM_FIELDS = {"item_type": ItemType, "status": ItemStatus, "priority": ItemPriority}
        for key, val in changes.items():
            if not hasattr(item, key):
                continue
            if key in _ENUM_FIELDS:
                val = _ENUM_FIELDS[key](val)
            setattr(item, key, val)
        item.updated_at = datetime.utcnow()
        project_store.save_item(item.to_dict())
        return item

    def delete_item(self, item_id: str) -> bool:
        if not project_store.load_item(item_id):
            return False
        project_store.delete_item(item_id)
        return True

    def move_to_sprint(self, item_id: str, sprint_id: Optional[str]) -> bool:
        """Assign (or unassign when sprint_id=None) an item to a sprint."""
        return self.update_item(item_id, sprint_id=sprint_id) is not None

    def backlog(self, project_id: str) -> List[BacklogItem]:
        """Items not yet assigned to any sprint."""
        return self.list_items(project_id, sprint_id="")

    # ------------------------------------------------------------------
    # AI story-point estimation
    # ------------------------------------------------------------------

    def estimate_story_points(self, item_id: str) -> Optional[int]:
        """Ask the LLM to estimate story points for a backlog item.

        Returns the estimated integer or None if LLM is unavailable.
        Points follow a Fibonacci-like scale: 1, 2, 3, 5, 8, 13.
        """
        item = self.get_item(item_id)
        if not item:
            return None

        try:
            if get_provider is None:
                raise RuntimeError("llm not available")
            provider = get_provider()
        except Exception:
            logger.debug("BacklogManager: LLM unavailable for story point estimation")
            return None

        prompt = (
            "You are an agile estimation assistant. "
            "Estimate the story points for the following work item using the Fibonacci scale "
            "(1, 2, 3, 5, 8, 13). Reply with ONLY a single integer — no explanation.\n\n"
            f"Type: {item.item_type.value}\n"
            f"Title: {item.title}\n"
        )
        if item.description:
            prompt += f"Description: {item.description}\n"
        if item.acceptance_criteria:
            prompt += f"Acceptance criteria: {item.acceptance_criteria}\n"

        try:
            response = provider.generate(prompt, max_tokens=10)
            text = response.strip().split()[0]
            points = int(text)
            valid = {1, 2, 3, 5, 8, 13}
            if points not in valid:
                # Snap to nearest valid value
                points = min(valid, key=lambda v: abs(v - points))
            self.update_item(item_id, story_points=points)
            logger.info("BacklogManager: estimated %d points for item %s", points, item_id)
            return points
        except Exception as exc:
            logger.warning("BacklogManager: point estimation failed: %s", exc)
            return None

    # ------------------------------------------------------------------
    # Sprints
    # ------------------------------------------------------------------

    def create_sprint(
        self,
        project_id: str,
        name: str,
        *,
        goal: str = "",
        start_date: Optional[datetime] = None,
        end_date: Optional[datetime] = None,
        capacity_points: Optional[int] = None,
    ) -> Sprint:
        sprint = Sprint(
            id=str(uuid.uuid4()),
            project_id=project_id,
            name=name,
            goal=goal,
            start_date=start_date,
            end_date=end_date,
            capacity_points=capacity_points,
        )
        project_store.save_sprint(sprint.to_dict())
        logger.info("BacklogManager: created sprint %s (%s)", sprint.id, sprint.name)
        return sprint

    def get_sprint(self, sprint_id: str) -> Optional[Sprint]:
        row = project_store.load_sprint(sprint_id)
        return Sprint.from_dict(row) if row else None

    def list_sprints(
        self,
        project_id: str,
        *,
        status: Optional[str] = None,
    ) -> List[Sprint]:
        rows = project_store.load_sprints(project_id, status=status)
        return [Sprint.from_dict(r) for r in rows]

    def update_sprint(self, sprint_id: str, **changes) -> Optional[Sprint]:
        row = project_store.load_sprint(sprint_id)
        if not row:
            return None
        sprint = Sprint.from_dict(row)
        for key, val in changes.items():
            if not hasattr(sprint, key):
                continue
            if key == "status":
                val = SprintStatus(val)
            setattr(sprint, key, val)
        sprint.updated_at = datetime.utcnow()
        project_store.save_sprint(sprint.to_dict())
        return sprint

    def close_sprint(self, sprint_id: str) -> Optional[Sprint]:
        """Mark sprint closed and tally completed story points."""
        sprint = self.get_sprint(sprint_id)
        if not sprint:
            return None
        completed = project_store.sprint_completed_points(sprint_id)
        return self.update_sprint(
            sprint_id,
            status=SprintStatus.CLOSED.value,
            completed_points=completed,
        )

    def delete_sprint(self, sprint_id: str) -> bool:
        if not project_store.load_sprint(sprint_id):
            return False
        project_store.delete_sprint(sprint_id)
        return True

    def active_sprint(self, project_id: str) -> Optional[Sprint]:
        sprints = self.list_sprints(project_id, status="active")
        return sprints[0] if sprints else None

    def average_velocity(self, project_id: str) -> Optional[float]:
        """Average completed points per closed sprint."""
        closed = self.list_sprints(project_id, status="closed")
        if not closed:
            return None
        total = sum(s.completed_points for s in closed)
        return round(total / len(closed), 1)
