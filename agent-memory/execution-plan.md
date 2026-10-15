---
name: Active execution plan
description: Ordered pickup sequence for TASK-157, UI integration, and release gates
type: project
---

# Active execution plan

`Data/agent_logs/project_board.md` remains the task/status authority. Do not start implementation
without an approved board task and a dedicated branch targeting `dev`.

## Pickup checkpoint

- Current integration branch: `origin/dev` at `a6a75b0` after PR #278.
- PR #278 integrated the latest qualified TASK-157 closure slice. Required PR checks and the
  applicable post-merge workflows passed; the rolling release is `dev-a6a75b0`.
- Active follow-up review: PR #279 from `features/TASK-157-sage-backend-isolation`, created from
  current `origin/dev`. It contains the runtime/knowledge path-isolation slice and is fully green
  but unmerged. PR #280 is the active stacked review from
  `features/TASK-157-sage-model-backends` into `dev`; all 19 hosted checks passed.
- PR #281 closes portable harness paths, native-launcher adaptations, and additive
  install/removal; all 19 hosted checks passed. PR #282 closes daemon/process and CLI lifecycle
  parity without adding a second Sage watcher; all 19 hosted checks passed. Active implementation
  branch: `features/TASK-157-sage-capture-history`, stacked on PR #282. Commit `215222b` closes
  capture/logging and the remaining Codex-history rows. PR #283 targets `dev`; all 19 hosted checks
  passed and the review is clean. The active stacked branch is
  `features/TASK-157-sage-ide-parity`; commit `836feb7`
  resolves the last four ledger rows without advertising another harness. PR #284 targets `dev`;
  all 19 hosted checks pass. PR #285 carries the final repeated cross-platform acceptance gate;
  all 19 hosted checks pass and the review is clean.
- Qualified TASK-157 slices: executable 136-scenario ledger, durable SQLite queue/leases, daemon
  lifecycle, deterministic Markdown writer/publication, persistent routes, restart-safe merge/refile,
  skipped-action records, full-entry search/topic reconciliation, truthful diagnostics, bounded
  retries, safe local knowledge commits, model-free documented-action reuse, and conservative
  section matching.
- Ledger checkpoint: 119 implemented, 17 adapted, 0 partial, 0 pending. Full Go tests, vet, ledger
  consistency, parity-checker tests, shared-memory validation, and whitespace validation pass on
  the active branch. `scripts/check_sage_parity.py --require-complete` passes.
- Next unused task ID is TASK-164, subject to board verification before allocation.

## Ordered work

### 1. Integrate runtime and knowledge-root isolation

Review the active branch and merge it through `dev` after hosted checks pass. Relative runtime and
knowledge overrides must be stable across daemon and CLI working directories; an explicit knowledge
override must not move private queue, state, or log storage.

Gate: the three isolation rows have exact Go evidence and the focused/full Go suites remain green.

### 2. Integrate explicit model backends

Review the active stacked branch and integrate it through `dev`. Ollama HTTP remains the default;
installed command providers are explicit opt-ins, execute fixed argv without a shell, defer prompt
substitution per request, fail safely when missing, and suppress recursive capture only in their
process tree.

Gate: the five backend/process-local rows have exact Go evidence and hosted Windows checks pass.

### 3. Integrate platform layouts and additive installers

Review the active stacked branch and integrate it through `dev`. The layout layer must remain pure
and must not advertise an unqualified adapter. Installer mutation is restricted to owned or known
legacy entries, preserves foreign settings, is idempotent, and reports missing targets.

Gate: all 18 platform/installer rows have exact Go evidence, full tests/vet and Windows cross-build
pass, and the native-launcher adaptations remain explicit.

### 4. Close the parity ledger

Completed on the active IDE-parity branch. The existing opt-in Codex history adapter owns the four
reference scenarios; no additional harness is advertised. Three rows map directly and the
raw-command/source-default difference is an explicit privacy-preserving adaptation.

Gate: all 136 scenarios are implemented or deliberately adapted; `--require-complete` passes.

### 5. Close SAGE-003 end to end

Completed locally on `features/TASK-157-sage-acceptance`. The clean Codex
capture-to-self-written-knowledge journey runs twice: install capture, capture a
useful command silently, import once, distill, publish deterministic Markdown, search it, correct
its route, merge/refile it, recover from a model failure, verify privacy canaries are absent,
preserve unrelated Git state, and uninstall without damaging unrelated configuration.

Gate: the resilient journey passes twice on supported Windows and Linux paths with applicable unit,
golden, fuzz, fault-injection, and race coverage.

### 6. Integrate the remaining SAGE-003 closure through `dev`

Update the board and documentation from verified evidence, then open a new reviewable PR targeting
`dev`. PR #278 integrated diagnostics and safe local commits; the repeated end-to-end gate passes,
but TASK-157 remains open until the stacked reviews integrate. Do not expose raw events through
MCP or add another harness before the Codex closure gate.

### 7. Resolve the server-admin UI branch

Review `feat/TASK-154-server-admin-ui` against current `dev`, reconcile template/route/test/CSS
drift, rerun functional/accessibility/responsive checks, and integrate through `dev` or explicitly
retire it.

### 8. Close release follow-ups

1. Qualify packaged artifacts rather than a source checkout.
2. Capture and approve privacy-reviewed media from integrated packaged behavior.
3. Record the exact Glama listing path and update the score badge on
   `punkpeye/awesome-mcp-servers` PR #13608.
4. Treat remaining submissions and launch publication as owner-authorized external actions.

## Sequencing guardrails

- Finish one resilient Codex journey before adding other harnesses.
- Do not expose Sage knowledge through MCP until the local CLI, deterministic files, corrections,
  retries, diagnostics, and privacy boundaries are stable.
- Playback, visible-reasoning capture, autonomous Git operations, and raw-activity cloud sync remain
  out of scope.
- PRs target `dev`, never `main`.

## Resume procedure

1. Read `INDEX.md`, `current-state.md`, this file, `initiatives/sage.md`, and the board's active
   TASK-157 entry.
2. Inspect Git status and preserve all pre-existing work.
3. Verify task authorization and the next task ID before editing the board or implementation.
4. Start with the earliest incomplete gate above.
