# DevTrack Project Memory

**Last Updated**: April 30, 2026
**Project Status**: Production-Ready (v2.0.12+; CS-1–CS-3 complete; standalone-CLI mode shipped; devtrack-server CLI added; repo renamed to Devtrack_; 502 tests passing)
**Current Branch**: main

## Project Overview

**DevTrack** - offline-first developer automation tool
- Monitors Git activity and scheduled timers
- Prompts for work updates, enriches with AI, routes to project management systems
- Focus: Workflows BEFORE and AFTER coding (not code generation)

## Architecture at a Glance

**Two-Layer System**:
- **Go Layer** (devtrack-bin/): Git monitoring, scheduling, HTTP trigger client, database
- **Python Layer** (backend/): NLP parsing, LLM enhancement, TUI prompts, integrations

**Primary transport (CS-1)**: Go daemon sends triggers over HTTPS POST to `backend/webhook_server.py`
TCP IPC (`127.0.0.1:35893`) is retained as a legacy internal channel only.

**Binary**: Built with `cd devtrack-bin && go build -o devtrack . && cp devtrack ..`
The `devtrack` alias points to `/Users/sraj/git_apps/personal/automation_tools/devtrack`

## Completed Phases (summary — see topic files for detail)

| Phase | Status | Key file(s) |
|---|---|---|
| 1-3: Git Workflow | Done | `commit_message_enhancer.py`, `conflict_auto_resolver.py`, `python_bridge.py` |
| 4: Project Management | Done | `backend/project_manager.py` |
| 4B: SQLite PM persistence | Done | `backend/db/project_store.py` |
| Personalization + RAG | Done | `backend/personalization.py`, `backend/rag/` |
| git-sage Session UX | Done | `backend/git_sage/cli.py`, `agent.py` |
| CS-1: HTTP transport | Done | `devtrack-bin/http_trigger.go`, `backend/webhook_server.py` |
| Autostart (launchd/systemd) | Done | `devtrack-bin/cli.go` (autostart-install) |
| Anonymous telemetry ping | Done | `devtrack-bin/ping.go` |
| Jira Alerter | Done | `backend/alerters/jira_alerter.py` |
| Multi-repo PM overrides | Done | `backend/workspace_router.py` |
| SQLite alert_state fallback | Done | `backend/alert_poller.py`, `devtrack-bin/database.go` |
| Webhook Server + Alert Poller | Done | `backend/webhook_server.py`, `backend/alert_poller.py` |
| Env-First Config Bug Fix | Done | `backend/config.py` (removed `_load_env`) |
| CS-2: os.getenv audit | Done | all 40+ backend modules use `backend.config` accessors |
| Server-TUI Stats Panel | Done | `backend/server_tui/stats_client.py` |
| Server-TUI Headless Tests | Done | `backend/tests/test_server_tui.py` (37 tests; total: 433) |
| Local Agents System | Done | `.claude/agents/devtrack-engineer.md`, `project-vision.md`, `post-generator.md` |
| CS-2 Config Audit — All Modules | Done | TASK-001–007: 50+ new config accessors, os.getenv eliminated in all modules |
| TASK-010: Docs Sync | Done | `CLAUDE.md`, `README.md` synced to CS-1 reality |
| CS-3: Admin GUI MVP | Done | TASK-011–015: route tests, role/disable, license page, trigger stats, polish+embed |
| Hardcoded values eliminated | Done | TASK-016–018: scrypt params, session cookie, routes, webhook, dashboard, audit limit |
| Auto-load .env at startup | Done | `devtrack-bin/loadenv.go` (`AutoLoadEnv()`) |
| Interactive setup wizard | Done | `devtrack-bin/setup.go` (`devtrack setup`) |
| CS-standalone: Lightweight + External modes | Done | TASK-021–024: mode wizard, `ServerModeLightweight`, capability guards, non-fatal paths |
| devtrack-server management CLI | Done | `devtrack-server` binary wraps tarball-deployed Python backend |
| Self-update + migration (`devtrack upgrade`) | Done | downloads latest release, applies versioned migrations, auto-restarts |

## Key Files & Locations

