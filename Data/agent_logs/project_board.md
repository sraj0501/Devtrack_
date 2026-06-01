# DevTrack Project Board

_Last updated: 2026-05-29 by PM_
_Next DevTrack task ID: TASK-055_
_Branch for all current work: `feature/go-client-standalone`_
_Shipped: v3.0.0 (2026-05-29) — package refactor + client-standalone + connection/config fixes._

---

## INITIATIVE: Go-Client Standalone

**Vision**: The `devtrack` Go binary becomes fully self-contained — PM connectors (GitHub/GitLab/ADO), git-sage, and all client capabilities run natively in Go. The Python server becomes AI-only (NLP, LLM, boardroom, plan decomposition, bots, admin UI, reporting, personalization/RAG). The binary works end-to-end with no Python server running.

**GitHub Issues**: #137 (TASK-A), #138 (TASK-B), #139 (TASK-C), #140 (TASK-D)
**Branch**: `feature/go-client-standalone` (local only — do not push until developer tests)

---

## IN PROGRESS

### TASK-055 — Client-Server Decoupling Phase 2: Native Go alert poller and notifiers
**Branch**: `feat/client-server-decoupling`
**Goal**: Replace Python subprocess spawns for alert polling and notifications with native Go implementations.
**Engineer status**: 6/6 criteria done — last commit: abad449 "feat(alerts): Phase 2 — native Go alert poller and notifiers" — 2026-05-31

**New files**:
- `devtrack_client/connectors/github/notifications.go` — ListNotificationsSince via GitHub Notifications API
- `devtrack_client/connectors/azure/list.go` — added ListWorkItemsChangedAfter (WIQL delta)
- `devtrack_client/internal/alerts/alerts.go` — NotificationFilter type
- `devtrack_client/internal/alerts/github.go` — GitHub alerter
- `devtrack_client/internal/alerts/azure.go` — Azure alerter (per workspace)
- `devtrack_client/internal/alerts/poller.go` — Poller goroutine
- `devtrack_client/internal/notify/notify.go` — Notifier interface, Multi, Terminal
- `devtrack_client/internal/notify/telegram.go` — Telegram Bot API delivery
- `devtrack_client/internal/notify/slack.go` — Slack incoming webhook
- `devtrack_client/internal/notify/os_windows.go` — Windows PowerShell balloon
- `devtrack_client/internal/notify/os_unix.go` — macOS/Linux OS notifications

**Modified files**:
- `devtrack_client/internal/config/config_env.go` — added 12 alert config functions
- `devtrack_client/internal/db/database.go` — added InsertNotificationNew (bool return)
- `devtrack_client/internal/daemon/daemon.go` — removed Python subprocess fields + spawn/restart functions; added startAlertPoller()
- `devtrack_client/internal/health/health.go` — removed telegramPID field, SetTelegramPID, SetTelegramRestartCallback; checkTelegramBot now config-based
- `devtrack_client/cli.go` — handleAlerts() reads SQLite directly (removed uv run call)
- `docs/CLIENT_SERVER_DECOUPLING_PLAN.md` — marked Phase 2 complete

**Acceptance criteria**:
- [x] `go build ./...` passes
- [x] `go vet ./...` passes
- [x] `go test ./...` passes
- [x] Python subprocess spawns for alerts removed from daemon.go
- [x] Native Go alert poller wired into daemon startup
- [x] All notifiers (Terminal, Telegram, Slack, OS) implemented

**COMPLETE** — ready for PM review — 2026-05-31 (commit abad449 on feat/client-server-decoupling)

**Follow-up housekeeping** — commit e993507 "chore: remove stale Python-era config, env vars, and help text" — 2026-05-31
Removed 7 dead functions (GetLearningPythonPath/ScriptPath/DailyScriptPath, IsAzurePollerEnabled, IsGitLabPollerEnabled, IsSlackEnabled, GetHealthAutoRestartTelegram), fileExists helper, 3 EnvConfig fields, 5 stale env vars from setup.go + .env_sample, updated cli_info.go component list to reflect HTTP-boundary architecture. No behaviour changes. build/vet/test clean.

