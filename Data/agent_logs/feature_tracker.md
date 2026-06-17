# DevTrack Feature Tracker

_Last updated: 2026-06-17 by PM (TASK-072 complete — commit 87e4915, PR #179 open against dev; TASK-073 next)_

---

## Roadmap Status

> **Pivot 2026-06-10**: the roadmap is now driven by `PRODUCT_BIBLE.md` § Build Phases.
> The build arc is sequenced safe → accurate → automated → autonomous. The old
> "Client-Server Arc" (CS-1–5) and "Product Phases 1–10" schemes are retired —
> their shipped work is captured in Task History below and on the project board.

### Product Bible Build Phases

| Phase | Name | Status | Exit criterion (short) |
|---|---|---|---|
| 0 | Foundation reset (silent daemon) | DONE | Daemon runs a full day with no prompts shown |
| 1 | Pending actions queue | DONE | A week of outbound actions all staged; nothing unexpected posts |
| 2 | Opinionated ticket extractor | DONE | >80% commits mapped to tickets, no config beyond branch naming — verified 100% (10/10) live |
| 3 | Silent commit handler | ACTIVE — TASK-072 done, TASK-073 next | Commit → ticket commented + state-transitioned; dev did nothing |
| 4 | EOD pipeline | QUEUED | Accurate EOD email every evening, in the dev's voice |
| 5 | Voice training (low friction) | QUEUED | Generated text passes "did I write this?" after 1 week |
| 6 | Dialectic self-improvement | QUEUED | 30-day correction rate down; ≥3 autonomous skills emerged |
| 7 | TUI as visibility + correction | QUEUED | TUI shows last 24h + everything about to happen |
| 8 | PR review loop (puppet master) | QUEUED | PR nit comments resolved without dev touching the PR |

### Deprioritised (pivot 2026-06-10)
CLI aesthetics/theming, savings counter, how-to videos, PG-5 (`/internal/stats`),
Redis R-1–R-6, boardroom/plan as primary features. Not cancelled — below the phases.

### Shipped foundation (pre-pivot, retained)
Git workflow (Phases 1–3), Project + SQLite PM (4A/B), CS-1 IPC→HTTP, CS-2 server-TUI
+ config audit, CS-3 Admin GUI MVP, EPIC-SPLIT (three-codebase), client-server
decoupling Phases 1–2. Detail in Task History below.

---

## Task History

## 2026-06-16 — TASK-071: Wire Phase 2 ticket_id into process_commit; graceful skip when unlinked
**Phase**: Phase 3 — Silent commit handler
**Status**: DONE (PR #178 open against dev, not yet merged)
**Files**:
- `devtrack_server/backend/webhook_server.py` — `TriggerProcessor.process_commit()`: reads
  `data.get("ticket_id", "")` as `resolved_ticket_id` (the Go-resolved, Phase-2-verified
  signal); PM-sync stage now gated on `resolved_ticket_id` truthiness first (skip + log,
  no exception, no queue row, when absent/unlinked), then on `self.workspace_router` alone
  (not `task_data and self.workspace_router`); every `task_data.get(...)` read inside the
  branch falls back to `commit_msg`/`""` when `task_data` is `None`; deleted the
  `task_data.get("ticket_id", "") or commit_hash[:12]` fallback target entirely; confidence
  for the staged `post_comment` action changed from conditional `0.80/0.70` to flat `0.85`.
- `devtrack_server/backend/tests/test_http_triggers.py` — 9 tests added/rewritten: ticket_id
  present/absent/empty-string, NLP-guess independence, no-fallback-hash-target, and (from the
  PM-review fix-up) `task_data is None` and `nlp_parser.parse()` raises — both must still
  stage successfully when `resolved_ticket_id` is present.

**Vision check**: PASS — Non-Negotiable #2 (everything outbound staged via the pending
queue, never bypassed) and #8 (never block on failure: unlinked commits skip silently with
no error; NLP-degraded commits — a documented graceful-degradation state per CLAUDE.md —
still stage successfully now) both upheld.

**Hardcoded scan**: CLEAN — no `os.getenv` outside `config.py` in changed files, no hardcoded
hosts/ports/secrets.

