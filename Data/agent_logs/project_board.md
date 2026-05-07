# DevTrack Project Board

_Last updated: 2026-04-30 by PM (TASK-025 dispatched — Windows native build; TASK-026/027 planned)_
_Next task ID: TASK-028_

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

## 🔴 IN PROGRESS

### TASK-009 — Ticket cache: SQLite schema + GitHub sync
**Assigned to**: engineer
**Phase**: Phase 5 / CS-1
**Started**: 2026-05-07
**Branch**: features/TASK-009-ticket-cache

**Acceptance criteria**:
- [ ] `ticket_cache` and `pm_update_queue` tables created on daemon start
- [ ] All seven Go CRUD methods compile with `go build ./...`
- [ ] `GitHubTicketSync.sync()` pages open assigned issues and writes to SQLite
- [ ] `devtrack init` triggers sync and prints progress
- [ ] New env vars documented in `.env_sample`
- [ ] `test_ticket_cache.py` passes with `uv run pytest`
- [ ] No hardcoded tokens, hosts, or paths anywhere in new code

**Engineer status**: started — adding SQLite tables/CRUD in database.go, Go config accessors in config_env.go, Python TicketDB helper, GitHubTicketSync class, devtrack init integration, env vars in .env_sample + config.py, and pytest suite

---

### TASK-025 — Windows native build support (build-tag syscall split)
**Assigned to**: engineer
**Phase**: CS-standalone
**Started**: 2026-04-30
**Branch**: fix/TASK-025-windows-native-build

**Spec**:
Split Unix-only syscall sites out of `cli.go` and `daemon.go` into build-tag-gated files
so `go build ./...` succeeds natively on Windows (D:/git_apps/Devtrack_).

Changes required:

1. Create `devtrack-bin/daemon_unix.go` (`//go:build !windows`)
   - Move `Setsid: true` SysProcAttr usage from `daemon.go`
   - Move SIGUSR2 daemon listener from `daemon.go`

2. Create `devtrack-bin/daemon_windows.go` (`//go:build windows`)
   - Stub equivalents: `CREATE_NEW_PROCESS_GROUP` flag instead of `Setsid`
   - HTTP or named-pipe signal for trigger, or a clearly-commented no-op stub

3. Create `devtrack-bin/cli_unix.go` (`//go:build !windows`)
   - Move `syscall.SIGUSR2` usage from `cli.go` (the `devtrack trigger` handler)

4. Create `devtrack-bin/cli_windows.go` (`//go:build windows`)
   - Stub for trigger handler on Windows

5. No change to Linux/macOS behavior — build tags must preserve all existing code paths

**Checkout instruction**: `git checkout -b fix/TASK-025-windows-native-build` from main.
Do NOT target main in the PR — use `gh pr create --base dev`.

**Acceptance criteria**:
- [ ] `go build ./...` exits 0 on Windows (this machine: D:/git_apps/Devtrack_)
- [ ] `go vet ./...` exits 0 on Windows
- [ ] `go test ./...` exits 0 on Windows
- [ ] No change to Linux behavior (verified by reading build tags — no existing code paths altered)
- [ ] `devtrack start` and `devtrack stop` logic is provably intact on Linux (moved code is identical, just in a new file)
- [ ] PR opened targeting `dev` (not main): `gh pr create --base dev`

**Engineer status**: not started
**Blockers**: none

---

## 🟡 PLANNED

### TASK-026 — Remove GetPythonBridgePath dead code from config_env.go
**Priority**: LOW
**Phase**: CS-standalone
**Depends on**: TASK-025

**Spec**:
`GetPythonBridgePath()` in `devtrack-bin/config_env.go` was made non-fatal by TASK-024.
No callers remain after that refactor. Delete the function entirely.

- File: `devtrack-bin/config_env.go`
- Action: Remove `GetPythonBridgePath()` function definition
- Verify with `grep -rn "GetPythonBridgePath" devtrack-bin/` that no callers exist before deletion
- Run `go build ./...`, `go vet ./...`, `go test ./...` to confirm nothing breaks