```
backend/
  ├── webhook_server.py        - Primary Python entry point (FastAPI; spawned by Go daemon)
  ├── webhook_handlers.py      - WebhookEventHandler: Azure/GitHub/Jira event routing
  ├── webhook_notifier.py      - OS + terminal notification delivery for webhook events
  ├── alert_poller.py          - Full async alert poller (GitHub + Azure + SQLite fallback)
  ├── personalization.py       - Global inject_style() — combines profile + RAG
  ├── personalized_ai.py       - Talk Like You AI engine
  ├── rag/                     - embedder.py, vector_store.py, sample_indexer.py
  ├── llm/                     - provider_factory.py, groq_provider.py
  ├── db/project_store.py      - Phase 4B: SQLite CRUD for projects/backlog/sprints
  ├── alerters/jira_alerter.py - Jira async poller (assigned/comments/status_change)
  ├── server_tui/stats_client.py - TriggerStats dataclass + get_trigger_stats()
  └── tests/
      ├── test_http_triggers.py      - 28 tests
      ├── test_admin_auth.py         - 19 tests
      ├── test_admin_user_manager.py - 33 tests
      ├── test_license_manager.py    - 33 tests
      ├── test_jira_alerter.py       - 26 tests
      └── test_server_tui.py         - 37 headless tests

devtrack-bin/
  ├── ping.go              - Anonymous telemetry ping (install + active events)
  ├── http_trigger.go      - HTTPTriggerClient: HTTPS POST to webhook_server
  ├── http_trigger_test.go - 20 Go tests for HTTPTriggerClient
  ├── main.go              - CLI entry; `sage` delegates to runGitSage()
  ├── cli.go               - All CLI commands (incl. autostart-install/uninstall/status)
  ├── cli_workspace.go     - Workspace management + install-hooks command
  ├── learning.go          - LearningCommands
  ├── loadenv.go           - AutoLoadEnv(): auto-loads .env at startup (DEVTRACK_ENV_FILE → ~/.devtrack/devtrack.conf → binary-adjacent .env)
  └── setup.go             - `devtrack setup` interactive onboarding wizard (LLM provider, credentials, workspace → .env + ~/.devtrack/devtrack.conf)

backend/git_sage/
  ├── cli.py    - Session approval, follow-up loop, history/undo
  ├── agent.py  - _run_loop(), run(), followup(), step_log, undo_step()
  ├── llm.py    - json_mode, provider prefix stripping
  └── config.py - Env-driven config

.claude/agents/
  ├── project-vision.md    - PM agent: breaks plans into tasks, dispatches engineer, tracks board
  ├── devtrack-engineer.md - Engineer agent: all commits via devtrack CLI, logs to engineer_log.md
  └── post-generator.md    - Post generation from engineer logs (dev.to, HN, LinkedIn)

Data/agent_logs/           (gitignored, created at runtime)
  ├── project_board.md     - Shared PM↔engineer task board (IN PROGRESS / PLANNED / DONE)
  └── engineer_log.md      - Per-commit + per-task log (AI enhancement quality, ticket linking)
```

## Configuration Architecture

**Env-first model**: env vars must be in the process environment BEFORE the daemon starts.
The daemon does NOT reload `.env` at runtime.

- Shell: `source .env` before `devtrack start`
- Autostart: `devtrack autostart-install` bakes vars into launchd/systemd — recommended
- Go: `config_env.go:LoadEnvConfig()` reads from process environment
- Python: `backend/config.py` typed accessors — `os.getenv` banned outside `config.py`
- 12+ required vars (timeouts, hosts, models) — missing any → startup error with clear message
- WARNING: `.env` must be a regular file, not a named pipe/FIFO — use `cp .env_sample .env`

## Platform Strategy

