# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**DevTrack** is a developer automation tool that monitors Git activity and scheduled timers, prompting developers for work updates and routing them through an AI pipeline to update project management systems and generate reports.

## Build & Run Commands

### Go daemon (devtrack-bin/)

```bash
cd devtrack-bin
go build -o devtrack .          # Build binary
go test ./...                   # Run all Go tests
go vet ./...                    # Run Go linter
```

### Python backend

```bash
uv sync                         # Install/sync dependencies (uv manages the venv)
uv run pytest backend/tests/    # Run all Python tests (uses conftest.py setup)
uv run pytest backend/tests/test_nlp_parser.py  # Run single test file
uv run pytest backend/tests/ -k test_name       # Run tests by name filter
uv run python validate_env_sample.py            # Validate .env keys match .env_sample
```

### Testing Patterns

- **Test structure**: `backend/tests/` uses pytest with `conftest.py` that adds repo root to `sys.path`
- **LLM provider isolation**: Tests that change `LLM_PROVIDER` must call `reset_provider_cache()` before/after to avoid cross-test contamination
- **Optional imports**: Python subsystems (NLP, TUI, LLM, report generator) degrade gracefully if dependencies are missing

### Running the daemon locally

```bash
export PROJECT_ROOT="/path/to/automation_tools"
export DEVTRACK_ENV_FILE="$PROJECT_ROOT/.env"
devtrack start &                # Start background daemon
devtrack status                 # Verify it's running
devtrack logs                   # View recent log output
```

## Architecture

The system has two main runtime components communicating over a TCP socket (default `127.0.0.1:35893`, configurable via `IPC_HOST`/`IPC_PORT` in `.env`):

```
Git commits / cron timer
        │
        ▼
┌──────────────────┐     TCP IPC (JSON)     ┌──────────────────────────┐
│  Go Daemon       │ ──────────────────────▶ │  Python Bridge           │
│  devtrack-bin/   │                         │  python_bridge.py        │
│  - git_monitor   │                         │  - NLP parsing (spaCy)   │
│  - scheduler     │                         │  - LLM enhancement       │
│  - ipc (server)  │ ◀────────────────────── │  - TUI user prompts      │
│  - database      │    task_update / ack    │  - Report generation     │
│  - cli           │                         │  - Project mgmt APIs     │
└──────────────────┘                         └──────────────────────────┘
        │                                              │
        ▼                                              ▼
  SQLite (Data/db/)                        Azure DevOps / GitHub / Jira
  PID/logs (Data/)                         Microsoft Graph (Teams/Email)
```

### Go layer (`devtrack-bin/`)

| File | Purpose |
|---|---|
| `main.go` | Entry point; routes CLI args or delegates `git` subcommand to shell wrapper |
| `cli.go` | All CLI command implementations (`start`, `stop`, `status`, `logs`, etc.) |
| `daemon.go` | Lifecycle management (PID file, signals, Python bridge process) |
| `integrated.go` | `IntegratedMonitor` — wires together git monitor, scheduler, and IPC server |
| `git_monitor.go` | fsnotify-based Git repository watcher; fires `commit_trigger` on new commits |
| `scheduler.go` | Cron-based periodic trigger using robfig/cron; fires `timer_trigger` |
| `ipc.go` | TCP IPC server (Go side); JSON-delimited messages, one handler per `MessageType` |
| `database.go` | SQLite via modernc.org/sqlite; stores trigger history and task updates |
| `config.go` | YAML config struct (`Data/configs/config.yaml`); all runtime values via `config_env.go` |
| `config_env.go` | All `.env` key accessors for Go — the single source of truth for env var names |
| `learning.go` | Personalized AI learning consent and profile management |

### Python layer (`backend/` + `python_bridge.py`)

#### Core Infrastructure

| Module | Purpose |
|---|---|
| `python_bridge.py` | Entry point started by Go daemon; connects to IPC server and dispatches triggers |
| `backend/config.py` | Centralized config — all modules use `backend.config.get()`, not `os.getenv()` directly |
| `backend/ipc_client.py` | TCP IPC client (Python side); mirrors message types from Go's `ipc.go` |

#### NLP & AI Processing

| Module | Purpose |
|---|---|
| `backend/nlp_parser.py` | spaCy-based NLP for commit/user text → structured task data (entity extraction, action detection) |
| `backend/description_enhancer.py` | Ollama-based description enhancement and categorization |
| `backend/llm/` | Multi-provider LLM abstraction (`provider_factory.py` builds fallback chain: primary → OpenAI/Anthropic → Ollama) |
| `backend/personalized_ai.py` | AI learning from user communications for personalized responses |
| `backend/learning_integration.py` | Learning consent management and profile handling |

#### User Interaction & Reporting

| Module | Purpose |
|---|---|
| `backend/user_prompt.py` | Terminal TUI for interactive work-update prompts |
| `backend/daily_report_generator.py` | AI-enhanced daily/weekly report generation (multiple output formats: Terminal, HTML, Markdown, JSON) |
| `backend/email_reporter.py` | Report delivery via email/Teams |
| `backend/task_matcher.py` | Fuzzy + semantic matching of natural language to tracked tasks |

