# DevTrack Monorepo Split Manifest

_Generated: 2026-05-24 | TASK-041 | Branch: features/SPLIT-001-monorepo-restructure_

This document catalogues every file and directory in the monorepo root and assigns each
to one of five owners:

- **CLIENT** — belongs in `devtrack_client` (Go binary + bundled git-sage Python tool)
- **SERVER** — belongs in `devtrack_server` (Python LLM pipeline, admin GUI, integrations)
- **BOTH** — genuinely needed by both repos (with notes on what changes per codebase)
- **WIKI** — already separated into `devtrack_wiki/`; classified with source-of-truth note
- **DELETE** — dead code, migration artefacts, or superseded files

Excluded from this audit per spec: `.git/`, `devtrack_wiki/`, `Data/`, `.claude/`.

---

## 1. Top-Level File and Directory Table

| Path | Owner | Notes |
|---|---|---|
| `devtrack-bin/` | CLIENT | Full Go source tree — see Section 2 |
| `backend/` | SERVER (mostly) | Python backend — see Section 3; `backend/git_sage/` is CLIENT |
| `docs/` | BOTH | Internal dev docs; keep a copy in each repo (content same, no code) |
| `devtrack_wiki/` | WIKI | Public website + wiki; GitLab remote is source of truth for wiki pages |
| `ci/` | SERVER | Contains `devtrack_server.gitlab-ci.yml`; belongs with server repo |
| `scripts/` | BOTH | Mix of client and server test/utility scripts — see Section 4 |
| `.github/workflows/` | CLIENT | `ci.yml`, `release.yml`, `bump-version.yml`, `sync-gitlab.yml` all drive the Go client |
| `infra/` | SERVER | Infrastructure helpers (ping-worker); server deployment concern |
| `demo/` | DELETE | Demo scripts tied to old monolith structure; superseded |
| `bin/` | DELETE | Pre-built binary artefacts (`devtrack`, `devtrack-git`); not source-controlled output |
| `CLAUDE.md` | BOTH | Root CLAUDE.md spans both codebases; each codebase gets its own trimmed copy |
| `README.md` | BOTH | Root README to be updated to describe three-codebase layout; each repo gets its own |
| `pyproject.toml` | SERVER | Python project manifest; client uses `go.mod` only |
| `uv.lock` | SERVER | uv lockfile for Python deps; server only |
| `python_bridge.py` | DELETE | Legacy bridge at repo root; superseded by `backend/webhook_server.py` |
| `docker-compose.yml` | SERVER | Server backing services: PostgreSQL is required for persistence and server-side events; MongoDB remains optional as a Teams voice-learning source. |
| `Dockerfile` | SERVER | Builds Python backend image |
| `Dockerfile.server` | SERVER | Alternate/named server Dockerfile |
| `devtrack-server` | SERVER | `devtrack-server` CLI script (Python server management binary) |
| `LICENSE` | BOTH | MIT licence; copy to both repos unchanged |
| `TERMS.md` | BOTH | Terms of service; copy to both repos unchanged |
| `Makefile` | BOTH | Top-level convenience targets; each repo should have its own minimal Makefile |
| `uninstall.sh` | CLIENT | Uninstall script for the binary; client-side |
| `workspaces.yaml` | CLIENT | Workspace config consumed by Go daemon |
| `workspaces.yaml.example` | CLIENT | Example workspace config for users |
| `workspaces.yaml.sample` | CLIENT | Sample workspace config |

---

## 2. `devtrack-bin/` — All Files Are CLIENT

Every file in `devtrack-bin/` belongs in `devtrack_client/`. The Go module is already named
`gitlab.com/devtrack3_cloud/devtrack_client` (confirmed in `go.mod`).

