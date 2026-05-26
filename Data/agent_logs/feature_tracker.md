# DevTrack Feature Tracker

_Last updated: 2026-05-24 by PM (EPIC-SPLIT TASK-048 complete)_

---

## Roadmap Status

### Client-Server Arc

| Phase | Name | Status | Notes |
|---|---|---|---|
| CS-1 | IPC to HTTP | DONE | `http_trigger.go` + `webhook_server.py` |
| CS-2 | Server TUI + Config Audit | DONE | StatsRow panel (TASK-008); 37 headless tests (TASK-009); os.getenv audit (TASK-001–007) |
| CS-3 | Admin GUI MVP | DONE | Route tests (TASK-011); role/disable (TASK-012); license page (TASK-013); trigger stats (TASK-014); polish+embed (TASK-015) |
| CS-4 | Managed SaaS | PLANNED | Cloud infra + billing |
| CS-5 | Full admin console | FUTURE | Post-SaaS expansion |

### Product Phases

| Phase | Name | Status | Priority |
|---|---|---|---|
| 1-3 | Git workflow | DONE | — |
| 4A/B | Project + SQLite PM | DONE | — |
| 5 | Task planning + sprints | PARTIAL | BacklogManager done; SprintPlanner pending |
| 6 | Context + intelligence | PLANNED | Q4 2026 |
| 7 | Analytics + insights | PLANNED | Q1 2027 |
| 8 | Automation + integrations | PLANNED | Q2 2027 |
| 9-10 | Advanced + interfaces | FUTURE | 2027+ |

---

## Task History

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
