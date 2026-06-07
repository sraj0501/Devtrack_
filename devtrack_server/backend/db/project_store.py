"""
SQLAlchemy persistence for Projects, BacklogItems, and Sprints.

All three tables live in the main devtrack.db (SQLite) or PostgreSQL,
selected via POSTGRES_URL.  Uses SQLAlchemy Core — no ORM.

Public API
----------
# Schema
init_schema()                  — create tables (idempotent, called on first use)

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
from datetime import datetime
from typing import Any, Dict, List, Optional

from sqlalchemy import Column, Float, Integer, Index, Table, Text, and_, func, select
from sqlalchemy.engine import Engine

from backend.db.engine import get_engine, metadata, upsert

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Table definitions (registered with shared metadata)
# ---------------------------------------------------------------------------

projects_table = Table(
    "projects", metadata,
    Column("id",                       Text, primary_key=True),
    Column("name",                     Text, nullable=False),
    Column("description",              Text),
    Column("status",                   Text, nullable=False),
    Column("template_type",            Text),
    Column("start_date",               Text),
    Column("end_date",                 Text),
    Column("budget_estimate",          Float),
    Column("risk_level",               Text),
    Column("risk_description",         Text),
    Column("external_id",              Text),
    Column("external_source",          Text),
    Column("external_sync_at",         Text),
    Column("goals_json",               Text),
    Column("stakeholders_json",        Text),
    Column("related_project_ids_json", Text),
    Column("metadata_json",            Text),
    Column("created_at",               Text, nullable=False),
    Column("updated_at",               Text, nullable=False),
    Index("idx_projects_status",     "status"),
    Index("idx_projects_updated_at", "updated_at"),
)

sprints_table = Table(
    "sprints", metadata,
    Column("id",               Text, primary_key=True),
    Column("project_id",       Text, nullable=False),
    Column("name",             Text, nullable=False),
    Column("goal",             Text),
    Column("status",           Text, nullable=False),
    Column("start_date",       Text),
    Column("end_date",         Text),
    Column("capacity_points",  Integer),
    Column("completed_points", Integer, nullable=False),
    Column("created_at",       Text, nullable=False),
    Column("updated_at",       Text, nullable=False),
    Index("idx_sprints_project_id", "project_id"),
    Index("idx_sprints_status",     "status"),
)

backlog_items_table = Table(
    "backlog_items", metadata,
    Column("id",                  Text, primary_key=True),
    Column("project_id",          Text, nullable=False),
    Column("parent_id",           Text),
    Column("sprint_id",           Text),
    Column("item_type",           Text, nullable=False),
    Column("title",               Text, nullable=False),
    Column("description",         Text),
    Column("acceptance_criteria", Text),
    Column("status",              Text, nullable=False),
    Column("priority",            Text, nullable=False),
    Column("story_points",        Integer),
    Column("labels_json",         Text),
    Column("assigned_to",         Text),
    Column("external_id",         Text),
    Column("external_source",     Text),
    Column("created_at",          Text, nullable=False),
    Column("updated_at",          Text, nullable=False),
    Index("idx_backlog_project_id", "project_id"),
    Index("idx_backlog_sprint_id",  "sprint_id"),
    Index("idx_backlog_status",     "status"),
    Index("idx_backlog_item_type",  "item_type"),
)

# ---------------------------------------------------------------------------
# Schema init
# ---------------------------------------------------------------------------

_schema_done: bool = False


def init_schema(engine: Optional[Engine] = None) -> None:
    """Create tables if they don't exist (idempotent)."""
    global _schema_done
    if engine is None:
        if _schema_done:
            return
        eng = get_engine()
        metadata.create_all(eng, tables=[projects_table, sprints_table, backlog_items_table])
        _schema_done = True
    else:
        metadata.create_all(engine, tables=[projects_table, sprints_table, backlog_items_table])
    logger.debug("project_store: schema ready")


def _eng(engine: Optional[Engine] = None) -> Engine:
    init_schema(engine)
    return engine or get_engine()


# ---------------------------------------------------------------------------
# Project CRUD
# ---------------------------------------------------------------------------

