# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**DevTrack** is a developer automation tool that monitors Git activity and scheduled timers, prompting developers for work updates and routing them through an AI pipeline to update project management systems and generate reports.

## Codebase Map

This monorepo contains three independent codebases undergoing a structural split (EPIC-SPLIT):

| Directory | Language | Role |
|---|---|---|
| `devtrack_client/` | Go + Python (git-sage) | **CLIENT** — binary, git monitor, scheduler, CLI, git-sage bundled tool. New canonical source. |
| `devtrack_server/` | Python | **SERVER** — AI pipeline, webhook server, NLP, LLM, admin UI, integrations. New canonical source. |
| `devtrack_wiki/` | HTML/Markdown | **DOCS** — website and wiki (pushed to GitLab wiki remote separately). |
| `devtrack-bin/` | Go | **LEGACY** — original Go source. Being retired in TASK-048. Do not add new code here. |
| `backend/` | Python | **LEGACY** — original Python source. Being retired in TASK-048. Do not add new code here. |

See `docs/split-manifest.md` for the full file-ownership catalogue.
See `docs/HTTP_API.md` for the HTTP/JSON boundary between client and server.

## Build & Run Commands

### Go client (devtrack_client/) — canonical

```bash
cd devtrack_client
go build -o devtrack .          # Build binary
go test ./...                   # Run all Go tests
go vet ./...                    # Run Go linter
```

### Python server (devtrack_server/) — canonical

```bash
cd devtrack_server
uv sync                         # Install/sync dependencies (uv manages the venv)
uv run pytest backend/tests/    # Run all Python tests (uses conftest.py setup)
uv run pytest backend/tests/test_nlp_parser.py  # Run single test file
uv run pytest backend/tests/ -k test_name       # Run tests by name filter
```

### Legacy paths (devtrack-bin/ and root backend/) — do not use for new work

```bash
cd devtrack-bin
go build -o devtrack .          # LEGACY — use devtrack_client/ instead
uv run pytest backend/tests/    # LEGACY — use devtrack_server/ instead
```

### Testing Patterns

- **Test structure**: `backend/tests/` uses pytest with `conftest.py` that adds repo root to `sys.path`
- **LLM provider isolation**: Tests that change `LLM_PROVIDER` must call `reset_provider_cache()` before/after to avoid cross-test contamination
- **Optional imports**: Python subsystems (NLP, TUI, LLM, report generator) degrade gracefully if dependencies are missing

### Running the daemon locally

```bash
# Source .env so all vars are in the process environment before the daemon starts
export PROJECT_ROOT="/path/to/automation_tools"
source "$PROJECT_ROOT/.env"
devtrack start &                # Start background daemon (starts webhook_server.py as subprocess)
devtrack status                 # Verify it's running
devtrack logs                   # View recent log output
```

> The daemon reads env vars from the process environment at startup — it does not reload `.env`
> at runtime. For persistent autostart use `devtrack autostart-install` which bakes all vars
> into the launchd plist (macOS) or systemd unit (Linux).

## Architecture

Three independent codebases share a single HTTP/JSON boundary. See `docs/HTTP_API.md` for the full API contract.

```
devtrack_client/          devtrack_server/              devtrack_wiki/
(Go binary + git-sage)    (Python AI pipeline)          (docs site)
        │                          │                           │
        │   HTTPS POST /trigger/*  │                    GitLab wiki remote
        │ ─────────────────────── ▶│
        │                          │
Git commits / cron timer    NLP / LLM / Admin UI
CLI commands                Azure DevOps / GitHub / Jira
SQLite (Data/db/)           Microsoft Graph (Teams/Email)
```

The Go client sends triggers to the Python server over HTTPS POST (CS-1 transport). TCP IPC (`127.0.0.1:35893`) is retained as a legacy internal channel only. There is no shared compiled artefact between client and server — the boundary is HTTP/JSON only.

**LEGACY paths** (being retired in TASK-048):
- `devtrack-bin/` — original Go source, mirror of `devtrack_client/`. Do not add new code here.
- `backend/` at monorepo root — original Python source, mirror of `devtrack_server/backend/`. Do not add new code here.

### Go layer (`devtrack_client/`)

| File | Purpose |
|---|---|
| `devtrack_client/main.go` | Entry point; routes CLI args. The `git` subcommand runs Go-natively via `gitsage.RunGit` (no shell wrapper) |
| `devtrack_client/cli.go` | All CLI command implementations (`start`, `stop`, `status`, `logs`, etc.) |
| `devtrack_client/daemon.go` | Lifecycle management (PID file, signals, webhook server subprocess) |
| `devtrack_client/integrated.go` | `IntegratedMonitor` — wires together git monitor, scheduler, and IPC server |
| `devtrack_client/git_monitor.go` | fsnotify-based Git repository watcher; fires `commit_trigger` on new commits |
| `devtrack_client/scheduler.go` | Cron-based periodic trigger using robfig/cron; fires `timer_trigger` |
| `devtrack_client/ipc.go` | TCP IPC server (Go side); JSON-delimited messages, one handler per `MessageType` |
| `devtrack_client/database.go` | SQLite via modernc.org/sqlite; stores trigger history and task updates |
| `devtrack_client/config.go` | YAML config struct (`Data/configs/config.yaml`); all runtime values via `config_env.go` |
| `devtrack_client/config_env.go` | All `.env` key accessors for Go — the single source of truth for env var names |
| `devtrack_client/learning.go` | Personalized AI learning consent and profile management |
| `devtrack_client/cli_boardroom.go` | `devtrack boardroom` command — multi-persona AI plan review with SWOT, verdict, and interactive chat |
| `devtrack_client/cli_plan.go` | `devtrack plan` command — decompose a problem into Epic/Story/Task hierarchy and create on PM platform |
| `devtrack_client/versioninfo.json` | Windows binary version metadata for `goversioninfo` (`go generate` embeds as `resource_windows_amd64.syso`) |
| `devtrack_client/resource_windows_amd64.syso` | Pre-built Windows resource object (icon + version info); `_windows_amd64` suffix constrains linkage to that target only |
| `devtrack_client/git_sage/` | Python git-sage agent (client-owned; bundled here, not in devtrack_server/) |
| `devtrack_client/internal/telegram/` | Native Go Telegram bot — starts with daemon; handles /status /logs /health /trigger /pause /resume /stop /restart /reload /commits |

