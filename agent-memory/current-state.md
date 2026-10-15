---
name: Project current state
description: Active implementation work, integration state, and unresolved release follow-ups
type: project
---

**Active product initiative:** TASK-166 / SAGE-004 adds OpenCode as the fourth independently
managed Sage capture adapter on `features/TASK-166-sage-opencode-adapter`. The
authoritative reference is `C:\Work\git_apps\ai_sessions_skills` at current clean HEAD `ea2e067`;
its newer HEAD is intentional and includes the reference bug fixes. OpenCode installs one owned
user plugin that observes `tool.execute.before` and `tool.execute.after`, directly invokes the
packaged Go endpoint, preserves unknown plugin content, and adds no session process. It normalizes
only bounded privacy-minimal shell/search/edit events. Generated-plugin syntax, focused Linux and
native Windows tests, the full Go suite, vet, and modified-package race coverage pass locally.
TASK-165 PR #287 is mergeable and all 19 final-head checks pass at `2568b72`; TASK-166 docs and PR
review are complete. TASK-166 commit `ac17e66` is pushed in PR #288 targeting `dev`; hosted
final-head checks are the active gate.

**Sage integration state:** PRs #279–#282 are merged to `dev` at `f07bb97`. PRs #283–#285 are
clean, fully green reviews carrying capture/history closure, the complete 136-row parity ledger
(119 implemented, 17 adapted, 0 partial, 0 pending), and the twice-run native Windows/Linux
acceptance journey. Playback, visible-reasoning capture, autonomous Git operations, and raw-event
cloud sync remain out of scope.

**Integration state:** Current `origin/dev` is `f07bb97` after PR #282. PRs #283–#285 remain
stacked, clean, and green pending integration.

**Distribution state:** Stable v3.1.1 remains the latest public stable release. The rolling `dev`
release boundary must be rechecked from GitHub before publication; current `origin/dev` is
`f07bb97`. `main` and `dev` have diverged in Git
ancestry and require deliberate reconciliation before a future promotion; do not assume a simple
fast-forward.

**Active UI work:** The full server-admin redesign remains isolated on
`feat/TASK-154-server-admin-ui` at `54ceb05` and is not integrated into current `dev`. Review and
reconcile it against current templates/routes or explicitly retire it before packaged-build
qualification and media capture. `initiatives/server-ui.md` owns the integration gate.

**Unresolved release follow-ups:** packaged-build acceptance; privacy-reviewed media; exact Glama
listing path and score-badge update on awesome-mcp-servers PR #13608.

**Known communication-learning gap:** Managed onboarding can seed voice data from local Git
history, but several Python HTTP learning/status/sync/profile adapters are incomplete. Treat those
CLI paths and Teams/Outlook learning as unavailable until repaired.

**Storage boundary:** Go remains SQLite-only and never connects to PostgreSQL. Python requires
`POSTGRES_URL`, validates it, and applies Alembic before serving; client-event sync is opt-in and
idempotent.

**MCP boundary:** `devtrack mcp` is a local, read-only stdio server backed by the Go client's SQLite
database. The Python HTTP server is not an MCP transport. Raw Sage events never enter MCP,
PostgreSQL, telemetry, or remote synchronization.

**Authority:** `Data/agent_logs/project_board.md` owns task status and IDs; GitHub
`sraj0501/Devtrack_` owns source and releases. Do not revive NATS/Redis/external brokers,
Kubernetes, multi-tenancy, DDD layers, or deleted pre-pivot architecture documents.
