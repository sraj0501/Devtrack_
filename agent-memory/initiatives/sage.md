---
name: DevTrack Sage implementation plan
description: Active execution plan for cross-harness, self-writing personal command knowledge
type: project
---

The durable implementation plan is `docs/DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md`. The capture,
model-free search, and legacy-removal foundation is integrated into `dev`; this memory records the
remaining execution order and the durable boundaries that future work must preserve.

## Direction

DevTrack Sage is exclusively the local, cross-harness, self-writing command-knowledge product.
It captures useful command/tool activity from supported coding harnesses and turns it into private,
searchable, deterministic personal documentation.

DevTrack Sage is entirely Go-native. Port required behavior and regression coverage from
`F:\git_apps\Personal_Projects\ai_sessions_skills` at authoritative commit `b85a1ab`; production Sage must not
spawn Python or require a Python environment. Port for behavioral parity rather than copying
Python-specific process boundaries.

The reference defines command/tool capture, signatures, deduplication, routing, local-model
distillation, structured entries, deterministic Markdown knowledge files, search,
correction-stable routes, safe local commits, logging, lifecycle control, diagnostics, and
supported harness behavior. Features absent from that reference are outside the parity milestone.

Legacy repository chat and Git-operation behavior is not part of DevTrack Sage. Remove
`sage ask`, `sage do`, `sage pr`, `sage interactive`, free-form question routing, Git aliases,
legacy provider configuration, and unused agent code. Relocate only independently used Git or LLM
utilities into neutral packages after auditing their consumers.

Playback, visible-reasoning capture, autonomous Git operations, and raw-activity cloud sync are
explicit non-goals.

## Product surface

```text
devtrack sage status
devtrack sage pause|resume
devtrack sage search <query> [--topic <topic>]
devtrack sage topics
devtrack sage doctor
devtrack sage log
devtrack sage routes
devtrack sage route <binary> <topic>
devtrack sage merge <source> <destination>
devtrack sage harness list|install|uninstall
```

A hidden, non-interactive hook endpoint accepts supported harness events. Hooks must return
quickly, print nothing, perform no model or network call, and never break an agent session.

## Current state

- `dev` contains the event contract, silent Codex capture, atomic spool, bounded
  importer/quarantine, SQLite events, pause/resume, and idempotent harness lifecycle.
- The current SAGE-003 slice is model-free: deterministic grouping by normalized signature,
  source attribution, command-family topics, FTS5 search, and the `search`/`topics` CLI.
- PR #263 merged the neutral, context-aware LLM transport, Sage-owned structured distiller, and
  non-blocking background worker into `dev`. Model timeout, idle polling, and retry delay are bounded
  and configurable for slower offline models. The TASK-157 branch now connects the worker to a
  durable SQLite queue and daemon lifecycle; upstream integration is still pending.
- Remaining SAGE-003 work begins with deterministic Markdown,
  then routing/corrections, safe commits, persisted retries, diagnostics, and capture-to-knowledge
  closure.

## Ordered next steps

### 1. Make reference parity executable

2026-09-30: `features/TASK-157-sage-parity` reconciles all 136 scenario names and source lines
against pinned Git objects from the relocated checkout above. Every row names existing evidence
or an exact planned closure test. `scripts/check_sage_parity.py` validates the ledger and test
references; `--reference PATH` verifies the baseline and `--require-complete` rejects open rows.
CI runs ledger validation and eight fault-injection checks. The conservative inventory is six
implemented, fifteen partial, and 115 pending; these are coverage classifications, not a product
completion percentage. Missing terminal-transition ordering and full-entry search assertions
remain partial. New real-HTTP tests cover empty model output remaining retryable and uncapped
background requests. Focused Go tests/vet passed on Windows and Fedora WSL2. Durable queue/daemon
wiring is still the next implementation slice; Sage remains open.

Refresh `docs/SAGE_PORT_PARITY_MATRIX.md` against all 136 reference tests at `b85a1ab`. Every row
must identify the Go owner, exact Go test, status (`implemented`, `partial`, `pending`, or
`adapted`), and the reason for any Go-specific adaptation. The matrix, not file presence, is the
completion authority.

### 2. Connect asynchronous distillation to durable daemon state

Implemented on the TASK-157 branch on 2026-09-30. `sage_jobs` is populated transactionally with
knowledge insertion; migration backfills older signatures. Atomic claims carry random fencing
tokens, bounded leases, attempts, and retry times. Expired claims recover after restart; stale
workers cannot acknowledge replacement claims. Daemon-owned cancellation stops importer/model
work before the database closes. Pause prevents new claims; an already-running request may finish.
Only local Ollama configuration is loaded by this daemon path. Valid drafts persist as `distilled`,
which does not mean Markdown has been written. Error/skip diagnostics use fixed safe categories.
Evidence: `docs/SAGE_DURABLE_QUEUE_QUALIFICATION.md`. Final Sage parity remains open.

- Wire the merged Sage background worker into daemon startup and shutdown; startup must remain
  non-blocking and cancellation must promptly end polling, retry waits, and in-flight requests.
