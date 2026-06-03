# devtrack-server

Python AI pipeline for DevTrack. Receives triggers from the Go client over HTTPS, runs NLP/LLM processing, syncs work items to Azure DevOps / GitHub / GitLab / Jira, and serves the admin console.

**Version**: 1.1.0 | **Requires**: Python 3.12–3.13

---

## Role in the monorepo

```
devtrack_client (Go binary)
        │
        │  HTTPS POST /trigger/*
        │  X-DevTrack-API-Key header
        ▼
devtrack_server/backend/webhook_server.py   ← FastAPI on :8089
        │
        ├── /trigger/commit   → NLP → LLM → PM sync
        ├── /trigger/timer    → TUI prompt → work update → report
        ├── /webhooks/*       → inbound platform webhooks
        ├── /admin/*          → web admin console (JWT auth)
        ├── /boardroom        → multi-persona AI plan review
        ├── /plan             → Epic/Story/Task decomposition
        └── /health, /version
```

The server and client share no compiled artefact. The only interface is HTTP/JSON. See `docs/HTTP_API.md` for the full contract.

---

## Quick start

```bash
cd devtrack_server

# 1. Install dependencies
uv sync                    # core deps
uv sync --extra ai         # + spaCy NLP and ChromaDB RAG

# 2. Configure
cp .env_sample .env
# Edit .env — at minimum set PROJECT_ROOT, LLM_PROVIDER, DEVTRACK_API_KEY,
# ADMIN_USERNAME, ADMIN_PASSWORD, ADMIN_SECRET_KEY

# 3. Start
uv run python -m backend.webhook_server

# 4. Verify
curl http://localhost:8089/health
```

> **Windows dev**: run directly with `uv run python -m backend.webhook_server` from this directory. No shell script needed.

### Docker

```bash
docker compose up -d          # starts MongoDB, Redis, PostgreSQL + server
docker compose down
```

---

## Running tests

```bash
uv run pytest backend/tests/ -x -q               # all tests
uv run pytest backend/tests/ -x -q --ignore=backend/tests/test_nlp_parser.py  # skip spaCy
uv run pytest backend/tests/test_api_contract.py -v   # HTTP contract tests only
```

---

## Configuration

All config is read from environment variables via `backend/config.py`. No module calls `os.getenv()` directly. Missing required vars raise `ConfigError` at startup with the exact variable name.

Copy `.env_sample` to `.env` and fill in your values.

### Required at startup

| Variable | Purpose |
|---|---|
| `PROJECT_ROOT` | Absolute path to this directory |
| `DEVTRACK_API_KEY` | Auth token for `/trigger/*` requests from Go client |
| `ADMIN_USERNAME` | Admin console login username |
| `ADMIN_PASSWORD` | Admin console login password |
| `ADMIN_SECRET_KEY` | JWT signing secret (any random string, keep private) |
| `HTTP_TIMEOUT_SHORT` | Short HTTP ops timeout (seconds) |
| `HTTP_TIMEOUT` | Standard HTTP ops timeout (seconds) |
| `HTTP_TIMEOUT_LONG` | Long HTTP ops timeout (seconds) |
| `LLM_REQUEST_TIMEOUT_SECS` | LLM API call timeout (seconds) |

### LLM providers

Set `LLM_PROVIDER` to one of: `ollama` | `openai` | `anthropic` | `groq` | `lmstudio`

Configured providers with valid credentials are added as automatic fallbacks.

```bash
# Local (default)
LLM_PROVIDER=ollama
OLLAMA_HOST=http://localhost:11434
OLLAMA_MODEL=llama3

# Cloud fallbacks (optional)
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GROQ_API_KEY=gsk_...
```

### PM platform config (two-layer model)

| Layer | What it holds |
|---|---|
| `.env` | Secrets only: `GITHUB_TOKEN`, `GITLAB_PAT`, `AZURE_DEVOPS_PAT` |
| `workspaces.yaml` (client-side) | All non-secret PM config: org, username, API URL |

The server reads workspace routing from triggers sent by the Go client. `workspaces.yaml` is managed by `devtrack workspace` commands on the client.

---

## Admin console

Available at `http://localhost:8089/admin` when `ADMIN_EMBED=true` (default), or on a separate port when `ADMIN_EMBED=false`.

| Route | What it does |
|---|---|
| `GET /admin/` | Dashboard: process health, LLM info, trigger stats |
| `GET /admin/users` | Manage users, roles, API keys |
| `GET /admin/server` | LLM config, integration status |
| `GET /admin/license` | License tier and seat usage |
| `GET /admin/audit` | Audit log (last 200 events) |

Login with `ADMIN_USERNAME` / `ADMIN_PASSWORD`. Session is JWT cookie, valid for `ADMIN_SESSION_HOURS` (default 8).

---

## Key modules

| Module | Purpose |
|---|---|
| `backend/webhook_server.py` | FastAPI app, all routes, entry point |
| `backend/webhook_handlers.py` | Routes inbound platform webhook events |
| `backend/config.py` | Centralized config — the only place `os.getenv` is called |
| `backend/nlp_parser.py` | spaCy NLP: commit/user text → structured task data (optional) |
| `backend/description_enhancer.py` | Ollama description enhancement |
| `backend/llm/` | Multi-provider LLM abstraction with fallback chain |
| `backend/admin/` | Admin console (FastAPI + HTMX, JWT auth) |
| `backend/boardroom/` | Multi-persona AI plan review (7 personas, SWOT, verdict) |
| `backend/azure/` | Azure DevOps REST client |
| `backend/github/` | GitHub REST client (async aiohttp) |
| `backend/gitlab/` | GitLab REST client (async aiohttp) |
| `backend/jira/` | Jira REST client |
| `backend/telegram/` | Telegram bot (optional) |
| `backend/slack/` | Slack bot (optional) |
| `backend/server_tui/` | Terminal UI for monitoring server processes |
| `backend/work_tracker/` | Work session tracking and EOD report generation |
| `backend/alert_poller.py` | Background polling for ticket assignments and comments |
| `backend/rag/` | ChromaDB RAG for personalization (optional, `--extra ai`) |
| `backend/db/` | Database models (SQLite primary, MongoDB/PostgreSQL optional) |

---

## Observability

The server uses [runtime-narrative](https://pypi.org/project/runtime-narrative/) for structured JSON logging. Every request stage emits a `StoryStarted` / `StagCompleted` / `FailureOccurred` event to `Data/logs/narrative.log`.

LLM failures trigger `OllamaFailureAnalyzer` which queries a small local model for a diagnosis and fix suggestion.

```bash
# Stream live events
tail -f Data/logs/narrative.log | python -m json.tool

# Enable console renderer in dev (add to .env)
NARRATIVE_RENDERER=console
PYTHONIOENCODING=utf-8   # required on Windows
```

See `docs/RUNTIME_NARRATIVE.md` for the full event schema.

---

## Optional services

Start infrastructure with Docker Compose when needed:

```bash
docker compose up -d mongodb    # personalization, alert persistence
docker compose up -d redis      # caching (future)
docker compose up -d postgres   # multi-user ticket cache
```

All are optional. The server degrades to SQLite and in-memory storage when they are absent.

---

## Package extras

```bash
uv sync --extra ai           # spaCy NLP + ChromaDB RAG
uv sync --extra openai       # OpenAI provider
uv sync --extra anthropic    # Anthropic provider
uv sync --extra cloud        # OpenAI + Anthropic
uv sync --extra mongodb      # motor async MongoDB driver
uv sync --extra postgres     # psycopg2 PostgreSQL driver
uv sync --extra notifications # desktop notifications (plyer)
```
