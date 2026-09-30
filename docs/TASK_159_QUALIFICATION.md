# TASK-159 qualification — 2026-09-30

Local qualification is complete. The automatic-time implementation merged through
PR #272 at `94dbc7c`, as recorded in local Git history. These results qualify
`3c3fedf` on `fix/TASK-159-schema-transactions` plus the working-tree change to
`scripts/e2e.ps1`. They do not establish hosted CI success or upstream integration
of the follow-up. GitHub CLI returned HTTP 401 during this verification.

## Changes qualified

- Schema initialization batches base DDL/migrations, migration tables, and
  server-sync triggers into separate immediate transactions. Failed groups roll
  back; the connection remains owned until commit or rollback. Knowledge backfill
  starts after the migration-table transaction finishes.
- The Windows E2E helper compares stdout only. Timestamped startup diagnostics
  on stderr remain visible but cannot falsely fail persistence comparisons.
  The restart check waits 1.1 seconds to cross a log timestamp boundary.

## Results

| Gate | Environment | Result |
| --- | --- | --- |
| `go test ./... -count=1 -timeout 60s` | Windows amd64, Go 1.27.0 | PASS; database package 6.915 s |
| `go vet ./...` | Windows amd64, Go 1.27.0 | PASS |
| `scripts/e2e.ps1` | Windows PowerShell 5.1 | PASS |
| `go test ./... -count=1 -timeout 60s` | Fedora 44 WSL2 amd64, Go 1.24.4 | PASS, including real-Git pipe/PTY cases |
| `go vet ./...` | Fedora 44 WSL2 amd64, Go 1.24.4 | PASS |
| `sh scripts/e2e.sh` with cross-compiled `DEVTRACK_E2E_BINARY` | Fedora 44 WSL2 | PASS |
| `TestInitSchemaRollsBackFailure` | Windows and Linux suites | PASS; failed DDL leaves no partial tables and repair permits retry |
| Shared-memory boundary and `git diff --check` | Windows | PASS |

Both isolated, no-send E2E runs observed a real Git commit, inferred ticket time,
applied a 45-minute correction, restarted the daemon, verified identical persisted
status, exercised explicit start/stop, retained the earlier correction, and exposed
the commit through MCP. Both ended with no pending actions and cleaned up their
temporary daemon and workspace. Windows observed `977925288cf5`; Linux observed
`cc5c8b26ad8b` in disposable repositories.

Clock-controlled coverage includes `TestWorkActivityRestartReplayAndCorrections`,
`TestStartOnlyAutomaticClosureDoesNotInventTime`,
`TestIdleAndEODClosureUseLastEvidence`, `TestWorkSessionUpgradeFromLegacySchema`,
`TestWorkTimeBoundaries`, `TestExplicitStartTicketSwitchAndLateEvidence`,
`TestCorrectionsSurviveLaterEvidence`, `TestWorkWindowArrivalPermutations`,
`TestStopCannotRewriteAutomaticTicketClosure`, and the `internal/worktime` tests.
The algorithm and privacy boundary are documented in [WORK_TIME_INFERENCE.md](WORK_TIME_INFERENCE.md).

Three-iteration Windows initialization benchmarks reported about 29.1 ms for a
fresh database and 10.7 ms for reopening. These are local observations, not a
controlled before/after performance claim.

## Environment and remaining integration gate

Windows used repository-local Go/uv caches. PowerShell required process-scoped
execution-policy bypass to run the repository E2E script. WSL initially lacked Go
and the `script` PTY utility; an isolated Go 1.24.4 toolchain and Fedora's
`util-linux-script` package enabled the full passing rerun. The initial Linux
failure was limited to the missing PTY utility. Linux coverage here is WSL2,
not a dedicated native-Linux machine or hosted runner.

The Linux E2E emitted a duplicate-column warning for legacy migration 013; the
run completed successfully. Windows PowerShell 5.1 formats redirected native
stderr as `NativeCommandError` diagnostics; the helper preserves the native exit
code, and the E2E completed with exit code zero.

Remaining: restore authenticated GitHub access, verify the follow-up PR/hosted
checks, and integrate the reviewed schema/E2E follow-up into `dev`. Documentation
and the E2E change remain local until committed and published. Sage is the next
implementation initiative after that integration gate.

Publication follow-up (2026-09-30): the E2E change and this qualification record were committed
through DevTrack as `2e50763` and pushed to `origin/fix/TASK-159-schema-transactions`.
`git ls-remote` confirmed the full upstream hash `2e507636031444b0100cd4d4a911390aa9659da4`.
GitHub API authentication still fails; hosted checks and integration into `dev` remain unverified.
Sage parity preparation continues on a branch stacked on this published follow-up.