### Python layer (`devtrack_server/backend/`)

The canonical Python source lives at `devtrack_server/backend/`. The root `backend/` directory is a legacy mirror being retired in TASK-048. The server entry point is `devtrack_server/backend/webhook_server.py` (not `python_bridge.py`, which is a legacy reference only).

#### Core Infrastructure

| Module | Purpose |
|---|---|
| `devtrack_server/backend/webhook_server.py` | Primary Python entry point started by Go client; FastAPI server handling inbound webhook events AND outbound triggers from Go |
| `devtrack_server/backend/webhook_handlers.py` | `WebhookEventHandler` — routes Azure/GitHub/Jira events; separated from HTTP routing for testability |
| `devtrack_server/backend/config.py` | Centralized config — all modules use `backend.config.get()`, not `os.getenv()` directly |
| `devtrack_server/backend/ipc_client.py` | TCP IPC client (Python side) — legacy internal channel; mirrors message types from Go's `ipc.go` |
| `python_bridge.py` | Legacy bridge at monorepo root (kept for reference only); superseded by `webhook_server.py` |

#### Boardroom & Plan

| Module | Purpose |
|---|---|
| `devtrack_server/backend/boardroom/` | Multi-persona AI boardroom module: `personas.py` (7 personas), `session.py` (session orchestration), `interactive.py` (chat loop), `report.py` (SWOT/verdict rendering) |
| `devtrack_server/backend/plan_parser.py` | Parses free-text or Markdown plan files into structured sections for the boardroom and plan commands |

#### NLP & AI Processing

| Module | Purpose |
|---|---|
| `devtrack_server/backend/nlp_parser.py` | spaCy-based NLP for commit/user text → structured task data (entity extraction, action detection) |
| `devtrack_server/backend/description_enhancer.py` | Ollama-based description enhancement and categorization |
| `devtrack_server/backend/llm/` | Multi-provider LLM abstraction (`provider_factory.py` builds fallback chain: primary → OpenAI/Anthropic → Ollama) |
| `devtrack_server/backend/personalized_ai.py` | AI learning from user communications for personalized responses |
| `devtrack_server/backend/learning_integration.py` | Learning consent management and profile handling |

#### User Interaction & Reporting

| Module | Purpose |
|---|---|
| `devtrack_server/backend/user_prompt.py` | Terminal TUI for interactive work-update prompts |
| `devtrack_server/backend/daily_report_generator.py` | AI-enhanced daily/weekly report generation (multiple output formats: Terminal, HTML, Markdown, JSON) |
| `devtrack_server/backend/email_reporter.py` | Report delivery via email/Teams |
| `devtrack_server/backend/task_matcher.py` | Fuzzy + semantic matching of natural language to tracked tasks |

#### Git Integration

| Module | Purpose |
|---|---|
| `devtrack_server/backend/commit_message_enhancer.py` | AI-powered iterative commit message refinement (multi-attempt workflow) |
| `devtrack_server/backend/git_diff_analyzer.py` | Analyze staged changes to enhance context for commit messages |

Note: `git_sage/` is **client-owned** and lives at `devtrack_client/git_sage/`, not in devtrack_server. It is invoked as `python -m backend.git_sage` from the client's bundled Python environment.

##### git-sage Sub-modules (`devtrack_client/git_sage/`)

| Module | Purpose |
|---|---|
| `cli.py` | Ask/do/interactive modes; session approval dialog; command history; offer_undo; follow-up loop |
| `agent.py` | Agentic loop with suggest_only mode, step_log (HEAD snapshots), followup(), undo_step() |
| `llm.py` | Ollama + OpenAI-compatible backends; json_mode enforcement; strips LiteLLM `provider/` prefix |
| `context.py` | Git repository state collection and formatting |
| `config.py` | Env-driven config: `.env` > `~/.config/git-sage/config.json` > hard defaults |
| `git_operations.py` | Advanced git operations: branches, commits, merges, status, blame, stash (300+ lines) |
| `conflict_resolver.py` | Intelligent conflict analysis and resolution with multiple strategies (280+ lines) |
| `pr_finder.py` | PR/MR utilities: metadata extraction, branch analysis, diff statistics (220+ lines) |
| `__main__.py` | Entry point for `python -m backend.git_sage` |

#### External Integrations

| Module | Purpose |
|---|---|
| `devtrack_server/backend/jira/` | Jira REST API client for issue management |
| `devtrack_server/backend/github/` | GitHub API integration (PR analysis, repository insights) |
| `devtrack_server/backend/azure/` | Azure DevOps work item fetching and updating |
| `devtrack_server/backend/msgraph_python/` | Microsoft Graph integration (Teams chat, Outlook email, sentiment analysis) |

#### Utilities & Helpers

| Module | Purpose |
|---|---|
| `devtrack_server/backend/utils/` | Shared utilities (formatting, validation, helpers) |
| `devtrack_server/backend/autodoc/` | Auto-documentation generation from code |
| `devtrack_server/backend/db/` | Database models and migrations |
| `devtrack_server/backend/ai/` | Low-level AI utilities (Ollama client, inference helpers) |

## Configuration Architecture

All configuration flows from environment variables and `workspaces.yaml`. There are **no hardcoded fallback values** for paths or credentials. The env-first model means variables must be in the process environment before the daemon starts — the daemon does not reload `.env` at runtime.

