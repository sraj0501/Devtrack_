# DevTrack Project Board

_Last updated: 2026-04-24 by PM (TASK-021 through TASK-024 planned — standalone-cli-mode)_
_Next task ID: TASK-025_

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

**Background**:
`RunSetup()` in `devtrack-bin/setup.go` calls `detectProjectRoot()` as its very first
action. That function walks up from the binary looking for a `backend/` directory and
hard-fails if absent. This blocks any client deployment where only the Go binary is
distributed (no Python source).

The fix is to prompt the user for an operating mode *before* `detectProjectRoot()` is
called, then conditionally skip the `backend/` check in Lightweight and External modes.

**Spec — exact changes to `devtrack-bin/setup.go`**:

1. Add a `DevTrackMode` type and three constants at the top of the file (or in a new
   short block before `RunSetup`):

   ```go
   type DevTrackMode string
   const (
       ModeLightweight DevTrackMode = "lightweight"
       ModeExternal    DevTrackMode = "external"
       ModeManaged     DevTrackMode = "managed"
   )
   ```

2. Add a `SetupConfig.Mode DevTrackMode` field to `SetupConfig`.

3. At the very top of `RunSetup()`, before *any* call to `detectProjectRoot()`, insert
   a mode-selection prompt:

   ```
   Which mode do you want to run DevTrack in?
     [1] Managed    (default) — full AI features. Requires Python backend/ on this machine.
     [2] Lightweight           — git monitoring + scheduling only. No Python needed.
     [3] External              — daemon only; Python runs on a separate server.

   Choice [1]:
   ```

   Store the result in `cfg.Mode` (default: `ModeManaged`).

4. Determine `projectRoot` differently depending on mode:
   - `ModeManaged`: call existing `detectProjectRoot()` — keep all current behavior.
   - `ModeLightweight` / `ModeExternal`: skip `detectProjectRoot()` entirely. Use the
     binary's parent directory as `projectRoot`:
     ```go
     execPath, _ := os.Executable()
     projectRoot = filepath.Dir(execPath)
     ```
     If `os.Executable()` fails, fall back to `os.Getwd()`.

5. Move the existing `.env` existence check to *after* `projectRoot` is determined
   (it already reads `envPath` which depends on `projectRoot` — just ensure the order
   is correct after the mode-selection block).

6. Wrap `checkPythonBackend(projectRoot)` (step 3 in the current wizard) in an `if
   cfg.Mode == ModeManaged` guard. In Lightweight / External modes, skip the Python /
   uv check entirely and print a single info line instead:
   ```
   [Lightweight mode] Python backend not required — skipping prerequisite check.
   ```

7. In `generateEnvContent(cfg)`, set `DEVTRACK_SERVER_MODE` from `cfg.Mode`:
   - `ModeManaged`     → `DEVTRACK_SERVER_MODE=managed`
   - `ModeLightweight` → `DEVTRACK_SERVER_MODE=lightweight`
   - `ModeExternal`    → `DEVTRACK_SERVER_MODE=external`

   The current code hardcodes `DEVTRACK_SERVER_MODE=managed` — replace that single line.

8. In `printSetupComplete()`, for Managed mode keep the existing "Next steps" text.
   For Lightweight / External modes, omit the "Install Python dependencies" step 1
   and replace it with:
   ```
   Next steps:
     1. Start DevTrack:  devtrack start
     2. Check status:    devtrack status
   Note: AI features (reports, integrations, commit enhancement) require Managed mode.
   Re-run 'devtrack setup' and choose [1] to enable them.
   ```

9. Do NOT change the LLM provider section, workspace section, or PM platform section —
   those remain the same for all modes (user may still configure them for future use).

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

## 🟡 PLANNED

### TASK-022 — daemon.go: Lightweight mode skips Python subprocess spawning
**Priority**: HIGH
**Phase**: CS-standalone
**Depends on**: TASK-021

**Background**:
`server_config.go` already defines `ServerModeManaged`, `ServerModeExternal`, and
`ServerModeCloud`. `IsExternalServer()` returns true for External and Cloud — which
already causes `startWebhookServer()` to skip spawning Python. We need the same skip
for a new `lightweight` mode, and we need to add the `ServerModeLightweight` constant.

**Spec — exact changes to `devtrack-bin/daemon.go` and `devtrack-bin/server_config.go`**:

