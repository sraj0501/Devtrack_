"""
Project Data Model

This module defines the Project dataclass used throughout the application
for representing projects with their metadata, status, and context.
"""

from dataclasses import dataclass, field, asdict
from datetime import datetime
from typing import Optional, List, Dict, Any
from enum import Enum


class ProjectStatus(str, Enum):
    """Project lifecycle status"""
    SETUP = "setup"  # Project being configured
    ACTIVE = "active"  # Project in progress
    CLOSED = "closed"  # Project completed


class ProjectTemplate(str, Enum):
    """Project template types for quick setup"""
    WEB_APP = "web_app"
    LIBRARY = "library"
    SERVICE = "service"
    MICROSERVICE = "microservice"
    CLI_TOOL = "cli_tool"
    MOBILE_APP = "mobile_app"
    DATA_PIPELINE = "data_pipeline"
    GENERIC = "generic"


class RiskLevel(str, Enum):
    """Project risk assessment levels"""
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"


@dataclass
class ProjectGoal:
    """Individual project goal or objective"""
    id: str
    title: str
    description: str
    status: str = "pending"  # pending, in_progress, completed
    priority: int = 1  # 1-5, higher = more important


@dataclass
class ProjectStakeholder:
    """Project stakeholder information"""
    name: str
    email: Optional[str] = None
    role: str = "contributor"  # contributor, reviewer, lead, manager, sponsor


@dataclass
class Project:
    """
    Represents a project with all its metadata and context.

    This dataclass is the primary model for projects in DevTrack,
    used throughout the application for project management operations.
    """

    # Identification
    id: str
    name: str
    description: str

    # Status and timeline
    status: ProjectStatus = ProjectStatus.SETUP
    start_date: Optional[datetime] = None
    end_date: Optional[datetime] = None

    # Project context
    template_type: ProjectTemplate = ProjectTemplate.GENERIC
    goals: List[ProjectGoal] = field(default_factory=list)
    stakeholders: List[ProjectStakeholder] = field(default_factory=list)

    # Resource and risk
    budget_estimate: Optional[float] = None  # In currency units
    risk_level: RiskLevel = RiskLevel.MEDIUM
    risk_description: str = ""

    # External sync tracking
    external_id: Optional[str] = None  # Azure iteration path, Jira project key, etc.
    external_source: Optional[str] = None  # "azure_devops", "jira", "github"
    external_sync_at: Optional[datetime] = None  # Last successful sync timestamp

    # Metadata
    created_at: datetime = field(default_factory=datetime.utcnow)
    updated_at: datetime = field(default_factory=datetime.utcnow)
    metadata: Dict[str, Any] = field(default_factory=dict)

    # Relations (IDs only, objects loaded separately)
    related_project_ids: List[str] = field(default_factory=list)

    def is_active(self) -> bool:
        """Check if project is currently active."""
        return self.status == ProjectStatus.ACTIVE

    def is_setup(self) -> bool:
        """Check if project is still in setup phase."""
        return self.status == ProjectStatus.SETUP

    def is_closed(self) -> bool:
        """Check if project has been completed/closed."""
        return self.status == ProjectStatus.CLOSED

    def days_remaining(self) -> Optional[int]:
        """
        Calculate days remaining until end_date.

        Returns:
            Number of days remaining, or None if end_date not set.
            Returns 0 if end_date is today or past.
        """
        if not self.end_date:
            return None

        delta = self.end_date - datetime.utcnow()
        days = max(0, delta.days)
        return days

    def progress_percentage(self) -> int:
        """
        Calculate project progress as percentage (0-100) based on timeline.

        Assumes linear progress: (days_elapsed / total_days) * 100

        Returns:
            Integer percentage (0-100), or 0 if dates not set.
        """
        if not self.start_date or not self.end_date:
            return 0

        total_duration = (self.end_date - self.start_date).total_seconds()
        if total_duration <= 0:
            return 0

        elapsed = (datetime.utcnow() - self.start_date).total_seconds()
        if elapsed <= 0:
            return 0

        percentage = min(100, int((elapsed / total_duration) * 100))
        return percentage

    def to_dict(self) -> Dict[str, Any]:
        """
        Convert project to dictionary representation.

        Handles enum and datetime serialization.
        """
        data = asdict(self)

        # Convert enums to strings
        data['status'] = self.status.value
        data['template_type'] = self.template_type.value
        data['risk_level'] = self.risk_level.value

        # Convert datetime to ISO format strings
        if self.created_at:
            data['created_at'] = self.created_at.isoformat()
        if self.updated_at:
            data['updated_at'] = self.updated_at.isoformat()
        if self.start_date:
            data['start_date'] = self.start_date.isoformat()
        if self.end_date:
            data['end_date'] = self.end_date.isoformat()
        if self.external_sync_at:
            data['external_sync_at'] = self.external_sync_at.isoformat()

        # Convert nested dataclasses
        if self.goals:
            data['goals'] = [asdict(g) for g in self.goals]
        if self.stakeholders:
            data['stakeholders'] = [asdict(s) for s in self.stakeholders]

        return data

    @staticmethod
    def from_dict(data: Dict[str, Any]) -> 'Project':
        """
        Create a Project instance from a dictionary.

        Handles deserialization of enums and datetime objects.
        """
        # Make a copy to avoid modifying original
        d = dict(data)

        # Convert string enums back to enum instances
        if isinstance(d.get('status'), str):
            d['status'] = ProjectStatus(d['status'])
        if isinstance(d.get('template_type'), str):
            d['template_type'] = ProjectTemplate(d['template_type'])
        if isinstance(d.get('risk_level'), str):
            d['risk_level'] = RiskLevel(d['risk_level'])

        # Convert ISO format strings back to datetime
        for date_field in ['created_at', 'updated_at', 'start_date', 'end_date', 'external_sync_at']:
            if isinstance(d.get(date_field), str):
                d[date_field] = datetime.fromisoformat(d[date_field])

        # Reconstruct goal objects
        if d.get('goals'):
            d['goals'] = [
                ProjectGoal(**g) if isinstance(g, dict) else g
                for g in d['goals']
            ]

        # Reconstruct stakeholder objects
        if d.get('stakeholders'):
            d['stakeholders'] = [
                ProjectStakeholder(**s) if isinstance(s, dict) else s
                for s in d['stakeholders']
            ]

        return Project(**d)

    def add_goal(self, goal: ProjectGoal) -> None:
        """Add a goal to the project."""
        if goal not in self.goals:
            self.goals.append(goal)
            self.updated_at = datetime.utcnow()

    def add_stakeholder(self, stakeholder: ProjectStakeholder) -> None:
        """Add a stakeholder to the project."""
        if stakeholder not in self.stakeholders:
            self.stakeholders.append(stakeholder)
            self.updated_at = datetime.utcnow()

    def update_status(self, new_status: ProjectStatus) -> None:
        """Update project status."""
        self.status = new_status
        self.updated_at = datetime.utcnow()

    def mark_as_active(self) -> None:
        """Mark project as active."""
        self.update_status(ProjectStatus.ACTIVE)

    def mark_as_closed(self) -> None:
        """Mark project as closed/completed."""
        self.update_status(ProjectStatus.CLOSED)

    def get_goal_by_id(self, goal_id: str) -> Optional[ProjectGoal]:
        """Get a goal by its ID."""
        for goal in self.goals:
            if goal.id == goal_id:
                return goal
        return None

    def get_active_goals(self) -> List[ProjectGoal]:
        """Get all non-completed goals."""
        return [g for g in self.goals if g.status != "completed"]
