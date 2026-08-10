# CLAUDE.md

This file guides Claude Code (claude.ai/code) when working in this repository.

> **Source of truth for product direction:** [`PRODUCT_BIBLE.md`](PRODUCT_BIBLE.md) (pivot 2026-06-10).
> DevTrack is a silent background AI layer that absorbs developer meta-work — ticket
> updates, EOD reports, PR review cycles — by watching commits and inferring the rest.
> Current build arc (Phase 0 → 8) and what's shipped: `Data/agent_logs/project_board.md`.
> This file describes **how the code is built**; the Bible describes **where it's going**.

## Project Overview

**DevTrack** monitors Git activity and scheduled timers, enriches the signal with AI, and
routes updates to project-management systems (Azure DevOps, GitHub, GitLab, Jira). It is
offline-first: the core loop runs on Ollama + SQLite with no internet required.

## Codebase Map

Monorepo with three independent codebases sharing one HTTP/JSON boundary:

| Directory | Language | Role |
|---|---|---|
| `devtrack_client/` | Go | **CLIENT** — binary, git monitor, scheduler, CLI, PM connectors, Go-native gitsage, alerts, Telegram bot |
| `devtrack_server/` | Python | **SERVER** — AI pipeline, webhook server, NLP, LLM, admin UI, integrations, personalization |
| `devtrack_wiki/` | HTML/Markdown | **DOCS** — website (Netlify → devtrack.cloud) |
| `devtrack-bin/`, root `backend/` | — | **RETIRED** in TASK-048. Deleted; do not recreate. |

- `docs/ARCHITECTURE.md` — client↔server boundary (this replaced the old `HTTP_API.md`).
- `docs/split-manifest.md` — file-ownership catalogue from the monorepo split.

## Build & Run Commands

### Go client (`devtrack_client/`)
```bash
cd devtrack_client
go build -o devtrack .     # Build binary
go test ./...              # Run Go tests
go vet ./...               # Lint
```

### Python server (`devtrack_server/`)
```bash
cd devtrack_server
uv sync                                 # Install/sync deps (uv manages the venv — never pip)
uv run pytest backend/tests/            # All Python tests
uv run pytest backend/tests/ -k name    # Filter by name
```

### Run the daemon locally
```bash
source .env            # daemon reads env from the process at startup; it does NOT reload .env
devtrack start &       # managed mode spawns webhook_server.py as a subprocess
devtrack status        # verify
devtrack logs          # recent log output
```
For persistent autostart, `devtrack autostart-install` bakes `.env` into the launchd
plist (macOS) / systemd unit (Linux).

### Testing notes
- `backend/tests/conftest.py` adds repo root to `sys.path`.
- Tests that change `LLM_PROVIDER` must call `reset_provider_cache()` before/after to avoid
  cross-test contamination.
- Tests touching `DATABASE_DIR` need an autouse `monkeypatch.setenv("DATABASE_DIR", tmp_path)`
  fixture to avoid SQLite cross-contamination.
- Python subsystems (NLP, TUI, LLM, report generator, git_sage) degrade gracefully if deps
  are missing — every optional import is `try/except` gated.

## Architecture

```
devtrack_client/ (Go)            devtrack_server/ (Python)         devtrack_wiki/
        │   HTTPS POST /trigger/* │                                  Netlify
        │ ───────────────────────▶│
Git commits / cron / CLI    NLP · LLM · Admin UI · Personalization
SQLite (offline source)     PostgreSQL server events · PM APIs · MS Graph
```

PostgreSQL is mandatory for Python server persistence and server-side events. The Go client remains
SQLite-only so observation, queueing, MCP context, and offline backlog replay do not require a server.

The only client↔server interface is **HTTPS POST to `/trigger/*`** and related endpoints
(see `docs/ARCHITECTURE.md`). There is no shared compiled artefact. The legacy TCP IPC
channel (`127.0.0.1:35893`) is retained internally but new trigger types must use HTTP.

### Go client packages (`devtrack_client/`)
The client was refactored into layered `internal/` packages (acyclic):
`config` · `db` · `health` · `learning` ← `trigger` ← `infra` ← `daemon`; plus `trigger` ← `tui`.
Phase 1–8 added `ticket` (branch → ticket-ID extraction), `match`, `reviewer` (PR review loop),
and `mcp` (JSON-RPC 2.0 stdio server).

