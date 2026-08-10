---
name: Project current state
description: v3.0.10; Phases 0–8 complete; mandatory server PostgreSQL migration has no raw-sqlite modules left; Phase 9 next
type: project
---

**Version:** v3.0.10. As of 2026-08-10, `origin/main` is `40d6fe9` and green; `origin/dev` contains the active PostgreSQL migration work and remains ahead of main. GitHub `sraj0501/Devtrack_` is the sole source. Release: `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Build arc Phases 0–8: ALL COMPLETE.** Phase history lives in `Data/agent_logs/project_board.md` — don't re-derive it here.

**Post-arc shipped:** TASK-102 (Azure `IsPRApproved`); TASK-103–108 Managed Install; TASK-109 repo cleanup + opt-in telemetry; TASK-110/111 wiki+docs reconciliation; **Silent Worker Correctness epic (TASK-126–130, PR #226)** — fixed ticket_id routing, merge→Done detection, PR-loop nil approval checker, queue bypasses; **TASK-131 quick wins (PR #227)** — `devtrack logs -f` and TUI queue edit; **TASK-132 doc/wiki sync (PR #230)**.

**In progress: EPIC PostgreSQL Backend (TASK-112–116, started 2026-07-31, PR #231)** — PostgreSQL is mandatory for Python server persistence and server-side events (user decision, 2026-08-10). The Go client remains SQLite-only for offline observation, queueing, MCP, and replay. TASK-141 completes the original module inventory locally: 14 of 15 raw-`sqlite3` modules are ported and `alert_poller.py` was dead code removed under TASK-133, leaving no production raw-`sqlite3` imports. TASK-114 client→server event sync, TASK-115 migrations/import, and TASK-116 required-Postgres deployment are unstarted.

**TASK-140:** merged to `dev` at `f7a95a6`; it moved `psycopg2-binary` into core dependencies and restored green standard/live-PostgreSQL CI. **TASK-141:** complete locally and uncommitted; `/voice/status` now uses the shared SQLAlchemy stores, with 900/4-skipped full-suite and 4/4 live-PostgreSQL verification.

**Task-ID ledger:** `Data/agent_logs/project_board.md` is authoritative. TASK-140 is on `dev`; TASK-141 is active locally; next unused ID: **TASK-142**.

**Queued next:** Phase 9 — Adoption Gate — TASK-117–124 (`docs/NEXT_STEPS.md`), after the Postgres epic.

**DEPRIORITISED:** PG-5 (**a stats pipeline — nothing to do with the Postgres epic**), Redis R-1→R-6, CLI aesthetics, savings counter, videos, boardroom/plan as primary. **Never:** NATS/Redis/external queues/brokers, Kubernetes, multi-tenancy, DDD layers. The two docs that once proposed these (`implementation-plan.md`, `devtrack-architecture.html`) were deleted in TASK-109 — don't recreate them.

**PostgreSQL is mandatory server-side** (user, 2026-08-10) — it replaces SQLite for server persistence and server-side events. `POSTGRES_URL` becomes required as TASK-114–116 land. Offline-first Rule 0 is preserved at the Go client boundary: local SQLite remains the client's source of truth and the Go client never speaks Postgres directly.