def save_project(project: Dict[str, Any], engine: Optional[Engine] = None) -> None:
    """Insert or replace a project row from a dict (ProjectManager.to_dict())."""
    now = datetime.utcnow().isoformat()
    row = _project_to_row(project, now)
    update_cols = {k: v for k, v in row.items() if k not in ("id", "created_at")}
    stmt = (
        upsert(projects_table)
        .values(**row)
        .on_conflict_do_update(index_elements=["id"], set_=update_cols)
    )
    with _eng(engine).begin() as conn:
        conn.execute(stmt)


def load_project(project_id: str, engine: Optional[Engine] = None) -> Optional[Dict[str, Any]]:
    with _eng(engine).connect() as conn:
        row = conn.execute(
            select(projects_table).where(projects_table.c.id == project_id)
        ).mappings().first()
    return _row_to_project(row) if row else None


def load_all_projects(engine: Optional[Engine] = None) -> List[Dict[str, Any]]:
    with _eng(engine).connect() as conn:
        rows = conn.execute(
            select(projects_table).order_by(projects_table.c.updated_at.desc())
        ).mappings().all()
    return [_row_to_project(r) for r in rows]


def delete_project(project_id: str, engine: Optional[Engine] = None) -> None:
    with _eng(engine).begin() as conn:
        conn.execute(projects_table.delete().where(projects_table.c.id == project_id))


# ---------------------------------------------------------------------------
# BacklogItem CRUD
# ---------------------------------------------------------------------------

def save_item(item: Dict[str, Any], engine: Optional[Engine] = None) -> None:
    now = datetime.utcnow().isoformat()
    row = _item_to_row(item, now)
    update_cols = {k: v for k, v in row.items() if k not in ("id", "created_at")}
    stmt = (
        upsert(backlog_items_table)
        .values(**row)
        .on_conflict_do_update(index_elements=["id"], set_=update_cols)
    )
    with _eng(engine).begin() as conn:
        conn.execute(stmt)


def load_item(item_id: str, engine: Optional[Engine] = None) -> Optional[Dict[str, Any]]:
    with _eng(engine).connect() as conn:
        row = conn.execute(
            select(backlog_items_table).where(backlog_items_table.c.id == item_id)
        ).mappings().first()
    return _row_to_item(row) if row else None


def load_items(
    project_id: str,
    *,
    status: Optional[str] = None,
    sprint_id: Optional[str] = None,
    item_type: Optional[str] = None,
    engine: Optional[Engine] = None,
) -> List[Dict[str, Any]]:
    t = backlog_items_table
    conditions = [t.c.project_id == project_id]
    if status:
        conditions.append(t.c.status == status)
    if sprint_id is not None:
        if sprint_id == "":
            conditions.append(t.c.sprint_id.is_(None))
        else:
            conditions.append(t.c.sprint_id == sprint_id)
    if item_type:
        conditions.append(t.c.item_type == item_type)

    stmt = (
        select(t)
        .where(and_(*conditions))
        .order_by(t.c.priority.desc(), t.c.created_at.asc())
    )
    with _eng(engine).connect() as conn:
        rows = conn.execute(stmt).mappings().all()
    return [_row_to_item(r) for r in rows]


def delete_item(item_id: str, engine: Optional[Engine] = None) -> None:
    with _eng(engine).begin() as conn:
        conn.execute(
            backlog_items_table.delete().where(backlog_items_table.c.id == item_id)
        )


# ---------------------------------------------------------------------------
# Sprint CRUD
# ---------------------------------------------------------------------------

def save_sprint(sprint: Dict[str, Any], engine: Optional[Engine] = None) -> None:
    now = datetime.utcnow().isoformat()
    row = _sprint_to_row(sprint, now)
    update_cols = {k: v for k, v in row.items() if k not in ("id", "created_at")}
    stmt = (
        upsert(sprints_table)
        .values(**row)
        .on_conflict_do_update(index_elements=["id"], set_=update_cols)
    )
    with _eng(engine).begin() as conn:
        conn.execute(stmt)


def load_sprint(sprint_id: str, engine: Optional[Engine] = None) -> Optional[Dict[str, Any]]:
    with _eng(engine).connect() as conn:
        row = conn.execute(
            select(sprints_table).where(sprints_table.c.id == sprint_id)
        ).mappings().first()
    return dict(row) if row else None