- Development: macOS (developer's machine)
- Primary deployment: Linux (Python server hosted on Linux)
- Rule: All server-side code is Linux-first. No macOS-specific assumptions in any server_tui or backend code.
- Go binary: already cross-platform; not affected by this rule.

## Session: April 5–6, 2026

- CS-2 config audit complete (TASK-001–007): eliminated os.getenv in all 40+ backend modules; added 50+ typed config accessors to `backend/config.py`; all 433 tests pass
- Server-TUI stats panel shipped (TASK-008): `backend/server_tui/stats_client.py` with `TriggerStats` dataclass and `get_trigger_stats()`
- Server-TUI headless tests added (TASK-009): 37 tests in `backend/tests/test_server_tui.py`; test total reached 433
- Docs synced to CS-1 reality (TASK-010): `CLAUDE.md` and `README.md` updated to reflect HTTP transport as primary path
- Stale `_load_env` removed from `python_bridge.py` (was calling removed function)
- Local agents system introduced: `project-vision` (PM), `devtrack-engineer` (engineer), `post-generator` agents in `.claude/agents/`
- PM↔engineer coordination protocol: shared project board at `Data/agent_logs/project_board.md`, per-commit engineer log at `Data/agent_logs/engineer_log.md`
- No-direct-push-to-main rule enforced in both PM and engineer agents
- PR #65/#67 (features/TASK-009-server-tui-tests → dev) and PR #66/#68 (dev → main) merged; branch fully landed on main

## Session: April 10, 2026

- CS-3 Admin GUI MVP complete (TASK-011–015):
  - TASK-011: 31 HTTP-level route tests in `backend/tests/test_admin_routes.py`
  - TASK-012: User role update + soft-disable/enable routes; idempotent `disabled` column migration
  - TASK-013: `/admin/license` page surfacing license_manager data; dashboard license tier stat card
  - TASK-014: Trigger stats HTMX panel on dashboard; `/admin/_partials/stats` route with 30s auto-refresh
  - TASK-015: Password reset route; ADMIN_EMBED single-process mode; config accessor + .env_sample; docs updated
- Test total: 492 passed (was 433)
- PRs: #69 (TASK-011), #70 (TASK-012), #71 (TASK-013), #72 (TASK-014), #73 (TASK-015) — all target main

## Session: April 23, 2026

- **TASK-016**: Eliminated hardcoded scrypt params and session cookie `max_age` from admin GUI — now env-var driven
- **TASK-017**: Eliminated hardcoded values in admin routes, webhook handler, and dashboard (medium-severity)
- **TASK-018**: Eliminated low-severity hardcoded audit limit and license email from admin GUI
- **Auto-load `.env` at daemon startup** (`devtrack-bin/loadenv.go`): `AutoLoadEnv()` runs before any command; resolution order: `DEVTRACK_ENV_FILE` env var → `~/.devtrack/devtrack.conf` → `.env` next to binary; never overwrites existing env vars
- **Interactive setup wizard** (`devtrack-bin/setup.go`): `devtrack setup` walks new users through LLM provider selection, credentials, workspace path; generates `.env` and writes `~/.devtrack/devtrack.conf` so future starts are automatic
- **Test fix**: `TestProjectManager` now uses `isolate_db` autouse fixture with `monkeypatch.setenv("DATABASE_DIR", str(tmp_path))` to prevent stale data pollution across test runs (commit `c1c05fa`)
- **Bumped to v2.0.0** (`e379dd2`)
- Test total: 502 passed (was 492; 1 previously failing test fixed + new tests)
- PR #79: `features/loadEnvs` → `main` opened (incorrectly targeted `main` directly — developer will manually re-route through `dev`)
- **PR target rule reinforced**: `project-vision.md` agent directive updated to mandate `--base dev` on all PRs; `feedback_pr_target_branch.md` memory updated with incident context
- **Docs synced** (docu-agent run): wiki new SETUP_WIZARD page + v2.0.1-dev WHATS_NEW section; README updated with `devtrack setup` as recommended first-run path; `project_autoload_env.md` memory created

## Session: April 24–30, 2026

- **TASK-021**: Mode selection wizard in `setup.go` — prompts user for Managed/Lightweight/External mode before `detectProjectRoot()`. Lightweight + External modes skip the Python backend check; `.env` gets `DEVTRACK_SERVER_MODE` set accordingly. Commit: `fd208f6`
- **TASK-022**: `ServerModeLightweight` constant added to `server_config.go`; `IsExternalServer()` returns `true` for lightweight; daemon skips Python spawn in lightweight mode. Commit: `744acd2`
- **TASK-023**: `requiresManagedMode()` helper added to `cli.go`; 28 backend-dependent handlers (learning, report, azure, gitlab, github, server/admin) guarded with clear error in lightweight mode. Commit: `0cde877`
- **TASK-024**: `GetEmailReporterPath()`, `GetLearningDailyScriptPath()`, `GetPythonBridgePath()` changed from `os.Exit` to `(string, error)` return; all callers updated. Commit: `4de127b`
- **PR #82** merged (`features/standalone-cli-mode → main`) covering all 4 TASK-02x tasks
- **devtrack-server CLI** (`feat(server)`: `8196279`): `devtrack-server` management CLI added — wraps the tarball-deployed server with start/stop/status/logs commands
- **Self-update + migration** (`feat(upgrade)`: `214d78a`): `devtrack upgrade` command downloads latest release binary, applies versioned migrations, auto-restarts daemon
- **Setup hardening**: git prereq check, auto-generate `ADMIN_SECRET_KEY`, `workspaces.yaml` written to XDG home, shell integration written automatically. Commits: `491ccd3`, `4ddf157`, `9904c43`
- **Repo rename**: `automation_tools` → `Devtrack_` (GitHub + all CLAUDE.md/README references). Commit: `6b66641`
- **setup_local.sh removed**: consolidated on `devtrack-server` tarball. Commit: `1ae7966`
- **Next task ID**: TASK-025

## Next Steps for Future Sessions

1. **Windows native support** — Go daemon does not compile for Windows (3 hard errors: `Setsid`, `SIGUSR2` ×3); needs build-tag split into `daemon_unix.go`/`daemon_windows.go`, signal replacement with named pipe/HTTP, and Windows Service autostart. See `project_windows_gap.md`. WSL (linux_amd64) is the current workaround.
2. **Webhook server integration tests** — inbound webhook event endpoints at `/inbound/*` not yet tested; add pytest tests
3. **CS-4: Managed SaaS** — cloud infra + billing; next CS phase after CS-3
4. **`GetPythonBridgePath` dead code** — no callers remain after TASK-024; safe to remove in a follow-up cleanup
5. **`handleWork()` sub-commands** — `work report` sub-command calls Python internally but was not guarded in TASK-023; deferred per spec, needs follow-up

## Memory File Index

| File | Contents |
|---|---|
| `feedback_pr_target_branch.md` | Always raise PRs to `dev`, never `main` |
| `feedback_branching_strategy.md` | Branching and merge strategy |
| `feedback_cli_never_gui.md` | CLI-only rule for the Go binary |
| `feedback_git_bypass.md` | Never bypass git hooks |
| `feedback_local_first.md` | Offline-first design rule |
| `feedback_no_api_keys_in_docs.md` | Never put API keys in docs |
| `feedback_no_auto_commits.md` | Never commit without explicit request |
| `feedback_wiki_gifs.md` | Wiki GIF guidelines |
| `reference_git_sage.md` | git-sage architecture, UX, session approval, undo |
| `reference_rag_personalization.md` | RAG system, ChromaDB, injection points |
| `reference_azure_devops.md` | Azure DevOps integration details |
| `project_cs1_validation.md` | CS-1 test coverage, bugs fixed (April 1, 2026) |
| `project_cs2_config_audit.md` | CS-2: os.getenv audit + server-TUI stats + 433 tests |
| `project_cs3_admin_gui.md` | CS-3: Admin GUI MVP — route tests, role/disable, license page, trigger stats, password reset, ADMIN_EMBED (492 tests) |
| `project_phase4b_sqlite.md` | Phase 4B: SQLite PM store for projects/backlog/sprints |
| `project_autostart.md` | launchd/systemd env-first autostart implementation |
| `project_jira_alerter.md` | Jira alerter: assigned/comments/status_change polling |
| `project_workspace_pm_overrides.md` | Per-workspace PM overrides wired to platform APIs |
| `project_alert_state_sqlite.md` | SQLite alert_state fallback for delta tracking |
| `project_webhook_server.md` | FastAPI webhook server + alert poller |
| `project_anon_ping.md` | Anonymous install/active telemetry ping |
| `project_runtime_narrative.md` | Runtime narrative middleware |
| `project_managed_mode.md` | Managed mode design |
| `project_saas_license.md` | SaaS license tiers |
| `project_launch_strategy.md` | Launch strategy |
| `project_multi_repo.md` | Multi-repo monitoring plan |
| `project_telemetry_plan.md` | Telemetry / anon ping plan |
| `project_tui_navigation.md` | TUI navigation patterns |
| `project_commit_workflow.md` | Git commit workflow |
| `project_gitlab_pm_plan.md` | GitLab PM integration |
| `project_vision_roadmap.md` | Long-term vision |
| `project_local_agents.md` | Local agents system: project-vision (PM), devtrack-engineer, post-generator |
| `project_autoload_env.md` | AutoLoadEnv() resolution order, setup wizard, ~/.devtrack/devtrack.conf |
| `project_windows_gap.md` | Windows native support gap: compile errors, what needs build-tag splitting, WSL workaround |
| `project_devtrack_server_cli.md` | devtrack-server Bash CLI: install/setup/start/stop/upgrade for tarball-deployed Python backend |
| `project_upgrade_command.md` | devtrack upgrade: GitHub release download, sudo cp fallback, versioned migrations, auto-restart |
| `project_standalone_cli_mode.md` | CS-standalone: Managed/Lightweight/External modes, capability guards, XDG home, shell integration |
| `ARCHITECTURE.md` | System architecture deep-dive |
| `STATUS.md` | Detailed phase status |

## User Preferences & Notes

- Vision: offline-first, optional cloud — comprehensive developer automation
- Code style: follows existing patterns (check CLAUDE.md)
- No hardcoded values — all config via env vars
- Testing required before commits