- **Shell**: `source .env` before running `devtrack start` in a terminal session
- **Autostart**: `devtrack autostart-install` bakes all `.env` vars into launchd (macOS) / systemd (Linux) so the daemon always starts with the correct env
- **Docker**: pass `--env-file .env` to `docker run`
- Go reads env vars via `config_env.go` (`LoadEnvConfig()` reads from the process environment)
- Python reads via `backend/config.py` functions (`get()`, `get_int()`, `get_bool()`, `get_path()`)
- Runtime data lives under `Data/` (db, logs, reports, pids) — paths configurable via `DATA_DIR`, `DATABASE_DIR`, etc.

### PM Connector Configuration (two-layer model)

Configuration for PM connectors (GitHub, GitLab, Azure DevOps) is split across two sources:

| Source | What it holds |
|---|---|
| `.env` | Secrets only: `GITHUB_TOKEN`, `GITLAB_PAT`, `AZURE_DEVOPS_PAT` |
| `workspaces.yaml` | All non-secret PM config: `pm_org`, `pm_username`, `pm_api_url` |

`workspaces.yaml` is **always required** — the single-repo fallback mode that read these values from env vars has been removed. Each workspace entry carries:
- `pm_org` — Azure org name (Azure) or owner/org (GitHub/GitLab)
- `pm_username` — assignee filter: GitHub login / GitLab username / Azure email
- `pm_api_url` — optional self-hosted URL override (GitHub Enterprise, self-hosted GitLab, etc.)

All connector `NewClient()` constructors (`pm.NewGitHubClient(ws)`, `pm.NewGitLabClient(ws)`, `pm.NewAzureClient(ws)`) take an explicit workspace parameter and read token secrets from env — they never call `os.Getenv` for non-secret fields.

The LLM provider is selected by `LLM_PROVIDER` (`ollama` | `openai` | `anthropic`). Providers with available credentials are added as automatic fallbacks in `backend/llm/provider_factory.py`.

### Configuration Pattern: NO Defaults

All configuration functions require environment variables with **no fallback defaults**:

- Missing env var → clear error message specifying which var is missing
- Invalid value (e.g., negative timeout) → validation error with requirements
- This approach prevents silent failures from missing config

### Required Configuration Variables (12 Total)

All these variables **must** be set in `.env` or deployment will fail:

**Timeouts (4)**:

- `IPC_CONNECT_TIMEOUT_SECS` - IPC server connection timeout (seconds)
- `HTTP_TIMEOUT_SHORT` - Short HTTP operations timeout (seconds)
- `HTTP_TIMEOUT` - Standard HTTP operations timeout (seconds)
- `HTTP_TIMEOUT_LONG` - Long HTTP operations timeout (seconds)

**Hosts (2)**:

- `OLLAMA_HOST` - Ollama server URL (e.g., `http://localhost:11434`)
- `LMSTUDIO_HOST` - LMStudio server URL (e.g., `http://localhost:1234/v1`)

**Models (1)**:

- `GIT_SAGE_DEFAULT_MODEL` - Default LLM model for git-sage (e.g., `llama3`)

**Delays (1)**:

- `IPC_RETRY_DELAY_MS` - IPC reconnection retry delay (milliseconds)

**Prompt Timeouts (3)**:

- `PROMPT_TIMEOUT_SIMPLE_SECS` - Simple prompt timeout (seconds)
- `PROMPT_TIMEOUT_WORK_SECS` - Work update prompt timeout (seconds)
- `PROMPT_TIMEOUT_TASK_SECS` - Task prompt timeout (seconds)

**LLM (1)**:

- `LLM_REQUEST_TIMEOUT_SECS` - LLM API request timeout (seconds)

**Sentiment (1)**:

- `SENTIMENT_ANALYSIS_WINDOW_MINUTES` - Sentiment analysis window (minutes)

See [Configuration Reference](docs/CONFIGURATION.md) for complete list with examples.

### Configuration Functions (Go)

**config_env.go** provides typed accessors for all environment variables. All functions panic with clear error if var missing:

```go
// Main new timeout function
func GetIPCConnectTimeoutSecs() int  // Returns IPC_CONNECT_TIMEOUT_SECS

// All functions follow same pattern:
// - Panic if env var missing
// - Panic if value invalid (not integer, negative, etc.)
// - Return typed value ready to use
```

### Configuration Functions (Python)

**backend/config.py** provides 11+ new config functions. All require env var or raise ConfigError:

```python
# Timeouts
get_http_timeout_short() -> int    # HTTP_TIMEOUT_SHORT
get_http_timeout() -> int          # HTTP_TIMEOUT
get_http_timeout_long() -> int     # HTTP_TIMEOUT_LONG

# Hosts
get_ollama_host() -> str           # OLLAMA_HOST
get_lmstudio_host() -> str         # LMSTUDIO_HOST

# Models
get_git_sage_default_model() -> str  # GIT_SAGE_DEFAULT_MODEL

# Delays
get_ipc_retry_delay_ms() -> int    # IPC_RETRY_DELAY_MS

# Prompts
get_prompt_timeout_simple() -> int  # PROMPT_TIMEOUT_SIMPLE_SECS
get_prompt_timeout_work() -> int    # PROMPT_TIMEOUT_WORK_SECS
get_prompt_timeout_task() -> int    # PROMPT_TIMEOUT_TASK_SECS

# LLM
get_llm_request_timeout_secs() -> int  # LLM_REQUEST_TIMEOUT_SECS

# Sentiment
get_sentiment_analysis_window_minutes() -> int  # SENTIMENT_ANALYSIS_WINDOW_MINUTES
```

**Error Handling Pattern**:

