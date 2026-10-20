---
name: Active execution plan
description: Ordered pickup sequence for active UI integration and release gates
type: project
---

# Active execution plan

`Data/agent_logs/project_board.md` is the task/status authority. Work from a dedicated branch and
target PRs to `dev`, never `main`.

## Pickup checkpoint

- Sage implementation integration commit: `77e2f5b` after merged PR #292; later documentation-only
  merges may advance `dev` without changing the implementation boundary.
- Sage SAGE-001–SAGE-005 is complete; no approved Sage implementation slice remains pending.
- Reference: `C:\Work\git_apps\ai_sessions_skills` at authoritative clean HEAD `ea2e067`.
- TASK-171 is active on `features/TASK-171-v3.2.0-release` for v3.2.0 qualification and promotion.

## Ordered work

1. Qualify and integrate the v3.2.0 release candidate into `dev`.
2. Reconcile `dev` into `main` through the explicitly authorized promotion PR.
3. Tag the verified `main` commit and verify release assets plus MCP Registry publication.
4. Reconcile TASK-154 separately after release; capture privacy-reviewed media from tested behavior.
5. Record the exact Glama listing path and update the score badge.

## Guardrails

- Sage remains Go-native, offline-first, and local. Hooks stay silent, bounded, and fail-open.
- MCP is read-only local stdio. Do not expose raw capture activity or add remote transport.
- SQLite owns queues and derived indexes; deterministic Markdown is the portable knowledge source.
- Playback, visible-reasoning capture, autonomous Git operations, and raw-activity cloud sync are
  out of scope.
- TASK-171 has owner authorization to create the release PRs, promote `dev` to `main`, tag v3.2.0,
  and verify publication after all recorded gates pass. Do not bypass a failed gate.

## Resume procedure

Read `INDEX.md`, `current-state.md`, this file, the initiative record relevant to the task, and the
active board entry. Inspect Git status and current GitHub state before acting.
