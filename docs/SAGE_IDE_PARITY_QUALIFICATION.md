# Sage Optional IDE Parity Qualification

_Qualified 2026-10-15 against `ai_sessions_skills` commit `ea2e067`._

## Scope

This TASK-157 slice resolves the last four open scenarios in the 136-scenario Sage parity ledger.
It does not add or advertise another harness. The existing opt-in, read-only Codex history adapter
owns the compatible `vscode` and `cli` behavior.

## Behavior

- First use starts at the current time, so older history is excluded. Each later supported command
  is captured once and repeat polls are empty.
- DevTrack deliberately enables both stable Codex history sources together. Unlike the reference's
  IDE-default/raw-command path, it emits the same privacy-minimized command representation for
  `vscode` and `cli`; private arguments and paths remain excluded. This is the one explicit
  adaptation in the slice.
- Persistent pause returns before opening either Codex database and leaves the checkpoint bytes
  unchanged.
- Pause and resume advance the capture cutoff and clear in-memory thread progress on the next poll,
  preventing commands from the paused interval from being backfilled.
- If event publication fails, the poll returns the error without saving its advanced ordinal. The
  following successful poll captures that event once.

## Evidence

- Exact tests:
  `internal/sage/codexhistory/poller_test.go#TestReferenceCapturesOriginalCommandOnceAndExcludesCLIAndOldHistory`,
  `internal/sage/codexhistory/poller_test.go#TestReferencePauseDoesNotReadOrAdvanceHistory`,
  `internal/sage/codexhistory/poller_test.go#TestReferenceResetExcludesCommandsFromThePausedPeriod`,
  and `internal/sage/codexhistory/poller_test.go#TestReferenceFailureDoesNotAdvanceCheckpoint`.
- All four exact tests pass in the Linux Go 1.24 Docker toolchain and as a native Windows test
  binary.
- The full Go suite, `go vet ./...`, and Linux race coverage for `internal/sage/codexhistory` pass.
- `scripts/check_sage_parity.py --reference C:\Work\git_apps\ai_sessions_skills
  --require-complete` passes with 136 rows: 119 implemented, 17 adapted, 0 partial, and 0 pending.

The parity ledger is complete. The later repeated clean Windows/Linux capture-to-knowledge journey
passes; TASK-157 remains open only for hosted review and integration of the stacked reviews.
PR #284 carries this slice into `dev`; all 19 hosted checks pass.
