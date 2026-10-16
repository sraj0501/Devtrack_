---
name: Active execution plan
description: Ordered pickup sequence for active Sage work, UI integration, and release gates
type: project
---

# Active execution plan

`Data/agent_logs/project_board.md` is the task/status authority. Work from a dedicated branch and
target PRs to `dev`, never `main`.

## Pickup checkpoint

- Integration base: `origin/dev` at `1b87ee6` after merged PR #290.
- Active branch: `features/TASK-169-sage-mcp-readonly`; implementation commit `122c06e`.
- Local TASK-169 gates: full Go tests, vet, focused race coverage, and native Windows tests pass.
- Reference: `C:\Work\git_apps\ai_sessions_skills` at authoritative clean HEAD `ea2e067`.
- Next unused task ID: TASK-171, subject to board verification.

## Ordered work

1. Commit and push the completed TASK-169 documentation, open a dedicated PR to `dev`, and require
   all final-head hosted checks to pass.
2. After TASK-169 is integrated, create TASK-170 from current `origin/dev`: retention/deletion,
   quarantine inspection, deterministic rebuild/repair, and Markdown export/import.
3. Resolve or retire the server-admin UI branch.
4. Close packaged-build, privacy-reviewed media, and exact Glama badge follow-ups.

## Guardrails

- Sage remains Go-native, offline-first, and local. Hooks stay silent, bounded, and fail-open.
- MCP is read-only local stdio. Do not expose raw capture activity or add remote transport.
- SQLite owns queues and derived indexes; deterministic Markdown is the portable knowledge source.
- Playback, visible-reasoning capture, autonomous Git operations, and raw-activity cloud sync are
  out of scope.
- Do not merge PRs or publish externally without explicit authorization.

## Resume procedure

Read `INDEX.md`, `current-state.md`, this file, `initiatives/sage.md`, `sage-resume.md`, and the
active board entry. Inspect Git status and current GitHub state before acting.
