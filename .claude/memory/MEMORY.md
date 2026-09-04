# DevTrack Project Memory

_Last updated: 2026-09-04_ | public v3.1.1 | official MCP Registry active/latest | PR #13608 awaits a passing Glama listing/badge | GitHub is canonical

DevTrack is an offline-first silent Go daemon with a Python AI/server layer.

## Read first
- `PRODUCT_BIBLE.md` is definitive product direction; `CLAUDE.md` is the build guide.
- `Data/agent_logs/project_board.md` is the task/status/ID authority; dated notes are historical.
- Phases 0–9 and PostgreSQL are complete. v3.1.1 is public with five native MCPB bundles and an active official MCP Registry record.

## Completed — 2026-09-04
- TASK-150 qualified five native MCPB targets, merged PR #259 to `main` at `186036f`, published v3.1.0 with 12 release assets, independently verified all 10 payload hashes, and published `io.github.sraj0501/devtrack` to the official MCP Registry.
- Directory preflight found that v3.1.0 lacked Anthropic's required manifest `privacy_policies` declaration. Patch v3.1.1 at `17bc86f` added that field, a bundled Privacy Policy section, enforcement in packaging/native smoke tests, 12 replacement release assets, independently verified payload hashes, and an active/latest official-registry 3.1.1 record.
- `punkpeye/awesome-mcp-servers` PR #13608 adds DevTrack under Developer Tools and its submission check passed. Other third-party forms and held dev.to/Show HN/LinkedIn posts remain unpublished because their authenticated owner sessions/contact details are unavailable; do not claim those actions are complete.
- PR #13608 subsequently gained a Glama listing/score-badge requirement. `Dockerfile.mcp` is the paste-ready Glama build definition and CI validates its intended initialize/tools-list/shutdown exchange. The owner must submit it through Glama, wait for all checks, and then add the badge using the exact resulting Glama path.

## Completed — 2026-09-03
- TASK-149 synchronized README, wiki, durable project memory, registry evidence, and release gates after TASK-147/148. PR #257 merged to `dev` at `e22f709`.

## Completed — 2026-09-02
- TASK-124 produced held, evidence-backed dev.to, Show HN, and LinkedIn drafts; nothing was published.
- TASK-147 synchronized dependency compatibility, Windows SQLite handling, HTTP contracts, and CI coverage.
- TASK-148 completed MCPB build readiness: current handshake negotiation, six annotated read-only tools, explicit packaged-database selection, reproducible platform bundles, and CI manifest validation.
- TASK-148's MCP distribution work shipped in v3.1.0 and its privacy-compliant bundles shipped in v3.1.1. Commit `17bc86f` is the latest release merge on `main`; post-release documentation is being synchronized through `dev`.

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
