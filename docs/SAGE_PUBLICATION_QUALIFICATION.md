# Sage durable Markdown publication

TASK-157 branch checkpoint, 2026-09-30. The daemon now publishes saved drafts to
Markdown independently of distillation. This is local runtime qualification using
an HTTP model fixture, not hosted CI, a live-model acceptance run, or full Sage parity.

## Persistence and recovery

`sage_jobs` continues to own model work; `distilled` means the validated draft is
saved. A transactional trigger enqueues `sage_publications`, and migration backfills
existing drafts idempotently. Publication has separate waiting, retrying, and
documented states, attempts, next retry time, safe error category, filename, and
completion timestamp. Only successful topic **and** README index writes permit the
documented transition. File failures never send the draft back to the model.

The publisher acquires SQLite's write lock before reading a draft and retains it
through local file replacement and acknowledgement. This serializes publishers
sharing the same database, including across processes. Cancellation does not release
the lock while the non-interruptible file operation is still running. Crash or
cancelled acknowledgement rolls back database state; the next attempt reuses the
signature and repairs the index without duplicating the entry.

This deliberately holds a database writer lock during local file I/O. A slow disk
or large knowledge directory can delay other database writers and daemon shutdown;
model, network, and Git operations are forbidden inside this transaction. Hooks
remain spool-only. Separate databases sharing one output directory and concurrent
external file editors are outside this serialization guarantee. Arbitrary power-loss
durability is not claimed. The writer's existing size, symlink, and path checks apply.

## Output contract

`DEVTRACK_SAGE_KNOWLEDGE_DIR` overrides the default `<Sage data directory>/knowledge`.
Use an absolute override and restart the daemon after configuration changes. The
deterministic existing command-family topic selects the file; the model cannot pick
a path. SHA-256 of the canonical normalized signature supplies the stable lowercase
hex Markdown marker; random queue claim tokens never appear in files. This is the
Go marker contract, not a claim of reference-store import compatibility.

Pause prevents new publication and model work; an operation already started may
finish. Shutdown joins both workers before database close. Existing search remains
the model-free command index; full-entry search, user-visible publication diagnostics,
manual routing/merge, and safe local commits remain open.

## Evidence

- `TestSagePublicationFailureRestartAndAcknowledgementRecovery`: malformed index
  after successful topic write, persisted safe retry, due time, database reopen,
  cancellation after successful files, duplicate-free replay, no repeat model claim.
- `TestSagePublicationBackfillsExistingDrafts`: old drafts enqueue exactly once.
- `TestSagePublicationCancellationRetainsLockUntilWriterReturns`: two database
  handles cannot publish concurrently when the first caller cancels during a write.
- `TestSageDaemonSpoolToDurableDraftSurvivesRestart`: real daemon lifecycle imports
  a spool event, retries HTTP outage and malformed output, persists and publishes a
  valid draft, then restarts with byte-identical topic/index and only three model calls.

Windows full Go suite and vet passed, followed by the added cancellation-lock test.
Linux focused race/vet results are recorded in the engineer log. Reference scenario
counts remain 13 implemented, 15 partial, 108 pending: these integration checks do
not by themselves close additional reference scenarios.
