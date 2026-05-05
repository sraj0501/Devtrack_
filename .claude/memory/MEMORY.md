# DevTrack Project Memory

**Last Updated**: May 6, 2026
**Project Status**: GitLab migration 100% complete. Three GitLab repos live on `main`, CI passing. Active development workspace: `D:\git_apps\Devtrack_` (branch: `migration`). Next major phase: Managed Cloud Mode (Layer 4).
**Current Branch**: migration

## Project Overview

**DevTrack** — offline-first developer automation tool
- Monitors Git activity and scheduled timers
- Prompts for work updates, enriches with AI, routes to project management systems
- Focus: Workflows BEFORE and AFTER coding (not code generation)
- Architecture: `devtrack-server` (Go daemon + Python backend) + `devtrack-cli` (thin HTTP client)

## GitLab Repos (all live, CI passing)

| Repo | GitLab URL | SSH |
|---|---|---|
| devtrack_server | `gitlab.com/devtrack3_cloud/devtrack_server` | `git@gitlab.com:devtrack3_cloud/devtrack_server.git` |
| devtrack_cli | `gitlab.com/devtrack3_cloud/devtrack_cli` | `git@gitlab.com:devtrack3_cloud/devtrack_cli.git` |
| devtrack_contract | `gitlab.com/devtrack3_cloud/devtrack_contract` | `git@gitlab.com:devtrack3_cloud/devtrack_contract.git` |
| devtrack_wiki | `gitlab.com/devtrack3_cloud/devtrack_wiki` | `git@gitlab.com:devtrack3_cloud/devtrack_wiki.git` |

## Architecture at a Glance

**Three-Layer System** (post-migration):
- **Go Daemon** (`devtrack_server/devtrack-bin/`): Git monitoring, scheduling, HTTP API, TCP IPC, SQLite
- **Python Backend** (`devtrack_server/backend/`): NLP, LLM, TUI prompts, integrations, admin GUI
- **CLI Client** (`devtrack_client/`): Thin Go binary; proxies commands to daemon over HTTP (port 8765)

**Shared contract** (`contract/api.go`): HTTP route constants + request/response types — read this first for any server↔client issue.

**Primary transport (post CS-1)**: Go daemon sends triggers over HTTPS POST to `backend/webhook_server.py`. TCP IPC (`127.0.0.1:35893`) retained as legacy internal channel.

**HTTP API** (port `DEVTRACK_SERVER_HTTP_PORT`, default 8765): 9 REST endpoints, optional `X-DevTrack-Token` auth.

## Completed Phases

| Phase | Status | Key file(s) |
|---|---|---|
| 1-3: Git Workflow | Done | `commit_message_enhancer.py`, `conflict_auto_resolver.py` |
| 4: Project Management | Done | `backend/project_manager.py` |
| 4B: SQLite PM persistence | Done | `backend/db/project_store.py` |
| Personalization + RAG | Done | `backend/personalization.py`, `backend/rag/` |
| git-sage Session UX | Done | `backend/git_sage/cli.py`, `agent.py` |
| CS-1: HTTP transport | Done | `devtrack-bin/http_trigger.go`, `backend/webhook_server.py` |
| CS-2: os.getenv audit + server-TUI stats | Done | all 40+ backend modules use `backend.config` accessors |
| CS-3: Admin GUI MVP | Done | `backend/admin/` — 492 tests |
| Autostart (launchd/systemd) | Done | `devtrack-bin/cli.go` (autostart-install) |
| Anonymous telemetry ping | Done | `devtrack-bin/ping.go` |
| Jira Alerter | Done | `backend/alerters/jira_alerter.py` |
| Multi-repo PM overrides | Done | `backend/workspace_router.py` |
| SQLite alert_state fallback | Done | `backend/alert_poller.py`, `devtrack-bin/database.go` |
| Webhook Server + Alert Poller | Done | `backend/webhook_server.py`, `backend/alert_poller.py` |
| Auto-load .env at startup | Done | `devtrack-bin/loadenv.go` (`AutoLoadEnv()`) |
| Interactive setup wizard | Done | `devtrack-bin/setup.go` (`devtrack setup`) |
| CS-standalone: Managed/Lightweight/External modes | Done | `devtrack-bin/server_config.go`, `setup.go` |
| devtrack-server management CLI | Done | `devtrack-server` Bash script |
| Self-update (`devtrack upgrade`) + migrations | Done | `devtrack-bin/upgrade.go`, `migrations.go` |
| SaaS license + auth system | Done | `backend/license_manager.py`, `backend/auth/` |
| GitLab migration + CLI/server split | Done (May 2026) | `devtrack_client/`, `contract/api.go`, `http_api.go` |

