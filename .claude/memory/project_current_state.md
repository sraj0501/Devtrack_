---
name: Project current state
description: dev tip 0fe0aa9; ALL Phases 0–8 COMPLETE; TASK-102 COMPLETE (Azure IsPRApproved); next TASK-103; post-arc mode
type: project
---

**Version:** v3.0.10 on GitHub (main). Active branch: `dev`. GitHub (`sraj0501/Devtrack_`) sole source. Releases: `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Dev tip:** `0fe0aa9` (branch `feat/TASK-102-azure-pr-approved`). Next task: TASK-103 (not yet defined). Python test baseline: 797 pass, 1 pre-existing failure (`test_ollama_host_returns_string`). Zero open GitHub issues.

**Build arc: ALL PHASES 0–8 COMPLETE** (PRODUCT_BIBLE.md pivot 2026-06-10). Phase 7 (PR Puppet Master) shipped PRs #201,202,206,207,208. Phase 8 (MCP Server) shipped PRs #203–205: JSON-RPC 2.0, 6 SQLite-backed tools (`get_active_context`, `get_today_commits`, `get_pending_actions`, `get_voice_profile`, `get_ticket_context`, `get_eod_summary`), `devtrack mcp setup/status/test`.

**Post-arc shipped:** TASK-102 — Real `IsPRApproved` for Azure DevOps (`connectors/azure/pr.go`, `ListPRReviewers`).

**Post-arc queue (not yet tasked):** headless orchestration (global agent control via MCP), Tier 4 Hermes voice model, GitLab `IsPRApproved`, dev → main promotion.

**Layout:** `devtrack_client/` (Go + gitsage), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify → devtrack.cloud). Legacy `devtrack-bin/` + root `backend/` retired (TASK-048).

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `alerts`, `notify`, `telegram`. Layer: config/db/health/learning ← trigger ← infra ← daemon; trigger ← tui.

**DEPRIORITISED:** PG-5, Redis R-1→R-6, CLI aesthetics, savings counter, videos, boardroom/plan as primary.
