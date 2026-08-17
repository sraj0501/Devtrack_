---
name: Project current state
description: v3.0.10; Phases 0–8 and PostgreSQL epic complete; Phase 9 active; TASK-118 in progress
type: project
---

**Version:** v3.0.10. As of 2026-08-18, `origin/dev` is `47a2d59` after TASK-142 PR #253 and remains ahead of `origin/main`. GitHub `sraj0501/Devtrack_` is the sole source. Release: `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Build arc Phases 0–8: ALL COMPLETE.** Phase history lives in `Data/agent_logs/project_board.md` — don't re-derive it here.

**Post-arc shipped:** TASK-102 (Azure `IsPRApproved`); TASK-103–108 Managed Install; TASK-109 repo cleanup + opt-in telemetry; TASK-110/111 wiki+docs reconciliation; **Silent Worker Correctness epic (TASK-126–130, PR #226)** — fixed ticket_id routing, merge→Done detection, PR-loop nil approval checker, queue bypasses; **TASK-131 quick wins (PR #227)** — `devtrack logs -f` and TUI queue edit; **TASK-132 doc/wiki sync (PR #230)**.

**Complete: EPIC PostgreSQL Backend (TASK-112–116, PRs #231–251)** — PostgreSQL is mandatory for Python server persistence and server-side events. The Go client remains SQLite-only for offline observation, queueing, MCP, and replay. Fourteen of fifteen raw-`sqlite3` modules were ported; `alert_poller.py` was dead code removed under TASK-133. TASK-114 added opt-in, idempotent client-event sync; TASK-115 added Alembic and one-shot import; TASK-116 enforced and documented required PostgreSQL deployment.

**Recent merges:** TASK-141 PR #247; TASK-114 PR #249; TASK-115 PR #250; TASK-116 PR #251; TASK-117 PR #252; TASK-142 PR #253. TASK-142 replaced the legacy NLP/spaCy surface with configured-LLM enrichment and a non-blocking raw/template fallback.

**Task-ID ledger:** `Data/agent_logs/project_board.md` is authoritative. TASK-118 is active on `features/TASK-118-background-bootstrap`; next unused ID: **TASK-143**.

**Active next:** Phase 9 — Adoption Gate — TASK-117–124 (`docs/NEXT_STEPS.md`). TASK-117 and prerequisite TASK-142 are complete. TASK-118 is implementing detached managed-server checkout/`uv sync`/local Ollama pull with durable `status`/`doctor` degradation reporting; TASK-119 follows.

**DEPRIORITISED:** PG-5 (**a stats pipeline — nothing to do with the Postgres epic**), Redis R-1→R-6, CLI aesthetics, savings counter, videos, boardroom/plan as primary. **Never:** NATS/Redis/external queues/brokers, Kubernetes, multi-tenancy, DDD layers. The two docs that once proposed these (`implementation-plan.md`, `devtrack-architecture.html`) were deleted in TASK-109 — don't recreate them.

**PostgreSQL is mandatory server-side** (user, 2026-08-10; implementation complete 2026-08-18) — it replaces SQLite for server persistence and server-side events. `POSTGRES_URL` is required and startup validates connectivity and applies Alembic migrations. Offline-first Rule 0 is preserved at the Go client boundary: local SQLite remains the client's source of truth and the Go client never speaks Postgres directly.
