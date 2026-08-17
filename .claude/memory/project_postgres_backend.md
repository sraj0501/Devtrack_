---
name: PostgreSQL backend completion
description: Durable server storage boundary completed through TASK-116, with opt-in client sync, Alembic migrations, and required deployment validation
type: project
---

# PostgreSQL Backend

## Why

The Python server cannot aggregate commercial/server-side state reliably when some modules use
PostgreSQL while others silently read a local SQLite file. The completed epic makes the ownership
boundary explicit without weakening offline-first behavior.

## Current architecture

- The Go client always uses local SQLite for observation, queueing, MCP reads, and offline replay.
- The Go client never connects directly to PostgreSQL and must not gain a PostgreSQL driver.
- The Python server requires `POSTGRES_URL`; startup validates connectivity and advances Alembic to
  head before serving requests. There is no supported server-side SQLite fallback.
- Opted-in client events cross the existing HTTP/JSON boundary. Stable client/event identifiers make
  retries idempotent and retain developer attribution.
- Existing history can be imported once through `python -m backend.db.sqlite_import`.
- MongoDB remains optional and limited to its documented Teams voice-learning source role.

## Delivery evidence

- TASK-141 / PR #247: final raw-SQLite production read moved to shared SQLAlchemy stores.
- TASK-114 / PR #249: opt-in client-to-server event sync and offline backlog replay.
- TASK-115 / PR #250: Alembic schema lifecycle and one-shot SQLite import.
- TASK-116 / PR #251: deployment enforcement, Compose wiring, startup validation, and installation docs.
- TASK-117 / PR #252: managed setup writes the required connection surface and visible defaults.
- `origin/dev` reached `197c079`; the final deployment and setup changes passed all seven CI checks.

## How to apply

When changing storage, preserve the client/server boundary: client-owned SQLite remains available
without a server; server-owned persistence uses the shared SQLAlchemy/PostgreSQL path. Add schema
changes through Alembic, isolate database tests, and keep synchronization opt-in and idempotent.
