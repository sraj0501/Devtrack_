"""
Tests for backend.db.project_store — SQLite CRUD for projects, items, sprints.
Uses an in-memory (tmp) database so tests are hermetic and fast.
"""

import pytest
import tempfile
import os
from datetime import datetime


@pytest.fixture
def db(tmp_path):
    """Return a fresh temp database path for each test."""
    return str(tmp_path / "test.db")


# ---------------------------------------------------------------------------
# Schema init
# ---------------------------------------------------------------------------

def test_init_schema_idempotent(db):
    from backend.db.project_store import init_schema
    init_schema(db)
    init_schema(db)  # second call must not raise


# ---------------------------------------------------------------------------
# Project CRUD
# ---------------------------------------------------------------------------

def _project(id="p1", name="Alpha"):
    return {
        "id": id,
        "name": name,
        "description": "Test project",
        "status": "setup",
        "template_type": "generic",
        "start_date": None,
        "end_date": None,
        "budget_estimate": None,
        "risk_level": "low",
        "risk_description": "",
        "external_id": None,
        "external_source": None,
        "external_sync_at": None,
        "goals": [{"description": "Ship v1", "status": "pending", "priority": "high", "target_date": None}],
        "stakeholders": [],
        "related_project_ids": [],
        "metadata": {},
        "created_at": datetime(2026, 4, 1, 10, 0, 0),
        "updated_at": datetime(2026, 4, 1, 10, 0, 0),
    }


def test_save_and_load_project(db):
    from backend.db.project_store import save_project, load_project
    p = _project()
    save_project(p, db)
    row = load_project("p1", db)
    assert row is not None
    assert row["name"] == "Alpha"
    assert row["status"] == "setup"


def test_load_project_missing(db):
    from backend.db.project_store import load_project, init_schema
    init_schema(db)
    assert load_project("nonexistent", db) is None


def test_save_project_upserts(db):
    from backend.db.project_store import save_project, load_project
    p = _project()
    save_project(p, db)
    p["name"] = "Alpha v2"
    p["status"] = "active"
    save_project(p, db)
    row = load_project("p1", db)
    assert row["name"] == "Alpha v2"
    assert row["status"] == "active"


def test_load_all_projects(db):
    from backend.db.project_store import save_project, load_all_projects
    save_project(_project("p1", "Alpha"), db)
    save_project(_project("p2", "Beta"), db)
    rows = load_all_projects(db)
    assert len(rows) == 2
    names = {r["name"] for r in rows}
    assert names == {"Alpha", "Beta"}


def test_delete_project(db):
    from backend.db.project_store import save_project, load_project, delete_project
    save_project(_project(), db)
    delete_project("p1", db)
    assert load_project("p1", db) is None


def test_project_goals_roundtrip(db):
    from backend.db.project_store import save_project, load_project
    p = _project()
    p["goals"] = [{"description": "Goal A", "status": "pending", "priority": "high", "target_date": None}]
    save_project(p, db)
    row = load_project("p1", db)
    assert row["goals"][0]["description"] == "Goal A"


# ---------------------------------------------------------------------------
# BacklogItem CRUD
# ---------------------------------------------------------------------------

def _item(id="i1", project_id="p1", title="Login page"):
    return {
        "id": id,
        "project_id": project_id,
        "parent_id": None,
        "sprint_id": None,
        "item_type": "story",
        "title": title,
        "description": "As a user I can log in",
        "acceptance_criteria": "Login form accepts email+password",
        "status": "open",
        "priority": "high",
        "story_points": 3,
        "labels": ["auth", "ui"],
        "assigned_to": "alice",
        "external_id": None,
        "external_source": None,
        "created_at": datetime(2026, 4, 1),
        "updated_at": datetime(2026, 4, 1),
    }


@pytest.fixture
def db_with_project(db):
    from backend.db.project_store import save_project
    save_project(_project(), db)
    return db


def test_save_and_load_item(db_with_project):
    from backend.db.project_store import save_item, load_item
    save_item(_item(), db_with_project)
    row = load_item("i1", db_with_project)
    assert row is not None
    assert row["title"] == "Login page"
    assert row["labels"] == ["auth", "ui"]
    assert row["story_points"] == 3


