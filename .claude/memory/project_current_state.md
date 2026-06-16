---
name: Project current state
description: Post-pivot (2026-06-10) — direction now PRODUCT_BIBLE.md Phase 0→8; Phase 0-2 complete, Phase 3 active; v3.0.10 shipped; platform quirks
type: project
---

**Live phase/task status:** tracked in user auto-memory `project_current_state.md`, not here — check that file for current Phase/TASK-NNN status (this file covers build history and platform quirks, which change less often).

**Version:** v3.0.10 (latest tag). Active branch: `dev`. GitHub (`sraj0501/Devtrack_`) sole source. Releases: `.\scripts\release.ps1 [-Bump patch|minor|major]`.

**Migration COMPLETE (2026-06-10):** GitLab→GitHub migration achieved. The `migration` branch and the `gitlab-client` / `gitlab-server` / `gitlab-wiki` remotes are now vestigial — safe to remove. (Former "do NOT delete `migration`" protection lifted.)

**Layout:** `devtrack_client/` (Go + gitsage), `devtrack_server/` (Python, port 8089), `devtrack_wiki/` (Netlify → devtrack.cloud). Legacy: `devtrack-bin/` + root `backend/` (TASK-048, no new code).

**Go internal packages** (`devtrack_client/internal/`): `config`, `db`, `health`, `learning`, `trigger`, `infra`, `daemon`, `tui`, `match`, `alerts`, `notify`, `telegram`. Layer: config/db/health/learning ← trigger ← infra ← daemon; trigger ← tui.

**Recently shipped (v3.0 line):** `motor` optional dep; `upgrade.go` → GitHub API + semver guard; Telegram bot migrated to Go (`internal/telegram/`); boardroom + `devtrack plan` live; Windows CLI parity + autostart (Task Scheduler); automated GitHub Actions release pipeline; stale-health fix (migration 005 prunes legacy Redis/MongoDB rows); v3.0.9 `skip_issues` workspace field for dual-platform (GitHub+ADO) duplicate-ticket fix; v3.0.10 significant Windows fixes (isatty via mattn/go-isatty, editor-commit BeforeCommit/AfterCommit hooks, background auto-enhance via `DEVTRACK_AUTO_ENHANCE=true`).

**Build arc — driven by PRODUCT_BIBLE.md (pivot 2026-06-10):** Phase 0 (silent daemon), Phase 1 (pending actions queue), Phase 2 (ticket extractor) COMPLETE. Phase 3 (silent commit handler) ACTIVE. Then Phase 4 EOD pipeline → Phases 5-8 voice training, dialectic self-improvement, TUI-as-visibility, PR-review puppet master. See `Data/agent_logs/project_board.md`.

**DEPRIORITISED (post-pivot):** PG-5 (`stats_client.py` → `/internal/stats`), Redis R-1→R-6, CLI aesthetics/theming, savings counter, how-to videos, boardroom/plan as primary features. Not cancelled — below the phases.

**Open decision:** gitsage AI commit enhancement is client-native (→Ollama), collides with "AI=server" rule — `docs/CAPABILITIES_OWNERSHIP.md`.

**Platform quirks (non-obvious):**
- Azure WIQL only accepts date-only precision (`2006-01-02`, not RFC3339) — `connectors/azure/list.go:ListWorkItemsChangedAfter`. Affects at minimum `process_intelligence` and `ei-rd-eff-deliverymetrics` projects.
- Go notify constructors (`NewTelegramFromConfig`, `NewSlackFromConfig` in `internal/notify/`) must return `Notifier` interface, not concrete `*Telegram`/`*Slack` — returning concrete type causes nil pointer panic in alert poller when feature is disabled.
