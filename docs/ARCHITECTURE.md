# DevTrack System Architecture

Complete overview of DevTrack's system design, components, and data flow.

> **Monorepo split (EPIC-SPLIT) — complete.** Canonical sources are `devtrack_client/` (Go) and `devtrack_server/` (Python); the legacy `devtrack-bin/` and root `backend/` directories were retired in TASK-048. See `docs/split-manifest.md` for the ownership catalogue. This document is the canonical client↔server boundary reference.
>
> **Client-server decoupling Phase 1 + Phase 2 — complete.** Server-management commands removed from the client; reports/learning/auth/license call the server over HTTP; `workspaces.yaml` is the sole non-secret PM config source; alerts, notifiers, and the Telegram bot are Go-native. See `docs/CLIENT_SERVER_DECOUPLING_PLAN.md`.

---

## Three-Codebase Structure

| Directory | Language | Role |
|---|---|---|
| `devtrack_client/` | Go + Python (git-sage) | Binary, git monitor, scheduler, CLI — canonical Go source |
| `devtrack_server/` | Python | AI pipeline, webhook server, admin UI — canonical Python source |
| `devtrack_wiki/` | HTML/Markdown | Website (Netlify → devtrack.cloud) |

Client and server share no compiled artefact. The only interface between them is HTTPS POST over a documented JSON API (the `/trigger/*` endpoints, described below).

---

## Client-Server Architecture

DevTrack is explicitly a **client-server tool** with two independently deployable components:

| Component | Source | Technology | Size | Role |
|---|---|---|---|---|
| **`devtrack` binary** | `devtrack_client/` | Pure Go | ~5 MB | Client / daemon — git monitoring, scheduling, CLI |
| **Python server** | `devtrack_server/` | Python + uv | separate | Server — AI, NLP, integrations, reports, admin |

The Go binary contains **no embedded Python** (except the bundled git-sage tool at `devtrack_client/git_sage/`). The Python server is set up separately and can run as a local subprocess, a Docker container, or a remote server.

### Server Modes

| Mode | Config | Use case |
|---|---|---|
| **managed** (default) | `DEVTRACK_SERVER_MODE=managed` | Local dev — daemon spawns the Python backend as a subprocess |
| **external** | `DEVTRACK_SERVER_MODE=external` + `DEVTRACK_SERVER_URL=http://...` | Docker container or cloud-hosted Python server |

```bash
# managed mode (default — no extra config needed)
devtrack start         # daemon starts Python bridge automatically

# external mode — Python server runs separately
DEVTRACK_SERVER_MODE=external
DEVTRACK_SERVER_URL=http://localhost:8089
devtrack start         # daemon connects to the external server
```

### Docker Option

A `Dockerfile.server` ships with the repo to containerize the Python backend:

```bash
# Build and run the Python backend container
docker build -f Dockerfile.server -t devtrack-server .
docker run -p 8089:8089 --env-file .env devtrack-server

# Or use docker compose (starts Python backend + MongoDB + Redis)
docker compose up
```

The Go binary on the host then connects via `DEVTRACK_SERVER_MODE=external`.

### `devtrack install`

Running `devtrack install` prints setup instructions for the client-server architecture — it does **not** extract or bundle Python. Use the printed instructions to set up the Python backend in whichever mode suits your environment.

### Binary Releases