- Back the worker queue with SQLite claims, processing leases, attempt counts, next-retry time,
  sanitized errors, explicit skip reasons, and completion state. Recover abandoned leases after
  restart without losing or duplicating work.
- Keep offline-model timing configurable through `DEVTRACK_SAGE_MODEL_TIMEOUT_SECS`,
  `DEVTRACK_SAGE_IDLE_POLL_MS`, and `DEVTRACK_SAGE_RETRY_DELAY_SECS`.
- Keep hooks and foreground Git operations detached from model availability and queue progress.
  Invalid output and model outages remain retryable; only an explicit validated verdict may skip.

### 3. Write deterministic Markdown knowledge

2026-09-30 checkpoint: the writer library now implements portable filenames, fenced-code-aware
parsing, deterministic rendering, section insertion, signature reuse/attachment, atomic file
replacement, and replayable topic-index repair. Seven reference rows now have executable Go tests;
Windows full suite/vet and Linux writer race/fuzz/vet pass. Evidence:
`docs/SAGE_MARKDOWN_QUALIFICATION.md`. Durable publication is now wired through a separate
SQLite publication queue, configurable knowledge root, and stable signature markers. File/index
failures retry without repeating the model; daemon restart preserves exact Markdown bytes.
See `docs/SAGE_PUBLICATION_QUALIFICATION.md` for transaction and filesystem limits.
Do not mark a job documented merely because its draft exists. Routing/action-equivalence and
fuzzy section matching remain open alongside the later steps below.

Implement stable topic filenames, parseable headings/signatures, atomic replacement, stable entry
ordering, idempotent insertion, existing-entry reuse, topic-index maintenance, and path traversal
protection. SQLite owns queue/processing/search state; Markdown topic files are the durable,
user-visible artifacts. Reprocessing the same events must be byte-stable and duplicate-free.

### 4. Add routing and correction operations

2026-10-01: persistent routes, `routes`/`route`, existing-entry precedence after manual moves,
and publication integration are implemented and locally qualified. Windows suite/vet and Linux
focused race/vet pass. Nine additional reference scenarios are covered (22 implemented, 15 partial,
99 pending). See `docs/SAGE_ROUTING_QUALIFICATION.md`. Next implement restart-safe merge/refile;
simple-command routing does not close compound-action parsing or full-entry search.

Implement persistent routes, manual correction, merge/refile, skipped-action records, sanitized
topic/filename handling, and the `routes`, `route`, `merge`, and `log` commands. Existing documented
entries override stale inferred routes, and user corrections survive later classification.

### 5. Add safe local knowledge commits

Commit only Sage-owned knowledge paths while preserving unrelated staged and unstaged work. Skip
ownership-ambiguous files, handle spaces and Sage-owned deletions, avoid empty commits, respect an
auto-commit-off setting, and no-op outside Git repositories. Sage never pushes.

### 6. Finish retries and diagnostics

Persist attempt count, next retry time, sanitized last error, processing lease, skip reason, and
completion state. `status`, `doctor`, and `log` must distinguish captured, waiting, retrying,
processing, documented, explicitly skipped, quarantined, and terminally failed work. Restart must
recover abandoned leases without loss or duplication.

### 7. Close SAGE-003

Run the clean capture-to-self-written-knowledge journey twice: install Codex capture, capture a
useful command silently, import once, distill, write deterministic Markdown, index/search it,
correct its route, merge/refile it, recover from a model failure, verify privacy canaries are
absent, preserve unrelated Git state, and uninstall without damaging unrelated configuration.
Require unit, golden, fuzz, fault-injection, race, Windows, and Linux coverage for the applicable
parity scenarios.

### 8. Expand harness support

Finish Codex first, then add Claude Code, Copilot CLI, OpenCode, Cursor/IDE history, and Devin CLI
one at a time. Do not advertise an adapter until it passes the shared silence, latency, privacy,
deduplication, fail-open, install/removal, configuration-preservation, and unsupported-event suite.

### 9. Add read-only integration and operational closure

After the local CLI is stable, expose read-only MCP knowledge search/topics/status/entry retrieval.
Add retention/deletion controls, quarantine inspection, deterministic index rebuild, repair, and
Markdown export/import. Raw harness events must never enter MCP, `client_events`, PostgreSQL,
telemetry, or remote synchronization.

## Durable invariants

- Hooks only normalize and atomically spool bounded events; they never open SQLite or wait for the
  daemon, model, Python, or a network service.
- SQLite owns queues, retry/process state, and search indexes. Deterministic Markdown is the
  portable human-readable knowledge artifact.
- Normal capture excludes raw output, user messages, reasoning, and private argument values where
  possible.
- Runtime queues/logs are private and gitignored. Knowledge may be committed locally only through
  the safe Sage-owned path; Sage never pushes.
- A model failure is retryable infrastructure state, not a judgment that an event is useless.
- Complete one resilient capture-to-self-written-knowledge journey before adding more harnesses.
