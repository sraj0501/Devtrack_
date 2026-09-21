---
name: Project current state
description: Active implementation planning and unresolved release follow-ups
type: project
---

**Active product initiative:** Complete DevTrack Sage as the Go-native port of the cross-harness, self-writing command-knowledge product in `D:\git_apps\ai_sessions_skills` at `b85a1ab`. Current `dev` includes the SAGE-001/SAGE-002 capture foundation, model-free SAGE-003 search slice, removal of the legacy Git Sage CLI/agent surface, and neutral ownership for the independently used Git/commit-enhancement helpers. The active SAGE-003 branch now contains a neutral context-aware LLM client, validated structured distillation, and a non-blocking background worker with configurable bounded timing for slower offline models. Next connect that worker to durable SQLite claims and daemon lifecycle, then implement deterministic Markdown, routing, merge/refile, safe local commits, persisted retries, diagnostics, and parity closure. Playback is not in scope. `initiatives/sage.md` owns the ordered execution plan.

**Active UI work:** The server-admin redesign is committed on `feat/TASK-154-server-admin-ui` but is not integrated into current `dev`. Review and integrate or explicitly retire that branch before packaged-build qualification and media capture. `initiatives/server-ui.md` owns the design scope and integration gate.

**Unresolved release follow-ups:** packaged-build acceptance; privacy-reviewed media; exact Glama listing path and the score-badge update on awesome-mcp-servers PR #13608.

**Known communication-learning gap:** Managed onboarding can seed voice data from local Git history,
but the Python HTTP adapter behind enable/sync/reset/cron/profile/test/revoke calls methods absent
from `LearningIntegration`. Treat those CLI paths as unavailable until repaired; `learning-status`
remains inspection-only evidence, and Teams/Outlook learning is not end-to-end complete.

**Pickup sequence:** `execution-plan.md` owns the cross-initiative order and acceptance gates. In
brief: land the pending documentation baseline; make SAGE-003 parity executable; complete one
asynchronous, deterministic Codex capture-to-knowledge journey; then resolve the separate UI branch
and release follow-ups. Do not expand harness support or MCP exposure before the Codex closure gate.

**Storage boundary:** Go remains SQLite-only and never connects to PostgreSQL. Python requires `POSTGRES_URL`, validates it, and applies Alembic before serving; client-event sync is opt-in and idempotent.

**MCP boundary:** `devtrack mcp` is a local, read-only stdio server backed by the Go client's SQLite database. The Python HTTP server is not an MCP transport.

**Authority:** `Data/agent_logs/project_board.md` owns task status and IDs; GitHub `sraj0501/Devtrack_` owns source and releases. Do not use closed task history or superseded validation plans to infer current work.

**Do not revive:** NATS/Redis/external brokers, Kubernetes, multi-tenancy, DDD layers, or deleted pre-pivot architecture documents.
