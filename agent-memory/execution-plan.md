---
name: Active execution plan
description: Ordered pickup sequence for documentation closure, SAGE-003, UI integration, and release gates
type: project
---

# Active execution plan

Use this record to resume work. `Data/agent_logs/project_board.md` remains the authority for task
IDs and statuses; verify it before allocating or dispatching a task. Do not start implementation
without an authorized board task and a dedicated branch targeting `dev`.

## Pickup checkpoint

- Current `dev` contains the SAGE-001/SAGE-002 capture foundation, the model-free SAGE-003 search
  slice, removal of legacy Git Sage, and neutral `internal/gitcmd` ownership.
- A documentation synchronization is present as documentation-only working-tree changes under
  `agent-memory/` and `devtrack_wiki/wiki/wiki.html`. Review and land or deliberately revise this
  baseline before creating an implementation branch; preserve unrelated work.
- TASK-157 / SAGE-003 is the active product initiative. The next task ID appears to be TASK-158,
  but the board must be checked before allocation.
- The board's active summary now describes the foundation as merged into `dev`; preserve closed
  task history and do not rewrite historical branch references.

## Ordered work

### 1. Close the documentation baseline

1. Review the pending documentation and memory diff against current `dev`.
2. Allocate the verified next task ID and use a `docs/TASK-NNN-*` branch.
3. Reconcile active board wording without rewriting closed history.
4. Validate with `git diff --check`, `python scripts/check_agent_memory.py`, and
   `python devtrack_wiki/check_inline_js.py`.
5. Commit only the authorized documentation and memory files, then open a PR targeting `dev`.

Gate: the next implementation branch starts from a clean, reviewed documentation baseline whose
board, memory, and wiki agree on the current Sage and UI state.

### 2. Make SAGE-003 parity executable

Refresh `docs/SAGE_PORT_PARITY_MATRIX.md` against `D:\git_apps\ai_sessions_skills` at `b85a1ab`.
The 135-versus-136 discrepancy is resolved: the omitted reference scenario was
`test_captures_cli_when_explicitly_enabled`, and the matrix now contains all 136 methods. Every
reference scenario must still name its Go owner, exact Go test, status (`implemented`, `partial`,
`pending`, or `adapted`), and adaptation rationale where applicable.

Gate: scenario totals reconcile exactly, every row is actionable, and the matrix—not file
presence or a prose claim—is the completion authority.

### 3. Build one asynchronous distillation vertical slice

1. Add a neutral `internal/llmclient` transport and Sage-owned orchestration.
2. Keep hooks limited to bounded normalization and atomic spooling; no model or network work may
   occur on the hook path.
3. Default to local Ollama and validate structured `title`, `what`, `why`, `example`, and `notes`
   fields plus an explicit skip verdict.
4. Keep topic/path selection deterministic and outside the model.
5. Treat invalid output and model outages as retryable infrastructure state, never as a skip.

Gate: one imported event can be distilled asynchronously through a tested local-model boundary,
and timeout, malformed-output, cancellation, and outage cases retain recoverable work.

### 4. Produce deterministic Markdown knowledge

Implement stable topic filenames, parseable headings/signatures, atomic replacement, stable entry
ordering, idempotent insertion, existing-entry reuse, topic-index maintenance, and path-traversal
protection. SQLite owns processing/search state; Markdown is the portable, user-visible artifact.

Gate: processing the same input twice is byte-stable and duplicate-free, interrupted writes cannot
corrupt an existing knowledge file, and unsafe topic/path values cannot escape the knowledge root.

### 5. Complete routing, safe commits, retries, and diagnostics

1. Add persistent routes, manual correction, merge/refile, skipped-action records, and the
   `routes`, `route`, `merge`, and `log` commands.
2. Preserve corrections across later classification and let existing documented entries override
   stale inferred routes.
3. Commit only Sage-owned knowledge paths while preserving unrelated staged and unstaged work;
   handle spaces and owned deletions, avoid empty commits, respect auto-commit-off, no-op outside a
   repository, and never push.
4. Persist attempts, next-retry time, sanitized errors, processing leases, skip reasons, and
   completion state. Recover abandoned leases after restart.
5. Make `status`, `doctor`, and `log` distinguish captured, waiting, retrying, processing,
   documented, explicitly skipped, quarantined, and terminally failed states.

Gate: correction and retry behavior survives restarts, Git operations cannot capture unrelated
work, and diagnostics explain every queued event's state without exposing private content.

### 6. Close SAGE-003 end to end

Run the clean Codex capture-to-self-written-knowledge journey twice: install capture, capture a
useful command silently, import once, distill, write deterministic Markdown, index/search it,
correct its route, merge/refile it, recover from a model failure, verify privacy canaries are
absent, preserve unrelated Git state, and uninstall without damaging unrelated configuration.

Require applicable unit, golden, fuzz, fault-injection, race, Windows, and Linux coverage. Record
sanitized evidence and update the parity matrix from actual tests.

Gate: one resilient Codex journey passes twice. Do not add another harness before this gate passes.

### 7. Resolve the server-admin UI branch

Review `feat/TASK-154-server-admin-ui` against current `dev`. Reconcile template, route, test, and
CSS drift, rerun focused functional/accessibility/responsive checks, then either integrate it
through a PR to `dev` or explicitly retire it. Do not mix this decision into SAGE-003 changes.

Gate: no release document or media workflow depends on an unintegrated UI branch.

### 8. Close release follow-ups

1. Qualify the packaged build rather than a source checkout.
2. Capture and approve privacy-reviewed media from the integrated packaged behavior.
3. Record the exact Glama listing path and update the score badge on
   `punkpeye/awesome-mcp-servers` PR #13608; never guess the normalized slug.
4. Treat any remaining directory submissions or launch-post publication as external owner actions
   requiring explicit authorization and authenticated sessions.

## Sequencing guardrails

- Finish one resilient Codex capture-to-knowledge journey before adding Claude Code, Copilot CLI,
  OpenCode, Cursor/IDE history, or Devin CLI adapters.
- Do not expose Sage knowledge through MCP until the local CLI, deterministic files, correction,
  retries, and privacy boundaries are stable. Raw harness events never enter MCP, PostgreSQL,
  telemetry, or remote synchronization.
- Playback, visible-reasoning capture, autonomous Git operations, and raw-activity cloud sync stay
  out of scope.
- Each implementation unit should be one reviewable PR with explicit acceptance evidence and a
  rollback path; PRs target `dev`, never `main`.

## Resume procedure

1. Read `agent-memory/INDEX.md`, `current-state.md`, this file, the relevant initiative record, and
   the active entries at the top of `Data/agent_logs/project_board.md`.
2. Run `GIT_NO_DEVTRACK=1 git status --short` and preserve all pre-existing work.
3. Verify the next task ID and obtain explicit authorization before editing the board or
   dispatching implementation.
4. Start with the earliest incomplete gate above; do not skip ahead because later work appears
   easier.
