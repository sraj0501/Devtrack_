# DevTrack Sage Implementation Plan

## Decision

DevTrack Sage is the product name for DevTrack's local, cross-harness command memory. Its scope is
larger than Git: it captures useful command and tool activity from supported coding harnesses,
turns that activity into searchable personal knowledge, and later supports deliberate session
playback.

The existing `devtrack sage ask`, `do`, `pr`, and interactive commands remain shipped Git-oriented
behavior. They are a compatibility surface, not the architecture for the new capture pipeline.
During migration they will remain available and gain explicit `devtrack sage git ...` aliases.

## First releasable outcome

A user can install one supported harness adapter, work normally, and later find a useful command
through `devtrack sage search` without the hook blocking their harness, calling a model, or sending
raw activity off the machine.

Playback, visible-reasoning capture, automatic cloud synchronization, and autonomous command
execution are not part of this first outcome.

## Product contract

Initial public commands:

```text
devtrack sage status
devtrack sage pause
devtrack sage resume
devtrack sage doctor
devtrack sage install-hooks [--harness <name>]
devtrack sage search <query>
devtrack sage topics
devtrack sage git ask|do|pr|interactive
```

The old `devtrack sage ask|do|pr|interactive` forms remain temporary aliases and emit a migration
notice. The legacy `gitsage` Go package is not renamed or removed during the first milestone.

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
    -> local asynchronous distillation
    -> deterministic knowledge records and search index

CLI / later MCP tools
    -> read-only search, topics, status, and diagnostics
```

The hook hot path must not open SQLite, call a model or network service, wait on the daemon, or
write ordinary stdout/stderr. A spool write failure is observable through diagnostics but must not
break the host harness. The daemon owns retries, processing state, and model work.

Recommended code boundaries:

- `internal/sage`: event contracts, normalization, redaction, spool writer, importer, and service.
- `internal/sage/hooks`: fixture-driven harness adapters and idempotent installers.
- `internal/db`: append-only Sage migrations and query methods using the existing database owner.
- `devtrack_client`: CLI routing and daemon lifecycle integration.
- `internal/llmclient` (later): shared provider transport extracted from `gitsage/llm.go` without
  changing legacy behavior.

## Storage contract

The first schema should store normalized facts, not complete transcripts:

- Event identity, schema version, harness, harness session, event type, tool, and timestamps.
- Redacted command, normalized signature, project identity, success state, and exit code when known.
- Import state, bounded attempt count, and a sanitized last error.
- Knowledge entries, topics, source-event links, and an FTS search index.

Event IDs make imports idempotent. A secondary signature prevents common duplicate hook deliveries
within a session. Raw command output and reasoning are excluded from normal capture. Any future
playback store must be manually activated, visibly indicated, separately retained, and local-only
by default.

## Delivery milestones

### SAGE-001 — Contract and compatibility seam

Define versioned normalized event fixtures, the CLI namespace, data/config/spool roots, redaction
rules, exit behavior, and the legacy alias policy. Add parser and compatibility tests before wiring
a real harness.

Acceptance:

- Malformed, oversized, duplicated, and secret-bearing fixture inputs have deterministic results.
- Existing Git-oriented Sage tests and commands continue to pass.
- `status`, `pause`, `resume`, and `doctor` have stable machine-readable exit behavior.

### SAGE-002 — One vertical capture slice

Implement the silent atomic spool writer and one adapter selected from the best-supported local
harness. Wire the bounded importer into `IntegratedMonitor.Start(ctx)` and persist normalized events
through an append-only SQLite migration.

Acceptance:

- Installation and removal are idempotent and preserve unrelated user hook configuration.
- The harness continues normally when DevTrack is stopped, storage is unavailable, or input is bad.
- Two repeated clean runs capture the fixture journey once, with no model or network work in-hook.
- Pause/resume is immediate and status/doctor explain backlog and sanitized failures.

### SAGE-003 — Searchable personal command knowledge

Add deterministic command grouping, topics, FTS-backed search, source attribution, and Markdown
rendering. Introduce optional local-model distillation behind the importer; deterministic fallback
must remain useful when no model is available.

Acceptance:

- A clean install can capture a command and retrieve it through `search` end to end.
- Reprocessing produces byte-stable rendered knowledge and no duplicate entries.
- Model outage, timeout, and invalid output retry within bounds and never lose source events.
- Secrets and absolute private paths do not appear in knowledge output or diagnostics.

### SAGE-004 — Cross-harness expansion

Add adapters one at a time using recorded, sanitized fixtures for each current hook contract.
Support is declared per event type; missing capabilities degrade explicitly rather than being
inferred.

Acceptance:

- Every advertised harness passes the same silence, latency, deduplication, privacy, install, and
  failure-isolation suite.
- Adapter documentation identifies unsupported events and the verified contract version/date.
- Upgrades preserve user-authored hook entries and existing captured knowledge.

### SAGE-005 — Read-only integration and migration closure

Expose knowledge search and status to the existing MCP server, finalize the Git command migration,
and add bounded retention/repair tooling.

Acceptance:

- MCP additions remain read-only and do not expose raw event payloads.
- Old aliases have tested migration messaging and a separately approved removal policy.
- Corrupt spool files are quarantined with sanitized diagnostics; healthy work continues.

Playback becomes a separate epic only after SAGE-003 is accepted. Its design must not broaden the
normal capture contract implicitly.

## Test and release strategy

- Unit tests: payload normalization, redaction, signatures, limits, state transitions, renderer.
- Golden fixtures: one sanitized fixture set per harness and hook-contract version.
- Fault injection: unwritable spool, truncated input, locked database, daemon restart, model outage,
  duplicate delivery, and migration replay.
- Integration: disposable config/data roots; no mutation of the developer's real installation.
- Performance: record hook wall time and enforce a small bounded payload; no synchronous dependency.
- Privacy: canary secrets and private paths must be absent from stored normalized data, knowledge,
  logs, and MCP responses.

Each milestone ships behind an explicit Sage enablement setting until its vertical acceptance suite
passes twice from a clean isolated state. Rollback disables hook capture while retaining local data
for inspection or export.

## Key risks and decisions still needed

1. Harness hook contracts differ and change. Treat each adapter as versioned integration code,
   verified from current official documentation and captured fixtures.
2. Existing `sage` syntax already belongs to the Git agent. Adopt the compatibility aliases above;
   do not silently change the meaning of an existing command.
3. Redaction cannot make arbitrary transcripts safe. Keep normal capture structured and minimal;
   exclude raw output and reasoning from this milestone.
4. Model-generated knowledge can be unstable. Preserve normalized source events and make the
   rendered knowledge deterministic and rebuildable.
5. Decide which harness is the first supported vertical slice after comparing event completeness,
   installer safety, and the team's actual daily usage.

## Immediate next step

Start SAGE-001 with a short contract spike: collect sanitized fixtures from the candidate harnesses,
score their available lifecycle/tool events, and check in the normalized v1 event schema plus tests.
Do not build playback or all adapters before one complete capture-to-search slice works.
