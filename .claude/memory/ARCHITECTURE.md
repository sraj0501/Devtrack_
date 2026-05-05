# DevTrack Architecture & Patterns

**Last Updated**: May 6, 2026 (updated for CLI/server split and HTTP API)

## Three-Layer Architecture (post-migration)

```
User Git Activity / Cron Timer
        ↓
┌──────────────────────────────────────────────────────────────────────────┐
│  devtrack-server binary (devtrack_server/devtrack-bin/)                  │
│                                                                          │
│  ┌─────────────────┐   HTTP API :8765   ┌──────────────────────────┐    │
│  │  Go Daemon      │ ◀─────────────────▶ │  devtrack-cli            │    │
│  │  - git_monitor  │                     │  (devtrack_client/)      │    │
│  │  - scheduler    │   TCP IPC :35893   ┌──────────────────────────┐    │
│  │  - http_api.go  │ ◀─────────────────▶ │  Python Bridge           │    │
│  │  - database     │                     │  python_bridge.py        │    │
│  │  - cli          │                     │  - NLP, LLM, TUI         │    │
│  └─────────────────┘                     │  - admin GUI             │    │
│                                          │  - integrations          │    │
└──────────────────────────────────────────┴──────────────────────────┘    │
        ↓                                              ↓
  SQLite (Data/)                        Azure/GitHub/GitLab/Jira/Telegram
```

**Key transport rules:**
- `devtrack-cli` → `devtrack-server`: HTTP on port `DEVTRACK_SERVER_HTTP_PORT` (8765)
- Go daemon → Python backend: HTTPS POST to `webhook_server.py` (CS-1 transport)
- TCP IPC (`127.0.0.1:35893`): retained for legacy internal Go↔Python messages only
- `contract/api.go`: single source of truth for all HTTP route constants and request/response types

## Core Components

### Go Layer (`devtrack_server/devtrack-bin/`)

**File Roles**:
- `main.go` - Entry point; routes CLI args
- `cli.go` - All CLI command implementations
- `daemon.go` - Lifecycle management (PID file, signals, Python bridge process)
- `http_api.go` - HTTP API server: 9 REST routes, `X-DevTrack-Token` auth, starts in `daemon.Start()`
- `integrated.go` - IntegratedMonitor: wires git monitor, scheduler, IPC server
- `git_monitor.go` - fsnotify-based Git watcher; fires `commit_trigger` on commits
- `scheduler.go` - Cron-based periodic trigger; fires `timer_trigger`
- `ipc.go` - TCP IPC server (Go side); JSON-delimited messages
- `database.go` - SQLite via modernc.org/sqlite; trigger history, task updates, alert_state
- `config_env.go` - **Single source of truth** for env var names
- `loadenv.go` - `AutoLoadEnv()`: loads .env before any command
- `setup.go` - `devtrack setup` interactive onboarding wizard
- `upgrade.go` - `devtrack upgrade` self-update via GitLab releases
- `ping.go` - Anonymous install/active telemetry ping
- `server_config.go` - `DEVTRACK_SERVER_MODE`: managed/lightweight/external/cloud

### CLI Client (`devtrack_client/`)

**Thin Go binary** — proxies user commands to devtrack-server over HTTP.

- `cmd/cli/main.go` - Entry point; reads `DEVTRACK_SERVER_URL`, routes commands
- `cli_client.go` - `CLIClient` struct: all 9 HTTP methods using `contract` types
- `go.mod` - module: `gitlab.com/devtrack3_cloud/devtrack_cli`
- Build: `go build -o devtrack-cli ./cmd/cli`
- Required env: `DEVTRACK_SERVER_URL`, `CLI_APP_NAME`, `DEVTRACK_VERSION`

### Shared Contract (`contract/api.go`)

Module `gitlab.com/devtrack3_cloud/devtrack_contract`. Defines:
- HTTP route path constants (`RouteHealth`, `RouteStatus`, ...)
- Request/response structs for all 9 endpoints
- `AuthHeader` constant (`X-DevTrack-Token`)

**Always read `contract/api.go` first when debugging a server-client issue.**

### HTTP API Routes (9 endpoints)

| Route | Method | Purpose |
|---|---|---|
| `/health` | GET | Server liveness check |
| `/status` | GET | Daemon status + stats |
| `/git/commit` | POST | Trigger commit processing |
| `/start` | POST | Start daemon |
| `/stop` | POST | Stop daemon |
| `/projects` | GET | List monitored projects |
| `/configure` | POST | Update config at runtime |
| `/log` | GET | Recent log entries |
| `/alerts` | GET | Active alerts |