## Key Files & Locations

```
devtrack_server/
  devtrack-bin/
    main.go              - Entry point
    cli.go               - All CLI commands
    http_api.go          - HTTP API server (9 routes, port 8765)
    http_trigger.go      - HTTPTriggerClient: HTTPS POST to webhook_server
    daemon.go            - Lifecycle (PID, signals, Python bridge)
    config_env.go        - Single source of truth for env var names
    loadenv.go           - AutoLoadEnv(): auto-loads .env at startup
    setup.go             - devtrack setup wizard
    upgrade.go           - devtrack upgrade self-update
    ping.go              - Anonymous install/active telemetry ping
  backend/
    webhook_server.py    - Primary Python entry point (FastAPI)
    config.py            - Centralized config (all modules use this, not os.getenv)
    personalization.py   - Global inject_style()
    workspace_router.py  - Per-workspace PM routing
    admin/               - Admin GUI (Jinja2/HTMX)
    alerters/            - Jira, GitHub, Azure alert pollers
    git_sage/            - git-sage autonomous agent subsystem
    tests/               - 502+ pytest tests

devtrack_client/
  cmd/cli/main.go        - CLI entry point
  cli_client.go          - CLIClient: all 9 HTTP methods
  go.mod                 - module: gitlab.com/devtrack3_cloud/devtrack_cli

contract/
  api.go                 - HTTP route constants + request/response types (READ FIRST for server-client issues)

.claude/agents/
  project-vision.md      - PM agent
  devtrack-engineer.md   - Engineer agent (commits via devtrack CLI)
  post-generator.md      - Weekly post generator

Data/agent_logs/         (gitignored, created at runtime)
  project_board.md       - PM↔engineer task board
  engineer_log.md        - Per-commit log
```

## Configuration Architecture

**Env-first model**: env vars must be in the process environment BEFORE the daemon starts.
- Shell: `source .env` before `devtrack start`
- Autostart: `devtrack autostart-install` bakes vars into launchd/systemd — recommended
- Go: `config_env.go:LoadEnvConfig()` reads from process environment
- Python: `backend/config.py` typed accessors — `os.getenv` banned outside `config.py`
- **CLI only needs**: `DEVTRACK_SERVER_URL`, `CLI_APP_NAME`, `DEVTRACK_VERSION` (+ optional `DEVTRACK_API_TOKEN`)

## Platform Strategy

