---
name: Active execution plan
description: Ordered pickup sequence for TASK-157, UI integration, and release gates
type: project
---

# Active execution plan

`Data/agent_logs/project_board.md` remains the task/status authority. Do not start implementation
without an approved board task and a dedicated branch targeting `dev`.

## Pickup checkpoint

- Active branch: `features/TASK-157-sage-parity` at `c126a40`.
- Current integration branch: `origin/dev` at `a329b6f` after PR #273; all post-merge workflows
  passed and the rolling release is `dev-a329b6f`.
- TASK-157 is twelve commits ahead of its merge base and one commit behind `dev`; reconcile the
  missing `dev` tip before opening its integration PR.
- Qualified TASK-157 slices: executable 136-scenario ledger, durable SQLite queue/leases, daemon
  lifecycle, deterministic Markdown writer/publication, persistent routes, restart-safe merge/refile,
  skipped-action records, and full-entry search/topic reconciliation.
- Ledger checkpoint: 27 implemented, 1 adapted, 12 partial, 96 pending.
- Next unused task ID is TASK-163, subject to board verification before allocation.

## Ordered work

### 1. Reconcile the active Sage branch

Bring current `dev` into TASK-157 without discarding the qualified branch work. Re-run the focused
Sage, database, daemon, parity-ledger, and cross-platform checks affected by the reconciliation.

Gate: TASK-157 contains PR #273's schema/restart fix and remains cleanly reviewable against `dev`.

### 2. Finish diagnostics and safe local knowledge commits

1. Make `status`, `doctor`, and `log` distinguish captured, waiting, retrying, processing,
   documented, explicitly skipped, quarantined, and terminally failed work without exposing private
   content.
2. Commit only Sage-owned knowledge paths, preserving unrelated staged and unstaged work.
3. Handle spaces and owned deletions, avoid empty commits, respect auto-commit-off, no-op outside a
   repository, and never push.
4. Preserve retry attempts, next-retry time, sanitized errors, leases, skip reasons, and completion
   state across restart.

Gate: diagnostics explain every queued event and Sage Git operations cannot capture unrelated work.

### 3. Close the parity ledger

Use `docs/SAGE_PORT_PARITY_MATRIX.md` as the completion authority. Implement or explicitly adapt
the remaining reference scenarios with exact Go tests and rationale. Keep the matrix checker and
fault-injection lane green.

Gate: all 136 scenarios are implemented or deliberately adapted; `--require-complete` passes.

### 4. Close SAGE-003 end to end

Run the clean Codex capture-to-self-written-knowledge journey twice: install capture, capture a
useful command silently, import once, distill, publish deterministic Markdown, search it, correct
its route, merge/refile it, recover from a model failure, verify privacy canaries are absent,
preserve unrelated Git state, and uninstall without damaging unrelated configuration.

Gate: the resilient journey passes twice on supported Windows and Linux paths with applicable unit,
golden, fuzz, fault-injection, and race coverage.

### 5. Integrate TASK-157 through `dev`

Update the board and documentation from verified evidence, then open a reviewable PR targeting
`dev`. Do not expose raw events through MCP or add another harness before the Codex closure gate.

### 6. Resolve the server-admin UI branch

Review `feat/TASK-154-server-admin-ui` against current `dev`, reconcile template/route/test/CSS
drift, rerun functional/accessibility/responsive checks, and integrate through `dev` or explicitly
retire it.

### 7. Close release follow-ups

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
