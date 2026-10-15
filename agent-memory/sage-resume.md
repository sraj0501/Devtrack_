---
name: Sage fresh-session handoff
description: Exact resume context and remaining acceptance gates for TASK-157 / SAGE-003
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
  checks passed, but it remains unmerged. PR #280 and PR #281 are fully green stacked reviews for
  command-model backends and platform installers. Active stacked branch:
  `features/TASK-157-sage-capture-history`, based on PR #282's green lifecycle head. PR #282
  targets `dev`; all 19 hosted checks passed. Commit `215222b` closes capture/logging and the two
  partial Codex-history rows. PR #283 targets `dev`; all 19 hosted checks passed and the review is
  clean. The active stacked branch is `features/TASK-157-sage-ide-parity`; commit `836feb7`
  resolves the final four rows. PR #284 targets `dev`; all 19 hosted checks pass. PR #285 carries
  the twice-run Windows/Linux acceptance gate; all 19 hosted checks pass and the review is clean.
- Integrated implementation head: `4bb74f6`; merge commit: `a6a75b0`.
- Behavioral reference: `C:\Work\git_apps\ai_sessions_skills` at current commit `ea2e067`.
  Its post-`b85a1ab` changes affect generated knowledge only; the 136 reference tests are unchanged.
- Ledger: 136 scenarios — 119 implemented, 17 adapted, 0 partial, and 0 pending;
  `--require-complete` passes.
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

PR #282's lifecycle branch resolves all 10 daemon/process and 7 CLI lifecycle rows. DevTrack's
single daemon owns locking, liveness, and background work; Windows native children have no console,
and Sage start/stop/restart aliases manage persistent capture state without spawning a watcher.
See `docs/SAGE_LIFECYCLE_QUALIFICATION.md`.

## Parity ledger

All 136 rows have exact Go evidence or an explicit behavioral adaptation. The existing opt-in
Codex history adapter closes the four optional IDE rows without advertising another harness; see
`docs/SAGE_IDE_PARITY_QUALIFICATION.md`. There are no partial or pending rows.

## Acceptance status

The clean, resilient Codex capture-to-self-written-knowledge journey passes twice on native Windows
and twice on Linux. It covers install, silent capture, one import, model outage and malformed-output
recovery, deterministic publication, search, route correction, merge/refile, restart stability,
privacy canaries, unrelated Git-state preservation, and safe uninstall. Applicable full-suite, vet,
race, and `--require-complete` gates pass. See `docs/SAGE_END_TO_END_ACCEPTANCE.md`.

## Recommended pickup order

1. Review and integrate IDE-parity PR #284 after its hosted checks pass.
2. Review the acceptance slice and integrate the remaining stacked closure through `dev`.

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
