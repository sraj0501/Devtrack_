---
name: Project current state
description: Phase 6 IN PROGRESS (TASK-087 active on feat/TASK-087-inference-injection); Phases 0–5 COMPLETE; v3.0.10; platform quirks
type: project
---

**Version:** v3.0.10. Active branch: `feat/TASK-087-inference-injection` (from dev tip `b557b58`), all PRs through #193 merged. GitHub (`sraj0501/Devtrack_`) sole source. Releases: `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Build arc (PRODUCT_BIBLE.md pivot 2026-06-10):** Phases 0–5 COMPLETE. Phase 6 (dialectic self-improvement) IN PROGRESS — TASK-085 + TASK-086 merged; TASK-087 (inference-to-generation injection) ACTIVE. Then Phases 7-8 (TUI-as-visibility, PR puppet master). See `Data/agent_logs/project_board.md`.

**Python test baseline:** 753 pass, 1 pre-existing failure (`test_ollama_host_returns_string`).

**Layout:** `devtrack_client/` (Go + gitsage), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify → devtrack.cloud). Legacy `devtrack-bin/` + root `backend/` retired (TASK-048).

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `alerts`, `notify`, `telegram`. Layer order: config/db/health/learning ← trigger ← infra ← daemon; trigger ← tui.

**DEPRIORITISED (post-pivot):** PG-5, Redis R-1→R-6, CLI aesthetics, savings counter, videos, boardroom/plan as primary.

**Platform quirks (non-obvious):**
- Azure WIQL only accepts date-only precision (`2006-01-02`, not RFC3339) — `connectors/azure/list.go:ListWorkItemsChangedAfter`.
- Go notify constructors (`NewTelegramFromConfig`, `NewSlackFromConfig` in `internal/notify/`) must return `Notifier` interface, not concrete type — concrete type causes nil-panic in alert poller when feature is disabled.