```python
try:
    timeout = get_http_timeout_short()
except ConfigError as e:
    # e.message explains which var is missing
    # e.var_name is the env var name
    logger.error(f"Config error: {e.message}")
```

## Session Completion Status (Current)

**Last Updated**: May 31, 2026

**Phases Completed**:

- Phase 1: Enhanced Commit Messages ✅
- Phase 2: Conflict Resolution & PR-Aware Parsing ✅
- Phase 3: Event-Driven Integration ✅
- Phase 4: Project Management ✅
- Phase 4B: SQLite PM persistence ✅
- Personalization "Talk Like You" ✅
- RAG personalization (ChromaDB few-shot) ✅
- git-sage Session UX ✅
- CS-1: HTTP transport (Go → HTTPS POST → webhook_server.py) ✅
- CS-2: Config audit (os.getenv eliminated) + server-TUI stats panel ✅
- CS-3: Admin GUI MVP (users/licenses/health web UI on FastAPI) ✅
- Autostart (launchd/systemd env-first) ✅
- Anonymous telemetry ping ✅
- Jira alerter ✅
- Webhook server + alert poller ✅
- Logo integration: Windows binary icon (`devtrack.ico` via `goversioninfo`), website header images ✅
- Wiki folder consolidation: `devtrack_wiki/` is canonical; `wiki/` subfolder removed from monorepo ✅
- ARM64 cross-compilation fix: `.syso` renamed to `resource_windows_amd64.syso` ✅
- Boardroom feature: multi-persona AI plan review (`devtrack boardroom`, `devtrack plan`) 🔄 IN PROGRESS
- **Client-server decoupling Phase 1 (1a–1d)** ✅ — server-mgmt commands removed from client; reports/learning/auth/license commands converted to HTTP; workspaces.yaml is sole non-secret PM config source; `devtrack workspace add` offers git init; `devtrack help`/`status` fully rewritten.
- **Client-server decoupling Phase 2** ✅ — Go-native alerts (`internal/alerts/`), notifiers (`internal/notify/`), and interactive Telegram bot (`internal/telegram/`). Daemon no longer spawns Python telegram/alert subprocesses. Bot starts automatically with `TELEGRAM_ENABLED=true`.

**Production Readiness**: VERY HIGH

### CS-3 Admin GUI — Key Components

| Route | Description |
|---|---|
| `GET /admin/login` `POST /admin/login` | JWT cookie auth against env ADMIN_USERNAME/PASSWORD |
| `GET /admin/` | Dashboard: process health, LLM info, license tier, trigger stats |
| `GET /admin/users` | List users; inline role-change select; disable/enable/delete |
| `POST /admin/users/create` | Create user with username/password/role |
| `POST /admin/users/{u}/role` | Update role (viewer/admin) |
| `POST /admin/users/{u}/disable` / `/enable` | Soft-disable user (self blocked) |
| `POST /admin/users/{u}/reset-password` | Password reset (self requires current_password) |
| `GET /admin/users/{u}/keys` | API key listing |
| `POST /admin/users/{u}/keys/create` | Create API key; raw key shown once |
| `POST /admin/keys/{id}/revoke` | Revoke API key |
| `GET /admin/license` | License tier, T&C acceptance, seat check |
| `GET /admin/server` | LLM config, integration status, process control |
| `GET /admin/audit` | Audit log (last 200 events) |
| `GET /admin/_partials/processes` | HTMX fragment for process table |
| `GET /admin/_partials/stats` | HTMX fragment for trigger activity stats |

**ADMIN_EMBED**: Set `ADMIN_EMBED=true` to mount the admin UI directly on the main webhook
server (port 8089) instead of running a separate process on ADMIN_PORT.

## Phase Implementation Status

### Phase 1: Enhanced Commit Messages ✅

Commits include git context (branch, PR, recent commits) in AI prompts for better message generation.

- Modified: `backend/commit_message_enhancer.py` with `get_git_context()` method
- File: **GIT_SAGE_INTEGRATION_PHASE_1_2.md**

### Phase 2: Conflict Resolution & PR-Aware Parsing ✅

Automatic merge conflict resolution and git-aware work update parsing.

- New: `backend/conflict_auto_resolver.py` (ConflictAutoResolver class)
- New: `backend/work_update_enhancer.py` (WorkUpdateEnhancer class)
- Modified: `backend/nlp_parser.py` to accept repo_path and extract git context
- File: **GIT_SAGE_INTEGRATION_PHASE_1_2.md**

### Phase 3: Event-Driven Integration ✅

Automatic conflict detection and work context enrichment wired into the trigger pipeline.

- Conflict auto-resolution triggered after work updates
- Timer trigger injects git context (branch, PR, changes) into work updates before NLP parsing
- Commit trigger logs git metadata
- Phase 3 logic now lives in `backend/webhook_server.py` and associated handlers
- File: **PHASE_3_IMPLEMENTATION.md**

### Boardroom Feature (IN PROGRESS — `feature/boardroom`)