| Package / file | Purpose |
|---|---|
| `main.go` | Entry point; routes CLI args. `git` subcommand runs Go-native via `gitsage.RunGit` |
| `cli.go`, `cli_*.go` | CLI command implementations (`start`/`stop`/`status`/`boardroom`/`plan`/…) |
| `internal/config/` | YAML + `.env` config; `config.go` (modes, workspaces), `config_env.go` (typed env accessors) |
| `internal/db/` | SQLite via modernc.org/sqlite; trigger history, task updates, notifications |
| `internal/daemon/` | Lifecycle (PID, signals, webhook-server subprocess), HTTP control API |
| `internal/infra/` | IPC server, `IntegratedMonitor`, scheduler (robfig/cron), fsnotify git monitor |
| `internal/trigger/` | HTTP trigger client, TLS cert, trigger data types |
| `internal/health/`, `internal/learning/` | Health snapshots; AI-learning consent + license |
| `internal/tui/` | Bubbletea TUI (overview, activity, alerts, workspaces) |
| `internal/alerts/`, `internal/notify/` | **Go-native** ticket alert poller + notifiers (Terminal/Telegram/Slack/OS) |
| `internal/ticket/`, `internal/match/` | Ticket-ID extraction from branch names (Phase 2) + ticket matching |
| `internal/reviewer/` | PR review loop — fix-commit-push orchestration + escalation (Phase 7) |
| `internal/mcp/`, `mcp_cmd.go` | MCP server (JSON-RPC 2.0 stdio); 6 read-only SQLite tools (Phase 8) |
| `cli_queue.go`, `cli_eod.go`, `cli_review.go` | Pending-actions queue, EOD report, PR review CLI groups |
| `internal/telegram/` | **Go-native** Telegram bot; starts with daemon when `TELEGRAM_ENABLED=true` |
| `connectors/{github,gitlab,azure}/` | **Go-native** PM connectors (list/view/sync/check) |
| `gitsage/` | **Go-native** git-sage (commit enhance, agent, conflict, git ops, PR finder) — the only git-sage |
| `versioninfo.json`, `resource_windows_amd64.syso` | Windows binary metadata/icon (`go generate` via goversioninfo) |

### Python server modules (`devtrack_server/backend/`)
Entry point: `webhook_server.py` (handles inbound webhooks **and** `/trigger/*` from the client).

| Module | Purpose |
|---|---|
| `webhook_server.py`, `webhook_handlers.py` | FastAPI server + event routing (Azure/GitHub/GitLab/Jira) |
| `config.py` | Centralized config — all modules use `get()`/`get_int()`/`get_bool()`/`get_path()`, never `os.getenv` |
| `nlp_parser.py`, `description_enhancer.py`, `task_matcher.py` | spaCy NLP, Ollama enhancement, fuzzy/semantic task matching |
| `llm/` | Multi-provider LLM (`provider_factory.py` builds fallback chain: primary → OpenAI/Anthropic/Groq → Ollama) |
| `commit_message_enhancer.py`, `git_diff_analyzer.py` | AI commit-message refinement; staged-change analysis |
| `boardroom/`, `plan_parser.py` | 7-persona plan review (`personas`/`session`/`interactive`/`report`) + plan decomposition |
| `daily_report_generator.py`, `email_reporter.py`, `user_prompt.py` | Reports (Terminal/HTML/MD/JSON), delivery, TUI prompts |
| `personalization.py`, `personalized_ai.py`, `rag/` | Style injection + RAG few-shot (ChromaDB) — see Personalization below |
| `jira/`, `github/`, `azure/`, `msgraph_python/` | External API clients (MS Graph = Teams/Outlook) |
| `admin/` | FastAPI admin UI (users/licenses/health/audit) — see Admin UI below |
| `db/`, `utils/`, `ai/`, `autodoc/` | DB models, shared helpers, low-level AI, auto-docs |

## Configuration

Config flows from environment variables + `workspaces.yaml`. **No hardcoded fallback values**
for paths or credentials — a missing/invalid var produces a clear startup error, never a
silent default. The daemon reads env from the process at startup and does not reload `.env`.

- **Go** reads via `internal/config/config_env.go` (typed accessors; panic with clear error if missing).
- **Python** reads via `backend/config.py` (`get*()`; raises `ConfigError` with the var name).
- Runtime data lives under `Data/` (`db`, `logs`, `reports`, `pids`) — paths via `DATA_DIR`, `DATABASE_DIR`, …
- LLM provider selected by `LLM_PROVIDER` (`ollama` | `openai` | `anthropic`); credentialed
  providers auto-added as fallbacks in `llm/provider_factory.py`.

### PM connector config (two-layer)
| Source | Holds |
|---|---|
| `.env` | Secrets only: `GITHUB_TOKEN`, `GITLAB_PAT`, `AZURE_DEVOPS_PAT` |
| `workspaces.yaml` | **Always required.** Non-secret PM config: `pm_org`, `pm_username`, `pm_api_url`, `skip_issues` |