1. In `devtrack-bin/server_config.go`:
   - Add the constant:
     ```go
     ServerModeLightweight ServerMode = "lightweight"
     ```
   - Update `GetServerMode()` to recognize `"lightweight"`:
     ```go
     if os.Getenv("DEVTRACK_SERVER_MODE") == "lightweight" {
         return ServerModeLightweight
     }
     ```
     Add this check before the `external` check (order: cloud → lightweight → external → managed).
   - Update `IsExternalServer()` to return true for Lightweight too:
     ```go
     func IsExternalServer() bool {
         mode := GetServerMode()
         return mode == ServerModeExternal || mode == ServerModeCloud || mode == ServerModeLightweight
     }
     ```

2. In `devtrack-bin/daemon.go`:
   - `startWebhookServer()` already returns early when `IsExternalServer()` is true — no
     change needed there; the updated `IsExternalServer()` covers it.
   - Add a `IsLightweightMode()` helper in `server_config.go`:
     ```go
     func IsLightweightMode() bool {
         return GetServerMode() == ServerModeLightweight
     }
     ```
   - In `daemon.go`'s `Start()` method, after the `startWebhookServer()` call, add a log
     line when in lightweight mode so it's visible in the log:
     ```go
     if IsLightweightMode() {
         log.Println("Running in Lightweight mode — Python backend disabled")
     }
     ```
   - In `startAssignmentPoller()`, `startGitLabPoller()`, `startTelegramBot()`, and
     `startSlackBot()`: these already check their respective enable flags and return early.
     No additional changes needed — they won't spawn Python if the flags are false (which
     a fresh Lightweight `.env` will have by default).

