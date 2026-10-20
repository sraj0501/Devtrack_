# Sage durable queue qualification — 2026-09-30

The TASK-157 branch connects the background distiller to SQLite and the daemon lifecycle.
This completes the imported-event-to-persisted-draft slice, not the full self-writing knowledge
journey. Markdown writing, correction-stable routing, safe local commits, user-facing queue
diagnostics, and final parity closure remain open.

## Behavior

- Event insertion, model-free indexing, and first-signature enqueue commit together. A failed
  enqueue rolls the import back so the spool can retry. Existing signatures are backfilled.
- One atomic SQLite update claims eligible work. Random claim tokens fence stale workers;
  attempts, retry times, leases, verdicts, and draft JSON survive reopening the database.
- Cancelled work remains leased until expiry, then can be reclaimed. Daemon leases exceed the
  configured model timeout by 30 seconds. A crash can repeat a model request, but a stale claim
  cannot overwrite a newer result. This is recoverable processing, not exactly-once generation.
- Outages and malformed output retry. Explicit model skips persist, and completed signatures
  do not trigger new model calls. The terminal queue state `distilled` means a persisted draft,
  not a documented Markdown entry.
- Pause stops new claims. In-flight work may finish. Shutdown cancels model work and waits for
  the Sage importer/worker before closing SQLite. Capture hooks never wait on this queue.
- The daemon uses local Ollama configuration. Raw errors and model skip prose are replaced by
  fixed diagnostic categories so response bodies and credentials cannot enter queue diagnostics.

## Tests

- `TestSageQueueRestartRetriesAndFencesStaleClaims`: live/expired lease behavior after reopening,
  stale completion rejection, persisted retry timing, diagnostic canary exclusion, and draft retention.
- `TestSageQueueConcurrentClaimsAndPersistentSkip`: two independent database handles contend
  for one signature; exactly one claims it, and a validated skip remains terminal.
- `TestSageQueueMigrationBackfillAndImportAtomicity`: legacy backfill is idempotent, and an
  injected enqueue failure rolls event insertion back before a successful retry.
- `TestSageDaemonWorkerPauseAndInflightCancellation`: pause prevents a model call; resume starts
  it; cancellation promptly ends the client request while leaving a recoverable lease.
- `TestSageDaemonSpoolToDurableDraftSurvivesRestart`: the production importer and worker consume
  a real spool event, recover from HTTP outage and malformed model output, persist a draft, and
  reopen without repeating generation. The model is a local HTTP fixture, not a live Ollama model.

Full uncached Go suites (`-timeout 60s`) and `go vet ./...` passed on native Windows and Fedora
WSL2. The first cancellation test run exposed an unread request body in test-server cleanup;
the fixture now consumes the body and guarantees cleanup. This was not a worker cancellation failure.

Hosted CI and PR integration are not established by these local results.

Focused Linux race checks also passed for `internal/db`, `internal/infra`, and
`internal/sage/distill` (Sage/worker tests). Fedora initially lacked GCC; installing it enabled
the CGO-based race detector. The worker aligns the default HTTP timeout with the configured
model timeout, preventing the transport's 120-second default from cutting short slower models.
