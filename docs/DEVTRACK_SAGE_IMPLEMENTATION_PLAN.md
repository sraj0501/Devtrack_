# DevTrack Sage Implementation Plan

## Decision

DevTrack Sage is the product name for DevTrack's local, cross-harness command memory. Its scope is
larger than Git: it captures useful command and tool activity from supported coding harnesses,
and turns that activity into searchable, self-writing personal command knowledge.

DevTrack Sage is implemented entirely in Go. The Python implementation in
`D:\git_apps\ai_sessions_skills` is the behavioral reference for the port, not a runtime
dependency. The authoritative `tool/` baseline is current reference commit `b85a1ab`. The parity
matrix must be refreshed whenever that baseline changes; generated personal knowledge content is
test evidence, not source code to copy.

The `devtrack sage` namespace belongs exclusively to this cross-harness command-knowledge product.

## First releasable outcome

A user can install one supported harness adapter, work normally, and later find a useful,
self-written command entry through `devtrack sage search`. The hook never blocks the harness,
calls a model, or sends raw activity off the machine. Distillation happens asynchronously and
defaults to a local model; unavailable models leave work retryable.

Visible-reasoning capture, automatic cloud synchronization, and autonomous command execution are
not part of this product plan.

## Go port contract

The port targets behavioral parity, not a line-for-line translation. Every required Python
capability, invariant, and regression scenario must have an owned Go destination and a Go test
before the corresponding Python component is considered replaced. Production DevTrack Sage must
never spawn Python or require a Python environment.

| Python reference | Go destination | Ported behavior |
|---|---|---|
| `capture.py`, `ide_capture.py` | `internal/sage/capture` and harness adapters | Payload normalization, command signatures, filtering, deduplication, bounded enqueue, success/failure capture |
| `cfg.py` | `internal/sage/config` | Cross-platform roots, harness layouts, pause state, limits, backend selection |
| `json_merge.py`, installers and launchers | `internal/sage/hooks` | Idempotent additive install, pure removal, foreign-setting preservation, Windows/POSIX launch behavior |
| `watcher.py`, `session.py`, `procs.py` | existing DevTrack daemon plus `internal/sage/importer` | Lifecycle, liveness, locking, bounded batches, retry, restart recovery; no detached Python watcher |
| `distill.py` and prompts | `internal/sage/distill` plus shared Go LLM transport | Classification, structured response validation, retry semantics, deterministic fallback |
| `kb.py`, `search.py`, `sync_seen.py` | `internal/sage/knowledge` and `internal/db` | Routing, entry matching, deterministic rendering, merge/refile, search, durable processed state |
| `doctor.py`, `log.py` | Sage CLI diagnostics and structured Go logging | Truthful health, sanitized errors, bounded local diagnostic history |

Port tracking must use this matrix and the Python test inventory. A module is not done merely
because an equivalent Go file exists.

## Product contract

Initial public commands:

```text
devtrack sage status
devtrack sage pause
devtrack sage resume
devtrack sage doctor
devtrack sage harness list
devtrack sage harness install <name>
devtrack sage harness uninstall <name>
devtrack sage search <query>
devtrack sage topics
devtrack sage log
devtrack sage routes
devtrack sage route <binary> <topic>
devtrack sage merge <source> <destination>
```

## Architecture

```text
harness hook
    -> bounded stdin parsing and normalization
    -> atomic immutable spool file
    -> return success silently

DevTrack daemon
    -> bounded spool importer
    -> redaction and deduplication
    -> SQLite event store
    -> daemon-owned asynchronous distillation worker
    -> deterministic knowledge records and search index

CLI / later MCP tools
    -> read-only search, topics, status, and diagnostics
```

The hook hot path must not open SQLite, call a model or network service, wait on the daemon, or
write ordinary stdout/stderr. A spool write failure is observable through diagnostics but must not
break the host harness. The daemon owns retries, processing state, and model work.

Background model requests default to a forgiving three-minute timeout for offline models and are
configurable with `DEVTRACK_SAGE_MODEL_TIMEOUT_SECS` (bounded to 5–1800 seconds). The worker uses a
short, cancellation-aware idle poll (`DEVTRACK_SAGE_IDLE_POLL_MS`, default 500 ms) and retry delay
(`DEVTRACK_SAGE_RETRY_DELAY_SECS`, default 2 seconds), so it neither busy-spins nor becomes stuck in
long sleep cycles during shutdown or when new work arrives.

Recommended code boundaries:

