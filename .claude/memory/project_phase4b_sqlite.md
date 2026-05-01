---
name: Phase 4B SQLite PM Persistence
description: SQLite-backed persistence layer for project management entities (projects, backlog items, sprints)
type: feature
---

# Phase 4B: SQLite Persistence for Projects, Backlog, and Sprints

**Shipped**: April 4, 2026
**Commit**: `60d08bf feat: Phase 4B — SQLite persistence for projects, backlog, and sprints`

## Why

Phase 4 introduced the `ProjectManager` in-memory model. Phase 4B gives it durable storage so project state survives daemon restarts. Uses the same `devtrack.db` SQLite file that Go already manages, keeping infrastructure minimal (no separate DB process).

## Implementation

**File**: `backend/db/project_store.py`

- Raw `sqlite3` module — no ORM (keeps dep surface small)
- Three tables created idempotently via `init_schema(db_path)`:
  - `projects` — id, name, description, status, template_type, dates, budget, risk fields, JSON `metadata` blob, timestamps
  - `backlog_items` — id, project_id, sprint_id, title, description, item_type, status, priority, story_points, assignee, JSON `tags` + `metadata`, timestamps
  - `sprints` — id, project_id, name, goal, status, start/end dates, velocity, JSON `metadata`, timestamps
- All `save_*` functions use `INSERT OR REPLACE` (upsert semantics)
- `load_items()` and `load_sprints()` accept optional keyword filters (status, sprint_id, item_type)
- JSON blobs (tags, metadata) serialised with `json.dumps` / `json.loads`; default empty dicts/lists on read

## Public API

```python
from backend.db.project_store import (
    init_schema,
    save_project, load_project, load_all_projects, delete_project,
    save_item, load_item, load_items, delete_item,
    save_sprint, load_sprint, load_sprints, delete_sprint,
)
```

## How to apply

- Call `init_schema(db_path)` once at startup (idempotent; safe to call repeatedly)
- `db_path` comes from `backend.config.get_path("DATABASE_DIR") / "devtrack.db"` — same file Go uses
- `save_project(project_dict)` — dict must include `id` and `name` at minimum
- All load functions return plain `dict` objects (not dataclasses); `None` on miss for single-item loads

## Relationship to Go database.go

The Go layer (`devtrack-bin/database.go`) owns schema migrations for Go tables. The Python `init_schema` is additive — it only creates `projects`, `backlog_items`, and `sprints` tables, which Go never touches. They co-exist in the same file safely.