The Go binary (~5 MB, no Python) is published to **GitHub Releases** (`github.com/sraj0501/Devtrack_`) on every tagged release via the automated pipeline (`release.yml`). `devtrack upgrade` fetches from GitHub automatically. Users set up the Python backend separately — see [devtrack.cloud](https://devtrack.cloud).

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Developer's Workflow                     │
│ (Git commits, scheduled times, command line)                │
└────────────────────────┬────────────────────────────────────┘
                         │
            ┌────────────┼────────────┐
            │            │            │
            ▼            ▼            ▼
      Git Commit   Cron Timer    CLI Commands
            │            │            │
            └────────────┼────────────┘
                         │
         ┌───────────────▼───────────────┐
         │    Go Background Daemon       │
         │   (devtrack_client/ packages) │
         │                               │
         ├─ Git Repository Monitor      │
         ├─ Scheduler (cron-based)      │
         ├─ HTTP Trigger Client         │
         ├─ SQLite Database             │
         └─ Configuration Manager       │
                         │
        ─────────────────┼───── HTTPS POST /trigger/* ──────────
                         │
         ┌───────────────▼───────────────────────────────────┐
         │   Python Backend (webhook_server.py + backend/)   │
         │  FastAPI on port 8089, self-signed ECDSA cert      │
         │                                                   │
         ├─ Trigger Handlers (/trigger/commit, /timer…)      │
         ├─ NLP Parser (spaCy)                              │
         ├─ LLM Integration (Ollama/OpenAI/Anthropic)       │
         ├─ TUI (Terminal User Interface)                    │
         ├─ Report Generator                                │
         ├─ Task Matching                                    │
         └─ API Integrations                                │
                         │
        ┌────────────────┼────────────────┬─────────────────┐
        │                │                │                 │
        ▼                ▼                ▼                 ▼
    Azure DevOps      GitHub          Teams              Outlook
    Work Items        Issues/PRs      Notifications      Email
```

---

## Component Breakdown

### 1. Go Daemon Layer (devtrack_client/)

The lightweight background service that monitors and coordinates.

#### Key Components

| Component | File | Purpose |
|-----------|------|---------|
| **Entry Point** | main.go | Routes CLI args or delegates git subcommand |
| **CLI Handler** | cli.go | Implements all CLI commands (start, stop, status, etc.) |
| **Daemon Lifecycle** | daemon.go | Manages lock file, PID file, signals, Python bridge process |
| **Lock (Unix)** | lock_unix.go | Exclusive `flock(LOCK_EX\|LOCK_NB)` on `devtrack.lock` |
| **Lock (Windows)** | lock_windows.go | Exclusive `LockFileEx` on `devtrack.lock` |
| **Process check (Unix)** | process_unix.go | `Signal(0)` liveness probe |
| **Process check (Windows)** | process_windows.go | `OpenProcess` + `GetExitCodeProcess` liveness probe |
| **Integration Hub** | integrated.go | Wires together git monitor, scheduler, HTTP trigger client |
| **Git Monitor** | git_monitor.go | fsnotify-based repository watcher, fires commit_trigger; lazy-wired on first commit for empty repos |
| **Auto-Enhance** | auto_enhance.go | Background AI enhancement of plain `git commit` messages (opt-in via `DEVTRACK_AUTO_ENHANCE=true`) |
| **Scheduler** | scheduler.go | Cron-based periodic trigger, fires timer_trigger |
| **HTTP Trigger Client** | server_config.go + daemon.go | POSTs JSON to Python `/trigger/*` via HTTPS (self-signed ECDSA cert) |
| **Database** | database.go | SQLite access, trigger history, task updates |
| **Configuration** | config.go, config_env.go | YAML struct + .env accessors |
| **Upgrade** | upgrade.go, upgrade_unix.go, upgrade_windows.go | `devtrack upgrade` — fetches from GitHub Releases; Unix uses `sudo cp`, Windows prints Administrator guidance |
| **Learning** | learning.go | AI learning consent and profile management |
| **Message Queue** | queue.go | Store-and-forward IPC messages for offline resilience |
| **Health Monitor** | health.go | Periodic service health checks with auto-restart |
| **Deferred Commits** | deferred_commit.go | Queue commits for later AI enhancement |

#### Single-Instance Enforcement

`daemon.Start()` acquires an OS-level exclusive lock on `devtrack.lock` (in the PID directory) as its very first operation, before writing the PID file or starting any subprocess.

- **Unix**: `flock(LOCK_EX|LOCK_NB)` — non-blocking; fails immediately if another instance holds the lock.
- **Windows**: `LockFileEx(LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY)` — equivalent Windows API.

If the lock cannot be acquired the daemon exits with:

```
Error: daemon already running (could not acquire lock)
```

The OS releases the lock automatically when the process exits for any reason, so a crashed daemon never leaves a stale lock behind. There is no need to delete `devtrack.lock` manually.

`IsRunning()` uses a platform-specific process liveness check: `Signal(0)` on Unix and `OpenProcess`+`GetExitCodeProcess` on Windows, so stale PID files from terminated processes are correctly treated as "not running".

#### Trigger Endpoints

Go daemon sends all triggers to the Python backend via HTTPS POST. The Python backend (`webhook_server.py`) exposes `/trigger/*` endpoints:

```
POST /trigger/commit_trigger   → Git commit detected (includes workspace_name, pm_platform, pm_project)
POST /trigger/timer_trigger    → Scheduled time reached (includes workspace context in multi-repo mode)
POST /trigger/workspace_reload → Reload workspaces.yaml and restart monitors
GET  /health                   → Python backend health check
```

All trigger requests include an `X-DevTrack-API-Key` header (when `DEVTRACK_API_KEY` is set). Go pins the Python server's TLS cert at startup to prevent MITM on localhost.

#### Multi-Repo Mode

`workspaces.yaml` is always required — the single-repo env-var fallback has been removed. `IntegratedMonitor` starts one `WorkspaceMonitor` per enabled workspace. Each monitor fires `handleCommitForWorkspace`, embedding `workspace_name`, `pm_platform`, and `pm_project` into the trigger message.

```
workspaces.yaml
      │
      ▼
IntegratedMonitor
  ├── WorkspaceMonitor (work-api, pm_platform=azure)
  │     └── GitMonitor → commit_trigger {pm_platform: "azure", workspace_name: "work-api"}
  ├── WorkspaceMonitor (oss-lib, pm_platform=github)
  │     └── GitMonitor → commit_trigger {pm_platform: "github", workspace_name: "oss-lib"}
  └── WorkspaceMonitor (internal-tools, pm_platform=gitlab)
        └── GitMonitor → commit_trigger {pm_platform: "gitlab", workspace_name: "internal-tools"}
```

Python bridge reads `pm_platform` from the trigger data and calls `_route_pm_sync()`, which dispatches directly to the declared platform without running the priority chain.

#### PM Connector Configuration

PM connectors use a two-layer configuration model (as of `feat/client-server-decoupling`):

| Layer | Source | Contents |
|---|---|---|
| Secrets | `.env` | `GITHUB_TOKEN`, `GITLAB_PAT`, `AZURE_DEVOPS_PAT` |
| Non-secret config | `workspaces.yaml` | `pm_org`, `pm_username`, `pm_api_url` |

Each workspace entry in `workspaces.yaml` carries:
- `pm_org` — Azure org name, or GitHub/GitLab owner/org
- `pm_username` — assignee filter (GitHub login / GitLab username / Azure email)
- `pm_api_url` — optional self-hosted URL override (GitHub Enterprise, self-hosted GitLab, etc.)
- `skip_issues` — `true` marks the workspace as code-only; excluded from `devtrack issues`, `SyncAllTickets`, `PushCachedTickets`, and the commit-time ticket picker. Use when one repo is tracked in two platforms (e.g. GitHub for code, Azure DevOps for PM) to prevent duplicate ticket lists. `ResolveWorkspaceForPath` prefers non-skip workspaces at equal path depth so PM-authoritative entries win commit routing.

All connector constructors (`pm.NewGitHubClient(ws)`, `pm.NewGitLabClient(ws)`, `pm.NewAzureClient(ws)`) take an explicit workspace struct and never read non-secret config from env.

#### Data Storage

SQLite database (`Data/db/devtrack.db`) stores:
- Trigger history (commits, timers)
- Task updates sent to external systems
- User preferences and learning profiles
- Error logs and debugging info
- Message queue (store-and-forward for offline IPC)
- Deferred commits (commits awaiting AI enhancement)
- Health snapshots (service health check history)

---

### 2. Python Intelligence Layer (backend/)

The smart processing engine that handles AI, NLP, and integrations.

#### Core Infrastructure

| Module | Purpose |
|--------|---------|
| **webhook_server.py** | FastAPI entry point started by Go daemon; exposes `/trigger/*` HTTP endpoints, inbound webhooks, Admin Console, Slack bot, and Vacation auto-responder |
| **backend/config.py** | Centralized config; all modules use `get()`, `get_int()`, `get_bool()`, `get_path()` |

#### NLP & AI Processing

> **Requires the `ai` tier.** The modules below depend on `spacy`, `sentence-transformers`, and `chromadb`, which are not installed by default. Install them with `devtrack-server enable ai`. When the `ai` tier is absent, `backend/config.py:is_ai_available()` returns `False` and the server logs `feature:ai disabled` at startup — all other features continue to work normally.

| Module | Purpose |
|--------|---------|
| **backend/nlp_parser.py** | spaCy-based NLP for commit/user text → structured task data (`ai` tier) |
| **backend/description_enhancer.py** | Ollama-based description enhancement and categorization (`ai` tier) |
| **backend/llm/provider_factory.py** | Multi-provider LLM abstraction with fallback chain |
| **backend/llm/ollama_provider.py** | Local Ollama integration |
| **backend/llm/openai_provider.py** | OpenAI GPT-4 integration |
| **backend/llm/anthropic_provider.py** | Anthropic Claude integration |
| **backend/personalized_ai.py** | AI learning from user communications (`ai` tier) |
| **backend/rag/** | ChromaDB-backed RAG few-shot examples for personalization (`ai` tier) |
| **backend/learning_integration.py** | Learning consent and profile handling |

#### User Interaction & Reporting

| Module | Purpose |
|--------|---------|
| **backend/user_prompt.py** | Terminal TUI for interactive work-update prompts |
| **backend/daily_report_generator.py** | AI-enhanced daily/weekly report generation (Terminal, HTML, Markdown, JSON) |
| **backend/email_reporter.py** | Report delivery via email/Teams |
| **backend/task_matcher.py** | Fuzzy + semantic matching of natural language to tracked tasks |

#### Git Integration

| Module | Purpose |
|--------|---------|
| **backend/commit_message_enhancer.py** | AI-powered iterative commit message refinement |
| **backend/git_diff_analyzer.py** | Analyzes staged changes for context |
| **backend/git_sage/agent.py** | Agentic loop for autonomous git operations |
| **backend/git_sage/llm.py** | Ollama and OpenAI-compatible LLM backends |
| **backend/git_sage/context.py** | Git repository state collection |
| **backend/git_sage/config.py** | ~/.config/git-sage/config.json management |
| **backend/git_sage/git_operations.py** | Advanced git operations (branches, commits, merges, blame, stash) |
| **backend/git_sage/conflict_resolver.py** | Intelligent conflict analysis and resolution |
| **backend/git_sage/pr_finder.py** | PR/MR utilities and analysis |

#### External Integrations

| Module | Purpose |
|--------|---------|
| **backend/workspace_router.py** | Routes PM sync calls to the correct platform based on `pm_platform` from workspaces.yaml |
| **backend/azure/client.py** | Azure DevOps work item fetching/updating |
| **backend/gitlab/client.py** | GitLab issue fetching, commenting, creating |
| **backend/github/client.py** | GitHub issue fetching, commenting, creating (GHE-ready) |
| **backend/jira/client.py** | Jira REST API client |
| **backend/msgraph_python/** | Microsoft Graph integration (Teams, Outlook) |

---

## Data Flow Diagrams

### 1. Commit Trigger Flow

```
Developer makes a commit
         │
         ▼
Git hook detected by fsnotify
         │
         ▼
Go daemon receives file event
         │
         ▼
[DEVTRACK_AUTO_ENHANCE=true] Auto-enhance: read diff, call LLM,
amend commit message in-place (skipped if commit is already pushed)
         │
         ▼
Go daemon logs to database
         │
         ▼
Go daemon POSTs commit_trigger to Python /trigger/commit_trigger
         │
         ▼
Python backend receives commit_trigger
         │
         ├─ Extract commit hash, message, diff
         ├─ Get git context (branch, PR, recent commits)
         ├─ Parse commit message with NLP (repo_path support)
         ├─ Enhance with AI (Ollama)
         ├─ Extract task numbers
         │
         ▼
Send task_update to project management (Azure DevOps, GitHub, etc.)
         │
         ▼
Send acknowledge back to Go daemon
         │
         ▼
Log completion in database
```

#### Background Commit-Message Auto-Enhancement

When `DEVTRACK_AUTO_ENHANCE=true`, the daemon enhances every detected commit's message before forwarding it to the Python server. This is designed for workflows where developers use plain `git commit` without going through `devtrack git commit`.

**How it works:**

1. The git monitor detects a new commit (via fsnotify or 2-second polling).
2. The daemon checks:
   - Is `DEVTRACK_AUTO_ENHANCE=true`?
   - Is the configured LLM reachable (Ollama / OpenAI / Groq)?
   - Has the commit been pushed to any remote? (If yes, skip — amending would require a force-push.)
   - Are there any staged changes in the working tree? (If yes, skip — amending would accidentally include them.)
3. If all checks pass, the daemon reads the full commit diff (`git show HEAD`), sends it to the LLM, and generates a Conventional Commits-formatted message.
4. The commit is amended in-place (`git commit --amend -m "enhanced message"`).
5. The new commit hash is recorded internally to prevent re-processing the amended commit.
6. The trigger payload sent to the Python server carries the enhanced message.

**Safeguards:**

| Guard | Behaviour |
|---|---|
| Commit already pushed | Skipped — amending would diverge the branch |
| Staged changes present | Skipped — amending would alter the commit's tree |
| LLM unreachable | Skipped — original message is preserved |
| LLM returns same or empty message | Skipped — no amend performed |
| Amended commit re-detected | Skipped via in-memory hash set (loop guard) |

**Configuration:**

```env
DEVTRACK_AUTO_ENHANCE=true   # opt-in (default: false)

# The LLM used is whichever SAGE_PROVIDER points to (default: ollama)
GIT_SAGE_PROVIDER=ollama
OLLAMA_HOST=http://localhost:11434
GIT_SAGE_DEFAULT_MODEL=llama3.2
```

### 2. Timer Trigger Flow

```
Scheduled time reached (cron)
         │
         ▼
Go daemon POSTs timer_trigger to Python /trigger/timer_trigger
         │
         ▼
Python backend receives timer_trigger
         │
         ├─ [Vacation mode active?] → VacationAutoResponder generates update from commits,
         │                            scores confidence, posts to PM if above threshold
         │
         │
         ├─ Show TUI prompt to user
         ├─ Get work update from user input
         ├─ Enhance with work context (git branch, recent commits)
         ├─ Parse with NLP (repo_path support for PR detection)
         ├─ Enhance description with AI
         ├─ Extract task numbers
         │
         ▼
Check for merge conflicts
         │
         ├─ Run git status
         ├─ Detect conflict markers
         ├─ Auto-resolve with ConflictAutoResolver
         │
         ▼
Send task_update to project management
         │
         ├─ Update task status
         ├─ Add work log entry
         ├─ Update time tracking
         │
         ▼
Generate optional report
         │
         ├─ Collect recent work items
         ├─ Summarize with AI
         ├─ Format (Terminal, HTML, Email)
         │
         ▼
Send acknowledge back to Go daemon
         │
         ▼
Log completion in database
```

### 3. User Prompt to Task Update Flow

```
User triggered (timer or manual)
         │
         ▼
TUI Prompt: "What are you working on?"
         │
         ▼
User types natural language: "Working on PR #123 - fixed auth bug, took 2 hours"
         │
         ▼
NLP Parser (spaCy)
├─ Tokenize and POS tag
├─ Extract entities (task numbers, time)
├─ Detect actions (working on, fixed, completed)
└─ Create structured data: {task: "PR #123", action: "in progress", time: 2h}
         │
         ▼
Work Update Enhancer (Phase 2)
├─ Add git context (branch name, recent commits)
├─ Auto-detect PR/issue from branch name or history
└─ Enrich with linked tasks
         │
         ▼
Description Enhancer (AI)
├─ Improve clarity and grammar
├─ Add technical context
└─ Categorize work (feature, bug, doc, etc.)
         │
         ▼
Task Matcher
├─ Fuzzy match to known tasks
├─ Semantic match using sentence-transformers
└─ Verify against project management system
         │
         ▼
Create task_update message
{
  "task_id": "PR-123",
  "status": "in progress",
  "description": "Fixed authentication bug in OAuth flow",
  "time_spent_hours": 2,
  "category": "bug fix"
}
         │
         ▼
Send to project management APIs
├─ Azure DevOps: Update work item
├─ GitHub: Update PR/Issue
├─ Teams: Post status update
└─ Log in local database
         │
         ▼
Acknowledge to user: "Updated PR #123 with 2 hours of work"
```

---

## LLM Provider Architecture

DevTrack uses a flexible LLM provider system with fallback chain:

```python
# backend/llm/provider_factory.py
def get_llm_provider():
    """
    Returns LLM provider with automatic fallback:
    1. Primary: User's chosen provider (OpenAI, Anthropic, or Ollama)
    2. Fallback 1: If primary unavailable, try next available
    3. Fallback 2: Eventually fallback to Ollama (always works offline)
    """

    # Load primary from LLM_PROVIDER env var
    primary = LLM_PROVIDER  # e.g., "openai"

    # Auto-add fallbacks based on credentials
    fallbacks = []
    if OPENAI_API_KEY:
        fallbacks.append("openai")
    if ANTHROPIC_API_KEY:
        fallbacks.append("anthropic")

    # Try each in order
    for provider in [primary] + fallbacks:
        if is_available(provider):
            return load_provider(provider)

    # Last resort: always have Ollama
    return OllamaProvider()
```

**Benefits**:
- Use local Ollama by default (free, offline)
- Transparent fallback to commercial APIs if needed
- Graceful degradation if all AI unavailable
- Cost optimization (try cheaper option first)

---

## Configuration Management

All configuration flows from a single `.env` file with **no hardcoded defaults**.

### How It Works

1. **Go layer** (`devtrack_client/`):
   - Loads `.env` via `joho/godotenv`
   - Exposes variables through `config_env.go` functions
   - All access goes through these functions (not `os.Getenv` directly)

2. **Python layer** (`backend/`):
   - Loads `.env` via `python-dotenv`
   - Accesses via `backend/config.py` functions
   - All modules use `get()`, `get_int()`, `get_bool()`, `get_path()`
   - No `os.getenv()` calls in business logic

3. **Override mechanism**:
   - `DEVTRACK_ENV_FILE` env var overrides default `.env` location
   - If `DEVTRACK_ENV_FILE` not set, looks for `.env` in working directory
   - If neither found, exits with explicit error message

### Key Environment Variables

| Variable | Layer | Purpose | Example |
|----------|-------|---------|---------|
| `PROJECT_ROOT` | Both | Path to repository | `/home/user/automation_tools` |
| `DEVTRACK_WORKSPACE` | Both | Git repo to monitor | Same as PROJECT_ROOT or custom repo |
| `DATA_DIR` | Both | Runtime data location | `${PROJECT_ROOT}/Data` |
| `IPC_HOST` | Both | IPC server host | `127.0.0.1` |
| `IPC_PORT` | Both | IPC server port | `35893` |
| `LLM_PROVIDER` | Python | Primary AI provider | `ollama` or `openai` or `anthropic` |
| `OLLAMA_URL` | Python | Ollama server URL | `http://localhost:11434` |
| `OPENAI_API_KEY` | Python | OpenAI credentials | (secret) |
| `ANTHROPIC_API_KEY` | Python | Anthropic credentials | (secret) |
| `AZURE_DEVOPS_PAT` | Go + Python | Azure DevOps PAT (secret only — org/username in workspaces.yaml) | (secret) |
| `GITHUB_TOKEN` | Go + Python | GitHub PAT (secret only — org/username in workspaces.yaml) | (secret) |
| `GITLAB_PAT` | Go | GitLab PAT (secret only — org/username in workspaces.yaml) | (secret) |
| `TEAMS_BOT_ID` | Python | Teams bot ID | (secret) |

---

## Database Schema

SQLite database stored at `Data/db/devtrack.db`.

### Tables

#### triggers
```sql
CREATE TABLE triggers (
    id INTEGER PRIMARY KEY,
    type TEXT NOT NULL,           -- 'commit' or 'timer'
    trigger_time DATETIME,        -- When triggered
    git_ref TEXT,                 -- Commit hash or branch
    parsed_data JSON,             -- Extracted task data
    ai_enhanced_data JSON,        -- AI-enhanced version
    status TEXT,                  -- 'pending', 'processing', 'completed', 'error'
    error_message TEXT
);
```

#### task_updates
```sql
CREATE TABLE task_updates (
    id INTEGER PRIMARY KEY,
    task_id TEXT,                 -- e.g., 'PROJ-123'
    system TEXT,                  -- 'azure_devops', 'github', 'jira'
    action TEXT,                  -- 'status_change', 'comment', 'time_tracking'
    payload JSON,                 -- Full update data
    status TEXT,                  -- 'queued', 'sent', 'failed'
    created_at DATETIME,
    sent_at DATETIME
);
```

#### learning_profiles
```sql
CREATE TABLE learning_profiles (
    id INTEGER PRIMARY KEY,
    user_id TEXT,
    communication_style JSON,     -- Learned patterns
    preferred_terms JSON,         -- Favorite phrases
    last_updated DATETIME,
    consent_given BOOLEAN
);
```

#### message_queue
```sql
CREATE TABLE message_queue (
    id INTEGER PRIMARY KEY,
    message_type TEXT NOT NULL,
    message_id TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 10,
    last_error TEXT,
    created_at DATETIME,
    updated_at DATETIME
);
```

#### deferred_commits
```sql
CREATE TABLE deferred_commits (
    id INTEGER PRIMARY KEY,
    original_message TEXT NOT NULL,
    diff_patch TEXT,
    branch TEXT,
    repo_path TEXT,
    files_changed TEXT,
    status TEXT DEFAULT 'pending',
    enhanced_message TEXT,
    created_at DATETIME,
    updated_at DATETIME
);
```

#### health_snapshots
```sql
CREATE TABLE health_snapshots (
    id INTEGER PRIMARY KEY,
    service TEXT NOT NULL,
    status TEXT NOT NULL,
    latency_ms INTEGER DEFAULT 0,
    details TEXT,
    checked_at DATETIME
);
```

---

## Trigger Protocol (HTTPS POST)

Go daemon sends all triggers as JSON POST requests to the Python backend over HTTPS. A self-signed ECDSA certificate is generated at daemon startup (`Data/tls/server.{crt,key}`) and cert-pinned in the Go TLS client — no CA required.

### Trigger Endpoint Format

```
POST https://127.0.0.1:<WEBHOOK_PORT>/trigger/<type>
Content-Type: application/json
X-DevTrack-API-Key: <DEVTRACK_API_KEY>   (if configured)

{
  "commit_hash": "abc123def456",
  "branch": "feature/auth",
  "message": "Fixed OAuth flow",
  "workspace_name": "my-api",
  "pm_platform": "azure"
}
```

### Trigger Types

| Endpoint | Direction | Purpose |
|----------|-----------|---------|
| `POST /trigger/commit_trigger` | Go → Python | Git commit detected |
| `POST /trigger/timer_trigger` | Go → Python | Scheduled time reached |
| `POST /trigger/workspace_reload` | Go → Python | Reload workspaces.yaml |
| `GET  /health` | Go → Python | Liveness check |

### TLS Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DEVTRACK_TLS` | `true` | Enable HTTPS (disable only in trusted Docker networks) |
| `DEVTRACK_TLS_CERT` | `Data/tls/server.crt` | Auto-generated cert path |
| `DEVTRACK_TLS_KEY` | `Data/tls/server.key` | Auto-generated key path |
| `DEVTRACK_API_KEY` | — | Optional bearer token for cloud/public deployments |

---

## Offline Resilience

DevTrack's Go daemon operates as a resilient local agent that works offline and syncs when services recover.

### Store-and-Forward Queue

```
Trigger → MessageQueue.SendOrQueue()
              ├── IPC Send succeeds → done
              └── No clients → Enqueue in SQLite
                                    ↓
                      Drain goroutine (every 10s)
                          → Check HasClients()
                          → Send pending messages
                          → Mark completed/retry
```

### Health Monitoring

The daemon checks 6 services every 30 seconds:

| Service | Check Method | Auto-Restart |
|---------|-------------|--------------|
| Python backend | HTTP GET /health | Yes |
| Ollama | HTTP GET /api/tags | No |
| Azure DevOps | Config presence check | No |
| Webhook Server | Process liveness | Yes |
| Telegram bot | Process liveness | Yes |
| MongoDB | TCP dial timeout | No |

### Deferred Commits

When AI is unavailable during `devtrack git commit`:
- User can queue the commit for later enhancement
- Stored in SQLite with diff, branch, files metadata
- `devtrack commits review` for interactive approval when AI returns

Offline-first is a core non-negotiable — see [`PRODUCT_BIBLE.md`](../PRODUCT_BIBLE.md).

---

## Technology Stack

### Go Dependencies
- `github.com/robfig/cron/v3` - Cron scheduling
- `github.com/fsnotify/fsnotify` - File system monitoring
- `github.com/joho/godotenv` - .env file loading
- `modernc.org/sqlite` - SQLite database
- `gopkg.in/yaml.v3` - YAML configuration

### Python Dependencies

**Core tier** (installed by default via `uv sync`):
- `python-dotenv` - .env file loading
- `requests` - HTTP client
- `azure-devops` - Azure DevOps SDK
- `PyGithub` - GitHub API
- `atlassian-python-api` - Jira API
- `msgraph-core` - Microsoft Graph SDK

**AI tier** (installed via `devtrack-server enable ai`):
- `spacy[en_core_web_sm]` - NLP and entity extraction
- `sentence-transformers` - Semantic matching
- `chromadb` - RAG vector store for personalization

---

## Phases & Evolution

### Phase 1: Enhanced Commit Messages
Added git context (branch, PR, recent commits) to AI prompts for better commit message generation.

### Phase 2: Conflict Resolution & PR-Aware Parsing
Automatic merge conflict resolution and git-aware work update parsing with PR/issue auto-detection.

### Phase 3: Event-Driven Integration
Seamless integration of Phases 1 & 2 into webhook_server.py's real-time event pipeline.

### Offline-First Resilience
Store-and-forward message queue, deferred commit enhancement, health monitoring with auto-restart, and enhanced status dashboard.

### Future Phases
- Dashboard and analytics
- Mobile notifications
- Plugin system
- Advanced project planning

---

## Next Steps

- **For development**: See [CLAUDE.md](../CLAUDE.md)
- **Setup, configuration, and troubleshooting**: see [devtrack.cloud](https://devtrack.cloud)
- **Client↔server boundary & env config**: this document and the project `CLAUDE.md`