**Acceptance criteria**:
- [ ] `server_config.go` has `ServerModeLightweight` constant.
- [ ] `GetServerMode()` returns `ServerModeLightweight` when env var is `"lightweight"`.
- [ ] `IsExternalServer()` returns `true` for lightweight mode.
- [ ] `IsLightweightMode()` helper function exists and works correctly.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` all pass.
- [ ] A daemon started with `DEVTRACK_SERVER_MODE=lightweight` does not attempt to
      spawn any Python subprocess (verified by reading the log output).

---

### TASK-023 — cli.go: capability guard for backend-dependent commands
**Assigned to**: engineer
**Priority**: HIGH
**Phase**: CS-standalone
**Started**: 2026-04-24
**Branch**: features/standalone-cli-mode
**Depends on**: TASK-022 (complete)

**Background**:
In Lightweight mode, commands that depend on the Python backend (reports, learning,
azure-*, github-*, gitlab-*, git-sage) currently fail with cryptic Python errors
(missing script, subprocess launch failure, etc.). We need a clean capability guard
that prints a clear message when the mode cannot support the command.

**Spec — exact changes to `devtrack-bin/cli.go`**:

1. Add a helper function near the top of `cli.go`:

   ```go
   // requiresManagedMode prints a clear error and returns an error when the
   // current server mode does not include a Python backend.
   func requiresManagedMode(command string) error {
       if IsLightweightMode() {
           fmt.Printf("'%s' requires Managed mode (Python backend).\n", command)
           fmt.Println("Re-run 'devtrack setup' and choose [1] Managed to enable AI features.")
           return fmt.Errorf("command unavailable in Lightweight mode")
       }
       return nil
   }
   ```

2. Add `requiresManagedMode` guard at the top of each of the following handlers in
   `cli.go` (return immediately if the check returns a non-nil error):

   **Learning commands** (all call Python learning scripts):
   - `handleEnableLearning()`
   - `handleShowProfile()`
   - `handleTestResponse()`
   - `handleRevokeConsent()`
   - `handleLearningStatus()`
   - `handleLearningSetupCron()`
   - `handleLearningRemoveCron()`
   - `handleLearningCronStatus()`
   - `handleLearningSync()`
   - `handleLearningReset()`

   **Report commands** (call Python email_reporter.py):
   - `handlePreviewReport()`
   - `handleSendReport()`
   - `handleSaveReport()`
   - `handleSendSummary()`

   **Integration commands** (call Python Azure/GitHub/GitLab clients):
   - `handleAzureCheck()`
   - `handleAzureList()`
   - `handleAzureSync()`
   - `handleAzureView()`
   - `handleGitLabCheck()`
   - `handleGitLabList()`
   - `handleGitLabSync()`
   - `handleGitLabView()`
   - `handleGitHubCheck()`
   - `handleGitHubList()`
   - `handleGitHubSync()`
   - `handleGitHubView()`

   **Server TUI / Admin** (requires running Python server):
   - `handleServerTUI()`
   - `handleAdminStart()`

   Note: `handleWork()`, `handleVacation()`, `handleAlerts()` are IPC/TUI commands —
   leave them unguarded for now; they are lower risk and can be addressed in a follow-up.

3. `handleStart()` should NOT be guarded — lightweight mode still starts the Go daemon.

4. `handleSendReport()` and `handlePreviewReport()` may also call `GetEmailReporterPath()`
   internally. Those `GetEmailReporterPath()` / `GetLearningDailyScriptPath()` callers in
   `learning.go` should remain as-is — the guard in the CLI handler will prevent reaching
   them in Lightweight mode.

**Acceptance criteria**:
- [x] All listed handlers return early with `requiresManagedMode()` when mode is
      `lightweight`.
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

### TASK-024 — config_env.go: non-fatal GetEmailReporterPath + GetLearningDailyScriptPath
**Priority**: MEDIUM
**Phase**: CS-standalone
**Depends on**: TASK-023

**Background**:
`GetEmailReporterPath()` and `GetLearningDailyScriptPath()` in `config_env.go` currently
call `os.Exit(1)` when the script file is not found. In Lightweight mode there is no
`backend/` directory, so these would exit the entire process if ever reached. The CLI
guard from TASK-023 prevents normal execution paths from hitting them, but defense-in-
depth: make them return an error instead of exiting so tests and any unexpected callsite
don't blow up.

**Spec — exact changes to `devtrack-bin/config_env.go`**:

1. Change `GetEmailReporterPath()` signature from `string` to `(string, error)`:

   ```go
   func GetEmailReporterPath() (string, error) {
       config, err := LoadEnvConfig()
       if err != nil {
           return "", fmt.Errorf("config load failed: %w", err)
       }
       path := filepath.Join(config.ProjectRoot, "backend", "email_reporter.py")
       if !fileExists(path) {
           return "", fmt.Errorf("email reporter script not found at %s (Managed mode required)", path)
       }
       return path, nil
   }
   ```

2. Update all callers of `GetEmailReporterPath()` to handle the error. Search callers:
   - `devtrack-bin/cli.go` — any handler that calls it should log/return the error.

3. Change `GetLearningDailyScriptPath()` from returning `string` with internal
   `os.Exit` on a derived path issue to returning `(string, error)`. The current
   implementation actually does NOT call `os.Exit` — it has graceful fallback. Verify
   this is still correct after TASK-021/022/023 changes and add a file-existence check:

   ```go
   func GetLearningDailyScriptPath() (string, error) {
       config, err := LoadEnvConfig()
       if err != nil {
           return "", fmt.Errorf("config load failed: %w", err)
       }
       var path string
       if config.LearningDailyScriptPath != "" {
           path = expandPath(config.LearningDailyScriptPath)
       } else {
           path = filepath.Join(config.ProjectRoot, "backend", "run_daily_learning.py")
       }
       if !fileExists(path) {
           return "", fmt.Errorf("daily learning script not found at %s (Managed mode required)", path)
       }
       return path, nil
   }
   ```

4. Update all callers of `GetLearningDailyScriptPath()` to handle the error:
   - `devtrack-bin/learning.go` — `NewLearningCommands()` calls it. Update to propagate
     the error cleanly, or move the call into individual command methods that are already
     guarded.

5. `GetPythonBridgePath()` also has an `os.Exit` on missing file — apply the same
   non-fatal treatment:
   ```go
   func GetPythonBridgePath() (string, error) {
       ...
       if !fileExists(path) {
           return "", fmt.Errorf("Python bridge script not found at %s", path)
       }
       return path, nil
   }
   ```
   Update callers accordingly.

**Acceptance criteria**:
- [ ] `GetEmailReporterPath()`, `GetLearningDailyScriptPath()`, `GetPythonBridgePath()`
      all return `(string, error)` instead of calling `os.Exit`.
- [ ] All callers updated to handle the returned error.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass.
- [ ] No `os.Exit` calls remain in any of the three functions.

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