**Follow-up housekeeping** — commit c74179f "chore: clean up .env_sample for client and server" — 2026-05-31
devtrack_client/.env_sample: removed PYTHON_BRIDGE_SCRIPT; replaced Python Telegram/Slack bot vars (TELEGRAM_ALLOWED_CHAT_IDS, TELEGRAM_NOTIFY_*, HEALTH_AUTO_RESTART_TELEGRAM, SLACK_ENABLED, SLACK_BOT_TOKEN, SLACK_APP_TOKEN, SLACK_ALLOWED_CHANNEL_IDS) with TELEGRAM_CHAT_ID + SLACK_WEBHOOK_URL (native Go notifiers); updated section headings; bumped version to v3.0.0. devtrack_server/.env_sample: removed LEARNING_PYTHON_PATH/SCRIPT_PATH/DAILY_SCRIPT_PATH (server imports modules directly) and AZURE_POLL_ENABLED (wrong key); bumped version to v3.0.0. Documentation only — no code changes.

**Follow-up housekeeping** — commit d4d9b5f "chore(server): normalise .env_sample to single-# comments and literal values" — 2026-05-31
devtrack_server/.env_sample: converted all ## / ### section headers to universally valid single-# separator style; unquoted EMAIL_SUBJECT and LEARNING_CRON_SCHEDULE; expanded ${VAR} interpolation in MONGODB_URI/REDIS_URL/POSTGRES_URL to literal defaults; re-added Telegram and Slack bot sections (backend/telegram/ and backend/slack/ are still live Python server modules). Documentation only — no code changes.

---

## INITIATIVE: v3.x — Polish & Growth

**Vision**: Now that the v3.0 standalone client is shipped and stable, the next arc is about making DevTrack *delightful* and *visible* — a beautiful themed CLI, a quiet "look how much I saved you" signal, and the first round of how-to videos to drive adoption. Keep the project visibly alive and earn user trust.

**Sequencing**: TASK-052 (CLI aesthetics) first — it's the highest-impact item and makes the videos (TASK-054) look great. TASK-053 (savings counter) can run in parallel as a small self-contained win. Videos last, recording the polished CLI.

---

## PLANNED — v3.x

### TASK-052 — CLI aesthetics & theming  ⭐ HIGHEST PRIORITY
**Goal**: Make the `devtrack` CLI genuinely good-looking, consistent, and themeable.

**Approach**:
- Adopt **lipgloss** (Charmbracelet — already using bubbletea for the TUI) as the styling layer.
- Define a **Theme** = palette (primary / accent / success / warn / error / muted) + styled primitives (headers, boxes, tables, badges, spinners, key-value rows).
- **Theme selection**: `DEVTRACK_THEME` env var + config file + `devtrack theme list` / `devtrack theme set <name>`.
- **Graceful degradation**: honor `NO_COLOR`, auto-plain on non-TTY / piped output, handle Windows terminal color quirks.
- Route all CLI output through styled helpers (retire ad-hoc `fmt.Println`).

**Phases**:
- [ ] Phase 1 — theme engine + palette types + the default (cyan/dark, matches website brand)
- [ ] Phase 2 — restyle high-traffic commands: `status`, `git commit`, `plan`, `boardroom`
- [ ] Phase 3 — roll out to remaining commands + ship a theme set (cyan/dark, dracula, solarized, mono/no-color, light)

**Acceptance**: consistent styling across commands; theme switch works and persists; plain output when piped / `NO_COLOR`; no Windows rendering regressions.

---

### TASK-053 — "Work you didn't write" savings counter
**Goal**: Silently track and subtly surface how much manual effort DevTrack saved the user.

**Metric (decided)**: proxy from the text DevTrack auto-generated *for* the user — commit messages, work updates, reports, PM comments, plans. Sum characters → approx tokens (≈ chars/4). Framed as "typing/effort you skipped," not AI cost.

**Approach**:
- SQLite counter table (cumulative + per-category: commits / updates / reports / comments / plans), incremented wherever DevTrack emits generated text on the user's behalf.
- Display **silently / unobtrusively**: a line in `devtrack status` (e.g. "DevTrack has saved you ~48,200 tokens — 180 commit messages, 92 ticket updates, 14 reports") and optionally a tiny footer after an enhanced commit.
- Counts accrue in standalone mode too (commit-message generation is local).

