---
name: Project current state
description: dev tip 6a40a9f — Phases 0–6 COMPLETE; Phase 7 DEFERRED; Phase 8 COMPLETE (TASK-098+099+100+101 done); PR #205 open targeting dev
type: project
---

**Version:** v3.0.10 on GitHub (main). Active branch: `dev`. GitHub (`sraj0501/Devtrack_`) sole source. Releases: `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Dev tip:** `6a40a9f` (branch `feat/TASK-101-phase8-exit-verification`). PR #205 open targeting `dev`. Next task: TBD (Phase 7 TASK-095–097, or next phase). Python test baseline: 760 pass, 1 pre-existing failure (`test_ollama_host_returns_string`). Zero open GitHub issues (11 superseded Phase B/C/D issues closed 2026-06-24).

**Build arc (PRODUCT_BIBLE.md pivot 2026-06-10):** Phases 0–6 COMPLETE. Phase 7 (PR Puppet Master) DEFERRED — TASK-093+094 done, TASK-095–097 deferred until after Phase 8. Phase 8 (MCP Server + Headless Integration) COMPLETE — TASK-098 (MCP server core: JSON-RPC 2.0, tool registry, stdio transport), TASK-099 (6 read-only SQLite-backed tools: `get_active_context`, `get_today_commits`, `get_pending_actions`, `get_voice_profile`, `get_ticket_context`, `get_eod_summary`), TASK-100+101 (`devtrack mcp setup/status/test` CLI subcommands; `NewDatabase()` now calls `applyMigrationTables()` so migration tables always present on first open) — all merged to dev; exit criterion verified: Claude Code can query active ticket, voice profile, and pending queue without manual context-setting. See `Data/agent_logs/project_board.md`.

**Layout:** `devtrack_client/` (Go + gitsage), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify → devtrack.cloud). Legacy `devtrack-bin/` + root `backend/` retired (TASK-048).

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `alerts`, `notify`, `telegram`, `reviewer`. Layer order: config/db/health/learning ← trigger ← infra ← daemon; trigger ← tui.

**DEPRIORITISED (post-pivot):** PG-5, Redis R-1→R-6, CLI aesthetics, savings counter, videos, boardroom/plan as primary.

**Platform quirks (non-obvious):**
- Azure WIQL only accepts date-only precision (`2006-01-02`, not RFC3339) — `connectors/azure/list.go:ListWorkItemsChangedAfter`.
- Go notify constructors (`NewTelegramFromConfig`, `NewSlackFromConfig` in `internal/notify/`) must return `Notifier` interface, not concrete type — concrete type causes nil-panic in alert poller when feature is disabled.
