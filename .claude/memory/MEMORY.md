# DevTrack Project Memory

_Last updated: 2026-05-24_ | v2.2.14 | Branch: features/SPLIT-001-monorepo-restructure

DevTrack: offline-first Go daemon + Python backend — monitors git/timers, enriches with AI, routes to PM systems.

## Rules
- [feedback_rules.md](feedback_rules.md) — Git/PR/commit rules, offline-first, CLI-only, no hardcoded values, GIT_NO_DEVTRACK, no credentials

## Project State
- [project_current_state.md](project_current_state.md) — v2.2.14; EPIC-SPLIT done; TASK-049 COMPLETE; next: TASK-050, build-runner, delete devtrack_contract

## Project Context
- [project_platform_modes.md](project_platform_modes.md) — Managed/Lightweight/External modes; Windows WSL2 dev; ARM64 .syso fix shipped
- [project_deployment_ops.md](project_deployment_ops.md) — `devtrack upgrade` (GitLab Releases API); `devtrack-server` bash CLI; versioned migrations
- [project_autoload_env.md](project_autoload_env.md) — `AutoLoadEnv()` resolution order; test isolation pattern
- [project_local_agents.md](project_local_agents.md) — 5 agents: roles, board format, engineer log format, git-agent/memory-compactor rules
- [project_saas_license.md](project_saas_license.md) — License tiers, T&C, auth sessions; cloud server not yet built

## References
- [reference_subsystems.md](reference_subsystems.md) — git-sage UX/config/Groq, RAG personalization, Azure DevOps env vars
- [project_launch_strategy.md](project_launch_strategy.md) — Wedge, positioning rules, channel sequence, decision framework
