"""
Tests for ProjectManager and Project models

Tests the core functionality of the project management system.
"""

import pytest
from datetime import datetime, timedelta
from backend.models.project import (
    Project, ProjectStatus, ProjectTemplate, RiskLevel, ProjectGoal, ProjectStakeholder
)
from backend.project_manager import ProjectManager


class TestProjectModel:
    """Test the Project dataclass"""

    def test_project_creation(self):
        """Test creating a project"""
        project = Project(
            id="test-1",
            name="Test Project",
            description="A test project"
        )
        assert project.id == "test-1"
        assert project.name == "Test Project"
        assert project.status == ProjectStatus.SETUP

    def test_project_is_active(self):
        """Test is_active method"""
        project = Project(
            id="test-1",
            name="Active Project",
            description="",
            status=ProjectStatus.ACTIVE
        )
        assert project.is_active() is True
        assert project.is_setup() is False
        assert project.is_closed() is False

    def test_project_days_remaining(self):
        """Test days_remaining calculation"""
        project = Project(
            id="test-1",
            name="Project",
            description="",
            end_date=datetime.utcnow() + timedelta(days=10)
        )
        days = project.days_remaining()
        assert days is not None
        assert 9 <= days <= 10  # Allow for time passing during test

    def test_project_progress_percentage(self):
        """Test progress_percentage calculation"""
        now = datetime.utcnow()
        project = Project(
            id="test-1",
            name="Project",
            description="",
            start_date=now - timedelta(days=10),
            end_date=now + timedelta(days=10)
        )
        progress = project.progress_percentage()
        assert 40 <= progress <= 60  # Should be around 50%

    def test_project_to_dict(self):
        """Test serializing project to dict"""
        project = Project(
            id="test-1",
            name="Project",
            description="Test description",
            status=ProjectStatus.ACTIVE,
            template_type=ProjectTemplate.WEB_APP
        )
        data = project.to_dict()
        assert data['id'] == 'test-1'
        assert data['name'] == 'Project'
        assert data['status'] == 'active'
        assert data['template_type'] == 'web_app'

    def test_project_from_dict(self):
        """Test deserializing project from dict"""
        original = Project(
            id="test-1",
            name="Project",
            description="Test",
            status=ProjectStatus.ACTIVE
        )
        data = original.to_dict()
        restored = Project.from_dict(data)
        assert restored.id == original.id
        assert restored.name == original.name
        assert restored.status == original.status

    def test_project_add_goal(self):
        """Test adding goals to project"""
        project = Project(id="test-1", name="Project", description="")
        goal = ProjectGoal(id="goal-1", title="Complete setup", description="")
        project.add_goal(goal)
        assert len(project.goals) == 1
        assert project.goals[0].id == "goal-1"

    def test_project_add_stakeholder(self):
        """Test adding stakeholders to project"""
        project = Project(id="test-1", name="Project", description="")
        stakeholder = ProjectStakeholder(name="John", email="john@test.com")
        project.add_stakeholder(stakeholder)
        assert len(project.stakeholders) == 1
        assert project.stakeholders[0].name == "John"

    def test_project_update_status(self):
        """Test updating project status"""
        project = Project(id="test-1", name="Project", description="")
        assert project.status == ProjectStatus.SETUP
        project.mark_as_active()
        assert project.status == ProjectStatus.ACTIVE
        project.mark_as_closed()
        assert project.status == ProjectStatus.CLOSED