**Acceptance**: counter persists across restarts; categories tracked; display is subtle and opt-out-able; works offline.

---

### TASK-054 — "How-to" video series  (due 2026-06-30)
**Goal**: Publish the first round of how-to videos before the end of June 2026. **Do after TASK-052** so the CLI looks polished on camera.

**Proposed topics (script + record each)**:
- [ ] Install & setup (`devtrack setup`) — 0 → first run
- [ ] Your first auto-standup (commit → standup update)
- [ ] Plan command — plain-English → Epics/Stories/Tasks
- [ ] The Boardroom — 7-persona plan review + verdict
- [ ] Offline mode — 100% local (Ollama + SQLite)
- [ ] Telegram bot — control DevTrack from your phone
- [ ] PM sync — GitHub / Azure DevOps / Jira

**Tooling**: terminal demos via asciinema or `vhs` (reproducible, clean) showcasing the themed CLI from TASK-052.

**Acceptance**: first batch published (devtrack.cloud / YouTube) before 2026-06-30.

---

## DONE (this initiative)

### TASK-F — Complete Go package refactor (trigger / infra / daemon / tui)
**Completed**: 2026-05-29
**Branch**: `feature/package-refactor`
**Build**: `go build ./...` ✓  `go vet ./...` ✓  cross-compile linux/darwin (amd64+arm64) ✓  binary smoke-test ✓

**Layered split (deviated from original infra/daemon/tui plan — see below):**
The board's 3-package plan (infra + daemon + tui) was **not acyclic as written**: `HTTPTriggerClient` (http_trigger.go) is used by both the CLI and the runtime, yet depends on the trigger data structs in ipc.go. Resolved by extracting a 4th, lower-layer `internal/trigger` package. Final dependency layering is acyclic:

`config/db/health/learning` ← `trigger` ← `infra` ← `daemon`, plus `trigger` ← `tui`, and `package main` (CLI) on top.

**What moved**:
- `internal/trigger` — http_trigger.go, tls_cert.go, + CommitTriggerData/TimerTriggerData/TaskUpdateData (split out of ipc.go into types.go). Tests: http_trigger_test.go, api_contract_test.go moved here; config-only tests relocated to internal/config/server_config_test.go.
- `internal/infra` — ipc.go (IPC server/client + Create*Message), integrated.go, scheduler.go, git_monitor.go, infra.go. Added `IntegratedMonitor.Scheduler()` / `.Database()` accessors.
- `internal/daemon` — daemon.go, daemon_unix.go, daemon_windows.go, http_api.go, lock_unix.go, lock_windows.go, process_unix.go, process_windows.go, queue.go, ping.go. Exported `CheckProcessAlive` / `ReadPID` / `SendReloadSignal`; added `Daemon.Monitor()` accessor; HealthMonitor wiring + `health.IsProcessAlive` init moved here from package main.
- `internal/tui` — tui.go, tui_activity.go, tui_alerts.go, tui_overview.go, tui_workspaces.go.
- `internal/config` — gained `DevtrackDataHome()` (moved from setup.go).

**Shims** (package main): trigger_shim.go, infra_shim.go, daemon_shim.go, tui_shim.go — type aliases + function forwards for the symbols the CLI still references. health_shim.go trimmed to the `isProcessAlive` wrapper.

**Cleanup**: removed 10 empty 1-line `package main` stub files left over from TASK-E (config.go, config_env.go, server_config.go, cloud.go, loadenv.go, database.go, migrations.go, health.go, learning.go, license.go).

**Acceptance criteria**:
- [x] All packages compile independently
- [x] `go build ./...` passes
- [x] `go vet ./...` passes
- [x] No remaining cross-package symbol refs without proper import

---

### TASK-E — Full Go package refactor
**Completed**: 2026-05-29
**Branch**: `feature/package-refactor`
**Build**: `go build ./...` ✓  `go vet ./...` ✓

