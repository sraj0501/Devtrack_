---
name: Active execution plan
description: Ordered basic-workflow acceptance and deferred UI/distribution follow-ups
type: project
---

# Active execution plan

`Data/agent_logs/project_board.md` is the task/status authority. Work from a dedicated branch and
target PRs to `dev`, never `main`.

## Pickup checkpoint

- Basic capture/report fixes, time-tracking removal, and v3.3.0 preparation are on `dev` at
  `860b91b`. The installed host version has not been updated.
- v3.2.0 is the last recorded stable release; development packages use `dev-<commit>`.
- Sage has no pending approved implementation slice; its qualification is on hold.

## Ordered work

1. Qualify the exact packaged development build in an isolated container with PostgreSQL.
2. Run native Windows acceptance, including restart/outage and local-date reporting boundaries.
3. Verify controlled real delivery and the intended model's report quality.
4. Complete a representative workday and reconcile commit/ticket coverage against EOD.

Use [the concrete test plan](../docs/testing/basic-workflow.md). Only after accepting the basics,
resume Sage qualification. Legacy learning repairs and platform parity remain on hold. UI
reconciliation (TASK-154), privacy-reviewed media, the Glama path/badge, and owner-authenticated
directory submissions are deferred behind the immediate daily-use priority.

## Guardrails

- Sage remains Go-native, offline-first, and local. Hooks stay silent, bounded, and fail-open.
- MCP is read-only local stdio. Do not expose raw capture activity or add remote transport.
- SQLite owns queues and derived indexes; deterministic Markdown is the portable knowledge source.
- Playback, visible-reasoning capture, autonomous Git operations, and raw-activity cloud sync are
  out of scope.
- Release completion does not authorize later source changes, directory submissions, or publishing
  through owner accounts. Obtain the relevant authorization for each follow-up.

## Resume procedure

Read `INDEX.md`, `current-state.md`, this file, the initiative record relevant to the task, and the
active board entry. Inspect Git status and current GitHub state before acting.