### Python Layer (`devtrack_server/backend/` + `python_bridge.py`)

**Module Organization**:
- `python_bridge.py` - Entry point started by Go daemon; connects to IPC
- `backend/config.py` - Centralized config (all modules use this, never `os.getenv`)
- `backend/webhook_server.py` - FastAPI: handles inbound webhooks + Go trigger calls
- `backend/personalization.py` - Global `inject_style()` — combines profile + RAG
- `backend/workspace_router.py` - Per-workspace PM platform routing
- `backend/admin/` - Admin GUI (Jinja2/HTMX, 21 routes)
- `backend/alerters/jira_alerter.py` - Jira async poller
- `backend/alert_poller.py` - GitHub + Azure async alert polling
- `backend/git_sage/` - git-sage autonomous agent subsystem
- `backend/rag/` - ChromaDB + Ollama embedding for personalization
- `backend/llm/` - Multi-provider LLM abstraction (Ollama/OpenAI/Anthropic/Groq)

**Key Pattern**: All subsystems imported with `try/except`, individually gated; degrades gracefully

## IPC Message Protocol

**CRITICAL**: Message types defined in TWO PLACES — MUST STAY IN SYNC:
1. Go: `devtrack-bin/ipc.go` (MessageType constants)
2. Python: `backend/ipc_client.py` (MessageType enum)

**Format**: JSON-newline-delimited over TCP  
**Default**: `127.0.0.1:35893` (configurable via `IPC_HOST`/`IPC_PORT`)

## Configuration Pattern: NO Defaults Approach

**Single Source of Truth**: `.env` file (configurable via `DEVTRACK_ENV_FILE`)
- Missing env var → clear error message (no silent failures)
- Go reads via `config_env.go` functions — all panic with clear message if missing
- Python reads via `backend/config.py` typed accessors — `os.getenv` banned outside `config.py`
- `AutoLoadEnv()` in `loadenv.go` handles auto-loading before any command runs

## Deployment Modes (`DEVTRACK_SERVER_MODE`)

| Mode | Constant | Behaviour |
|---|---|---|
| `managed` | `ServerModeManaged` | Default; daemon spawns Python backend as subprocess |
| `lightweight` | `ServerModeLightweight` | Git monitoring + scheduling only; no Python |
| `external` | `ServerModeExternal` | Daemon only; Python runs on separate server |
| `cloud` | `ServerModeCloud` | Remote cloud-hosted backend |

## Testing Patterns

```bash
# Go
cd devtrack_server/devtrack-bin && go test ./...

# Python
uv run pytest devtrack_server/backend/tests/

# CLI unit tests
cd devtrack_client && go test ./...
```

**Test isolation**: Use `DATABASE_DIR` monkeypatching (`monkeypatch.setenv("DATABASE_DIR", str(tmp_path))`) for any test touching SQLite.

## Common Development Tasks

### Adding a New HTTP API Route
1. Add route constant to `contract/api.go`
2. Add request/response structs to `contract/api.go`
3. Register handler in `devtrack_server/devtrack-bin/http_api.go`
4. Add corresponding client method in `devtrack_client/cli_client.go`
5. Add CLI command dispatch in `devtrack_client/cmd/cli/main.go`

### Adding New CLI Command (server-side only)
1. Create handler in `devtrack_server/devtrack-bin/cli.go`
2. If it requires Python backend: add to `requiresManagedMode()` guard list
3. Wire into CLI switch statement + help text

### Adding New Python Module
1. Create in `devtrack_server/backend/` directory
2. Import with `try/except` at top of files using it
3. Set availability flag (`HAS_MODULE = True/False`)
4. Add config accessors to `backend/config.py` — never `os.getenv` directly
5. Add tests in `backend/tests/`

## Debugging Patterns

**Cross-repo server-client issue**: read `contract/api.go` + `http_api.go` + `cli_client.go` together.

**Enable Debug Logging**:
```bash
LOG_LEVEL=debug devtrack start
```

**Inspect Database**:
```bash
sqlite3 Data/db/devtrack.db
sqlite> SELECT * FROM triggers ORDER BY trigger_time DESC LIMIT 10;
```

**Check IPC Communication**:
```bash
devtrack logs -f    # watch daemon logs in one terminal
devtrack force-trigger    # in another terminal
```

## File Organization Principles

- **Python**: One module per file, directories for related modules
- **Go**: Related functions in same file, separate files by concern
- **Config**: All in `.env`, accessed via config functions — zero hardcoded values
- **Tests**: Mirror source structure in `backend/tests/`
- **Docs**: User-facing in `devtrack_wiki/docs/`; architecture reference in `.claude/memory/`