**What moved**:
- `internal/config` — config.go, config_env.go, server_config.go, cloud.go, loadenv.go
- `internal/db` — database.go, migrations.go (+ Database.Exec() method)
- `internal/health` — health.go
- `internal/learning` — learning.go, license.go

**Shims**: config_shim.go, db_shim.go, health_shim.go, learning_shim.go — type aliases + function forwards; zero call-sites changed.

**CLI split**: cloud_cli.go (cloud login/logout/status), license_cli.go (login/logout/license/terms/telemetry/EnsureTermsAccepted) extracted from cloud.go and license.go.

**Not moved** (deferred — higher complexity): internal/infra (ipc, integrated, scheduler, git_monitor), internal/tui, internal/daemon.

---

### TASK-D — Update release script for standalone binary
**GitHub Issue**: https://github.com/sraj0501/Devtrack_/issues/140
**Completed**: 2026-05-29
**Branch**: `feature/go-client-standalone`
**Vision check**: PASS
**Hardcoded scan**: CLEAN

**Changes**:
- `scripts/release.ps1` — release notes template already had correct two-mode messaging (Standalone / Full); no change needed
- `devtrack_wiki/wiki/download.html` — hero description updated to mention standalone capability; "Everything is in the single binary" section rewritten to explain two deployment modes; Docker chip replaced with "Python server (optional, for AI features)"; step 2 description removed MongoDB/Redis reference

**Acceptance criteria**:
- [x] release.ps1 release notes mention Python server as optional
- [x] devtrack_wiki download page updated with two-mode messaging
- [ ] Manual test: binary starts, `github-list` runs, `sage ask` runs — no Python server required (developer to verify)

---

### TASK-C — Clean up Go daemon subprocess list
**GitHub Issue**: https://github.com/sraj0501/Devtrack_/issues/139
**Completed**: 2026-05-27 (completed implicitly in commit 117bd58)
**Branch**: `feature/go-client-standalone`
**Vision check**: PASS

**Notes**: Acceptance criteria were satisfied by commit 117bd58 ("Making the client side actions independent of python server"). `run_sync.py` and `python -m backend.git_sage` subprocess spawning were removed as part of TASK-A/B implementation. Remaining Python spawns (webhook_server.py, Telegram, Slack, alert_poller.py) are unchanged.

---

### TASK-A — Port PM Connectors to Go
**GitHub Issue**: https://github.com/sraj0501/Devtrack_/issues/137
**Completed**: 2026-05-27
**Branch**: `feature/go-client-standalone`
**Vision check**: PASS
**Hardcoded scan**: CLEAN

**Spec**:
Create `devtrack_client/connectors/` with three sub-packages: `github/`, `gitlab/`, `azure/`. Each sub-package contains:
- `client.go` — HTTP client struct, base URL, auth header setup
- `list.go` — list issues/work items assigned to user
- `view.go` — view a single issue/work item by ID
- `sync.go` — full sync: fetch + upsert to SQLite via `database.go`
- `check.go` — check for new/changed items since last sync

Auth env vars:
- GitHub: `GITHUB_TOKEN`
- GitLab: `GITLAB_PAT`
- Azure DevOps: `AZURE_DEVOPS_PAT`, `AZURE_ORG`, `AZURE_PROJECT`

Results stored to existing SQLite via `database.go` connection pattern. No new database files — extend existing schema if needed.

CLI wiring: add cases in `cli.go` (devtrack_client/cli.go) for:
- `github-list`, `github-view`, `github-sync`, `github-check`
- `gitlab-list`, `gitlab-view`, `gitlab-sync`, `gitlab-check`
- `azure-list`, `azure-view`, `azure-sync`, `azure-check`

Start with GitHub as reference implementation. GitLab and Azure follow the same pattern.

Do NOT push or create PRs. Leave as working tree edits on `feature/go-client-standalone`.

**Acceptance criteria**:
- [x] `devtrack_client/connectors/github/` package builds cleanly
- [x] `devtrack_client/connectors/gitlab/` package builds cleanly
- [x] `devtrack_client/connectors/azure/` package builds cleanly
- [x] CLI commands `github-list`, `github-view`, `github-sync`, `github-check` routed in cli.go
- [x] CLI commands `gitlab-list`, `gitlab-view`, `gitlab-sync`, `gitlab-check` routed in cli.go
- [x] CLI commands `azure-list`, `azure-view`, `azure-sync`, `azure-check` routed in cli.go
- [x] No hardcoded credentials — auth via os.Getenv; public API base URLs overridable via env vars
- [x] `go vet ./...` passes