#### Git Integration

| Module | Purpose |
|---|---|
| `backend/commit_message_enhancer.py` | AI-powered iterative commit message refinement (multi-attempt workflow) |
| `backend/git_diff_analyzer.py` | Analyze staged changes to enhance context for commit messages |
| `backend/git_sage/` | **Local LLM-powered git agent** with complete implementation |

##### git-sage Sub-modules

| Module | Purpose |
|---|---|
| `cli.py` | Ask/do/interactive modes for git assistance |
| `agent.py` | Agentic loop: plan → execute → verify → recover (300+ lines) |
| `llm.py` | Ollama and OpenAI-compatible LLM backends |
| `context.py` | Git repository state collection and formatting |
| `config.py` | Configuration management (~/.config/git-sage/config.json) |
| `git_operations.py` | Advanced git operations: branches, commits, merges, status, blame, stash (300+ lines) |
| `conflict_resolver.py` | Intelligent conflict analysis and resolution with multiple strategies (280+ lines) |
| `pr_finder.py` | PR/MR utilities: metadata extraction, branch analysis, diff statistics (220+ lines) |

#### External Integrations

| Module | Purpose |
|---|---|
| `backend/jira/` | Jira REST API client for issue management |
| `backend/github/` | GitHub API integration (PR analysis, repository insights) |
| `backend/azure/` | Azure DevOps work item fetching and updating |
| `backend/msgraph_python/` | Microsoft Graph integration (Teams chat, Outlook email, sentiment analysis) |

#### Utilities & Helpers

| Module | Purpose |
|---|---|
| `backend/utils/` | Shared utilities (formatting, validation, helpers) |
| `backend/autodoc/` | Auto-documentation generation from code |
| `backend/db/` | Database models and migrations |
| `backend/ai/` | Low-level AI utilities (Ollama client, inference helpers) |

## Configuration Architecture

All configuration flows from a single `.env` file. There are **no hardcoded fallback values** for paths or credentials.

- Go reads env vars via `config_env.go` (loaded with `joho/godotenv`)
- Python reads via `backend/config.py` functions (`get()`, `get_int()`, `get_bool()`, `get_path()`)
- `DEVTRACK_ENV_FILE` overrides `.env` location; if unset, Go requires `.env` in the working directory
- Runtime data lives under `Data/` (db, logs, reports, pids) — paths configurable via `DATA_DIR`, `DATABASE_DIR`, etc.

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

**Last Updated**: March 12, 2026

**Phases Completed**:

- Phase 1: Enhanced Commit Messages ✅
- Phase 2: Conflict Resolution & PR-Aware Parsing ✅
- Phase 3: Event-Driven Integration ✅
- Phase 4: Project Management ✅
- Personalization "Talk Like You" ✅ FULLY WORKING

**Major Accomplishments (March 12, 2026 session)**:

- Personalization pipeline working end-to-end: Teams → MongoDB → Profile → Personalized responses
- Fixed user identification: Teams messages use Azure AD object ID (not UPN) for matching
- Fixed `graph.py` `get_user()` to include `id` in `$select`
- Fixed delta state: stale `last_collected` blocks collection; use `learning-sync --full` to reset
- Added MongoDB storage for learning data (motor async driver)
- Docker Compose with mongo, redis, postgres
- 10 new CLI commands for learning management
- `test-response` no longer triggers Graph auth
- Daily cron at 20:00 for automatic delta sync

**Production Readiness**: VERY HIGH

**Planned Next**:
- launchd plist for macOS auto-start on login
- Multi-repo monitoring with per-repo project management config

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

python_bridge.py integration with automatic conflict detection and work context enrichment.

- Modified: `python_bridge.py` with Phase 3 imports and handler enhancements
- New: `_check_and_resolve_conflicts()` method for automatic conflict resolution
- Enhanced: `handle_timer_trigger()` with work context injection
- Enhanced: `handle_commit_trigger()` with git context logging
- File: **PHASE_3_IMPLEMENTATION.md**

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

