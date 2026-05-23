"""
SQLite persistence for Projects, BacklogItems, and Sprints.

All three tables live in the shared devtrack.db file alongside the Go tables,
platform_sync_cache, and learning tables.  Uses raw sqlite3 — no ORM.

Public API
----------
# Schema
init_schema(db_path)           — create tables (idempotent, called on import)

# Projects
save_project(project)          — insert or replace
load_project(id) -> dict|None
load_all_projects() -> list[dict]
delete_project(id)

# Backlog items
save_item(item)                — insert or replace
load_item(id) -> dict|None
load_items(project_id, *, status, sprint_id, item_type) -> list[dict]
delete_item(id)

# Sprints
save_sprint(sprint)            — insert or replace
load_sprint(id) -> dict|None
load_sprints(project_id, *, status) -> list[dict]
delete_sprint(id)
"""

from __future__ import annotations

import json
import logging
import sqlite3
from contextlib import contextmanager
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, Generator, List, Optional

import backend.config as config

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Schema DDL
# ---------------------------------------------------------------------------

_DDL = """
CREATE TABLE IF NOT EXISTS projects (
    id                      TEXT PRIMARY KEY,
    name                    TEXT NOT NULL,
    description             TEXT DEFAULT '',
    status                  TEXT NOT NULL DEFAULT 'setup',
    template_type           TEXT,
    start_date              TEXT,
    end_date                TEXT,
    budget_estimate         REAL,
    risk_level              TEXT DEFAULT 'low',
    risk_description        TEXT DEFAULT '',
    external_id             TEXT,
    external_source         TEXT,
    external_sync_at        TEXT,
    goals_json              TEXT DEFAULT '[]',
    stakeholders_json       TEXT DEFAULT '[]',
    related_project_ids_json TEXT DEFAULT '[]',
    metadata_json           TEXT DEFAULT '{}',
    created_at              TEXT NOT NULL,
    updated_at              TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_projects_status     ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_updated_at ON projects(updated_at);

CREATE TABLE IF NOT EXISTS sprints (
    id               TEXT PRIMARY KEY,
    project_id       TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    goal             TEXT DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'planned',
    start_date       TEXT,
    end_date         TEXT,
    capacity_points  INTEGER,
    completed_points INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sprints_project_id ON sprints(project_id);
CREATE INDEX IF NOT EXISTS idx_sprints_status     ON sprints(status);

CREATE TABLE IF NOT EXISTS backlog_items (
    id                  TEXT PRIMARY KEY,
    project_id          TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_id           TEXT REFERENCES backlog_items(id) ON DELETE SET NULL,
    sprint_id           TEXT REFERENCES sprints(id) ON DELETE SET NULL,
    item_type           TEXT NOT NULL DEFAULT 'story',
    title               TEXT NOT NULL,
    description         TEXT DEFAULT '',
    acceptance_criteria TEXT DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'open',
    priority            TEXT NOT NULL DEFAULT 'medium',
    story_points        INTEGER,
    labels_json         TEXT DEFAULT '[]',
    assigned_to         TEXT DEFAULT '',
    external_id         TEXT,
    external_source     TEXT,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_backlog_project_id ON backlog_items(project_id);
CREATE INDEX IF NOT EXISTS idx_backlog_sprint_id  ON backlog_items(sprint_id);
CREATE INDEX IF NOT EXISTS idx_backlog_status     ON backlog_items(status);
CREATE INDEX IF NOT EXISTS idx_backlog_item_type  ON backlog_items(item_type);
"""

# ---------------------------------------------------------------------------
# Connection helpers
# ---------------------------------------------------------------------------

def _db_path() -> str:
    return str(config.database_path())


@contextmanager
def _conn(db_path: Optional[str] = None) -> Generator[sqlite3.Connection, None, None]:
    path = db_path or _db_path()
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    con = sqlite3.connect(path, check_same_thread=False)
    con.row_factory = sqlite3.Row
    con.execute("PRAGMA journal_mode=WAL")
    con.execute("PRAGMA foreign_keys=ON")
    try:
        yield con
        con.commit()
    except Exception:
        con.rollback()
        raise
    finally:
        con.close()