| File | Owner | Notes |
|---|---|---|
| `main.go` | CLIENT | Entry point; CLI dispatch |
| `cli.go` | CLIENT | All CLI command implementations |
| `cli_boardroom.go` | CLIENT | `devtrack boardroom` command; calls `/trigger/boardroom` on server |
| `cli_plan.go` | CLIENT | `devtrack plan` command; calls `/trigger/plan/*` on server |
| `cli_unix.go` | CLIENT | Unix-only CLI implementations (signals, force-trigger) |
| `cli_windows.go` | CLIENT | Windows stubs for Unix-only CLI ops |
| `cli_vacation.go` | CLIENT | Vacation mode CLI |
| `cli_work.go` | CLIENT | Work session CLI |
| `cli_workspace.go` | CLIENT | Workspace management CLI |
| `cloud.go` | CLIENT | Cloud/managed mode helpers |
| `config.go` | CLIENT | YAML config struct |
| `config_env.go` | CLIENT | All `.env` key accessors for Go — single source of truth for env var names |
| `daemon.go` | CLIENT | Daemon lifecycle management |
| `daemon_unix.go` | CLIENT | Unix-specific daemon ops (fork, flock) |
| `daemon_windows.go` | CLIENT | Windows daemon stubs |
| `daemon.log` | DELETE | Stale log file accidentally committed; should be gitignored |
| `database.go` | CLIENT | SQLite via modernc.org/sqlite; local trigger history |
| `deferred_commit.go` | CLIENT | Deferred commit cache logic |
| `demo.go` | CLIENT | Demo/smoke-test helpers for CLI |
| `deps.go` | CLIENT | Dependency injection helpers |
| `devtrack.ico` | CLIENT | Windows binary icon; linked via goversioninfo |
| `fs_watcher.go` | CLIENT | fsnotify-based filesystem watcher |
| `git_monitor.go` | CLIENT | Git repository watcher; fires commit_trigger |
| `gitsage/` | CLIENT | Go-native git-sage experiment (agent.go, context.go, llm.go); early prototype — keep with client |
| `go-cli/` | DELETE | Empty directory (no files found); migration artefact |
| `go.mod` | CLIENT | Go module manifest; module name `gitlab.com/devtrack3_cloud/devtrack_client` |
| `go.sum` | CLIENT | Go dependency lockfile |
| `health.go` | CLIENT | Health check client helpers |
| `http_api.go` | CLIENT | Internal HTTP control API server (for `devtrack reload-config`) |
| `http_trigger.go` | CLIENT | HTTP trigger client — sends HTTPS POSTs to server |
| `http_trigger_test.go` | CLIENT | Tests for http_trigger.go |
| `infra.go` | CLIENT | Infrastructure mode helpers |
| `integrated.go` | CLIENT | `IntegratedMonitor` — wires git monitor, scheduler, IPC server |
| `ipc.go` | CLIENT | TCP IPC server (Go side); legacy internal channel |
| `learning.go` | CLIENT | Personalized AI learning consent/profile management (Go side) |
| `license.go` | CLIENT | License tier management |
| `loadenv.go` | CLIENT | `.env` loader at startup |
| `lock_unix.go` | CLIENT | Unix single-instance lock (flock) |
| `lock_windows.go` | CLIENT | Windows single-instance lock (LockFileEx) |
| `main.go` | CLIENT | (listed above) |
| `migrations.go` | CLIENT | SQLite migration runner |
| `ping.go` | CLIENT | Telemetry/liveness ping |
| `process_unix.go` | CLIENT | Unix process management |
| `process_windows.go` | CLIENT | Windows process management |
| `queue.go` | CLIENT | Trigger queue logic |
| `resource_windows_amd64.syso` | CLIENT | Pre-built Windows resource object (icon + version); `_windows_amd64` suffix constrains linkage |
| `scheduler.go` | CLIENT | Cron-based periodic trigger (robfig/cron) |
| `server_config.go` | CLIENT | Server connection config struct |
| `setup.go` | CLIENT | Mode selection wizard |
| `test_manual_triggers.sh` | CLIENT | Manual trigger test script |
| `tls_cert.go` | CLIENT | TLS certificate helpers for HTTPS connection to server |
| `tui.go` | CLIENT | TUI framework entry (Bubbletea/Lipgloss) |
| `tui_activity.go` | CLIENT | TUI activity panel |
| `tui_alerts.go` | CLIENT | TUI alerts panel |
| `tui_overview.go` | CLIENT | TUI overview panel |
| `tui_workspaces.go` | CLIENT | TUI workspaces panel |
| `uninstall.go` | CLIENT | Uninstall command implementation |
| `uninstall_unix.go` | CLIENT | Unix uninstall helpers |
| `uninstall_windows.go` | CLIENT | Windows uninstall helpers |
| `upgrade.go` | CLIENT | In-place binary upgrade logic |
| `upgrade_unix.go` | CLIENT | Unix upgrade helpers |
| `upgrade_windows.go` | CLIENT | Windows upgrade helpers |
| `version.go` | CLIENT | Version constants and build info |
| `versioninfo.json` | CLIENT | goversioninfo metadata for Windows binary |

