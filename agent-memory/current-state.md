---
name: Project current state
description: Active implementation planning and unresolved release follow-ups
type: project
---

**Active product initiative:** Complete DevTrack Sage as the Go-native port of the cross-harness, self-writing command-knowledge product in `D:\git_apps\ai_sessions_skills` at `b85a1ab`. The work on `feat/SAGE-003-knowledge` is not contained in the current `dev` branch and must be integrated first. Then remove the legacy Git Sage CLI/agent surface and finish SAGE-003 structured distillation, deterministic Markdown, routing, merge/refile, safe local commits, retries, diagnostics, and parity coverage. Playback is not in scope. `initiatives/sage.md` owns the ordered execution plan.

**Active UI work:** Uncommitted server-admin visual changes remain work in progress. Stabilize or isolate them before packaged-build qualification and media capture. `initiatives/server-ui.md` owns the design scope.

**Unresolved release follow-ups:** packaged-build acceptance; privacy-reviewed media; exact Glama listing path and the score-badge update on awesome-mcp-servers PR #13608.

**Storage boundary:** Go remains SQLite-only and never connects to PostgreSQL. Python requires `POSTGRES_URL`, validates it, and applies Alembic before serving; client-event sync is opt-in and idempotent.

**MCP boundary:** `devtrack mcp` is a local, read-only stdio server backed by the Go client's SQLite database. The Python HTTP server is not an MCP transport.

**Authority:** `Data/agent_logs/project_board.md` owns task status and IDs; GitHub `sraj0501/Devtrack_` owns source and releases. Do not use closed task history or superseded validation plans to infer current work.

**Do not revive:** NATS/Redis/external brokers, Kubernetes, multi-tenancy, DDD layers, or deleted pre-pivot architecture documents.
