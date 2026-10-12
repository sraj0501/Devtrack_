---
name: Project current state
description: Active implementation work, integration state, and unresolved release follow-ups
type: project
---

**Active product initiative:** TASK-157 / SAGE-003 is the Go-native port of the cross-harness,
self-writing command-knowledge product in `F:\git_apps\Personal_Projects\ai_sessions_skills` at
`b85a1ab`. The active branch is `features/TASK-157-sage-parity` at `c126a40`. It implements durable
SQLite claims, daemon lifecycle ownership, deterministic Markdown publication, persistent routing,
restart-safe merge/refile, skipped-action records, and full-entry search. The 136-scenario ledger
currently records 27 implemented, 1 adapted, 12 partial, and 96 pending scenarios. Next: truthful
diagnostics, safe local knowledge commits, remaining parity closure, and the repeated clean
capture-to-knowledge acceptance journey. Playback is not in scope. `initiatives/sage.md` owns the
detailed boundaries and execution order.

**Integration state:** The TASK-157 branch is twelve commits ahead of its merge base with `dev` and
one commit behind current `dev`. PR #273 merged the missing TASK-159 schema/restart follow-up to
`dev` at `a329b6f` on 2026-10-07. Its general CI, client CI, Windows/Linux no-send E2E, PostgreSQL,
MCPB, registry, and rolling-release checks passed. Reconcile that `dev` tip into TASK-157 before its
integration PR; do not describe TASK-159 verification as pending.

**Distribution state:** Stable v3.1.1 remains the latest public stable release. The rolling `dev`
release is `dev-a329b6f`, matching current `origin/dev`. `main` and `dev` have diverged in Git
ancestry and require deliberate reconciliation before a future promotion; do not assume a simple
fast-forward.

**Active UI work:** The full server-admin redesign remains isolated on
`feat/TASK-154-server-admin-ui` at `54ceb05` and is not integrated into current `dev`. Review and
reconcile it against current templates/routes or explicitly retire it before packaged-build
qualification and media capture. `initiatives/server-ui.md` owns the integration gate.

**Current workflow risk:** Open PR #274 (`cursor/cloud-agent-env-b914`) targets `main` directly.
Project policy requires feature branches to target `dev`; it must be retargeted or replaced before
merge consideration.

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
