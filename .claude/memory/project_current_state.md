---
name: Project current state
description: v3.0.10; Phases 0–8 + Managed Install + Silent Worker epic COMPLETE; Postgres epic (TASK-112–116) in progress; Phase 9 next
type: project
---

**Version:** v3.0.10. Branch `dev`, in sync with `main` as of 2026-07-31. GitHub `sraj0501/Devtrack_` sole source. Release: `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Build arc Phases 0–8: ALL COMPLETE.** Phase history lives in `Data/agent_logs/project_board.md` — don't re-derive it here.

**Post-arc shipped:** TASK-102 (Azure `IsPRApproved`); TASK-103–108 Managed Install; TASK-109 repo cleanup + opt-in telemetry; TASK-110/111 wiki+docs reconciliation; **Silent Worker Correctness epic (TASK-126–130, PR #226)** — fixed ticket_id routing, merge→Done detection, PR-loop nil approval checker, queue bypasses; **TASK-131 quick wins (PR #227)** — `devtrack logs -f` and TUI queue edit; **TASK-132 doc/wiki sync (PR #230)**.

**In progress: EPIC PostgreSQL Backend (TASK-112–116, started 2026-07-31, PR #231)** — 1 of 15 raw-`sqlite3` modules ported + a Postgres test lane foundation. See `postgres_epic.md` in user memory for full detail.

**Task-ID ledger:** `Data/agent_logs/project_board.md` is authoritative. Next ID: **TASK-133**.

**Queued next:** Phase 9 — Adoption Gate — TASK-117–124 (`docs/NEXT_STEPS.md`), after the Postgres epic.

**Open PRs: none** as of 2026-07-31. #216–#231 all merged.

**DEPRIORITISED:** PG-5 (**a stats pipeline — nothing to do with the Postgres epic**), Redis R-1→R-6, CLI aesthetics, savings counter, videos, boardroom/plan as primary. **Never:** NATS/Redis/external queues/brokers, Kubernetes, multi-tenancy, DDD layers. The two docs that once proposed these (`implementation-plan.md`, `devtrack-architecture.html`) were deleted in TASK-109 — don't recreate them.

**PostgreSQL is wanted** before commercial launch (user, 2026-07-13) — server-side only, opt-in via `POSTGRES_URL`, SQLite stays the default so offline-first Rule 0 holds. The Go client never speaks Postgres.