---

## 3. `backend/` — File-by-File Classification

`backend/git_sage/` is CLIENT (git-sage is a local tool bundled with the client).
Everything else in `backend/` is SERVER unless noted.

### `backend/` root files

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `webhook_server.py` | SERVER | Primary Python entry point; FastAPI server |
| `webhook_handlers.py` | SERVER | Routes Azure/GitHub/Jira events |
| `config.py` | SERVER | Centralised Python config; all modules use `backend.config.get()` |
| `ipc_client.py` | SERVER | TCP IPC client (Python side); legacy internal channel |
| `llm_task_parser.py` | SERVER | Strict configured-provider task enrichment; no ticket routing inference |
| `description_enhancer.py` | SERVER | Ollama-based description enhancement |
| `commit_message_enhancer.py` | SERVER | AI commit message refinement pipeline |
| `git_diff_analyzer.py` | SERVER | Staged-change analysis for commit context |
| `conflict_auto_resolver.py` | SERVER | Automatic merge conflict resolution |
| `work_update_enhancer.py` | SERVER | Git context injection into work updates |
| `user_prompt.py` | SERVER | Terminal TUI for interactive work-update prompts |
| `daily_report_generator.py` | SERVER | AI-enhanced report generation (Terminal/HTML/Markdown/JSON) |
| `email_reporter.py` | SERVER | Report delivery via email/Teams |
| `task_matcher.py` | SERVER | Fuzzy + semantic task matching |
| `personalization.py` | SERVER | `inject_style()` personalisation entry point |
| `personalized_ai.py` | SERVER | AI learning from user communications |
| `learning_integration.py` | SERVER | Learning consent management |
| `data_collectors.py` | SERVER | MS Graph Teams data collection |
| `run_daily_learning.py` | SERVER | Daily learning cron entrypoint |
| `plan_parser.py` | SERVER | Parses free-text/Markdown plan into structured sections |
| `workspace_router.py` | SERVER | Routes requests to correct workspace backend |
| `webhook_notifier.py` | SERVER | Outbound webhook notification helpers |
| `telemetry.py` | SERVER | Anonymous telemetry ping |
| `alert_poller.py` | SERVER | Background polling for GitHub/Azure/Jira ticket events |
| `alert_notifier.py` | SERVER | macOS + terminal notification delivery |
| `github_ticket_sync.py` | SERVER | GitHub issue → local SQLite sync |
| `backlog_manager.py` | SERVER | Backlog management (PM) |
| `project_manager.py` | SERVER | Project management API client |
| `pm_agent.py` | SERVER | PM agent orchestration |
| `license_manager.py` | SERVER | License tier enforcement |

### `backend/git_sage/` — CLIENT (bundled with devtrack_client)

