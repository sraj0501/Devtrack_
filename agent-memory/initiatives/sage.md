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
`C:\Work\git_apps\ai_sessions_skills` at authoritative commit `ea2e067`; production Sage must not
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

- `dev` through PR #282 contains the event contract, silent Codex capture, atomic spool, bounded
  importer/quarantine, SQLite events, the complete durable knowledge pipeline, pause/resume, and
  idempotent harness lifecycle.
- The current SAGE-003 slice is model-free: deterministic grouping by normalized signature,
  source attribution, command-family topics, FTS5 search, and the `search`/`topics` CLI.
- PR #263 merged the neutral, context-aware LLM transport, Sage-owned structured distiller, and
  non-blocking background worker into `dev`. Model timeout, idle polling, and retry delay are bounded
  and configurable for slower offline models.
- PR #275 integrated the TASK-157 durable SQLite queue and daemon lifecycle, deterministic Markdown
  publication, persistent routing, restart-safe merge/refile, skipped-action records, and
  full-entry search/topic reconciliation into `dev` at `a1777b6`.
- PR #278 merged the latest SAGE-003 closure slice to `dev` at `a6a75b0`. It includes
  truthful diagnostics, bounded retries, safe local knowledge commits, model-free action reuse,
  quote-aware compound command signatures, cross-harness payload normalization, deterministic
  temperature-zero model requests, token-limit diagnostics, and background classify-once routing
  with safe fallback and existing-section context. Required PR checks and all applicable post-merge
  workflows passed.
- PRs #279–#282 are merged to `dev` at `f07bb97`. PRs #283–#285 are clean and fully green, carrying
  capture/history closure, the complete 119-implemented/17-adapted ledger, and repeated native
  Windows/Linux acceptance.
- TASK-164 adds Claude Code beside Codex with independently selected install/uninstall, additive
  owned hooks, bounded Bash/search/edit normalization, Bash failure capture, and no second watcher;
  green PR #286 targets `dev`. TASK-165 adds GitHub Copilot CLI through one owned
  version-1 `postToolUse` entry, portable shell commands, and bounded shell/search/edit
  normalization; PR #287 is mergeable and all 19 final-head checks pass. See
  `docs/SAGE_CLAUDE_ADAPTER_QUALIFICATION.md` and `docs/SAGE_COPILOT_ADAPTER_QUALIFICATION.md`.
- TASK-166 adds OpenCode through one owned user plugin, reference-compatible before/after tool
  observation, direct native execution, bounded normalization, and no session lifecycle process.
  PR #288 is mergeable and all 19 hosted checks pass on final head `adca3e7`. TASK-167 adds Cursor
  through merged `afterShellExecution` and matched `postToolUse` entries, bounded shell/write
  normalization, deterministic cross-hook identity, and no session process. Its implementation and
  local cross-platform gates pass; PR #289 is mergeable and all 19 hosted checks pass on reviewed
  head `a4142e6`. TASK-168 adds Devin through one platform-root `PostToolUse` hook with bounded
  shell/search/glob/edit/write/patch normalization and no session process. Local cross-platform
  gates and two documentation passes succeed; hosted review remains. See `docs/SAGE_OPENCODE_ADAPTER_QUALIFICATION.md`,
  `docs/SAGE_CURSOR_ADAPTER_QUALIFICATION.md`, and `docs/SAGE_DEVIN_ADAPTER_QUALIFICATION.md`.

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

Refresh `docs/SAGE_PORT_PARITY_MATRIX.md` against all 136 reference tests at `ea2e067`. Every row
must identify the Go owner, exact Go test, status (`implemented`, `partial`, `pending`, or
`adapted`), and the reason for any Go-specific adaptation. The matrix, not file presence, is the
completion authority.

### 2. Connect asynchronous distillation to durable daemon state

Implemented on the TASK-157 branch on 2026-09-30. `sage_jobs` is populated transactionally with
knowledge insertion; migration backfills older signatures. Atomic claims carry random fencing
tokens, bounded leases, attempts, and retry times. Expired claims recover after restart; stale
workers cannot acknowledge replacement claims. Daemon-owned cancellation stops importer/model
work before the database closes. Pause prevents new claims; an already-running request may finish.
Ollama HTTP remains the default daemon model path. An explicitly configured installed command
provider can instead run through fixed argv without a shell; missing providers leave durable work
retryable. Valid drafts persist as `distilled`, which does not mean Markdown has been written.
Error/skip diagnostics use fixed safe categories. Evidence:
`docs/SAGE_DURABLE_QUEUE_QUALIFICATION.md` and `docs/SAGE_MODEL_BACKEND_QUALIFICATION.md`. Final
Sage parity remains open.

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
99 pending). See `docs/SAGE_ROUTING_QUALIFICATION.md`. Restart-safe merge/refile is now also
locally qualified (`docs/SAGE_MERGE_QUALIFICATION.md`): journal replay, edit-conflict detection,
route repointing, and index repair. The ledger is now 24 implemented, 15 partial, 97 pending.
Skipped-action records and full-entry search are now locally qualified (2026-10-01).
Terminal skips publish atomic, idempotent `_skipped.md` signatures with a fixed safe reason;
publication failure never repeats model work. Search and topics transactionally refresh a SQLite
cache from whole Markdown entries and reconcile moved/deleted publication filenames. Literal AND
keywords, Unicode case folding, topic filtering, and reference ranking are covered. Windows full
suite/vet and Linux focused race/vet pass. Ledger: 27 implemented, 1 adapted, 12 partial, 96 pending.
See `docs/SAGE_SKIPPED_SEARCH_QUALIFICATION.md`. Next: diagnostics and safe local commits.
Simple-command routing does not close compound-action parsing or action equivalence.

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

Completed locally on 2026-10-15. The clean capture-to-self-written-knowledge journey runs twice on
native Windows and Linux: install Codex capture, capture a
useful command silently, import once, distill, write deterministic Markdown, index/search it,
correct its route, merge/refile it, recover from a model failure, verify privacy canaries are
absent, preserve unrelated Git state, and uninstall without damaging unrelated configuration.
Require unit, golden, fuzz, fault-injection, race, Windows, and Linux coverage for the applicable
parity scenarios. Evidence: `docs/SAGE_END_TO_END_ACCEPTANCE.md`.

### 8. Expand harness support

Codex, Claude Code, Copilot CLI, OpenCode, Cursor, and Devin CLI are implemented as independent
adapters. Do not advertise an adapter until it passes the
shared silence, latency, privacy,
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
