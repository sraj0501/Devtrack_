# DevTrack Project Memory

_Last updated: 2026-08-28_ | v3.0.10 latest release | `dev` includes TASK-121–123 and TASK-144, ahead of `main` | GitHub `sraj0501/Devtrack_` sole source of truth; devtrack.cloud on Netlify

DevTrack: offline-first Go daemon + Python backend — watches git/timers, enriches with AI, routes to PM systems, never prompts.

## Read first
- **`PRODUCT_BIBLE.md`** (repo root) is the definitive source of truth for product direction (pivot 2026-06-10).
- Build arc **Phases 0–8 ALL COMPLETE**; Managed Install, cleanup/wiki work, and Silent Worker correctness all shipped. **The PostgreSQL Backend epic is complete** through TASK-116/PR #251: server PostgreSQL is mandatory for Python persistence while the Go client remains SQLite-only/offline-first. **Phase 9 Adoption Gate is active** (`docs/NEXT_STEPS.md`, TASK-117–124); TASK-117–123 and prerequisite TASK-142 are complete on `dev`.
- Task history and the authoritative task-ID ledger: `Data/agent_logs/project_board.md`. TASK-124 is the remaining Phase 9 work; TASK-143 reconciled project records; TASK-144 fixed live-demo runtime defects. Next unused ID: **TASK-145**.

## Rules
- [feedback_rules.md](feedback_rules.md) — git/PR flow, GIT_NO_DEVTRACK, telemetry is opt-in, offline-first, no hardcoded values, uv not pip

## Project State
- [project_current_state.md](project_current_state.md) — v3.0.10 latest release; PostgreSQL epic complete; Phase 9 active; TASK-124 remains
- [project_postgres_backend.md](project_postgres_backend.md) — completed storage boundary, sync, Alembic, deployment, and operational invariants

## Project Context
- [project_platform_modes.md](project_platform_modes.md) — GetServerMode() guard list; Windows signal gap / WSL2
- [project_autoload_env.md](project_autoload_env.md) — AutoLoadEnv() resolution order; DATABASE_DIR test isolation
- [project_local_agents.md](project_local_agents.md) — 5 agents live in .claude/agents/_archive/; pm-config.md drives project values
- [project_saas_license.md](project_saas_license.md) — license tiers, T&C, auth; telemetry opt-in; cloud server never built
- [project_launch_strategy.md](project_launch_strategy.md) — wedge, positioning rules, channels — the input to Phase 9

## References
- [reference_subsystems.md](reference_subsystems.md) — Telegram, RAG, and Azure DevOps config not covered by CLAUDE.md