git-sage is a local LLM-powered git agent. It runs entirely on the developer's machine
(offline-first, no server dependency) and is invoked by the Go daemon as a subprocess.
It belongs in `devtrack_client/git_sage/`.

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | CLIENT | Package marker |
| `__main__.py` | CLIENT | Entry point for `python -m backend.git_sage` |
| `agent.py` | CLIENT | Agentic loop; suggest_only mode; undo; follow-up |
| `cli.py` | CLIENT | ask/do/interactive modes; session approval dialog |
| `config.py` | CLIENT | Env-driven config; reads `.env` > `~/.config/git-sage/config.json` |
| `context.py` | CLIENT | Git repository state collection and formatting |
| `git_operations.py` | CLIENT | Advanced git operations (300+ lines) |
| `conflict_resolver.py` | CLIENT | Conflict analysis and resolution strategies (280+ lines) |
| `pr_finder.py` | CLIENT | PR/MR metadata extraction (220+ lines) |
| `llm.py` | CLIENT | Ollama + OpenAI-compatible backends; json_mode enforcement |
| `setup.py` | CLIENT | Package setup stub |
| `README.md` | CLIENT | git-sage usage docs; ships with client |

### `backend/boardroom/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `personas.py` | SERVER | 7 persona definitions and LLM prompt templates |
| `session.py` | SERVER | Multi-persona review orchestration |
| `interactive.py` | SERVER | Post-review chat loop |
| `report.py` | SERVER | SWOT matrix and verdict rendering |

### `backend/llm/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `base.py` | SERVER | LLM provider base class |
| `provider_factory.py` | SERVER | Builds fallback chain: primary → OpenAI/Anthropic → Ollama |
| `ollama_provider.py` | SERVER | Ollama provider implementation |
| `openai_provider.py` | SERVER | OpenAI-compatible provider |
| `anthropic_provider.py` | SERVER | Anthropic provider |
| `groq_provider.py` | SERVER | Groq provider |

### `backend/admin/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `__main__.py` | SERVER | Admin process entry point |
| `app.py` | SERVER | FastAPI admin application |
| `auth.py` | SERVER | JWT cookie auth |
| `routes.py` | SERVER | All admin routes (users, licenses, audit, server status) |
| `server_status.py` | SERVER | Process health aggregation |
| `user_manager.py` | SERVER | User CRUD and password management |
| `static/` | SERVER | Admin UI static assets (CSS, JS) |
| `templates/` | SERVER | Jinja2 HTML templates for admin pages |

### `backend/tests/` — SERVER

All tests belong to the server package. The Go client has its own `*_test.go` files.

| File | Owner | Notes |
|---|---|---|
| `conftest.py` | SERVER | pytest setup; adds repo root to sys.path |
| `test_admin_*.py` | SERVER | Admin console tests |
| `test_backlog_manager.py` | SERVER | Backlog management tests |
| `test_config.py` | SERVER | Config function tests |
| `test_create_tasks.py` | SERVER | AI task creation tests |
| `test_description_enhancer.py` | SERVER | Description enhancer tests |
| `test_http_triggers.py` | SERVER | HTTP trigger endpoint tests |
| `test_inbound_webhooks.py` | SERVER | Inbound webhook handler tests |
| `test_integrations.py` | SERVER | Integration tests |
| `test_jira_*.py` | SERVER | Jira client and alerter tests |
| `test_license_manager.py` | SERVER | License manager tests |
| `test_llm_providers.py` | SERVER | LLM provider tests |
| `test_llm_task_parser.py` | SERVER | Structured schema, invalid response, provider failure, and raw fallback tests |
| `test_pm_agent.py` | SERVER | PM agent tests |
| `test_pr_analyzer.py` | SERVER | PR analyser tests |
| `test_project_*.py` | SERVER | Project management tests |
| `test_server_tui.py` | SERVER | Server TUI tests |
| `test_ticket_cache.py` | SERVER | Ticket cache SQLite tests |
| `test_user_prompt.py` | SERVER | User prompt TUI tests |
| `test_work_tracker.py` | SERVER | Work tracker tests |
| `test_workspace_*.py` | SERVER | Workspace config and router tests |

