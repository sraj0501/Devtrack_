---
name: Project current state
description: Active implementation work, integration state, and unresolved release follow-ups
type: project
---

**Active product initiative:** TASK-170 / SAGE-005 closes local Sage operations on
`features/TASK-170-sage-operational-closure`, stacked from fully green TASK-169 head `bf963dd`.
Retention dry-runs by default; quarantine inspection is opaque and bounded; exact deletion,
repair/cache rebuilding, and validated Markdown export/import are implemented. Full Linux Go
tests/vet, focused race/fault coverage, native Windows focused tests, and two documentation passes
pass locally. All 19 hosted checks pass on PR #292; integration remains and requires explicit
authorization.

**Sage integration state:** `origin/dev` is `1b87ee6` after PR #290. PRs #279–#290 are integrated,
including the complete 136-row parity ledger (119 implemented, 17 adapted), repeated clean
Windows/Linux acceptance, and all six independent capture adapters. The authoritative reference is
`C:\Work\git_apps\ai_sessions_skills` at clean HEAD `ea2e067`; its newer bug fixes are intentional.
TASK-169 PR #291 remains open/mergeable. TASK-170 is the final approved Sage roadmap slice.

**Distribution state:** Stable v3.1.1 remains the latest public stable release. Recheck the rolling
`dev` release boundary from GitHub before publication. `main` and `dev` have diverged in ancestry;
do not assume a simple fast-forward.

**Active UI work:** The server-admin redesign remains isolated on
`feat/TASK-154-server-admin-ui` at `54ceb05`. Review and reconcile it against current routes and
templates or explicitly retire it before packaged-build qualification and media capture.

**Unresolved release follow-ups:** packaged-build acceptance; privacy-reviewed media; exact Glama
listing path and score-badge update on awesome-mcp-servers PR #13608.

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