**Acceptance criteria**:
- [ ] `GetPythonBridgePath()` function no longer exists in `config_env.go`
- [ ] `grep -rn "GetPythonBridgePath" devtrack-bin/` returns no matches
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] PR opened targeting `dev`: `gh pr create --base dev`

---

### TASK-027 — Guard handleWork() work report subcommand in Lightweight mode
**Priority**: MEDIUM
**Phase**: CS-standalone
**Depends on**: TASK-025

**Background**:
The `work report` subcommand inside `handleWork()` in `cli.go` calls Python internally
(the email reporter). It was excluded from the `requiresManagedMode()` guard added in
TASK-023 per spec ("handleWork() ... leave them unguarded for now; they are lower risk
and can be addressed in a follow-up"). This is that follow-up.

**Spec**:
Inside `handleWork()` in `devtrack-bin/cli.go`, locate the `work report` subcommand
dispatch branch. Add a `requiresManagedMode("work report")` guard at the top of that
branch only. Leave all other `handleWork()` subcommands unguarded.

**Acceptance criteria**:
- [ ] `devtrack work report` in Lightweight mode prints:
      `'work report' requires Managed mode (Python backend).`
      followed by the re-run-setup line
- [ ] All other `devtrack work` subcommands (e.g. `work update`, `work status`) still
      work normally in Lightweight mode
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] PR opened targeting `dev`: `gh pr create --base dev`

---

## ✅ DONE (session 2026-04-24)

### TASK-024 — config_env.go: non-fatal GetEmailReporterPath + GetLearningDailyScriptPath
**Assigned to**: engineer
**Priority**: MEDIUM
**Phase**: CS-standalone
**Branch**: features/standalone-cli-mode
**Depends on**: TASK-023 (complete)

**Acceptance criteria**:
- [x] `GetEmailReporterPath()`, `GetLearningDailyScriptPath()`, `GetPythonBridgePath()` all return `(string, error)` instead of calling `os.Exit`.
- [x] All callers updated to handle the returned error.
- [x] `go build ./...`, `go vet ./...`, `go test ./...` pass (pre-existing Windows syscall errors only; clean on Linux).
- [x] No `os.Exit` calls remain in any of the three functions.

**Engineer status**: 4/4 criteria done — last commit: 4de127b "refactor(config): make GetEmailReporterPath, GetLearningDailyScriptPath, GetPythonBridgePath return error instead of os.Exit (TASK-024)" — 2026-04-24
**Blockers**: none
**PR**: https://github.com/sraj0501/automation_tools/pull/82

**COMPLETE** — ready for PM review — 2026-04-24

---

### TASK-022 — daemon.go: Lightweight mode skips Python subprocess spawning
**Assigned to**: engineer
**Phase**: CS-standalone
**Started**: 2026-04-24
**Branch**: features/standalone-cli-mode
**Depends on**: TASK-021 (complete)

**Acceptance criteria**:
- [x] `server_config.go` has `ServerModeLightweight` constant.
- [x] `GetServerMode()` returns `ServerModeLightweight` when env var is `"lightweight"`.
- [x] `IsExternalServer()` returns `true` for lightweight mode.
- [x] `IsLightweightMode()` helper function exists and works correctly.
- [x] `go build ./...`, `go vet ./...`, `go test ./...` all pass (pre-existing Windows syscall errors only; clean on Linux).
- [x] A daemon started with `DEVTRACK_SERVER_MODE=lightweight` does not attempt to spawn any Python subprocess (verified: `startWebhookServer()` returns early via updated `IsExternalServer()`).

**Engineer status**: 6/6 criteria done — last commit: 744acd2 "feat(daemon): add ServerModeLightweight — skip Python spawn in lightweight mode (TASK-022)" — 2026-04-24
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-04-24

### TASK-021 — setup.go: mode selection wizard + backend-free root detection
**Assigned to**: engineer
**Phase**: CS-standalone
**Started**: 2026-04-24
**Branch**: features/standalone-cli-mode

