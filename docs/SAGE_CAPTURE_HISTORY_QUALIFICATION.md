# Sage Capture and Codex-History Qualification

_Qualified 2026-10-14 against `ai_sessions_skills` commit `ea2e067`._

## Scope

This TASK-157 slice closes the four capture/logging rows and the two partial Codex-history rows in
the 136-scenario Sage parity ledger. The remaining four open scenarios belong to the optional IDE
adapter boundary.

## Behavior

- A persistently paused Sage session suppresses the installed Codex adapter before it writes a
  spool event or capture decision.
- One hook payload is enqueued as a batch. The first normalized action is `new`, an identical
  immutable delivery is `known`, and the same signature repeated within that payload is `repeat`.
- Later distinct uses of the same signature remain separate events so knowledge occurrence counts
  stay truthful. Durable deduplication continues to use the delivery identity.
- Capture decisions are bounded NDJSON containing only an opaque signature ID, decision, harness,
  local timestamp, and `capture` stage. Commands, raw signatures, paths, output, prompts, and
  reasoning are excluded. `devtrack sage log` exposes these entries alongside durable pipeline
  diagnostics.
- Decision-log creation, append, and trimming are best-effort. Every failure is swallowed so a
  logging problem cannot interrupt a coding-agent tool call.
- The read-only Codex history poller retains an in-progress item while collecting a later fast
  command, captures the slow item once it finishes, and emits nothing on the following poll.
- Failed history commands preserve their terminal failure and exact non-zero exit code through
  normalization, spool publication, and import.

## Evidence

- Exact tests: `internal/sage/spool_test.go#TestReferenceLogNeverRaises`,
  `internal/sage/spool_test.go#TestReferenceEnqueueLogsNewThenKnown`,
  `sage_cli_test.go#TestReferencePausedStopsCapture`,
  `sage_cli_test.go#TestReferenceRepeatWithinOneCommandLineIsMarked`,
  `internal/sage/codexhistory/poller_test.go#TestReferenceInProgressCommandIsCollectedWhenItFinishes`,
  and `internal/sage/codexhistory/poller_test.go#TestReferenceFailedCommandPreservesExitStatus`.
- `sage_cli_test.go#TestSageLogIncludesPrivacySafeCaptureDecisions` proves the CLI exposes the
  decision while excluding command and raw-signature content.
- All six exact tests passed in the Linux Go 1.24 Docker toolchain and as native Windows test
  binaries. The full Go suite and `go vet ./...` passed in Docker.
- `scripts/check_sage_parity.py --reference C:\Work\git_apps\ai_sessions_skills` passes with
  136 rows: 116 implemented, 16 adapted, 0 partial, and 4 pending.

TASK-157 remains open for the four optional IDE rows and the repeated clean Windows/Linux
capture-to-knowledge acceptance journey.
