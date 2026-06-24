---
name: Project current state
description: dev tip 713865d — Phases 0–6 COMPLETE; Phase 7 IN PROGRESS (TASK-093+094 merged to dev; TASK-095–097 remaining); next TASK-098
type: project
---

**Version:** v3.0.10 on GitHub (main). Active branch: `dev`. GitHub (`sraj0501/Devtrack_`) sole source. Releases: `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Dev tip:** `713865d`. Next task: TASK-098. Python test baseline: 760 pass, 1 pre-existing failure (`test_ollama_host_returns_string`).

**Build arc (PRODUCT_BIBLE.md pivot 2026-06-10):** Phases 0–6 COMPLETE. Phase 7 (PR Puppet Master) IN PROGRESS — TASK-093+094 merged to dev; TASK-095–097 remaining. Phase 8 (MCP + headless) QUEUED (last). See `Data/agent_logs/project_board.md`.

**Layout:** `devtrack_client/` (Go + gitsage), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify → devtrack.cloud). Legacy `devtrack-bin/` + root `backend/` retired (TASK-048).

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `alerts`, `notify`, `telegram`, `reviewer`. Layer order: config/db/health/learning ← trigger ← infra ← daemon; trigger ← tui.

**DEPRIORITISED (post-pivot):** PG-5, Redis R-1→R-6, CLI aesthetics, savings counter, videos, boardroom/plan as primary.

**Platform quirks (non-obvious):**
- Azure WIQL only accepts date-only precision (`2006-01-02`, not RFC3339) — `connectors/azure/list.go:ListWorkItemsChangedAfter`.
- Go notify constructors (`NewTelegramFromConfig`, `NewSlackFromConfig` in `internal/notify/`) must return `Notifier` interface, not concrete type — concrete type causes nil-panic in alert poller when feature is disabled.
