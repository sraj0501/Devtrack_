# DevTrack Project Memory

_Last updated: 2026-08-10_ | v3.0.10 | `origin/dev` ahead of `origin/main` | GitHub `sraj0501/Devtrack_` sole source of truth; devtrack.cloud on Netlify

DevTrack: offline-first Go daemon + Python backend — watches git/timers, enriches with AI, routes to PM systems, never prompts.

## Read first
- **`PRODUCT_BIBLE.md`** (repo root) is the definitive source of truth for product direction (pivot 2026-06-10).
- Build arc **Phases 0–8 ALL COMPLETE**; Managed Install, TASK-109 cleanup, TASK-110/111 wiki+docs, Silent Worker epic (TASK-126–131), TASK-132 doc sync all done. **In progress: EPIC PostgreSQL Backend (TASK-112–116)** — PostgreSQL is mandatory for server persistence/events; Go remains SQLite-only and offline-first. TASK-141 completes the original module inventory: 14 of 15 raw-`sqlite3` modules are ported and one dead module was removed, leaving no production raw-`sqlite3` imports. TASK-114–116 remain. Queued after the epic: **Phase 9 Adoption Gate** (`docs/NEXT_STEPS.md`, TASK-117–124).
- Task history and the authoritative task-ID ledger: `Data/agent_logs/project_board.md`. Next unused ID: **TASK-142**.

## Rules
- [feedback_rules.md](feedback_rules.md) — git/PR flow, GIT_NO_DEVTRACK, telemetry is opt-in, offline-first, no hardcoded values, uv not pip

## Project State
- [project_current_state.md](project_current_state.md) — v3.0.10; mandatory server PostgreSQL migration in progress; Go remains local SQLite; Phase 9 queued

## Project Context
- [project_platform_modes.md](project_platform_modes.md) — GetServerMode() guard list; Windows signal gap / WSL2
- [project_autoload_env.md](project_autoload_env.md) — AutoLoadEnv() resolution order; DATABASE_DIR test isolation
- [project_local_agents.md](project_local_agents.md) — 5 agents live in .claude/agents/_archive/; pm-config.md drives project values
- [project_saas_license.md](project_saas_license.md) — license tiers, T&C, auth; telemetry opt-in; cloud server never built
- [project_launch_strategy.md](project_launch_strategy.md) — wedge, positioning rules, channels — the input to Phase 9

## References
- [reference_subsystems.md](reference_subsystems.md) — Telegram, RAG, and Azure DevOps config not covered by CLAUDE.md