Connector constructors (`pm.NewGitHubClient(ws)` etc.) take an explicit workspace and read
only secrets from env. `skip_issues: true` excludes a code-hosting workspace from issue/ticket
listing (fixes duplicate tickets on dual-platform GitHub+ADO setups).

### Required vars (12)
Must be set or startup fails:
- **Timeouts:** `IPC_CONNECT_TIMEOUT_SECS`, `HTTP_TIMEOUT_SHORT`, `HTTP_TIMEOUT`, `HTTP_TIMEOUT_LONG`
- **Hosts:** `OLLAMA_HOST`, `LMSTUDIO_HOST`
- **Model:** `GIT_SAGE_DEFAULT_MODEL`
- **Delay:** `IPC_RETRY_DELAY_MS`
- **Prompt timeouts:** `PROMPT_TIMEOUT_SIMPLE_SECS`, `PROMPT_TIMEOUT_WORK_SECS`, `PROMPT_TIMEOUT_TASK_SECS`
- **LLM:** `LLM_REQUEST_TIMEOUT_SECS`  ·  **Sentiment:** `SENTIMENT_ANALYSIS_WINDOW_MINUTES`

### Operating modes (`DEVTRACK_SERVER_MODE`, via `GetServerMode()`)
| Mode | Behaviour |
|---|---|
| `managed` (default) | Spawns `webhook_server.py` subprocess for AI features |
| `lightweight` | Git monitoring + scheduling + Go-native commands only; AI/server commands blocked |
| `external` | Daemon only; Python on a separate host via `DEVTRACK_SERVER_URL` |

PM connectors, gitsage, and alerts are Go-native and work in all modes.

## Project Status & Direction

- **Direction:** see [`PRODUCT_BIBLE.md`](PRODUCT_BIBLE.md). Build arc **Phase 0→8 COMPLETE** on `dev`.
  Post-arc queue (active): **EPIC: Managed Install** (TASK-103–108) — `devtrack setup` sparse-clones
  Python server + runs `uv sync`; daemon fallback fixed; upgrade updates server; Windows autostart
  bakes env vars; docs/INSTALLATION.md created. After that: headless orchestration (global agent
  control via MCP), voice/dialectic Tier 4 (local Hermes persona model), GitLab `IsPRApproved`.
- **Board & history:** `Data/agent_logs/project_board.md` (current tasks) and `feature_tracker.md`.
- **Shipped (v3.x):** three-codebase split (EPIC-SPLIT); client-server decoupling (Go-native
  connectors, gitsage, alerts, Telegram bot); CS-1 HTTP transport; CS-3 admin UI; boardroom + plan;
  automated release pipeline; v3.0.9 `skip_issues`; v3.0.10 Windows fixes (isatty, editor hooks,
  auto-enhance); Phases 0–8 (silent daemon, pending-actions queue, ticket extractor, silent commit,
  EOD pipeline, voice training, dialectic self-improvement, PR puppet master, MCP server);
  TASK-102 Azure `IsPRApproved` via ADO Pull Requests API (`connectors/azure/pr.go`).
- **Docs:** `docs/ARCHITECTURE.md`, `docs/CAPABILITIES_OWNERSHIP.md`,
  `docs/CLIENT_SERVER_DECOUPLING_PLAN.md`, `docs/TELEGRAM_BOT.md`, `docs/split-manifest.md`.

## Key Patterns

- **Client entry:** `main.go` routes CLI args; `git` subcommand is Go-native via `gitsage.RunGit`
  (`gitsage/commit.go`) — AI-enhanced commit/add/history + transparent pass-through, no shell
  wrapper, no Python dependency.
- **Server entry:** `webhook_server.py` is the process the client spawns (managed) or connects to
  (external). Handles inbound webhooks and `/trigger/*` from the client.
- **HTTP boundary:** no shared artefact; only HTTPS POST `/trigger/*`. New trigger types use HTTP,
  not the legacy TCP IPC.
- **git-sage ownership:** Go-native `gitsage/` is the only git-sage, client-owned. The earlier
  Python `git_sage/` was removed; it is not in `devtrack_server/`.
- **Config centralization:** Python via `backend.config`; Go via `internal/config`. Never `os.getenv`
  outside `config.py`.
- **Database access:** centralized via `db/` models — no raw SQLite in business logic.
- **Optional imports:** every Python subsystem is `try/except` gated; the server degrades gracefully.
- **Commit enhancement:** `devtrack git commit` is stateful (caches attempt count + refined versions)
  across up to 5 iterations.
- **git-sage agent:** plans → executes → observes → rolls back on failure; asks only on genuine
  ambiguity. Approval modes `auto`/`review`/`suggest-only`; up to 5 follow-ups; `undo [N]`.
- **git-sage squash:** always `git reset --soft HEAD~N && git commit` — never `git rebase -i`
  (interactive editor blocks the agent loop).