**Acceptance criteria**:
- [x] Running `devtrack setup` presents the 3-option mode menu as the first prompt.
- [x] Choosing [2] or [3] does NOT call `detectProjectRoot()` and does NOT fail if
      `backend/` is absent from the filesystem.
- [x] `.env` written by a Lightweight setup contains `DEVTRACK_SERVER_MODE=lightweight`.
- [x] `.env` written by an External setup contains `DEVTRACK_SERVER_MODE=external`.
- [x] `.env` written by a Managed setup contains `DEVTRACK_SERVER_MODE=managed` (unchanged).
- [x] `checkPythonBackend()` is skipped in Lightweight/External modes.
- [x] `go build ./...` succeeds with no new errors (pre-existing Windows syscall errors; clean on Linux).
- [x] `go vet ./...` passes (same caveat as above).
- [x] `go test ./...` passes (same caveat as above).

**Engineer status**: 8/8 criteria done — last commit: fd208f6 "feat(setup): add mode selection wizard for standalone-cli support (TASK-021)" — 2026-04-24
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-04-24 00:00

---

### TASK-023 — cli.go: capability guard for backend-dependent commands
**Assigned to**: engineer
**Priority**: HIGH
**Phase**: CS-standalone
**Started**: 2026-04-24
**Branch**: features/standalone-cli-mode
**Depends on**: TASK-022 (complete)

**Acceptance criteria**:
- [x] All listed handlers return early with `requiresManagedMode()` when mode is `lightweight`.
- [x] The error message printed is exactly:
      `'<command>' requires Managed mode (Python backend).`
      followed by the re-run-setup line.
- [x] `handleStart()`, `handleStop()`, `handleStatus()`, `handleLogs()`,
      `handleForceTrigger()`, `handleVersion()`, `handleWorkspace()` work normally in
      Lightweight mode (no guard).
- [x] `go build ./...`, `go vet ./...`, `go test ./...` pass (pre-existing Windows syscall errors only; clean on Linux).

**Engineer status**: 4/4 criteria done — last commit: 0cde877 "feat(cli): capability guard for backend-dependent commands in lightweight mode (TASK-023)" — 2026-04-24

**COMPLETE** — ready for PM review — 2026-04-24

---

## ✅ DONE (session 2026-04-23)

### TASK-020 — Inbound webhook integration tests
**Assigned to**: engineer
**Phase**: CS-1
**Completed**: 2026-04-23
**Branch**: features/inbound-webhook-tests
**Commit(s)**: `805cad8` — test(webhooks): add inbound webhook integration tests (TASK-020)
**PR**: https://github.com/sraj0501/automation_tools/pull/80
**Vision check**: PASS
**Notes**: Integration tests for inbound webhook handling via FastAPI TestClient.

---

### TASK-019 — Ship features/loadEnvs to main (fix pre-existing test + open PR)
**Assigned to**: engineer
**Phase**: CS-1 / auto-env-load
**Started**: 2026-04-23
**Branch**: features/loadEnvs
**Commit(s)**: `c8be0ea` — auto environment load | `c1c05fa` — test(project-manager): isolate DB per test to fix test_find_related_projects
**PR**: https://github.com/sraj0501/automation_tools/pull/79
**Vision check**: PASS
**Hardcoded scan**: CLEAN (localhost literals in setup.go are prompt defaults for .env generation, not runtime values)
**Suite**: 502 passed (was 501; pre-existing failure resolved)

---

### TASK-018 — CS-3 audit: low-severity hardcoded values (audit log limit + license email)
**Completed**: 2026-04-10
**Commit(s)**: `c0c8a58` — fix(admin): eliminate low-severity hardcoded audit limit and license email (TASK-018)
**PR**: https://github.com/sraj0501/automation_tools/pull/77
**Vision check**: PASS
**Hardcoded scan**: CLEAN

---

### TASK-017 — CS-3 audit: medium-severity hardcoded values (ports fallback + shutdown grace + HTMX intervals)
**Completed**: 2026-04-10
**Commit(s)**: `46f2cda` — fix(admin): eliminate medium-severity hardcoded values in routes, webhook, dashboard (TASK-017)
**PR**: https://github.com/sraj0501/automation_tools/pull/76
**Vision check**: PASS
**Hardcoded scan**: CLEAN

