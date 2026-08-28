---
name: PostgreSQL backend completion
description: Required server storage, opt-in client sync, Alembic, and offline boundary
type: project
---

**Why:** mixed server SQLite/PostgreSQL reads made aggregate state unreliable; TASK-112–116 plus TASK-140/141 completed the boundary.
**Boundary:** Go uses local SQLite for observation, queue, MCP, and replay and never imports a PostgreSQL driver. Python requires `POSTGRES_URL`; no supported server SQLite fallback.
**Lifecycle:** server startup validates PostgreSQL and advances Alembic. Existing history imports once through `uv run python -m backend.db.sqlite_import`.
**Sync:** opted-in client events cross HTTP/JSON with stable IDs/revisions for idempotent replay and attribution.
**Optional stores:** MongoDB may back configured learning/spec paths; ChromaDB is optional RAG. Neither replaces PostgreSQL.
**Apply:** schema changes use Alembic; database tests are isolated; sync stays opt-in and retry-safe.
