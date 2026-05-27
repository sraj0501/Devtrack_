# DevTrack Project Memory

_Last updated: 2026-05-27_ | v2.2.22 | GitHub sole source of truth; devtrack.cloud live on Netlify

DevTrack: offline-first Go daemon + Python backend — monitors git/timers, enriches with AI, routes to PM systems.

## Rules
- [feedback_rules.md](feedback_rules.md) — Git/PR/commit rules, offline-first, CLI-only, no hardcoded values, GIT_NO_DEVTRACK, no credentials

## Project State
- [project_current_state.md](project_current_state.md) — v2.2.22; GitLab retired; releases via scripts/release.ps1; devtrack.cloud on Netlify

## Project Context
- [project_platform_modes.md](project_platform_modes.md) — Managed/Lightweight/External modes; Windows WSL2 dev; ARM64 .syso fix shipped
- [project_deployment_ops.md](project_deployment_ops.md) — scripts/release.ps1; devtrack-server bash CLI; versioned migrations
- [project_autoload_env.md](project_autoload_env.md) — AutoLoadEnv() resolution order; test isolation pattern
- [project_local_agents.md](project_local_agents.md) — 6 global agents at ~/.claude/agents/; pm-config.md drives project values; /init bootstraps
- [project_saas_license.md](project_saas_license.md) — License tiers, T&C, auth sessions; cloud server not yet built

## References
- [reference_subsystems.md](reference_subsystems.md) — git-sage UX/config/Groq, RAG personalization, Azure DevOps env vars
- [project_launch_strategy.md](project_launch_strategy.md) — Wedge, positioning rules, channel sequence, decision framework