**PM review caught a real bug before merge**: the original implementation gated PM-sync
staging on `task_data and self.workspace_router` — meaning a perfectly good, Phase-2-resolved
`ticket_id` would be silently dropped (no queue action staged at all) whenever the NLP parser
was unavailable or raised. This would have broken the Phase 3 exit criterion on any NLP-
degraded setup. Engineer fixed in a follow-up commit on the same PR: condition changed to
`elif self.workspace_router:`, with `task_data` treated as optional enrichment only. Two new
regression tests cover the exact failure mode (NLP parser absent, NLP parse raises).

**PM independently verified**: re-diffed the branch, re-ran the test file standalone
(36/36 passed), re-ran the hardcoded scan — all clean. Full suite (`uv run pytest backend/tests/ -q`)
625 passed, 1 pre-existing documented failure (`test_ollama_host_returns_string`), no
regressions.

**Engineer notes**: Commits `dffd32c` (initial implementation + tests), `dddaf55` (PM-review
fix-up + regression tests), `e9c52d1`/`770170b` (board/log updates). PR #178 → `dev`. Unblocks
TASK-072 (voice-aware ticket comment generation) and TASK-073 (state-transition queue action),
both dependent on this task's ticket-targeting fix.

---

## 2026-06-17 — TASK-072: Voice-aware ticket comment generation
**Phase**: Phase 3 — Silent commit handler
**Status**: DONE (commit 87e4915, PR #179 open against dev)
**Files**:
- `devtrack_server/backend/commit_message_enhancer.py` — new `generate_ticket_comment(commit_message, diff, files, ticket_id, repo_path)`: reuses `CommitMessageEnhancer._get_provider()` lazy-init LLM chain; same config accessors (`http_timeout()`, `commit_llm_temperature()`, `commit_llm_max_tokens()`); applies `inject_style(context_type="comment")`; fetches diff via `GitDiffAnalyzer.get_commit_diff()` when not in payload; falls back to `f"Commit {short_id}: {commit_message}"` on any exception.
- `devtrack_server/backend/webhook_server.py` — `process_commit`: `pm_payload["description"]` and `pm_payload["comment"]` now sourced from `generate_ticket_comment()` (with belt-and-suspenders try/except falling back to NLP description).
- `devtrack_server/backend/tests/test_ticket_comment_generation.py` — 8 new tests (LLM available, LLM unavailable, inject_style assertions).
- `devtrack_server/backend/tests/test_http_triggers.py` — 3 existing tests updated to patch `generate_ticket_comment`.

**Vision check**: PASS — offline-first (reuses existing Ollama chain, graceful fallback when LLM absent); CLI stays CLI (no GUI/browser); no README/marketing changes.
**Hardcoded scan**: CLEAN — zero `os.getenv` introduced; no hardcoded model names, hosts, or timeout literals.
**Tests**: 633 passed, 0 regressions (1 pre-existing `test_ollama_host_returns_string` failure, documented since TASK-058).
**Engineer notes**: Commit 87e4915. PR #179 → `dev`. Unblocks no further dependencies (TASK-073 was already unblocked by TASK-071 independently).

---

## 2026-06-16 — Phase 3 breakdown: TASK-071 through TASK-074
**Phase**: Phase 3 — Silent commit handler (opened)
**Status**: PLANNED — TASK-071 dispatched to engineer, TASK-072/073/074 queued
**Breakdown rationale**: Investigation during planning found `TriggerProcessor.process_commit()`
(`devtrack_server/backend/webhook_server.py`) still derives its `post_comment` queue target
from the NLP task-matcher's own ticket guess, not from the `ticket_id` field Go's Phase 2
extractor already sends in the commit-trigger JSON payload (`ticket_id,omitempty` on
`CommitTriggerData`, ~100% hit-rate verified in TASK-070). TASK-071 closes that gap first —
every other Phase 3 task depends on the queue action being keyed off the trustworthy
Go-resolved ID. TASK-072 (voice-aware ticket comment via existing `commit_message_enhancer.py`
pipeline, not a new LLM path) and TASK-073 (state transition as an independent queue action
type, with per-platform state-vocabulary mapping) both build on TASK-071 but are independent
of each other. TASK-074 closes the phase with a live runtime verification, mirroring the
TASK-070/Phase-2 closure pattern — checking actual PM-platform side effects, not just queue
status flips.
**Files (planned, not yet touched)**: `devtrack_server/backend/webhook_server.py`
(`process_commit`, `_execute_pm_action`), `devtrack_server/backend/commit_message_enhancer.py`
(new `generate_ticket_comment`), new `devtrack_server/backend/ticket_state_mapper.py`,
`devtrack_client/internal/db/database.go` (`CountTicketCommits`), `devtrack_client/internal/trigger/types.go`
(`IsFirstCommitForTicket`), `devtrack_client/internal/infra/integrated.go`.
**Vision check**: PASS — both new queue actions route through Phase 1's pending_actions
table (Non-Negotiable #2, never bypassed); unresolved tickets skip gracefully, never block
(Non-Negotiable #8); no new cloud dependency; no TUI/CLI gating (CLI parity already covers
queue approve/reject via TASK-064).
**Board**: `Data/agent_logs/project_board.md` — Phase 3 section with TASK-071..074 specs.

---

## 2026-06-16 — TASK-070: Unlinked commit logging + hit-rate metrics in `devtrack status` — PHASE 2 COMPLETE
**Phase**: Phase 2 — Opinionated ticket extractor (closes the phase)
**Status**: DONE (PR #177 open against dev, not yet merged)
**Files**:
- `devtrack_client/internal/db/database.go` — `Database.TicketStats(repoPath string, lastN int) (total, linked, unlinked int, err error)`, using `sql.NullInt64` for the `SUM(...)` aggregate (NULL on empty result set).
- `devtrack_client/internal/db/ticket_stats_test.go` (new) — 4 table-driven tests: counting, `lastN` window limiting, cross-repo aggregation, empty-table edge case.
- `devtrack_client/cli_daemon.go` — new `printTicketExtractionStats(repoPath string)` wired into both branches of `handleStatus()`; named constants `ticketExtractionWindow=50`, `ticketExtractionMinSample=5`.
- `devtrack_client/internal/infra/integrated.go` — `[UNLINKED]` tagged log line added in `handleTrigger()` alongside the pre-existing `ticket_id=unlinked` line from TASK-068.

**Vision check**: PASS — pure local SQLite read + CLI text output; no cloud dependency, no browser, no GUI; offline-first preserved.
**Hardcoded scan**: CLEAN — no secrets, hosts, ports in the diff. Two pre-existing `time.Sleep` literals in `cli_daemon.go` (lines 117, 571) predate this task's diff and were not touched.

**Phase 2 exit criterion — VERIFIED LIVE, not just unit-tested**: engineer registered a disposable scratch repo as a temporary workspace, restarted the live daemon, and ran 10 real commits through the actual fsnotify → handleCommitForWorkspace → handleTrigger → SQLite pipeline (mix: 5 branch-linked, 1 message-linked, 4 active-ticket-fallback). `TicketStats` returned **10/10 linked = 100%**, well above the 80% target. `devtrack status` now shows this as a PASS/BELOW TARGET line on every run, making the criterion permanently checkable rather than a one-time log audit. Scratch workspace and daemon state were cleaned up afterward.

**Engineer notes**: Discovered the git monitor's 2-second poll loop coalesces commits made faster than that interval (only the latest HEAD state at poll time fires a trigger) — not a defect, but worth a future docs note so the next person doesn't lose time on it during similar verification runs. Stretch goal (`devtrack logs --unlinked` filter) was explicitly skipped per spec (optional). Commits: `0b8608d` (implementation + tests), `981112e` (logs/board). PR #177 → `dev`.

**Phase 3 (Silent commit handler) is next**: exit criterion is "Commit → ticket commented + state-transitioned within auto-approve window; dev did nothing." This phase will consume both Phase 1's pending-actions queue (confidence-scored staging, already shipped) and Phase 2's ticket extraction (this phase) to actually post comments/transitions against the resolved ticket ID — the first phase where DevTrack acts on a PM system without being asked.

---

## 2026-06-16 — TASK-069: Commit-message fallback + active-ticket fallback
**Phase**: Phase 2 — Opinionated ticket extractor
**Status**: DONE (PR open against dev, not yet merged)
**Files**: `devtrack_client/internal/infra/integrated.go` (3-strategy fallback chain in `handleCommitForWorkspace`: branch → commit message → `Database.GetLastTicketID`), `devtrack_client/internal/infra/ticket_extraction_test.go` (3 new fallback-chain tests).
**Vision check**: PASS — pure local SQLite + regex logic, no cloud dependency, no CLI/UI surface change.
**Hardcoded scan**: CLEAN — no secrets, hosts, ports, sleeps, or stray `os.Getenv` in changed files.
**Engineer notes**: `GetLastTicketID` and its DB-level test were already built in TASK-068 ahead of schedule and reused as-is — no reimplementation. `go build`, `go vet`, `go test ./...` all clean on branch `feat/TASK-069-commit-message-fallback`. Commits: `6fc4e64` (implementation), `08deca1` (logs/board). PR #176 → `dev`.

---

## 2026-06-14 — Phase 0: Foundation Reset (TASK-057 / TASK-058 / TASK-059)
**Phase**: Phase 0 — Foundation Reset (silent daemon)
**Status**: COMPLETE (code criteria met; runtime verification pending developer binary install)
**Tasks**:
- TASK-057: Silence `handleTrigger()` stdout in `devtrack_client/internal/infra/integrated.go`
- TASK-058: Audit and gate `user_prompt.py` from trigger path in `devtrack_server/backend/`
- TASK-059: Verify Phase 0 exit criterion, run scans, update docs, open PR

**Files changed**:
- `devtrack_client/internal/infra/integrated.go` — 15 `fmt.Print*` calls removed from `handleTrigger()`; replaced with 2 structured `log.Printf` lines (one per trigger type). Decorative separator lines, "What happens next:" paragraph, and "Waiting for next event..." lines removed entirely. `TestIntegrated()` fmt calls left untouched (dev-test helper, never called in production).
- `devtrack_server/backend/user_prompt.py` — two-line status comment added at module level: `# STATUS: Legacy module. Not called from any trigger path as of Phase 0.` / `# Safe to delete once the TUI correction interface (Phase 7) is implemented.`

**Exit criterion status**:
- Code scan: `grep -n "fmt\.Print" devtrack_client/internal/infra/integrated.go` — zero matches in `handleTrigger`; all matches (lines 489–579) are in `TestIntegrated()` only. PASS.
- Build: `go build ./...` PASS | `go vet ./...` PASS.
- Hardcoded-values scan: pre-existing violations only (see note below). No new violations introduced by Phase 0 work.
- Runtime verification: PENDING — developer must install new binary (`go build -o devtrack .` from `devtrack_client/`) and restart daemon (`devtrack stop && devtrack start`), then make a test commit and confirm zero terminal banner output.

**Hardcoded-values scan note (pre-existing, no action required)**:
- Go client (`localhost:[0-9]` scan, excluding test/config/Get): `gitsage/llm.go:53` (Ollama fallback URL), `internal/health/health.go:164,174` (normalizeOllamaHost fallback — documented fix from TASK-043), `setup.go:182,185,230,457,465,690` (interactive setup wizard prompts and .env defaults — correct UX for setup flow).
- Python server (`os.getenv` scan, excluding config.py/conftest/tests): `commit_message_enhancer.py:490` (reads GIT_DIR env — context-specific, not a config var), `github/ghAnalysis.py:228` (reads USER_NAME env — context-specific), `license_manager.py:118` (reads USER/USERNAME OS env — platform identity, not a DevTrack config var), `server_tui/stats_client.py:60,61` and `work_tracker/session_store.py:39,40` (IPC_HOST/port reads — pre-existing from TASK-007 era; not in trigger path).
- None of these were introduced by TASK-057 or TASK-058.

**Vision check**: PASS — Phase 0 removes terminal noise from the trigger flow; offline-first and daemon operation unchanged.

---

## 2026-06-09 — TASK-056: skip_issues flag for dual-platform workspaces
**Phase**: Phase 4A (PM connectors)
**Status**: DONE
**Files**:
- `devtrack_client/internal/config/config.go` — SkipIssues bool added to WorkspaceConfig; ResolveWorkspaceForPath prefers non-skip at equal path depth
- `devtrack_client/cli_connectors.go` — handleIssues() respects SkipIssues
- `devtrack_client/ticket_sync.go` — SyncAllTickets() and PushCachedTickets() respect SkipIssues
- `devtrack_client/versioninfo.json` — bumped to 3.0.9
- `devtrack_wiki/wiki/wiki.html` — WORKSPACES page: skip_issues field, dual-platform example, Common mistakes row; What's New v3.0.9; version badges
**Vision check**: PASS
**Engineer notes**: Bug: two workspace entries for the same repo path (GitHub + Azure DevOps) caused devtrack issues to concatenate results from both platforms. Fixed by skip_issues: true field. 3 code files + version + wiki committed on dev, merged to main. Released as v3.0.9.

---

## 2026-05-24 — TASK-048: Retire legacy directories
**Phase**: EPIC-SPLIT / Phase 3 — cleanup
**Status**: DONE
**Files deleted**:
- `devtrack-bin/` (65+ files — fully superseded by `devtrack_client/`)
- `backend/` at monorepo root (190+ files — fully superseded by `devtrack_server/backend/`)
- `bin/` (2 pre-built binaries)
- `demo/` (9 files)
- `python_bridge.py` (legacy root entry point)
- `scripts/setup_claude_memory.py`
- `docs/build-runner-plan.md`
**Files updated**:
- `Makefile` — all devtrack-bin/ and backend/ references replaced with devtrack_client/ and devtrack_server/
**Vision check**: PASS — pure deletion; canonical copies in devtrack_client/ and devtrack_server/ unaffected; offline-first unchanged
**Engineer notes**: 281 files changed, 69,068 lines deleted. Post-deletion builds: go build ./... EXIT 0; pytest 584 pass (1 pre-existing failure unchanged). Pushed to origin.

---

## 2026-05-29 — gitsage undo wiring + TASK-D download page
**Phase**: Go-Client Standalone initiative / gitsage polish
**Status**: DONE (working tree, branch `feature/go-client-standalone`)
**Files**:
- `devtrack_client/gitsage/agent.go` — `Do()` now returns `(*StepLog, error)` instead of `error`; records HEAD snapshot via `log.Record(repoPath)` before each command batch
- `devtrack_client/gitsage/cli.go` — `RunFollowUpLoop` accepts `*StepLog`; intercepts "undo [N]" input before LLM call, calls `log.Undo(repoPath, n)`, refreshes conversation context after rollback; `RunDoVerbose` passes StepLog from both auto and review/suggest-only paths
- `devtrack_wiki/wiki/download.html` — hero sub, "Getting started" section, step 2, and req-chips updated for two-mode messaging (Standalone vs Full, Python server optional)
**Vision check**: PASS — offline-first; undo works via local git reset; no browser
**Engineer notes**: Undo in follow-up loop now works for all approval modes. Auto mode captures StepLog from Do(). Review/suggest-only mode creates StepLog and records before each batch. "undo N" is intercepted before LLM so it doesn't get interpreted as a question.

---

## 2026-05-27 — TASK-A: Port PM Connectors to Go
**Phase**: Go-Client Standalone initiative / Phase 8 (Integrations)
**Status**: DONE (working tree, branch `feature/go-client-standalone` — not yet pushed)
**Files**:
- `devtrack_client/connectors/github/` — client.go, list.go, view.go, sync.go, check.go (new)
- `devtrack_client/connectors/gitlab/` — client.go, list.go, view.go, sync.go, check.go (new)
- `devtrack_client/connectors/azure/` — client.go, list.go, view.go, sync.go, check.go (new)
- `devtrack_client/database.go` — added `DB() *sql.DB` accessor
- `devtrack_client/cli.go` — replaced 12 Python subprocess handler bodies with Go connector calls; added connector/sage imports; removed requiresManagedMode from all connector commands
**Vision check**: PASS — offline-first (no cloud dependency; auth via env vars), CLI-only, no browser
**Engineer notes**: All 12 connector CLI commands now work without Python server. Tables created lazily on first sync. Hardcoded scan clean.

## 2026-05-27 — TASK-B: Port git-sage to Go (gitsage package extension)
**Phase**: Go-Client Standalone initiative / Phase 6 (Context + Intelligence)
**Status**: DONE (working tree, branch `feature/go-client-standalone` — not yet pushed)
**Files**:
- `devtrack_client/gitsage/config.go` — Config struct, LoadConfig(), multi-provider support (Ollama/OpenAI/Groq) (new)
- `devtrack_client/gitsage/git_ops.go` — GitOps struct, 16 structured git operation methods (new)
- `devtrack_client/gitsage/conflict.go` — Resolver (4 strategies), ParseConflicts, DetectConflicts, Resolve, Report (new)
- `devtrack_client/gitsage/cli.go` — ApprovalMode, ShowApprovalDialog, PromptCommandApproval, CommandHistory, RunFollowUpLoop, RunAsk/RunDo/RunInteractive (new)
- `devtrack_client/gitsage/agent.go` — added unmarshalAgentStep() helper
- `devtrack_client/cli.go` — added sage case, handleSage() dispatcher
**Vision check**: PASS — fully offline with Ollama; no browser; Python server optional (personalization HTTP call degrades gracefully)
**Engineer notes**: Built on existing gitsage/ Go stubs. Undo via ResetSoft available in git_ops.go; dedicated undo command deferred to TASK-C. Follow-up loop retains full conversation context for up to 5 questions.

---

## 2026-05-24 — TASK-047: Update CLAUDE.md and docs for three-codebase split
**Phase**: EPIC-SPLIT / Phase 2 — docs
**Status**: DONE
**Files**:
- `CLAUDE.md` (Codebase Map, three-codebase architecture diagram, devtrack_client/ paths, HTTP_API.md link)
- `README.md` (three-codebase summary, testing section updated to devtrack_client/)
- `devtrack_client/CLAUDE.md` (expanded from stub — build/test/arch/config reference)
- `devtrack_server/CLAUDE.md` (expanded from stub — run/test/arch/boundary reference)
- `docs/ARCHITECTURE.md` (EPIC-SPLIT notice, three-codebase table, diagram updated)
**Vision check**: PASS — pure documentation; no product logic; offline-first unaffected
**Engineer notes**: No new files created. All devtrack-bin/ references updated to devtrack_client/
in developer-facing docs. Legacy flags added on devtrack-bin/ and root backend/. git-sage ownership
clarified as client-owned (devtrack_client/git_sage/). HTTP_API.md and split-manifest.md linked.

---

## 2026-04-10 — TASK-011 through TASK-015: CS-3 Admin GUI MVP
**Phase**: CS-3
**Status**: DONE
**Files**:
- `backend/tests/test_admin_routes.py` (new — 51 tests covering all admin HTTP routes)
- `backend/admin/user_manager.py` (disabled column, disable_user, enable_user)
- `backend/admin/routes.py` (role, disable, enable, reset-password, license, stats routes)
- `backend/admin/templates/license.html` (new — tier/acceptance status page)
- `backend/admin/templates/_stats_panel.html` (new — HTMX trigger activity fragment)
- `backend/admin/templates/base.html` (License nav link)
- `backend/admin/templates/dashboard.html` (license tier stat card, trigger activity card)
- `backend/admin/templates/users.html` (inline role select, disable/enable buttons)
- `backend/config.py` (get_admin_embed accessor)
- `backend/webhook_server.py` (ADMIN_EMBED single-process mount)
- `.env_sample` (ADMIN_EMBED documented)
**Vision check**: PASS — no cloud dependency, no browser launch from CLI, no GUI in Go binary
**Engineer notes**:
TASK-011: 31 HTTP-level route tests via starlette TestClient. DB isolated to tmp_path;
  get_snapshot mocked; ADMIN_USERNAME/PASSWORD set via monkeypatch for check_credentials.
TASK-012: Idempotent ALTER TABLE migration adds `disabled` column; disable_user/enable_user
  helpers; 3 new routes + inline role-change select in users.html; 8 unit + 6 route tests.
TASK-013: GET /admin/license surfacing detect_tier/check_seat_limit/get_acceptance_record;
  license.html with acceptance card, tier card, comparison table; dashboard stat card updated.
TASK-014: get_trigger_stats() wired into dashboard (guarded); _partials/stats HTMX route;
  _stats_panel.html with 4-stat grid; dashboard 30s auto-refresh.
TASK-015: POST reset-password route (self requires current_password verification);
  ADMIN_EMBED=true mounts admin on webhook_server as single process; get_admin_embed() in
  config.py; .env_sample documented; docs updated.
Total suite: 492 passed (was 433 at CS-2 start), 1 pre-existing failure unchanged.

## 2026-04-06 — TASK-010: Full Documentation and Memory Audit
**Phase**: Maintenance / Cross-cutting
**Status**: DONE
**Files**: `CLAUDE.md`, `README.md`, `MEMORY.md`, `Data/agent_logs/feature_tracker.md`, `Data/agent_logs/project_board.md`
**Vision check**: PASS
**Engineer notes**: Corrected all stale docs to reflect CS-1 reality (webhook_server.py as primary Python entry point; env-first config model). CLAUDE.md architecture diagram updated, Python layer section header corrected, Key Patterns updated, Session Completion Status refreshed, Phase 3 debug entries fixed. README webhook section corrected (server runs in managed mode too). MEMORY.md trimmed from 228 to ~160 lines: phase entries collapsed to summary table, "Next Steps" updated, Memory File Index verified against disk. No new inaccuracies introduced.

## 2026-04-05 — TASK-009: CS-2 server_tui headless tests
**Phase**: CS-2
**Status**: DONE
**Files**: `backend/tests/test_server_tui.py` (new, 660 lines, 37 tests)
**Vision check**: PASS
**Engineer notes**: All four non-Textual helpers covered. Linux-first: pytest
tmp_path, generic "python3" cmdlines, no macOS paths or service names. Three
fix iterations: (1) property-raising mocks for AccessDenied/NoSuchProcess;
(2) timestamp format normalised to match SQL cutoff comparisons; (3) URL test
patched backend.config source due to lazy imports. Full suite: 433 passed.

## 2026-04-05 — TASK-008: CS-2 trigger throughput stats panel
**Phase**: CS-2
**Status**: DONE
**Files**: `backend/server_tui/stats_client.py` (new), `backend/server_tui/app.py` (modified)
**Vision check**: PASS
**Engineer notes**: Queries `triggers` table in SQLite; errors defined as unprocessed triggers older than 5 min (no explicit error column in schema). `database_path()` in config.py handles all path fallback. Smoke test returned live data (`last_trigger='17:25'`). StatsRow placed between StatsBar and DataTable with 15s refresh. errors_24h rendered in red when non-zero.

## 2026-04-05 — TASK-007: Fix remaining os.getenv violations
**Phase**: CS-1 / Config
**Status**: DONE
**Files**: `backend/webhook_server.py`, `backend/git_sage/agent.py`, `backend/config.py`
**Vision check**: PASS
**Engineer notes**: Added `get_webhook_gitlab_secret()` accessor. webhook_server.py _cfg()/_cfg_bool() fallbacks replaced. TLS vars in main() use typed accessors. git_sage/agent.py wrapped in try/except import guard for backend.config.

## 2026-04-05 — TASK-001 through TASK-006: os.getenv config cleanup
**Phase**: Config refactor
**Status**: DONE
**Files**: 22 files across backend/azure/, github/, gitlab/, admin/, server_tui/, rag/, and more
**Vision check**: PASS
**Engineer notes**: 50+ new config accessors added to backend/config.py. All os.getenv direct calls replaced across the codebase. .env_sample updated. 397 tests pass.
