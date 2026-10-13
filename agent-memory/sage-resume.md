---
name: Sage fresh-session handoff
description: Exact resume context, remaining parity groups, and completion gates for TASK-157 / SAGE-003
type: project
---

# Sage fresh-session handoff

Use this record when resuming Sage from a new session. The project board owns task status and IDs;
`docs/SAGE_PORT_PARITY_MATRIX.md` owns scenario-level completion. Recheck Git and GitHub before
acting because branch, PR, and CI state can change after this record is updated.

## Current checkpoint

- Active task: TASK-157 / SAGE-003, Go-native self-writing command knowledge.
- Active branch: `features/TASK-157-sage-closure`, based on `origin/dev` at `549c430`.
- Active review: PR #278, `features/TASK-157-sage-closure` into `dev`.
- Board reconciliation checkpoint: `a49643d`; always re-read branch HEAD before continuing.
- Pinned behavioral reference: `sraj0501/ai_sessions_skills` commit `b85a1ab`.
- Ledger: 136 scenarios — 82 implemented, 1 adapted, 2 partial, and 51 pending.
- Local qualification: full Go tests, `go vet ./...`, ledger consistency, parity-checker tests,
  shared-memory validation, and whitespace validation passed for the current implementation slice.
  Hosted checks must be re-read from PR #278 rather than assumed from this snapshot.

## What is already done

The following queue-to-knowledge foundation is integrated into `dev`: silent Codex capture,
immutable spool/import, SQLite events and durable jobs, leases and fencing, daemon lifecycle,
deterministic Markdown publication, persistent routes, recoverable merge/refile, explicit skipped
records, and full-entry search/topic reconciliation.

PR #278 adds the current closure slice: truthful `status`/`doctor`/`log` diagnostics, bounded
retries, privacy-safe NDJSON logs, Sage-owned local commits that preserve unrelated Git state and
never push, including an isolated private-index path that stages the exact validated knowledge
bytes without depending on ambient Git filters or filesystem monitors; model-free documented-action
reuse; deterministic rendering; conservative section matching; quote-aware compound-command
capture; cross-harness payload normalization;
temperature-zero model requests, token-limit diagnostics, and background-only classify-once
routing with sanitized persistence, deterministic fallback, and existing-section context.

## Remaining parity work

Every open matrix row must gain an exact Go regression test or an explicit behavioral adaptation.
The final ledger command must pass with `--require-complete`.

| Group | Pending | Required outcome |
|---|---:|---|
| Backend and provider behavior | 4 | Render backend placeholders, drop empty model values, handle missing executables, declare prompt modes, and preserve prompt-file failure semantics without adding Python to the Sage runtime. |
| Runtime/knowledge isolation | 3 | Keep knowledge files separate from queue/state/log runtime storage, including relative configuration and explicit knowledge-root overrides. |
| Platform roots and shims | 13 | Correct Windows `APPDATA`/home fallback, Linux XDG/default, macOS, Devin/OpenCode, skill destinations, and Windows/POSIX launcher behavior. |
| Additive install/remove | 5 | Reinstall without duplicate hooks, preserve foreign settings, remove only Sage-owned configuration, purge the known legacy layout, and report a missing target correctly. |
| Daemon/process lifecycle | 10 | Cover exclusive and stale locks, no-console helper behavior, piped input, detached execution, and PID/liveness edge cases, adapted to the DevTrack-owned daemon where appropriate. |
| Configuration, pause, capture, and logging | 5 | Prove process-local configuration, installed-adapter pause behavior, fail-open logging, new/known enqueue records, and repeated actions within one command line. |
| CLI lifecycle | 7 | Cover start/stop/restart, watcher replacement, aliases, loud typo handling, stop reasons, silent hook verbs, and prevention of legacy-shim restart. |
| Optional IDE adapter | 4 | Cover original-command filtering, pause/reset/checkpoint behavior, and failure handling. Do not add another harness before the resilient Codex journey; explicitly adapt these rows if they are outside the current milestone. |
| **Total pending** | **51** | All must become implemented or deliberately adapted. |

The two partial rows are:

1. Codex history terminal collection: add interleaved fast/slow command coverage and repeat-poll
   deduplication.
2. Failed-command exit status: normalization is covered, but the reference-specific history path
   still needs exact parity evidence or an explicit adaptation.

## Acceptance still required

1. Run one clean, resilient Codex capture-to-self-written-knowledge journey twice on Windows and
   twice on Linux: install, silent capture, one import, distillation, deterministic publication,
   search, route correction, merge/refile, model-failure recovery, privacy-canary checks,
   preservation of unrelated staged/unstaged Git state, and safe uninstall.
2. Keep applicable unit, golden, fuzz, fault-injection, and race coverage green.
3. Make `scripts/check_sage_parity.py --require-complete` pass.
4. Obtain required PR #278 checks and review, merge through `dev`, and reconcile the board, parity
   matrix, execution plan, and active memory from the integrated commit.

## Recommended pickup order

1. Backend/provider selection and runtime/knowledge isolation.
2. Platform roots, shims, and additive installer/removal behavior.
3. Daemon/process and CLI lifecycle adaptations.
4. Capture, pause, logging, and the two partial Codex-history rows.
5. Resolve the optional IDE rows without violating the Codex-first sequencing guardrail.
6. Run the repeated Windows/Linux acceptance journey, close the ledger, and integrate PR #278.

At the start of a fresh session, read `agent-memory/INDEX.md`, `current-state.md`,
`execution-plan.md`, `initiatives/sage.md`, this file, and the active TASK-157 board entry. Then run:

```text
git status --short --branch
git log -6 --oneline --decorate
python scripts/check_sage_parity.py
python scripts/check_agent_memory.py
gh pr checks 278
```

The current machine does not have host Go. Use the documented Docker-based Go 1.24 workflow when
that remains true, and keep Ollama on the host unless a test specifically requires a containerized
model endpoint.

## Scope boundaries

- Sage production remains Go-native; Python is development tooling only.
- Hooks stay silent, bounded, fail-open, and detached from model/network/database availability.
- Never capture raw output, user messages, reasoning, or private argument values unnecessarily.
- SQLite owns runtime state; deterministic Markdown is the user-visible knowledge artifact.
- Sage may commit only Sage-owned knowledge files locally and never pushes.
- Playback, visible-reasoning capture, autonomous Git operations, and raw-activity cloud sync are
  out of scope.
- Additional full harness adapters, read-only MCP knowledge exposure, retention/deletion controls,
  repair tools, and Markdown import/export are post-core roadmap work, not TASK-157 closure gates
  unless the board explicitly changes the milestone.