- Development: macOS (developer's machine) / Windows (current workspace via WSL)
- Deployment: Linux (Python server hosted on Linux)
- Rule: All server-side code is Linux-first. Go binary cross-compiles for linux/darwin/windows.
- Windows native daemon: deferred (3 compile errors; WSL is the workaround) — see `project_windows_gap.md`

## Next Steps for Future Sessions

1. **Windows native support** — build-tag split `daemon_unix.go`/`daemon_windows.go`; replace `Setsid`/`SIGUSR2`; Windows Service autostart. See `project_windows_gap.md`.
2. **Webhook server integration tests** — inbound `/inbound/*` endpoints not yet tested
3. **Managed Cloud Mode (Layer 4)** — cloud API + WebSocket push from cloud to daemon; `devtrack login`; always-on Telegram. See `project_managed_mode.md`.
4. **Jira alerter wiring** — `JiraAlerter` implemented but not wired into `backend/alert_poller.py` yet
5. **Commit workflow redesign** — context-aware ticket ranking, shadow branches, approval TUI. See `project_commit_workflow.md`.
6. **Full telemetry dashboard** — SSE panel in admin console. Anon ping done; see `project_telemetry_plan.md`.

## Memory File Index

| File | Contents |
|---|---|
| `feedback_pr_target_branch.md` | MRs must target `dev`, never `main` |
| `feedback_branching_strategy.md` | Branching and merge strategy |
| `feedback_cli_never_gui.md` | CLI-only rule for the Go binary |
| `feedback_git_bypass.md` | Always use `GIT_NO_DEVTRACK=1` prefix |
| `feedback_local_first.md` | Offline-first design rule (Rule 0) |
| `feedback_no_api_keys_in_docs.md` | Never put API keys in docs |
| `feedback_no_auto_commits.md` | Never commit without explicit request |
| `feedback_wiki_gifs.md` | Wiki GIF policy |
| `reference_git_sage.md` | git-sage architecture, UX, session approval, undo |
| `reference_rag_personalization.md` | RAG system, ChromaDB, injection points |
| `reference_azure_devops.md` | Azure DevOps integration details |
| `project_cs1_validation.md` | CS-1 test coverage — 133 new tests |
| `project_cs2_config_audit.md` | CS-2: os.getenv audit + server-TUI stats |
| `project_cs3_admin_gui.md` | CS-3: Admin GUI MVP (492 tests) |
| `project_phase4b_sqlite.md` | Phase 4B: SQLite PM store |
| `project_autostart.md` | launchd/systemd env-first autostart |
| `project_jira_alerter.md` | Jira alerter (implemented, not yet wired) |
| `project_workspace_pm_overrides.md` | Per-workspace PM overrides |
| `project_alert_state_sqlite.md` | SQLite alert_state fallback |
| `project_webhook_server.md` | FastAPI webhook server + alert poller |
| `project_anon_ping.md` | Anonymous install/active ping (shipped) |
| `project_managed_mode.md` | Managed Cloud Mode architecture (next major phase) |
| `project_saas_license.md` | SaaS license tiers, auth, T&C flow (shipped) |
| `project_launch_strategy.md` | Launch positioning and channel strategy |
| `project_multi_repo.md` | Multi-repo monitoring via workspaces.yaml |
| `project_telemetry_plan.md` | Full telemetry plan (anon ping done; dashboard planned) |
| `project_tui_navigation.md` | TUI navigation design (FlowController — not yet built) |
| `project_commit_workflow.md` | Commit workflow redesign (not yet built) |
| `project_vision_roadmap.md` | Long-term vision: 4-layer roadmap |
| `project_local_agents.md` | Local agents: project-vision, devtrack-engineer, post-generator |
| `project_autoload_env.md` | AutoLoadEnv() + devtrack setup wizard |
| `project_windows_gap.md` | Windows native support gap (deferred) |
| `project_devtrack_server_cli.md` | devtrack-server Bash CLI (shipped) |
| `project_upgrade_command.md` | devtrack upgrade self-update (shipped) |
| `project_standalone_cli_mode.md` | CS-standalone: three deployment modes (shipped) |
| `ARCHITECTURE.md` | System architecture deep-dive |
| `archived_STATUS_march2026.md` | ARCHIVED: March 2026 phase status snapshot |
| `archived_project_gitlab_pm_plan.md` | ARCHIVED: GitLab integration plan (now built) |
| `archived_project_runtime_narrative.md` | ARCHIVED: runtime-narrative feasibility (deferred) |

## User Preferences & Notes

- Vision: offline-first, optional cloud — comprehensive developer automation
- Code style: follows existing patterns (check CLAUDE.md)
- No hardcoded values — all config via env vars
- Testing required before commits
