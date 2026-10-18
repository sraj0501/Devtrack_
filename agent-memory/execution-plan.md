---
name: Active execution plan
description: Ordered pickup sequence for active UI integration and release gates
type: project
---

# Active execution plan

`Data/agent_logs/project_board.md` is the task/status authority. Work from a dedicated branch and
target PRs to `dev`, never `main`.

## Pickup checkpoint

- Integration base: `origin/dev` at `77e2f5b` after merged PR #292.
- Sage SAGE-001–SAGE-005 is complete; no approved Sage implementation slice remains pending.
- Reference: `C:\Work\git_apps\ai_sessions_skills` at authoritative clean HEAD `ea2e067`.
- Next unused task ID: TASK-171, subject to board verification.

## Ordered work

1. Resolve or retire the server-admin UI branch.
2. Close packaged-build, privacy-reviewed media, and exact Glama badge follow-ups.

## Guardrails

- Sage remains Go-native, offline-first, and local. Hooks stay silent, bounded, and fail-open.
- MCP is read-only local stdio. Do not expose raw capture activity or add remote transport.
- SQLite owns queues and derived indexes; deterministic Markdown is the portable knowledge source.
- Playback, visible-reasoning capture, autonomous Git operations, and raw-activity cloud sync are
  out of scope.
- Do not merge PRs or publish externally without explicit authorization.

## Resume procedure

Read `INDEX.md`, `current-state.md`, this file, the initiative record relevant to the task, and the
active board entry. Inspect Git status and current GitHub state before acting.
