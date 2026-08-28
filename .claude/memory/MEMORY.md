# DevTrack Project Memory

_Last updated: 2026-08-28_ | public v3.0.10 | unreleased Phase 9 work on `dev` | GitHub is canonical

DevTrack is an offline-first silent Go daemon with a Python AI/server layer.

## Read first
- `PRODUCT_BIBLE.md` is definitive product direction; `CLAUDE.md` is the build guide.
- `Data/agent_logs/project_board.md` is the task/status/ID authority; dated notes are historical.
- Phases 0–8 and PostgreSQL are complete. TASK-124 is the remaining Phase 9 work.

## Rules
- [feedback_rules.md](feedback_rules.md) — authorization, Git, privacy, architecture, and dependency rules

## Project State
- [project_current_state.md](project_current_state.md) — release boundary, completed arcs, and active Phase 9 work
- [project_postgres_backend.md](project_postgres_backend.md) — required server storage and opt-in client sync

## Project Context
- [project_platform_modes.md](project_platform_modes.md) — server-mode guards and current Zorin/Linux host assumptions
- [project_autoload_env.md](project_autoload_env.md) — AutoLoadEnv() resolution order; DATABASE_DIR test isolation
- [project_documentation.md](project_documentation.md) — docs/site sources, release boundary, and stale-claim rules
- [project_local_agents.md](project_local_agents.md) — role sources, Codex adapters, memory, and authorization
- [project_saas_license.md](project_saas_license.md) — local license/auth; hosted SaaS remains unbuilt
- [project_launch_strategy.md](project_launch_strategy.md) — wedge, positioning rules, channels — the input to Phase 9

## References
- [reference_subsystems.md](reference_subsystems.md) — Telegram, RAG, and Azure DevOps config not covered by CLAUDE.md