- **git-sage architecture**: Modular design with GitOperations, ConflictResolver, PRFinder as reusable components. Agent uses these as helpers for autonomous git operations. Can be used standalone via CLI or as Python library.
- **Phase 3 integration**: python_bridge.py event handlers now integrate git-sage features: timer triggers enhance work context, commit triggers log git metadata, post-update checks detect and resolve conflicts. All features degrade gracefully if git-sage unavailable.
- **IPC message protocol**: JSON-newline-delimited over TCP. Message types are defined in both `devtrack-bin/ipc.go` (`MessageType` constants) and `backend/ipc_client.py` (`MessageType` enum) — **keep in sync when adding new trigger types**.
- **Python optional imports**: All Python subsystems (NLP, TUI, LLM, report generator, git_sage, work_enhancer, conflict_resolver) are imported with `try/except` and individually gated; the bridge degrades gracefully if a dependency is missing.
- **Config centralization**: All Python modules access config via `backend.config.get()`, `get_int()`, `get_bool()`, `get_path()` — never `os.getenv()` directly.
- **Database access**: Centralized via `backend/db/` models; no direct SQLite queries in business logic.
- **Commit message enhancement**: The `devtrack git commit` workflow is stateful (caches attempt count, original message, refined versions) across up to 5 iterations before creating a commit.
- **Work update enrichment**: Timer trigger enhancements (Phase 3) inject git context (branch, PR, changes) into work updates before NLP parsing for better task extraction and auto PR-number detection.
- **Conflict auto-resolution**: Post-update hook (Phase 3) automatically detects and resolves merge conflicts using smart strategies, reports status to user via TUI or logs.
- **Git-sage agent mode**: The `git-sage` tool runs autonomously: it plans operations, executes them, reads output, handles failures with rollback, and only asks for input on genuine ambiguities.

## Personalization System

### Architecture

Teams chats are collected via MS Graph API and stored in MongoDB. The system learns your communication style and generates personalized response suggestions.

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

- Verify `.env` has correct `IPC_HOST` and `IPC_PORT`
- Check for stale processes: `lsof -i :35893`
- Firewall may block localhost ports on some systems
- Restart daemon after `.env` changes

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

**Phase 3: Work context not enriching work updates:**

- Verify `work_enhancer_available` is True (check logs at startup)
- Ensure repo_path is correct (default: "." in python_bridge.py)
- Check git repo is valid and on a feature branch
- Review logs: `tail -f Data/logs/python_bridge.log | grep "context"`

## Ticket Alerter (Planned Feature)

### Overview

A background polling service that watches Jira, Azure DevOps, and GitHub for ticket events relevant to the developer, delivering OS/terminal notifications and persisting them to MongoDB.

### Events to Watch

| Source | Events |
|---|---|
| Jira | Assigned to me, comment added, status changed, priority changed |
| Azure DevOps | Work item assigned, comment added, state changed |
| GitHub | Issue/PR assigned, review requested, comment on my PR |

### Notification Delivery

- **macOS OS notification**: `osascript -e 'display notification ...'` or `terminal-notifier` (richer, with actions)
- **Terminal**: Bell + formatted output when devtrack is in foreground
- **Configurable per-source**: opt in/out of each integration and event type via `.env`

### MongoDB Schema

```
notifications collection:
  _id: ObjectId
  source: "jira" | "azure" | "github"
  event_type: "assigned" | "comment" | "status_change" | "review_requested"
  ticket_id: "PROJ-123"
  title: "Fix login bug"
  summary: "John commented: ..."
  url: "https://..."
  timestamp: datetime
  read: false
  dismissed: false
  raw: { ...full API payload... }
```

### CLI Commands (Planned)

```bash
devtrack alerts                   # Show unread notifications (last 24h)
devtrack alerts --all             # Show all notifications
devtrack alerts --clear           # Mark all as read
devtrack alerts --pause           # Pause polling
devtrack alerts --resume          # Resume polling
```

### Architecture

```
Poller (Python, async)
  ├── JiraPoller      — REST API, polls every ALERT_POLL_INTERVAL_SECS
  ├── AzurePoller     — Azure DevOps REST API
  └── GitHubPoller    — GitHub REST API (or webhooks if hosted)
          │
          ▼
  MongoDB notifications collection
          │
          ▼
  Notifier
    ├── macOS: osascript / terminal-notifier
    └── Terminal: print to stdout if TTY attached
```

**Polling**: Run as part of the existing Go daemon — a new `alert_poller.go` that spawns `backend/alert_poller.py` via the IPC bridge, similar to how the scheduler works.

**State tracking**: Each integration stores `last_checked` timestamp (like learning delta) in MongoDB `alert_state` collection keyed by `source`.

### Configuration (`.env` keys to add)

```
ALERT_ENABLED=true
ALERT_POLL_INTERVAL_SECS=300        # Poll every 5 minutes
ALERT_JIRA_ENABLED=true
ALERT_AZURE_ENABLED=true
ALERT_GITHUB_ENABLED=true
ALERT_NOTIFY_ASSIGNED=true
ALERT_NOTIFY_COMMENTS=true
ALERT_NOTIFY_STATUS_CHANGES=true
```

### Implementation Files (To Create)

```
backend/
  ├── alert_poller.py         — Main async poller, coordinates all sources
  ├── alert_notifier.py       — OS + terminal notification delivery
  ├── alerters/
  │   ├── jira_alerter.py     — Jira polling logic
  │   ├── azure_alerter.py    — Azure DevOps polling logic
  │   └── github_alerter.py   — GitHub polling logic
  └── db/mongo_alerts.py      — MongoAlertsStore (notifications + alert_state)

devtrack-bin/
  ├── alert_poller.go         — Launches alert_poller.py, manages lifecycle
  └── cli_alerts.go           — `devtrack alerts` CLI commands
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
