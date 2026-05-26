"""
Project Manager

This module provides the ProjectManager class for creating, managing, and querying
projects in DevTrack. It serves as the main interface for project operations.
"""

import logging
import uuid
from datetime import datetime, timedelta
from typing import Optional, List, Dict, Any

from backend.models.project import (
    Project, ProjectStatus, ProjectTemplate, RiskLevel, ProjectGoal, ProjectStakeholder
)

logger = logging.getLogger(__name__)

try:
    from backend.db import project_store as _project_store
    _store_available = True
except Exception:
    _project_store = None  # type: ignore
    _store_available = False

try:
    from backend.personalization import inject_style as _inject_style
except ImportError:
    def _inject_style(prompt: str, context_type: str = "general") -> str:
        return prompt


class ProjectManager:
    """
    Manages project lifecycle and operations.

    Handles:
    - Project creation with optional AI-guided setup
    - CRUD operations on projects
    - Project context gathering
    - Risk assessment
    - Related projects discovery
    """

    # Default project templates with suggested goals
    TEMPLATES = {
        ProjectTemplate.WEB_APP: {
            "description": "Web application with frontend and backend",
            "default_goals": [
                "Design and plan architecture",
                "Implement core features",
                "Add comprehensive tests",
                "Deploy to production",
                "Monitor and optimize performance"
            ]
        },
        ProjectTemplate.LIBRARY: {
            "description": "Reusable library or package",
            "default_goals": [
                "Design public API",
                "Implement core functionality",
                "Write documentation and examples",
                "Release first version",
                "Gather user feedback"
            ]
        },
        ProjectTemplate.SERVICE: {
            "description": "Microservice or backend service",
            "default_goals": [
                "Define service contract and APIs",
                "Implement core logic",
                "Add monitoring and logging",
                "Deploy and integrate",
                "Optimize for scale"
            ]
        },
        ProjectTemplate.MICROSERVICE: {
            "description": "Microservice within larger system",
            "default_goals": [
                "Understand dependencies and contracts",
                "Implement business logic",
                "Add health checks",
                "Integrate with system",
                "Load test and optimize"
            ]
        },
        ProjectTemplate.CLI_TOOL: {
            "description": "Command-line tool or utility",
            "default_goals": [
                "Design command structure",
                "Implement core commands",
                "Add help and documentation",
                "Test on multiple platforms",
                "Package for distribution"
            ]
        },
        ProjectTemplate.MOBILE_APP: {
            "description": "Mobile application",
            "default_goals": [
                "Design UI/UX",
                "Implement features",
                "Add offline support",
                "Test on devices",
                "Submit to app store"
            ]
        },
        ProjectTemplate.DATA_PIPELINE: {
            "description": "Data processing or ETL pipeline",
            "default_goals": [
                "Define data schema",
                "Implement processors",
                "Add data validation",
                "Set up monitoring",
                "Optimize performance"
            ]
        },
        ProjectTemplate.GENERIC: {
            "description": "Generic project (no template)",
            "default_goals": [
                "Define project scope",
                "Break down into milestones",
                "Execute work",
                "Deliver results",
                "Reflect and improve"
            ]
        }
    }

    def __init__(self, provider=None):
        """
        Initialize ProjectManager.

        Args:
            provider: Optional LLM provider for AI-guided features.
                      If not provided, some AI features will be unavailable.
        """
        self._provider = provider
        self._projects: Dict[str, Project] = {}  # In-memory cache
        self._sync_provider = None  # External sync provider (e.g. AzureProjectSync)
        self._load_from_db()
        logger.info("ProjectManager initialized")

    def _load_from_db(self) -> None:
        """Populate in-memory cache from SQLite on startup."""
        if not _store_available:
            return
        try:
            rows = _project_store.load_all_projects()
            for row in rows:
                project = self._row_to_project(row)
                self._projects[project.id] = project
            if rows:
                logger.info("ProjectManager: loaded %d project(s) from DB", len(rows))
        except Exception as exc:
            logger.warning("ProjectManager: could not load from DB: %s", exc)

    def _row_to_project(self, row: Dict[str, Any]) -> Project:
        """Convert a project_store row dict back to a Project dataclass."""
        def _dt(v):
            if v is None:
                return None
            if isinstance(v, datetime):
                return v
            try:
                return datetime.fromisoformat(v)
            except Exception:
                return None

        goals = [
            ProjectGoal(description=g) if isinstance(g, str) else ProjectGoal(**g)
            for g in (row.get("goals") or [])
        ]
        stakeholders = [
            ProjectStakeholder(**s) if isinstance(s, dict) else s
            for s in (row.get("stakeholders") or [])
        ]
        return Project(
            id=row["id"],
            name=row["name"],
            description=row.get("description", ""),
            status=ProjectStatus(row.get("status", "setup")),
            template_type=ProjectTemplate(row["template_type"]) if row.get("template_type") else ProjectTemplate.GENERIC,
            start_date=_dt(row.get("start_date")),
            end_date=_dt(row.get("end_date")),
            goals=goals,
            stakeholders=stakeholders,
            budget_estimate=row.get("budget_estimate"),
            risk_level=RiskLevel(row.get("risk_level", "low")),
            risk_description=row.get("risk_description", ""),
            external_id=row.get("external_id"),
            external_source=row.get("external_source"),
            external_sync_at=_dt(row.get("external_sync_at")),
            related_project_ids=row.get("related_project_ids") or [],
            metadata=row.get("metadata") or {},
            created_at=_dt(row.get("created_at")) or datetime.utcnow(),
            updated_at=_dt(row.get("updated_at")) or datetime.utcnow(),
        )

    def _save_to_db(self, project: Project) -> None:
        if not _store_available:
            return
        try:
            _project_store.save_project(project.to_dict())
        except Exception as exc:
            logger.warning("ProjectManager: DB save failed for %s: %s", project.id, exc)

    def _delete_from_db(self, project_id: str) -> None:
        if not _store_available:
            return
        try:
            _project_store.delete_project(project_id)
        except Exception as exc:
            logger.warning("ProjectManager: DB delete failed for %s: %s", project_id, exc)

    def _get_provider(self):
        """Lazily load LLM provider if needed."""
        if self._provider is None:
            try:
                from backend.llm import get_provider
                self._provider = get_provider()
            except (ImportError, Exception) as e:
                logger.warning(f"Could not load LLM provider: {e}")
                self._provider = False  # Cache that it's unavailable
        return self._provider if self._provider is not False else None

    def is_ai_available(self) -> bool:
        """Check if LLM provider is available."""
        provider = self._get_provider()
        return provider is not None and provider.primary.is_available()

    def create_project(
        self,
        name: str,
        description: str,
        template: ProjectTemplate = ProjectTemplate.GENERIC,
        goals: Optional[List[str]] = None,
        timeline_days: Optional[int] = None,
        budget: Optional[float] = None,
        stakeholders: Optional[List[ProjectStakeholder]] = None,
        ai_enhance: bool = True,
        start_date: Optional[datetime] = None,
        end_date: Optional[datetime] = None,
    ) -> Project:
        """
        Create a new project.

        Args:
            name: Project name
            description: Project description
            template: Project template type
            goals: List of goal descriptions (if None, uses template defaults)
            timeline_days: Estimated duration in days
            budget: Budget estimate
            stakeholders: List of stakeholders
            ai_enhance: Whether to use AI for risk assessment

        Returns:
            Created Project instance
        """
        project_id = str(uuid.uuid4())

        # Use template defaults if goals not provided
        if goals is None:
            template_info = self.TEMPLATES.get(template, {})
            goal_descriptions = template_info.get("default_goals", [])
        else:
            goal_descriptions = goals

        # Create goal objects
        project_goals = [
            ProjectGoal(
                id=str(uuid.uuid4()),
                title=goal,
                description=""
            )
            for goal in goal_descriptions
        ]

        # Calculate end date if timeline provided (only if not explicitly given)
        if end_date is None and timeline_days:
            end_date = datetime.utcnow() + timedelta(days=timeline_days)

        # Assess risk
        risk_level = self._assess_risk(description, template, timeline_days, budget)

        project = Project(
            id=project_id,
            name=name,
            description=description,
            template_type=template,
            goals=project_goals,
            status=ProjectStatus.SETUP,
            start_date=start_date,
            end_date=end_date,
            budget_estimate=budget,
            risk_level=risk_level,
            stakeholders=stakeholders or []
        )

        # AI enhancement if available and requested
        if ai_enhance and self.is_ai_available():
            self._enhance_project_with_ai(project)

        # Persist and cache
        self._save_to_db(project)
        self._projects[project_id] = project

        logger.info(f"Created project {project_id}: {name} (template: {template.value})")
        return project

    def get_project(self, project_id: str) -> Optional[Project]:
        """
        Get a project by ID.

        Args:
            project_id: Project ID

        Returns:
            Project instance or None if not found
        """
        return self._projects.get(project_id)

    def list_projects(
        self,
        status: Optional[ProjectStatus] = None,
        template: Optional[ProjectTemplate] = None
    ) -> List[Project]:
        """
        List projects with optional filtering.

        Args:
            status: Filter by project status
            template: Filter by template type

        Returns:
            List of Project instances
        """
        projects = list(self._projects.values())

        if status:
            projects = [p for p in projects if p.status == status]

        if template:
            projects = [p for p in projects if p.template_type == template]

        return sorted(projects, key=lambda p: p.created_at, reverse=True)

    def update_project(self, project_id: str, **changes) -> Optional[Project]:
        """
        Update a project with the given changes.

        Args:
            project_id: Project ID
            **changes: Fields to update

        Returns:
            Updated Project instance or None if not found

        Example:
            project = manager.update_project(project_id, status="active", budget_estimate=10000)
        """
        project = self.get_project(project_id)
        if not project:
            logger.warning(f"Project {project_id} not found for update")
            return None

        # Handle special case conversions
        if 'status' in changes and isinstance(changes['status'], str):
            changes['status'] = ProjectStatus(changes['status'])

        if 'template_type' in changes and isinstance(changes['template_type'], str):
            changes['template_type'] = ProjectTemplate(changes['template_type'])

        if 'risk_level' in changes and isinstance(changes['risk_level'], str):
            changes['risk_level'] = RiskLevel(changes['risk_level'])

        # Apply changes
        for key, value in changes.items():
            if hasattr(project, key):
                setattr(project, key, value)

        project.updated_at = datetime.utcnow()
        self._save_to_db(project)
        logger.info(f"Updated project {project_id}")
        return project

    def delete_project(self, project_id: str) -> bool:
        """
        Delete a project.

        Args:
            project_id: Project ID

        Returns:
            True if deleted, False if not found
        """
        if project_id not in self._projects:
            logger.warning(f"Project {project_id} not found for deletion")
            return False

        del self._projects[project_id]
        self._delete_from_db(project_id)
        logger.info(f"Deleted project {project_id}")
        return True

    def get_project_context(self, project_id: str) -> Dict[str, Any]:
        """
        Get comprehensive context for a project.

        Gathers all project metadata, goals, stakeholders, and related information.

        Args:
            project_id: Project ID

        Returns:
            Dictionary with full project context
        """
        project = self.get_project(project_id)
        if not project:
            return {}

        context = {
            "project": project.to_dict(),
            "status_summary": self._get_status_summary(project),
            "goals_summary": {
                "total": len(project.goals),
                "completed": len([g for g in project.goals if g.status == "completed"]),
                "in_progress": len([g for g in project.goals if g.status == "in_progress"]),
                "pending": len([g for g in project.goals if g.status == "pending"])
            },
            "timeline": {
                "days_remaining": project.days_remaining(),
                "progress_percentage": project.progress_percentage()
            },
            "related_projects": self._find_related_projects(project),
            "risk_assessment": self._get_risk_assessment(project)
        }

        return context

    def find_related_projects(self, project_id: str, max_results: int = 5) -> List[Project]:
        """
        Find projects related to the given project.

        Relationships based on:
        - Similar template type
        - Overlapping timelines
        - Shared stakeholders
        - Similar descriptions

        Args:
            project_id: Project ID
            max_results: Maximum number of related projects to return

        Returns:
            List of related Project instances
        """
        project = self.get_project(project_id)
        if not project:
            return []

        return self._find_related_projects(project, max_results)

    def get_project_template(self, template: ProjectTemplate) -> Dict[str, Any]:
        """
        Get template information.

        Args:
            template: Template type

        Returns:
            Dictionary with template details
        """
        return self.TEMPLATES.get(template, {})

    def suggest_project_improvements(self, project_id: str) -> List[str]:
        """
        Suggest improvements to the project setup.

        Uses AI if available to analyze project and provide recommendations.

        Args:
            project_id: Project ID

        Returns:
            List of improvement suggestions
        """
        project = self.get_project(project_id)
        if not project:
            return []

        suggestions = []

        # Basic heuristic suggestions
        if not project.stakeholders:
            suggestions.append("Consider adding stakeholders to the project")

        if not project.end_date:
            suggestions.append("Set an end date to track project timeline")

        if len(project.goals) < 3:
            suggestions.append("Break down the project into more specific goals")

        if project.risk_level == RiskLevel.HIGH and not project.risk_description:
            suggestions.append("Document risks identified in the project")

        if project.budget_estimate is None:
            suggestions.append("Estimate project budget for better tracking")

        # AI suggestions if available
        if self.is_ai_available():
            ai_suggestions = self._get_ai_suggestions(project)
            suggestions.extend(ai_suggestions)

        return suggestions

    def set_external_sync(self, sync_provider) -> None:
        """
        Set the external sync provider for bidirectional project synchronization.

        Args:
            sync_provider: An object implementing sync_project_to_azure(project_id)
                           (e.g. AzureProjectSync). Must have async methods.
        """
        self._sync_provider = sync_provider
        logger.info("External sync provider set: %s", type(sync_provider).__name__)

    async def sync_to_external(self, project_id: str) -> bool:
        """
        Sync a local project to the external system via the configured provider.

        Args:
            project_id: Project ID to sync.

        Returns:
            True if sync succeeded, False if no provider or sync failed.
        """
        if not self._sync_provider:
            logger.debug("sync_to_external: no sync provider configured")
            return False

        project = self.get_project(project_id)
        if not project:
            logger.warning("sync_to_external: project %s not found", project_id)
            return False

        try:
            return await self._sync_provider.sync_project_to_azure(project_id)
        except Exception as e:
            logger.error("sync_to_external failed for %s: %s", project_id, e)
            return False

    # Private helper methods

    def _assess_risk(
        self,
        description: str,
        template: ProjectTemplate,
        timeline_days: Optional[int],
        budget: Optional[float]
    ) -> RiskLevel:
        """
        Assess project risk based on description and parameters.

        Simple heuristic-based risk assessment.
        """
        risk_indicators = 0

        # Check for high-risk keywords
        high_risk_keywords = [
            "unknown", "uncertain", "experimental", "critical",
            "complex", "large-scale", "distributed", "legacy"
        ]
        if any(keyword in description.lower() for keyword in high_risk_keywords):
            risk_indicators += 1

        # Short timeline is riskier
        if timeline_days and timeline_days < 7:
            risk_indicators += 1

        # No budget estimate is riskier
        if budget is None:
            risk_indicators += 1

        # Certain templates are inherently riskier
        risky_templates = [
            ProjectTemplate.MICROSERVICE,
            ProjectTemplate.DATA_PIPELINE
        ]
        if template in risky_templates:
            risk_indicators += 1

        if risk_indicators >= 3:
            return RiskLevel.HIGH
        elif risk_indicators >= 1:
            return RiskLevel.MEDIUM
        else:
            return RiskLevel.LOW

    def _enhance_project_with_ai(self, project: Project) -> None:
        """
        Enhance project details using AI.

        Improves goals, risk assessment, and other fields based on LLM analysis.
        """
        try:
            provider = self._get_provider()
            if not provider:
                return

            # Ask LLM to analyze and refine goals
            prompt = f"""Analyze this project and suggest 2-3 key goals:

Project: {project.name}
Description: {project.description}
Template: {project.template_type.value}

Provide concise, measurable goals (one per line)."""

            prompt = _inject_style(prompt, context_type="task")
            response = provider.primary.generate(prompt)
            if response and response.strip():
                ai_goals = response.strip().split('\n')
                # Add AI-suggested goals (keep existing ones)
                for goal_text in ai_goals[:3]:  # Limit to 3 new goals
                    if goal_text.strip():
                        project.add_goal(ProjectGoal(
                            id=str(uuid.uuid4()),
                            title=goal_text.strip(),
                            description=""
                        ))

            logger.info(f"Enhanced project {project.id} with AI suggestions")

        except Exception as e:
            logger.warning(f"Error enhancing project with AI: {e}")

    def _get_status_summary(self, project: Project) -> str:
        """Get human-readable status summary."""
        if project.is_closed():
            return "Completed"
        elif project.is_active():
            progress = project.progress_percentage()
            return f"Active ({progress}% complete)"
        else:
            return "Setup in progress"

    def _find_related_projects(
        self,
        project: Project,
        max_results: int = 5
    ) -> List[Project]:
        """Find projects related to this one."""
        related = []
        all_projects = list(self._projects.values())

        # Remove current project
        all_projects = [p for p in all_projects if p.id != project.id]

        # Score each project for relevance
        scores = []
        for other in all_projects:
            score = 0

            # Same template type
            if other.template_type == project.template_type:
                score += 10

            # Overlapping timelines
            if (project.start_date and other.start_date and
                project.end_date and other.end_date):
                if (project.start_date <= other.end_date and
                    project.end_date >= other.start_date):
                    score += 5

            # Shared stakeholders
            shared = len(set(s.email or s.name for s in project.stakeholders) &
                        set(s.email or s.name for s in other.stakeholders))
            score += shared * 3

            # Similar description (simple keyword overlap)
            project_words = set(project.description.lower().split())
            other_words = set(other.description.lower().split())
            overlap = len(project_words & other_words)
            if overlap > 0:
                score += overlap

            if score > 0:
                scores.append((other, score))

        # Sort by score and return top results
        scores.sort(key=lambda x: x[1], reverse=True)
        return [p for p, _ in scores[:max_results]]

    def _get_risk_assessment(self, project: Project) -> Dict[str, Any]:
        """Get detailed risk assessment."""
        return {
            "level": project.risk_level.value,
            "description": project.risk_description,
            "factors": self._identify_risk_factors(project)
        }

    def _identify_risk_factors(self, project: Project) -> List[str]:
        """Identify specific risk factors for the project."""
        factors = []

        if not project.stakeholders:
            factors.append("No stakeholders defined")

        if not project.goals:
            factors.append("No goals defined")

        days_remaining = project.days_remaining()
        if days_remaining is not None and days_remaining < 7:
            factors.append(f"Very short timeline ({days_remaining} days remaining)")

        if project.budget_estimate is None:
            factors.append("Budget not estimated")

        if not project.start_date:
            factors.append("Start date not set")

        return factors

    def _get_ai_suggestions(self, project: Project) -> List[str]:
        """Get AI-powered improvement suggestions."""
        suggestions = []

        try:
            provider = self._get_provider()
            if not provider:
                return suggestions

            prompt = f"""Provide 2 specific improvement suggestions for this project:

Name: {project.name}
Status: {project.status.value}
Risk Level: {project.risk_level.value}
Goals: {len(project.goals)} defined
Stakeholders: {len(project.stakeholders)} assigned

Be concise and specific."""

            prompt = _inject_style(prompt, context_type="comment")
            response = provider.primary.generate(prompt)
            if response and response.strip():
                for line in response.strip().split('\n'):
                    if line.strip():
                        suggestions.append(line.strip())

        except Exception as e:
            logger.debug(f"Error getting AI suggestions: {e}")

        return suggestions