### `backend/db/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `models.py` | SERVER | SQLAlchemy/dataclass models |
| `models/` | SERVER | Model sub-package |
| `learning_store.py` | SERVER | Learning data persistence |
| `mongo_alerts.py` | SERVER | MongoDB alerts store |
| `mongo_learning.py` | SERVER | MongoDB learning store |
| `platform_store.py` | SERVER | PM platform integration store |
| `project_store.py` | SERVER | Project/backlog SQLite store |
| `ticket_db.py` | SERVER | Ticket cache SQLite access |

### `backend/jira/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `client.py` | SERVER | Jira REST API client |

### `backend/github/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `check.py` | SERVER | GitHub connectivity check |
| `client.py` | SERVER | GitHub API client |
| `ghAnalysis.py` | SERVER | Repository insights |
| `list_items.py` | SERVER | Issue/PR listing |
| `pr_analyzer.py` | SERVER | PR analysis |
| `run_sync.py` | SERVER | Sync runner |
| `sync.py` | SERVER | Issue sync logic |
| `view_item.py` | SERVER | Issue/PR viewer |

### `backend/azure/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `assignment_poller.py` | SERVER | Azure DevOps assignment polling |
| `check.py` | SERVER | Azure connectivity check |
| `client.py` | SERVER | Azure DevOps REST API client |
| `list_items.py` | SERVER | Work item listing |
| `run_sync.py` | SERVER | Sync runner |
| `sync.py` | SERVER | Work item sync logic |
| `view_item.py` | SERVER | Work item viewer |

### `backend/gitlab/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `assignment_poller.py` | SERVER | GitLab assignment polling |
| `check.py` | SERVER | GitLab connectivity check |
| `client.py` | SERVER | GitLab API client |
| `list_items.py` | SERVER | Issue listing |
| `run_sync.py` | SERVER | Sync runner |
| `sync.py` | SERVER | Issue sync |
| `view_item.py` | SERVER | Issue viewer |

### `backend/rag/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `embedder.py` | SERVER | Ollama `/api/embed` calls |
| `sample_indexer.py` | SERVER | High-level RAG API |
| `vector_store.py` | SERVER | ChromaDB persistent collection wrapper |

### `backend/msgraph_python/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `chat_analyzer.py` | SERVER | Teams chat analysis |
| `config.cfg` | SERVER | MS Graph config |
| `graph.py` | SERVER | MS Graph API client |
| `main.py` | SERVER | MS Graph entry point |
| `sentiment_analysis.py` | SERVER | Sentiment analysis via MS Graph |

### `backend/server_tui/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `__main__.py` | SERVER | Textual TUI entry point |
| `app.py` | SERVER | Textual application definition |
| `health_client.py` | SERVER | Health endpoint client |
| `log_viewer.py` | SERVER | Log viewer panel |
| `process_monitor.py` | SERVER | Process monitor panel |
| `stats_client.py` | SERVER | Stats panel client |

### `backend/alerters/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `azure_alerter.py` | SERVER | Azure DevOps alerter |
| `github_alerter.py` | SERVER | GitHub alerter |
| `gitlab_alerter.py` | SERVER | GitLab alerter |
| `jira_alerter.py` | SERVER | Jira alerter |

### `backend/auth/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `cloud_auth.py` | SERVER | Cloud/managed mode auth |
| `local_auth.py` | SERVER | Local auth helpers |
| `session.py` | SERVER | Auth session management |

### `backend/work_tracker/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `eod_emailer.py` | SERVER | End-of-day email delivery |
| `eod_report_generator.py` | SERVER | End-of-day report generation |
| `session_store.py` | SERVER | Work session persistence |

### `backend/vacation/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `auto_responder.py` | SERVER | Vacation auto-responder |

### `backend/slack/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `__main__.py` | SERVER | Slack bot entry point |
| `bot.py` | SERVER | Slack bot implementation |
| `handlers.py` | SERVER | Slack event handlers |
| `notifier.py` | SERVER | Slack notification delivery |

