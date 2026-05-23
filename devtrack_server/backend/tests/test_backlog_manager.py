"""
Tests for BacklogManager — backlog item and sprint CRUD.
Uses a temp SQLite DB; no real LLM calls.
"""

import pytest
from datetime import datetime
from unittest.mock import patch, MagicMock


@pytest.fixture
def db(tmp_path):
    return str(tmp_path / "test.db")


@pytest.fixture
def project(db):
    """Insert a project row so FK constraints pass."""
    from backend.db.project_store import save_project
    p = {
        "id": "proj-1",
        "name": "Test Project",
        "description": "",
        "status": "active",
        "template_type": "generic",
        "start_date": None,
        "end_date": None,
        "budget_estimate": None,
        "risk_level": "low",
        "risk_description": "",
        "external_id": None,
        "external_source": None,
        "external_sync_at": None,
        "goals": [],
        "stakeholders": [],
        "related_project_ids": [],
        "metadata": {},
        "created_at": datetime(2026, 4, 1),
        "updated_at": datetime(2026, 4, 1),
    }
    save_project(p, db)
    return "proj-1"


@pytest.fixture
def bm(db):
    """BacklogManager wired to the temp DB."""
    from backend.backlog_manager import BacklogManager
    import backend.db.project_store as store
    # Patch _db_path so all store calls use the temp DB
    with patch.object(store, "_db_path", return_value=db):
        yield BacklogManager()


# ---------------------------------------------------------------------------
# BacklogItem CRUD
# ---------------------------------------------------------------------------

