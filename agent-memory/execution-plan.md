---
name: Active execution plan
description: Ordered pickup sequence for silent Git validation, ticket and time corrections, SAGE-003, UI integration, and release gates
type: project
---

# Active execution plan

Use this record to resume work. `Data/agent_logs/project_board.md` remains the authority for task
IDs and statuses; verify it before allocating or dispatching a task. Do not start implementation
without an authorized board task and a dedicated branch targeting `dev`.

## Pickup checkpoint

- Current `dev` at `fe3ad38` contains the SAGE-001/SAGE-002 capture foundation, the model-free
  SAGE-003 search slice, removal of legacy Git Sage, neutral `internal/gitcmd` ownership, and the
  SAGE-003 LLM transport, structured distiller, and non-blocking worker from PR #263. Durable SQLite
  queue ownership and daemon lifecycle wiring remain the next Sage implementation boundary.
- TASK-158 merged through PR #264. Native Linux plus explicit TTY/non-TTY qualification remain
  unchecked. A separate local test draft is not part of TASK-161 and is not validation evidence.
- PR #263 and PR #264 each retain a historical failed hosted-Windows unit-test check. On PR #264 the
  60-second job timed out in SQLite-backed `internal/db` and `internal/mcp`. PR #265 later passed
  all 19 branch checks, including the full Windows test job, so the hosted baseline is green again;
  TASK-158's native-Linux TTY/non-TTY qualification remains separate and incomplete.
- TASK-161 owns the rolling `dev` release/update-channel work on
  `features/TASK-161-rolling-dev-channel`. Keep it separate from TASK-158 validation and do not
  claim it as shipped until review and first prerelease publication succeed.
- TASK-157 / SAGE-003 is the active product initiative. TASK-160 and TASK-159 remain allocated to
  the deterministic ticket contract and automatic-time corrections;
  the next unused task ID is TASK-162, subject to board verification.
- The board's active summary now describes the foundation as merged into `dev`; preserve closed
  task history and do not rewrite historical branch references.

## Ordered work

### 1. Close the silent Git native-Linux validation follow-up

1. Preserve the merged TASK-158 implementation and unrelated working-tree changes.
2. Review the separate TTY/non-TTY test draft independently; do not treat its presence as a pass.
3. Run the silence and fail-open path on native Linux in both non-TTY and pseudo-TTY contexts, then
   run the Go suite and `go vet ./...`.
4. Preserve PR #265's successful full hosted-Windows unit-test rerun as the replacement baseline;
   do not erase the historical PR #263/#264 failures or misattribute other green lanes as the test.
5. Record sanitized native-Linux evidence on the board and close the remaining acceptance boxes only
   after both TTY and non-TTY paths pass. Commit, push, or open a PR only when explicitly authorized.

Gate: normal `git commit` is proven silent and fail-open on native Linux in both TTY and non-TTY
execution, with the merged explicit helper still free of post-commit questions.

### 2. Enforce deterministic branch-to-ticket mapping

Implement TASK-160 around one canonical grammar: `<kind>/<ticket-key>-<number>-<slug>`, with the ticket ID
validated by the workspace's configured pattern. Resolution order is canonical branch, explicit
commit prefix/trailer, explicit active-ticket override, then unlinked. Free-form message scanning,
last-ticket reuse, and LLM suggestions are not authoritative mappings. Persist provenance and
confidence, keep nonconformance non-blocking, and surface it later through status/doctor and
correction channels. `initiatives/ticket-mapping.md` is the implementation contract and must be read
before task decomposition or dispatch.

Gate: contradictory-signal tests prove branch precedence; incidental prose IDs and prior mappings
cannot silently link a commit; custom patterns and hot reload work; and every mapping is explainable.

### 3. Implement silent automatic time inference

Implement TASK-159 without adding surveillance. Derive bounded, deterministic work windows from
local commit and session activity, use real last-activity evidence for inactivity/EOD closure, and
retain explicit `work start|stop|adjust` as optional override/correction commands. Preserve measured
and adjusted values for audit, attach confidence, and never ask for duration after a commit.

Gate: time evidence is produced without per-commit input, corrections are auditable, privacy
boundaries are explicit, and clock-controlled tests cover gaps, restarts, EOD, and ticket changes.

### 4. Make SAGE-003 parity executable

Refresh `docs/SAGE_PORT_PARITY_MATRIX.md` against `D:\git_apps\ai_sessions_skills` at `b85a1ab`.
The 135-versus-136 discrepancy is resolved: the omitted reference scenario was
`test_captures_cli_when_explicitly_enabled`, and the matrix now contains all 136 methods. Every
reference scenario must still name its Go owner, exact Go test, status (`implemented`, `partial`,
`pending`, or `adapted`), and adaptation rationale where applicable.

Gate: scenario totals reconcile exactly, every row is actionable, and the matrix—not file
presence or a prose claim—is the completion authority.

### 5. Complete the asynchronous distillation vertical slice

1. Wire the merged non-blocking worker into daemon startup and cancellation-aware shutdown.
2. Implement its queue interface with durable SQLite claims, processing leases, attempts,
   next-retry timestamps, sanitized errors, explicit skip reasons, and completion state.
3. Recover abandoned leases after restart without loss or duplicate completion.
4. Preserve configurable bounded model, polling, and retry timing. Hooks and foreground Git
   operations never wait on Sage.
5. Feed validated drafts into deterministic Markdown while keeping topic/path selection outside the
   model; malformed output and outages remain retryable state.

Gate: one imported event can be claimed durably and distilled asynchronously through the daemon;
startup returns immediately, shutdown cancels promptly, restart recovers expired leases, and
timeout, malformed-output, cancellation, and outage cases retain recoverable work.

### 6. Produce deterministic Markdown knowledge

Implement stable topic filenames, parseable headings/signatures, atomic replacement, stable entry
ordering, idempotent insertion, existing-entry reuse, topic-index maintenance, and path-traversal
protection. SQLite owns processing/search state; Markdown is the portable, user-visible artifact.

Gate: processing the same input twice is byte-stable and duplicate-free, interrupted writes cannot
corrupt an existing knowledge file, and unsafe topic/path values cannot escape the knowledge root.

### 7. Complete routing, safe commits, retries, and diagnostics

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

### 8. Close SAGE-003 end to end

Run the clean Codex capture-to-self-written-knowledge journey twice: install capture, capture a
useful command silently, import once, distill, write deterministic Markdown, index/search it,
correct its route, merge/refile it, recover from a model failure, verify privacy canaries are
absent, preserve unrelated Git state, and uninstall without damaging unrelated configuration.

Require applicable unit, golden, fuzz, fault-injection, race, Windows, and Linux coverage. Record
sanitized evidence and update the parity matrix from actual tests.

Gate: one resilient Codex journey passes twice. Do not add another harness before this gate passes.

### 9. Resolve the server-admin UI branch

Review `feat/TASK-154-server-admin-ui` against current `dev`. Reconcile template, route, test, and
CSS drift, rerun focused functional/accessibility/responsive checks, then either integrate it
through a PR to `dev` or explicitly retire it. Do not mix this decision into SAGE-003 changes.

Gate: no release document or media workflow depends on an unintegrated UI branch.

### 10. Close release follow-ups

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
