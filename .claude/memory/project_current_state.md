---
name: Project current state
description: v3.0.10 public; Phases 0–8 and PostgreSQL complete; Phase 9 launch work active
type: project
---

**Release:** v3.0.10 is latest public. `dev` contains unreleased TASK-117–123, TASK-142, and TASK-144. A pushed `v*.*.*` tag runs the canonical GitHub Actions release workflow.
**Completed:** Phases 0–8; Managed Install; Silent Worker Correctness; PostgreSQL TASK-112–116 plus TASK-140/141; onboarding/parser/demo work through TASK-144.
**Storage:** Go remains SQLite-only and never connects to PostgreSQL. Python requires `POSTGRES_URL`, validates it, and applies Alembic before serving; client-event sync is opt-in and idempotent.
**Active next:** TASK-124 evidence-backed dev.to, Show HN, and LinkedIn drafts. The current uncommitted docs sync uses `docs/TASK-145-full-documentation-sync`; board state is unchanged.
**Authority:** `Data/agent_logs/project_board.md` owns task history/IDs; GitHub `sraj0501/Devtrack_` owns source/releases.
**Do not revive:** NATS/Redis/external brokers, Kubernetes, multi-tenancy, DDD layers, or deleted pre-pivot architecture documents.