# ---------------------------------------------------------------------------
# Schema initialisation
# ---------------------------------------------------------------------------

_schema_initialised: set[str] = set()


def init_schema(db_path: Optional[str] = None) -> None:
    """Create all tables and indexes (idempotent)."""
    path = db_path or _db_path()
    if path in _schema_initialised:
        return
    with _conn(path) as con:
        con.executescript(_DDL)
    _schema_initialised.add(path)
    logger.debug("project_store: schema initialised at %s", path)


# ---------------------------------------------------------------------------
# Project CRUD
# ---------------------------------------------------------------------------

def save_project(project: Dict[str, Any], db_path: Optional[str] = None) -> None:
    """Insert or replace a project row from a dict (ProjectManager.to_dict())."""
    init_schema(db_path)
    now = datetime.utcnow().isoformat()
    with _conn(db_path) as con:
        con.execute(
            """
            INSERT INTO projects (
                id, name, description, status, template_type,
                start_date, end_date, budget_estimate,
                risk_level, risk_description,
                external_id, external_source, external_sync_at,
                goals_json, stakeholders_json, related_project_ids_json,
                metadata_json, created_at, updated_at
            ) VALUES (
                :id, :name, :description, :status, :template_type,
                :start_date, :end_date, :budget_estimate,
                :risk_level, :risk_description,
                :external_id, :external_source, :external_sync_at,
                :goals_json, :stakeholders_json, :related_project_ids_json,
                :metadata_json, :created_at, :updated_at
            )
            ON CONFLICT(id) DO UPDATE SET
                name                     = excluded.name,
                description              = excluded.description,
                status                   = excluded.status,
                template_type            = excluded.template_type,
                start_date               = excluded.start_date,
                end_date                 = excluded.end_date,
                budget_estimate          = excluded.budget_estimate,
                risk_level               = excluded.risk_level,
                risk_description         = excluded.risk_description,
                external_id              = excluded.external_id,
                external_source          = excluded.external_source,
                external_sync_at         = excluded.external_sync_at,
                goals_json               = excluded.goals_json,
                stakeholders_json        = excluded.stakeholders_json,
                related_project_ids_json = excluded.related_project_ids_json,
                metadata_json            = excluded.metadata_json,
                updated_at               = excluded.updated_at
            """,
            _project_to_row(project, now),
        )


def load_project(project_id: str, db_path: Optional[str] = None) -> Optional[Dict[str, Any]]:
    init_schema(db_path)
    with _conn(db_path) as con:
        row = con.execute("SELECT * FROM projects WHERE id = ?", (project_id,)).fetchone()
    return _row_to_project(row) if row else None


def load_all_projects(db_path: Optional[str] = None) -> List[Dict[str, Any]]:
    init_schema(db_path)
    with _conn(db_path) as con:
        rows = con.execute("SELECT * FROM projects ORDER BY updated_at DESC").fetchall()
    return [_row_to_project(r) for r in rows]


def delete_project(project_id: str, db_path: Optional[str] = None) -> None:
    init_schema(db_path)
    with _conn(db_path) as con:
        con.execute("DELETE FROM projects WHERE id = ?", (project_id,))


# ---------------------------------------------------------------------------
# BacklogItem CRUD
# ---------------------------------------------------------------------------

def save_item(item: Dict[str, Any], db_path: Optional[str] = None) -> None:
    """Insert or replace a backlog item row."""
    init_schema(db_path)
    now = datetime.utcnow().isoformat()
    with _conn(db_path) as con:
        con.execute(
            """
            INSERT INTO backlog_items (
                id, project_id, parent_id, sprint_id, item_type,
                title, description, acceptance_criteria,
                status, priority, story_points, labels_json,
                assigned_to, external_id, external_source,
                created_at, updated_at
            ) VALUES (
                :id, :project_id, :parent_id, :sprint_id, :item_type,
                :title, :description, :acceptance_criteria,
                :status, :priority, :story_points, :labels_json,
                :assigned_to, :external_id, :external_source,
                :created_at, :updated_at
            )
            ON CONFLICT(id) DO UPDATE SET
                parent_id           = excluded.parent_id,
                sprint_id           = excluded.sprint_id,
                item_type           = excluded.item_type,
                title               = excluded.title,
                description         = excluded.description,
                acceptance_criteria = excluded.acceptance_criteria,
                status              = excluded.status,
                priority            = excluded.priority,
                story_points        = excluded.story_points,
                labels_json         = excluded.labels_json,
                assigned_to         = excluded.assigned_to,
                external_id         = excluded.external_id,
                external_source     = excluded.external_source,
                updated_at          = excluded.updated_at
            """,
            _item_to_row(item, now),
        )


