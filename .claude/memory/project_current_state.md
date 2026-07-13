---
name: Project current state
description: v3.0.10; Phases 0–8 + Managed Install + TASK-109/110/111 COMPLETE; queued are the Postgres epic (TASK-112–116) and Phase 9 Adoption Gate (TASK-117–124); no open PRs
type: project
---

**Version:** v3.0.10. Branch: `dev` (tip `f329c8a`). GitHub `sraj0501/Devtrack_` sole source. Release: `.\scripts\release.ps1 [-Bump patch|minor|major]`. Baseline: 798 Python + 112 Go tests, zero open issues.

**Build arc Phases 0–8: ALL COMPLETE and merged to `dev`.** (Silent daemon → pending-actions queue → ticket extractor → silent commit → EOD → voice/dialectic → PR puppet master → MCP server.) Phase history lives in `Data/agent_logs/project_board.md` — don't re-derive it here.

**Post-arc shipped:** TASK-102 (Azure `IsPRApproved`); TASK-103–108 EPIC Managed Install (`devtrack setup` sparse-clones the Python server + `uv sync`); TASK-109 repo cleanup (32 remote branches → 2; telemetry flipped opt-out → **opt-in**); TASK-110/111 (website + wiki reconciled with the shipped product).

**Task-ID ledger:** `Data/agent_logs/project_board.md` is authoritative. Next ID is **TASK-125**.

**Queued, in priority order:**
1. **EPIC: PostgreSQL Backend — TASK-112–116.** Must land before commercial launch; server-side only. See [[postgres_epic]] and the epic section at the bottom of the board.
2. **Phase 9 — Adoption Gate — TASK-117–124** (`docs/NEXT_STEPS.md`). Packaging and narrative, not new capability: stranger → staged action + Claude Code context in under 10 min. (These were originally numbered TASK-110–117; renumbered 2026-07-13 after the board issued 110–116.)

**Open PRs: none.** #216/#217/#218 merged; **#220 (`dev` → `main`) merged 2026-07-13** — `main` is current again.

**Deferred (Phase 10+):** headless orchestration (global agent control via MCP), Tier 4 Hermes voice model, GitLab `IsPRApproved`.

**Layout:** `devtrack_client/` (Go; Go-native `gitsage/`; **zero `.py` files**), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify → devtrack.cloud). Legacy `devtrack-bin/` + root `backend/` retired (TASK-048).

**DEPRIORITISED:** PG-5 (**a stats pipeline — nothing to do with the Postgres epic**; don't conflate them), Redis R-1→R-6, CLI aesthetics, savings counter, videos, boardroom/plan as primary. **Never, ever:** NATS/Redis/external queues/brokers, Kubernetes, multi-tenancy, DDD layers. The two docs proposing these (`implementation-plan.md`, `devtrack-architecture.html`) were **deleted in TASK-109** — they predated the pivot and were a live trap for agents grepping docs before reading memory. Don't recreate them.

**PostgreSQL is no longer on the never list** (user, 2026-07-13). It is wanted before commercial launch — server-side only, opt-in via `POSTGRES_URL`, SQLite stays the default so offline-first Rule 0 holds. The Go client never speaks Postgres. See [[postgres_epic]].
