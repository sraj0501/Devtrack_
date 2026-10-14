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
- Integration branch: `origin/dev` at `a6a75b0` after PR #278 merged on 2026-10-13.
- Active review: PR #279 from `features/TASK-157-sage-backend-isolation` into `dev`; all 19 hosted
  checks passed, but it remains unmerged. Active stacked branch:
  `features/TASK-157-sage-platform-installers`, based on PR #280's fully green model-backend head.
- Integrated implementation head: `4bb74f6`; merge commit: `a6a75b0`.
- Behavioral reference: `C:\Work\git_apps\ai_sessions_skills` at current commit `ea2e067`.
  Its post-`b85a1ab` changes affect generated knowledge only; the 136 reference tests are unchanged.
- Ledger: 136 scenarios — 105 implemented, 4 adapted, 2 partial, and 25 pending.
- Qualification: full Go tests, `go vet ./...`, ledger consistency, parity-checker tests,
  shared-memory validation, and whitespace validation passed locally. All required PR #278 checks
  and applicable post-merge workflows passed, including hosted Windows tests.

## What is already done

The following queue-to-knowledge foundation is integrated into `dev`: silent Codex capture,
immutable spool/import, SQLite events and durable jobs, leases and fencing, daemon lifecycle,
deterministic Markdown publication, persistent routes, recoverable merge/refile, explicit skipped
records, and full-entry search/topic reconciliation.

PR #278 integrated the latest closure slice: truthful `status`/`doctor`/`log` diagnostics, bounded
retries, privacy-safe NDJSON logs, Sage-owned local commits that preserve unrelated Git state and
never push, including an isolated private-index path that stages the exact validated knowledge
bytes without depending on ambient Git filters or filesystem monitors; model-free documented-action
reuse; deterministic rendering; conservative section matching; quote-aware compound-command
capture; cross-harness payload normalization;
temperature-zero model requests, token-limit diagnostics, and background-only classify-once
routing with sanitized persistence, deterministic fallback, and existing-section context.

The active follow-up resolves all three runtime/knowledge isolation rows: relative runtime and
knowledge overrides resolve from the stable Sage root, an explicit knowledge directory cannot move
private runtime state, and the Go prompt is explicitly path-free rather than exposing a runtime
batch path. See `docs/SAGE_RUNTIME_ISOLATION_QUALIFICATION.md`.

The active stacked branch resolves the four backend/provider rows plus process-local capture
suppression. Ollama HTTP remains the default; installed command providers are explicit opt-ins,
use direct argv with declared prompt modes, and cannot recursively capture their own subprocess.
See `docs/SAGE_MODEL_BACKEND_QUALIFICATION.md`.

The active stacked branch resolves all 18 platform-layout and additive installer/removal rows.
Eleven layout rows and five mutation rows are implemented; two Python script-shim rows are
explicitly adapted to packaged native Go launchers. Codex remains the only advertised adapter.
See `docs/SAGE_PLATFORM_INSTALLER_QUALIFICATION.md`.

## Remaining parity work

Every open matrix row must gain an exact Go regression test or an explicit behavioral adaptation.
The final ledger command must pass with `--require-complete`.

| Group | Pending | Required outcome |
|---|---:|---|
| Daemon/process lifecycle | 10 | Cover exclusive and stale locks, no-console helper behavior, piped input, detached execution, and PID/liveness edge cases, adapted to the DevTrack-owned daemon where appropriate. |
| Configuration, pause, capture, and logging | 4 | Prove installed-adapter pause behavior, fail-open logging, new/known enqueue records, and repeated actions within one command line. |
| CLI lifecycle | 7 | Cover start/stop/restart, watcher replacement, aliases, loud typo handling, stop reasons, silent hook verbs, and prevention of legacy-shim restart. |
| Optional IDE adapter | 4 | Cover original-command filtering, pause/reset/checkpoint behavior, and failure handling. Do not add another harness before the resilient Codex journey; explicitly adapt these rows if they are outside the current milestone. |
| **Total pending** | **25** | All must become implemented or deliberately adapted. |

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

## Recommended pickup order

1. Daemon/process and CLI lifecycle adaptations.
2. Capture, pause, logging, and the two partial Codex-history rows.
3. Resolve the optional IDE rows without violating the Codex-first sequencing guardrail.
4. Run the repeated Windows/Linux acceptance journey, close the ledger, and integrate the remaining
   closure through a new PR to `dev`.

At the start of a fresh session, read `agent-memory/INDEX.md`, `current-state.md`,
`execution-plan.md`, `initiatives/sage.md`, this file, and the active TASK-157 board entry. Then run:

```text
git status --short --branch
git log -6 --oneline --decorate origin/dev
python scripts/check_sage_parity.py
python scripts/check_agent_memory.py
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
