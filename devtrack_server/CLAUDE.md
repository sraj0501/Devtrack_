# devtrack_server — Python AI Pipeline

Python package: `devtrack-server` (see `pyproject.toml`)

This is the **canonical** Python source for the DevTrack server. The monorepo's root `backend/` directory is a legacy mirror being retired in TASK-048. All new Python development goes here.

See the monorepo `CLAUDE.md` for full project context, configuration patterns, and vision rules.
See `docs/ARCHITECTURE.md` for the HTTP/JSON boundary between `devtrack_client` (Go) and this server.

**Note**: git-sage is NOT here. It is Go-native and client-owned at `devtrack_client/gitsage/`.

## Run & Test

```bash
cd devtrack_server

# Install dependencies (includes required PostgreSQL driver)
uv sync                                      # core deps (no AI/spaCy)
uv sync --extra ai                           # include AI/NLP deps

# Start the server (POSTGRES_URL is required; migrations run automatically)
uv run python -m backend.webhook_server      # FastAPI on port 8089

# Run tests
uv run pytest backend/tests/                 # all tests
uv run pytest backend/tests/ -x -q          # stop on first failure
uv run pytest backend/tests/test_api_contract.py -v  # API contract tests
```

## Architecture

The server is a FastAPI application that handles two classes of requests:

1. **Inbound webhooks** from external platforms (Azure DevOps, GitHub, GitLab, Jira) at `/webhooks/<source>`
2. **Trigger calls** from the Go client at `/trigger/commit`, `/trigger/timer`, etc.

```
devtrack_client (Go)
        |
        | HTTPS POST /trigger/*
        |
devtrack_server/backend/webhook_server.py   <-- FastAPI entry point
        |
        |-- /trigger/commit  --> NLP parser -> LLM enhancement -> PM APIs
        |-- /trigger/timer   --> TUI prompt -> work update -> report
        |-- /webhooks/*      --> webhook_handlers.py -> event routing
        |-- /admin/*         --> admin UI (HTMX, JWT auth)
        |-- /health          --> health check
        |-- /version         --> version info
        |-- /boardroom       --> multi-persona AI review
        |-- /plan            --> Epic/Story/Task decomposition
```

### Key Modules

| Module | Purpose |
|---|---|
| `backend/webhook_server.py` | FastAPI app; all routes; spawned by Go client in managed mode |
| `backend/webhook_handlers.py` | `WebhookEventHandler` — routes Azure/GitHub/Jira events |
| `backend/config.py` | All config — use `backend.config.get()`, never `os.getenv()` directly |
| `backend/ipc_client.py` | Legacy TCP IPC client (Python side) — prefer HTTP triggers for new code |
| `backend/nlp_parser.py` | spaCy NLP for commit/user text → structured task data |
| `backend/description_enhancer.py` | Ollama-based description enhancement |
| `backend/llm/` | Multi-provider LLM abstraction (Ollama / OpenAI / Anthropic / Groq) |
| `backend/user_prompt.py` | Terminal TUI for interactive work-update prompts |
| `backend/daily_report_generator.py` | AI-enhanced EOD/weekly report generation |
| `backend/boardroom/` | Multi-persona AI plan review (7 personas, SWOT, verdict) |
| `backend/plan_parser.py` | Parses free-text or Markdown plans into structured sections |
| `backend/jira/` | Jira REST API client |
| `backend/github/` | GitHub API integration |
| `backend/azure/` | Azure DevOps integration |
| `backend/msgraph_python/` | Microsoft Graph (Teams, Outlook) |
| `backend/db/` | PostgreSQL-backed server stores and migrations; client SQLite boundary helpers |
| `backend/ai/` | Low-level AI utilities (Ollama client, inference helpers) |
| `backend/admin/` | Admin console routes (FastAPI + HTMX, JWT auth, user/license management) |

### What is NOT here

- git-sage — Go-native at `devtrack_client/gitsage/` (client-owned)
- `python_bridge.py` — legacy root entry point; removed in TASK-048

## Configuration

All Python modules access config via `backend.config.get()`, `get_int()`, `get_bool()`, `get_path()`. No module calls `os.getenv()` directly — this is enforced by the CS-2 audit.

Server-side env vars are documented in `devtrack_server/.env_sample`. The server requires these at startup; missing vars raise `ConfigError` with the exact variable name.

Key server vars: `POSTGRES_URL` (required), `DEVTRACK_API_KEY`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`, `ADMIN_SECRET_KEY`, `LLM_PROVIDER`, `OLLAMA_HOST`, `HTTP_TIMEOUT`, `LLM_REQUEST_TIMEOUT_SECS`.

PostgreSQL is mandatory for server persistence and server-side events. SQLite belongs to the Go
client's offline source-of-truth path; remaining Python SQLite branches are migration compatibility
debt, not the final server storage mode. Server startup validates PostgreSQL connectivity and applies
Alembic migrations before accepting requests. TASK-141 removed the final direct `sqlite3` import
from production server code.

## Client-Server Boundary

The Go client communicates with this server exclusively over HTTPS. There is no shared code, module, or compiled artefact between them. Full contract: `docs/ARCHITECTURE.md`.

Auth for client→server calls: `X-DevTrack-API-Key` header (value from `DEVTRACK_API_KEY` env var).
Admin routes use JWT cookie auth, not the API key.
