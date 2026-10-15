---
name: Active execution plan
description: Ordered pickup sequence for active Sage adapters, UI integration, and release gates
type: project
---

# Active execution plan

`Data/agent_logs/project_board.md` remains the task/status authority. Do not start implementation
without an approved board task and a dedicated branch targeting `dev`.

## Pickup checkpoint

- Current integration branch: `origin/dev` at `f07bb97` after PR #282.
- PRs #283–#285 are clean and fully green. The 136-scenario ledger is complete at 119 implemented,
  17 adapted, 0 partial, and 0 pending; repeated Windows/Linux acceptance passes.
- Latest completed task and branch: TASK-167 on `features/TASK-167-sage-cursor-adapter`. PR #289
  targets `dev`, is mergeable, and all 19 hosted checks pass on reviewed head `88bf9f0`.
- Current reference checkout: `C:\Work\git_apps\ai_sessions_skills` at `ea2e067`.
- Active next Sage task is TASK-168 Devin, followed by TASK-169 read-only MCP
  retrieval/diagnostics and TASK-170 operational closure.
- Next unused task ID is TASK-171, subject to board verification before allocation.

## Current ordered work

1. Implement TASK-168 Devin, preserving the shared silence, privacy, fail-open, additive-install,
   and cross-platform acceptance gates.
2. Review/integrate green PRs #286–#289 only with explicit user authorization.
3. Implement TASK-169 read-only Sage MCP search/topics/status/entry and adapter diagnostics.
4. Implement TASK-170 retention/deletion, quarantine inspection, deterministic rebuild/repair,
   and Markdown export/import.
5. Resolve the server-admin UI branch, then close packaged-build, media, and Glama follow-ups.

## Historical SAGE-003 integration record

PRs #279–#282 integrated runtime isolation, model backends, portable installers, and lifecycle.
PRs #283–#285 contain the remaining green SAGE-003 capture/history, ledger, and acceptance slices.
The detailed historical evidence remains in Git, the project board, and the qualification docs;
it is not active pickup work.

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
   task entry.
2. Inspect Git status and preserve all pre-existing work.
3. Verify task authorization and the next task ID before editing the board or implementation.
4. Start with the earliest incomplete gate above.