def load_item(item_id: str, db_path: Optional[str] = None) -> Optional[Dict[str, Any]]:
    init_schema(db_path)
    with _conn(db_path) as con:
        row = con.execute("SELECT * FROM backlog_items WHERE id = ?", (item_id,)).fetchone()
    return _row_to_item(row) if row else None


def load_items(
    project_id: str,
    *,
    status: Optional[str] = None,
    sprint_id: Optional[str] = None,
    item_type: Optional[str] = None,
    db_path: Optional[str] = None,
) -> List[Dict[str, Any]]:
    init_schema(db_path)
    clauses = ["project_id = ?"]
    params: list = [project_id]
    if status:
        clauses.append("status = ?")
        params.append(status)
    if sprint_id is not None:
        if sprint_id == "":
            clauses.append("sprint_id IS NULL")
        else:
            clauses.append("sprint_id = ?")
            params.append(sprint_id)
    if item_type:
        clauses.append("item_type = ?")
        params.append(item_type)
    sql = f"SELECT * FROM backlog_items WHERE {' AND '.join(clauses)} ORDER BY priority DESC, created_at ASC"
    with _conn(db_path) as con:
        rows = con.execute(sql, params).fetchall()
    return [_row_to_item(r) for r in rows]


def delete_item(item_id: str, db_path: Optional[str] = None) -> None:
    init_schema(db_path)
    with _conn(db_path) as con:
        con.execute("DELETE FROM backlog_items WHERE id = ?", (item_id,))


# ---------------------------------------------------------------------------
# Sprint CRUD
# ---------------------------------------------------------------------------

def save_sprint(sprint: Dict[str, Any], db_path: Optional[str] = None) -> None:
    init_schema(db_path)
    now = datetime.utcnow().isoformat()
    with _conn(db_path) as con:
        con.execute(
            """
            INSERT INTO sprints (
                id, project_id, name, goal, status,
                start_date, end_date, capacity_points, completed_points,
                created_at, updated_at
            ) VALUES (
                :id, :project_id, :name, :goal, :status,
                :start_date, :end_date, :capacity_points, :completed_points,
                :created_at, :updated_at
            )
            ON CONFLICT(id) DO UPDATE SET
                name             = excluded.name,
                goal             = excluded.goal,
                status           = excluded.status,
                start_date       = excluded.start_date,
                end_date         = excluded.end_date,
                capacity_points  = excluded.capacity_points,
                completed_points = excluded.completed_points,
                updated_at       = excluded.updated_at
            """,
            _sprint_to_row(sprint, now),
        )


def load_sprint(sprint_id: str, db_path: Optional[str] = None) -> Optional[Dict[str, Any]]:
    init_schema(db_path)
    with _conn(db_path) as con:
        row = con.execute("SELECT * FROM sprints WHERE id = ?", (sprint_id,)).fetchone()
    return _row_to_sprint(row) if row else None


def load_sprints(
    project_id: str,
    *,
    status: Optional[str] = None,
    db_path: Optional[str] = None,
) -> List[Dict[str, Any]]:
    init_schema(db_path)
    clauses = ["project_id = ?"]
    params: list = [project_id]
    if status:
        clauses.append("status = ?")
        params.append(status)
    sql = f"SELECT * FROM sprints WHERE {' AND '.join(clauses)} ORDER BY start_date ASC, created_at ASC"
    with _conn(db_path) as con:
        rows = con.execute(sql, params).fetchall()
    return [_row_to_sprint(r) for r in rows]


def delete_sprint(sprint_id: str, db_path: Optional[str] = None) -> None:
    init_schema(db_path)
    with _conn(db_path) as con:
        con.execute("DELETE FROM sprints WHERE id = ?", (sprint_id,))