- `internal/sage`: event contracts, normalization, redaction, spool writer, importer, and service.
- `internal/sage/harness`: adapter registry and selective lifecycle; no adapter is enabled globally.
- `internal/sage/hooks`: fixture-driven harness adapters and idempotent installers.
- `internal/db`: append-only Sage migrations and query methods using the existing database owner.
- `devtrack_client`: CLI routing and daemon lifecycle integration.
- `internal/llmclient`: reusable local/OpenAI-compatible provider transport owned independently
  by the new Sage pipeline.

External adapter evolution follows `SAGE_HARNESS_PLUGIN_CONTRACT.md`: versioned declarative
packages first, with an optional sandboxed WASI normalizer for payloads that need code. MCP is a
discovery/management surface, never a dependency of the hook hot path. Dynamic Go plugins are not
used because they do not provide a portable Windows extension mechanism.

## Storage contract

The first schema should store normalized facts, not complete transcripts:

- Event identity, schema version, harness, harness session, event type, tool, and timestamps.
- Redacted command, normalized signature, project identity, success state, and exit code when known.
- Import state, bounded attempt count, and a sanitized last error.
- Knowledge entries, topics, source-event links, and an FTS search index.
- Deterministically rendered `knowledge/*.md`, topic index, route decisions, and skipped-action
  records matching the reference product behavior.

Event IDs make imports idempotent. A secondary signature prevents common duplicate hook deliveries
within a session. Raw command output, user messages, and reasoning are excluded from capture.

## Delivery milestones

### SAGE-001 — Contract and port boundary

Inventory the pinned Python behaviors and tests, assign each one to a Go package and test, then
define versioned normalized event fixtures, the CLI namespace, data/config/spool roots, redaction
rules, and exit behavior. Add parser and contract tests before wiring a real harness.

Acceptance:

- Malformed, oversized, duplicated, and secret-bearing fixture inputs have deterministic results.
- Every in-scope Python test scenario is present in a checked-in port-parity matrix with a Go test
  name or an explicit later-milestone assignment.
- `status`, `pause`, `resume`, and `doctor` have stable machine-readable exit behavior.
- The Sage CLI exposes only the command-knowledge product surface.

### SAGE-002 — One vertical capture slice

Implement the silent atomic spool writer and one adapter selected from the best-supported local
harness. Wire the bounded importer into `IntegratedMonitor.Start(ctx)` and persist normalized events
through an append-only SQLite migration.

Acceptance:

- Installation and removal are idempotent and preserve unrelated user hook configuration.
- The harness continues normally when DevTrack is stopped, storage is unavailable, or input is bad.
- Two repeated clean runs capture the fixture journey once, with no model or network work in-hook.
- Pause/resume is immediate and status/doctor explain backlog and sanitized failures.

Implementation status (2026-09-15): the Go client now has an atomic immutable event spool, a
bounded failure-isolated importer, quarantine handling, durable delivery-key deduplication, and
the append-only `sage_events` SQLite migration. `IntegratedMonitor.Start(ctx)` owns the importer.
The Codex history path reads `state_5.sqlite` and
`thread_history_1.sqlite` with SQLite `mode=ro`, accepts only active `vscode` and `cli` sources,
starts from the time it is enabled, and is gated by either installation state or
`DEVTRACK_SAGE_CODEX_HISTORY=true`. It never reads user messages or persists command output.
Idempotent install/remove preserves unrelated Codex settings; Windows selects history mode and
removes only obsolete DevTrack-owned per-command hooks. The isolated packaged executable journey
passes install, silent capture, privacy-canary, status/backlog, and uninstall checks. SAGE-002 is
complete; sanitized observation from a trusted live Codex hook remains a separate external gate.

### SAGE-003 — Searchable personal command knowledge

Add deterministic command grouping, topic routing, FTS-backed search, source attribution, and
reference-compatible Markdown knowledge files. Introduce local-model distillation behind the
importer. Go owns routing, deduplication, file mutation, signatures, and formatting; the model
returns validated structured title/what/why/example/notes fields and never edits files directly.

Acceptance:

- A clean install can capture a command and retrieve it through `search` end to end.
- Reprocessing produces byte-stable rendered knowledge and no duplicate entries.
- Model outage, timeout, and invalid output retry within bounds and never lose source events.
- Secrets and absolute private paths do not appear in knowledge output or diagnostics.
- Route correction and merge/refile decisions persist and override later classification.
- Safe local commits include only Sage-owned knowledge changes, preserve unrelated work, and never
  push to a remote.