- **git-sage JSON mode:** Ollama `format:"json"`; OpenAI-compatible `response_format:{"type":"json_object"}`
  with `BadRequestError` fallback. Model names strip `provider/` prefix. Groq prefers
  `llama-3.3-70b-versatile` over `compound-beta`; uses the `openai` SDK to avoid Cloudflare 403.

## Personalization

Opt-in. Learns the user's writing voice and injects it into every generated text. Two signals,
combined via `backend/personalization.py:inject_style(prompt, context_type, query_text)` (returns
the prompt unchanged if no profile — fully graceful):

1. **Style instruction** — profile constraints (formality, length, phrases) from `PersonalizedAI.get_style_instruction()`.
2. **RAG few-shot** — semantically similar past writing via ChromaDB + `nomic-embed-text` (`rag/`).

Injection points use `context_type` ∈ {commit, description, report, task, comment}. Setup:
`ollama pull nomic-embed-text` (ChromaDB ships via `uv sync`; data in `DATA_DIR/learning/chroma/`).

**Data sources:** git history (automatic) + optional Teams via MS Graph (`TEAMS_ENABLED`). Teams
messages → MongoDB when `MONGODB_URI` set + `motor` installed; user matched by Azure AD object ID
(`consent.json:user_object_id`), not UPN. CLI: `enable-learning`, `learning-sync [--full]`,
`show-profile`, `test-response`, `learning-status`, `learning-reset`.

> Direction (`PRODUCT_BIBLE.md`): personalization evolves into local dialectic user modeling
> (SQLite FTS5 + ChromaDB, Hermes persona model) — local-first, Teams as an opt-in tier.

## Admin UI (`devtrack_server/backend/admin/`)

FastAPI admin console: JWT-cookie auth (env `ADMIN_USERNAME`/`PASSWORD`); dashboard (process
health, LLM info, license tier, trigger stats); user management (create/role/disable/reset/API
keys); license page; server/process control; audit log; HTMX partials for processes/stats.
`ADMIN_EMBED=true` mounts it on the main webhook server (port 8089) instead of a separate process.

## Ticket Alerter (Go-native)

Background poller watching GitHub, Azure DevOps, and Jira for events relevant to the developer
(assigned, comment, state change, review requested), delivering OS/terminal/Telegram/Slack
notifications. **Go-native** in the client: `internal/alerts/` (poller + per-source alerters) and
`internal/notify/` (Terminal, Telegram, Slack, OS). State + notifications persist to SQLite via
`internal/db/`. The daemon starts the poller directly — the earlier Python `alert_poller.py`
subprocess + MongoDB approach was retired in client-server decoupling Phase 2.

CLI: `devtrack alerts [--all|--clear]`. Config: `ALERT_ENABLED`, `ALERT_POLL_INTERVAL_SECS`,
`ALERT_{GITHUB,AZURE}_ENABLED`, `ALERT_NOTIFY_{ASSIGNED,COMMENTS,STATUS_CHANGES,REVIEW_REQUESTED}`.

> **Platform quirks:**
> - Azure WIQL accepts date-only precision (`2006-01-02`, not RFC3339) — `connectors/azure/list.go:ListWorkItemsChangedAfter`.
> - Azure `IsPRApproved` calls `GET {projectURL}/_apis/git/pullrequests?pullRequestId={id}&api-version=7.0` (searches across all repos in the project); vote `>= 10` = Approved — `connectors/azure/pr.go:ListPRReviewers`.
> - The Go notify constructors (`NewTelegramFromConfig`/`NewSlackFromConfig`) must return the `Notifier` interface, not the concrete type, or the poller nil-panics when the feature is disabled.

## Common Debugging Patterns

- **AI commit enhancement silently fell back:** check Ollama is running (`ollama serve`); enhancement
  failures degrade to the original message.
- **Git monitor not detecting commits:** confirm daemon is in the right repo (`devtrack status`),
  `DEVTRACK_WORKSPACE` points to it; `tail -f Data/logs/daemon.log | grep -i commit`.
- **spaCy model missing:** `uv run python -m spacy download en_core_web_sm`.
- **Tests "provider not found":** call `reset_provider_cache()` in setup/teardown when changing `LLM_PROVIDER`.
- **git-sage does nothing then says Done:** LLM returned prose, not JSON — check the `Raw:` snippet;
  for Groq use a native model name (`llama-3.3-70b-versatile`) and ensure `openai` is installed.
- **git-sage agent loops:** lower `max_steps` (default 30), `--verbose`, Ctrl+C and check `git status`.
- **IPC errors:** legacy channel; verify `IPC_HOST`/`IPC_PORT` exported; `lsof -i :35893` for stale procs.