Multi-persona AI review of a plan or problem statement. Seven AI personas (architect, security, PM, devil's advocate, engineer, analyst, scalability) each produce PROs/CONs, then the session synthesises a SWOT matrix and final verdict. Optionally continues into an interactive chat.

**CLI commands:**

```bash
devtrack boardroom "<problem>"               # Inline problem review by all 7 personas
devtrack boardroom --file <plan.md>          # Review a Markdown plan file
devtrack boardroom --folder <plans/>         # Review all .md files in a folder
devtrack boardroom --file <plan.md> --output <report.md>   # Save report to file
devtrack boardroom --file <plan.md> --interactive          # Auto-enter chat after review
devtrack boardroom --file <plan.md> --interactive          # Chat session with personas

devtrack plan "<problem>"                    # Decompose problem → Epic/Story/Task + create on PM platform
devtrack plan --file <plan.md>               # Load problem from Markdown file
devtrack plan --folder <plans/>              # Process all .md files in folder
```

**New files:**

- `backend/boardroom/personas.py` — 7 persona definitions and LLM prompt templates
- `backend/boardroom/session.py` — orchestrates multi-persona review; calls LLM for each persona in turn
- `backend/boardroom/interactive.py` — post-review chat loop; routes follow-up questions to relevant personas
- `backend/boardroom/report.py` — renders SWOT matrix and verdict to terminal or Markdown file
- `backend/plan_parser.py` — parses free-text or Markdown into structured plan sections
- `devtrack-bin/cli_boardroom.go` — Go CLI for `devtrack boardroom`; calls `/boardroom` webhook endpoint
- `devtrack-bin/cli_plan.go` — Go CLI for `devtrack plan`; calls existing `/plan` webhook endpoint

### Windows Binary Build Notes

**`goversioninfo` and `.syso` files:**

```bash
cd devtrack-bin
go generate   # runs: goversioninfo -64 -o resource_windows_amd64.syso versioninfo.json
```

`main.go` carries the directive:

```go
//go:generate goversioninfo -64 -o resource_windows_amd64.syso versioninfo.json
```

**`.syso` filename constraint pattern:** Go links every `.syso` file found in a package for *all* build targets unless the filename contains a build-constraint suffix. Name syso files as `name_GOOS_GOARCH.syso` to constrain linkage.

- `resource_windows_amd64.syso` → only linked when `GOOS=windows GOARCH=amd64`
- A plain `resource.syso` would be linked for all targets including Linux/ARM64, causing `unknown ARM64 relocation type 3` cross-compilation failures

## Documentation Organization

All user-facing documentation has been reorganized for clarity:

### Quick Navigation

- **[📖 Complete Documentation Index](docs/INDEX.md)** — Master index of all documentation
- **[Getting Started](docs/GETTING_STARTED.md)** — New user introduction and concepts
- **[Installation Guide](docs/INSTALLATION.md)** — Step-by-step setup for all platforms
- **[Quick Start Guide](docs/QUICK_START.md)** — Get running in 15 minutes

### Using DevTrack

- **[Architecture Overview](docs/ARCHITECTURE.md)** — System design and component details
- **[Git Features Guide](docs/GIT_FEATURES.md)** — Enhanced commits, conflict resolution, work parsing
- **[LLM Configuration Guide](docs/LLM_GUIDE.md)** — AI provider setup and optimization
- **[Configuration Reference](docs/CONFIGURATION.md)** — All .env variables explained
- **[Troubleshooting Guide](docs/TROUBLESHOOTING.md)** — Common issues and solutions

### Advanced & Phase-Specific

- **[Roadmap & Phases](docs/PHASES.md)** — Current phase status and timeline
- **[Vision & Roadmap](VISION_AND_ROADMAP.md)** — Long-term strategic vision
- **[Hybrid LLM Strategy](HYBRID_LLM_STRATEGY.md)** — Multi-provider AI architecture

### Phase Implementation Details

- **[Phase Completion Summary](COMPLETION_SUMMARY.md)** — Overview of Phases 1-3
- **[Phase 1-2 Integration](GIT_SAGE_INTEGRATION_PHASE_1_2.md)** — Enhanced commits and conflict resolution
- **[Phase 3 Implementation](PHASE_3_IMPLEMENTATION.md)** — Event-driven integration
- **[Phase 3 Quick Start](PHASE_3_QUICK_START.md)** — Phase 3 quick reference
- **[GIT_COMMIT_WORKFLOW.md](GIT_COMMIT_WORKFLOW.md)** — Detailed git commit workflow guide

### Troubleshooting & Known Issues

- **[Known Issues](KNOWN_ISSUES.md)** — Known bugs and workarounds
- **[Phase 3 Verification](PHASE_3_VERIFICATION.md)** — Verify proper installation
- **[Local Setup Guide](LOCAL_SETUP.md)** — Development setup details
- **[Usage Guide](USAGE_GUIDE.md)** — Feature usage documentation

## Key Patterns

- **Client entry point**: `devtrack_client/main.go` is the Go binary entry point. It routes CLI args; the `git` subcommand is handled Go-natively by `gitsage.RunGit` (`devtrack_client/gitsage/commit.go`) — AI-enhanced commit, `add`, `history`, and transparent pass-through for all other git commands, with no dependency on `devtrack-git-wrapper.sh` or the Python backend. The legacy `devtrack-bin/main.go` is a mirror being retired in TASK-048.
- **Server entry point**: `devtrack_server/backend/webhook_server.py` is the Python process the Go client spawns (in managed mode) or connects to (in external mode). It handles both inbound webhook events (Azure/GitHub/GitLab/Jira) and trigger calls from the Go client (`/trigger/commit`, `/trigger/timer`, etc.). `python_bridge.py` at the monorepo root is a legacy reference only — do not use it for new code.
- **HTTP API boundary**: Client and server share no compiled artefact. The only interface is HTTPS POST to `/trigger/*` and related endpoints. See `docs/HTTP_API.md` for the full contract. The legacy TCP IPC channel (`127.0.0.1:35893`) is retained but new trigger types must use HTTP.
- **git-sage ownership**: git-sage is client-owned Python, bundled at `devtrack_client/git_sage/`. It is NOT in `devtrack_server/`. Invoke as `python -m backend.git_sage` from the client's Python environment.
- **git-sage architecture**: Modular design with GitOperations, ConflictResolver, PRFinder as reusable components. Agent uses these as helpers for autonomous git operations. Can be used standalone via CLI or as Python library.
- **Trigger pipeline**: webhook_server.py trigger handlers integrate git-sage features: timer triggers enhance work context, commit triggers log git metadata, post-update checks detect and resolve conflicts. All features degrade gracefully if git-sage unavailable.
- **IPC message protocol**: JSON-newline-delimited over TCP. Message types are defined in both `devtrack_client/ipc.go` (`MessageType` constants) and `devtrack_server/backend/ipc_client.py` (`MessageType` enum) — legacy internal channel; new trigger types should use the HTTP `/trigger/*` endpoints instead.
- **Python optional imports**: All Python subsystems (NLP, TUI, LLM, report generator, git_sage, work_enhancer, conflict_resolver) are imported with `try/except` and individually gated; the webhook server degrades gracefully if a dependency is missing.
- **Config centralization**: All Python modules access config via `backend.config.get()`, `get_int()`, `get_bool()`, `get_path()` — never `os.getenv()` directly.
- **Database access**: Centralized via `devtrack_server/backend/db/` models; no direct SQLite queries in business logic.
- **Commit message enhancement**: The `devtrack git commit` workflow is stateful (caches attempt count, original message, refined versions) across up to 5 iterations before creating a commit.
- **Work update enrichment**: Timer trigger enhancements (Phase 3) inject git context (branch, PR, changes) into work updates before NLP parsing for better task extraction and auto PR-number detection.
- **Conflict auto-resolution**: Post-update hook (Phase 3) automatically detects and resolves merge conflicts using smart strategies, reports status to user via TUI or logs.
- **Git-sage agent mode**: The `git-sage` tool runs autonomously: it plans operations, executes them, reads output, handles failures with rollback, and only asks for input on genuine ambiguities.
- **git-sage session UX**: `do` mode shows an approval dialog (auto/review/suggest-only) before the first command. After the task completes, up to 5 follow-up questions can be asked in the same conversation context. Command history and `undo [N]` are available at any point.
- **git-sage squash**: Agent always uses `git reset --soft HEAD~N && git commit -m "..."` — never `git rebase -i` (interactive editor blocks the agent loop).
- **git-sage LLM JSON mode**: `raw_chat(..., json_mode=True)` is set on every agent call. Ollama uses `format:"json"`, OpenAI-compatible uses `response_format:{"type":"json_object"}` with a `BadRequestError` fallback. Model names strip `provider/` prefixes (LiteLLM convention) before the API call.
- **Groq provider**: `devtrack_server/backend/llm/groq_provider.py` + added to `provider_factory.py` fallback chain. git-sage uses `GIT_SAGE_PROVIDER=groq` with `GROQ_API_KEY` / `GROQ_HOST` / `GROQ_MODEL` env vars.

## Personalization System

### Architecture

Teams chats are collected via MS Graph API and stored in MongoDB. The system learns your communication style and generates personalized response suggestions. Every LLM prompt in the system is augmented with two personalization signals:

1. **Profile-based style instruction** — fast, always available. Captures constraints: formality, length, emoji preference, common phrases. Produced by `PersonalizedAI.get_style_instruction()`.

2. **RAG few-shot examples** — retrieves the most semantically similar past responses the user wrote, using ChromaDB + `nomic-embed-text`. Shows the LLM *actual* examples of how the user writes. Much more effective at capturing voice nuance than abstract style descriptions.

Both are injected globally via `backend/personalization.py`:`inject_style(prompt, context_type, query_text)`. If no profile exists the prompt is returned unchanged — fully graceful.

**RAG setup** (one-time):
```bash
ollama pull nomic-embed-text    # Pull the embedding model
# ChromaDB is installed automatically via: uv sync
# Data stored at: DATA_DIR/learning/chroma/
```

**RAG sub-modules** (`backend/rag/`):

| Module | Purpose |
|---|---|
| `embedder.py` | Ollama `/api/embed` calls; returns `None` if model unavailable |
| `vector_store.py` | ChromaDB persistent collection wrapper; cosine similarity |
| `sample_indexer.py` | High-level API: `index_sample()`, `index_samples()`, `retrieve_examples()` |

**Injection points** (all use `inject_style()` from `backend/personalization.py`):

| File | context_type | RAG query |
|---|---|---|
| `commit_message_enhancer.py` | `commit` | original commit message or plain change summary |
| `description_enhancer.py` | `description` | raw user input |
| `git_sage/agent.py` | `commit` | appended to system prompt |
| `daily_report_generator.py` | `report` | first 200 chars of prompt |
| `ai/create_tasks.py` | `task` | first 200 chars of prompt |
| `project_manager.py` | `task`/`comment` | first 200 chars of prompt |

### CLI Commands

```bash
devtrack enable-learning          # Consent + initial data collection
devtrack learning-sync            # Delta sync (only new messages since last run)
devtrack learning-sync --full     # Force full 30-day re-collection
devtrack learning-setup-cron      # Install daily cron (uses LEARNING_CRON_SCHEDULE)
devtrack learning-remove-cron     # Remove cron entry
devtrack learning-cron-status     # Show cron status
devtrack learning-reset           # Wipe all data (MongoDB + files) and start fresh
devtrack show-profile             # Display learned communication profile
devtrack test-response <text>     # Generate a personalized response (no auth needed)
devtrack learning-status          # Show consent/sample count status
```

### Key Implementation Details

**User identification**: Teams messages do NOT contain `userPrincipalName` in `additional_data` (only `tenantId`). User matching is done by **Azure AD object ID** stored in `consent.json` as `user_object_id`. This is fetched from `graph.get_user()` which must include `'id'` in `$select`.

**`AsyncTeamsDataCollector._is_user_message()`**: Overrides base class to match by `user_object_id` first, falls back to UPN. The base class in `data_collectors.py` uses UPN only.

**Delta sync**: `learning_state` MongoDB collection tracks `last_collected` per user email. If `learning-sync` runs repeatedly with 0 samples, the window shrinks. Fix: `learning-sync --full`.

**MongoDB mode**: When `MONGODB_URI` is set and `motor` is installed, `PersonalizedAI._mongo_mode=True` suppresses file writes. Samples deduplicated by Teams message GUID using `$setOnInsert`.

**`test-response` / `show-profile` / `revoke-consent`**: Skip MS Graph auth entirely — they only need the local profile from MongoDB/files.

**sys.path**: `learning_integration.py` adds repo root to `sys.path` so `backend.llm` imports work when the script runs standalone.

### MongoDB Collections

| Collection | `_id` | Purpose |
|---|---|---|
| `communication_samples` | Teams message GUID | Trigger→response pairs |
| `user_profiles` | user email | Computed style profile |
| `learning_state` | user email | Delta sync timestamp |

### Consent File

`Data/learning/consent.json` stores:
- `user_email` — used as fallback when Graph auth fails
- `user_object_id` — Azure AD ID for message matching (saved on first successful `get_user()`)

### Infrastructure

```bash
docker compose up -d    # Start MongoDB, Redis, PostgreSQL
docker compose down     # Stop services
```

Cron runs at `LEARNING_CRON_SCHEDULE` (default `0 20 * * *`) via `backend/run_daily_learning.py`.

---

## Common Debugging Patterns

**AI enhancement failing silently in `devtrack git commit`:**

- Check if Ollama is running (`ollama serve`)
- The wrapper checks stdout for the word "enhanced", but Python logging goes to stderr
- If enhancement fails, the wrapper falls back silently to the original message
- See [KNOWN_ISSUES.md](KNOWN_ISSUES.md#ai-enhancement-intermittent-failure) for detailed debugging

**IPC connection errors:**

- Verify `IPC_HOST` and `IPC_PORT` are exported in the environment before starting
- Check for stale processes: `lsof -i :35893`
- Firewall may block localhost ports on some systems
- After changing `.env`, re-source it and restart the daemon (`source .env && devtrack start`)

**Git monitor not detecting commits:**

- Ensure daemon is running in the correct repository (`devtrack status`)
- Verify `DEVTRACK_WORKSPACE` in `.env` points to the monitored repo
- Check logs: `tail -f Data/logs/daemon.log | grep -i "git\|commit"`

**spaCy NLP model missing:**

- Run: `uv run python -m spacy download en_core_web_sm`
- Verify with: `uv run python -c "import spacy; spacy.load('en_core_web_sm')"`

**Tests failing with "provider not found" errors:**

- Call `reset_provider_cache()` in test setup/teardown when changing `LLM_PROVIDER`
- This prevents LLM provider state leaking between tests

**git-sage agent failing to resolve conflicts:**

- Check if conflict markers are valid (<<<<<<< ======= >>>>>>>)
- Try explicit strategy: `ConflictResolver(strategy="both")` instead of "smart"
- Use `ConflictAnalyzer` to inspect conflicts before resolution
- If still unresolvable, agent will report which conflicts need manual intervention

**git-sage LLM not responding:**

- Verify Ollama is running: `curl http://localhost:11434/api/tags`
- Check config: `git-sage --show-config`
- Test with simple ask: `git-sage ask "hello"`
- Increase timeout in llm.py if network is slow

**git-sage agent loops infinitely:**

- Set `max_steps` parameter lower (default 30)
- Use `--verbose` flag to see what agent is doing
- Check LLM responses are valid JSON
- Interrupt with Ctrl+C and check git status

**git-sage parse error / agent does nothing then says Done:**

- The LLM is returning prose instead of JSON. Check the `Raw:` snippet printed after `[parse error]`.
- If using Groq: verify `GROQ_MODEL` uses the native model name (e.g. `compound-beta`, `llama-3.3-70b-versatile`) — the `groq/` prefix is stripped automatically but the base name must be valid.
- `compound-beta` ignores `response_format` — it eventually obeys text-level instructions. Switch to `llama-3.3-70b-versatile` for more reliable JSON compliance.
- JSON mode is enforced via `response_format={"type":"json_object"}` for OpenAI-compatible providers and `"format":"json"` for Ollama. If a model raises `BadRequestError` for `response_format`, it falls back to text-only.

**git-sage Groq 403 Cloudflare block:**

- Caused by `urllib` User-Agent being blocked. All non-Ollama providers now use the `openai` SDK which sets a proper User-Agent. If you still see 403, run `uv add openai` to ensure the package is installed.

**Phase 3: Work context not enriching work updates:**

- Verify `work_enhancer_available` is True (check logs at startup)
- Ensure repo_path is correct (default: "." in webhook_server.py)
- Check git repo is valid and on a feature branch
- Review logs: `tail -f Data/logs/daemon.log | grep "context"`

## Ticket Alerter

### Overview

A background polling service that watches GitHub, Azure DevOps, and Jira for ticket events relevant to the developer, delivering OS/terminal notifications and persisting them to MongoDB.

### Events by Source

| Source | Events | Status |
|---|---|---|
| GitHub | Issue/PR assigned, review requested, comment on involved issue | ✅ Shipped |
| Azure DevOps | Work item assigned, comment added, state changed | ✅ Shipped |
| Jira | Assigned to me, comment added, status changed | ✅ Shipped |

### Notification Delivery

- **macOS OS notification**: `osascript -e 'display notification ...'`
- **Terminal**: formatted output when devtrack is in foreground
- **Configurable per-source**: opt in/out of each integration and event type via `.env`

### MongoDB Schema

```
notifications collection:
  _id: ObjectId
  source: "github" | "azure"
  event_type: "assigned" | "comment" | "status_change" | "review_requested"
  ticket_id: "org#123" | "owner/repo#456"
  title: "Fix login bug"
  summary: "John commented: ..."
  url: "https://..."
  timestamp: datetime
  read: false
  dismissed: false
  raw: { ...full API payload... }
```

### CLI Commands

```bash
devtrack alerts                   # Show unread notifications (last 24h)
devtrack alerts --all             # Show all notifications
devtrack alerts --clear           # Mark all as read
```

### Architecture

```
Poller (Python, async)
  ├── AzureAlerter    — Azure DevOps REST API (work item updates + comments APIs)
  └── GitHubAlerter   — GitHub REST API
          │
          ▼
  MongoDB notifications collection
          │
          ▼
  Notifier
    ├── macOS: osascript
    └── Terminal: print to stdout if TTY attached
```

**Polling**: `backend/alert_poller.py` runs as a subprocess launched by the Go daemon. Each source tracks `last_checked` per user in the MongoDB `alert_state` collection.

**Azure alerter detail** (`backend/alerters/azure_alerter.py`):
- Uses `AzureDevOpsClient.get_my_work_items(changed_after=last_checked)` for delta fetches
- Per-item updates API (`_apis/wit/workitems/{id}/updates`) classifies assigned vs state-change events
- Per-item comments API (`_apis/wit/workItems/{id}/comments?api-version=7.1-preview.3`) for new comments
- Skips own comments via `AZURE_EMAIL` / `EMAIL` env var (falls back to Azure profile API)
- First-run guard: assigned + state-change polls skip silently when `last_checked` is None

### Configuration

```
ALERT_ENABLED=true
ALERT_POLL_INTERVAL_SECS=300        # Poll every 5 minutes
ALERT_GITHUB_ENABLED=true
ALERT_AZURE_ENABLED=true
ALERT_NOTIFY_ASSIGNED=true
ALERT_NOTIFY_COMMENTS=true
ALERT_NOTIFY_STATUS_CHANGES=true
ALERT_NOTIFY_REVIEW_REQUESTED=true  # GitHub only
# Optional: filter own Azure comments
# AZURE_EMAIL=you@yourorg.com
```

### Implementation Files

```
backend/
  ├── alert_poller.py         — Main async poller; _poll_github() + _poll_azure()
  ├── alert_notifier.py       — macOS osascript + terminal notifications
  ├── alerters/
  │   ├── github_alerter.py   — GitHub assigned/comments/review-requests ✅
  │   ├── azure_alerter.py    — Azure DevOps assigned/comments/state-changes ✅
  │   └── jira_alerter.py     — Jira assigned/comments/status_change polling ✅
  └── db/mongo_alerts.py      — MongoAlertsStore: notifications + alert_state collections
```

---

## Hardcoding Refactoring Summary

### What Changed

All 22 hardcoded values were refactored from source code to required environment variables:

**Values Eliminated**:

- IPC connection timeout (was hardcoded to 5 seconds)
- HTTP request timeouts (was hardcoded to 10/30/60 seconds)
- IPC retry delay (was hardcoded to 2000ms)
- Ollama and LMStudio hosts and default model
- Prompt timeouts for simple/work/task interactions
- LLM request timeout
- Sentiment analysis window

### Why This Matters

**Explicit Configuration**: Deployments must explicitly set all timeouts/hosts. No hidden defaults mean:

- Config errors caught immediately with clear messages
- No surprises from unset variables
- Easy to tune for different environments
- Production safety: missing config → immediate clear error

**Files Modified** (22 total files, 35+ locations):

- Go: `config_env.go`, `ipc.go`, `daemon.go`, `integrated.go`, `cli.go`
- Python: `backend/config.py`, `python_bridge.py`, `user_prompt.py`, `ipc_client.py`
- Git-sage: `git_sage/llm.py`, `git_sage/context.py`, `git_sage/conflict_resolver.py`
- Other: `backend/nlp_parser.py`, `backend/task_matcher.py`, multiple test files

**Git Commits** (clean history showing progression):

- Commit 1: Extract timeout vars (IPC, HTTP)
- Commit 2: Extract host/model vars (Ollama, LMStudio)
- Commit 3: Extract prompt timeout vars
- Commit 4: Update all usages
- Commit 5: Add validation and error handling
- (40+ total commits in this session)

### Breaking Changes

**For Existing Deployments**:

1. All 12 variables **must** be set in `.env`
2. Missing any variable → daemon fails at startup with clear error
3. Upgrade path: Copy `.env_sample` and fill in values

**Error Messages Guide Users**:

```
ERROR: Configuration missing IPC_CONNECT_TIMEOUT_SECS
This variable is required for daemon startup.
Set it in .env file: IPC_CONNECT_TIMEOUT_SECS=5
See docs/CONFIGURATION.md for details.
```

### How to Deploy with New Config

```bash
# 1. Copy sample
cp .env_sample .env

# 2. Edit with your values
nano .env

# 3. Verify all 12 required vars set
grep -E "IPC_CONNECT_TIMEOUT_SECS|HTTP_TIMEOUT_SHORT|HTTP_TIMEOUT|HTTP_TIMEOUT_LONG|IPC_RETRY_DELAY_MS|OLLAMA_HOST|LMSTUDIO_HOST|GIT_SAGE_DEFAULT_MODEL|PROMPT_TIMEOUT_SIMPLE_SECS|PROMPT_TIMEOUT_WORK_SECS|PROMPT_TIMEOUT_TASK_SECS|LLM_REQUEST_TIMEOUT_SECS" .env

# 4. Start daemon (will error if missing any)
devtrack start
```

**Phase 3: Conflicts not auto-resolving:**

- Check `conflict_resolver_available` is True (check logs at startup)
- Some conflicts require manual judgment — this is expected and safe
- Review `unresolvable` list in conflict report
- Run `get_conflict_report()` to see detailed conflict analysis
- Check git-sage modules are properly imported

**Phase 3: Git context not extracted in commits:**

- Verify NLP parser called with `repo_path` parameter
- Ensure on valid git branch (`git branch -a`)
- Check GitOperations and PRFinder initialization in commit_message_enhancer.py
- Look for exceptions in logs tagged "git context"
