# DevTrack Project Board

_Last updated: 2026-05-27 by PM_
_Next DevTrack task ID: TASK-052_
_Branch for all current work: `feature/go-client-standalone`_

---

## INITIATIVE: Go-Client Standalone

**Vision**: The `devtrack` Go binary becomes fully self-contained — PM connectors (GitHub/GitLab/ADO), git-sage, and all client capabilities run natively in Go. The Python server becomes AI-only (NLP, LLM, boardroom, plan decomposition, bots, admin UI, reporting, personalization/RAG). The binary works end-to-end with no Python server running.

**GitHub Issues**: #137 (TASK-A), #138 (TASK-B), #139 (TASK-C), #140 (TASK-D)
**Branch**: `feature/go-client-standalone` (local only — do not push until developer tests)

---

## IN PROGRESS

_None — both active tasks complete. TASK-C and TASK-D pending developer dispatch._

---

## DONE (this initiative)

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

### TASK-C — Clean up Go daemon subprocess list
**GitHub Issue**: https://github.com/sraj0501/Devtrack_/issues/139
**Priority**: MEDIUM
**Depends on**: TASK-A and TASK-B

**Spec**:
Remove subprocess spawning for Python connector scripts and git-sage from `daemon.go` and `integrated.go`. Keep spawning: `webhook_server.py`, `backend.telegram`, `backend.slack`, `alert_poller.py`.

**Acceptance criteria**:
- [ ] `daemon.go` no longer spawns azure/run_sync.py, github/run_sync.py, gitlab/run_sync.py
- [ ] `daemon.go` no longer spawns `python -m backend.git_sage`
- [ ] Remaining spawns (webhook_server.py, bots, alert_poller) unchanged
- [ ] `go build ./...` passes

---

### TASK-D — Update release script for standalone binary
**GitHub Issue**: https://github.com/sraj0501/Devtrack_/issues/140
**Priority**: LOW
**Depends on**: TASK-C

**Spec**:
Update `scripts/release.ps1` release notes template and `devtrack_wiki` download page to reflect that Python server is now optional (AI features only). Verify binary runs without server archive.

**Acceptance criteria**:
- [ ] release.ps1 release notes mention Python server as optional
- [ ] devtrack_wiki download page updated
- [ ] Manual test: binary starts, `github-list` runs, `sage ask` runs — no Python server required

---

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
