# Sage Lifecycle Qualification

_Qualified 2026-10-14 against `ai_sessions_skills` commit `ea2e067`._

## Scope

This TASK-157 slice closes the 10 daemon/process and 7 CLI lifecycle rows in the 136-scenario
Sage parity ledger. DevTrack keeps one Go daemon; it does not reproduce the reference's detached
Python Sage watcher.

## Behavior

- The daemon lock is exclusive and reusable. Because Windows `LockFileEx` and POSIX `flock` are
  process-owned, a crashed process releases the lock automatically and an old unlocked lock file
  cannot block startup.
- Windows daemon children request `CREATE_NO_WINDOW` and a new process group. A real native child
  test proves both the missing console and intact piped stdin. POSIX uses a new session without any
  Windows process attributes.
- Daemon PID checks reject missing and dead PIDs, accept the current process, treat POSIX
  permission errors conservatively, and treat Windows access-denied queries as possibly alive so
  restricted process visibility cannot cause a duplicate daemon.
- `devtrack sage stop|pause|off [reason]` persistently pauses capture, while
  `start|resume|on` resumes it. The optional local reason is exposed as `pause_reason` while paused.
- `devtrack sage restart` advances the capture cutoff and resumes daemon-owned work without
  starting a second watcher.
- Unknown actions fail loudly. Compatibility `hook-start`, `hook-end`, `start <harness>`, and
  `end <harness>` calls are silent no-ops and cannot resume a user-paused capture session.

## Adaptation boundary

Five lifecycle rows map directly to Go behavior and 12 are explicit adaptations. The adaptations
replace Python directory-lock cleanup, helper wrappers, `tasklist`, heartbeat files, and watcher
replacement with OS locks, build-tagged native launch code, native process queries, the DevTrack
PID record, and the already-running daemon. These changes preserve lifecycle outcomes without
adding a competing Sage process.

## Evidence

- Exact tests: `internal/daemon/lifecycle_test.go`,
  `internal/daemon/process_windows_test.go`, `cli_unix_test.go`, `cli_windows_test.go`, and
  `sage_cli_test.go`.
- Actual Windows execution passed all 16 applicable exact lifecycle tests, including real
  no-console child execution and stdin preservation. The POSIX-only launch test passed in Linux.
- `go test ./...` and `go vet ./...` passed in the Go 1.24 Docker toolchain.
- PR #282 passed all 19 hosted checks, including Windows build/vet, the full Windows unit suite,
  Windows no-send E2E, and Windows MCPB smoke. An initial Windows E2E runner timed out before its
  isolated daemon PID appeared; the unchanged rerun passed in 1m49s, consistent with a successful
  local Windows reproduction.
- `scripts/check_sage_parity.py --reference C:\Work\git_apps\ai_sessions_skills` passed with
  136 rows: 110 implemented, 16 adapted, 2 partial, and 8 pending.

The remaining open rows are four capture/logging scenarios, four optional IDE scenarios, and two
partial Codex-history scenarios. Repeated clean Windows/Linux capture-to-knowledge acceptance is
also still required before SAGE-003 can close.