def test_load_item_missing(db_with_project):
    from backend.db.project_store import load_item, init_schema
    init_schema(db_with_project)
    assert load_item("nope", db_with_project) is None


def test_item_upsert_updates_fields(db_with_project):
    from backend.db.project_store import save_item, load_item
    save_item(_item(), db_with_project)
    updated = _item()
    updated["status"] = "done"
    updated["story_points"] = 5
    save_item(updated, db_with_project)
    row = load_item("i1", db_with_project)
    assert row["status"] == "done"
    assert row["story_points"] == 5


def test_load_items_filter_by_status(db_with_project):
    from backend.db.project_store import save_item, load_items
    save_item(_item("i1", title="Open item"), db_with_project)
    done = _item("i2", title="Done item")
    done["status"] = "done"
    save_item(done, db_with_project)
    open_items = load_items("p1", status="open", db_path=db_with_project)
    assert len(open_items) == 1
    assert open_items[0]["title"] == "Open item"


def test_load_items_unassigned_sprint(db_with_project):
    from backend.db.project_store import save_item, save_sprint, load_items
    save_sprint(_sprint("s1", "p1", "Sprint 1"), db_with_project)
    save_item(_item("i1"), db_with_project)
    in_sprint = _item("i2", title="In sprint")
    in_sprint["sprint_id"] = "s1"
    save_item(in_sprint, db_with_project)
    backlog = load_items("p1", sprint_id="", db_path=db_with_project)
    assert len(backlog) == 1
    assert backlog[0]["id"] == "i1"


def test_delete_item(db_with_project):
    from backend.db.project_store import save_item, load_item, delete_item
    save_item(_item(), db_with_project)
    delete_item("i1", db_with_project)
    assert load_item("i1", db_with_project) is None


# ---------------------------------------------------------------------------
# Sprint CRUD
# ---------------------------------------------------------------------------

def _sprint(id="s1", project_id="p1", name="Sprint 1"):
    return {
        "id": id,
        "project_id": project_id,
        "name": name,
        "goal": "Ship auth",
        "status": "planned",
        "start_date": datetime(2026, 4, 7),
        "end_date": datetime(2026, 4, 21),
        "capacity_points": 20,
        "completed_points": 0,
        "created_at": datetime(2026, 4, 1),
        "updated_at": datetime(2026, 4, 1),
    }


def test_save_and_load_sprint(db_with_project):
    from backend.db.project_store import save_sprint, load_sprint
    save_sprint(_sprint(), db_with_project)
    row = load_sprint("s1", db_with_project)
    assert row is not None
    assert row["name"] == "Sprint 1"
    assert row["capacity_points"] == 20


def test_sprint_upsert(db_with_project):
    from backend.db.project_store import save_sprint, load_sprint
    save_sprint(_sprint(), db_with_project)
    s = _sprint()
    s["status"] = "active"
    s["completed_points"] = 8
    save_sprint(s, db_with_project)
    row = load_sprint("s1", db_with_project)
    assert row["status"] == "active"
    assert row["completed_points"] == 8


def test_load_sprints_filter_status(db_with_project):
    from backend.db.project_store import save_sprint, load_sprints
    save_sprint(_sprint("s1", name="Sprint 1"), db_with_project)
    s2 = _sprint("s2", name="Sprint 2")
    s2["status"] = "active"
    save_sprint(s2, db_with_project)
    active = load_sprints("p1", status="active", db_path=db_with_project)
    assert len(active) == 1
    assert active[0]["name"] == "Sprint 2"


def test_delete_sprint(db_with_project):
    from backend.db.project_store import save_sprint, load_sprint, delete_sprint
    save_sprint(_sprint(), db_with_project)
    delete_sprint("s1", db_with_project)
    assert load_sprint("s1", db_with_project) is None


def test_sprint_completed_points(db_with_project):
    from backend.db.project_store import save_sprint, save_item, sprint_completed_points
    save_sprint(_sprint(), db_with_project)
    for i, (pts, status) in enumerate([
        (3, "done"), (5, "done"), (2, "in_progress")
    ]):
        item = _item(f"i{i}")
        item["sprint_id"] = "s1"
        item["story_points"] = pts
        item["status"] = status
        save_item(item, db_with_project)
    assert sprint_completed_points("s1", db_with_project) == 8  # only done items
