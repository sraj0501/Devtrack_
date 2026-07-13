# DevTrack Project Memory

_Last updated: 2026-07-13_ | v3.0.10 | branch `dev` | GitHub `sraj0501/Devtrack_` sole source of truth; devtrack.cloud on Netlify

DevTrack: offline-first Go daemon + Python backend — watches git/timers, enriches with AI, routes to PM systems, never prompts.

## Read first
- **`PRODUCT_BIBLE.md`** (repo root) is the definitive source of truth for product direction (pivot 2026-06-10).
- Build arc **Phases 0–8 ALL COMPLETE**; Managed Install, TASK-109 cleanup, TASK-110/111 wiki+docs done. **Queued: EPIC PostgreSQL Backend (TASK-112–116)**, then **Phase 9 Adoption Gate** (`docs/NEXT_STEPS.md`, TASK-117–124).
- Task history and the authoritative task-ID ledger: `Data/agent_logs/project_board.md`. Next ID: **TASK-125**.

## Rules
- [feedback_rules.md](feedback_rules.md) — git/PR flow, GIT_NO_DEVTRACK, telemetry is opt-in, offline-first, no hardcoded values, uv not pip

## Project State
- [project_current_state.md](project_current_state.md) — v3.0.10; Phases 0–8 + TASK-109/110/111 done; Postgres epic then Phase 9 queued; no open PRs

## Project Context
- [project_platform_modes.md](project_platform_modes.md) — GetServerMode() guard list; Windows signal gap / WSL2
- [project_autoload_env.md](project_autoload_env.md) — AutoLoadEnv() resolution order; DATABASE_DIR test isolation
- [project_local_agents.md](project_local_agents.md) — 5 agents live in .claude/agents/_archive/; pm-config.md drives project values
- [project_saas_license.md](project_saas_license.md) — license tiers, T&C, auth; telemetry opt-in; cloud server never built
- [project_launch_strategy.md](project_launch_strategy.md) — wedge, positioning rules, channels — the input to Phase 9

## References
- [reference_subsystems.md](reference_subsystems.md) — Telegram, RAG, and Azure DevOps config not covered by CLAUDE.md