def load_sprints(
    project_id: str,
    *,
    status: Optional[str] = None,
    engine: Optional[Engine] = None,
) -> List[Dict[str, Any]]:
    t = sprints_table
    conditions = [t.c.project_id == project_id]
    if status:
        conditions.append(t.c.status == status)
    stmt = (
        select(t)
        .where(and_(*conditions))
        .order_by(t.c.start_date.asc(), t.c.created_at.asc())
    )
    with _eng(engine).connect() as conn:
        rows = conn.execute(stmt).mappings().all()
    return [dict(r) for r in rows]


def delete_sprint(sprint_id: str, engine: Optional[Engine] = None) -> None:
    with _eng(engine).begin() as conn:
        conn.execute(sprints_table.delete().where(sprints_table.c.id == sprint_id))


def sprint_completed_points(sprint_id: str, engine: Optional[Engine] = None) -> int:
    """Sum story_points of done items in a sprint."""
    t = backlog_items_table
    stmt = select(
        func.coalesce(func.sum(t.c.story_points), 0)
    ).where(and_(t.c.sprint_id == sprint_id, t.c.status == "done"))
    with _eng(engine).connect() as conn:
        result = conn.execute(stmt).scalar()
    return int(result or 0)


# ---------------------------------------------------------------------------
# Row ↔ dict converters
# ---------------------------------------------------------------------------

def _iso(v: Any) -> Optional[str]:
    if v is None:
        return None
    return v.isoformat() if hasattr(v, "isoformat") else str(v)


def _project_to_row(p: Dict[str, Any], now: str) -> Dict[str, Any]:
    return {
        "id":                       p["id"],
        "name":                     p["name"],
        "description":              p.get("description", ""),
        "status":                   p.get("status", "setup"),
        "template_type":            p.get("template_type"),
        "start_date":               _iso(p.get("start_date")),
        "end_date":                 _iso(p.get("end_date")),
        "budget_estimate":          p.get("budget_estimate"),
        "risk_level":               p.get("risk_level", "low"),
        "risk_description":         p.get("risk_description", ""),
        "external_id":              p.get("external_id"),
        "external_source":          p.get("external_source"),
        "external_sync_at":         _iso(p.get("external_sync_at")),
        "goals_json":               json.dumps(p.get("goals", [])),
        "stakeholders_json":        json.dumps(p.get("stakeholders", [])),
        "related_project_ids_json": json.dumps(p.get("related_project_ids", [])),
        "metadata_json":            json.dumps(p.get("metadata", {})),
        "created_at":               _iso(p.get("created_at")) or now,
        "updated_at":               now,
    }


def _row_to_project(row: Any) -> Dict[str, Any]:
    d = dict(row)
    for key in ("goals", "stakeholders", "related_project_ids", "metadata"):
        json_key = f"{key}_json"
        d[key] = json.loads(d.pop(json_key, None) or "[]")
    return d


def _item_to_row(item: Dict[str, Any], now: str) -> Dict[str, Any]:
    return {
        "id":                  item["id"],
        "project_id":          item["project_id"],
        "parent_id":           item.get("parent_id"),
        "sprint_id":           item.get("sprint_id"),
        "item_type":           item.get("item_type", "story"),
        "title":               item["title"],
        "description":         item.get("description", ""),
        "acceptance_criteria": item.get("acceptance_criteria", ""),
        "status":              item.get("status", "open"),
        "priority":            item.get("priority", "medium"),
        "story_points":        item.get("story_points"),
        "labels_json":         json.dumps(item.get("labels", [])),
        "assigned_to":         item.get("assigned_to", ""),
        "external_id":         item.get("external_id"),
        "external_source":     item.get("external_source"),
        "created_at":          _iso(item.get("created_at")) or now,
        "updated_at":          now,
    }


def _row_to_item(row: Any) -> Dict[str, Any]:
    d = dict(row)
    d["labels"] = json.loads(d.pop("labels_json", None) or "[]")
    return d


def _sprint_to_row(sprint: Dict[str, Any], now: str) -> Dict[str, Any]:
    return {
        "id":               sprint["id"],
        "project_id":       sprint["project_id"],
        "name":             sprint["name"],
        "goal":             sprint.get("goal", ""),
        "status":           sprint.get("status", "planned"),
        "start_date":       _iso(sprint.get("start_date")),
        "end_date":         _iso(sprint.get("end_date")),
        "capacity_points":  sprint.get("capacity_points"),
        "completed_points": sprint.get("completed_points", 0),
        "created_at":       _iso(sprint.get("created_at")) or now,
        "updated_at":       now,
    }
