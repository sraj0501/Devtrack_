# DevTrack Project Memory

_Last updated: 2026-09-03_ | public v3.0.10 | source synchronized through TASK-148; TASK-149 docs awaiting review | GitHub is canonical

DevTrack is an offline-first silent Go daemon with a Python AI/server layer.

## Read first
- `PRODUCT_BIBLE.md` is definitive product direction; `CLAUDE.md` is the build guide.
- `Data/agent_logs/project_board.md` is the task/status/ID authority; dated notes are historical.
- Phases 0–9 and PostgreSQL are complete. TASK-150 is the next unused task ID; no implementation task is active.

## Completed — 2026-09-03
- TASK-149 synchronized README, wiki, durable project memory, registry evidence, and release gates after TASK-147/148. Commit `f2c7fb5` is on `docs/TASK-149-documentation-sync` (PR #257 targeting `dev`); it is not yet part of `dev` or `main` in the checked-out repository state.

## Completed — 2026-09-02
- TASK-124 produced held, evidence-backed dev.to, Show HN, and LinkedIn drafts; nothing was published.
- TASK-147 synchronized dependency compatibility, Windows SQLite handling, HTTP contracts, and CI coverage.
- TASK-148 completed MCPB build readiness: current handshake negotiation, six annotated read-only tools, explicit packaged-database selection, reproducible platform bundles, and CI manifest validation.
- Commit `c1329f7` is synchronized on local `main` and remote `origin/main`/`origin/dev`; no local `dev` branch is currently checked out or recorded. All work after v3.0.10 remains unreleased.

## Rules
- [feedback_rules.md](feedback_rules.md) — authorization, Git, privacy, architecture, and dependency rules

## Project State
- [project_current_state.md](project_current_state.md) — release boundary, completed arcs, and next release gates
- [project_postgres_backend.md](project_postgres_backend.md) — required server storage and opt-in client sync
- [project_mcp_distribution.md](project_mcp_distribution.md) — local MCP boundary, MCPB packaging, and remaining release gates

## Project Context
- [project_platform_modes.md](project_platform_modes.md) — actual mode resolution and cross-platform host guidance
- [project_autoload_env.md](project_autoload_env.md) — AutoLoadEnv() resolution order; DATABASE_DIR test isolation
- [project_documentation.md](project_documentation.md) — docs/site sources, release boundary, and stale-claim rules
- [project_local_agents.md](project_local_agents.md) — checked-in role sources, external-agent boundary, memory, and authorization
- [project_saas_license.md](project_saas_license.md) — local license/auth; hosted SaaS remains unbuilt
- [project_launch_strategy.md](project_launch_strategy.md) — wedge, positioning rules, channels — the input to Phase 9

## References
- [reference_subsystems.md](reference_subsystems.md) — Telegram, RAG, and Azure DevOps config not covered by CLAUDE.md