### `backend/telegram/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `__main__.py` | SERVER | Telegram bot entry point |
| `bot.py` | SERVER | Telegram bot implementation |
| `handlers.py` | SERVER | Telegram event handlers |

### `backend/models/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `backlog.py` | SERVER | Backlog data model |
| `project.py` | SERVER | Project data model |

### `backend/ai/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `create_tasks.py` | SERVER | AI-driven task creation from natural language |
| `ollama_client.py` | SERVER | Low-level Ollama HTTP client |

### `backend/utils/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `paths.py` | SERVER | Path resolution utilities |
| `text_utils.py` | SERVER | Text formatting helpers |
| `validate_env_sample.py` | SERVER | Validates `.env` keys match `.env_sample` |

### `backend/project_spec/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `__init__.py` | SERVER | Package marker |
| `developer_roster.py` | SERVER | Developer roster management |
| `project_creator.py` | SERVER | Project creation from spec |
| `spec_emailer.py` | SERVER | Project spec email delivery |
| `spec_generator.py` | SERVER | LLM-driven spec generation |
| `spec_store.py` | SERVER | Spec persistence |
| `workload_analyzer.py` | SERVER | Workload analysis |

### `backend/autodoc/` — SERVER

| File | Owner | Notes |
|---|---|---|
| `analyze_code.py` | SERVER | Auto-documentation from code analysis |

---

## 4. Supporting Directories

### `docs/` — BOTH

Internal developer documentation. Both repos should carry a copy; the monorepo version
is the source of truth during the split. After TASK-050, each repo maintains its own docs.

| Path | Owner | Notes |
|---|---|---|
| `docs/VISION.md` | BOTH | Product vision; relevant to both teams |
| `docs/ROADMAP.md` | BOTH | CS-1→CS-5 arc; relevant to both teams |
| `docs/ARCHITECTURE.md` | BOTH | Will be updated by TASK-047 to show split layout |
| `docs/CONFIGURATION.md` | BOTH | Env var reference; both need it (each trims to their own vars) |
| `docs/LAUNCH_STRATEGY.md` | BOTH | Positioning rules |
| `docs/INSTALLATION.md` | BOTH | Will be split by TASK-049 into client + server paths |
| `docs/GIT_FEATURES.md` | CLIENT | Git workflow features; client-only |
| `docs/GIT_SAGE.md` | CLIENT | git-sage usage; client-only |
| `docs/LLM_GUIDE.md` | SERVER | LLM provider setup; server-facing |
| `docs/CLI_REFERENCE.md` | CLIENT | CLI commands reference |
| `docs/HTTP_API.md` | BOTH | Created by TASK-044; the shared boundary document |
| `docs/GETTING_STARTED.md` | BOTH | Intro doc; shared |
| `docs/QUICK_START.md` | BOTH | Quick start; shared |
| `docs/TROUBLESHOOTING.md` | BOTH | Troubleshooting; shared |
| `docs/INDEX.md` | BOTH | Master doc index |
| `docs/ADVANCED_FEATURES.md` | SERVER | Advanced AI/integration features |
| `docs/AUTOSTART.md` | CLIENT | Autostart (launchd/systemd) is a client concern |
| `docs/AZURE_DEVOPS.md` | SERVER | Azure integration; server |
| `docs/GITHUB.md` | SERVER | GitHub integration; server |
| `docs/GITLAB.md` | SERVER | GitLab integration; server |
| `docs/MULTI_REPO.md` | CLIENT | Multi-repo workspace config; client concern |
| `docs/PERSONALIZATION.md` | SERVER | Personalization/RAG system; server |
| `docs/PM_AGENT.md` | SERVER | PM agent; server |
| `docs/TELEGRAM_BOT.md` | SERVER | Telegram integration; server |
| `docs/TELEMETRY_PLAN.md` | BOTH | Telemetry spans both |
| `docs/TICKET_ALERTER.md` | SERVER | Ticket alerter; server |
| `docs/TUI_FLOWS.md` | CLIENT | TUI navigation; client |
| `docs/TUI_NAVIGATION_DESIGN.md` | CLIENT | TUI design; client |
| `docs/WORK_TRACKER.md` | SERVER | Work tracker; server |
| `docs/OFFLINE_RESILIENCE.md` | CLIENT | Offline-first design principles; client |
| `docs/LLM_STRATEGY.md` | SERVER | Multi-provider LLM strategy; server |
| `docs/COMMIT_WORKFLOW_DESIGN.md` | CLIENT | Commit workflow design; client |
| `docs/GIT_COMMIT_WORKFLOW.md` | CLIENT | Git commit workflow; client |
| `docs/REFACTORING.md` | BOTH | Refactoring history; reference for both |
| `docs/VERIFICATION.md` | BOTH | Setup verification steps |
| `docs/MACOS_AUTOSTART.md` | CLIENT | macOS launchd autostart; client |
| `docs/PROJECT_PLANNING.md` | SERVER | PM planning docs; server |
| `docs/build-runner-plan.md` | DELETE | Build-runner is a separate repo (gitlab.com/devtrack3_cloud/build-runner); this doc is superseded by the BR-* tasks on the project board |
| `docs/posts/` | WIKI | Blog post stubs; move to wiki |
| `docs/split-manifest.md` | BOTH | This file; reference doc for the split |

