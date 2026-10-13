---
name: Project current state
description: Active implementation work, integration state, and unresolved release follow-ups
type: project
---

**Active product initiative:** TASK-157 / SAGE-003 is the Go-native port of the cross-harness,
self-writing command-knowledge product in `F:\git_apps\Personal_Projects\ai_sessions_skills` at
`b85a1ab`. The qualified queue, publication, routing, merge/refile, skipped-action, and full-entry
search slice is integrated into `dev` through PR #275. The active closure branch is
`features/TASK-157-sage-closure`, based on `origin/dev` at `549c430`; its first review slice adds
truthful diagnostics, bounded retries, privacy-safe NDJSON logs, safe local knowledge commits,
model-free documented-action reuse, deterministic rendering, and conservative section matching.
The 136-scenario ledger now records 82 implemented, 1 adapted, 2 partial, and 51 pending scenarios.
PR #278 is the active review for this closure branch; follow-up commits add quote-aware compound
command signatures, cross-harness payload normalization, deterministic temperature-zero model
requests, token-limit diagnostics, and background-only classify-once routing with deterministic
fallback and existing-section context.
The full Go suite, `go vet ./...`, parity-ledger consistency check, and whitespace check pass. Next:
close the remaining parity ledger and repeated clean capture-to-knowledge acceptance journey.
Playback is not in scope. `initiatives/sage.md` owns the detailed boundaries and execution order.

**Integration state:** PR #275 merged the qualified TASK-157 implementation slice to `dev` at
`a1777b6`; its CI, client CI, Windows/Linux no-send E2E, PostgreSQL, MCPB, registry, and rolling
release workflows passed. PR #276 then merged the local Docker evaluation documentation at
`fc3217f`; its applicable CI and wiki checks passed. `origin/dev` is therefore `fc3217f`. The
superseded PR #274 was closed without merge and is no longer a workflow risk.

**Distribution state:** Stable v3.1.1 remains the latest public stable release. The rolling `dev`
release is `dev-a1777b6`, built from the TASK-157 implementation merge; current `origin/dev`
contains the later documentation-only merge at `fc3217f`. `main` and `dev` have diverged in Git
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
