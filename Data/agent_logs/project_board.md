# DevTrack Project Board

_Last updated: 2026-05-09 by PM — TASK-029 through TASK-033 complete (edits only, no commits)_
_Next task ID: TASK-034_

---

## Platform Strategy (recorded 2026-04-05)

- Development environment: macOS (developer's machine)
- Primary deployment target: Linux (Python server/bridge is hosted on Linux)
- Priority: Linux first, macOS compatibility maintained; Windows/WSL is a stretch goal
- Rule: All server-side code, path handling, process management, and service management
  must be written Linux-first. No macOS-specific assumptions in any server_tui or backend
  code.
- The Go binary is already cross-platform and not affected by this rule.

---

## ✅ DONE (session 2026-05-09 — server-slim-tiers)

### TASK-029 — Restructure pyproject.toml: two-tier dependencies (core + ai)
**Assigned to**: engineer
**Phase**: server-slim-tiers
**Started**: 2026-05-09
**Branch**: features/server-slim-tiers

**Spec**:
Restructure `D:\git_apps\Devtrack_\pyproject.toml` to split the single monolithic dependency
set into a mandatory `core` set and an optional `ai` extra.

1. Remove from `[project.dependencies]`:
   - `spacy>=3.7.0`
   - `en-core-web-sm @ https://...` (the wheel URL line)
   - `sentence-transformers>=2.2.0`
   - `chromadb>=0.4.0`

2. Remove from `[project.dependencies]` and move to `[dependency-groups] dev` (PEP 735 / uv-native):
   - `pytest>=7.4.0`
   - `pytest-asyncio>=0.21.0`
   - `pandas-stubs==2.3.2.250926`

3. Add a new `[project.optional-dependencies]` group named `ai`:
   ```toml
   [project.optional-dependencies]
   ai = [
       "spacy>=3.7.0",
       "en-core-web-sm @ https://github.com/explosion/spacy-models/releases/download/en_core_web_sm-3.8.0/en_core_web_sm-3.8.0-py3-none-any.whl",
       "sentence-transformers>=2.2.0",
       "chromadb>=0.4.0",
   ]
   ```

4. Keep existing optional groups (`openai`, `anthropic`, `cloud`, `mongodb`, `notifications`)
   exactly as they are — do not remove them.

5. Add a `[dependency-groups]` section (PEP 735 style, uv-native):
   ```toml
   [dependency-groups]
   dev = [
       "pytest>=7.4.0",
       "pytest-asyncio>=0.21.0",
       "pandas-stubs==2.3.2.250926",
   ]
   ```

File: `D:\git_apps\Devtrack_\pyproject.toml`
Do NOT commit — edits only.

**Acceptance criteria**:
- [ ] `spacy`, `en-core-web-sm`, `sentence-transformers`, `chromadb` removed from `[project.dependencies]`
- [ ] `[project.optional-dependencies]` has an `ai` group containing all four packages
- [ ] `pytest`, `pytest-asyncio`, `pandas-stubs` removed from `[project.dependencies]`
- [ ] `[dependency-groups]` section added with `dev` group containing those three test packages
- [ ] Existing optional groups (`openai`, `anthropic`, `cloud`, `mongodb`, `notifications`) unchanged
- [ ] File is valid TOML (no syntax errors)

**Completed**: 2026-05-09
**Files**: `D:\git_apps\Devtrack_\pyproject.toml`
**Vision check**: PASS
**Notes**: spacy/en-core-web-sm/sentence-transformers/chromadb moved to [project.optional-dependencies] ai; pytest/pytest-asyncio/pandas-stubs moved to [dependency-groups] dev; existing optional groups (openai/anthropic/cloud/mongodb/notifications) unchanged

---

### TASK-030 — Add is_ai_available() to backend/config.py
**Completed**: 2026-05-09
**Files**: `D:\git_apps\Devtrack_\backend\config.py`
**Vision check**: PASS
**Notes**: importlib.util imported at top; is_ai_available() placed after rag_enabled(), before log_dir()

---

### TASK-031 — Add AI feature log line to webhook_server.py lifespan
**Completed**: 2026-05-09
**Files**: `D:\git_apps\Devtrack_\backend\webhook_server.py`
**Vision check**: PASS
**Notes**: Local import of is_ai_available() inside lifespan(); log line placed after startup banner, before TriggerProcessor.get

---

### TASK-032 — devtrack-server: add features and enable subcommands
**Completed**: 2026-05-09
**Files**: `D:\git_apps\Devtrack_\devtrack-server`
**Vision check**: PASS
**Notes**: cmd_features() and cmd_enable() added; both wired into dispatch; FEATURES section added to cmd_help()

---

### TASK-033 — GitLab CI: add core-tests job and rename existing to full-tests
**Completed**: 2026-05-09
**Files**: `D:\git_apps\Devtrack_\ci\devtrack_server.gitlab-ci.yml`
**Vision check**: PASS
**Notes**: Old test: job replaced by core-tests (--group dev, ignores test_nlp_parser.py) and full-tests (--frozen --extra ai --group dev); docker: job unchanged

---

## 🟡 PLANNED

---

### TASK-026 — Remove GetPythonBridgePath dead code from config_env.go
**Priority**: LOW
**Phase**: CS-standalone
**Depends on**: TASK-025

---

## ✅ DONE (session 2026-05-01)

### TASK-026 — Remove GetPythonBridgePath dead code from config_env.go
**Completed**: 2026-05-01
**Branch**: fix/TASK-026-remove-python-bridge-path
**Commit(s)**: `332423c` — refactor(config): remove dead GetPythonBridgePath function (TASK-026)
**PR**: https://github.com/sraj0501/Devtrack_/pull/86 (targeting dev)
**Vision check**: PASS
**Hardcoded scan**: CLEAN
**Notes**: Confirmed no callers with grep before deletion. Removed 17-line dead function block from config_env.go. go build/vet/test all pass.

---

### TASK-027 — Guard handleWork() work report subcommand in Lightweight mode
**Completed**: 2026-05-01
**Branch**: fix/TASK-027-work-report-mode-guard
**Commit(s)**: `3980422` — feat(cli): guard work report subcommand in Lightweight mode (TASK-027)
**PR**: https://github.com/sraj0501/Devtrack_/pull/87 (targeting dev)
**Vision check**: PASS
**Hardcoded scan**: CLEAN
**Notes**: Added requiresManagedMode("work report") guard as first statement in handleWorkReport() in cli_work.go. Other work subcommands (start/stop/adjust/status) are pure Go/SQLite and remain unguarded. go build/vet/test all pass.

---

## ✅ DONE (session 2026-04-30)

### TASK-025 — Windows native build support (build-tag syscall split)
**Completed**: 2026-04-30
**Branch**: fix/TASK-025-windows-native-build
**Commit(s)**: `e0c45b9` — fix(build): split Unix-only syscall sites into build-tag-gated files for Windows native build (TASK-025)
**PR**: https://github.com/sraj0501/Devtrack_/pull/83
**Vision check**: PASS
**Hardcoded scan**: CLEAN
**Notes**: Created 4 platform files (daemon_unix.go, daemon_windows.go, cli_unix.go, cli_windows.go). Removed setupSignalHandlers() body from daemon.go; removed os/signal and syscall imports. Windows stub for force-trigger uses HTTP timer endpoint; Windows stub for process detach uses CREATE_NEW_PROCESS_GROUP. go build/vet/test all pass on Windows.

---

## 🔴 IN PROGRESS (pre-existing, from last session)

### TASK-028 — Internal HTTP control API + cross-platform AlertNotifier + reload-config
**Assigned to**: engineer
**Phase**: CS-standalone / Windows support
**Started**: 2026-05-08
**Branch**: features/TASK-009-ticket-cache

**Acceptance criteria**:
- [x] `go build ./...`, `go vet ./...`, `go test ./...` pass on Windows
- [x] `devtrack reload-config` handler exists and is wired in the switch
- [x] `/internal/reload-config` HTTP endpoint registered and implemented
- [x] `sendReloadConfigSignal()` defined in both `cli_unix.go` and `cli_windows.go`
- [x] `AlertNotifier` imports clean (`from backend.alert_notifier import AlertNotifier`)
- [x] `.gitignore` covers nested GitLab repo clones
- [x] Committed (8dddce9)
- [ ] PR merged (CI pending — flaky JWT test fixed at b6739a6)

**Engineer status**: code committed; CI re-triggered after JWT test fix — awaiting green

---

## ✅ DONE (session 2026-05-07)

### TASK-009 — Ticket cache: SQLite schema + GitHub sync
**Assigned to**: engineer
**Phase**: Phase 5 / CS-1
**Started**: 2026-05-07
**Branch**: features/TASK-009-ticket-cache

**Acceptance criteria**:
- [x] `ticket_cache` and `pm_update_queue` tables created on daemon start
- [x] All seven Go CRUD methods compile with `go build ./...`
- [x] `GitHubTicketSync.sync()` pages open assigned issues and writes to SQLite
- [x] `devtrack init` triggers sync and prints progress
- [x] New env vars documented in `.env_sample`
- [x] `test_ticket_cache.py` passes with `uv run pytest`
- [x] No hardcoded tokens, hosts, or paths anywhere in new code

**Engineer status**: 7/7 criteria done — last commit: 8ddf709 "feat(ticket-cache): add SQLite schema, Go CRUD, Python sync, and tests (TASK-009)" — 2026-05-07
**Blockers**: none
**PR**: https://github.com/sraj0501/Devtrack_/pull/114

**COMPLETE** — ready for PM review — 2026-05-07

---

### TASK-025 — Windows native build support (build-tag syscall split)
**Assigned to**: engineer
**Phase**: CS-standalone
**Started**: 2026-04-30
**Branch**: fix/TASK-025-windows-native-build

**Acceptance criteria**:
- [x] `go build ./...` exits 0 on Windows
- [x] `go vet ./...` exits 0 on Windows
- [x] `go test ./...` exits 0 on Windows
- [x] No change to Linux behavior (verified by reading build tags — no existing code paths altered)
- [x] `devtrack start` and `devtrack stop` logic is provably intact on Linux (moved code is identical, just in a new file)
- [x] PR opened targeting `dev` (not main): merged via PR #84

**Engineer status**: 6/6 criteria done — last commit: e0c45b9 "fix(build): split Unix-only syscall sites into build-tag-gated files for Windows native build (TASK-025)" — 2026-04-30
**Blockers**: none
**PR**: https://github.com/sraj0501/Devtrack_/pull/84

**COMPLETE** — ready for PM review — 2026-04-30

---

### TASK-023-PY — Cross-platform Python desktop notifications
**Phase**: CS-standalone
**Branch**: features/TASK-009-ticket-cache (absorbed into TASK-028)

Completed as part of TASK-028. `AlertNotifier` class, `plyer` dep, `get_notification_enabled()` config accessor all shipped in TASK-028 commit 8dddce9.

**COMPLETE** — 2026-05-08

---

### TASK-018 (verify) / TASK-025 (build-tag split verification)
**Phase**: CS-standalone
**Branch**: features/TASK-018-windows-build-tags
**PR**: https://github.com/sraj0501/Devtrack_/pull/113 — merged 2026-05-07

Build-tag split already in main (commit e0c45b9). TASK-018 verified all acceptance criteria on
`GOOS=windows` + Linux cross-compile. PR #113 merged to main.

**COMPLETE** — 2026-05-07

---

## ✅ DONE (session 2026-04-24)

### TASK-024 — config_env.go: non-fatal GetEmailReporterPath + GetLearningDailyScriptPath
**Completed**: 2026-04-24
**Commit(s)**: `4de127b` — refactor(config): make GetEmailReporterPath, GetLearningDailyScriptPath, GetPythonBridgePath return error instead of os.Exit (TASK-024)
**Vision check**: PASS

---

### TASK-022 — daemon.go: Lightweight mode skips Python subprocess spawning
**Completed**: 2026-04-24
**Commit(s)**: `744acd2`
**Vision check**: PASS

---

### TASK-021 — setup.go: mode selection wizard + backend-free root detection
**Completed**: 2026-04-24
**Commit(s)**: `fd208f6`
**Vision check**: PASS

---

### TASK-023 — cli.go: capability guard for backend-dependent commands
**Completed**: 2026-04-24
**Commit(s)**: `0cde877`
**Vision check**: PASS

---

## ✅ DONE (session 2026-04-23 and earlier)

### TASK-020 — Inbound webhook integration tests
**Completed**: 2026-04-23 | **PR**: https://github.com/sraj0501/automation_tools/pull/80

### TASK-019 — Ship features/loadEnvs to main
**Completed**: 2026-04-23 | **PR**: https://github.com/sraj0501/automation_tools/pull/79

### TASK-018 — CS-3 audit: low-severity hardcoded values
**Completed**: 2026-04-10 | **PR**: https://github.com/sraj0501/automation_tools/pull/77

### TASK-017 — CS-3 audit: medium-severity hardcoded values
**Completed**: 2026-04-10 | **PR**: https://github.com/sraj0501/automation_tools/pull/76

### TASK-016 — CS-3 audit: high-severity hardcoded values
**Completed**: 2026-04-10 | **PR**: https://github.com/sraj0501/automation_tools/pull/75

### TASK-015 — CS-3: Admin console polish + docs sync
**Completed**: 2026-04-10 | **PR**: https://github.com/sraj0501/automation_tools/pull/73

### TASK-014 — CS-3: Trigger stats panel on admin dashboard
**Completed**: 2026-04-10 | **PR**: https://github.com/sraj0501/automation_tools/pull/72

### TASK-013 — CS-3: License status page in admin UI
**Completed**: 2026-04-10 | **PR**: https://github.com/sraj0501/automation_tools/pull/71

### TASK-012 — CS-3: User role update + disable/enable routes
**Completed**: 2026-04-10 | **PR**: https://github.com/sraj0501/automation_tools/pull/70

### TASK-011 — CS-3: Admin route HTTP tests
**Completed**: 2026-04-10 | **PR**: https://github.com/sraj0501/automation_tools/pull/69

### TASK-010 — Full Documentation and Memory Audit
**Completed**: 2026-04-06 | **Commit**: `175a41d`

### TASK-008 — CS-2: Trigger throughput stats panel to Server TUI
**Completed**: 2026-04-05 | **Commit**: `9324027`

### TASK-007 through TASK-001 — Config audit, os.getenv elimination, config accessors
**Completed**: 2026-04-05

### TASK-000 — v1.0.0 release + local agents setup
**Completed**: 2026-04-05
