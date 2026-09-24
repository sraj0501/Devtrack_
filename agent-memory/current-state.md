---
name: Project current state
description: Active implementation planning and unresolved release follow-ups
type: project
---

**Active product initiative:** Complete DevTrack Sage as the Go-native port of the cross-harness, self-writing command-knowledge product in `D:\git_apps\ai_sessions_skills` at `b85a1ab`. Current `dev` includes the SAGE-001/SAGE-002 capture foundation, model-free SAGE-003 search slice, removal of the legacy Git Sage CLI/agent surface, neutral ownership for independently used Git/commit-enhancement helpers, and the SAGE-003 neutral LLM transport, structured distiller, and non-blocking worker merged in PR #263. Next connect that worker to durable SQLite claims and daemon lifecycle, then implement deterministic Markdown, routing, merge/refile, safe local commits, persisted retries, diagnostics, and parity closure. Playback is not in scope. `initiatives/sage.md` owns the ordered execution plan.

**Immediate validation follow-up:** TASK-158 merged to `dev` in PR #264 (`fe3ad38`). Normal Git is
no longer routed through DevTrack; the explicit AI commit helper returns after Git without ticket,
duration, PM, or push questions; generated observation hooks are silent and fail-open; and database
initialization no longer prints into the user-facing path. Native Linux with explicit TTY and
non-TTY execution is still unverified. Close this validation follow-up before treating TASK-158 as
fully qualified, then begin TASK-160.
TASK-159 then replaces manual per-commit time entry with local, privacy-bounded activity-window
inference; explicit work-session commands remain optional overrides/corrections.

**Current CI gap:** The merged PR #263 and PR #264 check sets each contain a failed hosted-Windows
unit-test job even though Windows build/vet, native MCPB smoke, and no-send E2E passed. PR #264's
failed job timed out after 60 seconds in SQLite-backed `internal/db` and `internal/mcp` tests; its
Sage distillation tests passed. Treat `dev` as lacking an all-green Windows unit-test baseline until
the timeout is reproduced or rerun successfully.

**Deterministic ticket contract:** TASK-160 makes `<kind>/<ticket-key>-<number>-<slug>` the canonical branch
grammar and assigns tickets in this order: canonical branch, explicit commit prefix/trailer,
explicit active-ticket override, otherwise unlinked. Free-form message scanning, last-ticket reuse,
and LLM output cannot silently become authoritative mappings. LLMs may suggest a reviewable mapping
only after deterministic evidence fails. Nonconforming branches never block Git; status/doctor and
the correction channels surface them later.

**Active UI work:** The server-admin redesign is committed on `feat/TASK-154-server-admin-ui` but is not integrated into current `dev`. Review and integrate or explicitly retire that branch before packaged-build qualification and media capture. `initiatives/server-ui.md` owns the design scope and integration gate.

**Unresolved release follow-ups:** packaged-build acceptance; privacy-reviewed media; exact Glama listing path and the score-badge update on awesome-mcp-servers PR #13608.

**Active distribution work:** TASK-161 owns the rolling `dev` prerelease and persisted update-channel
implementation on `features/TASK-161-rolling-dev-channel`. Stable `main` remains the default and the
feature remains unreleased until its PR is reviewed and its workflow publishes the first `dev`
prerelease. TASK-158 validation work is excluded from this branch. At allocation time GitHub had no
open pull requests; v3.1.1 remained the latest public release.

**Known communication-learning gap:** Managed onboarding can seed voice data from local Git history,
but the Python HTTP adapter behind status/enable/sync/reset/cron/profile/test/revoke is incomplete.
`LearningIntegration` lacks the method names used by status, enable, sync, reset, cron, test, and
revoke; profile calls an existing method before initialization and is not a useful end-to-end path.
Treat all of those CLI paths as unavailable until repaired. Teams/Outlook learning is not
end-to-end complete.

**Pickup sequence:** `execution-plan.md` owns the cross-initiative order and acceptance gates. In
brief: close TASK-158's native Linux TTY/non-TTY qualification; complete TASK-160 and TASK-159;
make SAGE-003 parity executable and complete one asynchronous, deterministic Codex
capture-to-knowledge journey; then resolve the separate UI branch and release follow-ups. Do not
expand harness support or MCP exposure before the Codex closure gate.

**Storage boundary:** Go remains SQLite-only and never connects to PostgreSQL. Python requires `POSTGRES_URL`, validates it, and applies Alembic before serving; client-event sync is opt-in and idempotent.

**MCP boundary:** `devtrack mcp` is a local, read-only stdio server backed by the Go client's SQLite database. The Python HTTP server is not an MCP transport.

**Authority:** `Data/agent_logs/project_board.md` owns task status and IDs; GitHub `sraj0501/Devtrack_` owns source and releases. Do not use closed task history or superseded validation plans to infer current work.

**Do not revive:** NATS/Redis/external brokers, Kubernetes, multi-tenancy, DDD layers, or deleted pre-pivot architecture documents.
