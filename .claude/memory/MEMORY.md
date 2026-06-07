# DevTrack Project Memory

_Last updated: 2026-06-04_ | v3.0.0 | GitHub sole source of truth; devtrack.cloud live on Netlify

DevTrack: offline-first Go daemon + Python backend — monitors git/timers, enriches with AI, routes to PM systems.

## Rules
- [feedback_rules.md](feedback_rules.md) — Git/PR/commit rules, offline-first, CLI-only, no hardcoded values, GIT_NO_DEVTRACK, HTTP_API.md boundary, uv not pip

## Project State
- [project_current_state.md](project_current_state.md) — v3.0.0; decoupling Phases 1+2 + runtime-narrative DONE; PG-5 + Redis next; internal package layer order

## Project Context
- [project_platform_modes.md](project_platform_modes.md) — Managed/Lightweight/External modes; Windows WSL2 dev; ARM64 .syso fix shipped
- [project_autoload_env.md](project_autoload_env.md) — AutoLoadEnv() resolution order; test isolation pattern
- [project_local_agents.md](project_local_agents.md) — 6 global agents at ~/.claude/agents/; pm-config.md drives project values; /init bootstraps
- [project_saas_license.md](project_saas_license.md) — License tiers, T&C, auth sessions; cloud server not yet built
- [project_launch_strategy.md](project_launch_strategy.md) — Wedge, positioning rules, channel sequence, decision framework

## References
- [reference_subsystems.md](reference_subsystems.md) — gitsage (Go-native), RAG personalization, Azure DevOps connector config