---

### TASK-016 — CS-3 audit: high-severity hardcoded values (session cookie + scrypt params)
**Completed**: 2026-04-10
**Commit(s)**: `25cec2f` — fix(admin): eliminate hardcoded scrypt params and session cookie max_age (TASK-016)
**PR**: https://github.com/sraj0501/automation_tools/pull/75
**Vision check**: PASS
**Hardcoded scan**: CLEAN

---

### TASK-015 — CS-3: Admin console polish + docs sync
**Completed**: 2026-04-10
**Commit(s)**: `1df6751` — feat(admin): CS-3 polish — password reset, ADMIN_EMBED, docs sync (TASK-015)
**PR**: https://github.com/sraj0501/automation_tools/pull/73
**Vision check**: PASS
**Hardcoded scan**: CLEAN

---

### TASK-014 — CS-3: Trigger stats panel on admin dashboard
**Completed**: 2026-04-10
**Commit(s)**: `5337f2f` — feat(admin): trigger stats panel on admin dashboard (TASK-014)
**PR**: https://github.com/sraj0501/automation_tools/pull/72
**Vision check**: PASS

---

### TASK-013 — CS-3: License status page in admin UI
**Completed**: 2026-04-10
**Commit(s)**: `a221c04` — feat(admin): license status page in admin console (TASK-013)
**PR**: https://github.com/sraj0501/automation_tools/pull/71
**Vision check**: PASS

---

### TASK-012 — CS-3: User role update + disable/enable routes
**Completed**: 2026-04-10
**Commit(s)**: `2ef7f14` — feat(admin): user role update + disable/enable routes (TASK-012)
**PR**: https://github.com/sraj0501/automation_tools/pull/70
**Vision check**: PASS

---

### TASK-011 — CS-3: Admin route HTTP tests
**Completed**: 2026-04-10
**Commit(s)**: `12d268e` — test(admin-routes): add HTTP-level route tests for admin console (TASK-011)
**PR**: https://github.com/sraj0501/automation_tools/pull/69
**Vision check**: PASS

---

### TASK-010 — Full Documentation and Memory Audit
**Completed**: 2026-04-06
**Commit(s)**: `175a41d` — docs: sync CLAUDE.md and README to CS-1 reality (TASK-010)
**Vision check**: PASS

---

### TASK-009 — CS-2: Tests for server_tui modules
**Completed**: 2026-04-05
**Commit(s)**: `4b5ad49`
**Vision check**: PASS

---

### TASK-008 — CS-2: Add trigger throughput stats panel to Server TUI
**Completed**: 2026-04-05
**Commit(s)**: `9324027`
**Vision check**: PASS

---

### TASK-007 — Fix remaining os.getenv violations
**Completed**: 2026-04-05
**Commit**: `df59693`

---

### TASK-006 — Fix os.getenv in remaining modules
**Completed**: 2026-04-05
**Commit**: `b9a910b`

---

### TASK-005 — Fix os.getenv in backend/admin/ and backend/server_tui/
**Completed**: 2026-04-05
**Commit**: `fd614d4`

---

### TASK-004 — Fix os.getenv in backend/gitlab/
**Completed**: 2026-04-05
**Commit**: `b21f639`

---

### TASK-003 — Fix os.getenv in backend/github/
**Completed**: 2026-04-05
**Commit**: `e10f7fa`

---

### TASK-002 — Fix os.getenv in backend/azure/
**Completed**: 2026-04-05
**Commit**: `fdd4fd2`

---

### TASK-001 — Add all missing config accessors to backend/config.py
**Completed**: 2026-04-05
**Commit**: `81028cc`
**Notes**: 50+ typed accessors. 397 tests pass.

---

### TASK-000 — v1.0.0 release + local agents setup
**Completed**: 2026-04-05
**Commit(s)**: `0cd0fad` · `37fc01b` · `63006de` · `8431dc3` · `3c4a037`
**Vision check**: PASS

---
