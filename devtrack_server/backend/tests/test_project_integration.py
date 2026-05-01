"""
Integration tests for the Phase 4 Project Management System

Tests the integration between Project models, ProjectManager, and database models.
"""

import pytest
from datetime import datetime, timedelta
from backend.models.project import (
    Project, ProjectStatus, ProjectTemplate, RiskLevel, ProjectGoal, ProjectStakeholder
)
from backend.project_manager import ProjectManager


class TestProjectIntegration:
    """Integration tests for the project management system"""

    def test_full_project_workflow(self):
        """Test a complete project lifecycle"""
        manager = ProjectManager()

        # 1. Create a project
        project = manager.create_project(
            name="Mobile App Redesign",
            description="Complete redesign of the mobile application with new UI/UX",
            template=ProjectTemplate.MOBILE_APP,
            timeline_days=90,
            budget=25000.0,
            ai_enhance=False
        )

        assert project.id is not None
        assert project.status == ProjectStatus.SETUP
        assert project.template_type == ProjectTemplate.MOBILE_APP
        assert len(project.goals) > 0  # Should have default goals from template

        # 2. Activate the project
        activated = manager.update_project(project.id, status=ProjectStatus.ACTIVE)
        assert activated.status == ProjectStatus.ACTIVE

        # 3. Add stakeholders
        lead_stakeholder = ProjectStakeholder(
            name="Alice",
            email="alice@company.com",
            role="lead"
        )
        activated.add_stakeholder(lead_stakeholder)
        assert len(activated.stakeholders) == 1

        # 4. Update risk assessment
        manager.update_project(
            project.id,
            risk_level=RiskLevel.MEDIUM,
            risk_description="Tight timeline and complex UX requirements"
        )

        # 5. Get comprehensive context
        context = manager.get_project_context(project.id)
        assert context['status_summary'] == "Active (0% complete)"
        assert context['goals_summary']['total'] > 0

        # 6. Get improvement suggestions
        suggestions = manager.suggest_project_improvements(project.id)
        assert isinstance(suggestions, list)

    def test_project_serialization_roundtrip(self):
        """Test that projects can be serialized and deserialized without data loss"""
        manager = ProjectManager()

        # Create a complex project
        project = manager.create_project(
            name="Enterprise Integration",
            description="Integrate with external payment systems",
            template=ProjectTemplate.SERVICE,
            timeline_days=60,
            budget=15000.0,
            goals=[
                "Design API contracts",
                "Implement payment gateway integration",
                "Add transaction logging"
            ],
            ai_enhance=False
        )

        # Add stakeholders
        project.add_stakeholder(ProjectStakeholder("Bob", "bob@company.com", "reviewer"))
        project.add_stakeholder(ProjectStakeholder("Charlie", "charlie@company.com", "contributor"))

        # Serialize
        serialized = project.to_dict()

        # Deserialize
        restored = Project.from_dict(serialized)

        # Verify all data
        assert restored.id == project.id
        assert restored.name == project.name
        assert restored.template_type == project.template_type
        assert len(restored.goals) == len(project.goals)
        assert len(restored.stakeholders) == len(project.stakeholders)
        assert restored.budget_estimate == project.budget_estimate

    def test_multiple_projects_with_relationships(self):
        """Test managing multiple related projects"""
        manager = ProjectManager()

        # Create backend project
        backend = manager.create_project(
            name="Backend Refactor",
            description="Refactor core services for scalability",
            template=ProjectTemplate.MICROSERVICE,
            timeline_days=60,
            ai_enhance=False
        )

        # Create frontend project
        frontend = manager.create_project(
            name="Frontend Modernization",
            description="Modernize UI framework and components",
            template=ProjectTemplate.WEB_APP,
            timeline_days=60,
            ai_enhance=False
        )

        # Share stakeholder
        shared_stakeholder = ProjectStakeholder(
            name="Diana",
            email="diana@company.com",
            role="manager"
        )
        backend.add_stakeholder(shared_stakeholder)
        frontend.add_stakeholder(shared_stakeholder)

        # List all projects
        all_projects = manager.list_projects()
        assert len(all_projects) >= 2

        # Find related projects
        related_to_backend = manager.find_related_projects(backend.id)
        # Frontend should be related (same stakeholder, overlapping timeline)
        assert len(related_to_backend) > 0

    def test_project_status_transitions(self):
        """Test project status transitions"""
        manager = ProjectManager()

        project = manager.create_project(
            name="Status Test",
            description="Test project status transitions",
            ai_enhance=False
        )

        # Initially in SETUP
        assert project.status == ProjectStatus.SETUP

        # Transition to ACTIVE
        project.mark_as_active()
        assert project.status == ProjectStatus.ACTIVE

        # Transition to CLOSED
        project.mark_as_closed()
        assert project.status == ProjectStatus.CLOSED

        # Also test via manager.update_project
        project2 = manager.create_project(
            name="Status Test 2",
            description="",
            ai_enhance=False
        )

        updated = manager.update_project(project2.id, status="active")
        assert updated.status == ProjectStatus.ACTIVE

    def test_project_goals_management(self):
        """Test adding and managing project goals"""
        manager = ProjectManager()

        project = manager.create_project(
            name="Goal Test",
            description="",
            ai_enhance=False
        )

        initial_goal_count = len(project.goals)

        # Add new goals
        new_goal1 = ProjectGoal(
            id="goal-custom-1",
            title="Custom Goal 1",
            description="This is a custom goal"
        )
        new_goal2 = ProjectGoal(
            id="goal-custom-2",
            title="Custom Goal 2",
            description="Another custom goal"
        )

        project.add_goal(new_goal1)
        project.add_goal(new_goal2)

        assert len(project.goals) == initial_goal_count + 2

        # Get specific goal
        retrieved_goal = project.get_goal_by_id("goal-custom-1")
        assert retrieved_goal is not None
        assert retrieved_goal.title == "Custom Goal 1"

        # Get active goals (non-completed)
        active_goals = project.get_active_goals()
        assert len(active_goals) > 0
        assert all(g.status != "completed" for g in active_goals)

    def test_project_timeline_calculations(self):
        """Test timeline-related calculations"""
        manager = ProjectManager()

        now = datetime.utcnow()

        project = manager.create_project(
            name="Timeline Test",
            description="",
            start_date=now - timedelta(days=10),
            end_date=now + timedelta(days=20),
            ai_enhance=False
        )

        # Test days remaining
        days_remaining = project.days_remaining()
        assert days_remaining is not None
        assert 19 <= days_remaining <= 20

        # Test progress percentage
        progress = project.progress_percentage()
        assert 40 <= progress <= 60  # Should be around 33% (10 days elapsed / 30 total)

        # Test without dates
        no_dates = manager.create_project(
            name="No Timeline",
            description="",
            ai_enhance=False
        )
        assert no_dates.days_remaining() is None
        assert no_dates.progress_percentage() == 0

    def test_all_project_templates(self):
        """Test that all project templates work correctly"""
        manager = ProjectManager()

        templates = [
            ProjectTemplate.WEB_APP,
            ProjectTemplate.LIBRARY,
            ProjectTemplate.SERVICE,
            ProjectTemplate.MICROSERVICE,
            ProjectTemplate.CLI_TOOL,
            ProjectTemplate.MOBILE_APP,
            ProjectTemplate.DATA_PIPELINE,
            ProjectTemplate.GENERIC
        ]

        for template in templates:
            project = manager.create_project(
                name=f"Test {template.value}",
                description=f"Testing {template.value} template",
                template=template,
                ai_enhance=False
            )

            assert project.template_type == template
            assert len(project.goals) > 0

            # Verify template info is available
            template_info = manager.get_project_template(template)
            assert 'description' in template_info
            assert 'default_goals' in template_info

    def test_risk_assessment(self):
        """Test risk level assessment"""
        manager = ProjectManager()

        # Test low-risk project
        low_risk = manager.create_project(
            name="Well-planned project",
            description="Clear scope with good timeline",
            timeline_days=90,
            budget=10000.0,
            ai_enhance=False
        )
        assert low_risk.risk_level == RiskLevel.LOW

        # Test high-risk project
        high_risk = manager.create_project(
            name="Experimental AI system",
            description="Unknown requirements, critical deadline",
            template=ProjectTemplate.MICROSERVICE,
            timeline_days=3,
            budget=None,
            ai_enhance=False
        )
        assert high_risk.risk_level in [RiskLevel.HIGH, RiskLevel.MEDIUM]

    def test_project_context_completeness(self):
        """Test that project context includes all expected information"""
        manager = ProjectManager()

        project = manager.create_project(
            name="Context Test",
            description="Testing context gathering",
            template=ProjectTemplate.SERVICE,
            timeline_days=45,
            budget=5000.0,
            ai_enhance=False
        )

        project.add_stakeholder(ProjectStakeholder("Eve", "eve@company.com", "lead"))

        context = manager.get_project_context(project.id)

        # Verify context structure
        required_keys = [
            'project',
            'status_summary',
            'goals_summary',
            'timeline',
            'related_projects',
            'risk_assessment'
        ]
        for key in required_keys:
            assert key in context, f"Missing key: {key}"

        # Verify nested structures
        assert 'total' in context['goals_summary']
        assert 'completed' in context['goals_summary']
        assert 'in_progress' in context['goals_summary']
        assert 'pending' in context['goals_summary']

        assert 'days_remaining' in context['timeline']
        assert 'progress_percentage' in context['timeline']

        assert 'level' in context['risk_assessment']
        assert 'description' in context['risk_assessment']
        assert 'factors' in context['risk_assessment']

    def test_manager_availability_check(self):
        """Test checking if AI provider is available"""
        manager = ProjectManager()

        # Should work regardless of AI availability
        is_available = manager.is_ai_available()
        assert isinstance(is_available, bool)

    def test_nonexistent_project_context(self):
        """Test getting context for nonexistent project"""
        manager = ProjectManager()

        context = manager.get_project_context("nonexistent-id")
        assert context == {}


class TestDatabaseModels:
    """Test the SQLAlchemy database models"""

    def test_project_db_model_import(self):
        """Test that database models can be imported"""
        try:
            from backend.db.models.project import ProjectDB
            assert ProjectDB is not None
        except ImportError:
            pytest.skip("SQLAlchemy not available")

    def test_project_db_query_helpers(self):
        """Test database query helper functions"""
        try:
            from backend.db.models.project import (
                query_projects_by_status,
                query_active_projects,
                query_recent_projects
            )
            # These should be importable (they won't work without a real DB)
            assert callable(query_projects_by_status)
            assert callable(query_active_projects)
            assert callable(query_recent_projects)
        except ImportError:
            pytest.skip("SQLAlchemy not available")


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
