---
name: project_cs2_config_audit
description: CS-2 config audit — eliminate all os.getenv violations across Python backend + server-TUI stats panel + 433 passing tests
type: project
---

# CS-2: Config Audit + Server-TUI Stats

**Session**: April 5, 2026
**Branch**: features/TASK-008-cs2-stats
**Commits**: TASK-001 through TASK-009 (9 commits)

## Why:

The env-first config refactor (CS-1) established that all Python config must flow through `backend/config.py` typed accessors. CS-2 completed the audit: dozens of modules still called `os.getenv()` directly, bypassing validation and error messaging. This session eliminated every violation and added a stats panel to the server TUI.

## What Was Done:

### TASK-001 — Missing config accessors + .env_sample entries
- Added all missing typed accessor functions to `backend/config.py`
- Added corresponding entries to `.env_sample` so new deployments get a complete template
- New accessors cover: auth tokens, GitLab config, GitHub config, Azure config, Slack/Telegram bot tokens, RAG/embedding settings, telemetry vars, work tracker settings, project spec emailer settings

### TASK-002 — backend/azure/ + data_collectors.py
- `backend/azure/client.py`, `assignment_poller.py`, `check.py`, `list_items.py`, `run_sync.py`, `sync.py`
- `backend/data_collectors.py`

### TASK-003 — backend/github/ + related
- `backend/github/client.py`, `check.py`, `ghAnalysis.py`, `pr_analyzer.py`, `sync.py`
- `backend/commit_message_enhancer.py`, `backend/git_diff_analyzer.py`
- `backend/git_sage/agent.py`

### TASK-004 — backend/gitlab/
- `backend/gitlab/client.py`, `assignment_poller.py`, `check.py`, `sync.py`

### TASK-005 — backend/admin/ + backend/server_tui/
- `backend/admin/app.py`, `auth.py`, `routes.py`, `server_status.py`, `user_manager.py`
- `backend/server_tui/health_client.py`, `log_viewer.py`, `app.py`

### TASK-006 + TASK-007 — Remaining modules
- `backend/auth/cloud_auth.py`, `local_auth.py`
- `backend/rag/embedder.py`
- `backend/slack/__main__.py`, `handlers.py`, `notifier.py`
- `backend/telegram/bot.py`, `handlers.py`
- `backend/telemetry.py`
- `backend/log_work.py`, `backend/pm_agent.py`, `backend/license_manager.py`
- `backend/db/learning_store.py`, `platform_store.py`
- `backend/work_tracker/eod_emailer.py`, `session_store.py`
- `backend/project_spec/spec_emailer.py`, `spec_store.py`
- `backend/webhook_server.py`

### TASK-008 — Server-TUI trigger throughput stats panel
- New file: `backend/server_tui/stats_client.py`
- `TriggerStats` dataclass: `triggers_today`, `commits_today`, `last_trigger` (HH:MM string), `errors_24h`
- `get_trigger_stats()` queries SQLite `triggers` table; returns zero-valued stats on any error (graceful degradation)
- `_db_path()` uses `backend.config.database_path()` — no os.getenv
- Wired into `backend/server_tui/app.py` as a live-refreshing panel

### TASK-009 — 37 headless tests for server_tui helpers
- New file: `backend/tests/test_server_tui.py`
- Covers: `ProcessMonitor.refresh()`, `tail()`, `LogTailer`, `get_trigger_stats()` / `_query_stats()`, `health_client.check_all()`
- Headless: no Textual app started; all dependencies mocked
- **Linux-first**: no macOS process names, signal names, or service references
- Total test suite after this session: **433 passing** (was 396 before)

## How to apply:

**Rule**: `os.getenv` must not appear anywhere in `backend/` except `backend/config.py` itself.

When adding a new env var:
1. Add a typed accessor in `backend/config.py` (use `get()`, `get_int()`, `get_bool()`, `get_path()`)
2. Add the var to `.env_sample` with a comment and example value
3. Import and call the accessor from the consuming module — never `os.getenv` directly

When writing new server_tui helpers:
- Add headless tests in `backend/tests/test_server_tui.py`
- Use `psutil` MagicMock pattern from existing tests for process-related tests
- Use `tmp_path` fixture + `sqlite3` for stats_client DB tests
- Keep tests Linux-first (platform strategy: Linux > macOS > WSL > native Windows)

## Files Added:
- `backend/server_tui/stats_client.py` — TriggerStats + get_trigger_stats()
- `backend/tests/test_server_tui.py` — 37 headless tests

## Files Heavily Modified:
- `backend/config.py` — new typed accessors for all new env vars
- `.env_sample` — new entries for all new vars
- 40+ backend modules (see task list above)
