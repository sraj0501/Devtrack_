# DevTrack Project Memory

_Last updated: 2026-06-10_ | v3.0.9 | GitHub sole source of truth; devtrack.cloud live on Netlify

DevTrack: offline-first Go daemon + Python backend — monitors git/timers, enriches with AI, routes to PM systems.

## POST-PIVOT — read first
- **`PRODUCT_BIBLE.md`** (repo root) is the definitive source of truth (pivot 2026-06-10).
- Direction: silent background AI layer that absorbs developer meta-work. Promise: "You write code. DevTrack handles the rest — silently, accurately, in your voice."
- Build arc = Phase 0 (silent daemon) → 1 (pending actions queue) → 2-8. See `Data/agent_logs/project_board.md`.
- Deprioritised: PG-5, Redis, CLI aesthetics, savings counter, videos, boardroom/plan as primary.

## Rules
- [feedback_rules.md](feedback_rules.md) — Git/PR/commit rules, offline-first, CLI-only, no hardcoded values, GIT_NO_DEVTRACK, ARCHITECTURE.md boundary, uv not pip

## Project State
- [project_current_state.md](project_current_state.md) — post-pivot direction (Bible Phase 0→8); v3.0.9 shipped; PG-5/Redis deprioritised; Azure WIQL date quirk; notify interface pitfall

## Project Context
- [project_platform_modes.md](project_platform_modes.md) — Managed/Lightweight/External modes; Windows WSL2 dev; ARM64 .syso fix shipped
- [project_autoload_env.md](project_autoload_env.md) — AutoLoadEnv() resolution order; test isolation pattern
- [project_local_agents.md](project_local_agents.md) — Agents in .claude/agents/_archive/; pm-config.md drives project values; /init bootstraps
- [project_saas_license.md](project_saas_license.md) — License tiers, T&C, auth sessions; cloud server not yet built
- [project_launch_strategy.md](project_launch_strategy.md) — Wedge, positioning rules, channel sequence, decision framework

## References
- [reference_subsystems.md](reference_subsystems.md) — gitsage (Go-native), Telegram bot, RAG personalization, Azure DevOps config