**Notes**: All 12 Python subprocess handlers replaced. `requiresManagedMode()` guard removed from all 12 — connectors now work in both Lightweight and Managed mode. `Database.DB()` accessor added.

---

### TASK-B — Port git-sage to Go
**GitHub Issue**: https://github.com/sraj0501/Devtrack_/issues/138
**Completed**: 2026-05-27
**Branch**: `feature/go-client-standalone`
**Vision check**: PASS
**Hardcoded scan**: CLEAN

**Spec**:
IMPORTANT: The `gitsage/` package at `devtrack_client/gitsage/` already exists with working Go implementations of `agent.go` (agentic loop with Ask/Do/Interactive), `llm.go` (Ollama HTTP client, JSON mode), and `context.go` (git state collection). Build ON TOP of this existing package — do not create a new `sage/` package. Extend `gitsage/` with the missing files.

Current state of `devtrack_client/gitsage/`:
- `agent.go` — agentic loop, Ask/Do/Interactive functions (DONE)
- `llm.go` — Ollama HTTP chat client, LLMConfig, Ping (DONE)
- `context.go` — RepoContext, CollectContext, Format (DONE)
- `setup.go` — exists (check contents before touching)
- Python files (`.py`) — reference only, do not modify

Add these missing files to `devtrack_client/gitsage/`:
1. `config.go` — env var accessors for SAGE_MODEL, SAGE_PROVIDER, OPENAI_API_KEY, OPENAI_BASE_URL; extend LoadLLMConfig() to support OpenAI-compatible endpoints. Safe defaults: model=llama3.2.
2. `git_ops.go` — structured git operations via os/exec: status (parsed), staged files list, commit, add, reset --soft, stash, branch list, merge, log structured, blame, diff full text. Each returns typed struct, not raw string.
3. `conflict.go` — detect conflicted files (git status | grep "^UU\|^AA\|^DD"), read conflict markers, resolution strategies (ours/theirs/both/smart), apply resolution.
4. `cli.go` — bubbletea-based approval dialog for `do` mode ("auto / review / suggest-only"), follow-up loop (up to 5 questions after task completes), command history using a simple slice.

Wire in `devtrack_client/cli.go`:
- `sage ask "<question>"` → calls gitsage.Ask()
- `sage do "<task>"` → calls gitsage.Do() with approval dialog
- `sage` (no subcommand) → calls gitsage.Interactive()

Add `sage` to the no-daemon command list in cli.go NewCLI() (line 25 area).

Build files in this order:

1. `config.go` — env var accessors: `SAGE_MODEL`, `SAGE_PROVIDER`, `OLLAMA_HOST`, `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `GIT_SAGE_DEFAULT_MODEL`. All via `os.Getenv` with clear defaults where safe (model default is `llama3`).
2. `llm.go` — HTTP client for Ollama (`POST /api/chat`) and OpenAI-compatible (`POST /v1/chat/completions`) endpoints. JSON mode enforcement. Strips `provider/` prefix from model names.
3. `context.go` — git state collection: current branch, recent commits, staged changes, PR number. Uses `os/exec` to call `git`.
4. `git_ops.go` — git operations via `os/exec`: status, add, commit, reset, stash, branch, merge, diff, log, blame. 
5. `conflict.go` — conflict detection, analysis, and resolution strategies (both-sides, ours, theirs, smart).
6. `agent.go` — agentic loop: plan → execute → observe → retry. `max_steps` cap (default 30). Step log with HEAD snapshots for undo. Suggest-only mode.
7. `cli.go` — `ask`, `do`, `interactive` modes. Approval dialog using bubbletea (reference existing bubbletea usage in the project). Follow-up loop (up to 5 questions). Command history.

Reference the Python implementation at `devtrack_client/git_sage/` for logic and behavior. The Go version does not need to be feature-identical on day 1 — focus on `ask` and `do` modes first, then interactive.

Wire CLI in `devtrack_client/cli.go`:
- `sage ask "<question>"`
- `sage do "<task>"`
- `sage` (no args → interactive mode)

Personalization: make an optional HTTP GET to `http://<WEBHOOK_SERVER_HOST>:<WEBHOOK_SERVER_PORT>/personalization/style` — if it fails (server not running), continue silently.