Implementation status (2026-09-20): the first model-free slice is implemented. Imported events
are grouped transactionally by normalized signature into deterministic knowledge records with
source attribution, command-family topics, occurrence/outcome counts, and an FTS5 index.
`devtrack sage search <query> [--topic <topic>]` and `devtrack sage topics` render stable Markdown.
Duplicate deliveries do not inflate knowledge counts. This is infrastructure, not milestone
completion: structured distillation, durable topic Markdown, route correction, merge/refile,
skipped-action records, safe local commits, retry state, and remaining parity scenarios are pending.

Implementation update (2026-09-21): the first structured-distillation boundary now exists. A
neutral context-aware `internal/llmclient` transport supports Ollama and OpenAI-compatible JSON
generation, and `internal/sage/distill` validates structured drafts, distinguishes explicit skips
from retryable failures, and exposes a daemon-owned worker whose `Start` method returns immediately.
This slice is not yet wired to a durable SQLite distillation queue or Markdown writer.

### SAGE-004 — Cross-harness expansion

Add adapters one at a time using recorded, sanitized fixtures for each current hook contract.
Support is declared per event type; missing capabilities degrade explicitly rather than being
inferred.

Acceptance:

- Every advertised harness passes the same silence, latency, deduplication, privacy, install, and
  failure-isolation suite.
- Adapter documentation identifies unsupported events and the verified contract version/date.
- Upgrades preserve user-authored hook entries and existing captured knowledge.

### SAGE-005 — Read-only integration and operational closure

Expose knowledge search and status to the existing MCP server and add bounded retention/repair
tooling.

Acceptance:

- MCP additions remain read-only and do not expose raw event payloads.
- Corrupt spool files are quarantined with sanitized diagnostics; healthy work continues.
- Search/index repair can rebuild from deterministic knowledge records without model calls.

## Test and release strategy

- Translate the pinned Python regression scenarios into Go table-driven tests before deleting or
  replacing behavior. Preserve sanitized input/output fixtures in this repository so CI does not
  depend on the external Python checkout.
- Unit tests: payload normalization, redaction, signatures, limits, state transitions, renderer.
- Golden fixtures: one sanitized fixture set per harness and hook-contract version.
- Fuzz tests: untrusted hook JSON, command tokenization/signatures, redaction, and configuration
  merge/removal boundaries.
- Concurrency tests: importer ownership, retry transitions, spool races, shutdown, and recovery;
  run the relevant packages with `go test -race`.
- Fault injection: unwritable spool, truncated input, locked database, daemon restart, model outage,
  duplicate delivery, and migration replay.
- Integration: disposable config/data/knowledge roots; no mutation of the developer's real
  installation or unrelated Git state.
- Performance: record hook wall time and enforce a small bounded payload; no synchronous dependency.
- Privacy: canary secrets and private paths must be absent from stored normalized data, knowledge,
  logs, and MCP responses.
- Platform CI: run the shared Go suite and hook golden fixtures on Windows and Linux. The release
  cannot retain Python as a fallback for a failed Go path.

Each milestone ships behind an explicit Sage enablement setting until its vertical acceptance suite
passes twice from a clean isolated state. Rollback disables hook capture while retaining local data
for inspection or export.

## Key risks and decisions still needed

1. Harness hook contracts differ and change. Treat each adapter as versioned integration code,
   verified from current official documentation and captured fixtures.
2. Keep the public command surface aligned with capture, knowledge, routing, diagnostics, and
   read-only retrieval.
3. Redaction cannot make arbitrary transcripts safe. Keep normal capture structured and minimal;
   exclude raw output and reasoning from this milestone.
4. Model-generated knowledge can be unstable. Preserve normalized source events and make the
   rendered knowledge deterministic and rebuildable.
5. “Port everything” means parity for the product behavior and tests in the pinned baseline. It
   does not require preserving Python-specific process boundaries, subprocess mechanics, or file
   layouts when DevTrack already has a safer Go-native owner.

## Immediate next step

SAGE-001 and SAGE-002 are complete. SAGE-003 now has its model-free index/search foundation plus
the shared LLM transport, validated structured-draft parser, and non-blocking worker boundary.
After the repository-wide TASK-160 deterministic ticket contract and TASK-159 automatic-time work,
first reconcile every reference scenario with an exact Go test and explicit status in the parity
matrix. Then wire that worker to durable SQLite claims and daemon lifecycle and implement
deterministic topic Markdown, route correction, merge/refile, skipped-action records, safe local
commits, and retry semantics. Keep the parity matrix executable as each slice lands. Do not expand
to another harness until one complete capture-to-self-written-knowledge journey passes twice from
a clean isolated installation.