### `ci/` — SERVER

| Path | Owner | Notes |
|---|---|---|
| `ci/devtrack_server.gitlab-ci.yml` | SERVER | Server GitLab CI pipeline; moves to `devtrack_server/.gitlab-ci.yml` |

### `.github/workflows/` — CLIENT

All four GitHub Actions workflows drive the Go client build/release cycle.

| Path | Owner | Notes |
|---|---|---|
| `.github/workflows/ci.yml` | CLIENT | Go build + test on every push |
| `.github/workflows/release.yml` | CLIENT | Tag-triggered multi-platform binary release to GitHub Releases |
| `.github/workflows/bump-version.yml` | CLIENT | Auto version bump on `devtrack-bin/` push to `main` |
| `.github/workflows/sync-gitlab.yml` | CLIENT | Mirror `dev` branch push to GitLab (no CI trigger) |

### `scripts/` — BOTH

| Path | Owner | Notes |
|---|---|---|
| `scripts/bump-version.sh` | CLIENT | Version bump helper for Go binary |
| `scripts/bundle-python.sh` | SERVER | Bundles Python backend for distribution |
| `scripts/create_project_issues.py` | SERVER | Creates GitLab/GitHub issues from a project spec |
| `scripts/setup_claude_memory.py` | DELETE | One-time Claude memory setup; not part of either product |
| `scripts/test_commit_enhancer.sh` | SERVER | Tests the Python commit enhancer |
| `scripts/test_commit_flow.sh` | CLIENT | Tests the Go commit flow end-to-end |
| `scripts/test_force_trigger.sh` | CLIENT | Tests force-trigger (Go daemon) |
| `scripts/test_integrations.sh` | SERVER | Tests Python integrations |
| `scripts/test_ipc_manual.py` | SERVER | Manual IPC test (Python side) |
| `scripts/test_preview_report.sh` | SERVER | Tests report preview (Python) |
| `scripts/verify_setup.sh` | BOTH | Overall setup verification |

### `infra/` — SERVER

| Path | Owner | Notes |
|---|---|---|
| `infra/ping-worker` | SERVER | Cloudflare/infrastructure ping worker; server deployment concern |

### `demo/` — DELETE

| Path | Owner | Notes |
|---|---|---|
| `demo/` | DELETE | Monolith-era demo scripts; tied to old structure; superseded |

### `bin/` — DELETE

| Path | Owner | Notes |
|---|---|---|
| `bin/devtrack` | DELETE | Pre-built binary output; should be gitignored, not committed |
| `bin/devtrack-git` | DELETE | Pre-built binary; same issue |

---

## 5. Shared Boundary — HTTP API Endpoints

