---
name: Project current state
description: Active implementation work, integration state, and unresolved release follow-ups
type: project
---

**Active product initiative:** TASK-157 / SAGE-003 is the Go-native port of the cross-harness,
self-writing command-knowledge product in `C:\Work\git_apps\ai_sessions_skills` at
`ea2e067`. Changes after `b85a1ab` affect generated reference knowledge only; the 136-scenario
behavioral corpus is unchanged. PR #278 merged the latest qualified closure slice to `dev` at
`a6a75b0`; its required checks and the post-merge CI, client CI, no-send end-to-end, and
development-release workflows passed. PR #279 is the fully green, unmerged follow-up review from
`features/TASK-157-sage-backend-isolation`, based on that merge. It separates the stable Sage
runtime root from portable Markdown knowledge overrides and anchors relative overrides to the Sage
configuration root rather than the caller's working directory. PR #280 adds explicit shipped
command-model providers while retaining Ollama HTTP as the default; all 19 hosted checks passed.
The active stacked branch is `features/TASK-157-sage-platform-installers`; it closes portable
harness-layout and additive install/removal parity without advertising another capture adapter.
PR #281 is the active review for this slice; all 19 hosted checks passed.
The 136-scenario ledger now records 105 implemented, 4 adapted, 2 partial, and 25 pending scenarios.
Full Go tests, `go vet ./...`,
parity-ledger consistency, parity-checker tests, shared-memory validation, and whitespace checks
pass locally. Next: review and integrate the stacked slices, then close daemon/process and CLI
lifecycle parity.
Playback is not in scope. `initiatives/sage.md` owns the detailed boundaries and execution order;
`sage-resume.md` is the fresh-session handoff with the exact pending groups and acceptance gates.

**Integration state:** PR #278 merged to `dev` at `a6a75b0` on 2026-10-13. All required PR checks
passed, including the hosted Windows suite, and all applicable post-merge workflows passed. The
superseded PR #274 was closed without merge and is no longer a workflow risk.

**Distribution state:** Stable v3.1.1 remains the latest public stable release. The rolling `dev`
release is `dev-a6a75b0`, built by the successful post-merge development-release workflow; current
`origin/dev` is `a6a75b0`. `main` and `dev` have diverged in Git
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
