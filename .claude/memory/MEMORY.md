# DevTrack Project Memory

_Last updated: 2026-06-30_ | v3.0.10 | GitHub sole source of truth; devtrack.cloud live on Netlify | ALL Phases 0–8 COMPLETE

DevTrack: offline-first Go daemon + Python backend — monitors git/timers, enriches with AI, routes to PM systems.

## POST-PIVOT — read first
- **`PRODUCT_BIBLE.md`** (repo root) is the definitive source of truth (pivot 2026-06-10).
- Direction: silent background AI layer. Build arc Phase 0→8 ALL COMPLETE. Post-arc queue in `Data/agent_logs/project_board.md`.
- Deprioritised: PG-5, Redis, CLI aesthetics, savings counter, videos, boardroom/plan as primary.

## Rules
- [feedback_rules.md](feedback_rules.md) — Git/PR/commit rules, offline-first, CLI-only, no hardcoded values, GIT_NO_DEVTRACK, ARCHITECTURE.md boundary, uv not pip

## Project State
- [project_current_state.md](project_current_state.md) — dev tip 0fe0aa9; ALL Phases 0–8 COMPLETE; TASK-102 COMPLETE (Azure IsPRApproved); next TASK-103

## Project Context
- [project_platform_modes.md](project_platform_modes.md) — Managed/Lightweight/External modes; Windows WSL2 dev; ARM64 .syso fix shipped
- [project_autoload_env.md](project_autoload_env.md) — AutoLoadEnv() resolution order; test isolation pattern
- [project_local_agents.md](project_local_agents.md) — 6 global agents at ~/.claude/agents/; pm-config.md drives project values; /init bootstraps
- [project_saas_license.md](project_saas_license.md) — License tiers, T&C, auth sessions; cloud server deprioritised (local-first pivot)
- [project_launch_strategy.md](project_launch_strategy.md) — Wedge, positioning rules, channel sequence, decision framework

## References
- [reference_subsystems.md](reference_subsystems.md) — gitsage (Go-native), Telegram bot, RAG personalization, Azure DevOps config
