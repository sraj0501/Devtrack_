---
name: Project current state
description: v3.0.10 public; source through TASK-148 and docs through TASK-149; Phases 0–9 complete
type: project
---

**Release:** v3.0.10 is latest public. Commit `c1329f7` is synchronized on local and remote `main`/`dev`; its Phase 9, compatibility, documentation, and MCPB work remains unreleased. A pushed `v*.*.*` tag runs the canonical GitHub Actions release workflow.
**Completed:** Phases 0–9; Managed Install; Silent Worker Correctness; PostgreSQL TASK-112–116 plus TASK-140/141; launch drafts TASK-124; documentation/wiki TASK-145/146; compatibility and contract sync TASK-147; MCPB build readiness TASK-148.
**Storage:** Go remains SQLite-only and never connects to PostgreSQL. Python requires `POSTGRES_URL`, validates it, and applies Alembic before serving; client-event sync is opt-in and idempotent.
**MCP:** Source exposes six local, read-only stdio tools. MCPB 0.3 bundles are reproducibly generated for five OS/architecture targets; CI validates the three platform manifest variants. v3.0.10 contains neither MCP nor MCPB artifacts.
**Documentation:** TASK-149 synchronized README, wiki, checked-in memory, registry evidence, and release gates on `docs/TASK-149-documentation-sync` for review into `dev`.
**Active next:** No implementation task is active; TASK-150 is the next unused ID. Before registry work, tag an MCP-capable release only after CI and Windows/macOS/Linux bundle install tests, privacy-policy review, release notes, and explicit authorization for publication.
**Authority:** `Data/agent_logs/project_board.md` owns task history/IDs; GitHub `sraj0501/Devtrack_` owns source/releases.
**Do not revive:** NATS/Redis/external brokers, Kubernetes, multi-tenancy, DDD layers, or deleted pre-pivot architecture documents.