class TestProjectManager:
    """Test the ProjectManager class"""

    def test_manager_creation(self):
        """Test creating a ProjectManager"""
        manager = ProjectManager()
        assert manager is not None

    def test_create_project(self):
        """Test creating a project through manager"""
        manager = ProjectManager()
        project = manager.create_project(
            name="Test Project",
            description="A test project",
            template=ProjectTemplate.WEB_APP,
            ai_enhance=False
        )
        assert project is not None
        assert project.name == "Test Project"
        assert project.template_type == ProjectTemplate.WEB_APP

    def test_get_project(self):
        """Test retrieving a project"""
        manager = ProjectManager()
        project = manager.create_project(
            name="Test Project",
            description="Test",
            ai_enhance=False
        )
        retrieved = manager.get_project(project.id)
        assert retrieved is not None
        assert retrieved.id == project.id

    def test_get_nonexistent_project(self):
        """Test getting a project that doesn't exist"""
        manager = ProjectManager()
        result = manager.get_project("nonexistent")
        assert result is None

    def test_list_projects(self):
        """Test listing projects"""
        manager = ProjectManager()
        project1 = manager.create_project(
            name="Project 1",
            description="Test",
            ai_enhance=False
        )
        project2 = manager.create_project(
            name="Project 2",
            description="Test",
            ai_enhance=False
        )
        projects = manager.list_projects()
        assert len(projects) >= 2
        assert any(p.id == project1.id for p in projects)
        assert any(p.id == project2.id for p in projects)

    def test_list_projects_by_status(self):
        """Test listing projects filtered by status"""
        manager = ProjectManager()
        project = manager.create_project(
            name="Test Project",
            description="Test",
            ai_enhance=False
        )
        manager.update_project(project.id, status=ProjectStatus.ACTIVE)
        active_projects = manager.list_projects(status=ProjectStatus.ACTIVE)
        assert any(p.id == project.id for p in active_projects)

    def test_update_project(self):
        """Test updating a project"""
        manager = ProjectManager()
        project = manager.create_project(
            name="Test Project",
            description="Test",
            ai_enhance=False
        )
        updated = manager.update_project(
            project.id,
            status=ProjectStatus.ACTIVE,
            budget_estimate=5000.0
        )
        assert updated is not None
        assert updated.status == ProjectStatus.ACTIVE
        assert updated.budget_estimate == 5000.0

    def test_update_nonexistent_project(self):
        """Test updating a project that doesn't exist"""
        manager = ProjectManager()
        result = manager.update_project("nonexistent", status=ProjectStatus.ACTIVE)
        assert result is None

    def test_delete_project(self):
        """Test deleting a project"""
        manager = ProjectManager()
        project = manager.create_project(
            name="Test Project",
            description="Test",
            ai_enhance=False
        )
        result = manager.delete_project(project.id)
        assert result is True
        assert manager.get_project(project.id) is None

    def test_delete_nonexistent_project(self):
        """Test deleting a project that doesn't exist"""
        manager = ProjectManager()
        result = manager.delete_project("nonexistent")
        assert result is False

    def test_get_project_context(self):
        """Test getting comprehensive project context"""
        manager = ProjectManager()
        project = manager.create_project(
            name="Test Project",
            description="Test description",
            template=ProjectTemplate.WEB_APP,
            timeline_days=30,
            ai_enhance=False
        )
        context = manager.get_project_context(project.id)
        assert 'project' in context
        assert 'status_summary' in context
        assert 'goals_summary' in context
        assert 'timeline' in context
        assert 'related_projects' in context
        assert 'risk_assessment' in context

    def test_find_related_projects(self):
        """Test finding related projects"""
        manager = ProjectManager()
        project1 = manager.create_project(
            name="Web App 1",
            description="A web application",
            template=ProjectTemplate.WEB_APP,
            ai_enhance=False
        )
        project2 = manager.create_project(
            name="Web App 2",
            description="Another web application",
            template=ProjectTemplate.WEB_APP,
            ai_enhance=False
        )
        related = manager.find_related_projects(project1.id)
        assert len(related) > 0
        assert any(p.id == project2.id for p in related)

    def test_suggest_improvements(self):
        """Test getting improvement suggestions"""
        manager = ProjectManager()
        project = manager.create_project(
            name="Incomplete Project",
            description="Missing details",
            ai_enhance=False
        )
        suggestions = manager.suggest_project_improvements(project.id)
        assert isinstance(suggestions, list)
        # Should suggest adding stakeholders
        assert any("stakeholder" in s.lower() for s in suggestions)

    def test_project_templates(self):
        """Test that project templates are defined"""
        manager = ProjectManager()
        for template in ProjectTemplate:
            template_info = manager.get_project_template(template)
            assert 'description' in template_info
            assert 'default_goals' in template_info


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
