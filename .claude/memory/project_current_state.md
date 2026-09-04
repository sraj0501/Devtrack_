---
name: Project current state
description: v3.1.0 public with native MCPB assets and an active official MCP Registry record
type: project
---

**Release:** v3.1.0 is latest public at merge `186036f`. Its GitHub release contains the five legacy platform binary assets, five native MCPBs, `checksums.txt`, and `server.json`. A pushed `v*.*.*` tag runs the canonical GitHub Actions release workflow.
**Completed:** Phases 0–9; Managed Install; Silent Worker Correctness; PostgreSQL TASK-112–116 plus TASK-140/141; launch drafts TASK-124; documentation/wiki TASK-145/146; compatibility and contract sync TASK-147; MCPB distribution TASK-148; v3.1.0 qualification and official-registry publication TASK-150.
**Storage:** Go remains SQLite-only and never connects to PostgreSQL. Python requires `POSTGRES_URL`, validates it, and applies Alembic before serving; client-event sync is opt-in and idempotent.
**MCP:** Six local, read-only stdio tools ship in v3.1.0. MCPB 0.3 bundles were executed on native Windows amd64, macOS amd64/arm64, and Linux amd64/arm64 CI runners; clean-project setup, handshake, tool listing/call, and shutdown passed. `io.github.sraj0501/devtrack` version 3.1.0 is active in the official MCP Registry.
**Documentation:** TASK-150 post-release synchronization records the public release, verified checksums, registry status, and remaining external-account actions.
**Active next:** Obtain owner-authenticated sessions to submit eligible third-party directories and publish the held dev.to, Show HN, and LinkedIn posts. Then collect real install feedback and prioritize Phase 10 from evidence rather than assumptions.
**Authority:** `Data/agent_logs/project_board.md` owns task history/IDs; GitHub `sraj0501/Devtrack_` owns source/releases.
**Do not revive:** NATS/Redis/external brokers, Kubernetes, multi-tenancy, DDD layers, or deleted pre-pivot architecture documents.
