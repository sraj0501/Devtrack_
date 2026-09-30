---
name: Project current state
description: Active implementation planning and unresolved release follow-ups
type: project
---

**Active product initiative:** Complete DevTrack Sage as the Go-native port of the cross-harness, self-writing command-knowledge product in `F:\git_apps\Personal_Projects\ai_sessions_skills` at `b85a1ab`. Current `dev` includes the SAGE-001/SAGE-002 capture foundation, model-free SAGE-003 search slice, removal of the legacy Git Sage CLI/agent surface, neutral ownership for independently used Git/commit-enhancement helpers, and the SAGE-003 neutral LLM transport, structured distiller, and non-blocking worker merged in PR #263. The TASK-157 branch now wires durable SQLite claims and daemon lifecycle. Next implement deterministic Markdown, routing, merge/refile, safe local commits, diagnostics, and parity closure. Playback is not in scope. `initiatives/sage.md` owns the ordered execution plan.

**Latest Sage checkpoint (2026-09-30):** On `features/TASK-157-sage-parity`, all 136 reference
scenarios reconcile with pinned `b85a1ab`. The ledger now has explicit statuses, existing test
references, planned closure tests, and an automated consistency gate. Eight checker fault tests
and focused Go tests/vet pass on Windows and Fedora WSL2. Six scenarios are implemented, fifteen
partial, and 115 pending; runtime parity remains open. The branch now also implements durable queue/daemon integration with full Windows/WSL suite and vet coverage. The deterministic Markdown writer library is now qualified with seven additional reference tests (13 implemented, 15 partial, 108 pending), full Windows suite/vet, and Linux writer race/fuzz/vet. Next: connect durable draft-to-Markdown publication; the daemon still ends at distilled. See `docs/SAGE_MARKDOWN_QUALIFICATION.md`.
This branch is stacked on TASK-159 follow-up `2e50763`, whose push was independently verified;
upstream integration and hosted checks remain unverified because the GitHub API returns HTTP 401.

**Completed validation follow-up:** TASK-158 merged to `dev` in PR #264 (`fe3ad38`). Normal Git is
no longer routed through DevTrack; the explicit AI commit helper returns after Git without ticket,
duration, PM, or push questions; generated observation hooks are silent and fail-open; and database
initialization no longer prints into the user-facing path. On 2026-09-26, Ubuntu 24.04.5 under WSL2
passed explicit redirected non-TTY and pseudo-TTY silence/fail-open tests plus the full Go suite and
`go vet ./...`. TASK-158 is qualified. TASK-160 merged through PR #270 at `a097a0b` after passing
its Windows and Ubuntu WSL2 implementation gates. Seventeen hosted checks passed; both no-send E2E
lanes used a stale `DEMO-201` fixture in an `E2E` workspace. TASK-162 corrected that fixture and
merged through PR #271 at `a4a06fb`; all six hosted checks passed, including both no-send E2E lanes.
TASK-159 implementation subsequently merged through PR #272 at `94dbc7c` (local Git evidence).
It replaces manual per-commit time entry with local, privacy-bounded activity-window inference;
explicit work-session commands remain optional overrides/corrections. The qualification follow-up
is on `fix/TASK-159-schema-transactions`: `3c3fedf` batches schema initialization transactions,
and the Windows E2E fix separates timestamped stderr diagnostics from persisted-status stdout.
On 2026-09-30, full Windows/Fedora WSL2 Go suites and vet plus both no-send E2E lanes passed,
including correction persistence across daemon restart. All five product criteria are locally
qualified. See `docs/TASK_159_QUALIFICATION.md` for evidence and upstream-verification limits.

**Current CI evidence:** The merged PR #263 and PR #264 check sets each retain a historical failed
hosted-Windows unit-test job; PR #264 timed out after 60 seconds in SQLite-backed `internal/db` and
`internal/mcp` tests. PR #265 subsequently passed all 19 checks and merged TASK-161 to `dev` at
`f6b0ec0`, including the full Windows unit-test job, Windows/Ubuntu no-send E2E, five native MCPB
smokes, PostgreSQL, wiki, and shared-memory gates. That re-establishes a green hosted baseline; the
separate TASK-158 Linux TTY/non-TTY qualification subsequently passed under Ubuntu WSL2.

**Qualified deterministic ticket contract:** TASK-160 makes `<kind>/<ticket-key>-<number>-<slug>` the canonical branch
grammar and assigns tickets in this order: canonical branch, explicit commit prefix/trailer,
explicit active-ticket override, otherwise unlinked. Free-form message scanning, last-ticket reuse,
and LLM output cannot silently become authoritative mappings. LLMs may suggest a reviewable mapping
only after deterministic evidence fails. Nonconforming branches never block Git; status/doctor and
the correction channels surface them later. The implementation and local cross-platform checks pass
on `features/TASK-160-deterministic-ticket-contract`; commit `92c0711` and its follow-ups merged to
`dev` in PR #270. TASK-162 fixed the stale no-send E2E fixture through merged PR #271.

**Active UI work:** The server-admin redesign is committed on `feat/TASK-154-server-admin-ui` but is not integrated into current `dev`. Review and integrate or explicitly retire that branch before packaged-build qualification and media capture. `initiatives/server-ui.md` owns the design scope and integration gate.

**Unresolved release follow-ups:** packaged-build acceptance; privacy-reviewed media; exact Glama listing path and the score-badge update on awesome-mcp-servers PR #13608.

**Distribution state:** TASK-161 merged through PR #265 at `f6b0ec0`. Its successful `dev` workflow
run `36136822555` published the rolling `dev-f6b0ec0` prerelease with checksums and all five platform
binaries at `https://github.com/sraj0501/Devtrack_/releases/tag/dev`. Stable `main` remains the
default, and v3.1.1 remains the latest stable public release; v3.1.1 does not contain the new channel
flags. TASK-158 validation is complete and remains separate from this completed distribution work.

**Known communication-learning gap:** Managed onboarding can seed voice data from local Git history,
but the Python HTTP adapter behind status/enable/sync/reset/cron/profile/test/revoke is incomplete.
`LearningIntegration` lacks the method names used by status, enable, sync, reset, cron, test, and
revoke; profile calls an existing method before initialization and is not a useful end-to-end path.
Treat all of those CLI paths as unavailable until repaired. Teams/Outlook learning is not
end-to-end complete.

**Pickup sequence:** `execution-plan.md` owns the cross-initiative order and acceptance gates. In
brief: finish TASK-159 follow-up integration and hosted verification (GitHub CLI currently returns
HTTP 401; local results are in `docs/TASK_159_QUALIFICATION.md`);
make SAGE-003 parity executable and complete one asynchronous, deterministic Codex
capture-to-knowledge journey; then resolve the separate UI branch and release follow-ups. Do not
expand harness support or MCP exposure before the Codex closure gate.

**Storage boundary:** Go remains SQLite-only and never connects to PostgreSQL. Python requires `POSTGRES_URL`, validates it, and applies Alembic before serving; client-event sync is opt-in and idempotent.

**MCP boundary:** `devtrack mcp` is a local, read-only stdio server backed by the Go client's SQLite database. The Python HTTP server is not an MCP transport.

**Authority:** `Data/agent_logs/project_board.md` owns task status and IDs; GitHub `sraj0501/Devtrack_` owns source and releases. Do not use closed task history or superseded validation plans to infer current work.

**Do not revive:** NATS/Redis/external brokers, Kubernetes, multi-tenancy, DDD layers, or deleted pre-pivot architecture documents.