The client communicates with the server exclusively via HTTPS POST/GET. There is no shared
code, module, or compiled artefact. The table below is extracted directly from
`devtrack-bin/http_trigger.go` (client side) and `backend/webhook_server.py` (server side).

This list defines what TASK-044 must document in `docs/HTTP_API.md`.

| Method | Path | Caller | Description |
|---|---|---|---|
| `GET` | `/health` | Go client | Server health check; called at daemon startup and on `devtrack status` |
| `GET` | `/version` | Go client | Server version string |
| `GET` | `/status` | Go client | Detailed server status (processes, features) |
| `POST` | `/trigger/commit` | Go client | Commit event — branch, commit hash, message, diff summary |
| `POST` | `/trigger/timer` | Go client | Scheduled tick — prompts work update |
| `POST` | `/trigger/workspace_reload` | Go client | Workspace config changed; server reloads routing |
| `POST` | `/trigger/shutdown` | Go client | Orderly server shutdown request |
| `POST` | `/trigger/ping` | Go client | Liveness check (lightweight) |
| `POST` | `/trigger/work_session_start` | Go client | Work session started; carries workspace + ticket context |
| `POST` | `/trigger/work_session_stop` | Go client | Work session ended |
| `POST` | `/trigger/plan/preview` | Go client | Plan decomposition preview (Epic/Story/Task); returns plan_token |
| `POST` | `/trigger/plan/create` | Go client | Execute plan creation on PM platform using plan_token |
| `POST` | `/trigger/boardroom` | Go client | Multi-persona AI plan review; returns SWOT + verdict |
| `POST` | `/trigger/boardroom/chat` | Go client | Follow-up chat turn within a boardroom session |
| `POST` | `/webhooks/azure-devops` | Azure webhook | Inbound Azure DevOps event |
| `POST` | `/webhooks/github` | GitHub webhook | Inbound GitHub event |
| `POST` | `/webhooks/gitlab` | GitLab webhook | Inbound GitLab event |
| `POST` | `/webhooks/jira` | Jira webhook | Inbound Jira event |
| `GET/POST` | `/admin/*` | Browser (admin only) | Admin console routes; not called by Go client |
| `GET` | `/spec/{spec_id}/review` | Internal | Project spec review |
| `POST` | `/spec/{spec_id}/review` | Internal | Submit spec review |

**Auth**: All `/trigger/*` endpoints validate the `X-DevTrack-API-Key` header.
The key is set by `DEVTRACK_API_KEY` env var on both client and server.

---

## 6. Go Module Name Recommendation

**Recommended module name for `devtrack_client`:**

```
gitlab.com/devtrack3_cloud/devtrack_client
```

This is already the module name in `devtrack-bin/go.mod` — no rename is required.
TASK-042 copies `devtrack-bin/` to `devtrack_client/` and the module name is preserved as-is.

The binary name (`devtrack`) is independent of the module name and does not change.

**Recommended module name for `devtrack_server`:**

The server is a Python project. Its `pyproject.toml` `name` field should be updated to
`devtrack-server` (TASK-043 spec). There is no Go module involved.

---

## 7. Summary Counts

| Owner | Count |
|---|---|
| CLIENT | 65 files in `devtrack-bin/` + 12 files in `backend/git_sage/` + 7 root/script files |
| SERVER | ~120 files across `backend/` (excluding `git_sage/`) + `ci/` + server scripts |
| BOTH | ~20 files (docs, root CLAUDE.md, README.md, LICENSE, TERMS.md, Makefile, .env_sample) |
| WIKI | `devtrack_wiki/` (excluded from audit per spec) + `docs/posts/` |
| DELETE | `bin/`, `demo/`, `python_bridge.py`, `scripts/setup_claude_memory.py`, `docs/build-runner-plan.md`, `devtrack-bin/daemon.log`, `devtrack-bin/go-cli/` |

---

_End of split manifest. All acceptance criteria for TASK-041 met._
_Next task: TASK-042 — Create `devtrack_client/` directory skeleton_
