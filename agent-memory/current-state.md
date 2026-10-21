---
name: Project current state
description: Active implementation work, integration state, and unresolved release follow-ups
type: project
---

**Immediate user priority (2026-10-21):** qualify basic Git capture and EOD reports for an upcoming
real use case before testing Sage. The owner explicitly removed time tracking as micromanagement;
work durations, sessions, corrections, and time metrics are no longer product requirements.
The working tree removes runtime time collection, CLI/session APIs, chat commands, report metrics,
and session synchronization; legacy tables remain inert for upgrade compatibility. The public
v3.2.0 package still contains the old behavior until a new build is installed.
Legacy learning repairs and platform parity remain on hold. Working-tree fixes now enumerate
unseen commits across local branch tips and read server report/queue history over authenticated
HTTP with `server:<id>` review identities. Container regression and full-suite results are recorded
in the qualification initiative; no installed/public package has been updated.
See [basic workflow qualification](initiatives/basic-workflow-qualification.md). On 2026-10-22,
the owner authorized the v3.3.0 version update and push to `dev`; stable publication and host
installation remain separate. Windows metadata and CI candidate bundles target 3.3.0; the rolling
development channel retains its `dev-<commit>` version convention. The next acceptance gates are
recorded in [the daily-use test plan](../docs/testing/basic-workflow.md). No board mutation is included.

**Sage product state:** The approved SAGE-001–SAGE-005 roadmap is complete and integrated into
`dev` at `77e2f5b`. PR #291 merged read-only MCP retrieval at `34aafb8`; PR #292 merged dry-run
retention, bounded opaque quarantine controls, exact deletion, deterministic repair/cache rebuild,
and validated Markdown export/import after all 19 hosted checks passed.

**Sage integration state:** The Sage implementation integration commit is `77e2f5b` after PR #292;
post-merge documentation reconciliation followed in PR #293. PRs #279–#292 are integrated,
including the complete 136-row parity ledger (119 implemented, 17 adapted), repeated clean
Windows/Linux acceptance, and all six independent capture adapters. The authoritative reference is
`C:\Work\git_apps\ai_sessions_skills` at clean HEAD `ea2e067`; its newer bug fixes are intentional.
No approved Sage implementation slice remains pending.

**Distribution state:** v3.2.0 is the latest stable public release. Five native MCPB bundles and
the official MCP Registry record for `io.github.sraj0501/devtrack` are published.

**Active UI work:** The server-admin redesign remains isolated on
`feat/TASK-154-server-admin-ui` at `54ceb05`. It is explicitly deferred from v3.2.0 because the old
branch predates canonical-memory migration and needs a separate drift/UI-acceptance pass.

**Active distribution follow-ups:** qualify the packaged build rather than a source checkout,
capture privacy-reviewed media from real behavior, and record the exact Glama path before updating
the score badge. Third-party directory forms remain owner-account actions.

**Known communication-learning gap:** Managed onboarding can seed voice data from local Git
history, but several Python HTTP learning/status/sync/profile adapters are incomplete. Treat those
CLI paths and Teams/Outlook learning as unavailable until repaired.

**Storage boundary:** Go remains SQLite-only. Python requires `POSTGRES_URL`, validates it, and
applies Alembic before serving; client-event sync is opt-in and idempotent.

**MCP boundary:** `devtrack mcp` is local read-only stdio. TASK-169's Sage tools read the current
deterministic Markdown snapshot and bounded SQLite diagnostics. Raw Sage events, logs, prompts,
output, reasoning, and adapter configuration paths never enter MCP, PostgreSQL, telemetry, or
remote synchronization.

**Authority:** `Data/agent_logs/project_board.md` owns task status and IDs; GitHub
`sraj0501/Devtrack_` owns source and releases.
