---
name: Project current state
description: v3.0.10; Phases 0–8 + Managed Install + TASK-109 cleanup COMPLETE; next is Phase 9 (Adoption Gate, TASK-110–117); PRs #216–218 open
type: project
---

**Version:** v3.0.10. Branch: `dev` (tip `f329c8a`). GitHub `sraj0501/Devtrack_` sole source. Release: `.\scripts\release.ps1 [-Bump patch|minor|major]`. Baseline: 798 Python + 112 Go tests, zero open issues.

**Build arc Phases 0–8: ALL COMPLETE and merged to `dev`.** (Silent daemon → pending-actions queue → ticket extractor → silent commit → EOD → voice/dialectic → PR puppet master → MCP server.) Phase history lives in `Data/agent_logs/project_board.md` — don't re-derive it here.

**Post-arc shipped:** TASK-102 (Azure `IsPRApproved`); TASK-103–108 EPIC Managed Install (`devtrack setup` sparse-clones the Python server + `uv sync`); TASK-109 repo cleanup (32 remote branches → 2; telemetry flipped opt-out → **opt-in**).

**Next: Phase 9 — Adoption Gate** (`docs/NEXT_STEPS.md`), TASK-110–117. Next task ID is **TASK-110**. Packaging and narrative, not new capability: stranger → staged action + Claude Code context in under 10 min.

**Open PRs:** #216 (telemetry opt-in fix) and #217 (docs cleanup) → `dev`; **#218 promotes `dev` → `main`** (main is 175 commits behind — this promotion is the reason TASK-109 happened).

**Deferred (Phase 10+):** headless orchestration (global agent control via MCP), Tier 4 Hermes voice model, GitLab `IsPRApproved`.

**Layout:** `devtrack_client/` (Go; Go-native `gitsage/`; **zero `.py` files**), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify → devtrack.cloud). Legacy `devtrack-bin/` + root `backend/` retired (TASK-048).

**DEPRIORITISED:** PG-5, Redis R-1→R-6, CLI aesthetics, savings counter, videos, boardroom/plan as primary. **Never, ever:** NATS/Redis/external queues, PostgreSQL migration, Kubernetes, multi-tenancy. The two docs proposing these (`implementation-plan.md`, `devtrack-architecture.html`) were **deleted in TASK-109** — they predated the pivot and were a live trap for agents grepping docs before reading memory. Don't recreate them.
