---
name: Project current state
description: dev tip f329c8a; Phases 0–8 + Managed Install + TASK-109 cleanup COMPLETE; next is Phase 9 (Adoption Gate, TASK-110–117); dev ~200 commits ahead of main
type: project
---

**Version:** v3.0.10. Active branch: `dev`. GitHub (`sraj0501/Devtrack_`) sole source. Releases: `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Dev tip:** `f329c8a`. Test baseline: 798 Python tests, 112 Go tests. Zero open GitHub issues.

**Build arc: ALL PHASES 0–8 COMPLETE** (PRODUCT_BIBLE.md pivot 2026-06-10). Phase 7 (PR Puppet Master) shipped PRs #201,202,206,207,208. Phase 8 (MCP Server) shipped PRs #203–205: JSON-RPC 2.0, 6 SQLite-backed tools (`get_active_context`, `get_today_commits`, `get_pending_actions`, `get_voice_profile`, `get_ticket_context`, `get_eod_summary`), `devtrack mcp setup/status/test`.

**Post-arc shipped:**
- TASK-102 — Real `IsPRApproved` for Azure DevOps (`connectors/azure/pr.go`, `ListPRReviewers`).
- TASK-103–108 — EPIC Managed Install: `devtrack setup` sparse-clones the Python server + runs `uv sync`; daemon fallback, upgrade, Windows autostart env, `docs/INSTALLATION.md`.
- TASK-109 — Repo cleanup: 32 remote branches → 2; doc drift fixed against code; **telemetry flipped opt-out → opt-in** (the old opt-out never worked — `devtrack telemetry off` called the server and never wrote the local marker `ping.go` checks).

**Next: Phase 9 — Adoption Gate** (`docs/NEXT_STEPS.md`), TASK-110–117. Packaging and narrative, not new capability. **Open:** `dev` is ~200 commits ahead of `main` — promotion PR overdue.

**Deferred (Phase 10+):** headless orchestration (global agent control via MCP), Tier 4 Hermes voice model, GitLab `IsPRApproved`.

**Layout:** `devtrack_client/` (Go + Go-native `gitsage/`; zero `.py` files), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify → devtrack.cloud). Legacy `devtrack-bin/` + root `backend/` retired (TASK-048).

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `alerts`, `notify`, `telegram`, plus `ticket`, `match`, `reviewer` (Phase 7), `mcp` (Phase 8). Layer: config/db/health/learning ← trigger ← infra ← daemon; trigger ← tui.

**DEPRIORITISED:** PG-5, Redis R-1→R-6, CLI aesthetics, savings counter, videos, boardroom/plan as primary. **Never:** NATS/Redis/external queues, PostgreSQL migration, Kubernetes, multi-tenancy — `docs/implementation-plan.md` and `docs/devtrack-architecture.html` propose these and are both marked SUPERSEDED.