Do NOT push or create PRs. Leave as working tree edits on `feature/go-client-standalone`.

**Acceptance criteria**:
- [x] `devtrack_client/gitsage/` package builds cleanly (extended existing package, not new `sage/`)
- [x] `sage ask`, `sage do`, `sage` (interactive) routed in cli.go
- [x] LLM config supports Ollama, OpenAI-compat, Groq via env vars
- [x] Git context collection works (branch, commits, staged/unstaged changes)
- [x] Conflict detection via git status markers + git grep
- [x] Conflict resolution strategies: ours, theirs, both, smart
- [x] Agentic loop with command interception in review/suggest-only modes
- [x] Approval dialog shown before first `do` command (auto/review/suggest-only)
- [x] Follow-up loop (up to 5 questions with refreshed context)
- [x] Command history tracked and printed
- [x] Works with no Python server running (offline-first)
- [x] `go vet ./...` passes

**Notes**: Built on existing gitsage/ package (agent.go, llm.go, context.go already present). Added config.go, git_ops.go (16 structured git operation methods), conflict.go (4 strategies), cli.go (approval dialog, follow-up loop, command history, RunAsk/RunDo/RunInteractive). Undo via ResetSoft is available via git_ops.go but not wired to a dedicated `undo` command yet (requires step-log HEAD snapshots — deferred to TASK-C cleanup).

---

## PLANNED


## DONE (summary carried from previous board)

### EPIC-SPLIT (2026-05-24, branch `features/SPLIT-001-monorepo-restructure`)

| Task | What |
|---|---|
| TASK-041 | Audit + split-manifest.md: every file catalogued by owner |
| TASK-042 | `devtrack_client/` skeleton (Go files + git_sage copy); go build/vet/test pass |
| TASK-043 | `devtrack_server/` skeleton (Python backend copy); pytest 549 pass |
| TASK-044 | `docs/HTTP_API.md` + Go + Python API contract tests |
| TASK-045 | `.github/workflows/client.yml` (GitHub Actions, 3-job, matrix cross-compile) |
| TASK-046 | `ci/devtrack_server_new.gitlab-ci.yml` (GitLab CI, 4-job, uv sync + docker) |
| TASK-047 | CLAUDE.md + README + docs/ARCHITECTURE.md updated for three-codebase split |
| TASK-048 | Retired `devtrack-bin/`, root `backend/`, `bin/`, `demo/`, `python_bridge.py` (281 files, 69k lines) |

### Earlier sessions

| Task | What |
|---|---|
| TASK-040 | DevTrack logo in website nav/footer + Windows binary icon via goversioninfo |
| TASK-029–033 | pyproject.toml two-tier deps; is_ai_available(); devtrack-server features/enable cmds; GitLab CI core+full jobs |
| TASK-026–028 | Remove dead GetPythonBridgePath; guard work-report in Lightweight mode; internal HTTP control API + cross-platform AlertNotifier |
| TASK-025 | Windows native build: build-tag syscall split (daemon_unix/windows, cli_unix/windows) |
| TASK-021–024 | setup.go mode wizard; Lightweight mode skips Python; capability guards; non-fatal config accessors |
| TASK-018–020 | CS-3 hardcoded value audit (high/med/low); inbound webhook integration tests |
| TASK-011–015 | CS-3 Admin GUI MVP: route tests, role/disable, license page, trigger stats, polish+embed |
| TASK-010 | Full documentation + memory audit |
| TASK-007–009 | CS-2 headless tests (37); stats panel; os.getenv remaining fixes |
| TASK-001–006 | Config audit: 50+ config accessors, all os.getenv eliminated across 22 files |
| TASK-000 | v1.0.0 release + local agents setup |