def sprint_completed_points(sprint_id: str, db_path: Optional[str] = None) -> int:
    """Sum story_points of done items in a sprint."""
    init_schema(db_path)
    with _conn(db_path) as con:
        row = con.execute(
            "SELECT COALESCE(SUM(story_points), 0) FROM backlog_items WHERE sprint_id = ? AND status = 'done'",
            (sprint_id,),
        ).fetchone()
    return int(row[0]) if row else 0


# ---------------------------------------------------------------------------
# Row ↔ dict converters
# ---------------------------------------------------------------------------

def _project_to_row(p: Dict[str, Any], now: str) -> Dict[str, Any]:
    def _iso(v):
        if v is None:
            return None
        return v.isoformat() if hasattr(v, "isoformat") else str(v)

    return {
        "id": p["id"],
        "name": p["name"],
        "description": p.get("description", ""),
        "status": p.get("status", "setup"),
        "template_type": p.get("template_type"),
        "start_date": _iso(p.get("start_date")),
        "end_date": _iso(p.get("end_date")),
        "budget_estimate": p.get("budget_estimate"),
        "risk_level": p.get("risk_level", "low"),
        "risk_description": p.get("risk_description", ""),
        "external_id": p.get("external_id"),
        "external_source": p.get("external_source"),
        "external_sync_at": _iso(p.get("external_sync_at")),
        "goals_json": json.dumps(p.get("goals", [])),
        "stakeholders_json": json.dumps(p.get("stakeholders", [])),
        "related_project_ids_json": json.dumps(p.get("related_project_ids", [])),
        "metadata_json": json.dumps(p.get("metadata", {})),
        "created_at": _iso(p.get("created_at")) or now,
        "updated_at": now,
    }


def _row_to_project(row: sqlite3.Row) -> Dict[str, Any]:
    d = dict(row)
    for key in ("goals", "stakeholders", "related_project_ids", "metadata"):
        json_key = f"{key}_json"
        d[key] = json.loads(d.pop(json_key, "[]") or "[]")
    return d


def _item_to_row(item: Dict[str, Any], now: str) -> Dict[str, Any]:
    def _iso(v):
        if v is None:
            return None
        return v.isoformat() if hasattr(v, "isoformat") else str(v)

    return {
        "id": item["id"],
        "project_id": item["project_id"],
        "parent_id": item.get("parent_id"),
        "sprint_id": item.get("sprint_id"),
        "item_type": item.get("item_type", "story"),
        "title": item["title"],
        "description": item.get("description", ""),
        "acceptance_criteria": item.get("acceptance_criteria", ""),
        "status": item.get("status", "open"),
        "priority": item.get("priority", "medium"),
        "story_points": item.get("story_points"),
        "labels_json": json.dumps(item.get("labels", [])),
        "assigned_to": item.get("assigned_to", ""),
        "external_id": item.get("external_id"),
        "external_source": item.get("external_source"),
        "created_at": _iso(item.get("created_at")) or now,
        "updated_at": now,
    }


def _row_to_item(row: sqlite3.Row) -> Dict[str, Any]:
    d = dict(row)
    d["labels"] = json.loads(d.pop("labels_json", "[]") or "[]")
    return d


def _sprint_to_row(sprint: Dict[str, Any], now: str) -> Dict[str, Any]:
    def _iso(v):
        if v is None:
            return None
        return v.isoformat() if hasattr(v, "isoformat") else str(v)

    return {
        "id": sprint["id"],
        "project_id": sprint["project_id"],
        "name": sprint["name"],
        "goal": sprint.get("goal", ""),
        "status": sprint.get("status", "planned"),
        "start_date": _iso(sprint.get("start_date")),
        "end_date": _iso(sprint.get("end_date")),
        "capacity_points": sprint.get("capacity_points"),
        "completed_points": sprint.get("completed_points", 0),
        "created_at": _iso(sprint.get("created_at")) or now,
        "updated_at": now,
    }


def _row_to_sprint(row: sqlite3.Row) -> Dict[str, Any]:
    return dict(row)