def test_add_item_returns_backlog_item(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        item = bm.add_item(project, "User login")
    assert item.id
    assert item.title == "User login"
    assert item.project_id == project


def test_add_item_persists(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        item = bm.add_item(project, "Dashboard")
        fetched = bm.get_item(item.id)
    assert fetched is not None
    assert fetched.title == "Dashboard"


def test_add_item_defaults(bm, project, db):
    from backend.db import project_store as store
    from backend.models.backlog import ItemType, ItemStatus, ItemPriority
    with patch.object(store, "_db_path", return_value=db):
        item = bm.add_item(project, "Something")
    assert item.item_type == ItemType.STORY
    assert item.status == ItemStatus.OPEN
    assert item.priority == ItemPriority.MEDIUM


def test_add_item_custom_fields(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        item = bm.add_item(
            project, "Fix crash",
            item_type="bug",
            priority="critical",
            story_points=2,
            labels=["crash", "p0"],
            assigned_to="bob",
        )
    assert item.item_type.value == "bug"
    assert item.priority.value == "critical"
    assert item.story_points == 2
    assert item.labels == ["crash", "p0"]
    assert item.assigned_to == "bob"


def test_list_items_empty(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        assert bm.list_items(project) == []


def test_list_items_returns_all(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        bm.add_item(project, "A")
        bm.add_item(project, "B")
        items = bm.list_items(project)
    assert len(items) == 2


def test_list_items_filter_status(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        bm.add_item(project, "Open")
        done = bm.add_item(project, "Done")
        bm.update_item(done.id, status="done")
        open_items = bm.list_items(project, status="open")
    assert len(open_items) == 1
    assert open_items[0].title == "Open"


def test_update_item(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        item = bm.add_item(project, "Feature X")
        updated = bm.update_item(item.id, story_points=5, status="in_progress")
    assert updated.story_points == 5
    assert updated.status.value == "in_progress"


def test_update_item_missing(bm, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        result = bm.update_item("nonexistent", title="x")
    assert result is None


def test_delete_item(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        item = bm.add_item(project, "To delete")
        assert bm.delete_item(item.id) is True
        assert bm.get_item(item.id) is None


def test_delete_item_missing(bm, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        assert bm.delete_item("nope") is False


def test_move_to_sprint(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        sprint = bm.create_sprint(project, "Sprint 1")
        item = bm.add_item(project, "Feature")
        bm.move_to_sprint(item.id, sprint.id)
        fetched = bm.get_item(item.id)
    assert fetched.sprint_id == sprint.id


def test_backlog_returns_unassigned(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        sprint = bm.create_sprint(project, "Sprint 1")
        bm.add_item(project, "Unassigned")
        assigned = bm.add_item(project, "Assigned")
        bm.move_to_sprint(assigned.id, sprint.id)
        backlog = bm.backlog(project)
    assert len(backlog) == 1
    assert backlog[0].title == "Unassigned"


# ---------------------------------------------------------------------------
# Sprint CRUD
# ---------------------------------------------------------------------------

def test_create_sprint(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        sprint = bm.create_sprint(project, "Sprint 1", capacity_points=20)
    assert sprint.id
    assert sprint.name == "Sprint 1"
    assert sprint.capacity_points == 20
    assert sprint.status.value == "planned"


def test_get_sprint(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        sprint = bm.create_sprint(project, "Sprint 1")
        fetched = bm.get_sprint(sprint.id)
    assert fetched.id == sprint.id


def test_get_sprint_missing(bm, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        assert bm.get_sprint("nope") is None


def test_list_sprints(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        bm.create_sprint(project, "Sprint 1")
        bm.create_sprint(project, "Sprint 2")
        sprints = bm.list_sprints(project)
    assert len(sprints) == 2


def test_list_sprints_filter_status(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        bm.create_sprint(project, "Planned")
        s2 = bm.create_sprint(project, "Active")
        bm.update_sprint(s2.id, status="active")
        active = bm.list_sprints(project, status="active")
    assert len(active) == 1
    assert active[0].name == "Active"


def test_update_sprint(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        sprint = bm.create_sprint(project, "Sprint 1")
        updated = bm.update_sprint(sprint.id, goal="Ship auth", status="active")
    assert updated.goal == "Ship auth"
    assert updated.status.value == "active"


def test_close_sprint_tallies_points(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        sprint = bm.create_sprint(project, "Sprint 1")
        bm.update_sprint(sprint.id, status="active")
        # done item (3 pts) + in_progress item (5 pts) → only 3 counted
        done = bm.add_item(project, "Done item", story_points=3)
        bm.move_to_sprint(done.id, sprint.id)
        bm.update_item(done.id, status="done")
        wip = bm.add_item(project, "WIP item", story_points=5)
        bm.move_to_sprint(wip.id, sprint.id)
        closed = bm.close_sprint(sprint.id)
    assert closed.status.value == "closed"
    assert closed.completed_points == 3


def test_delete_sprint(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        sprint = bm.create_sprint(project, "Sprint 1")
        assert bm.delete_sprint(sprint.id) is True
        assert bm.get_sprint(sprint.id) is None


def test_active_sprint(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        bm.create_sprint(project, "Planned")
        s2 = bm.create_sprint(project, "Running")
        bm.update_sprint(s2.id, status="active")
        active = bm.active_sprint(project)
    assert active.name == "Running"


def test_average_velocity_no_closed(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        bm.create_sprint(project, "Sprint 1")
        assert bm.average_velocity(project) is None


def test_average_velocity(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db):
        s1 = bm.create_sprint(project, "S1")
        bm.update_sprint(s1.id, status="closed", completed_points=10)
        s2 = bm.create_sprint(project, "S2")
        bm.update_sprint(s2.id, status="closed", completed_points=20)
        vel = bm.average_velocity(project)
    assert vel == 15.0


# ---------------------------------------------------------------------------
# AI estimation (mocked)
# ---------------------------------------------------------------------------

def test_estimate_story_points_mocked(bm, project, db):
    from backend.db import project_store as store
    mock_provider = MagicMock()
    mock_provider.generate.return_value = "5"
    with patch.object(store, "_db_path", return_value=db), \
         patch("backend.backlog_manager.get_provider", return_value=mock_provider):
        item = bm.add_item(project, "Complex feature", description="Needs API and UI")
        pts = bm.estimate_story_points(item.id)
    assert pts == 5
    fetched = bm.get_item(item.id)
    assert fetched.story_points == 5


def test_estimate_story_points_snaps_to_fibonacci(bm, project, db):
    from backend.db import project_store as store
    mock_provider = MagicMock()
    mock_provider.generate.return_value = "4"  # not in Fibonacci — snaps to 3 or 5
    with patch.object(store, "_db_path", return_value=db), \
         patch("backend.backlog_manager.get_provider", return_value=mock_provider):
        item = bm.add_item(project, "Item")
        pts = bm.estimate_story_points(item.id)
    assert pts in {3, 5}


def test_estimate_story_points_no_llm(bm, project, db):
    from backend.db import project_store as store
    with patch.object(store, "_db_path", return_value=db), \
         patch("backend.backlog_manager.get_provider", side_effect=Exception("no llm")):
        item = bm.add_item(project, "Item")
        pts = bm.estimate_story_points(item.id)
    assert pts is None
