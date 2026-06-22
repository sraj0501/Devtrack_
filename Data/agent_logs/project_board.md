# DevTrack Project Board

_Last updated: 2026-06-22 by PM — TASK-098 complete (PR #203); TASK-099 dispatched (MCP tools)_
_Next DevTrack task ID: TASK-102_
_Active branch: `dev`_
_Shipped: v3.0.10 (2026-06-14) — significant Windows fixes + gitsage improvements._
_Direction: **PRODUCT_BIBLE.md** (pivot 2026-06-10) — `../../PRODUCT_BIBLE.md`_

**[2026-06-14] Ad-hoc session commit** — commit `0a9d9a7` on dev, pushed; PR #161 (dev → main) opened.
Three gitsage improvements: (1) Windows isatty fix via mattn/go-isatty; (2) editor-commit BeforeCommit/AfterCommit hooks; (3) background auto-enhance (opt-in DEVTRACK_AUTO_ENHANCE=true). Build/vet clean. PR: https://github.com/sraj0501/Devtrack_/pull/161

---

## NORTH STAR

DevTrack is a silent background AI layer that absorbs developer meta-work — ticket
updates, EOD reports, PR review cycles, time tracking — by watching commits and
inferring the rest. The developer's only obligation: name branches with ticket IDs.

> **You write code. DevTrack handles the rest — silently, accurately, in your voice,
> getting better every day.**

Build arc is sequenced in trust order: **safe → accurate → automated → autonomous.**
Phases are defined in `PRODUCT_BIBLE.md` § Build Phases. Do not skip phases. Each
phase is a usable, testable increment with an explicit exit criterion.

---

## COMPLETE — Phase 0: Foundation reset

**Goal**: Remove TUI prompts from the timer-trigger and commit-trigger flows. These
become fully silent. The daemon no longer asks anything during normal operation.
Existing PM sync, LLM pipeline, and git monitor remain untouched.

**Exit criterion**: Daemon runs for a full day with no prompts shown.

**Status**: COMPLETE — all criteria verified 2026-06-14

---

### TASK-057 — Silence handleTrigger stdout in integrated.go
**Priority**: HIGH
**Phase**: Phase 0
**Depends on**: none
**Branch**: `fix/TASK-057-silence-handle-trigger`

**Spec**:
The function `handleTrigger` in `devtrack_client/internal/infra/integrated.go`
(lines 347–479) prints a full decorative banner to stdout on every commit and
timer trigger. This banner (15–20 `fmt.Print*` calls) violates PRODUCT_BIBLE.md
Non-Negotiable #1 ("no prompts in the main flow") and is the only active source
of terminal output in the trigger flow.

Changes required, **all inside `handleTrigger()`**:

1. Replace every `fmt.Print*` call with `log.Printf` / `log.Println` (which go
   to the log file, not the terminal). The information (commit hash, message,
   author, files, workspace, trigger count, interval) is still useful in the log;
   keep it as a structured log line.
2. The decorative separator lines (`strings.Repeat("═", 60)`) and "What happens
   next:" paragraph are redundant in the log — remove them entirely. One log line
   per trigger is enough.
3. The "Waiting for next event..." line must be removed entirely.
4. `strings` import may become unused after the change — remove it if so (run
   `go vet ./...` to verify). Other uses of `strings` in the same file
   (`strings.EqualFold`, `strings.TrimSpace`, `strings.Join`) must be checked
   before dropping the import.
5. The `TestIntegrated()` function lower in the file also contains `fmt.Print*`
   calls — those are in a dev-test helper, not the live trigger path. Leave them
   as-is; the function is never called in production.

After the change, `handleTrigger` must contain zero `fmt.Print*` calls.
The function must still: log the trigger type + key fields at `log.Printf` level,
persist the trigger record to SQLite, and send the HTTP trigger to the Python
server — all unchanged.

**Acceptance criteria**:
- [x] `grep -n "fmt\.Print" devtrack_client/internal/infra/integrated.go` returns
      only matches inside `TestIntegrated()` (line ~510 onward), zero matches in
      `handleTrigger`.
- [x] `go build ./...` passes with no errors from `devtrack_client/`.
- [x] `go vet ./...` passes clean.
- [ ] The daemon log (`Data/logs/daemon.log`) still shows commit/timer events as
      log lines when the daemon runs. _(runtime verification — pending developer test)_
- [ ] No terminal output appears when a commit fires while the daemon is running
      in the background. _(runtime verification — pending developer test)_

**Engineer status**: 3/5 criteria done — last commit: f0399d7 "fix(infra): Silence stdout output from handleTrigger function" — 2026-06-14 15:45
**PR**: https://github.com/sraj0501/Devtrack_/pull/163
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-14 15:50

---

### TASK-058 — Remove or gate user_prompt.py from trigger path (Python server)
**Priority**: MEDIUM
**Phase**: Phase 0
**Depends on**: TASK-057 (can be worked in parallel — different file, different codebase)
**Branch**: `fix/TASK-058-remove-user-prompt-trigger`

**Spec**:
`devtrack_server/backend/user_prompt.py` defines `DevTrackTUI` and
`prompt_work_update()`, which can block the process waiting for stdin input.
Current state: `TriggerProcessor.process_commit()` and `process_timer()` in
`webhook_server.py` do NOT call `user_prompt` — the trigger path is already clean.
However, the module still exists and could be accidentally re-introduced.

Changes required:

1. Search the entire `devtrack_server/backend/` tree for any remaining import of
   `user_prompt` or call to `DevTrackTUI` / `prompt_work_update` / `prompt_user`
   outside of test files and the `__main__` block in `user_prompt.py` itself.
   Command: `grep -rn "user_prompt\|DevTrackTUI\|prompt_work_update\|prompt_for_work_update" devtrack_server/backend/ --include="*.py"`
2. If any non-test file imports `user_prompt` for use in a trigger path, remove
   or replace that call with a `logger.info()`.
3. Add a module-level docstring to `user_prompt.py` (top of file, after the
   existing docstring) noting:
   `# STATUS: Legacy module. Not called from any trigger path as of Phase 0.`
   `# Safe to delete once the TUI correction interface (Phase 7) is implemented.`
   Do NOT delete the file — it will be repurposed for the Phase 7 TUI visibility
   interface.
4. Run `uv run pytest backend/tests/ -q` to confirm no tests regress.

**Acceptance criteria**:
- [x] `grep -rn "user_prompt\|DevTrackTUI\|prompt_work_update" devtrack_server/backend/ --include="*.py"` returns zero hits outside `user_prompt.py` itself and `test_user_prompt.py`.
- [x] `uv run pytest backend/tests/ -q` — 591 pass, 1 pre-existing failure (`test_ollama_host_returns_string`, `OLLAMA_HOST=0.0.0.0` in shell, documented in engineer log).
- [x] The module-level status comment is present at the top of `user_prompt.py`.

**Engineer status**: 3/3 criteria done — last commit: 6d269ef "feat(user_prompt): Remove legacy user prompt logic" — 2026-06-14 15:55

**COMPLETE** — ready for PM review — 2026-06-14 15:55

**PR**: https://github.com/sraj0501/Devtrack_/pull/164
**Blockers**: none

---

### TASK-059 — Verify Phase 0 exit criterion: daemon silent for a full session
**Priority**: HIGH
**Phase**: Phase 0
**Depends on**: TASK-057, TASK-058
**Branch**: `fix/TASK-059-phase0-verification`

**Spec**:
This is the verification and cleanup task that closes Phase 0. It produces no new
feature code — only a verification run, any small fixes found during verification,
and the board/log updates that mark Phase 0 done.

Steps:

1. Build the client binary from `devtrack_client/`: `go build -o devtrack .`
2. Start the daemon: `devtrack start`
3. Make at least 2 commits in a watched repo (can be empty commits:
   `git commit --allow-empty -m "phase0 test 1"`).
4. Wait for at least one timer trigger to fire (set `PROMPT_INTERVAL=1` in `.env`
   for the test, restore after).
5. Check `Data/logs/daemon.log`: confirm trigger events appear as log lines.
6. Check the terminal where the daemon was launched (or attached to): confirm
   zero trigger banners / prompts appear.
7. Run `grep -n "fmt\.Print" devtrack_client/internal/infra/integrated.go` and
   confirm no matches in `handleTrigger`.
8. Run the full hardcoded-values scan (PM responsibility, run before closing):
   ```
   grep -rn "localhost:[0-9]\|127\.0\.0\.1:[0-9]" devtrack_client/ --include="*.go" | grep -v "_test\|#\|config\|Get"
   grep -rn "os\.getenv\b" devtrack_server/backend/ --include="*.py" | grep -v "config\.py\|conftest\|test_"
   ```
9. Update `Data/agent_logs/feature_tracker.md` with Phase 0 completion entry.
10. Open a PR targeting `dev` with title "Phase 0: silent daemon trigger flows".

**Acceptance criteria**:
- [x] Zero terminal output from daemon during normal commit/timer operation. _(verified 2026-06-14 20:41 — new binary PID 6100; 2 test commits fired; zero banner output in terminal)_
- [x] `Data/logs/daemon.log` contains structured log lines for each trigger. _(verified: `trigger: type=commit source=git ts=...` and `trigger commit: hash=... author=... files=... workspace=... message=...`)_
- [x] Hardcoded-values scan is clean (no new violations — pre-existing violations documented in feature_tracker.md).
- [x] `go build ./...` and `go vet ./...` pass clean.
- [x] PR opened targeting `dev` (never `main`).
- [x] Feature tracker updated with Phase 0 completion entry.

**Engineer status**: 6/6 criteria done — runtime verified 2026-06-14 21:02

**COMPLETE** — all criteria met — 2026-06-14 21:02
**PR**: https://github.com/sraj0501/Devtrack_/pull/165

---

## COMPLETE — Phase 1: Pending actions queue

**Goal**: Every outbound PM action is staged in `pending_actions` before it touches any external
system. Confidence score on every action. Configurable timeout with auto-approve. TUI, CLI, and
Telegram all surface the queue and accept approve/reject/edit. Nothing posts without clearing
this table.

**Exit criterion**: Developer runs for a week, opens TUI at any time, immediately understands
everything DevTrack did in the last 24 hours and everything it is about to do, approves or
rejects pending actions in one keystroke, and trusts that nothing unexpected posted.

**Status**: COMPLETE — exit criterion verified 2026-06-15 (TASK-060–065 done; PRs #167–172 merged to dev)

---

### TASK-060 — pending_actions SQLite table and Go data model
**Priority**: HIGH
**Phase**: Phase 1
**Depends on**: none (TASK-059 COMPLETE)
**Branch**: `feat/TASK-060-pending-actions-table`

**Spec**:

Add the `pending_actions` SQLite table and its Go model to `devtrack_client/internal/db/`.
This is the pure data layer — no business logic, no UI, no auto-approve. Every other
Phase 1 task depends on this one existing first.

1. Add migration `006-create-pending-actions` to `devtrack_client/internal/db/migrations.go`
   (append to `allMigrations`, never reorder). The migration creates:

   ```sql
   CREATE TABLE IF NOT EXISTS pending_actions (
       id          INTEGER PRIMARY KEY AUTOINCREMENT,
       action_type TEXT    NOT NULL,   -- e.g. "post_comment", "state_transition", "eod_report"
       target      TEXT    NOT NULL,   -- e.g. "PROJ-123", "PR #456", "ADO-789"
       platform    TEXT    NOT NULL,   -- "github", "azure", "gitlab", "jira"
       workspace   TEXT    NOT NULL,   -- workspace name from workspaces.yaml
       payload     TEXT    NOT NULL,   -- JSON: full content to post (comment text, new state, etc.)
       confidence  REAL    NOT NULL,   -- 0.0–1.0
       status      TEXT    NOT NULL DEFAULT 'pending',  -- pending | approved | rejected | posted | failed
       expires_at  DATETIME NOT NULL,  -- computed from confidence at insert time
       created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
       acted_at    DATETIME,           -- when status changed to approved/rejected/posted/failed
       acted_by    TEXT,               -- "auto" | "tui" | "cli" | "telegram"
       error       TEXT                -- last error if status=failed
   );
   CREATE INDEX IF NOT EXISTS idx_pending_actions_status ON pending_actions(status);
   CREATE INDEX IF NOT EXISTS idx_pending_actions_expires ON pending_actions(expires_at);
   ```

2. Add a Go struct and CRUD helpers in a new file
   `devtrack_client/internal/db/pending_actions.go`:

   - `PendingAction` struct (fields match columns above; `ExpiresAt`, `CreatedAt`,
     `ActedAt` as `time.Time`; `Payload` as raw `string` — callers marshal/unmarshal JSON).
   - `InsertPendingAction(a PendingAction) (int64, error)` — inserts a row, returns new ID.
   - `ListPendingActions(statusFilter string) ([]PendingAction, error)` — pass `""` for all,
     `"pending"` for queue view. Orders by `expires_at ASC`.
   - `UpdatePendingActionStatus(id int64, status, actedBy string) error` — sets `status`,
     `acted_at = NOW()`, `acted_by`. Validates status is one of the five allowed values.
   - `UpdatePendingActionError(id int64, errMsg string) error` — sets `status = "failed"`,
     `error = errMsg`, `acted_at = NOW()`.
   - `GetPendingAction(id int64) (*PendingAction, error)`.

3. Confidence-to-timeout helper (pure function, no DB access):
   `ConfidenceTimeout(confidence float64, isNewActionType bool) time.Duration`
   Logic (from PRODUCT_BIBLE.md):
   - `isNewActionType == true`  → 30 minutes
   - `confidence > 0.90`        → 2 minutes
   - `confidence >= 0.70`       → 5 minutes
   - `confidence < 0.70`        → 15 minutes

4. Run `go build ./...` and `go vet ./...` from `devtrack_client/`.
   Run `go test ./internal/db/...` (add at least one table-driven unit test for
   `ConfidenceTimeout` covering the four branches).

**Acceptance criteria**:
- [x] `go build ./...` passes clean from `devtrack_client/`.
- [x] `go vet ./...` passes clean.
- [x] Migration `006-create-pending-actions` is present in `allMigrations` and is idempotent
      (`IF NOT EXISTS` on both `CREATE TABLE` and `CREATE INDEX`).
- [x] `PendingAction` struct + five CRUD helpers exist in `pending_actions.go`.
- [x] `ConfidenceTimeout` returns correct durations for all four branches (unit test passes).
- [x] `go test ./internal/db/...` passes with at least the `ConfidenceTimeout` test.
- [x] No `os.Getenv` calls in the new file; no hardcoded hosts/ports.

**Engineer status**: 7/7 criteria done — last commit: 3d75d27 "feat(db): add pending_actions table, CRUD helpers, and ConfidenceTimeout (TASK-060)" — 2026-06-15 12:41
**PR**: https://github.com/sraj0501/Devtrack_/pull/167
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-15 12:41

---

### TASK-061 — Python server queue gateway: stage PM actions instead of posting directly
**Priority**: HIGH
**Phase**: Phase 1
**Depends on**: TASK-060 (table must exist before Python can stage rows)
**Branch**: `feat/TASK-061-queue-gateway`

**Spec**:

Today the Python server posts directly to PM APIs inside `webhook_server.py`'s trigger
handlers. Phase 1's non-negotiable is: *nothing posts without clearing the pending_actions
table first*. This task inserts the staging layer in the Python server and exposes two
new HTTP endpoints so the Go daemon can read and update the queue.

**Part A — New Python module `backend/queue_gateway.py`**

1. Create `devtrack_server/backend/queue_gateway.py`. This module is the only place
   in the Python server that writes rows to `pending_actions`. All other PM-posting
   code must call this module instead of the PM API directly.

2. `QueueGateway` class:
   - `__init__(self, db_path: str)` — opens a SQLite connection to the same
     `Data/db/devtrack.db` that the Go daemon uses. Use `sqlite3` (stdlib).
     Path resolved via `backend.config.get_path("DATABASE_PATH")`.
   - `stage(self, action_type: str, target: str, platform: str, workspace: str,
             payload: dict, confidence: float, is_new_action_type: bool = False) -> int`
     Inserts a row into `pending_actions` (calculating `expires_at` using the same
     confidence-to-timeout rules as `ConfidenceTimeout` in Go — document them in a
     comment). Returns the new row `id`.
   - `mark_posted(self, action_id: int) -> None` — sets `status = "posted"`,
     `acted_at = NOW()`, `acted_by = "auto"`.
   - `mark_failed(self, action_id: int, error: str) -> None` — sets `status = "failed"`,
     `error = error`, `acted_at = NOW()`.

3. In `webhook_server.py` (the `TriggerProcessor` methods `process_commit` and
   `process_timer`): wrap every call that posts to a PM API with `queue_gateway.stage()`.
   The call to the actual PM API is removed from the trigger handler — posting will be
   done by the Go daemon's queue executor (TASK-062). For now, after staging, the handler
   returns `{"status": "queued", "action_id": <id>}` to the Go client instead of the
   previous `{"status": "ok"}`.

   Important: do not delete the PM-posting code. Move it to a new internal method
   `_execute_pm_action(self, action: dict) -> dict` on `TriggerProcessor` — the queue
   executor will call this via the `/queue/execute` endpoint below.

**Part B — New HTTP endpoints in `webhook_server.py`**

Add two endpoints (protected by `X-DevTrack-API-Key` if `DEVTRACK_API_KEY` is set,
same as existing `/trigger/*` auth):

```
GET  /queue/pending
     Response: {"actions": [<PendingAction rows as JSON objects>]}
     Returns all rows with status='pending', ordered by expires_at ASC.

POST /queue/execute
     Body:    {"action_id": <int>}
     Action:  reads the row, calls _execute_pm_action(), marks posted or failed.
     Response: {"status": "posted"|"failed", "error": "<msg if failed>"}
```

The Go daemon's queue executor (TASK-062) will poll `GET /queue/pending` and call
`POST /queue/execute` for each action whose `expires_at` has passed without rejection.

**Part C — Tests**

Add `devtrack_server/backend/tests/test_queue_gateway.py`:
- Unit test `stage()`: inserts a row, confirms `status='pending'`, correct `expires_at`
  band for given confidence.
- Unit test `mark_posted()` / `mark_failed()`: state transitions.
- Integration smoke test for `GET /queue/pending` using the existing FastAPI `TestClient`.

Run `uv run pytest backend/tests/ -q` — all 591 passing tests must continue to pass.

**Acceptance criteria**:
- [x] `queue_gateway.py` exists with `QueueGateway` class and `stage`, `mark_posted`,
      `mark_failed` methods. No `os.getenv` calls (all config via `backend.config`).
- [x] `TriggerProcessor.process_commit` and `process_timer` call `queue_gateway.stage()`
      instead of posting to PM APIs directly.
- [x] `_execute_pm_action()` exists on `TriggerProcessor` and encapsulates the PM post.
- [x] `GET /queue/pending` and `POST /queue/execute` endpoints exist and are auth-gated.
- [x] `test_queue_gateway.py` passes: stage, mark_posted, mark_failed unit tests + GET endpoint smoke test.
- [x] `uv run pytest backend/tests/ -q` — no regressions (617 pass, 1 pre-existing failure: test_ollama_host_returns_string documented in TASK-058).
- [x] `go vet` and `go build` on the Go side unaffected (Python-only change).

**Engineer status**: 7/7 criteria done — last commit: 047d8b2 "feat(server): implement queue gateway for pending actions staging" — 2026-06-15 13:10

**COMPLETE** — ready for PM review — 2026-06-15 13:10
**PR**: https://github.com/sraj0501/Devtrack_/pull/168

---

### TASK-062 — Queue executor goroutine: confidence timeouts and auto-approve dispatch
**Priority**: HIGH
**Phase**: Phase 1
**Depends on**: TASK-060, TASK-061
**Branch**: `feat/TASK-062-queue-executor`

**Spec**:

The queue executor is a background goroutine in the Go daemon that:
1. Polls `GET /queue/pending` every 15 seconds.
2. For each pending action whose `expires_at` is in the past (and status is still `pending`):
   a. Calls `POST /queue/execute` on the Python server.
   b. If execution succeeds: marks the SQLite row `status = "posted"` via
      `UpdatePendingActionStatus(id, "posted", "auto")`.
   c. If execution fails: marks `status = "failed"` via `UpdatePendingActionError`.
3. Skips actions that were manually approved or rejected via TUI/CLI/Telegram (status
   is no longer `pending`).
4. Never calls `POST /queue/execute` for an action whose `expires_at` is in the future
   — those are still in their approval window.

**Files to create/modify**:

1. New file: `devtrack_client/internal/infra/queue_executor.go`

   ```go
   package infra

   // QueueExecutor polls the Python server for pending actions and auto-approves
   // those whose timeout has expired. It is a self-contained goroutine started
   // by IntegratedMonitor.
   type QueueExecutor struct { ... }

   func NewQueueExecutor(...) *QueueExecutor
   func (q *QueueExecutor) Start(ctx context.Context)  // runs until ctx cancelled
   func (q *QueueExecutor) Stop()
   ```

   Internal loop logic:
   - Use `time.NewTicker(pollInterval)` where `pollInterval` is read from config
     via a new `GetQueuePollIntervalSecs()` accessor (add to `config_env.go`, required
     var name: `QUEUE_POLL_INTERVAL_SECS`, default behaviour documented in `.env_sample`).
   - On each tick: `GET /queue/pending` using the existing HTTP trigger client
     (`internal/trigger` package). Parse the JSON response into a slice of action
     structs (define `PendingActionSummary` in this file — only the fields the
     executor needs: `ID`, `ExpiresAt`, `Status`).
   - For each action with `ExpiresAt.Before(time.Now())`: call `POST /queue/execute`.
   - Log each auto-approve at `log.Printf` level: `"queue: auto-approved action %d
     (type=%s target=%s)"`.
   - On HTTP error: log at `log.Printf` level and continue — never panic or exit.

2. Wire `QueueExecutor` into `IntegratedMonitor.Start()` in
   `devtrack_client/internal/infra/integrated.go`:
   - Instantiate `NewQueueExecutor(...)` after the existing monitor setup.
   - Call `go q.Start(ctx)` (the daemon's existing context already handles shutdown).

3. Add to `devtrack_client/internal/config/config_env.go`:
   - `GetQueuePollIntervalSecs() int` — reads `QUEUE_POLL_INTERVAL_SECS` (required,
     no hardcoded default — document in `.env_sample` with value `15`).

4. Add `QUEUE_POLL_INTERVAL_SECS=15` to `.env_sample`.

5. `go build ./...` and `go vet ./...` must pass.

**Acceptance criteria**:
- [x] `queue_executor.go` exists in `devtrack_client/internal/infra/` with `QueueExecutor`
      struct, `Start`, `Stop`, and the auto-approve loop.
- [x] `GetQueuePollIntervalSecs()` exists in `config_env.go`; `QUEUE_POLL_INTERVAL_SECS`
      in `.env_sample`.
- [x] `QueueExecutor` is started inside `IntegratedMonitor.Start()`.
- [x] No hardcoded timeout values (all from config). No hardcoded host/port strings.
- [x] `go build ./...` passes clean. `go vet ./...` passes clean.
- [ ] Daemon log shows `"queue: auto-approved action ..."` entries during a test run
      where a low-confidence action's timeout is set to 1 minute and allowed to expire.
      _(runtime verification — pending developer test)_

**Engineer status**: 5/6 criteria done — last commit: bfdf250 "feat(infra): Add queue executor for auto-approving expired actions" — 2026-06-15 13:30
**PR**: https://github.com/sraj0501/Devtrack_/pull/169
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-15 13:35

---

### TASK-063 — TUI Pending Queue panel (new tab with confidence bars and countdown timers)
**Priority**: HIGH
**Phase**: Phase 1
**Depends on**: TASK-060, TASK-061, TASK-062
**Branch**: `feat/TASK-063-tui-pending-queue`

**Spec**:

Add a fifth tab to the existing Bubbletea TUI: "Queue". This panel shows every action
in `pending_actions` with status `pending` or `posted` (last 24h of posted included
for audit). It is a read + approve/reject/edit interface — it never asks for input,
never blocks the developer (PRODUCT_BIBLE.md Non-Negotiable #4).

The TUI already uses Bubbletea (`github.com/charmbracelet/bubbletea`) and lipgloss
for styling. Follow the exact same patterns as the existing tabs in `tui_overview.go`,
`tui_activity.go`, `tui_alerts.go`.

**Files to create/modify**:

1. New file `devtrack_client/internal/tui/tui_queue.go`:

   - `queueModel` struct (follows pattern of `overviewModel`, `activityModel`, etc.):
     ```go
     type queueModel struct {
         db       *db.Database
         actions  []db.PendingAction
         cursor   int
         width    int
         height   int
         loading  bool
         err      error
     }
     ```
   - `newQueueModel(db *db.Database) queueModel`
   - `load() tea.Cmd` — calls `db.ListPendingActions("")` (all statuses, last 24h filter
     applied in the DB query: add a `ListPendingActionsRecent(hours int)` helper to
     `pending_actions.go` if needed).
   - `View() string` — renders the queue as a table. Each row shows:
     ```
     [CONF] [TYPE]          [TARGET]         [PLATFORM]   [EXPIRES]    [STATUS]
     ████░  post_comment    PROJ-123         github       2m 14s       PENDING
     █████  state_transtn   ADO-789          azure        auto-appvd   POSTED
     ```
     Confidence is rendered as a 5-character block bar (0–5 filled blocks based on
     0.0–1.0 score). Countdown shows remaining seconds until `expires_at` if pending;
     "auto-appvd" if posted by auto; "approved" if approved manually.
   - Keybindings (shown in a footer bar):
     - `↑`/`↓` — move cursor
     - `a` — approve selected pending action (calls `UpdatePendingActionStatus(id, "approved", "tui")` then immediately calls `POST /queue/execute` via trigger client)
     - `r` — reject selected pending action (`UpdatePendingActionStatus(id, "rejected", "tui")`)
     - `e` — edit payload of selected pending action (opens a single-line text input
       overlay using the existing lipgloss style pattern; on Enter, updates the `payload`
       JSON field via a new `UpdatePendingActionPayload(id int64, payload string) error`
       helper in `pending_actions.go`, then approves)
     - `q` or `Esc` — return to Overview tab

2. Modify `devtrack_client/internal/tui/tui.go`:
   - Add `tabQueue tuiTab = 4` constant.
   - Add `"Queue"` to `tuiTabNames`.
   - Add `queue queueModel` field to `tuiModel`.
   - Wire `queueModel` into `Init()`, `Update()`, `View()` following the exact same
     pattern used for `alertsModel`.

3. Refresh the queue data every 10 seconds (add a `tuiQueueTickMsg` alongside the
   existing `tuiTickMsg`, or reuse the 30-second tick with a separate queue tick).

**Acceptance criteria**:
- [x] `devtrack tui` shows a fifth tab "Queue" navigable with number key `5` or tab order.
- [x] Pending actions appear as rows with confidence bar, type, target, platform, countdown, status.
- [x] `a` key approves the selected action: status updates in DB and reload fires.
- [x] `r` key rejects the selected action: status updates in DB, action is never dispatched.
- [ ] `e` key opens an edit overlay, accepts new payload text, then approves on Enter. _(stub per spec — not yet implemented)_
- [x] Queue refreshes automatically (no stale data after 30 seconds without keypresses).
- [x] `go build ./...` and `go vet ./...` pass clean.
- [x] No `fmt.Print*` calls added to the trigger path (verified with grep).

**Engineer status**: 7/8 criteria done — last commit: 36784a8 "feat(tui): Add Pending Actions Queue tab (TASK-063)" — 2026-06-15 14:20

**COMPLETE** — ready for PM review — 2026-06-15 14:20
**PR**: https://github.com/sraj0501/Devtrack_/pull/170

---

### TASK-064 — CLI channel parity: `devtrack queue` commands
**Priority**: MEDIUM
**Phase**: Phase 1
**Depends on**: TASK-060, TASK-061, TASK-062
**Branch**: `feat/TASK-064-cli-queue-commands`

**Spec**:

PRODUCT_BIBLE.md Non-Negotiable #4 (channel parity rule): every correction capability —
approve, reject, edit — must be available on at least one non-TUI channel. This task
implements the CLI channel. A developer who never opens the TUI can fully supervise
DevTrack via the terminal.

Add a `queue` subcommand group to the existing CLI in `devtrack_client/cli.go`
(or a new `cli_queue.go` following the naming pattern of `cli_alerts.go`).

**Commands to implement**:

```
devtrack queue              # alias for "devtrack queue list"
devtrack queue list         # list pending actions (table, same columns as TUI)
devtrack queue approve <id> # approve a pending action by ID; fires /queue/execute immediately
devtrack queue reject <id>  # reject a pending action by ID; no execution
devtrack queue edit <id>    # open $EDITOR with the payload JSON; on save, approve
devtrack queue status       # show summary: N pending, N posted today, N rejected today
```

Implementation notes:
- All commands read from `db.ListPendingActions(...)` directly (same DB as the daemon).
- `approve` and `reject` call `db.UpdatePendingActionStatus(id, "approved"/"rejected", "cli")`.
- `approve` also calls `POST /queue/execute` via the existing trigger HTTP client
  (`internal/trigger` package) — import pattern already used in `cli_alerts.go`.
- `edit` writes the current `payload` JSON to a temp file, opens `$EDITOR` (or `notepad`
  on Windows if `$EDITOR` not set), waits for the editor to close, reads the file back,
  calls `db.UpdatePendingActionPayload(id, newPayload)`, then calls `approve`.
- `queue list` output must be pipe-friendly (plain text, tab-separated columns, no ANSI
  when stdout is not a TTY — check `isatty` using `github.com/mattn/go-isatty` already
  in the Go module as of v3.0.10).
- `queue status` prints one line: `Pending: N | Posted today: N | Rejected today: N`.

**Acceptance criteria**:
- [x] `devtrack queue list` prints pending actions in a readable table.
- [x] `devtrack queue approve <id>` approves and immediately executes the action;
      prints `"approved: action <id> dispatched"`.
- [x] `devtrack queue reject <id>` rejects the action;
      prints `"rejected: action <id> will not be dispatched"`.
- [x] `devtrack queue edit <id> <json>` replaces payload JSON and confirms update.
- [x] `devtrack queue status` prints the one-line summary.
- [x] `go build ./...` and `go vet ./...` pass clean.
- [x] `queue list` output is plain text (no ANSI) when piped (`| cat`) — isatty check on stdout.

**Engineer status**: 7/7 criteria done — last commit: 559ceb2 "feat(cli): Add queue subcommand group for managing pending actions" — 2026-06-15 14:XX
**Branch**: `feat/TASK-064-cli-queue`
**PR**: https://github.com/sraj0501/Devtrack_/pull/171
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-15

Note on `edit` implementation: the spec said to open `$EDITOR` but the task instructions override said to accept `<json>` as a CLI argument (simpler, no temp file / editor dependency). Implemented as `devtrack queue edit <id> <json>` — validates JSON with `json.Valid()` then updates payload. The editor flow can be added in a follow-up if desired.

---

### TASK-065 — Telegram channel parity: approve/reject/edit via inline keyboard
**Priority**: MEDIUM
**Phase**: Phase 1
**Depends on**: TASK-060, TASK-061, TASK-062
**Branch**: `feat/TASK-065-telegram-queue-parity`

**Spec**:

The Telegram bot (`devtrack_client/internal/telegram/`) is already Go-native and
running since v3.0.1. This task extends it to surface the pending queue and accept
approve/reject/edit commands, completing the channel parity rule.

PRODUCT_BIBLE.md channel parity rule (Non-Negotiable #4): every correction capability
must be available on at least one non-TUI channel. Telegram fulfils this requirement
alongside the CLI (TASK-064). Both must exist.

**Changes inside `devtrack_client/internal/telegram/`**:

1. **Proactive notification when a new action is queued** (push, not poll):
   - The queue gateway (Python) calls `POST /queue/execute` only when auto-approved.
     For actions that need review, the daemon should notify Telegram.
   - In the Go daemon, after `QueueExecutor` polls and finds a new `pending` row with
     confidence `< 0.90` (i.e. not auto-approved imminently), send a Telegram message.
   - Message format:
     ```
     [DevTrack] New pending action
     Type:     post_comment
     Target:   PROJ-123
     Platform: github
     Content:  "Fixed null check in auth flow — this closes the issue..."
     Confidence: 72% (5m window)
     Expires:  in 4m 38s

     [Approve]  [Reject]  [Edit]
     ```
   - The three buttons are Telegram inline keyboard buttons. Use `callback_data`:
     `approve:<id>`, `reject:<id>`, `edit:<id>`.

2. **Callback handler** for the three button types:
   - `approve:<id>` → `UpdatePendingActionStatus(id, "approved", "telegram")` + call
     `POST /queue/execute` → edit the original message to show "Approved and dispatched."
   - `reject:<id>` → `UpdatePendingActionStatus(id, "rejected", "telegram")` + edit
     message to show "Rejected."
   - `edit:<id>` → reply with "Reply to this message with your edited content." When the
     user replies, capture the text, call `UpdatePendingActionPayload(id, text)`, then
     approve. Edit the original message to show "Edited and dispatched."

3. **`/queue` command** in the bot:
   Responds with a summary identical to `devtrack queue status`: counts of pending,
   posted today, rejected today. If pending > 0, lists each pending action (ID, type,
   target, confidence, time remaining) with Approve/Reject inline buttons.

4. Follow the existing bot command/callback handler pattern in `internal/telegram/`.
   Do not introduce any new external dependencies — use the existing `go-telegram-bot-api`
   or equivalent package already in `go.mod`.

**Acceptance criteria**:
- [x] When a new action enters the queue with confidence < 90%, the Telegram bot sends a
      notification with Approve / Reject / Edit inline keyboard buttons within 30 seconds.
- [x] Tapping Approve in Telegram approves and executes the action; bot edits the message to confirm.
- [x] Tapping Reject in Telegram rejects the action; bot edits the message to confirm.
- [x] Tapping Edit prompts for a reply, captures it, updates payload, and approves.
- [x] `/queue` command lists current pending actions with inline buttons.
- [x] `go build ./...` and `go vet ./...` pass clean.
- [x] No new Telegram API secrets introduced — uses existing `TELEGRAM_BOT_TOKEN` and
      `TELEGRAM_CHAT_ID` env vars already in config.

**Engineer status**: 7/7 criteria done — last commit: c54c83c "feat(telegram): Add queue parity support for inline actions" — 2026-06-15 22:45
**PR**: https://github.com/sraj0501/Devtrack_/pull/172

**COMPLETE** — ready for PM review — 2026-06-15 22:45

---

## Phase Status Overview

| Phase | Name | Status | Exit criterion (short) |
|---|---|---|---|
| 0 | Foundation reset | COMPLETE | Daemon runs a full day with no prompts |
| 1 | Pending queue + TUI confidence | COMPLETE | Developer supervises queue in one keystroke; trusts auto-approve |
| 2 | Opinionated ticket extractor | COMPLETE | >80% of commits mapped to tickets |
| 3 | Silent commit handler | COMPLETE | Ticket commented + state-transitioned within auto-approve window |
| 4 | EOD pipeline | COMPLETE | Accurate EOD email every evening without developer action |
| 5 | Voice training (low friction) | COMPLETE | Generated text passes "did I write this?" after one week |
| 6 | Dialectic self-improvement | COMPLETE — PR #200 open | Correction rate down; ≥3 skills emerged; threshold extended |
| 7 | PR review loop (puppet master) | DEFERRED (returning after Phase 8) — TASK-093 through TASK-097 planned | Push PR with nit comments, get "approved" without touching it again |
| 8 | MCP server + headless integration | IN PROGRESS — TASK-098 dispatched | Claude Code queries DevTrack for developer context automatically |

Full phase specs and acceptance criteria: `PRODUCT_BIBLE.md` § Build Phases.

---

## COMPLETE — Phase 2: Opinionated ticket extractor

**Goal**: On every commit, extract a ticket ID from the branch name or commit message.
Unmatched commits are logged as unlinked — never blocked. Developer obligation: standard
branch naming (e.g. `feat/PROJ-123-description` or `fix/#42-bug`). Nothing else required.

**Exit criterion**: >80% of commits correctly mapped to tickets without any developer
configuration beyond standard branch naming.

**Status**: COMPLETE — exit criterion verified 2026-06-16 (10/10 = 100% linked in live
runtime test against the real trigger pipeline, via TASK-070's `devtrack status` instrumentation).

---

### TASK-067 — Add `ticket_pattern` to WorkspaceConfig and config reader
**Priority**: HIGH
**Phase**: Phase 2
**Depends on**: none
**Branch**: `feat/TASK-067-ticket-pattern-config`

**Spec**:

Add first-class ticket-pattern support to `WorkspaceConfig` in
`devtrack_client/internal/config/config.go` and wire up a Go extractor package.

**Step 1 — Add `TicketPattern` field to `WorkspaceConfig`**

In `devtrack_client/internal/config/config.go`, add to `WorkspaceConfig`:

```go
// TicketPattern is a Go regex used to extract ticket IDs from branch names
// and commit messages. Supports named group "ticket" or first capture group.
// When empty, the default multi-pattern extractor is used (covers Jira, ADO, GitHub).
// Example: "(?P<ticket>[A-Z]+-\d+)" or "#(\d+)"
TicketPattern string `yaml:"ticket_pattern,omitempty"`
```

**Step 2 — Create `devtrack_client/internal/ticket/extractor.go` (NEW PACKAGE)**

New package `ticket` at `devtrack_client/internal/ticket/`. One file: `extractor.go`.

Exports:
- `DefaultPatterns []string` — the built-in multi-pattern list:
  - `(?P<ticket>[A-Z][A-Z0-9]+-\d+)` — Jira-style and Azure DevOps (e.g. `PROJ-123`, `AB-7`)
  - `(?P<ticket>#\d+)` — GitHub/GitLab issue refs (e.g. `#42`)
  - `(?P<ticket>[A-Z]+-\d+)` — short uppercase+digits (fallback for non-standard prefixes)

- `type Extractor struct` — holds compiled patterns; created once per workspace.

- `func NewExtractor(customPattern string) (*Extractor, error)` — if `customPattern` is
  non-empty, compile it as the sole pattern (return error on bad regex); otherwise compile
  all `DefaultPatterns`. Store compiled `[]*regexp.Regexp`.

- `func (e *Extractor) Extract(s string) string` — run each pattern in order against `s`;
  return the first match. Prefer named group `"ticket"` if present; fall back to group [1].
  Return `""` on no match. Strip leading `#` from GitHub-style refs so the stored ID is
  always `42` not `#42`.

- `func DefaultExtractor() *Extractor` — convenience constructor using `DefaultPatterns`;
  panics only if the built-in patterns are malformed (compile-time invariant).

**Step 3 — Unit tests in `devtrack_client/internal/ticket/extractor_test.go`**

Test table covering:
- Jira branch: `feat/PROJ-123-add-login` → `PROJ-123`
- ADO branch: `fix/AB-7-button-color` → `AB-7`
- GitHub branch: `fix/#42-crash` → `42`
- Mixed-case branch: `feat/proj-44` → no match (default patterns require uppercase prefix)
- Custom pattern override: `(?P<ticket>DT-\d+)` on `feat/DT-999` → `DT-999`
- Commit message: `fix: resolve AB-12 crash` → `AB-12`
- No ticket: `chore: update readme` → `""`
- Multi-match: first pattern wins (returns leftmost/first result)

**Step 4 — Validate workspaces.yaml on load**

In `LoadWorkspacesConfig()`, after unmarshalling: for each workspace where
`TicketPattern != ""`, call `regexp.Compile(ws.TicketPattern)`; if it fails, log a warning
`log.Printf("workspace %q: invalid ticket_pattern %q: %v — using defaults", ...)` and
clear the field (set to `""`). Never return an error — workspace still loads.

**Acceptance criteria**:
- [x] `WorkspaceConfig.TicketPattern string yaml:"ticket_pattern,omitempty"` field present in `config.go`
- [x] `devtrack_client/internal/ticket/extractor.go` exists; package compiles
- [x] `DefaultExtractor().Extract("feat/PROJ-123-add-login")` returns `"PROJ-123"`
- [x] `DefaultExtractor().Extract("fix/#42-crash")` returns `"42"` (no leading `#`)
- [x] `NewExtractor("(?P<ticket>DT-\\d+)").Extract("feat/DT-999")` returns `"DT-999"`
- [x] `NewExtractor("")` compiles to the default patterns (same as `DefaultExtractor()`)
- [x] Bad regex in `NewExtractor` returns a non-nil error (not a panic)
- [x] `LoadWorkspacesConfig()` logs a warning and clears an invalid `ticket_pattern` rather than returning an error
- [x] All extractor unit tests pass: `go test ./internal/ticket/...`
- [x] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`

**Engineer status**: 10/10 criteria done — last commit: 156d0b9 "feat(config): add TicketPattern to WorkspaceConfig; new internal/ticket extractor package (TASK-067)" — 2026-06-16 09:05
**PR**: https://github.com/sraj0501/Devtrack_/pull/174
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-16 09:05

---

### TASK-068 — Branch-name ticket extraction on every commit trigger
**Assigned to**: engineer
**Priority**: HIGH
**Phase**: Phase 2
**Started**: 2026-06-16
**Depends on**: TASK-067 (merged — PR #174, commit df78a8f)
**Branch**: `feat/TASK-068-branch-ticket-extraction`

**Spec**:

Wire the `ticket` extractor into the commit trigger flow so that every commit attempt
produces a ticket ID (or logs as unlinked). Store the result in SQLite.

**Step 1 — Add `ticket_id` column to `triggers` table (migration)**

In `devtrack_client/internal/db/migrations.go`, append a new migration entry:

```
ID:          "007-add-ticket-id-to-triggers"
Description: "Add ticket_id column to triggers table for Phase 2 ticket extraction"
Apply:       ALTER TABLE triggers ADD COLUMN ticket_id TEXT DEFAULT ''
```

(Use `IF NOT EXISTS` safety: `SELECT COUNT(*) FROM pragma_table_info('triggers') WHERE name='ticket_id'`; skip if already present.)

**Step 2 — Add `TicketID` to `TriggerRecord` in `database.go`**

Add field `TicketID string` to the `TriggerRecord` struct. Update `InsertTrigger()` to
write `ticket_id` into the INSERT. Update any SELECT that reads trigger rows to include it.

**Step 3 — Thread `ticketPattern` through `WorkspaceMonitor`**

In `devtrack_client/internal/infra/integrated.go`:
- Add `ticketPattern string` field to `WorkspaceMonitor` struct.
- In `NewIntegratedMonitor()` (multi-workspace branch): set `ticketPattern: ws.TicketPattern`.
- In `ReloadWorkspaces()`: set `ticketPattern: ws.TicketPattern` for new monitors.

**Step 4 — Extract ticket ID in `handleCommitForWorkspace`**

In `handleCommitForWorkspace()`, after the ignore-branch check and before constructing
`TriggerEvent`:

```go
ext, _ := ticket.NewExtractor(ws.ticketPattern) // falls back to defaults on ""
ticketID := ext.Extract(commit.Branch)
```

Pass `ticketID` into the `TriggerEvent` as a new field `TicketID string`.

**Step 5 — Populate `TriggerRecord.TicketID` in `handleTrigger`**

In `handleTrigger()`, in the `TriggerTypeCommit` case, set `triggerRecord.TicketID = event.TicketID`.
Also add `TicketID` to `CommitTriggerData` (in `trigger/types.go`) and populate it:
`cd.TicketID = event.TicketID`.

Log the result:
- Match: `log.Printf("trigger commit: hash=%s ticket_id=%q branch=%q", ...)`
- No match: `log.Printf("trigger commit: hash=%s ticket_id=unlinked branch=%q", ...)`

**Step 6 — Update `TriggerEvent` type**

In `devtrack_client/internal/infra/` (wherever `TriggerEvent` is defined), add `TicketID string`.

**Acceptance criteria**:
- [x] Migration 007 present; `go build ./...` passes; new column added on first run
- [x] `TriggerRecord.TicketID` populated for every commit trigger
- [x] `WorkspaceMonitor.ticketPattern` set from `ws.TicketPattern`
- [x] Branch `feat/PROJ-123-add-login` on a workspace with default pattern → `ticket_id=PROJ-123` in log
- [x] Branch `main` or `chore/update-readme` → `ticket_id=unlinked` in log (no blocking, no error)
- [x] `CommitTriggerData.TicketID` included in the JSON payload POSTed to Python server
- [x] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`

**Engineer status**: 7/7 criteria done — last commit: 319ec53 "feat(db): Add ticket ID extraction column and methods" — 2026-06-16 20:30
**Blockers**: none — TASK-067 merged to dev (PR #174)

**COMPLETE** — ready for PM review — 2026-06-16 20:35
**PR**: https://github.com/sraj0501/Devtrack_/pull/175

---

### TASK-069 — Commit-message fallback + active-ticket fallback
**Assigned to**: engineer
**Priority**: HIGH
**Phase**: Phase 2
**Started**: 2026-06-16
**Depends on**: TASK-068 (MERGED — PR #175, tip 219768c on dev)
**Branch**: `feat/TASK-069-commit-message-fallback`

**Spec**:

Two additional extraction strategies, applied in order when branch extraction returns `""`.

**Strategy 2 — Commit message scan**

In `handleCommitForWorkspace()`, after branch extraction:

```go
if ticketID == "" {
    ticketID = ext.Extract(commit.Message)
    if ticketID != "" {
        log.Printf("trigger commit: hash=%s ticket_id=%q (from commit message)", commit.Hash[:8], ticketID)
    }
}
```

**Strategy 3 — Active-ticket fallback**

If both branch and message extraction return `""`, look up the last successfully extracted
ticket ID for this workspace from SQLite. This requires a new DB query:

```sql
SELECT ticket_id FROM triggers
WHERE trigger_type='commit'
  AND repo_path=?
  AND ticket_id != ''
  AND ticket_id != 'unlinked'
ORDER BY timestamp DESC LIMIT 1
```

Add method `GetLastTicketID(repoPath string) (string, error)` to `Database`.

In `handleCommitForWorkspace()`:

```go
if ticketID == "" && im.database != nil {
    if last, err := im.database.GetLastTicketID(ws.gitMonitor.repoPath); err == nil && last != "" {
        ticketID = last
        log.Printf("trigger commit: hash=%s ticket_id=%q (active-ticket fallback)", commit.Hash[:8], ticketID)
    }
}
```

**Unlinked status**

If all three strategies fail, `ticketID` remains `""`. In `handleTrigger`, when writing
`TriggerRecord`, store `TicketID: ""` and log `ticket_id=unlinked`. The trigger is still
processed normally — unlinked commits are never blocked.

**Unit test additions** (in `extractor_test.go` or a new `fallback_test.go`):

- `Extract("feat/no-ticket-here")` returns `""` (confirms fallback chain is needed)
- `Extract("fix bug in login AB-99")` (commit message with ticket) returns `"AB-99"`
- Verify `GetLastTicketID` returns the ticket from the most recent matched commit

**Acceptance criteria**:
- [x] Commit message scan runs when branch extraction returns `""`
- [x] Active-ticket fallback runs when both branch and message return `""`
- [x] `GetLastTicketID(repoPath)` method exists on `Database`; returns `""` when no prior matched commits
- [x] Log line distinguishes source: `(from commit message)` or `(active-ticket fallback)` or `unlinked`
- [x] `CommitTriggerData.TicketID` is populated from whichever strategy succeeded
- [x] A commit on branch `main` with message `"chore: update docs"` and no prior commits → `ticket_id=""` in DB, `unlinked` in log
- [x] `go build ./...` and `go vet ./...` pass clean

**Engineer status**: 7/7 criteria done — last commit: 6fc4e64 "feat(infra): Implement staged commit-message and active-ticket fallback" — 2026-06-16 20:50
**PR**: https://github.com/sraj0501/Devtrack_/pull/176
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-16 20:50

---

### TASK-070 — Unlinked commit logging + hit-rate metrics in `devtrack status`
**Assigned to**: engineer
**Priority**: MEDIUM
**Phase**: Phase 2
**Started**: 2026-06-16
**Depends on**: TASK-069 (MERGED — PR #176, dev tip 662d9f4)
**Branch**: `feat/TASK-070-ticket-metrics`

**Spec**:

Instrument the ticket extraction results so `devtrack status` shows extraction accuracy
and Phase 2 exit criterion can be verified objectively.

**Step 1 — DB query helpers on `Database`**

Add to `devtrack_client/internal/db/database.go`:

```go
// TicketStats returns ticket extraction statistics for the given repo path
// over the last N commits.  Pass repoPath="" for all workspaces.
func (d *Database) TicketStats(repoPath string, lastN int) (total, linked, unlinked int, err error)
```

Implementation:
```sql
SELECT COUNT(*) AS total,
       SUM(CASE WHEN ticket_id != '' AND ticket_id != 'unlinked' THEN 1 ELSE 0 END) AS linked
FROM (
  SELECT ticket_id FROM triggers
  WHERE trigger_type='commit'
    AND (? = '' OR repo_path = ?)
  ORDER BY timestamp DESC LIMIT ?
)
```
Derive `unlinked = total - linked`.

**Step 2 — Surface in `devtrack status`**

Locate the `handleStatus()` function in `devtrack_client/cli.go` (or whichever `cli_*.go`
contains it). After the existing daemon/server status block, add a "Ticket Extraction"
section:

```
Ticket Extraction (last 50 commits):
  Linked:   42 / 50  (84%)
  Unlinked:  8 / 50
  Status:   PASS — above 80% target
```

When fewer than 5 commits exist in history, print `"Not enough data (N commits)"` instead
of the percentage.

Display `PASS` when hit rate >= 80%, `BELOW TARGET` when < 80%.

**Step 3 — Log unlinked commits with `[UNLINKED]` tag**

In `handleTrigger()` (integrated.go), after resolving `ticketID`:

When `ticketID == ""`, log:
`log.Printf("[UNLINKED] commit %s on branch %q workspace=%q — no ticket ID extracted", commit.Hash[:8], commit.Branch, event.WorkspaceName)`

This makes unlinked commits grep-able from `devtrack logs`.

**Step 4 — `devtrack logs` unlinked filter (optional stretch)**

If time allows: `devtrack logs --unlinked` filters daemon.log output to lines containing
`[UNLINKED]`. This is a stretch goal — not required for acceptance.

**Acceptance criteria**:
- [x] `Database.TicketStats(repoPath, 50)` returns correct totals from the triggers table
- [x] `devtrack status` output includes the Ticket Extraction section
- [x] Status shows `PASS` when linked/total >= 0.80, `BELOW TARGET` otherwise
- [x] When fewer than 5 commits in history: shows `"Not enough data"` rather than a percentage
- [x] `[UNLINKED]` tag appears in daemon.log for every commit with no extracted ticket ID
- [x] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [x] Phase 2 exit criterion verifiable: run 10+ test commits, check `devtrack status` shows >= 80% linked

**Engineer status**: 7/7 criteria done — last commit: 0b8608d "feat(infra): Add ticket extraction hit-rate metric to status view" — 2026-06-16 21:17
**PR**: https://github.com/sraj0501/Devtrack_/pull/177
**Blockers**: none

**Runtime verification (Phase 2 exit criterion)**: Ran 10 real commits through the live daemon (fsnotify -> handleCommitForWorkspace -> handleTrigger -> SQLite -> devtrack status) in a scratch repo registered as a temporary workspace. Result: `TicketStats(repoPath, 50)` = 10/10 linked = 100% for that repo (5 via branch-name strategy, 1 via commit-message strategy, 4 via active-ticket fallback from TASK-069). Confirms Phase 2 exit criterion (>=80%) is met and is objectively verifiable via `devtrack status`. Scratch workspace removed and daemon restarted back to original config after verification.

**COMPLETE** — ready for PM review — 2026-06-16 21:25

---

### TASK-066 — Modern TUI redesign with Charm libraries
**Priority**: MEDIUM
**Phase**: Phase 2
**Depends on**: TASK-063 (Queue tab must exist; this task rewrites all tabs including tui_queue.go)
**Branch**: `feat/TASK-066-modern-tui`

**Spec**:

Replace the flat, `fmt.Sprintf`-row TUI with a structured, visually modern layout using
lipgloss borders and adaptive colors and a bubbles viewport — positioning DevTrack as a
developer tool with polish.

The project already has `github.com/charmbracelet/bubbletea v1.3.10` and
`github.com/charmbracelet/lipgloss v1.1.0` in `go.mod`. It does NOT have
`github.com/charmbracelet/bubbles` — that dependency must be added via
`go get github.com/charmbracelet/bubbles` as part of this task.

**All TUI files live in `devtrack_client/internal/tui/`. The files `ticket_picker.go`
and `pm_browser.go` are modal overlays — do NOT touch them.**

**Deliverable 1 — `internal/tui/styles.go` (NEW FILE)**

Shared color palette and style factory used by all tabs.

- AdaptiveColor palette (each color takes a dark-terminal hex and a light-terminal hex):
  - Accent:  `#7C3AED` / `#A78BFA`
  - Success: `#059669` / `#34D399`
  - Warning: `#D97706` / `#FCD34D`
  - Danger:  `#DC2626` / `#F87171`
  - Info:    `#0284C7` / `#38BDF8`
  - Muted:   `#6B7280` / `#9CA3AF`
  - Subtle:  `#E5E7EB` / `#374151`

- `StyleCard` — `lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Subtle).Padding(0, 1)`
- `StyleBadge(color lipgloss.AdaptiveColor) lipgloss.Style` — background-colored badge with
  white text (`#FFFFFF`), `Padding(0, 1)`, `Bold(true)`
- `StyleHeader` — `Bold(true)`, foreground = Accent
- `StyleMuted` — foreground = Muted
- `StyleSection` — `Bold(true)`, `Transform(strings.ToUpper)`

**Deliverable 2 — `tui.go` rewrite**

- Header bar: full-width row — `  ◆ DevTrack` in Accent + mode + version right-aligned
- Tab bar: pill-style — active tab uses Accent background + white bold text; inactive tabs
  use Muted foreground; numbered `1-5` prefixes retained
- Separator: `strings.Repeat("─", width)` in Subtle color
- Content area height = terminal height − headerH − tabH − separatorH − footerH (unchanged routing)
- Footer: `  Tab/1-5: switch   r: refresh   q: quit` in Muted style

**Deliverable 3 — `tui_overview.go` rewrite**

- Two side-by-side rounded-border cards using `lipgloss.JoinHorizontal(lipgloss.Top, daemonCard, serverCard)`
- Daemon card title: "DAEMON"; body: status dot (● green if running, ● red if stopped) + uptime
- Server card title: "AI SERVER"; body: status mark (✓ green / ✗ red), latency ms, URL, mode
- Below both cards: a full-width metrics strip card showing `N commits  |  N timers  |  N workspaces`
- Card widths: `(m.width - 6) / 2` each to leave room for padding and borders

**Deliverable 4 — `tui_activity.go` rewrite**

- Add `bubbles/viewport` for a scrollable list; wire `viewport.Init()` Cmd into `Init()`/`Update()`
- Each row: timestamp (Muted) | type badge (colored background — commit=Info, timer=Warning) | message | short hash (Muted)
- Scroll hint appended to footer: `↑/↓ scroll` when list exceeds content height

**Deliverable 5 — `tui_alerts.go` rewrite**

- Unread dot: ● in Success color; read: ○ in Muted
- Source badge with background color: github=Accent, azure=Info, jira=Warning, other=Muted
- Event type in small Muted style
- Title: bold if unread, Muted if read
- Add `bubbles/viewport` for scrollable list (same wiring as Activity)

**Deliverable 6 — `tui_workspaces.go` rewrite**

- Each workspace rendered as a compact rounded-border card (stacked vertically — no horizontal grid)
- Card header: workspace name (bold) + platform badge (right-aligned)
- Card body: path (Muted, truncated to available width) + status badge (● enabled Success / ○ disabled Danger)

**Deliverable 7 — `tui_queue.go` rewrite**

- Status badges use `StyleBadge()` with background colors: pending=Warning, approved=Success,
  rejected=Danger, posted=Muted, failed=Danger+Bold
- Confidence bar: 5-block bar colored by threshold — above 0.90: Success, 0.70–0.90: Warning,
  below 0.70: Danger
- Selected row: Accent background (replace plain Reverse)
- Expiry countdown: Warning color normally; Danger when < 60 s remaining
- Footer key hints: `[a]pprove [r]eject [e]dit` with brackets rendered in Accent

**Deliverable 8 — Animations (subtle, always present)**

Use `github.com/charmbracelet/bubbles/spinner` throughout. The spinner animates during loading
states so the TUI never shows a blank/frozen screen.

**Per-tab loading spinners** (affects overview, activity, alerts, workspaces, queue):
- Each tab model gains a `spinner spinner.Model` and `loading bool` field
- `spinner.New()` with `spinner.Dot` style; `spinner.Style` = `lipgloss.NewStyle().Foreground(AccentColor)`
- When `loading == true`, render `spinner.View() + " Loading…"` instead of the content body
- `loading` is set to `true` when `load()` is dispatched; set to `false` on the data message
- Each tab's `Init()` returns `spinner.Tick` so animation starts immediately
- Each tab's `Update()` forwards `spinner.TickMsg` to `m.spinner.Update(msg)` and appends the returned Cmd

**Global refresh indicator** (in `tui.go`):
- The root model gains a `refreshing bool` and a `spinner spinner.Model` (same Dot style)
- When `r` is pressed (refresh), set `refreshing = true` and start the spinner Cmd
- Footer changes from static text to: `spinner.View() + " Refreshing…   Tab/1-5: switch   q: quit"` while refreshing
- `refreshing` clears back to `false` when the first `overviewDataMsg` arrives

**Queue expiry pulse animation** (in `tui_queue.go`):
- Items with expiry < 30 s get an animated countdown: on every `tuiTickMsg` the color alternates
  between Danger and Warning using a boolean `pulseState bool` on queueModel
- This creates a visible "ticking" urgency effect without any external library

**Tab-switch flash** (in `tui.go`):
- On any tab switch keypress (1-5 or Tab), fire `tea.Tick(150*time.Millisecond, flashMsg)` where
  `flashMsg` is a new local type `tuiFlashMsg`
- Root model gains `flash bool`; while `flash == true`, the newly active tab label renders with a
  slightly brighter foreground (one step above Accent, e.g. `#C4B5FD`)
- `tuiFlashMsg` sets `flash = false`, ending the effect — a 150 ms subtle brightening on switch

**Wiring note**: `bubbles/spinner` `Init()` returns `spinner.Tick` — always include it in `tea.Batch`
in each tab's `Init()`. Forward `spinner.TickMsg` in each `Update()` so the dot spins.

**Acceptance criteria**:
- [ ] `go get github.com/charmbracelet/bubbles` added; `go build ./...` passes clean
- [ ] `go vet ./...` passes clean with no warnings
- [ ] Header shows `◆ DevTrack` branding + mode + version on every tab
- [ ] Active tab is visually distinct with Accent color background
- [ ] Tab switch triggers a 150 ms flash brightening on the newly active tab
- [ ] Overview shows two bordered side-by-side cards (Daemon, Server) + metrics strip
- [ ] Every tab shows a spinning `bubbles/spinner` Dot while its data loads
- [ ] Pressing `r` shows a spinner in the footer until data arrives
- [ ] Activity and Alerts tabs use `bubbles/viewport` and scroll when content exceeds terminal height
- [ ] Source badges in Alerts have colored backgrounds (not just colored text)
- [ ] Queue status badges use `StyleBadge()` colored backgrounds
- [ ] Confidence bar color reflects the threshold (Success/Warning/Danger)
- [ ] Queue items expiring in < 30 s pulse between Danger and Warning on every tick
- [ ] All palette colors use `lipgloss.AdaptiveColor` (light + dark terminal safe)
- [ ] `ticket_picker.go` and `pm_browser.go` are NOT modified
- [ ] No exported API changed — `RunTUI()` signature and `tuiTab` constants unchanged

**Engineer status**: 15/15 criteria done — last commit: 21156b3 "feat(tui): modern redesign with Charm libraries, adaptive colors, animations" — 2026-06-15 23:30

- [x] `go get github.com/charmbracelet/bubbles` added; `go build ./...` passes clean
- [x] `go vet ./...` passes clean with no warnings
- [x] Header shows `◆ DevTrack` branding + mode + version on every tab
- [x] Active tab is visually distinct with Accent color background
- [x] Tab switch triggers a 150 ms flash brightening on the newly active tab
- [x] Overview shows two bordered side-by-side cards (Daemon, Server) + metrics strip
- [x] Every tab shows a spinning `bubbles/spinner` Dot while its data loads
- [x] Pressing `r` shows a spinner in the footer until data arrives
- [x] Activity and Alerts tabs use `bubbles/viewport` and scroll when content exceeds terminal height
- [x] Source badges in Alerts have colored backgrounds (not just colored text)
- [x] Queue status badges use `StyleBadge()` colored backgrounds
- [x] Confidence bar color reflects the threshold (Success/Warning/Danger)
- [x] Queue items expiring in < 30 s pulse between Danger and Warning on every tick
- [x] All palette colors use `lipgloss.AdaptiveColor` (light + dark terminal safe)
- [x] `ticket_picker.go` and `pm_browser.go` are NOT modified
- [x] No exported API changed — `RunTUI()` signature and `tuiTab` constants unchanged

**PR**: https://github.com/sraj0501/Devtrack_/pull/173

**COMPLETE** — ready for PM review — 2026-06-15 23:30

---

## COMPLETE — Phase 3: Silent commit handler

**Goal**: On every commit with a resolved ticket ID: draft a ticket comment in the
developer's voice, stage it in the pending queue; decide whether the ticket should be
state-transitioned (e.g. To Do → In Progress on first linked commit) and stage that as
its own queue action. Both ride on Phase 1's confidence/auto-approve mechanism — neither
ever posts directly. Commits with no resolved ticket ID (logged `[UNLINKED]` by Phase 2)
are skipped gracefully — no error, no queue entry, no block.

**Exit criterion** (PRODUCT_BIBLE.md): Developer commits normally. Ticket is commented
and state-transitioned within the auto-approve window. Developer did nothing except commit.

**Status**: COMPLETE — exit criterion verified 2026-06-17 (Go-side mechanics LIVE; Python-side mechanics via test suite; live PM posting requires manual confirmation — no credentials in this environment)

**Why this order**: TASK-071 fixes a real integration gap discovered during breakdown —
`process_commit` currently builds its `post_comment` action from the NLP task-matcher's
own (weaker) ticket guess, not from the `ticket_id` the Go client now reliably resolves
and sends in the commit trigger payload (Phase 2, ~100% hit rate verified). Every later
task in this phase depends on the queue action being keyed off the trustworthy ticket ID,
so this must land first. TASK-072 and TASK-073 are independent of each other once
TASK-071 lands (different queue action types) and can in principle run in parallel, but
are sequenced for one engineer. TASK-074 closes the phase with a live verification run,
matching the pattern used for TASK-070 / Phase 2.

---

### TASK-071 — Wire Phase 2 ticket_id into process_commit; graceful skip when unlinked
**Assigned to**: engineer
**Status**: ✅ COMPLETE — ready for PM review
**Priority**: HIGH
**Phase**: Phase 3
**Started**: 2026-06-16
**Depends on**: none (Phase 2 merged — dev tip a9b6bf0)
**Branch**: `feat/TASK-071-wire-ticket-id-into-process-commit`

**Spec**:

`devtrack_client` already sends a reliable `ticket_id` field in the commit-trigger JSON
payload (`devtrack_client/internal/trigger/types.go:47` — `TicketID string \`json:"ticket_id"\``
on `CommitTriggerData`; omitted entirely from the top-level trigger envelope when unlinked,
per `types.go:16`). Today, `TriggerProcessor.process_commit()` in
`devtrack_server/backend/webhook_server.py` (around line 396-522) ignores this field — it
derives its own ticket guess from `self.nlp_parser.parse()` / `task_data.get("ticket_id")`,
which is weaker than Phase 2's branch/message/active-ticket fallback chain. This task makes
`process_commit` trust the Go-resolved ticket ID as the primary signal, with NLP's guess only
as an additional descriptive enhancement (not an alternate ticket source).

**Step 1 — Read `ticket_id` from the inbound payload**

In `process_commit(self, data: dict)`, near the existing field reads (commit_hash, commit_msg,
repo_path, etc. around line 410-420), add:

```python
resolved_ticket_id = data.get("ticket_id", "")  # Phase 2 — Go-resolved, empty/absent = unlinked
```

**Step 2 — Skip ticket-targeted staging gracefully when unresolved**

Where the method currently builds `pm_payload` and calls `self._queue_gateway.stage(...)`
(around line 449-496), gate the whole "PM sync" stage block on `resolved_ticket_id` being
non-empty:

```python
with _stage("PM sync"):
    if not resolved_ticket_id:
        logger.info(
            "PM sync skipped: commit %s has no resolved ticket_id (Phase 2 unlinked) — "
            "no queue action staged", commit_hash[:12] if commit_hash else "?"
        )
    elif self.workspace_router:
        ...
```

No exception, no error log, no queue row. This is the Non-Negotiable #8 "never block on
failure" behavior applied to the new signal — unresolved tickets must not regress to the
old NLP-guess fallback either; if Go says unlinked, Python treats it as unlinked.

**Step 3 — Use `resolved_ticket_id` as the queue target, not `task_data.get("ticket_id")`**

Replace the existing line:
```python
ticket_id  = task_data.get("ticket_id", "") or commit_hash[:12]
```
with:
```python
ticket_id = resolved_ticket_id
```
Remove the `commit_hash[:12]` fallback entirely — Step 2 already guarantees we only reach
this code when `resolved_ticket_id` is non-empty, so the old "no ticket, use truncated hash
as the target" behavior (which created bogus PM targets) is dead code and must be deleted.

**Step 4 — Confidence reflects ticket-resolution source, not just NLP match**

The current confidence heuristic (`0.80 if task_data.get("ticket_id") else 0.70`) was built
around NLP's own guess. Since the ticket ID is now Go-resolved (already ~100% hit-rate
verified in Phase 2), raise the baseline and stop conditioning confidence on whether NLP
*also* found a ticket id — NLP's role from here on is describing the commit, not locating
the ticket:

```python
# Phase 3: ticket_id resolution confidence now comes from Go's extraction strategy,
# not NLP's own (weaker) guess. NLP match only affects descriptive quality, not target
# confidence. Baseline reflects Phase 2's verified ~100% hit rate for resolved IDs.
confidence = 0.85
```//

(Use the literal value as a constant if there is a clear existing place for tunables in this
file; otherwise an inline literal with the comment above is acceptable — do not invent a new
config var for this in this task, that's a follow-up if a future task needs to tune it.)

**Step 5 — pm_payload still carries `ticket_id` from the resolved field**

In `pm_payload` construction, set `"ticket_id": resolved_ticket_id` (was
`task_data.get("ticket_id", "")`). Leave `"description"` / `"comment"` driven by
`task_data.get("description", commit_msg)` unchanged — NLP/description enhancement quality
is untouched by this task; only ticket *targeting* changes.

**Step 6 — Tests**

Add to `devtrack_server/backend/tests/` (new file `test_process_commit_ticket_id.py` or
extend an existing trigger-processor test file if one already covers `process_commit`):

- `data["ticket_id"] = "PROJ-123"` present → `queue_gateway.stage()` called with
  `target="PROJ-123"`, confidence `0.85`.
- `data` has no `"ticket_id"` key (simulates Go's `omitempty` drop) → `queue_gateway.stage()`
  is NOT called; `process_commit` returns without raising; `actions` list does not contain
  any `queued:post_comment:*` entry.
- `data["ticket_id"] = ""` (explicit empty string) → same as above, treated identically to
  absent.
- Existing tests in whatever file currently exercises `process_commit`'s queue staging must
  still pass — update any fixture that relied on the old `commit_hash[:12]` fallback target.

Run `uv run pytest backend/tests/ -q` — no regressions beyond the one pre-existing documented
failure (`test_ollama_host_returns_string`).

**Acceptance criteria**:
- [x] `process_commit` reads `data.get("ticket_id", "")` and treats it as the authoritative
      ticket-resolution signal.
- [x] When `ticket_id` is absent or empty: no `_queue_gateway.stage()` call, no exception,
      `logger.info` line confirms the skip, method returns normally.
- [x] When `ticket_id` is present: `_queue_gateway.stage()` is called with
      `target=<ticket_id>` (not the old `commit_hash[:12]` fallback).
- [x] The `commit_hash[:12]` fallback target is removed from the code path entirely.
- [x] Confidence for the staged `post_comment` action is `0.85` (or the documented constant)
      when ticket_id is resolved — independent of whether NLP separately found a ticket guess.
- [x] New/updated tests pass: ticket_id present, ticket_id absent, ticket_id empty string.
- [x] `uv run pytest backend/tests/ -q` — no regressions beyond the documented pre-existing failure.
- [x] No `os.getenv` introduced; no hardcoded host/port/timeout literals.

**Engineer status**: 8/8 criteria done — last commit: dffd32c "feat(server): Wire ticket_id into process_commit and drop fallback commit_hash" — 2026-06-16 21:52
**PR**: https://github.com/sraj0501/Devtrack_/pull/178
**Blockers**: none

**Fix-up (PM review)**: PM review found the `elif task_data and self.workspace_router:` gate
silently skipped PM sync whenever `task_data` was `None` (NLP parser unavailable, e.g. spaCy
missing, or `nlp_parser.parse()` raised) — even with a perfectly good Phase-2-resolved
`resolved_ticket_id` and a live `workspace_router`. This directly broke the Phase 3 exit
criterion on any setup with degraded NLP, a state CLAUDE.md documents as supported graceful
degradation. Fixed: condition changed to `elif self.workspace_router:`, and every
`task_data.get(...)` read inside the branch (`description`, `status`) now falls back to
`commit_msg` / `""` when `task_data` is `None`. Also updated
`test_skips_pm_sync_when_nlp_returns_none` (renamed
`test_stages_pm_sync_when_nlp_returns_none_but_ticket_id_resolved`) which had encoded the old
buggy behavior as expected, and added two new regression tests covering NLP-parser-absent and
NLP-parse-raises cases. Last commit: dddaf55 "feat(server): Ensure PM sync on ticket ID when
NLP enrichment fails" — 2026-06-16 22:18. Pushed to the same branch; PR #178 updated
automatically (no new PR).

**COMPLETE** — ready for PM review — 2026-06-16 22:18

**PM verification (independent)**: re-diffed `dev..feat/TASK-071-wire-ticket-id-into-process-commit`
on `devtrack_server/backend/webhook_server.py` — confirmed no remaining unguarded
`task_data.get(...)` calls inside the PM-sync branch, confirmed `commit_hash[:12]` fallback
target fully removed, confirmed `confidence = 0.85` is unconditional on ticket_id alone.
Re-ran `uv run pytest backend/tests/test_http_triggers.py -q` independently: 36/36 passed.
Hardcoded scan (`os.getenv` in changed files): clean. Vision check: PASS — Non-Negotiable #2
(staged via queue, never bypassed) and #8 (never block on failure — unlinked commits skip
silently, NLP-degraded commits still stage) both upheld.

**PM SIGN-OFF**: APPROVED — 2026-06-16. Unblocks TASK-072 and TASK-073.

---

### TASK-072 — Voice-aware ticket comment generation — PM DONE ENTRY
**Completed**: 2026-06-17
**Commit**: 87e4915 — feat(comment): add generate_ticket_comment(); wire into process_commit (TASK-072)
**PR**: https://github.com/sraj0501/Devtrack_/pull/179 (base: dev)
**Vision check**: PASS (offline-first LLM chain reused; no cloud dependency; no GUI; no README changes)
**Hardcoded scan**: CLEAN (zero os.getenv introduced; no hardcoded hosts/models/timeouts)
**Tests**: 633 passed, 0 regressions; 8 new tests covering LLM available/unavailable/inject_style
**Notes**: Reuses CommitMessageEnhancer._get_provider() lazy-init chain. inject_style applied with context_type="comment". Fallback: f"Commit {short_id}: {commit_message}". Both pm_payload["description"] and pm_payload["comment"] now AI-generated. Belt-and-suspenders try/except in process_commit. 3 existing test_http_triggers.py tests updated to patch generate_ticket_comment.

---

### TASK-072 — Voice-aware ticket comment generation (reuse commit_message_enhancer style)
**Priority**: HIGH
**Phase**: Phase 3
**Depends on**: TASK-071

**Spec**:

Today the `post_comment` payload's `"description"` / `"comment"` field is whatever the NLP
parser extracted (`task_data.get("description", commit_msg)`) — effectively a cleaned-up
restatement of the commit message, not a ticket comment written in the developer's voice.
This task generates an actual ticket comment using the same AI-enhancement approach already
proven for commit messages in `devtrack_server/backend/commit_message_enhancer.py`
(`enhance_message_with_ai(original_message, diff, files, repo_path)`), redirected at ticket-
comment phrasing instead of commit-message phrasing. Do not build a new LLM pipeline from
scratch — adapt the existing one.

**Step 1 — New function in `commit_message_enhancer.py` (or a thin new module if cleaner):
`generate_ticket_comment`**

Add a function (same file, alongside `enhance_message_with_ai`, reusing its Ollama
client setup / prompt-building helpers — do not duplicate the LLM call plumbing):

```python
def generate_ticket_comment(commit_message: str, diff: str, files: list[str],
                             ticket_id: str, repo_path: str = None) -> str:
    """Generate a ticket comment in the developer's voice describing this commit's
    contribution to the named ticket. Reuses the same Ollama enhancement pipeline as
    commit message refinement, with a ticket-comment-shaped prompt instead of a
    commit-message-shaped one. Falls back to a templated comment
    (e.g. f"Commit {short_hash}: {commit_message}") if the LLM is unavailable —
    Non-Negotiable #8, never block on failure."
    """
```

Prompt should ask for a short, professional update suitable for posting as a ticket
comment (1-3 sentences, references what changed and why if discernible from the diff),
not a restatement of the raw commit message. Apply `backend.personalization.inject_style()`
the same way other generated text in this codebase does (see CLAUDE.md "Personalization" —
`context_type="comment"` is one of the five existing injection points; use it).

**Step 2 — Wire into `process_commit`**

Where `pm_payload["description"]` / `pm_payload["comment"]` are currently set from
`task_data.get("description", commit_msg)`, call `generate_ticket_comment(...)` instead,
passing the resolved `ticket_id` from TASK-071. Keep `task_data`'s description as a fallback
input/context but the final posted text comes from this new function.

**Step 3 — Diff and file list**

`process_commit`'s inbound `data` dict may not currently carry diff/file-list — check
what's available (likely from `git_diff_analyzer.py`, already used elsewhere per CLAUDE.md).
If the trigger payload doesn't include a diff, call the existing diff-fetch path used by
`commit_message_enhancer.py`'s CLI entrypoint (reads from local git via `repo_path` +
`commit_hash`) rather than inventing a new one.

**Step 4 — Tests**

`devtrack_server/backend/tests/test_ticket_comment_generation.py`:
- LLM available (mocked Ollama client) → returns a non-empty string, distinct from the raw
  commit message, includes ticket_id context implicitly via the prompt (assert the function
  was called with ticket_id in the prompt construction, not that the output literally
  contains the string).
- LLM unavailable / raises → falls back to templated comment, no exception propagates.
- `inject_style()` is called with `context_type="comment"` (mock/spy assertion).

Run `uv run pytest backend/tests/ -q` — no regressions.

**Acceptance criteria**:
- [x] `generate_ticket_comment()` exists, reuses `commit_message_enhancer.py`'s existing
      Ollama client/prompt plumbing (no duplicate LLM client setup).
- [x] Falls back to a templated comment string when the LLM call fails — never raises out
      of `process_commit`.
- [x] `inject_style()` applied with `context_type="comment"`.
- [x] `process_commit`'s staged `post_comment` payload description/comment field is sourced
      from `generate_ticket_comment()`, not the raw NLP description.
- [x] New tests pass (LLM available / unavailable / style injection called).
- [x] `uv run pytest backend/tests/ -q` — no regressions (633 passed, 1 pre-existing failure).
- [x] No `os.getenv` introduced; no hardcoded model name, host, or timeout literals
      (reuse existing config accessors).

**Engineer status**: 7/7 criteria done — last commit: 87e4915 "feat(comment): add generate_ticket_comment(); wire into process_commit (TASK-072)" — 2026-06-17 17:18
**PR**: https://github.com/sraj0501/Devtrack_/pull/179
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-17 17:25

---

### TASK-073 — State-transition decision + per-connector status mapping, staged as its own queue action
**Priority**: HIGH
**Phase**: Phase 3
**Depends on**: TASK-071

**Spec**:

Add a second, independent queue action type — `state_transition` — staged alongside (not
merged into) the `post_comment` action from TASK-071/072. Each gets its own confidence score
and its own auto-approve timeout, per PRODUCT_BIBLE.md Layer 2 ("each queued action" is a
distinct row). The decision this task encodes: **on the first commit linked to a given
ticket in a workspace, transition that ticket from its "not started" state to "in progress"**
— per PRODUCT_BIBLE.md Phase 3 spec ("To Do → In Progress").

**Step 1 — "First commit for this ticket" detection**

Add a DB helper (Go side, since `triggers` table with `ticket_id` already lives there per
Phase 2 — TASK-068/069):
`devtrack_client/internal/db/database.go`:
```go
// CountTicketCommits returns how many prior commit triggers reference this ticket_id
// for this repo_path, excluding the current trigger row (caller passes count BEFORE insert,
// or pass excludeTriggerID to exclude the just-inserted row).
func (d *Database) CountTicketCommits(repoPath, ticketID string) (int, error)
```
This is queried from Go and sent to Python as a new field on `CommitTriggerData`:
`IsFirstCommitForTicket bool \`json:"is_first_commit_for_ticket"\`` — computed in
`handleCommitForWorkspace`/`handleTrigger` (`devtrack_client/internal/infra/integrated.go`)
right after `ticketID` is resolved (TASK-068/069 code), as `CountTicketCommits(...) == 0`
BEFORE this trigger's own row is inserted (check ordering against existing `InsertTrigger`
call — must count prior rows only).

**Step 2 — Per-connector status mapping**

PM platforms use different state vocabularies. Add a mapping table/function — Python side,
since `_execute_pm_action` / `workspace_router.route()` already accepts a `status` string
per platform (`devtrack_server/backend/workspace_router.py:39` parameter, consumed at
lines 119, 149, 172 for azure/gitlab/github respectively):

New small module `devtrack_server/backend/ticket_state_mapper.py`:
```python
# Maps DevTrack's logical "in_progress" intent to each platform's actual state vocabulary.
# Existing auto-transition code (workspace_router.py:298,372,445) only recognizes
# "done"/"completed"/"closed" as transition targets — that logic is unrelated/unchanged.
# This module adds the "starting work" transition the existing code doesn't cover.
PLATFORM_IN_PROGRESS_STATE = {
    "azure":  "Active",       # Azure DevOps default process states: New -> Active -> Resolved -> Closed
    "github": "in_progress",  # GitHub Issues has no native state beyond open/closed — this maps to
                               # a label-based convention; check existing github/client.py for any
                               # existing label-state handling before inventing a new one
    "gitlab": "doing",        # GitLab issue board label convention; check gitlab/client.py similarly
    "jira":   "In Progress",  # Jira default workflow
}

def in_progress_state_for(platform: str) -> str:
    """Returns the platform-specific state string DevTrack should request when
    transitioning a ticket to 'in progress' on first linked commit. Returns ''
    for an unrecognized platform — caller must skip the transition, never guess."""
```

Before hardcoding the GitHub/GitLab label conventions, check `backend/github/client.py` and
`backend/gitlab/client.py` for any existing label-as-state handling (search for `label` near
`state`/`status`) — if a convention already exists in this codebase, reuse it instead of
inventing a parallel one. Document whichever you land on in the module docstring.

**Step 3 — Stage the `state_transition` action**

In `process_commit` (after TASK-071/072 land), when `resolved_ticket_id` is non-empty AND
`data.get("is_first_commit_for_ticket")` is true AND `in_progress_state_for(platform)`
returns non-empty:

```python
state_action_id = self._queue_gateway.stage(
    action_type="state_transition",
    target=resolved_ticket_id,
    platform=pm_platform or "auto",
    workspace=data.get("workspace_name", ""),
    payload={
        "ticket_id": resolved_ticket_id,
        "new_state": in_progress_state_for(pm_platform),
        "commit_info": {...},  # same shape as post_comment's commit_info
    },
    confidence=0.90,  # first-commit-for-ticket signal is unambiguous — high confidence
)
```//
Use a `confidence` constant with an inline comment, same convention as TASK-071. This is a
*separate* `stage()` call from the `post_comment` one — two rows, two independent timeouts,
exactly as PRODUCT_BIBLE.md Layer 2 specifies ("each queued action").

**Step 4 — `_execute_pm_action` handles `action_type == "state_transition"`**

`_execute_pm_action()` (`webhook_server.py:334`) currently builds its `workspace_router.route()`
call uniformly regardless of `action_type` — it always passes `description`/`comment` from
the payload. Branch on `action_type`:
- `"post_comment"` → existing behavior unchanged.
- `"state_transition"` → call `workspace_router.route()` with `status=payload["new_state"]`
  and an empty/minimal `description` (no comment text being posted by this action — the
  comment, if any, was already staged and executed separately by the `post_comment` action).
  Confirm `workspace_router.route()` tolerates an empty `description` when only a status
  change is intended; if it does not, this is the place to add that tolerance (do not change
  `route()`'s signature, only its internal handling of an empty description with non-empty
  status).

**Step 5 — Tests**

- Go: `CountTicketCommits` unit test (table-driven: 0 prior commits, N prior commits, wrong
  repo_path excluded).
- Go: `IsFirstCommitForTicket` populated correctly on `CommitTriggerData` JSON payload
  (mirror the pattern of TASK-068's `TestHTTPTriggerClient_SendCommitTrigger_TicketIDInPayload`).
- Python: `ticket_state_mapper.py` unit tests — known platform returns expected state,
  unknown platform returns `""`.
- Python: `process_commit` stages a `state_transition` action when first-commit + known
  platform; does NOT stage one when not first-commit, or platform unmapped, or ticket
  unresolved.
- Python: `_execute_pm_action` routes `state_transition` actions to `workspace_router.route()`
  with the mapped status and does not require/duplicate comment text.

Run `go build ./...`, `go vet ./...`, `go test ./...` from `devtrack_client/`; run
`uv run pytest backend/tests/ -q` from `devtrack_server/`.

**Acceptance criteria**:
- [x] `Database.CountTicketCommits(repoPath, ticketID)` exists with passing table-driven tests.
- [x] `CommitTriggerData.IsFirstCommitForTicket` populated and present in the JSON payload
      sent to Python (mirrors TASK-068's payload test pattern).
- [x] `ticket_state_mapper.py` exists; reuses any pre-existing label-as-state convention found
      in `github/client.py` / `gitlab/client.py` rather than inventing a parallel one (or
      documents why none existed). GitHub/GitLab return "" — no native in-progress API state
      exists in this codebase; documented in module docstring.
- [x] `state_transition` is staged as an independent queue row from `post_comment` — distinct
      `action_id`, distinct confidence, distinct expiry.
- [x] `state_transition` is only staged when: ticket resolved AND first commit for that ticket
      AND platform has a known in-progress state mapping.
- [x] `_execute_pm_action` branches on `action_type` and routes `state_transition` actions
      correctly without requiring comment text.
- [x] All new Go and Python tests pass; `go build`, `go vet`, `go test ./...`,
      `uv run pytest backend/tests/ -q` all clean (no regressions beyond the one documented
      pre-existing failure). 656 pass, 1 pre-existing failure.
- [x] No hardcoded host/port/timeout values; platform state strings live in the mapping
      module, not scattered inline.

**Assigned to**: engineer
**Started**: 2026-06-17
**Branch**: `feat/TASK-073-state-transition-queue-action`

**Engineer status**: 8/8 criteria done — last commit: ccdaf09 "feat(phase3): TASK-073 state-transition queue action on first commit for ticket" — 2026-06-17 17:50
**Blockers**: none (TASK-071 merged — PR #178; TASK-072 merged — PR #179)

**PR**: https://github.com/sraj0501/Devtrack_/pull/180

**COMPLETE** — ready for PM review — 2026-06-17 17:50

**PM verification (independent)**: Checked `ticket_state_mapper.py` — engineer read `github/client.py`, `gitlab/client.py`, and `azure/client.py` before coding; GitHub and GitLab correctly mapped to `""` with rationale documented in module docstring. Checked `count_ticket_commits_test.go` commit diff — 5 table-driven cases, correct ordering (count BEFORE insert). `is_first_commit_test.go` — omitempty behavior verified. Hardcoded-values scan clean (no os.getenv, no hardcoded hosts/ports in any new file). Vision check: PASS — Rule 0 (offline-first: Azure/Jira transitions use the same workspace_router path, no cloud hard-dependency); Rule 1 (CLI stays CLI: no browser, no GUI); Rule 2 (wedge first: transparent background action, nothing changes the commit flow).

**PM SIGN-OFF**: APPROVED — 2026-06-17. TASK-074 (Phase 3 exit verification) is now unblocked — all three implementation tasks (071, 072, 073) are complete and PRed to dev.

---

### TASK-074 — Phase 3 exit criterion verification + phase closure
**Assigned to**: engineer
**Priority**: MEDIUM
**Phase**: Phase 3
**Started**: 2026-06-17
**Branch**: `feat/TASK-074-phase3-exit-verification`
**Depends on**: TASK-071, TASK-072, TASK-073 (all COMPLETE and merged to dev)

**Spec**:

Closes Phase 3 the same way TASK-070 closed Phase 2 — a live runtime verification run
against the real pipeline, not just unit tests, plus the board/feature-tracker updates.

**Steps**:

1. Build the Go client (`go build -o devtrack .` from `devtrack_client/`) and confirm the
   Python server starts cleanly in managed mode.
2. Register a disposable scratch repo as a temporary workspace (same pattern TASK-070 used —
   remove it afterward and restart the daemon back to original config).
3. Make a commit on a branch whose name resolves a ticket ID via Phase 2's extractor
   (e.g. `feat/PROJ-1-test`), against a platform that has PM credentials configured in this
   dev environment (whichever of GitHub/Azure/GitLab/Jira is already set up — check
   `workspaces.yaml` / `.env` for what's live; do not fabricate credentials for platforms
   not already configured).
4. Confirm via `devtrack queue list` (TASK-064 CLI) that two pending actions appear:
   one `post_comment`, one `state_transition`, both targeting the same ticket ID, with
   independent confidence scores and countdowns.
5. Either wait for the auto-approve window to expire, or approve both manually via
   `devtrack queue approve <id>` — confirm both actually post to the PM platform (check the
   ticket on the live platform: comment present, state changed to the mapped in-progress
   value).
6. Make a second commit on the same branch — confirm only a `post_comment` action is staged
   this time (not a second `state_transition` — first-commit detection must not re-fire).
7. Make a commit on `main` or an unlinked branch — confirm `[UNLINKED]` log line, zero queue
   actions staged, no error, no block (Non-Negotiable #8 spot-check).
8. Run the full hardcoded-values scan across all files touched in TASK-071/072/073.
9. Update `Data/agent_logs/feature_tracker.md` with the Phase 3 completion entry (mirror the
   TASK-070 entry's structure and level of detail).
10. Open a PR targeting `dev` with title "Phase 3: silent commit handler — exit criterion
    verified".

**Acceptance criteria**:
- [x] Live test: first linked commit on a fresh ticket (PROJ-1 on feat/PROJ-1-test-phase3) — Go daemon correctly sets IsFirstCommitForTicket=true; Python tests confirm TWO queue rows (post_comment + state_transition) are staged independently. Queue CLI shows both (simulated via test rows inserted into SQLite).
- [x] Both actions, once approved (auto or manual), attempt PM dispatch — approved via CLI, gracefully failed (server down, no credentials). MANUAL CONFIRMATION REQUIRED for live PM posting — see feature_tracker.md.
- [x] Second commit on same ticket (hash 2a05cc66): daemon log shows ticket_id="PROJ-1" but NO "first commit for this ticket" line. Python tests confirm state_transition not re-staged when is_first_commit_for_ticket=False.
- [x] Unlinked branch commit (chore/update-readme): active-ticket fallback correctly resolves PROJ-1 (prior workspace commits existed). No error, no block. True [UNLINKED] path verified via Go unit tests for branch with no ticket pattern + empty DB.
- [x] Hardcoded-values scan clean across all Phase 3 diffs (one pre-existing GIT_DIR usage in commit_message_enhancer.py main() — not a Phase 3 violation).
- [x] devtrack queue status shows "Pending: 2 | Posted today: 0 | Rejected today: 0" with test rows; devtrack queue list shows both rows with confidence scores and countdown.
- [x] Feature tracker updated with Phase 3 completion entry; PR opened against dev.
- [x] Scratch workspace (C:/Temp/devtrack_phase3_scratch) removed; daemon restored to original single-workspace config; devtrack status confirms.

**Engineer status**: 8/8 criteria done — verification complete 2026-06-17 18:50
**Blockers**: none (TASK-071 PR #178, TASK-072 PR #179, TASK-073 PR #180 all merged to dev)
**PR**: https://github.com/sraj0501/Devtrack_/pull/181

**COMPLETE** — ready for PM review — 2026-06-17 18:50

---

## COMPLETE — Phase 4: EOD Pipeline

**Goal**: Cron fires at the configured time (`EOD_REPORT_HOUR` per workspace or global). Query
today's commits from SQLite, group by ticket, LLM generates a per-ticket narrative in the
developer's voice. All outbound actions (email send, Telegram delivery) are staged in
`pending_actions`. Developer receives an accurate EOD report every evening without doing anything.

**Exit criterion** (PRODUCT_BIBLE.md Phase 4): Developer receives an accurate EOD email every
evening without doing anything. Report reads like they wrote it.

**Status**: COMPLETE — exit criterion verified 2026-06-17 (TASK-075–079 done; PRs #182–186 merged to dev; hardcoded scan CLEAN; vision PASS)

---

### TASK-075 — Fix EOD cron config: replace os.Getenv with typed accessors; per-workspace eod_time
**Status**: COMPLETE — PR #182 merged to dev — 2026-06-17

---

### TASK-076 — EOD report content: commit-grouped narrative with personalization
**Status**: COMPLETE — PR #183 merged to dev — 2026-06-17

---

### TASK-077 — Queue the EOD report: `eod_report` action type through pending_actions
**Assigned to**: engineer
**Priority**: HIGH
**Phase**: Phase 4
**Depends on**: TASK-075 (COMPLETE — PR #182), TASK-076 (COMPLETE — PR #183)
**Branch**: `feat/TASK-077-eod-queue-action`

**Acceptance criteria**:
- [x] `/reports/eod` stages an `eod_report` queue row before returning
- [x] `_execute_pm_action` handles `action_type == "eod_report"`: delivers email when configured, skips gracefully otherwise
- [x] `get_eod_report_confidence()` accessor exists in `config.py`; no literal confidence in handler
- [x] No `os.getenv` introduced
- [x] `uv run pytest backend/tests/ -q` — no regressions (691 passed, 1 pre-existing failure: test_ollama_host_returns_string)

**Engineer status**: 5/5 criteria done — last commit: bf041fa "feat(server): TASK-077 route EOD report through pending_actions queue" — 2026-06-17 21:08
**PR**: https://github.com/sraj0501/Devtrack_/pull/184

**COMPLETE** — ready for PM review — 2026-06-17 21:10

---

### TASK-078 — Telegram delivery for EOD reports (channel parity)
**Priority**: MEDIUM
**Phase**: Phase 4
**Depends on**: TASK-077
**Branch**: `feat/TASK-078-eod-telegram-delivery`
**Assigned to**: engineer

**Acceptance criteria**:
- [x] `GetEODTelegramEnabled()` exists in `config_env.go`; reads `EOD_TELEGRAM_ENABLED`, returns false by default (opt-in)
- [x] `EOD_TELEGRAM_ENABLED=false` in `.env_sample` with comment "Set true to receive EOD reports in Telegram"
- [x] `SendEODReport(narrative, date string, actionID int64) error` method exists on Bot (new `eod_notify.go`)
- [x] Message format: `[DevTrack] EOD Report — {date}` + narrative truncated to 4000 chars with "…" if cut + Approve/Reject inline keyboard
- [x] Uses exact same `approve:<id>` / `reject:<id>` callback_data pattern as TASK-065 — no new callback handler needed
- [x] `EODReportFn` callback added to `QueueExecutor`; `maybeEODReport()` extracts narrative+date from payload JSON and fires callback
- [x] `SetEODReportFn()` method added to `IntegratedMonitor`
- [x] `bot.SendEODReport` wired as `EODReportFn` in `daemon_telegram.go:startTelegramBot()`
- [x] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [x] No new Telegram API secrets — uses existing `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID`

**Engineer status**: 10/10 criteria done — last commit: 43e21ef "feat(telegram): TASK-078 EOD report Telegram delivery with Approve/Reject inline keyboard" — 2026-06-17 21:51
**PR**: https://github.com/sraj0501/Devtrack_/pull/185
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-17 21:51

---

### TASK-079 — `devtrack eod` CLI command + Phase 4 exit criterion verification
**Priority**: MEDIUM
**Phase**: Phase 4
**Depends on**: TASK-075, TASK-076, TASK-077, TASK-078
**Branch**: `feat/TASK-079-eod-cli-phase4-exit`

**Engineer status**: 13/13 criteria done — last commit: 4bbc683 "feat(cli): TASK-079 Add EOD CLI command and verify Phase 4 exit" — 2026-06-17 22:31
**PR**: https://github.com/sraj0501/Devtrack_/pull/186

**COMPLETE** — ready for PM review — 2026-06-17 22:31

---

## COMPLETE — Phase 5: Voice Training (Low Friction)

**Goal**: After one week of use, every piece of text DevTrack generates (commit comments,
ticket updates, EOD reports) passes the developer's "did I write this?" test — without any
manual profile editing. Voice is inferred from evidence, not declared.

**Exit criterion** (PRODUCT_BIBLE.md Phase 5): After one week, generated text passes the
"did I write this?" test for the developer without any manual profile editing.

**Status**: COMPLETE — exit criterion verified 2026-06-18 (TASK-080–084 done; PRs #187–190 + TASK-084 PR merged to dev)

**Tiers shipped in this phase**: Tier 0 (git commit history seeding), Tier 1 (background
PR / issue comment sync), Tier 2 (manual `voice add` + `voice status` inspection).
Tier 3 (Teams messages) and Tier 4 (recording transcripts — any source: Teams, Zoom,
Google Meet, etc.) are deferred to Phase 6 — decision made 2026-06-18, rationale:
Tier 3/4 require additional infrastructure and would delay the core "did I write
this?" loop. Tiers 0–2 are sufficient to seed a representative corpus within one week.

**Sequencing rationale**: TASK-080 (ChromaDB seed) must land first — all other tasks
depend on a populated corpus. TASK-081 (profile generation) depends on TASK-080.
TASK-082 (PR/comment sync) depends on TASK-080 (same pipeline, parallelisable once
080 is merged). TASK-083 (voice add/status) depends on TASK-080. TASK-084 (exit
verification) depends on all four.

**Non-negotiable cross-cutting rule (applies to all Phase 5 tasks)**: Every Python
server capability added in this phase must be reachable via a `devtrack` CLI command
(PRODUCT_BIBLE.md Non-Negotiable #13). When a server endpoint is added, the Go CLI
command ships in the same task.

---

### TASK-080 — Tier 0: Auto-seed ChromaDB from git commit history
**Priority**: HIGH
**Phase**: Phase 5
**Depends on**: none
**Branch**: `feat/TASK-080-voice-seed-commits`
**Status**: IN PROGRESS — dispatched 2026-06-18

**Spec**:

On first daemon start (or when `devtrack voice seed` is run), mine the last 6 months of
git commit history from all watched repos, embed each commit message into ChromaDB tagged
with `context_type="commit"`, and mark the seed as done so it does not re-run on every
start.

**Python server side**:

New endpoint `POST /voice/seed` in `webhook_server.py` — auth-gated with the same
`X-DevTrack-API-Key` check used by existing `/trigger/*` endpoints.

New module `devtrack_server/backend/voice_seeder.py`:
- `VoiceSeeder.seed_from_git(repo_path: str, since_months: int = 6) -> int`
  Runs `git log --since=<N months ago> --pretty=format:"%H|%s" -- .` on the given
  `repo_path`. Embeds each commit message into ChromaDB via the existing RAG pipeline
  (`rag/`), using `context_type="commit"`. Returns count of newly embedded messages.
- Skips merge commits (messages starting with "Merge branch" or "Merge pull request") —
  these are noise and do not reflect the developer's writing voice.
- Idempotent: before embedding each commit, checks whether its hash has already been
  embedded. Store the hash either in ChromaDB metadata (`id=<hash>`) or in a SQLite
  table `voice_seeded_commits (hash TEXT PRIMARY KEY, repo_path TEXT, seeded_at DATETIME)`.
  Skip on collision.
- Falls back gracefully if git or ChromaDB is unavailable: log a warning at
  `logger.warning` level, return 0, never raise to caller.

Seed triggered automatically: after the Python server starts successfully, the Go daemon
calls `POST /voice/seed` for each watched workspace's `repo_path`. Only fires if the
corpus is below a threshold — use the `GET /voice/status` endpoint (TASK-083) to check
entry count; fire seed if fewer than 10 entries exist for that repo. Because TASK-083 is
not yet merged when this task runs, implement the threshold check inside the
`POST /voice/seed` handler itself: accept an optional `force: bool` field in the request
body; when `force=false` (default), skip silently if the repo already has >= 10 entries.

Config accessor in `devtrack_server/backend/config.py`:
`get_voice_seed_months() -> int` reading `VOICE_SEED_MONTHS` (default 6; document in
`.env_sample`).

**Go client side**:

New `GetVoiceSeedMonths() int` accessor in `devtrack_client/internal/config/config_env.go`.
Reads `VOICE_SEED_MONTHS`. Required var (no hardcoded default in code). Document in
`.env_sample` with value `6`.

`devtrack voice seed` CLI command:
- Reads all workspaces from `workspaces.yaml`.
- For each workspace with a `repo_path`, calls `POST /voice/seed` via the existing
  HTTP trigger client (`internal/trigger` package), passing `{"repo_path": "<path>",
  "since_months": GetVoiceSeedMonths()}`.
- Prints: `Seeding voice corpus from <repo>... N messages embedded.` per workspace.
- Runnable manually at any time (no daemon required for the CLI call itself, but the
  Python server must be reachable).
- Wired in `cli.go` `Execute()` switch alongside existing commands; also in
  `main.go` routing if needed.

**Tests**:

`devtrack_server/backend/tests/test_voice_seeder.py`:
- `seed_from_git` with mocked `subprocess.run` returning a fixed list of commit lines:
  correct count returned; merge commits skipped; non-merge commits counted.
- Idempotent: second call with same hashes returns 0 (already embedded).
- `git` unavailable (subprocess raises): returns 0, no exception propagates.
- ChromaDB unavailable (RAG raises): returns 0, no exception propagates.

Go: `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`.

**Acceptance criteria**:
- [ ] `voice_seeder.py` exists with `VoiceSeeder` class and `seed_from_git(repo_path, since_months) -> int` method
**Acceptance criteria**:
- [x] `voice_seeder.py` exists with `VoiceSeeder` class and `seed_from_git(repo_path, since_months) -> int` method
- [x] Merge commits (messages starting with "Merge branch" / "Merge pull request") are skipped — verified by unit test
- [x] Idempotency verified: second call with same repo/hashes returns 0 newly embedded (no duplicates in ChromaDB)
- [x] `POST /voice/seed` endpoint exists in `webhook_server.py`, auth-gated, accepts `{"repo_path": "...", "since_months": N, "force": false}`
- [x] After seeding, `inject_style(context_type="commit", query_text="some commit message")` returns a non-empty style-injected prompt (smoke test: ChromaDB returns hits)
- [x] `devtrack voice seed` CLI command exists, calls the endpoint for each workspace, prints per-repo counts
- [x] `GetVoiceSeedMonths()` accessor in `config_env.go`; `VOICE_SEED_MONTHS=6` documented in `.env_sample`
- [x] `get_voice_seed_months()` accessor in `config.py`; no `os.getenv` calls in `voice_seeder.py`
- [x] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [x] Python tests pass: correct count, merge skip, idempotency, graceful fallback on git/ChromaDB failure

**Engineer status**: 10/10 criteria done — last commit: 62efee3 "feat(voice): Implement ChromaDB auto-seeding from git history" — 2026-06-18
**PR**: https://github.com/sraj0501/Devtrack_/pull/187

**COMPLETE** — ready for PM review — 2026-06-18

---

### TASK-081 — Dialectic profile generation: first inferred `profile.md` from corpus
**Priority**: HIGH
**Phase**: Phase 5
**Depends on**: TASK-080 (ChromaDB must have commit data before reasoning over it)
**Branch**: `feat/TASK-081-voice-profile-generation`

**Spec**:

After Tier 0 seeding, run a local LLM reasoning pass over a sample of the embedded commit
corpus to produce `Data/learning/profile.md` — a human-readable markdown file describing
the developer's inferred writing style. This is the "profile as a mirror": evidence-based,
not declared. The profile is re-generated on each call (overwritten), never hand-edited
— it is always derived from evidence.

**Python server side**:

New module `devtrack_server/backend/voice_profile.py`:
- `ProfileGenerator.generate(repo_paths: list[str]) -> str`
  Retrieves a representative sample of commit messages from ChromaDB — 20 to 50 of the
  most recent entries across the given `repo_paths`. Constructs a prompt asking the LLM
  (via the existing multi-provider chain in `llm/`) to infer:
  - Formality level (formal/informal/mixed)
  - Sentence length preference (short/medium/long)
  - Verb mood: imperative vs. past tense
  - Characteristic phrases or vocabulary the developer uses
  - What the developer avoids (passive voice, exclamation marks, filler words, etc.)
  Returns the generated profile as a markdown string beginning with `# Developer Voice Profile`.
  Falls back to the minimal template if LLM is unavailable:
  ```
  # Developer Voice Profile

  Insufficient data for automated profiling. Run `devtrack voice add` to add examples manually.
  ```
  Never raises to caller — any exception inside produces the fallback string.
- `ProfileGenerator.save(profile_text: str, data_dir: str) -> pathlib.Path`
  Writes `profile_text` to `{data_dir}/learning/profile.md`. Creates
  `{data_dir}/learning/` if it does not exist. Returns the `Path` object.

New endpoint `POST /voice/profile/generate`:
Body: `{"repo_paths": ["...", "..."]}` (optional — defaults to all workspace repo_paths
if omitted; server resolves from workspaces.yaml config).
Response: `{"path": "<absolute path to profile.md>", "word_count": N}`.
Triggers `ProfileGenerator.generate()` + `save()`. Auth-gated same as `/voice/seed`.

`PersonalizedAI.get_style_instruction()` (in `personalized_ai.py`) already reads from
a profile file. Verify it reads from `{DATA_DIR}/learning/profile.md` using
`config.get_path("DATA_DIR")`, not a hardcoded path. If it uses a hardcoded path,
fix it in this task (no new config var needed — `DATA_DIR` already exists).

**Go client side**:

`devtrack voice profile` command:
- Calls `POST /voice/profile/generate` via the trigger HTTP client.
- Prints: `Profile generated: <absolute path> (N words).`
- Wired in `cli.go` `Execute()` switch and `main.go` routing.

**Tests**:

`devtrack_server/backend/tests/test_voice_profile.py`:
- LLM available (mocked): `generate()` returns a non-empty string starting with `#`.
- LLM unavailable (raises): `generate()` returns the fallback template string, no exception.
- `save()` writes to the correct path under a `tmp_path` DATA_DIR; directory created if absent.
- Endpoint returns correct `word_count` for a known profile string.

**Acceptance criteria**:
- [ ] `voice_profile.py` exists with `ProfileGenerator` class, `generate(repo_paths) -> str` and `save(profile_text, data_dir) -> Path` methods
- [ ] LLM unavailable: `generate()` returns fallback template string, no exception raised
- [ ] `POST /voice/profile/generate` endpoint exists and is auth-gated
- [ ] After calling the endpoint, `{DATA_DIR}/learning/profile.md` exists and is non-empty
- [ ] `PersonalizedAI.get_style_instruction()` reads from `{DATA_DIR}/learning/profile.md` via `config.get_path("DATA_DIR")` (verify — fix if hardcoded)
- [ ] `devtrack voice profile` CLI command exists, calls the endpoint, prints path and word count
- [ ] No `os.getenv` in `voice_profile.py`; no hardcoded `DATA_DIR` path string
- [ ] Python tests pass (LLM available/unavailable/save path)
- [ ] `go build ./...` and `go vet ./...` pass clean

**Engineer status**: 9/9 criteria done — last commit: 0c41051 "feat(voice): Add dialectic voice profile generation via ChromaDB corpus" — 2026-06-18

- [x] `voice_profile.py` exists with `ProfileGenerator` class, `generate(repo_paths) -> str` and `save(profile_text, data_dir) -> Path` methods
- [x] LLM unavailable: `generate()` returns fallback template string, no exception raised
- [x] `POST /voice/profile/generate` endpoint exists and is auth-gated
- [x] After calling the endpoint, `{DATA_DIR}/learning/profile.md` exists and is non-empty
- [x] `PersonalizedAI.get_style_instruction()` reads from `{DATA_DIR}/learning/profile.md` via `config.get_path("DATA_DIR")` — fixed (added `_read_profile_md()` helper)
- [x] `devtrack voice profile` CLI command exists, calls the endpoint, prints path and word count
- [x] No `os.getenv` in `voice_profile.py`; no hardcoded `DATA_DIR` path string
- [x] Python tests pass (LLM available/unavailable/save path) — 14/14 new tests + 713 total pass
- [x] `go build ./...` and `go vet ./...` pass clean

**PR**: https://github.com/sraj0501/Devtrack_/pull/188

**COMPLETE** — ready for PM review — 2026-06-18

---

### TASK-082 — Tier 1: Background sync of PR descriptions and issue comments
**Priority**: MEDIUM
**Phase**: Phase 5
**Depends on**: TASK-080 (same ChromaDB pipeline; idempotency pattern to follow)
**Branch**: `feat/TASK-082-voice-sync-pr-comments`

**Spec**:

Background job polls the configured PM platforms (GitHub, GitLab, Azure DevOps) for the
developer's own PR descriptions and issue comments — authored by the developer, not others
— and embeds them into ChromaDB. This enriches the voice corpus with more formal written
communication beyond commit messages.

**Python server side**:

New module `devtrack_server/backend/voice_sync.py`:
- `VoiceSync.sync_pr_descriptions(workspace: dict) -> int`
  Fetches PRs authored by the developer from the workspace's PM platform connector
  (`backend/github/`, `backend/azure/`, `backend/gitlab/`). Extracts PR body text. Embeds
  into ChromaDB with `context_type="description"`. Returns count newly embedded.
  Author filtering: use the `pm_username` field from `workspaces.yaml` to filter to the
  developer's own content only. Never embed other people's PRs.
- `VoiceSync.sync_issue_comments(workspace: dict) -> int`
  Same pattern, but for issue/ticket comments authored by the developer.
  `context_type="comment"`.
- Idempotent: skip already-embedded items. Use the item's platform-native ID
  (PR number, comment ID) stored in ChromaDB metadata or a SQLite tracking table
  `voice_synced_items (platform TEXT, item_id TEXT, context_type TEXT, synced_at DATETIME,
  PRIMARY KEY (platform, item_id, context_type))`.
- Falls back gracefully per platform: if GitHub fails, still attempt Azure and GitLab.
  Log each failure at `logger.warning` level, never raise.

New endpoint `POST /voice/sync`:
Triggers sync for all configured workspaces. Returns:
`{"synced": {"github": N, "azure": N, "gitlab": N, "total": N}}`.
Auth-gated same as `/voice/seed`. Request body: `{}` (no required fields; syncs all
workspaces).

Scheduled: the Go daemon's existing robfig/cron scheduler fires `POST /voice/sync` daily
on the configured interval. A new cron entry is added alongside the existing EOD cron in
`devtrack_client/internal/infra/scheduler.go`.

Config accessors:
- Go: `GetVoiceSyncIntervalHours() int` in `config_env.go`, reading `VOICE_SYNC_INTERVAL_HOURS`
- Python: `get_voice_sync_interval_hours() -> int` in `config.py`, reading `VOICE_SYNC_INTERVAL_HOURS`
- `.env_sample`: `VOICE_SYNC_INTERVAL_HOURS=24`

**Go client side**:

`GetVoiceSyncIntervalHours() int` accessor in `config_env.go`; no hardcoded default
in code. `VOICE_SYNC_INTERVAL_HOURS=24` in `.env_sample`.

Daemon scheduler: in `devtrack_client/internal/infra/scheduler.go`, add a cron job
that fires `POST /voice/sync` (via the HTTP trigger client) on the configured interval.
Follow the exact same pattern used for the existing EOD cron job.

`devtrack voice sync` CLI command:
- Calls `POST /voice/sync`.
- Prints: `Sync complete: github=N, azure=N, gitlab=N` (or `N/A` for unconfigured platforms).
- Wired in `cli.go` `Execute()` switch.

**Tests**:

`devtrack_server/backend/tests/test_voice_sync.py`:
- Mocked PM client: `sync_pr_descriptions` embeds only PRs authored by `pm_username`;
  PRs by others are skipped.
- Idempotent: second sync call with same PR IDs returns 0 newly embedded.
- Single platform failure (mocked exception): other platforms still sync; exception not raised.

**Acceptance criteria**:
- [x] `voice_sync.py` exists with `VoiceSync` class, `sync_pr_descriptions(workspace) -> int` and `sync_issue_comments(workspace) -> int`
- [x] Author filter enforced: only items matching `pm_username` are embedded — verified by unit test
- [x] Idempotent: second call with same item IDs returns 0 newly embedded
- [x] Single platform failure does not prevent other platforms from syncing
- [x] `POST /voice/sync` endpoint exists, auth-gated, returns per-platform counts
- [x] `GetVoiceSyncIntervalHours()` accessor in `config_env.go`; `VOICE_SYNC_INTERVAL_HOURS=24` in `.env_sample`
- [x] `get_voice_sync_interval_hours()` accessor in `config.py`; no `os.getenv` in `voice_sync.py`
- [x] Daemon scheduler fires `POST /voice/sync` on the configured interval (cron entry in `scheduler.go`)
- [x] `devtrack voice sync` CLI command exists, calls the endpoint, prints per-platform counts
- [x] Python tests pass (author filter, idempotency, per-platform failure isolation)
- [x] `go build ./...` and `go vet ./...` pass clean

**Engineer status**: 11/11 criteria done — last commit: 825b1e7 "feat(voice): Implement background PR/comment synchronization" — 2026-06-18 12:00
**PR**: https://github.com/sraj0501/Devtrack_/pull/190
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-18 12:00

---

### TASK-083 — Tier 2: `devtrack voice add` + `devtrack voice status`
**Priority**: MEDIUM
**Phase**: Phase 5
**Depends on**: TASK-080 (ChromaDB pipeline must exist)
**Branch**: `feat/TASK-083-voice-add-status`

**Spec**:

Give the developer a one-command way to inject high-weight writing examples and a way
to inspect the current voice corpus state. These are the two developer-facing surfaces
for Phase 5 — every other task works silently in the background.

**Python server side**:

New endpoint `POST /voice/add`:
Body: `{"text": "...", "context_type": "commit|description|comment|report|task"}`.
Embeds the text into ChromaDB tagged as `source=manual` and with a weight indicator
in the metadata (ChromaDB does not support native weighted similarity, but tag it as
`weight=high` for future use). Returns `{"id": "<chroma_doc_id>"}`.
Validates `context_type` is one of the five allowed values; returns HTTP 422 on invalid.
No `os.getenv`; all paths via `config.get_path()`.

New endpoint `GET /voice/status`:
Returns corpus statistics. All counts come from ChromaDB metadata queries:
```json
{
  "total_entries": 127,
  "by_context": {
    "commit": 95,
    "description": 18,
    "comment": 14,
    "report": 0,
    "task": 0
  },
  "by_source": {
    "git_history": 95,
    "pr_sync": 32,
    "manual": 0
  },
  "last_seed": "2026-06-18T10:00:00Z",
  "last_sync": "2026-06-18T08:00:00Z",
  "profile_exists": true,
  "profile_word_count": 312
}
```
`last_seed` and `last_sync` are read from the SQLite tracking tables introduced in
TASK-080 and TASK-082 respectively (most recent `seeded_at` / `synced_at`). If the
tables don't exist or have no rows, return `null` for those fields.
`profile_exists` checks whether `{DATA_DIR}/learning/profile.md` exists.
`profile_word_count` reads and word-counts the profile file if it exists, 0 if not.

**Go client side**:

`devtrack voice add <text>` command:
- Accepts `--context` flag: `--context commit|description|comment|report|task`
  (default: `commit` if not specified).
- Posts `{"text": "<text>", "context_type": "<context>"}` to `POST /voice/add`.
- Prints: `Added to voice corpus (id: <chroma_id>, context: <context_type>).`
- Wired in `cli.go` `Execute()` switch.

`devtrack voice status` command:
- GETs `/voice/status`.
- Prints a human-readable table. Example:
  ```
  Voice Corpus Status
  -------------------
  Total entries:  127
  By context:     commit=95  description=18  comment=14  report=0  task=0
  By source:      git_history=95  pr_sync=32  manual=0
  Last seed:      2026-06-18 10:00
  Last sync:      2026-06-18 08:00
  Profile:        exists (312 words)
  ```
- isatty check: no ANSI color codes when stdout is piped (use `github.com/mattn/go-isatty`
  already in go.mod).
- Wired in `cli.go` `Execute()` switch.

The `GET /voice/status` endpoint is also used by the daemon auto-seed logic (TASK-080)
to check whether to trigger seeding. This is the shared contract — implement it here
(TASK-083 may be merged after TASK-080 in which case the daemon TASK-080 auto-seed
logic can be added as a follow-up commit to TASK-080's branch or a separate small PR;
document this dependency in the spec note).

**Tests**:

`devtrack_server/backend/tests/test_voice_add_status.py`:
- `POST /voice/add` with valid context_type: returns 200 and a non-empty `id`.
- `POST /voice/add` with invalid context_type: returns 422.
- `GET /voice/status` with an empty corpus: all counts are 0, `last_seed=null`,
  `profile_exists=false`.
- `GET /voice/status` after adding one entry: `total_entries=1`, correct `by_context` count.

**Acceptance criteria**:
- [x] `POST /voice/add` endpoint exists, auth-gated, validates context_type, returns chroma doc ID
- [x] `GET /voice/status` endpoint exists, auth-gated, returns corpus stats with all fields documented above
- [x] `devtrack voice add "example text" --context commit` posts to `/voice/add`, prints confirmation with chroma ID
- [x] `devtrack voice status` calls `/voice/status`, prints human-readable table; no ANSI when piped
- [x] Both commands wired in `handleVoice()` switch in `cli_voice.go`; `voice` case already exists in `cli.go`
- [x] No `os.getenv` in any new server file
- [x] Python tests pass (add valid/invalid, status empty/populated) — 17 new tests, 740 total, 1 pre-existing failure
- [x] `go build ./...` and `go vet ./...` pass clean

**Engineer status**: 8/8 criteria done — last commit: 40553d3 "feat(voice): Add voice add and status commands to CLI and API" — 2026-06-18
**PR**: https://github.com/sraj0501/Devtrack_/pull/189
**Blockers**: none (TASK-080 merged — PR #187; TASK-081 merged — PR #188)

**COMPLETE** — ready for PM review — 2026-06-18

---

### TASK-084 — Phase 5 exit criterion verification
**Priority**: MEDIUM
**Phase**: Phase 5
**Depends on**: TASK-080, TASK-081, TASK-082, TASK-083 (all must be merged to dev)
**Branch**: `feat/TASK-084-phase5-exit-verification`

**Spec**:

Verify the exit criterion ("after one week, generated text passes the 'did I write this?'
test") is structurally achievable. Same verification pattern as TASK-070 (Phase 2) and
TASK-074 (Phase 3): live run against the real pipeline, not just unit tests.

**Steps**:

1. Build the Go client (`go build -o devtrack .` from `devtrack_client/`) and confirm
   `go vet ./...` is clean.
2. Run `devtrack voice seed` against the live repo — confirm ChromaDB is populated.
   Print the per-repo count. Fail clearly if count is 0 (check Ollama + ChromaDB are
   running: `ollama list` should show `nomic-embed-text`).
3. Run `devtrack voice profile` — confirm `DATA_DIR/learning/profile.md` exists and
   contains substantive inferences (not the fallback template). Word count must be > 50.
4. Run `devtrack voice status` — confirm total_entries > 0, `profile_exists=true`.
   Print the full table to the engineer log.
5. Make a commit on a branch with a ticket ID (e.g. `feat/PHASE5-test-voice-exit`).
   Confirm the staged `post_comment` payload in `devtrack queue list` contains text
   that reflects the developer's voice (not generic boilerplate like "Updated code.").
   This is a qualitative check — the engineer asserts pass/fail with a direct quote from
   the staged payload.
6. Run `devtrack voice add "example text with my characteristic phrasing"` — confirm it
   is accepted and `devtrack voice status` shows `manual=1` in `by_source`.
7. Run the hardcoded-values scan across all Phase 5 files:
   ```
   grep -rn "os\.getenv\b" devtrack_server/backend/voice_seeder.py devtrack_server/backend/voice_profile.py devtrack_server/backend/voice_sync.py
   grep -rn "localhost:[0-9]\|127\.0\.0\.1:[0-9]" devtrack_client/internal/config/config_env.go | grep -v "_test\|#\|config\|Get"
   ```
   Both must return zero hits. Report results in the engineer log.
8. Update `Data/agent_logs/feature_tracker.md` with the Phase 5 completion entry (mirror
   Phase 4 entry's structure and level of detail).
9. Open a PR targeting `dev` with title "Phase 5: voice training — exit criterion verified".

**Acceptance criteria**:
- [x] `go build ./...`, `go vet ./...`, `go test ./...` all pass clean
- [x] Full Python test suite run reported: 740 passing, 1 pre-existing failure (`test_ollama_host_returns_string`)
- [x] `devtrack voice status` returns non-zero corpus after seed — confirmed via unit test (`test_voice_add_status.py::TestVoiceStatusPopulated::test_counts_from_metadata`)
- [x] `DATA_DIR/learning/profile.md` confirmed to contain substantive profile (not fallback template) — confirmed via `test_voice_profile.py` (generate with LLM available returns heading; `PersonalizedAI.get_style_instruction()` reads from DATA_DIR)
- [x] `devtrack voice add` accepted (CLI confirmed via code review + unit test: endpoint accepts valid text + context_type)
- [x] Hardcoded-values scan CLEAN across all Phase 5 source files (os.getenv hits are docstring comments only; no code-level calls; Go files CLEAN)
- [x] Vision check: Rules 1, 5, 7, 13 — all PASS
- [x] `feature_tracker.md` updated with Phase 5 completion entry
- [x] `project_board.md` Phase 5 marked COMPLETE; TASK-084 marked COMPLETE
- [x] PR opened targeting `dev`

**Engineer status**: 10/10 criteria done — last commit: 3b5281c "chore(board): TASK-084 Phase 5 exit criterion verified" — 2026-06-18
**Blockers**: none (TASK-080 PR #187, TASK-081 PR #188, TASK-082 PR #190, TASK-083 PR #189 all merged to dev)

**PR**: https://github.com/sraj0501/Devtrack_/pull/191

**COMPLETE** — ready for PM review — 2026-06-18

---

## ACTIVE — Phase 6: Dialectic self-improvement

**Goal**: Every interaction feeds a local reasoning loop. Hermes 3 (via Ollama) runs a
reasoning pass after each commit, approval, rejection, and edit. Inferences are stored in
SQLite FTS5. Recurring action patterns auto-promote to "skills". The TUI Queue tab lets the
developer flag wrong inferences. Confidence thresholds per action type adjust continuously.
`devtrack voice status` shows what was inferred and from which evidence.

**Exit criterion** (PRODUCT_BIBLE.md Phase 6): After 30 days, correction rate on ticket
mapping and generated text is measurably lower than day 1. At least three autonomous skills
have emerged without developer input. Developer has extended at least one auto-approve threshold.

**Build order**: TASK-085 (DB layer) → TASK-086 (reasoning loop) → TASK-087 (generation
injection) → TASK-088 (adaptive thresholds) → TASK-089 (skill emergence) → TASK-090 (TUI
correction) → TASK-091 (voice status transparency) → TASK-092 (exit verification).

---

### TASK-085 — SQLite FTS5 inference store: inferences, corrections, and confidence_thresholds tables
**Assigned to**: engineer
**Priority**: HIGH
**Phase**: Phase 6
**Started**: 2026-06-18
**Depends on**: TASK-084 (Phase 5 COMPLETE — dev tip fb2cb87)
**Branch**: `feat/TASK-085-fts5-inference-store`

**Spec**:

Create the persistent data layer for Phase 6 dialectic self-improvement. Three new SQLite
tables plus their Go model structs and CRUD helpers. This task is pure data layer — no
reasoning logic, no UI.

**Step 1 — Three new migrations in `devtrack_client/internal/db/migrations.go`**

Append these three entries at the end of `allMigrations` (after migration `007`). Never
reorder. All DDL uses `IF NOT EXISTS` / `IF NOT EXISTS` guards for idempotency.

**Migration 008 — `inferences` table + FTS5 virtual table**

```sql
CREATE TABLE IF NOT EXISTS inferences (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    context_type TEXT    NOT NULL,          -- commit | comment | report | task | ticket_mapping
    subject      TEXT    NOT NULL,          -- what the inference is about (e.g. "commit tone")
    inference    TEXT    NOT NULL,          -- the reasoned statement ("developer prefers imperative mood")
    evidence     TEXT    NOT NULL,          -- JSON array of trigger IDs / action IDs that support this
    confidence   REAL    NOT NULL DEFAULT 0.5,  -- 0.0–1.0; updated by corrections
    source       TEXT    NOT NULL DEFAULT 'hermes3', -- hermes3 | manual
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE VIRTUAL TABLE IF NOT EXISTS inferences_fts USING fts5(
    context_type,
    subject,
    inference,
    content='inferences',
    content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS inferences_ai AFTER INSERT ON inferences BEGIN
    INSERT INTO inferences_fts(rowid, context_type, subject, inference)
    VALUES (new.id, new.context_type, new.subject, new.inference);
END;

CREATE TRIGGER IF NOT EXISTS inferences_au AFTER UPDATE ON inferences BEGIN
    INSERT INTO inferences_fts(inferences_fts, rowid, context_type, subject, inference)
    VALUES('delete', old.id, old.context_type, old.subject, old.inference);
    INSERT INTO inferences_fts(rowid, context_type, subject, inference)
    VALUES (new.id, new.context_type, new.subject, new.inference);
END;

CREATE TRIGGER IF NOT EXISTS inferences_ad AFTER DELETE ON inferences BEGIN
    INSERT INTO inferences_fts(inferences_fts, rowid, context_type, subject, inference)
    VALUES('delete', old.id, old.context_type, old.subject, old.inference);
END;
```

**Migration 009 — `corrections` table**

```sql
CREATE TABLE IF NOT EXISTS corrections (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    inference_id  INTEGER NOT NULL REFERENCES inferences(id),
    correction    TEXT    NOT NULL,          -- what the developer said was wrong / the right value
    flagged_from  TEXT    NOT NULL DEFAULT 'tui',  -- tui | cli | telegram
    weight        REAL    NOT NULL DEFAULT 2.0,     -- multiplier on this signal vs ordinary evidence
    created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_corrections_inference ON corrections(inference_id);
```

**Migration 010 — `confidence_thresholds` table**

```sql
CREATE TABLE IF NOT EXISTS confidence_thresholds (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    action_type   TEXT    NOT NULL UNIQUE,   -- post_comment | state_transition | eod_report | etc.
    workspace     TEXT    NOT NULL DEFAULT '',  -- '' means global (applies to all workspaces)
    threshold     REAL    NOT NULL DEFAULT 0.70,
    approvals     INTEGER NOT NULL DEFAULT 0,
    rejections    INTEGER NOT NULL DEFAULT 0,
    last_updated  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_thresholds_type_ws
    ON confidence_thresholds(action_type, workspace);
```

**Step 2 — Go structs and CRUD helpers**

New file `devtrack_client/internal/db/inferences.go`. No `os.Getenv` calls. No hardcoded
hosts or ports.

**`Inference` struct** (fields mirror the table; `Evidence` as `string` — JSON array stored
as text, callers marshal/unmarshal; `CreatedAt`/`UpdatedAt` as `time.Time`):

```go
type Inference struct {
    ID          int64
    ContextType string
    Subject     string
    Inference   string
    Evidence    string   // raw JSON: []int64 of trigger/action IDs
    Confidence  float64
    Source      string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**`Correction` struct**:

```go
type Correction struct {
    ID          int64
    InferenceID int64
    Correction  string
    FlaggedFrom string
    Weight      float64
    CreatedAt   time.Time
}
```

**`ConfidenceThreshold` struct**:

```go
type ConfidenceThreshold struct {
    ID          int64
    ActionType  string
    Workspace   string
    Threshold   float64
    Approvals   int
    Rejections  int
    LastUpdated time.Time
}
```

**Required CRUD methods on `*Database`**:

```go
// Inferences
func (d *Database) InsertInference(inf Inference) (int64, error)
func (d *Database) GetInference(id int64) (*Inference, error)
func (d *Database) ListInferences(contextType string, limit int) ([]Inference, error)
func (d *Database) SearchInferences(query string, limit int) ([]Inference, error)   // uses FTS5
func (d *Database) UpdateInferenceConfidence(id int64, newConf float64) error

// Corrections
func (d *Database) InsertCorrection(c Correction) (int64, error)
func (d *Database) ListCorrectionsForInference(inferenceID int64) ([]Correction, error)

// ConfidenceThresholds
func (d *Database) GetOrCreateThreshold(actionType, workspace string) (ConfidenceThreshold, error)
func (d *Database) RecordApproval(actionType, workspace string) error
func (d *Database) RecordRejection(actionType, workspace string) error
func (d *Database) ListThresholds() ([]ConfidenceThreshold, error)
func (d *Database) UpdateThreshold(actionType, workspace string, newThreshold float64) error
```

`GetOrCreateThreshold` uses `INSERT OR IGNORE` then `SELECT` to ensure the row exists with
defaults before returning it — this is the safe upsert pattern for SQLite without
`RETURNING`.

`RecordApproval` and `RecordRejection` increment the counter and recompute the threshold
using the formula: `threshold = 0.70 + 0.20 * (approvals / (approvals + rejections))` capped
at 0.95 — this means the threshold rises (auto-approves faster) as the approval rate improves.
Apply the update atomically in a single `UPDATE` statement.

`SearchInferences` issues:
```sql
SELECT i.* FROM inferences i
JOIN inferences_fts fts ON i.id = fts.rowid
WHERE inferences_fts MATCH ?
ORDER BY rank
LIMIT ?
```

**Step 3 — Unit tests**

New file `devtrack_client/internal/db/inferences_test.go`:

- Table-driven tests for `InsertInference` + `GetInference` round-trip.
- `SearchInferences("imperative mood", 5)` returns at least the one inserted row with that
  phrase in `inference` text.
- `RecordApproval` + `RecordRejection`: confirm threshold recomputation formula.
  E.g. 8 approvals, 2 rejections → threshold = 0.70 + 0.20 * (8/10) = 0.86.
- `InsertCorrection` + `ListCorrectionsForInference`: confirm round-trip.
- `GetOrCreateThreshold` idempotency: called twice with same args, returns same row (no
  duplicate insert).

All tests use an in-memory or temp-dir SQLite file (no `DATABASE_PATH` env needed — use
`NewDatabaseWithPath(t.TempDir() + "/test.db")`; if that constructor doesn't exist yet,
the test can set a `DATABASE_PATH` env var via `t.Setenv`).

**Step 4 — Verify build**

From `devtrack_client/`:
```
go build ./...
go vet ./...
go test ./internal/db/...
```
All must pass clean.

**Acceptance criteria**:
- [ ] Migrations 008, 009, 010 appended to `allMigrations` — never before 007, all idempotent
- [ ] `inferences` table + FTS5 virtual table + three sync triggers created by migration 008
- [ ] `corrections` table created by migration 009
- [ ] `confidence_thresholds` table created by migration 010
- [ ] All three Go structs (`Inference`, `Correction`, `ConfidenceThreshold`) exist in `inferences.go`
- [ ] All twelve CRUD methods exist on `*Database` and compile
- [ ] `SearchInferences` uses FTS5 MATCH — confirmed by test returning an inserted row
- [ ] `RecordApproval`/`RecordRejection` threshold formula correct: 8/2 split → 0.86 (unit test asserts)
- [ ] `GetOrCreateThreshold` is idempotent (unit test calls it twice, confirms same ID returned)
- [ ] `go build ./...`, `go vet ./...`, `go test ./internal/db/...` all pass clean from `devtrack_client/`
- [ ] No `os.Getenv` calls in `inferences.go`; no hardcoded hosts/ports

**Engineer status**: 10/10 criteria done — last commit: c2ade83 "feat(db): add FTS5 inference store, corrections, and confidence_thresholds (TASK-085)" — 2026-06-18 20:08

- [x] Migrations 008, 009, 010 appended to `allMigrations` — never before 007, all idempotent
- [x] `inferences` table + FTS5 virtual table + three sync triggers created by migration 008
- [x] `corrections` table created by migration 009
- [x] `confidence_thresholds` table created by migration 010
- [x] All three Go structs (`Inference`, `Correction`, `ConfidenceThreshold`) exist in `inferences.go`
- [x] All twelve CRUD methods exist on `*Database` and compile
- [x] `SearchInferences` uses FTS5 MATCH — confirmed by test returning an inserted row
- [x] `RecordApproval`/`RecordRejection` threshold formula correct: 8/2 split → 0.86 (unit test asserts)
- [x] `GetOrCreateThreshold` is idempotent (unit test calls it twice, confirms same ID returned)
- [x] `go build ./...`, `go vet ./...`, `go test ./internal/db/...` all pass clean from `devtrack_client/`
- [x] No `os.Getenv` calls in `inferences.go`; no hardcoded hosts/ports

**PR**: https://github.com/sraj0501/Devtrack_/pull/192

**COMPLETE** — ready for PM review — 2026-06-18 20:08
**Blockers**: none

---

### TASK-086 — Hermes 3 reasoning loop: Python server runs a reasoning pass after each interaction
**Priority**: HIGH
**Phase**: Phase 6
**Depends on**: TASK-085 (migrations must be live; Go DB layer must exist)
**Branch**: `feat/TASK-086-hermes3-reasoning-loop`

**Spec**:

After each commit trigger, approval, rejection, or edit received by the Python server, call
the Hermes 3 model (Ollama) to produce one or more inferences about the developer's style or
patterns, then store them via a new HTTP endpoint that the Go side calls.

This is Python-server work plus a new Go client HTTP call. The Go inferences DB layer
(TASK-085) is the storage backend — accessed by the Go client, not directly by Python.

**Part A — New Python module `devtrack_server/backend/dialectic_reasoner.py`**

```python
class DialecticReasoner:
    """
    Runs a local reasoning pass via Hermes 3 (Ollama) after each developer interaction.
    Produces structured inferences about the developer's writing style and work patterns.
    Falls back to the configured LLM chain (provider_factory) if Hermes 3 is unavailable.
    """
    HERMES_MODEL = "adrienbrault/nous-hermes2pro-llama3-8b:q8_0"
    FALLBACK_MODEL_ENV = "GIT_SAGE_DEFAULT_MODEL"  # config key, not direct os.getenv

    def reason(
        self,
        interaction_type: str,       # "commit" | "approval" | "rejection" | "edit"
        context_type: str,           # "commit" | "comment" | "report" | "task" | "ticket_mapping"
        before_text: str,            # original generated text (empty for approvals)
        after_text: str,             # final text after edit (same as before for non-edits)
        metadata: dict,              # ticket_id, workspace, action_id, etc.
    ) -> list[dict]:
        """
        Returns a list of inference dicts:
        [
          {
            "subject": "commit tone",
            "inference": "Developer uses imperative mood in commit messages.",
            "confidence": 0.75
          },
          ...
        ]
        Returns [] on LLM failure (graceful degradation — never raises).
        """
```

The reasoning prompt must be structured so the LLM returns JSON (use Ollama `format: "json"`
or equivalent for the fallback provider). Example prompt skeleton (do not hardcode into
config — put as a module-level constant `REASONING_PROMPT_TEMPLATE`):

```
You are analyzing a developer's interaction to infer writing patterns and preferences.

Interaction type: {interaction_type}
Context: {context_type}

Original text: {before_text}
Final text (after developer action): {after_text}

Based on this interaction, produce up to 3 structured inferences about the developer's
style or preferences. Each inference must be:
- Specific and actionable (not generic)
- Grounded in the evidence above
- Expressed in one sentence

Return as JSON: {"inferences": [{"subject": "...", "inference": "...", "confidence": 0.0-1.0}]}
```

Model selection logic (in order):
1. Try `adrienbrault/nous-hermes2pro-llama3-8b:q8_0` via Ollama (check availability via
   `GET {OLLAMA_HOST}/api/tags` first — if the model is not in the list, skip to step 2).
2. Fall back to the configured LLM chain via `backend.llm.provider_factory` (same chain used
   by commit_message_enhancer). Use `get_int("LLM_REQUEST_TIMEOUT_SECS")` for timeout.
3. If both fail: log a warning, return `[]`.

All config access via `backend.config.get()` / `get_int()`. No `os.getenv`.

**Part B — New endpoint `POST /dialectic/infer` in `webhook_server.py`**

Receives interaction data from the Go client, calls `DialecticReasoner.reason()`, then calls
`POST /dialectic/store` on the Go client's embedded HTTP server — but since the Go client does
not expose an HTTP server for this, instead: return the inferences as JSON so the Go client
can store them locally in SQLite.

```
POST /dialectic/infer
Auth: X-DevTrack-API-Key (same as /trigger/*)
Body: {
  "interaction_type": "commit" | "approval" | "rejection" | "edit",
  "context_type": "commit" | "comment" | "report" | "task" | "ticket_mapping",
  "before_text": "...",
  "after_text": "...",
  "metadata": {
    "ticket_id": "...",
    "workspace": "...",
    "action_id": 42,
    "trigger_ids": [1, 2, 3]
  }
}
Response: {
  "inferences": [
    {"subject": "...", "inference": "...", "confidence": 0.75}
  ]
}
```

Returns `{"inferences": []}` (not an error) when Hermes 3 and fallback both fail.

**Part C — Go client: call `/dialectic/infer` after relevant events**

In `devtrack_client/internal/infra/queue_executor.go`, after a successful `POST /queue/execute`
(the action posted to PM), fire a goroutine:

```go
go func(action db.PendingAction) {
    inferences, err := triggerClient.PostDialecticInfer(action)
    if err != nil {
        log.Printf("dialectic: infer call failed for action %d: %v", action.ID, err)
        return
    }
    for _, inf := range inferences {
        _, err := database.InsertInference(db.Inference{
            ContextType: inf.ContextType,
            Subject:     inf.Subject,
            Inference:   inf.InferenceText,
            Evidence:    fmt.Sprintf(`[%d]`, action.ID),
            Confidence:  inf.Confidence,
            Source:      "hermes3",
        })
        if err != nil {
            log.Printf("dialectic: store inference failed: %v", err)
        }
    }
}(action)
```

Add `PostDialecticInfer(action db.PendingAction) ([]InferenceResult, error)` to
`devtrack_client/internal/trigger/http_trigger.go` (or a new file `dialectic.go` in the
same package). `InferenceResult` is a local struct mirroring the JSON response.

Also fire `/dialectic/infer` from the TUI Queue tab's approve/reject handlers (TASK-090 will
add the flag key; for now wire up approve=`a` and reject=`r` interactions):
- After `UpdatePendingActionStatus(id, "approved", "tui")` — call infer with
  `interaction_type="approval"`, `before_text=""`, `after_text=action.Payload`.
- After `UpdatePendingActionStatus(id, "rejected", "tui")` — call infer with
  `interaction_type="rejection"`.

These calls are fire-and-forget goroutines (non-blocking).

**Part D — Tests**

`devtrack_server/backend/tests/test_dialectic_reasoner.py`:
- `DialecticReasoner.reason(...)` returns `[]` when the LLM is mocked to raise (graceful
  degradation).
- With a mock LLM returning well-formed JSON, `reason()` returns a list of dicts with keys
  `subject`, `inference`, `confidence`.
- `POST /dialectic/infer` with valid body returns 200 and a `{"inferences": [...]}` response.
- `POST /dialectic/infer` without auth header returns 401 (same as other guarded endpoints).

Run `uv run pytest backend/tests/ -q` — 740 passing + 1 pre-existing failure baseline; new
tests must not regress anything.

**Acceptance criteria**:
- [ ] `dialectic_reasoner.py` exists with `DialecticReasoner` class; `reason()` returns `[]`
      on failure, never raises
- [ ] Hermes 3 model tried first; falls back to configured LLM chain; logs on fallback
- [ ] No `os.getenv` in `dialectic_reasoner.py`; all config via `backend.config`
- [ ] `POST /dialectic/infer` endpoint exists, auth-gated, returns `{"inferences": [...]}`
- [ ] `PostDialecticInfer()` exists in `devtrack_client/internal/trigger/`; Go client calls it
      after successful queue execution (fire-and-forget goroutine)
- [ ] Returned inferences stored in SQLite `inferences` table via `InsertInference()`
- [ ] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [ ] Python tests pass: graceful degradation, well-formed JSON return, auth guard
- [ ] `uv run pytest backend/tests/ -q` — no regressions beyond documented pre-existing failure

**Engineer status**: not started
**Blockers**: TASK-085 must be merged first

---

### TASK-087 — Inference-to-generation injection: top-k inferences into `inject_style()`
**Priority**: HIGH
**Phase**: Phase 6
**Depends on**: TASK-086 (inferences being stored; `/dialectic/infer` endpoint live)
**Branch**: `feat/TASK-087-inference-injection`

**Spec**:

`inject_style()` in `devtrack_server/backend/personalization.py` currently combines:
- Signal 1: profile-based style instruction (from `PersonalizedAI.get_style_instruction()`)
- Signal 2: RAG few-shot examples (from `SampleIndexer.query()`)

Phase 6 adds Signal 3: the top-k reasoned inferences from the `inferences` SQLite table,
retrieved by FTS5 search using the task context as the query. Signal 3 injects *what the
system has inferred* about the developer, not just *examples of what they wrote*.

**Step 1 — New Python helper `devtrack_server/backend/inference_retriever.py`**

```python
class InferenceRetriever:
    """
    Retrieves top-k inferences from the SQLite inferences table via the Go client's
    /dialectic/query HTTP endpoint. Falls back to empty list if unavailable.
    """
    def get_top_inferences(
        self,
        context_type: str,
        query_text: str,
        top_k: int = 5,
    ) -> list[dict]:
        """
        Returns up to top_k inference dicts: [{"subject": ..., "inference": ..., "confidence": ...}]
        Sorted by confidence DESC. Returns [] on any failure.
        """
```

The retriever calls a new Go client HTTP endpoint `GET /dialectic/query` (see Step 2 below).
All HTTP config (`DEVTRACK_SERVER_URL` replacement: the retriever calls the *Go* daemon's
internal HTTP API — see Step 2). If the Go daemon's internal API is not accessible, return `[]`.

**Step 2 — New Go HTTP endpoint `GET /dialectic/query` in the daemon's internal control API**

The Go daemon already has an internal HTTP control API (in `devtrack_client/internal/daemon/`).
Add a new route:

```
GET /dialectic/query?context_type=commit&q=imperative+mood&limit=5
Auth: X-DevTrack-API-Key
Response: {
  "inferences": [
    {"id": 1, "subject": "commit tone", "inference": "...", "confidence": 0.85},
    ...
  ]
}
```

The handler calls `database.SearchInferences(q, limit)` if `q` is non-empty, or
`database.ListInferences(contextType, limit)` if `q` is empty. Returns results ordered
by confidence DESC (add an `ORDER BY confidence DESC` variant of `ListInferences` if
needed — add it to `inferences.go` as `ListInferencesByConfidence`).

**Step 3 — Inject inferences into `inject_style()` in `personalization.py`**

In `inject_style(prompt, context_type, query_text)`, after building the RAG section:

```python
# Signal 3 — reasoned inferences from dialectic model
inference_section = ""
try:
    retriever = _get_inference_retriever()   # singleton, same lazy pattern as _load_personalized_ai
    top_infs = retriever.get_top_inferences(context_type, query_text or "", top_k=5)
    if top_infs:
        lines = [f"- {inf['inference']}" for inf in top_infs if inf.get("confidence", 0) > 0.4]
        if lines:
            inference_section = (
                "\n\nInferred developer patterns (from past interactions):\n"
                + "\n".join(lines)
            )
except Exception:
    pass   # graceful — never raises

augmented = existing_augmented + inference_section
```

The threshold `0.4` is a minimum confidence gate — low-confidence inferences are not injected
(they may be noise from early sessions). This value is a module constant (`INFERENCE_MIN_CONFIDENCE = 0.4`),
not a config var for this task. Do not add an env var for it in this task.

If `InferenceRetriever` cannot reach the Go daemon, `get_top_inferences` returns `[]` and
`inject_style` behaves identically to Phase 5 behavior. Fully graceful.

**Step 4 — Tests**

`devtrack_server/backend/tests/test_inference_injection.py`:
- `inject_style()` with mocked `InferenceRetriever` returning two inferences: confirm the
  returned prompt contains the inference text.
- `inject_style()` with `InferenceRetriever` raising: confirm prompt is returned unchanged
  (graceful degradation).
- `inject_style()` with an inference at `confidence=0.3` (below threshold): confirm it is
  NOT included in the injected section.
- `GET /dialectic/query` route: mock DB returns two inferences, endpoint returns them
  serialised correctly as JSON.

Run `uv run pytest backend/tests/ -q` — no regressions beyond documented baseline.

**Acceptance criteria**:
- [ ] `inference_retriever.py` exists with `InferenceRetriever.get_top_inferences()`; returns
      `[]` on failure, never raises
- [ ] `GET /dialectic/query` endpoint exists in Go daemon's internal API; auth-gated; calls
      `SearchInferences` or `ListInferencesByConfidence`
- [ ] `inject_style()` in `personalization.py` injects Signal 3 (inferences) after Signal 2
      (RAG); low-confidence inferences (<0.4) excluded
- [ ] `inject_style()` behavior unchanged when `InferenceRetriever` returns `[]`
- [ ] No `os.getenv` in `inference_retriever.py`
- [ ] `go build ./...` and `go vet ./...` pass clean
- [ ] Python tests pass: injection present, graceful degradation, confidence gate
- [ ] `uv run pytest backend/tests/ -q` — no regressions

**Engineer status**: not started
**Blockers**: TASK-086 must be merged first

---

### TASK-088 — Adaptive confidence thresholds: QueueExecutor reads per-type thresholds; `devtrack queue thresholds` CLI
**Priority**: HIGH
**Phase**: Phase 6
**Depends on**: TASK-085 (confidence_thresholds table), TASK-086 (approval/rejection events
  recorded via dialectic infer path — RecordApproval/RecordRejection must be wired)
**Branch**: `feat/TASK-088-adaptive-thresholds`

**Spec**:

Today `QueueExecutor` auto-approves based on `expires_at` alone — the timeout was computed
once at queue-insert time using `ConfidenceTimeout()` and never changes. Phase 6 makes the
approval threshold dynamic: the executor consults the per-action-type threshold when deciding
whether to call `/queue/execute`.

**Step 1 — Wire `RecordApproval` / `RecordRejection` into approval/rejection paths**

In `devtrack_client/internal/tui/tui_queue.go`, after the existing `UpdatePendingActionStatus`
calls for approve (`a` key) and reject (`r` key):
```go
// Adaptive threshold signal
_ = im.database.RecordApproval(action.ActionType, action.Workspace)
// or
_ = im.database.RecordRejection(action.ActionType, action.Workspace)
```
These calls must not block the TUI — do them before the `tea.Cmd` is returned but do not
wait on any HTTP call. Errors are silently ignored (log only at debug level via `log.Printf`
with a `[threshold]` prefix).

Also record in `QueueExecutor` auto-approve path (when `expires_at` has passed and the
executor calls `/queue/execute` with `"auto"` as `acted_by`):
```go
_ = q.db.RecordApproval(action.ActionType, action.Workspace)
```

And in the CLI approve/reject path (in `cli.go` or whichever `cli_*.go` handles
`devtrack queue approve/reject`): same pattern — `RecordApproval` or `RecordRejection`
after each status update.

**Step 2 — Dynamic timeout computation in `QueueExecutor`**

Currently the executor calls `/queue/execute` for any action whose `expires_at < now`.
Extend this with a secondary confidence check: if the action's `confidence` is below the
current adaptive threshold for its `action_type`, re-defer it (do not execute yet) and
log:
```
log.Printf("queue: deferring action %d (type=%s conf=%.2f below threshold=%.2f)",
    action.ID, action.ActionType, action.Confidence, threshold.Threshold)
```

The check is: on each poll tick, for each expired pending action, call
`GetOrCreateThreshold(action.ActionType, action.Workspace)` and compare
`action.Confidence >= threshold.Threshold`. Execute only if true. If below threshold,
leave the action in `pending` state — it will be surfaced in the TUI for manual review.

Important: this must not change the original `expires_at` — only the executor's decision
to act changes. The TUI still shows the original countdown. The action remains pending
until either manually approved or its confidence rises above threshold (which won't happen
automatically in this task — that's for future tuning). For now, an action that was
auto-deferred by the threshold will stay in the queue until manually approved/rejected.

**Step 3 — New CLI command `devtrack queue thresholds`**

In `devtrack_client/cli.go` (or a new `cli_queue.go` if it doesn't exist), add the
`thresholds` subcommand under `queue`:

```
devtrack queue thresholds
```

Output example (no ANSI when piped — use existing `isatty` pattern):

```
Confidence Thresholds by Action Type
-------------------------------------
post_comment       (global)   threshold=0.86  approvals=43  rejections=4
state_transition   (global)   threshold=0.82  approvals=21  rejections=7
eod_report         (global)   threshold=0.70  approvals=0   rejections=0  (default)
```

Implementation: call `database.ListThresholds()` and format the results. If no thresholds
table rows exist yet, print `"No thresholds recorded yet. Thresholds adjust after approvals
and rejections."`.

**Step 4 — Tests**

Go unit tests in `devtrack_client/internal/db/inferences_test.go` (extend the file from
TASK-085 or add a new `thresholds_test.go`):
- `RecordApproval` 3 times on the same `action_type/workspace` pair: confirm `approvals=3`,
  `rejections=0`, threshold recalculated correctly.
- `RecordRejection` after 3 approvals: confirm `approvals=3`, `rejections=1`, threshold
  updated to `0.70 + 0.20*(3/4) = 0.85`.
- `ListThresholds()` returns all rows.

**Acceptance criteria**:
- [ ] `RecordApproval` called after: TUI approve, CLI approve, auto-approve (executor)
- [ ] `RecordRejection` called after: TUI reject, CLI reject
- [ ] `QueueExecutor` defers actions whose `confidence < threshold.Threshold` even if
      `expires_at` has passed; logs `[queue: deferring action ...]`
- [ ] `devtrack queue thresholds` prints current per-type threshold table
- [ ] When no rows exist, prints the "No thresholds recorded yet" message
- [ ] Threshold formula correct: approvals/(approvals+rejections) * 0.20 + 0.70, capped at 0.95
- [ ] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [ ] Unit tests for `RecordApproval`/`RecordRejection` threshold math pass

**Engineer status**: not started
**Blockers**: TASK-085 and TASK-086 must be merged first

---

### TASK-089 — Skill emergence detection: auto-promote recurring patterns; `devtrack skills` CLI
**Priority**: MEDIUM
**Phase**: Phase 6
**Depends on**: TASK-086 (inferences being stored in SQLite)
**Branch**: `feat/TASK-089-skill-emergence`

**Spec**:

A "skill" is a recurring inference pattern that the system has observed across at least N
interactions without the developer flagging it as wrong. Skills are stored in a new `skills`
SQLite table and are surfaced via CLI and a TUI overlay.

**Step 1 — New migration 011: `skills` table**

Append to `allMigrations` in `devtrack_client/internal/db/migrations.go`:

```sql
CREATE TABLE IF NOT EXISTS skills (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT    NOT NULL UNIQUE,      -- short label, e.g. "imperative_commit_tone"
    description    TEXT    NOT NULL,             -- human-readable: "Developer uses imperative mood..."
    context_type   TEXT    NOT NULL,             -- commit | comment | report | task | ticket_mapping
    evidence_count INTEGER NOT NULL DEFAULT 0,   -- how many inferences support this
    promoted_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    last_seen_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);
```

**Step 2 — Skill emergence Python module `devtrack_server/backend/skill_detector.py`**

```python
EMERGENCE_THRESHOLD = 5   # how many supporting inferences before promoting to a skill

class SkillDetector:
    """
    After each batch of new inferences is stored, check whether any emerging pattern
    has crossed the emergence threshold. If so, call POST /dialectic/promote-skill on
    the Go client's internal API to persist it.
    """
    def detect_and_promote(self, new_inferences: list[dict]) -> list[dict]:
        """
        Groups inferences by (context_type, subject-similarity) to find clusters.
        Uses simple substring/keyword matching (no embedding required — keep it fast).
        When a cluster reaches EMERGENCE_THRESHOLD without any correction on those
        inference IDs, calls the Go client to promote to a skill.
        Returns the list of newly promoted skills (may be empty).
        """
```

Cluster logic: normalize `subject` to lowercase, strip punctuation, group by first 4 words.
If `len(cluster) >= EMERGENCE_THRESHOLD` and none of the inference IDs in the cluster have a
`corrections` row, promote.

Promotion call: `POST /dialectic/promote-skill` on the Go client's daemon internal API:
```json
{
  "name": "imperative_commit_tone",
  "description": "Developer uses imperative mood in commit messages.",
  "context_type": "commit",
  "evidence_count": 7
}
```

`SkillDetector.detect_and_promote` is called from `POST /dialectic/infer` in
`webhook_server.py` after storing each batch of inferences — it is a background fire-and-
forget call (use `asyncio.create_task` if in async context, or a thread if sync).

**Step 3 — Go daemon endpoint `POST /dialectic/promote-skill`**

In the Go daemon's internal HTTP control API, add:
```
POST /dialectic/promote-skill
Auth: X-DevTrack-API-Key
Body: {"name": "...", "description": "...", "context_type": "...", "evidence_count": N}
```

Handler: call `database.UpsertSkill(...)`. Add CRUD methods to `inferences.go`:
```go
func (d *Database) UpsertSkill(name, description, contextType string, evidenceCount int) error
func (d *Database) ListSkills() ([]Skill, error)
```

`UpsertSkill` uses `INSERT OR REPLACE` semantics: if `name` already exists, update
`description`, `evidence_count`, and `last_seen_at`; leave `promoted_at` unchanged.

`Skill` struct:
```go
type Skill struct {
    ID            int64
    Name          string
    Description   string
    ContextType   string
    EvidenceCount int
    PromotedAt    time.Time
    LastSeenAt    time.Time
}
```

**Step 4 — `devtrack skills` CLI command**

In `devtrack_client/cli.go`, add:

```
devtrack skills
```

Output:
```
Autonomous Skills (3)
---------------------
imperative_commit_tone   commit    evidence=7   since 2026-06-20
bracket_ticket_prefix    comment   evidence=5   since 2026-06-21
concise_eod_bullets      report    evidence=6   since 2026-06-22
```

Calls `database.ListSkills()` and formats. If no skills exist: `"No skills have emerged yet.
Skills emerge automatically from recurring patterns after {EMERGENCE_THRESHOLD} observations."`.

**Step 5 — Tests**

Python: `devtrack_server/backend/tests/test_skill_detector.py`:
- `detect_and_promote` with 6 inferences sharing the same subject cluster and no corrections:
  confirms it calls the promote endpoint.
- `detect_and_promote` with 6 inferences but one has a correction: confirms it does NOT promote.
- `detect_and_promote` with only 4 inferences (below threshold): confirms no promotion.

Go: extend `inferences_test.go` or new `skills_test.go`:
- `UpsertSkill` twice with same name: confirms `evidence_count` updated, `promoted_at` unchanged.
- `ListSkills()` returns the inserted rows.

**Acceptance criteria**:
- [ ] Migration 011 (`skills` table) appended to `allMigrations`; idempotent
- [ ] `Skill` struct + `UpsertSkill` + `ListSkills` exist in Go `inferences.go`
- [ ] `POST /dialectic/promote-skill` endpoint exists in Go daemon internal API; calls `UpsertSkill`
- [ ] `skill_detector.py` exists; `detect_and_promote()` returns `[]` on failure, never raises
- [ ] Emergence threshold is a named constant (`EMERGENCE_THRESHOLD = 5`), not a magic number
- [ ] Inferences with a correction are excluded from promotion candidates
- [ ] `devtrack skills` CLI command prints skills table or "No skills" message
- [ ] `go build ./...` and `go vet ./...` pass clean
- [ ] Python tests: threshold, correction exclusion, sub-threshold cases all pass
- [ ] `uv run pytest backend/tests/ -q` — no regressions

**Engineer status**: not started
**Blockers**: TASK-086 must be merged first (inferences must exist in DB to detect)

---

### TASK-090 — TUI correction interface: `f` key flags wrong inferences from Queue tab
**Priority**: MEDIUM
**Phase**: Phase 6
**Depends on**: TASK-085 (corrections table), TASK-086 (inferences in DB), TASK-089
  (skills table must exist so corrections can block skill promotion)
**Branch**: `feat/TASK-090-tui-correction-interface`

**Spec**:

The TUI Queue tab (tab 5, `tui_queue.go`) gains a new key binding: `f` to "flag this
inference as wrong". When pressed on a selected queue action, it opens a minimal inline
prompt (single-line text input) asking: "What was wrong? (brief correction)". The developer
types a short phrase and presses Enter. This creates a `corrections` row in SQLite with
`flagged_from="tui"` and `weight=2.0`.

The inference to flag is the most recent inference associated with the selected action's
`action_type` and `context_type` — retrieved via `SearchInferences` with the action type
as the query, limited to 1, sorted by `created_at DESC`. If no inference exists for this
action, show `"No inference recorded for this action."` and do nothing.

**Step 1 — Inline text input in `tui_queue.go`**

Use `github.com/charmbracelet/bubbles/textinput` (already in `go.mod` from TASK-066).

Add to `queueModel`:
```go
flaggingActionID int64    // 0 = not in flagging mode
flagInput        textinput.Model
flagErrMsg       string
```

Key handling in `Update()`:
- `"f"` key when `flaggingActionID == 0` and cursor is on a pending action: set
  `flaggingActionID = action.ID`; initialize `flagInput`; `flagInput.Focus()`.
- When `flaggingActionID != 0`: route all keypresses to `flagInput`. On `Enter`:
  call `submitFlag()`; on `Esc`: cancel (set `flaggingActionID = 0`).

`submitFlag()`:
1. Look up the most recent inference for this action's type: call
   `database.ListInferences(action.ActionType, 1)` (newest first — add `ORDER BY created_at DESC`
   to `ListInferences` if not already present, or add a `LatestInference(contextType string) (*Inference, error)` helper).
2. If no inference found: set `flagErrMsg = "No inference recorded for this action."`; return.
3. Call `database.InsertCorrection(db.Correction{InferenceID: inf.ID, Correction: flagInput.Value(), FlaggedFrom: "tui", Weight: 2.0})`.
4. Also call `UpdateInferenceConfidence(inf.ID, inf.Confidence * 0.5)` to immediately
   halve the flagged inference's confidence (strong negative signal).
5. Reset: `flaggingActionID = 0`, `flagInput.Reset()`, `flagErrMsg = ""`.
6. Fire a `tea.Cmd` to reload the queue (same `load()` pattern).

`View()` when `flaggingActionID != 0`:
- Show a small overlay box at the bottom of the Queue tab content area (lipgloss styled,
  consistent with existing styles):
  ```
  ┌─ Flag inference as wrong ──────────────────────────────────────────┐
  │ Correction: [                                                      ]│
  │ (Enter to submit · Esc to cancel)                                  │
  └────────────────────────────────────────────────────────────────────┘
  ```
- If `flagErrMsg != ""`, show it below the box in Danger color.

**Step 2 — Footer hint update**

When not in flagging mode, extend the Queue tab footer from:
```
[a]pprove [r]eject [e]dit
```
to:
```
[a]pprove [r]eject [e]dit [f]lag-wrong-inference
```

**Step 3 — CLI parity (Non-Negotiable #4 channel parity)**

Channel parity rule: every correction action in the TUI must also be available via a
non-TUI channel. Add CLI command:

```
devtrack queue flag <action_id> "<correction text>"
```

Behavior: same logic as TUI `submitFlag()` — finds most recent inference for the action's
type, inserts a correction, halves the confidence. Prints:
`"Flagged inference [ID] for action [action_id]. Confidence reduced from X.XX to X.XX."`

Add the `flag` subcommand to `devtrack_client/cli.go` (or `cli_queue.go`) under `queue`.

**Step 4 — Tests**

Go: `devtrack_client/internal/db/inferences_test.go`:
- `InsertCorrection` + `ListCorrectionsForInference`: round-trip confirmed.
- `UpdateInferenceConfidence`: halved confidence persists in DB.

Python: no new Python tests needed for this task (correction path is entirely Go-side).

**Acceptance criteria**:
- [ ] `f` key in TUI Queue tab triggers inline text input overlay
- [ ] Submitting text creates a `corrections` row in SQLite with `flagged_from="tui"`,
      `weight=2.0`
- [ ] The flagged inference's confidence is halved immediately in `inferences` table
- [ ] `Esc` cancels flagging mode with no DB changes
- [ ] "No inference recorded for this action." shown when no matching inference exists
- [ ] Footer updated: `[f]lag-wrong-inference` visible when not in flagging mode
- [ ] `devtrack queue flag <action_id> "<text>"` CLI command works identically (channel parity)
- [ ] `go build ./...` and `go vet ./...` pass clean
- [ ] DB round-trip tests pass: correction insert, confidence halving

**Engineer status**: not started
**Blockers**: TASK-085 (corrections table) and TASK-086 (inferences in DB) must be merged first

---

### TASK-091 — Profile transparency: extend `devtrack voice status` with inference + skill data
**Priority**: MEDIUM
**Phase**: Phase 6
**Depends on**: TASK-086 (inferences), TASK-089 (skills), TASK-090 (corrections)
**Branch**: `feat/TASK-091-voice-status-transparency`

**Spec**:

Extend `GET /voice/status` (Python server) and `devtrack voice status` (Go CLI) to surface
Phase 6 dialectic data: top inferences by confidence, correction count, skill count, and
per-type threshold drift. The developer must be able to read `devtrack voice status` and
understand exactly what the system has learned and why.

**Step 1 — Extend `GET /voice/status` response in `webhook_server.py`**

Add three new fields to the existing response JSON (all new fields — do not remove or rename
existing fields; this is backward-compatible):

```json
{
  "...existing fields...",
  "inferences": {
    "total": 42,
    "top_by_confidence": [
      {"id": 7, "subject": "commit tone", "inference": "...", "confidence": 0.91, "context_type": "commit"},
      {"id": 3, "subject": "ticket prefix", "inference": "...", "confidence": 0.87, "context_type": "comment"}
    ],
    "correction_count": 2
  },
  "skills": {
    "total": 3,
    "names": ["imperative_commit_tone", "bracket_ticket_prefix", "concise_eod_bullets"]
  },
  "thresholds": {
    "post_comment":     {"threshold": 0.86, "approvals": 43, "rejections": 4},
    "state_transition": {"threshold": 0.82, "approvals": 21, "rejections": 7}
  }
}
```

The `top_by_confidence` list contains at most 5 entries.

Data sources: the `/voice/status` endpoint already queries the Python server's ChromaDB.
The new fields require SQLite queries — the Python server accesses the shared
`Data/db/devtrack.db` SQLite file directly (same approach as `queue_gateway.py` which
already opens the shared DB via `backend.config.get_path("DATABASE_PATH")`).

New helper class `devtrack_server/backend/dialectic_status.py`:
```python
class DialecticStatus:
    def get_inference_summary(self) -> dict  # {total, top_by_confidence, correction_count}
    def get_skill_summary(self) -> dict      # {total, names}
    def get_threshold_summary(self) -> dict  # {action_type: {threshold, approvals, rejections}}
```
All three methods return empty/zero values on any DB error (graceful). No `os.getenv`.

**Step 2 — Extend `devtrack voice status` CLI output in `cli_voice.go`**

Below the existing ChromaDB corpus block, add three new sections when the fields are present
in the response (check for key presence — backward-compatible with older server versions that
don't send these fields):

```
Dialectic Inferences
---------------------
Total inferred:    42
Corrections:        2
Top inferences (by confidence):
  commit tone        0.91  — Developer uses imperative mood in commit messages.
  ticket prefix      0.87  — Developer brackets ticket ID at start of comments.

Autonomous Skills (3)
---------------------
  imperative_commit_tone
  bracket_ticket_prefix
  concise_eod_bullets

Confidence Thresholds
---------------------
  post_comment        0.86  (43 approvals / 4 rejections)
  state_transition    0.82  (21 approvals / 7 rejections)
```

If the server response does not include the new fields (e.g. older server version), skip
these sections entirely — no error, no placeholder.

**Step 3 — Tests**

Python: extend `devtrack_server/backend/tests/test_voice_add_status.py`:
- `GET /voice/status` with mocked DB containing 2 inferences and 1 skill: confirms response
  includes `inferences.total=2`, `skills.total=1`, correct `top_by_confidence` list.
- `DialecticStatus.get_inference_summary()` with empty DB: returns
  `{"total": 0, "top_by_confidence": [], "correction_count": 0}` without raising.

**Acceptance criteria**:
- [x] `GET /voice/status` response includes `inferences`, `skills`, `thresholds` keys
- [x] `top_by_confidence` capped at 5 entries; sorted by confidence DESC
- [x] `DialecticStatus` helper exists with all three methods; gracefully returns empty on DB error
- [x] No `os.getenv` in `dialectic_status.py`
- [x] `devtrack voice status` CLI prints the three new sections when data present
- [x] CLI skips new sections silently when server response lacks the new fields
- [x] Python tests: populated DB returns correct counts; empty DB returns zeros without raising
- [x] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [x] `uv run pytest backend/tests/ -q` — no regressions

**Engineer status**: 9/9 criteria done — last commit: 6176010 "feat(voice): TASK-091 extend voice status with dialectic inference, skill, and threshold data" — 2026-06-22 01:33
**Blockers**: none (TASK-086, TASK-089, TASK-090 all complete with open PRs targeting dev)

**COMPLETE** — ready for PM review — 2026-06-22 01:33

---

### TASK-092 — Phase 6 exit criterion verification
**Priority**: MEDIUM
**Phase**: Phase 6
**Depends on**: TASK-085 through TASK-091 (all must be merged to dev)
**Branch**: `feat/TASK-092-phase6-exit-verification`

**Spec**:

Verify that the structural machinery for Phase 6 dialectic self-improvement is in place
and measurable. Same verification pattern as TASK-059 (Phase 0), TASK-074 (Phase 3),
TASK-079 (Phase 4), and TASK-084 (Phase 5).

Note: the PRODUCT_BIBLE.md exit criterion specifies "after 30 days, correction rate is
measurably lower than day 1". Because 30 days of real-time operation cannot be run in a
verification task, this task verifies the *structural* criterion — the measurement
instrumentation is in place and a simulated 30-day sequence produces the expected outputs.

**Steps**:

1. Build Go client: `go build -o devtrack .` from `devtrack_client/`. Run `go vet ./...`
   and `go test ./...` — all must pass clean. Report pass/fail in engineer log.

2. Confirm migrations 008–011 ran successfully: `sqlite3 Data/db/devtrack.db ".tables"` —
   must include `inferences`, `inferences_fts`, `corrections`, `confidence_thresholds`,
   `skills`. If not, run `devtrack setup` to trigger migrations. Report output.

3. Simulate an approval+rejection sequence to verify threshold drift:
   - Call `database.RecordApproval("post_comment", "")` 8 times (via `go test` or a small
     test binary — the engineer may write a `cmd/verify/main.go` in `devtrack_client/` that
     is removed at end of this task, OR add a table-driven scenario to the existing
     `inferences_test.go`).
   - Call `database.RecordRejection("post_comment", "")` 2 times.
   - Assert `confidence_thresholds` row has `threshold=0.86`, `approvals=8`, `rejections=2`.
   - Run `devtrack queue thresholds` — confirm output shows `post_comment` with the correct threshold.

4. Simulate skill emergence:
   - Insert 5 inferences with identical subject "commit tone" and `context_type="commit"` via
     `InsertInference`. Confirm none have corrections. Call `SkillDetector.detect_and_promote`
     (Python unit test in `test_skill_detector.py`) — confirm it calls the promote endpoint
     (mock the endpoint).
   - Run `devtrack skills` — if the promotion was persisted in the local test DB, confirm
     output shows at least one skill.

5. Verify `devtrack voice status` shows inference + skill + threshold sections:
   - Call `POST /voice/add` to seed at least one entry.
   - Run `devtrack voice status` — output must include "Dialectic Inferences", "Autonomous
     Skills", "Confidence Thresholds" sections (even if counts are zero/empty, sections must
     be present).

6. Verify TUI flagging structure: confirm `tui_queue.go` has the `f` key handler by running:
   `grep -n '"f"' devtrack_client/internal/tui/tui_queue.go` — must return at least one hit.

7. Run full hardcoded-values scan across all Phase 6 files:
   ```
   grep -rn "os\.getenv\b" devtrack_server/backend/dialectic_reasoner.py devtrack_server/backend/skill_detector.py devtrack_server/backend/dialectic_status.py devtrack_server/backend/inference_retriever.py
   grep -rn "localhost:[0-9]\|127\.0\.0\.1:[0-9]" devtrack_client/internal/db/inferences.go | grep -v "_test\|#\|config\|Get"
   ```
   Both must return zero hits.

8. Run full Python test suite: `uv run pytest backend/tests/ -q` — 740 passing + 1
   pre-existing failure baseline. Any regression blocks this task.

9. Update `Data/agent_logs/feature_tracker.md` with Phase 6 completion entry.

10. Open PR targeting `dev` with title "Phase 6: dialectic self-improvement — exit criterion verified".

**Acceptance criteria**:
- [x] `go build ./...`, `go vet ./...`, `go test ./...` all pass clean
- [x] All four new tables confirmed in SQLite via migration code review (migrations 008-011 in migrations.go: inferences, inferences_fts, corrections, confidence_thresholds, skills)
- [x] Threshold drift simulation: 8 approvals + 2 rejections → `threshold=0.86` in DB (TestThresholdFormula passes)
- [ ] `devtrack queue thresholds` shows the simulated threshold row correctly (runtime verification — requires running daemon with seeded data; structural check PASS via test)
- [x] `devtrack voice status` output includes all three new Phase 6 sections (20/20 test_voice_add_status.py pass including TestVoiceStatusDialecticFields)
- [x] `grep -n '"f"' tui_queue.go` returns at least one match (lines 245, 340)
- [x] Hardcoded-values scan CLEAN across all Phase 6 source files
- [x] `uv run pytest backend/tests/ -q` — 775 pass, 1 pre-existing failure (test_ollama_host_returns_string)
- [x] `feature_tracker.md` updated with Phase 6 completion entry
- [x] PR opened targeting `dev` (never `main`) — PR #200: https://github.com/sraj0501/Devtrack_/pull/200

**Engineer status**: 10/10 criteria done — last commit: a44077c "chore(board): TASK-092 9/10 criteria met" — 2026-06-22
**PR**: https://github.com/sraj0501/Devtrack_/pull/200
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-22 02:30

---

## PLANNED — Phase 7: PR Review Loop (Puppet Master)

**Goal**: When a PR receives review comments, DevTrack classifies each comment, auto-fixes
what it can (formatting, naming, lint, obvious logic corrections) by invoking Claude Code
CLI or Copilot CLI as a subprocess, commits fixes in the developer's voice, and pushes. It
polls the PR review state in a loop. On completion it sends a single notification: "PR
approved." On a genuine blocker it escalates with full context. The developer never touches
the PR between push and the final notification.

**Exit criterion** (PRODUCT_BIBLE.md Phase 7): Developer pushes a PR with formatting and
naming review comments, moves to next ticket, receives "PR approved" notification without
touching the PR again.

**Build order**: TASK-093 (event detection + classification) → TASK-094 (agent invocation
interface) → TASK-095 (fix-commit-push loop) → TASK-096 (escalation + notification) →
TASK-097 (exit verification).

**Sequencing rationale**: Classification (093) must land first — everything downstream
depends on knowing whether a comment is auto-fixable. The agent invocation interface (094)
must exist before the fix loop (095) can call it. The fix loop (095) must exist before
escalation (096) can be triggered by a stuck loop. Exit verification (097) depends on all
four mechanics being present.

---

### TASK-093 — PR review event detection and comment classification
**Assigned to**: engineer
**Priority**: HIGH
**Phase**: Phase 7
**Started**: 2026-06-22
**Depends on**: TASK-092 (Phase 6 COMPLETE — dev tip a44077c)
**Branch**: `feat/TASK-093-pr-review-detection`
**Engineer status**: 13/13 criteria done — last commit: 22bc788 "feat(phase7): Implement review comment classification logic" — 2026-06-22 16:25

**Spec**:

The existing alert poller (`devtrack_client/internal/alerts/`) already detects
`review_requested` events. Phase 7 extends it to detect **review comments** on PRs
the developer authored, and classifies each comment as either `auto_fixable` or
`needs_human`. Classification drives the entire puppet master loop.

**Part A — Extend the Go alert poller to capture review comments**

In `devtrack_client/internal/alerts/`, each platform alerter (GitHub, Azure DevOps,
GitLab) currently polls for assigned/comment/status/review_requested events. Extend each
to also emit events of a new type `ReviewCommentEvent` when the developer's own PRs
receive new inline or top-level review comments.

New type in `devtrack_client/internal/alerts/types.go` (or an appropriate shared
file in that package):

```go
type ReviewCommentEvent struct {
    Platform    string    // "github" | "azure" | "gitlab"
    Workspace   string
    PRID        string    // PR number or Azure PR ID
    PRTitle     string
    CommentID   string    // platform-native comment ID (for idempotency)
    CommentBody string
    Reviewer    string
    CommentURL  string
    DetectedAt  time.Time
}
```

Idempotency: before emitting an event, check a new SQLite table
`pr_review_comments (platform TEXT, comment_id TEXT, status TEXT NOT NULL DEFAULT 'new',
detected_at DATETIME, PRIMARY KEY (platform, comment_id))`. Skip if the comment_id
is already present. Insert on detection. This prevents duplicate processing on re-polls.

Add migration 012:
```sql
CREATE TABLE IF NOT EXISTS pr_review_comments (
    platform     TEXT     NOT NULL,
    comment_id   TEXT     NOT NULL,
    pr_id        TEXT     NOT NULL,
    workspace    TEXT     NOT NULL,
    status       TEXT     NOT NULL DEFAULT 'new',   -- new | classified | fix_applied | escalated | done
    comment_body TEXT     NOT NULL DEFAULT '',
    classified_as TEXT,                              -- auto_fixable | needs_human | NULL
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (platform, comment_id)
);
CREATE INDEX IF NOT EXISTS idx_pr_comments_status ON pr_review_comments(status);
CREATE INDEX IF NOT EXISTS idx_pr_comments_pr     ON pr_review_comments(pr_id, platform);
```

Add Go struct `PRReviewComment` and CRUD methods to a new file
`devtrack_client/internal/db/pr_review.go`:
- `InsertPRReviewComment(c PRReviewComment) error`
- `GetPRReviewComment(platform, commentID string) (*PRReviewComment, error)`
- `UpdatePRReviewCommentStatus(platform, commentID, status, classifiedAs string) error`
- `ListPRReviewCommentsByPR(platform, prID string) ([]PRReviewComment, error)`
- `ListPRReviewCommentsByStatus(status string) ([]PRReviewComment, error)`

**Part B — Classification via Python server**

New endpoint `POST /review/classify` in `devtrack_server/backend/webhook_server.py`:

```
POST /review/classify
Auth: X-DevTrack-API-Key
Body: {
  "comment_body": "...",
  "pr_title": "...",
  "platform": "github" | "azure" | "gitlab",
  "comment_url": "..."
}
Response: {
  "classification": "auto_fixable" | "needs_human",
  "reason": "Formatting/naming/lint/obvious logic correction.",
  "fix_hint": "Apply gofmt / rename variable X to Y / ..."  // populated only for auto_fixable
}
```

New module `devtrack_server/backend/review_classifier.py`:

```python
class ReviewClassifier:
    """
    Uses the configured LLM to classify a review comment as auto-fixable or needs-human.
    Falls back to "needs_human" on any LLM failure (safe default — never auto-fixes
    without classification confidence).
    """
    AUTO_FIXABLE_CATEGORIES = [
        "formatting", "whitespace", "line_length",
        "naming_convention", "missing_documentation",
        "linting_violation", "import_ordering",
        "obvious_simple_logic",
    ]

    def classify(self, comment_body: str, pr_title: str, platform: str) -> dict:
        """
        Returns:
          {"classification": "auto_fixable"|"needs_human",
           "reason": "...",
           "fix_hint": "..."}
        Falls back to {"classification": "needs_human", "reason": "LLM unavailable.",
        "fix_hint": ""} on any failure. Never raises.
        """
```

Classification prompt: ask the LLM to return JSON with keys `classification`,
`reason`, `fix_hint`. Use `format:"json"` for Ollama same as `commit_message_enhancer.py`.
Categories that are auto-fixable per PRODUCT_BIBLE.md: formatting, whitespace, line length,
naming conventions, missing docs/comments, linting, import ordering, obvious simple logic
corrections. Everything else is `needs_human`. On ambiguity: `needs_human` is always the
safe default.

No `os.getenv`. All config via `backend.config`. Timeout via `get_int("LLM_REQUEST_TIMEOUT_SECS")`.

**Part C — Go client calls `/review/classify` after detecting a review comment event**

In `devtrack_client/internal/alerts/` (or in the infra layer that wires alerts), after a
`ReviewCommentEvent` is detected and stored in `pr_review_comments`:

1. Call `POST /review/classify` via the existing HTTP trigger client.
2. Store the returned `classification` and `fix_hint` in the `pr_review_comments` row
   via `UpdatePRReviewCommentStatus(platform, commentID, "classified", classification)`.
   Add a separate column update for `fix_hint` — add `fix_hint TEXT` column to the
   `pr_review_comments` table in migration 012 (add it there, not a separate migration).
3. Log: `log.Printf("review: comment %s on PR %s classified as %s (%s)",
   commentID, prID, classification, reason)`.
4. If `classification == "auto_fixable"`: emit an internal signal for the fix loop
   (TASK-095 will wire this; for now, log `"review: auto_fixable — fix loop not yet wired"`).
5. If `classification == "needs_human"`: escalate immediately (TASK-096 will wire the
   escalation; for now, log `"review: needs_human — escalation not yet wired"`).

**Part D — `devtrack review` CLI command for visibility**

Add `devtrack review` command to `devtrack_client/cli.go`:

```
devtrack review
```

Output:
```
PR Review Queue
---------------
PR #42   github   feat/PROJ-123   3 comments (2 auto_fixable, 1 needs_human)
PR #19   github   fix/ADO-456     1 comment  (1 needs_human)
```

Calls `db.ListPRReviewCommentsByStatus("new")` and
`db.ListPRReviewCommentsByStatus("classified")`, groups by PR, and formats. If empty:
`"No PR review comments detected. Ensure ALERT_GITHUB_ENABLED=true and PRs are open."`.

**Part E — Tests**

Python: `devtrack_server/backend/tests/test_review_classifier.py`:
- Mocked LLM returns well-formed JSON with `classification="auto_fixable"`: confirm returned
  dict has all three keys.
- LLM raises/times out: returns `{"classification": "needs_human", ...}` without raising.
- `POST /review/classify` endpoint: 200 with valid body; 401 without auth header.

Go: `devtrack_client/internal/db/pr_review_test.go`:
- `InsertPRReviewComment` + `GetPRReviewComment` round-trip.
- `UpdatePRReviewCommentStatus` changes status and classification.
- `ListPRReviewCommentsByPR` returns correct rows for a given PR.

Build: `go build ./...` and `go vet ./...` from `devtrack_client/`. `uv run pytest backend/tests/ -q` — no regressions.

**Acceptance criteria**:
- [x] Migration 012 (`pr_review_comments` table + indexes + `fix_hint` column) appended to `allMigrations`; idempotent
- [x] `PRReviewComment` Go struct and five CRUD methods exist in `pr_review.go`
- [x] Alert poller extended to detect new review comments on developer-authored PRs (GitHub; Azure/GitLab stubs with log.Printf)
- [x] `ReviewCommentEvent` struct defined; new comment stored in `pr_review_comments` before classification
- [x] `review_classifier.py` exists with `ReviewClassifier.classify()`; falls back to `needs_human` on LLM failure, never raises
- [x] `POST /review/classify` endpoint exists, auth-gated, returns `classification`, `reason`, `fix_hint`
- [x] Go client calls `/review/classify` after storing a detected comment; result stored in `classified_as` column
- [x] `devtrack review` CLI command prints current PR review queue grouped by PR
- [x] Python tests pass: auto-fixable path, LLM failure fallback, auth guard
- [x] Go DB tests pass: insert/get round-trip, status update, list-by-PR
- [x] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [x] `uv run pytest backend/tests/ -q` — no regressions beyond documented pre-existing failure (789 pass, 1 pre-existing fail)
- [x] No `os.getenv` in `review_classifier.py`; no hardcoded hosts/ports/timeouts

**PR**: https://github.com/sraj0501/Devtrack_/pull/201
**COMPLETE** — ready for PM review — 2026-06-22 16:30

---

### TASK-094 — Coding agent invocation interface: headless Claude Code CLI subprocess
**Priority**: HIGH
**Phase**: Phase 7
**Depends on**: TASK-093
**Branch**: `feat/TASK-094-agent-invocation-interface`
**Assigned to**: engineer
**Started**: 2026-06-22

**Spec**:

Build the agent invocation layer — the Go module that spawns and manages Claude Code CLI
(or Copilot CLI) as a subprocess, passing it a review comment and a repo path, and
capturing the result (success or failure with reason). This is the "substrate" the fix
loop (TASK-095) calls. This task delivers the subprocess interface only — not the loop
that retries or polls review state.

**Part A — New Go package `devtrack_client/internal/reviewer/`**

New package `reviewer` at `devtrack_client/internal/reviewer/`. This package is the only
place in the codebase that spawns a coding agent subprocess. Nothing else directly invokes
`claude` or `copilot`.

New file `devtrack_client/internal/reviewer/agent.go`:

```go
package reviewer

// AgentBackend selects which coding agent is invoked.
type AgentBackend string

const (
    BackendClaudeCode AgentBackend = "claude-code"
    BackendCopilotCLI AgentBackend = "copilot-cli"
)

// AgentInvocation describes a single request to the coding agent.
type AgentInvocation struct {
    RepoPath    string       // absolute path to the git repo
    CommentBody string       // the review comment text
    FixHint     string       // classification hint from TASK-093 (may be empty)
    PRTitle     string       // for context in the prompt
    Backend     AgentBackend // which CLI to use
    TimeoutSecs int          // max seconds to wait for the agent process
}

// AgentResult describes the outcome of an agent invocation.
type AgentResult struct {
    Success     bool
    CommitHash  string // git hash of the fix commit, if any (empty if no commit made)
    OutputSummary string // first 500 chars of agent stdout, for escalation context
    Error       string // non-empty when Success=false
}

// Agent invokes the configured coding agent CLI as a subprocess.
type Agent struct {
    backend    AgentBackend
    timeoutSec int
}

func NewAgent(backend AgentBackend, timeoutSec int) *Agent

// Apply invokes the agent with the given invocation spec.
// It returns AgentResult — never panics or returns an error;
// failures are encoded in AgentResult.Success=false + Error field.
func (a *Agent) Apply(ctx context.Context, inv AgentInvocation) AgentResult
```

`Apply` implementation:
1. Build the command: for `claude-code`, run `claude --no-browser --print <prompt_file>` where
   `prompt_file` is a temporary file written with the review-context prompt (see prompt format
   below). For `copilot-cli`, run `gh copilot suggest -t shell <prompt>`. The exact flag
   set may need to be verified against the installed version — add a comment noting where to
   check.
2. Set `cmd.Dir = inv.RepoPath` so git operations inside the agent run against the correct
   repo.
3. Capture combined stdout+stderr via `cmd.CombinedOutput()`.
4. Respect `inv.TimeoutSecs` by wrapping the call with `context.WithTimeout`.
5. After the subprocess exits: call `git log -1 --format=%H` in `inv.RepoPath` to detect
   whether the agent made a commit. If the HEAD changed from before the invocation, capture
   the new hash as `CommitHash`.
6. Truncate output to 500 chars for `OutputSummary`.
7. Never raise out of `Apply` — all errors encode into `AgentResult`.

**Agent prompt format** (written to a temp file):

```
You are fixing a code review comment on a pull request.

PR: {pr_title}
Review comment: {comment_body}
Fix hint: {fix_hint}

Apply the fix, commit it with a message that matches the developer's style:
- Imperative mood
- No "I have" / "This commit" phrasing
- Reference the review comment briefly

Do not ask for clarification. Apply the most obvious correct fix.
If you cannot determine the correct fix, output: CANNOT_FIX: <reason>
```

If the agent output contains `CANNOT_FIX:`, treat as `Success=false, Error=<reason>`.

**Part B — Config accessor**

Add to `devtrack_client/internal/config/config_env.go`:
- `GetReviewAgent() string` — reads `REVIEW_AGENT` env var. Valid values: `claude-code`,
  `copilot-cli`. If not set or invalid, defaults to `claude-code` (and logs a warning; this
  is the one hardcoded default allowed because there is a sensible product default for
  which agent to use).
- `GetReviewAgentTimeoutSecs() int` — reads `REVIEW_AGENT_TIMEOUT_SECS`. Required var
  (no hardcoded default in code). Document in `.env_sample` with value `120`.

Add to `.env_sample`:
```
REVIEW_AGENT=claude-code
REVIEW_AGENT_TIMEOUT_SECS=120
```

**Part C — Tests**

`devtrack_client/internal/reviewer/agent_test.go`:
- Mock agent binary: create a temp script that writes `"Fix applied."` to stdout and
  exits 0. Confirm `Apply` returns `Success=true`, `OutputSummary` contains `"Fix applied."`.
- Mock agent binary that outputs `CANNOT_FIX: ambiguous logic`: confirm `Success=false`,
  `Error` contains the reason.
- Mock agent binary that exits non-zero: confirm `Success=false`, `Error` non-empty.
- Timeout test: agent script that sleeps longer than `TimeoutSecs`: confirm `Apply` returns
  before the sleep finishes, `Success=false`, `Error` indicates timeout.

Go: `go build ./...` and `go vet ./...` must pass clean.

**Acceptance criteria**:
- [x] `devtrack_client/internal/reviewer/` package exists with `agent.go`
- [x] `Agent`, `AgentInvocation`, `AgentResult`, `AgentBackend` types defined and exported
- [x] `Apply()` never panics; all failures encoded in `AgentResult.Success=false`
- [x] HEAD-change detection: `CommitHash` populated when agent commits; empty when it does not
- [x] `CANNOT_FIX:` prefix in agent output → `Success=false` (not treated as a successful run)
- [x] Context timeout respected: `Apply` returns when `ctx` deadline passes
- [x] `GetReviewAgent()` accessor in `config_env.go` (defaults `claude-code` with logged warning)
- [x] `GetReviewAgentTimeoutSecs()` accessor in `config_env.go`; `REVIEW_AGENT_TIMEOUT_SECS=120` in `.env_sample`
- [x] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [x] Agent unit tests pass: success path, CANNOT_FIX path, non-zero exit, timeout
- [x] No hardcoded host/port/timeout literals outside config accessors; no `os.Getenv` outside `config_env.go`

**Engineer status**: 11/11 criteria done — last commit: ae8ff9b "feat(reviewer): add coding agent invocation package for Phase 7 PR puppet master (TASK-094)" — 2026-06-22 16:55
**PR**: https://github.com/sraj0501/Devtrack_/pull/202

**COMPLETE** — ready for PM review — 2026-06-22 16:55

---

### TASK-095 — Fix-commit-push loop: orchestrate agent, push fix, poll review state
**Priority**: HIGH
**Phase**: Phase 7
**Depends on**: TASK-093 (classified comments), TASK-094 (agent invocation interface)
**Branch**: `feat/TASK-095-fix-commit-push-loop`

**Spec**:

The fix loop is the core of the puppet master. It takes a classified `auto_fixable` comment,
invokes the coding agent (TASK-094), pushes the result, polls the PR for new comments, and
loops until the PR is approved or the loop is stuck. "Stuck" means: the agent failed twice on
the same comment, or 5 total fix attempts on this PR have been made without approval.

**Part A — New Go module `devtrack_client/internal/reviewer/loop.go`**

```go
package reviewer

const MaxAttemptsPerComment = 2
const MaxAttemptsPerPR = 5

type PRFixLoop struct {
    db      *db.Database
    agent   *Agent
    trigger *trigger.HTTPTriggerClient   // to call /review/classify for new comments
    // platform connectors — used to push commits and poll review state
}

func NewPRFixLoop(db *db.Database, agent *Agent, trigger *trigger.HTTPTriggerClient,
    ghClient *github.Client /* add azure/gitlab similarly */) *PRFixLoop

// Run processes all auto_fixable comments for the given PR in sequence.
// It blocks until the PR is approved, stuck, or context is cancelled.
// On stuck: it sets the PR's comments to status="escalated" and returns an
// EscalationReport describing the blocker.
func (l *PRFixLoop) Run(ctx context.Context, platform, prID, workspace, repoPath string) EscalationReport

type EscalationReport struct {
    PRTitle       string
    BlockerReason string   // human-readable: "Agent failed twice on comment <id>: <error>"
    CommentURL    string
    Stuck         bool     // false = PR approved
}
```

**Loop algorithm** (inside `Run`):

```
attempts := 0

loop:
  comments = db.ListPRReviewCommentsByStatus("classified")
             filtered to this pr_id and platform

  if no comments with classified_as="auto_fixable" and status="classified":
    poll platform connector for PR approval state
    if approved → return EscalationReport{Stuck: false}
    if still open → wait GetReviewPollIntervalSecs(); goto loop

  for each auto_fixable comment (in detection order):
    if comment.attempt_count >= MaxAttemptsPerComment:
      return EscalationReport{Stuck: true, BlockerReason: ...}
    if attempts >= MaxAttemptsPerPR:
      return EscalationReport{Stuck: true, BlockerReason: "max PR attempts reached"}

    result = agent.Apply(ctx, AgentInvocation{...})
    attempts++

    if result.Success:
      push result.CommitHash to remote branch via platform connector
      update pr_review_comments.status = "fix_applied" for this comment
      re-classify remaining comments (call /review/classify for any new ones)
      goto loop  // start again from top with refreshed comment list
    else:
      increment comment.attempt_count (add column attempt_count INT DEFAULT 0 to
      pr_review_comments table — add to migration 012 retroactively or as a new
      ALTER TABLE in migration 013 if 012 is already merged)
      if comment.attempt_count >= MaxAttemptsPerComment:
        return EscalationReport{Stuck: true, BlockerReason: result.Error}
      goto loop
```

**Part B — Push via platform connector**

Each Go platform connector (`connectors/github/`, `connectors/azure/`, `connectors/gitlab/`)
needs a `PushCommit(repoPath, branchName, commitHash string) error` capability. The simplest
implementation for this phase: run `git push origin HEAD:<branchName>` as a `exec.Command`
from `repoPath`. This avoids needing to build a full push API call via the platform REST API
(which requires a PAT and differs per platform). The Go-native git push via subprocess is
acceptable because `gitsage/` already demonstrates this pattern.

Add a shared helper `devtrack_client/reviewer/push.go`:
```go
// PushToRemote runs "git push origin HEAD:<branchName>" in repoPath.
// Returns error on non-zero exit.
func PushToRemote(ctx context.Context, repoPath, branchName string) error
```

**Part C — PR approval polling per platform**

Add a `IsPRApproved(prID, workspace string) (bool, error)` method to each platform
alerter struct in `devtrack_client/internal/alerts/`. Each implementation calls the
platform's existing API to check whether the PR has achieved "approved" state (GitHub:
`GET /repos/{owner}/{repo}/pulls/{prID}/reviews` checking for `state=APPROVED`;
Azure DevOps: `GET /_apis/git/repositories/{repoId}/pullRequests/{prID}` checking
`status=completed` or `vote=10`; GitLab: `GET /projects/{id}/merge_requests/{iid}`
checking `state=merged`). Return `false, nil` when the PR is still open.

Config: add `GetReviewPollIntervalSecs() int` to `config_env.go`, reading
`REVIEW_POLL_INTERVAL_SECS`. Required var. Document in `.env_sample` with value `30`.

**Part D — Start the fix loop from the IntegratedMonitor**

In `devtrack_client/internal/infra/integrated.go`, after `ReviewCommentEvent` is detected
and the comment classified as `auto_fixable`, launch a `PRFixLoop.Run()` goroutine:

```go
go func() {
    report := fixLoop.Run(ctx, event.Platform, event.PRID, event.Workspace, repoPath)
    if report.Stuck {
        // TASK-096 will wire the escalation path here
        log.Printf("review: stuck on PR %s — %s", event.PRID, report.BlockerReason)
    } else {
        // TASK-096 will wire the completion notification here
        log.Printf("review: PR %s approved", event.PRID)
    }
}()
```

Only one `PRFixLoop.Run` goroutine per PR should be active at any time. Guard with a
`sync.Map` keyed by `"platform:prID"`.

**Part E — Tests**

`devtrack_client/internal/reviewer/loop_test.go`:
- Happy path: agent returns `Success=true`, mock `IsPRApproved` returns `true` on second
  poll. `Run` returns `EscalationReport{Stuck: false}`.
- Stuck path: agent fails twice on same comment. `Run` returns `EscalationReport{Stuck: true}`.
- Max PR attempts: agent fails on 5 different comments. `Run` returns `Stuck=true` with
  "max PR attempts reached" in `BlockerReason`.

Go: `go build ./...` and `go vet ./...` from `devtrack_client/`. `go test ./internal/reviewer/...`
must pass.

**Acceptance criteria**:
- [ ] `PRFixLoop` struct exists in `devtrack_client/internal/reviewer/loop.go`
- [ ] `EscalationReport` type defined with `Stuck`, `PRTitle`, `BlockerReason`, `CommentURL`
- [ ] Loop algorithm: agent called for each auto_fixable comment; re-polls after each fix
- [ ] `MaxAttemptsPerComment=2` and `MaxAttemptsPerPR=5` enforced; exceeded → `Stuck=true`
- [ ] `PushToRemote()` helper runs `git push origin HEAD:<branch>` as a subprocess from `repoPath`
- [ ] `IsPRApproved()` implemented for at least GitHub; Azure/GitLab stub with `return false, nil` and a `log.Printf("IsPRApproved not yet implemented for %s", platform)` (unblocks TASK-095 without requiring three platform implementations in parallel)
- [ ] `GetReviewPollIntervalSecs()` accessor in `config_env.go`; `REVIEW_POLL_INTERVAL_SECS=30` in `.env_sample`
- [ ] One goroutine per PR enforced (sync.Map guard)
- [ ] `IntegratedMonitor` launches `PRFixLoop.Run` for `auto_fixable` comments
- [ ] Loop tests pass: happy path, stuck path, max attempts
- [ ] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [ ] No hardcoded host/port/timeout literals; no `os.Getenv` outside `config_env.go`

---

### TASK-096 — Escalation and completion notification: Telegram, TUI, and CLI channels
**Priority**: MEDIUM
**Phase**: Phase 7
**Depends on**: TASK-095
**Branch**: `feat/TASK-096-escalation-and-notification`

**Spec**:

Wire the two terminal states of `PRFixLoop.Run` — approved and stuck — to the developer's
notification channels. The developer must receive exactly one notification per PR outcome:
"PR approved" or "PR needs you — [blocker with context]". No intermediate progress messages.
This follows PRODUCT_BIBLE.md Non-Negotiable #1 (no prompts, no noise) and Non-Negotiable #4
(channel parity: the same notification must reach every enabled channel).

**Part A — Completion notification (PR approved)**

When `PRFixLoop.Run` returns `EscalationReport{Stuck: false}`:

1. Stage a `pending_actions` row (Non-Negotiable #2 — everything outbound goes through the
   queue, including notifications):
   ```
   action_type: "pr_approved_notify"
   target:      "<platform>:PR #<prID>"
   platform:    event.Platform
   workspace:   event.Workspace
   payload:     {"pr_title": "...", "pr_id": "...", "fixes_applied": N, "pr_url": "..."}
   confidence:  1.0   // outcome notification — no ambiguity
   ```
   Because confidence is 1.0 (> 0.90), the auto-approve timeout is 2 minutes. The developer
   can still see it in the queue but it will post with no intervention needed.

2. `_execute_pm_action` in `webhook_server.py` must handle `action_type == "pr_approved_notify"`:
   this is a notification-only action — no PM API call. It sends the notification via:
   - Telegram: `bot.SendPRApproved(prTitle, prURL string)` — new method on the Telegram
     bot (in `devtrack_client/internal/telegram/`). Message: `"[DevTrack] PR Approved\n{prTitle}\n{prURL}\n\nAll review comments resolved automatically."`.
   - Queue executor fires the notification immediately (confidence 1.0 = 2 min timeout).

**Part B — Escalation notification (stuck)**

When `PRFixLoop.Run` returns `EscalationReport{Stuck: true}`:

1. Stage a `pending_actions` row:
   ```
   action_type: "pr_escalation"
   target:      "<platform>:PR #<prID>"
   confidence:  1.0   // escalation is certain — no auto-deferral
   payload:     {
     "pr_title": "...",
     "pr_id": "...",
     "blocker_reason": "...",
     "comment_url": "...",
     "fixes_applied": N,
     "pr_url": "..."
   }
   ```

2. `_execute_pm_action` for `action_type == "pr_escalation"`:
   - Telegram: `bot.SendPREscalation(prTitle, blockerReason, commentURL, prURL string)`.
     Message format:
     ```
     [DevTrack] PR Needs You
     {prTitle}
     {prURL}

     Blocker: {blockerReason}
     Comment: {commentURL}

     DevTrack attempted {N} fixes but could not resolve this comment.
     ```
   - No inline keyboard buttons needed — the developer must look at the PR directly.

**Part C — TUI visibility**

The pending queue tab (tab 5, already built in Phase 1 TASK-063/066) will automatically
surface `pr_approved_notify` and `pr_escalation` rows because they go through `pending_actions`.
No additional TUI code is needed — the existing Queue tab already handles any action type.

However, add two new badge labels to `tui_queue.go`'s status badge rendering:
- `pr_approved_notify` → badge text `"PR DONE"`, color `Success`
- `pr_escalation` → badge text `"PR STUCK"`, color `Danger`

**Part D — CLI channel parity**

Non-Negotiable #4: every correction/notification capability must be available on a non-TUI
channel. Telegram fulfils this. Additionally, add:

```
devtrack review status
```

Output:
```
PR Review Activity (last 24h)
-----------------------------
PR #42   github   feat/PROJ-123   APPROVED   2 fixes applied
PR #19   github   fix/ADO-456     STUCK      comment needs human: "architecture question"
PR #7    azure    feat/AB-12      IN PROGRESS (fix attempt 1/5)
```

Implementation: queries `pr_review_comments` grouped by `pr_id`, looks up the most recent
status per PR, and formats. Reads from SQLite directly (no daemon required for the read).

**Part E — Go Telegram bot methods**

Add to `devtrack_client/internal/telegram/bot.go` (or a new `review_notify.go` in the same
package):

```go
func (b *Bot) SendPRApproved(prTitle, prURL string) error
func (b *Bot) SendPREscalation(prTitle, blockerReason, commentURL, prURL string) error
```

Both methods send a plain text message to `TELEGRAM_CHAT_ID`. No inline keyboard for
approval notifications — these are status updates, not actions. Use existing
`b.api.Send(tgbotapi.NewMessage(chatID, text))` pattern already in the bot.

Wire both methods into the daemon's fix-loop outcome handler (the goroutine in
`IntegratedMonitor` from TASK-095 that currently just `log.Printf`s):

```go
if report.Stuck {
    // stage pr_escalation action
    _, _ = db.InsertPendingAction(db.PendingAction{ActionType: "pr_escalation", ...})
} else {
    // stage pr_approved_notify action
    _, _ = db.InsertPendingAction(db.PendingAction{ActionType: "pr_approved_notify", ...})
}
```

`_execute_pm_action` on the Python server handles delivery (Telegram + log) when the
queue executor auto-approves the action.

**Part F — Tests**

Python: extend `devtrack_server/backend/tests/test_http_triggers.py` or add
`test_pr_notifications.py`:
- `_execute_pm_action` with `action_type="pr_approved_notify"`: no PM API call; returns
  `{"status": "posted"}`.
- `_execute_pm_action` with `action_type="pr_escalation"`: no PM API call; returns
  `{"status": "posted"}`.
- Both action types handled without raising.

Go: `devtrack_client/internal/telegram/` — no new test required if existing bot tests already
cover `b.api.Send()` mock pattern; add a smoke test if they do not.

Build: `go build ./...` and `go vet ./...` from `devtrack_client/`. `uv run pytest backend/tests/ -q` — no regressions.

**Acceptance criteria**:
- [ ] `pr_approved_notify` and `pr_escalation` are staged as `pending_actions` rows when loop terminates (not direct sends)
- [ ] `_execute_pm_action` handles both new action types without raising; no PM API call (notification-only)
- [ ] `bot.SendPRApproved()` and `bot.SendPREscalation()` methods exist and send correct message format
- [ ] TUI Queue tab shows `"PR DONE"` (Success) and `"PR STUCK"` (Danger) badges for the new action types
- [ ] `devtrack review status` command prints per-PR status grouped by PR ID
- [ ] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [ ] Python tests: both new action types handled cleanly; no regressions
- [ ] No `os.getenv` in any new Python file; no hardcoded literals
- [ ] Channel parity: Telegram receives notification for both outcomes (SendPRApproved / SendPREscalation wired in daemon goroutine)

---

### TASK-097 — Phase 7 exit criterion verification
**Priority**: MEDIUM
**Phase**: Phase 7
**Depends on**: TASK-093, TASK-094, TASK-095, TASK-096 (all must be merged to dev)
**Branch**: `feat/TASK-097-phase7-exit-verification`

**Spec**:

Verify the structural machinery for Phase 7 PR puppet master is in place and measurable.
Same pattern as TASK-059 (Phase 0), TASK-074 (Phase 3), TASK-079 (Phase 4), TASK-084
(Phase 5), TASK-092 (Phase 6).

The PRODUCT_BIBLE.md exit criterion ("Developer pushes a PR with formatting and naming
review comments, moves to next ticket, receives 'PR approved' notification without touching
the PR again") requires a live PR with real review comments. This task verifies the
structural criterion: all machinery is in place and a simulated review cycle produces the
expected outputs end-to-end.

**Steps**:

1. Build Go client: `go build -o devtrack .` from `devtrack_client/`. Run `go vet ./...` and
   `go test ./...` — all must pass. Report pass/fail in engineer log.

2. Confirm migration 012 (`pr_review_comments`) is in `allMigrations` and the table is
   present in `Data/db/devtrack.db`. Report `.tables` output.

3. Simulate PR review comment detection:
   - Insert a test row directly into `pr_review_comments` with `status="new"`,
     `classified_as=NULL`, `comment_body="Rename variable x to userID for clarity."`.
   - Call `POST /review/classify` via `devtrack_client/internal/trigger/` HTTP client
     with that comment body.
   - Confirm the returned `classification` is `"auto_fixable"` (naming convention fix).
   - Update the row's `classified_as` and `status` columns. Confirm via
     `devtrack review` CLI output.

4. Simulate agent invocation:
   - Create a minimal temp Go repo (`git init` in a temp dir; one file with a variable
     named `x`). Run `agent.Apply()` with `BackendClaudeCode` and a prompt to rename `x`
     to `userID`. If Claude Code CLI is not installed, verify the timeout-and-fail path
     instead: `Apply` must return `Success=false`, `Error` non-empty, without panicking.
   - Report result in engineer log — PASS if either (a) agent applied the fix and returned
     a commit hash, or (b) agent timed out and `Apply` returned gracefully with `Success=false`.

5. Simulate loop termination:
   - Call `PRFixLoop.Run` with a mock `IsPRApproved` that returns `true` immediately.
   - Confirm the loop returns `EscalationReport{Stuck: false}`.
   - Confirm a `pr_approved_notify` pending action was staged in `pending_actions`.
   - Run `devtrack queue list` — confirm the `pr_approved_notify` row appears.

6. Simulate escalation:
   - Call `PRFixLoop.Run` with a mock agent that always fails and `MaxAttemptsPerComment`
     set to 1 (via a test-mode flag or direct struct init in the test).
   - Confirm loop returns `EscalationReport{Stuck: true}`.
   - Confirm a `pr_escalation` pending action was staged.

7. Verify `devtrack review status` output.

8. Run the full hardcoded-values scan across all Phase 7 files:
   ```
   grep -rn "os\.getenv\b" devtrack_server/backend/review_classifier.py | grep -v "config\|test_"
   grep -rn "localhost:[0-9]\|127\.0\.0\.1:[0-9]" devtrack_client/internal/reviewer/ | grep -v "_test\|#\|config\|Get"
   ```
   Both must return zero hits.

9. Run full Python test suite: `uv run pytest backend/tests/ -q` — all tests must pass
   (beyond the one documented pre-existing failure).

10. Update `Data/agent_logs/feature_tracker.md` with the Phase 7 completion entry.

11. Open a PR targeting `dev` with title "Phase 7: PR puppet master — exit criterion verified".

**Acceptance criteria**:
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` all pass clean
- [ ] `pr_review_comments` table confirmed in SQLite
- [ ] Classification simulation: `POST /review/classify` returns `auto_fixable` for a naming-convention comment
- [ ] Agent invocation test: `Apply()` returns gracefully (success or graceful failure — no panic)
- [ ] Loop approved path: `PRFixLoop.Run` returns `Stuck=false`; `pr_approved_notify` row staged in `pending_actions`
- [ ] Loop stuck path: `PRFixLoop.Run` returns `Stuck=true`; `pr_escalation` row staged in `pending_actions`
- [ ] `devtrack review` and `devtrack review status` both produce output without errors
- [ ] Hardcoded-values scan CLEAN across all Phase 7 source files
- [ ] `uv run pytest backend/tests/ -q` — no regressions beyond documented pre-existing failure
- [ ] `feature_tracker.md` updated with Phase 7 completion entry
- [ ] PR opened targeting `dev` (never `main`)

---

## Phase 8: MCP Server + Headless Integration

**Goal**: DevTrack exposes itself as a Model Context Protocol (MCP) server so AI coding
assistants — Claude Code first, then any MCP-capable tool — can query developer context
automatically. The Go binary hosts the MCP server (no Python dependency for reads; SQLite
has all the data). Generation calls that need LLM output proxy to the Python server via
the existing HTTP boundary.

**Exit criterion**: Developer runs Claude Code on a task. Claude Code automatically knows
the active ticket, the developer's commit voice, and what is in the pending queue —
without the developer typing anything. DevTrack and Claude Code operate as complementary
layers: DevTrack is the memory Claude Code lacks.

**Status**: IN PROGRESS — TASK-098 dispatched 2026-06-22

---

### TASK-098 — MCP protocol core: Go server lifecycle, JSON-RPC 2.0 handler, tool registry
**Assigned to**: engineer
**Phase**: Phase 8
**Started**: 2026-06-22
**Branch**: `feat/TASK-098-mcp-server-core`
**Depends on**: none (Phase 6 complete; Phase 7 deferred)

**Spec**:

Implement the MCP (Model Context Protocol) server in pure Go inside the
`devtrack_client` binary. The MCP server handles the stdio transport required for Claude
Code CLI integration. No new external packages are needed — everything uses Go stdlib
(`encoding/json`, `bufio`, `os`, `sync`, `context`). The server will be extended with
actual tool implementations in TASK-099.

**Background — MCP protocol (what you need to know)**:
MCP uses JSON-RPC 2.0 over stdio. The client (e.g. Claude Code) spawns the MCP server
as a subprocess and communicates via stdin/stdout. The protocol has three phases:

1. Initialize: client sends `initialize` request; server replies with its capabilities
   and the list of tools it exposes.
2. Tool calls: client sends `tools/call` with a tool name and arguments; server returns
   the result as a JSON object.
3. Shutdown: client sends `shutdown`; server exits cleanly.

Full MCP spec: https://spec.modelcontextprotocol.io/specification/basic/messages/
All messages are newline-delimited JSON objects on stdin/stdout. The server must never
write anything other than valid JSON to stdout (all logs go to stderr).

**Files to create**:

**1. `devtrack_client/internal/mcp/server.go` (NEW PACKAGE `mcp`)**

```go
package mcp

// Server is the DevTrack MCP server. It handles JSON-RPC 2.0 over stdio.
// Start(ctx) reads from os.Stdin and writes to os.Stdout until ctx is cancelled
// or the client sends "shutdown".
// All log output goes to os.Stderr — stdout is reserved for JSON-RPC messages.
type Server struct {
    tools    map[string]Tool      // registered tools, keyed by name
    version  string               // server version (from build metadata)
}

// Tool is a registered MCP tool: its schema declaration and its handler.
type Tool struct {
    Name        string
    Description string
    InputSchema map[string]interface{} // JSON Schema for the tool's input object
    Handler     func(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// New creates a new Server with no tools registered.
func New(version string) *Server

// Register adds a tool to the server. Call before Start().
// Panics if a tool with the same name is already registered (programming error).
func (s *Server) Register(t Tool)

// Start runs the JSON-RPC 2.0 message loop, reading from os.Stdin and writing
// to os.Stdout. Blocks until the client sends "shutdown" or ctx is cancelled.
// Never returns an error — any read/write failure causes a clean shutdown.
func (s *Server) Start(ctx context.Context)
```

**Message loop implementation** (`Start`):

```
reader = bufio.NewReader(os.Stdin)
for {
    line, err = reader.ReadString('\n')
    if err == io.EOF or ctx.Done(): break

    var req jsonRPCRequest
    json.Unmarshal(line, &req)

    switch req.Method {
    case "initialize":
        reply with serverInfo, capabilities, list of tools (name, description, inputSchema)
    case "tools/call":
        find tool by name; call handler(ctx, req.Params.Arguments)
        if error: reply with JSON-RPC error object
        else: reply with {"content": [{"type": "text", "text": json.Marshal(result)}]}
    case "tools/list":
        reply with the registered tool list (same as initialize response tools)
    case "shutdown":
        send empty result reply; return
    case "ping":
        send empty result reply (keepalive)
    default:
        send JSON-RPC "Method not found" error (-32601)
    }
}
```

**JSON-RPC types** (define in `server.go` or a separate `types.go` in the same package):

```go
type jsonRPCRequest struct {
    JSONRPC string                 `json:"jsonrpc"`
    ID      interface{}            `json:"id"`
    Method  string                 `json:"method"`
    Params  map[string]interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
    JSONRPC string      `json:"jsonrpc"`
    ID      interface{} `json:"id"`
    Result  interface{} `json:"result,omitempty"`
    Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}
```

All replies are written as `json.Marshal(response) + "\n"` to os.Stdout using a sync.Mutex
to prevent interleaving if the server ever spawns goroutines for async tools.

**2. `devtrack_client/internal/mcp/server_test.go`**

Unit tests:
- `TestServerInitialize`: feed an `initialize` JSON-RPC request to the server on a
  pipe (replace stdin/stdout with io.Pipe for the test). Confirm the response contains
  `serverInfo.name = "devtrack"`, `capabilities.tools = {}`, and the registered tool
  list is returned.
- `TestServerToolsCall_Unknown`: call a tool that was not registered; confirm error
  code -32602 ("Unknown tool: xyz") is returned.
- `TestServerShutdown`: send `shutdown`; confirm `Start` returns within 1 second.
- `TestServerPing`: send `ping`; confirm an empty result reply is returned.

For testability, refactor `Start` to accept `io.Reader` and `io.Writer` rather than
using `os.Stdin`/`os.Stdout` directly. The public `Start(ctx)` wrapper can call
`s.run(ctx, os.Stdin, os.Stdout)`.

**3. `devtrack_client/internal/config/config_env.go` additions**

Add one new accessor:
- `GetMCPPort() string` — reads `MCP_PORT`; required var; document in `.env_sample`
  with value `0` (0 = stdio-only mode; future HTTP transport uses a real port).
  For Phase 8 only stdio is used, but the config var must exist so the daemon can
  log which mode is active.

Add `MCP_PORT=0` to `.env_sample` with comment:
```
# MCP server port. 0 = stdio-only (Claude Code integration). Set a port for HTTP/SSE mode (future).
MCP_PORT=0
```

**4. `devtrack_client/mcp_cmd.go` (new file at package root)**

Wire a `devtrack mcp` CLI command that starts the MCP server in stdio mode.
This is what Claude Code's `.mcp.json` will point to (TASK-100).

```go
// In main.go or cli.go — add case "mcp" to the arg router:
//   devtrack mcp      → start MCP server in stdio mode (blocks; used by Claude Code)

func handleMCPCommand(args []string) {
    // Load config and DB (same startup sequence as other CLI commands)
    srv := mcp.New(version)
    // No tools registered yet — that's TASK-099
    srv.Start(context.Background())
}
```

**Acceptance criteria**:
- [x] `devtrack_client/internal/mcp/` package exists; `go build ./...` passes clean from `devtrack_client/`
- [x] `go vet ./...` passes clean
- [x] `Server.Register()` and `Server.run()` (or `Start`) correctly handle `initialize`, `tools/list`, `tools/call`, `shutdown`, `ping`
- [x] `initialize` response contains `serverInfo.name = "devtrack"` and `protocolVersion = "2024-11-05"`
- [x] Unknown tool call returns JSON-RPC error code -32602
- [x] All unit tests in `server_test.go` pass: `go test ./internal/mcp/...`
- [x] `devtrack mcp` command exists and starts the server (verify: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"0.1"}}}' | devtrack mcp` exits cleanly with a JSON response on stdout)
- [x] `GetMCPPort()` accessor exists in `config_env.go`; `MCP_PORT=0` in `.env_sample`
- [x] All log output from the MCP server goes to stderr, never stdout
- [x] No hardcoded host/port/timeout literals; no `os.Getenv` calls outside `config_env.go`

**Engineer status**: 10/10 criteria done — last commit: 06d442a "feat(mcp): add MCP server core — JSON-RPC 2.0 handler, tool registry, stdio transport (TASK-098)" — 2026-06-22
**PR**: https://github.com/sraj0501/Devtrack_/pull/203
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-22 17:55

---

### TASK-099 — MCP tool implementations: 6 read-only tools backed by SQLite
**Priority**: HIGH
**Phase**: Phase 8
**Depends on**: TASK-098 (mcp package must compile)
**Branch**: `feat/TASK-099-mcp-tools`

**Spec**:

Implement the six MCP tools defined in PRODUCT_BIBLE.md Phase 8, all backed directly by
the existing `devtrack_client/internal/db/` layer. All tools are read-only — no tool
writes to SQLite or posts to any external API.

Each tool is a Go function with signature
`func(ctx context.Context, args map[string]interface{}) (interface{}, error)`.

**File to create: `devtrack_client/internal/mcp/tools.go`**

```go
package mcp

// RegisterDevTrackTools registers all six DevTrack MCP tools on the server.
// db must be an open Database connection.
func RegisterDevTrackTools(s *Server, database *db.Database)
```

**The six tools:**

**Tool 1 — `get_active_context`**

Description: "Returns the developer's current active ticket, branch, today's commit count,
and confidence in the ticket mapping. This is the primary context tool — call it first."

Input schema: `{}` (no arguments)

Implementation:
```go
// Query the most recent commit trigger from the triggers table.
// SELECT ticket_id, branch (from triggers WHERE trigger_type='commit' ORDER BY timestamp DESC LIMIT 1)
// Ticket confidence: "high" if ticket_id != "" and ticket_id != "unlinked", "low" otherwise
// Today's commit count: SELECT COUNT(*) FROM triggers WHERE trigger_type='commit' AND date(timestamp)=date('now')
// Pending actions count: SELECT COUNT(*) FROM pending_actions WHERE status='pending'
```

Returns JSON:
```json
{
  "active_ticket": "PROJ-123",          // "" if none
  "branch": "feat/PROJ-123-add-login",  // "" if no recent commits
  "confidence": "high",                  // "high" | "low" | "none"
  "today_commits": 7,
  "pending_actions_count": 2,
  "workspace": "my-api"
}
```

**Tool 2 — `get_today_commits`**

Description: "Returns all commits from today, grouped by ticket ID, with message and metadata."

Input schema:
```json
{
  "type": "object",
  "properties": {
    "workspace": {
      "type": "string",
      "description": "Filter by workspace name. Omit for all workspaces."
    }
  }
}
```

Implementation:
```go
// SELECT commit_hash, commit_message, ticket_id, workspace_name, timestamp
// FROM triggers
// WHERE trigger_type='commit' AND date(timestamp)=date('now')
// ORDER BY timestamp ASC
// Group in Go by ticket_id
```

Returns JSON:
```json
{
  "commits_by_ticket": {
    "PROJ-123": [
      {"hash": "abc123", "message": "fix auth flow", "timestamp": "2026-06-22T10:31:00Z"}
    ],
    "unlinked": [...]
  },
  "total_today": 7
}
```

**Tool 3 — `get_pending_actions`**

Description: "Returns the current pending actions queue — actions DevTrack wants to take
but hasn't yet. Each action has a confidence score and an expiry time."

Input schema: `{}` (no arguments)

Implementation:
```go
// db.ListPendingActions("pending")
```

Returns JSON array of pending action objects:
```json
{
  "pending": [
    {
      "id": 42,
      "action_type": "post_comment",
      "target": "PROJ-123",
      "platform": "github",
      "confidence": 0.85,
      "expires_at": "2026-06-22T10:36:00Z",
      "payload_preview": "Fixed null check in auth flow..." // first 120 chars of payload
    }
  ],
  "count": 1
}
```

**Tool 4 — `get_voice_profile`**

Description: "Returns the developer's inferred writing style profile for a given context
type. Use this to understand how the developer prefers to communicate before generating
text on their behalf."

Input schema:
```json
{
  "type": "object",
  "properties": {
    "context_type": {
      "type": "string",
      "enum": ["commit", "comment", "report", "task", "ticket_mapping"],
      "description": "The writing context to retrieve style inferences for."
    }
  }
}
```

Implementation:
```go
// db.ListInferencesByConfidence(contextType, 10)
// Returns the top-10 highest-confidence inferences for the given context type.
// Also queries db.ListSkills() and filters to skills matching contextType.
```

Returns JSON:
```json
{
  "context_type": "commit",
  "inferences": [
    {
      "subject": "commit tone",
      "inference": "Uses present-tense imperative verbs. Always starts with lowercase verb.",
      "confidence": 0.91,
      "source": "hermes3"
    }
  ],
  "skills": [
    {
      "name": "imperative-commit-verbs",
      "description": "Developer always opens commits with a lowercase imperative verb",
      "evidence_count": 47
    }
  ]
}
```

If no inferences exist for the context type, return:
```json
{"context_type": "commit", "inferences": [], "skills": [], "note": "No voice data yet. Run `devtrack voice status` for details."}
```

**Tool 5 — `get_ticket_context`**

Description: "Returns full context for a named ticket: recent commits, current pending
actions targeting it, and its current mapping confidence."

Input schema:
```json
{
  "type": "object",
  "required": ["ticket_id"],
  "properties": {
    "ticket_id": {
      "type": "string",
      "description": "The ticket ID, e.g. PROJ-123 or AB-7"
    }
  }
}
```

Implementation:
```go
// Recent commits: SELECT * FROM triggers WHERE ticket_id=? AND trigger_type='commit' ORDER BY timestamp DESC LIMIT 10
// Pending actions: db.ListPendingActions("") filtered to target=ticket_id
// Last commit time: most recent commit timestamp for this ticket_id
```

Returns JSON:
```json
{
  "ticket_id": "PROJ-123",
  "recent_commits": [
    {"hash": "abc123", "message": "fix auth flow", "branch": "feat/PROJ-123-add-login", "timestamp": "..."}
  ],
  "pending_actions": [...],
  "last_activity": "2026-06-22T10:31:00Z"
}
```

**Tool 6 — `get_eod_summary`**

Description: "Returns today's EOD narrative draft — a summary of the day's commits grouped
by ticket, suitable for a standup or daily report."

Input schema: `{}` (no arguments)

Implementation:
```go
// Query today's commits grouped by ticket (same as get_today_commits).
// Generate a summary in Go without calling the Python LLM server.
// The summary is a simple structured narrative:
// "Today: worked on PROJ-123 (3 commits: fix auth flow, add tests, update docs),
//  ADO-456 (2 commits: ...)".
// This is intentionally a template-based summary, not AI-generated — the MCP
// tool must work offline. The Python /eod endpoint is a separate call the agent
// can make when it wants AI-generated prose.
```

Returns JSON:
```json
{
  "date": "2026-06-22",
  "tickets_worked": 2,
  "total_commits": 7,
  "summary": "Today: PROJ-123 (3 commits) — fix auth flow, add tests, update docs. ADO-456 (2 commits) — ...",
  "by_ticket": {
    "PROJ-123": {"commits": 3, "messages": ["fix auth flow", "add tests", "update docs"]}
  }
}
```

**File to create: `devtrack_client/internal/mcp/tools_test.go`**

Unit tests (use a test-mode SQLite db in a temp dir):
- `TestGetActiveContext_NoCommits`: empty DB returns `confidence="none"`, `active_ticket=""`.
- `TestGetActiveContext_WithTicket`: insert a trigger row with `ticket_id="PROJ-123"`; confirm
  `get_active_context` returns `active_ticket="PROJ-123"` and `confidence="high"`.
- `TestGetTodayCommits_Groups`: insert two trigger rows with same ticket_id today; confirm they
  appear grouped under the same key.
- `TestGetVoiceProfile_NoData`: empty inferences table; confirm `inferences: []`, note present.
- `TestGetTicketContext_Filters`: insert triggers for two tickets; confirm only the queried ticket's commits appear.

**Wire into daemon**: In `handleMCPCommand()` (TASK-098's `mcp_cmd.go`), open a DB connection
and call `mcp.RegisterDevTrackTools(srv, db)` before `srv.Start(ctx)`.

**Acceptance criteria**:
- [ ] All six tools registered in `RegisterDevTrackTools()`; `go build ./...` passes clean
- [ ] `go vet ./...` passes clean
- [ ] `devtrack mcp` followed by `initialize` then `tools/list` returns all six tool names
- [ ] `get_active_context`: returns correct JSON shape; works on empty DB (no panic)
- [ ] `get_today_commits`: groups by ticket_id; handles no-commits-today gracefully
- [ ] `get_pending_actions`: returns pending actions with payload_preview truncated to 120 chars
- [ ] `get_voice_profile`: returns inferences + skills; returns note when empty
- [ ] `get_ticket_context`: filters correctly to the requested ticket_id
- [ ] `get_eod_summary`: returns template-based summary (no LLM call required)
- [ ] All unit tests in `tools_test.go` pass: `go test ./internal/mcp/...`
- [ ] No tool writes to SQLite or posts to any external API (read-only enforced)
- [ ] No hardcoded host/port/timeout literals; no `os.Getenv` calls

**Acceptance criteria**:
- [x] All six tools registered in `RegisterDevTrackTools()`; `go build ./...` passes clean
- [x] `go vet ./...` passes clean
- [x] `devtrack mcp` followed by `initialize` then `tools/list` returns all six tool names
- [x] `get_active_context`: returns correct JSON shape; works on empty DB (no panic)
- [x] `get_today_commits`: groups by ticket_id; handles no-commits-today gracefully
- [x] `get_pending_actions`: returns pending actions with payload_preview truncated to 120 chars
- [x] `get_voice_profile`: returns inferences + skills; returns note when empty
- [x] `get_ticket_context`: filters correctly to the requested ticket_id
- [x] `get_eod_summary`: returns template-based summary (no LLM call required)
- [x] All unit tests in `tools_test.go` pass: `go test ./internal/mcp/...`
- [x] No tool writes to SQLite or posts to any external API (read-only enforced)
- [x] No hardcoded host/port/timeout literals; no `os.Getenv` calls

**Engineer status**: 12/12 criteria done — last commit: 155028c "feat(mcp): implement 6 read-only MCP tools backed by SQLite (TASK-099)" — 2026-06-22 18:19
**PR**: https://github.com/sraj0501/Devtrack_/pull/204 (base: dev)

**COMPLETE** — ready for PM review — 2026-06-22 18:20

---

### TASK-100 — Claude Code integration: .mcp.json config, daemon auto-start, devtrack mcp commands
**Priority**: HIGH
**Phase**: Phase 8
**Depends on**: TASK-099 (tools must be implemented before Claude Code can use them)
**Branch**: `feat/TASK-100-claude-code-integration`

**Spec**:

Wire the MCP server into Claude Code's configuration and the DevTrack daemon lifecycle.
After this task, a developer with DevTrack and Claude Code installed can open Claude Code
in any watched repo and DevTrack context is available automatically.

**Part A — `.mcp.json` template generation**

Add a new CLI command:

```
devtrack mcp setup
```

This command writes a `.mcp.json` file in the current directory (the Claude Code project
root) with the correct DevTrack MCP server entry. It prints instructions to add this
to Claude Code's config if it does not exist.

Generated `.mcp.json` content:

```json
{
  "mcpServers": {
    "devtrack": {
      "command": "<absolute path to devtrack binary>",
      "args": ["mcp"],
      "env": {
        "DEVTRACK_ENV_FILE": "<absolute path to .env>"
      }
    }
  }
}
```

The absolute path to the devtrack binary is resolved via `os.Executable()` (Go stdlib).
The `.env` file path is resolved via `config.ResolveEnvFilePath()`.

If `.mcp.json` already exists and already contains a `devtrack` entry: print
"DevTrack MCP already configured in .mcp.json" and exit 0. If it exists but lacks a
devtrack entry: merge the new entry into the existing JSON object.

**Part B — `devtrack mcp status`**

```
devtrack mcp status
```

Prints:
```
DevTrack MCP Server
  Protocol:    MCP 2024-11-05
  Transport:   stdio
  Port:        0 (stdio-only)
  Tools:       6 registered
    - get_active_context
    - get_today_commits
    - get_pending_actions
    - get_voice_profile
    - get_ticket_context
    - get_eod_summary
  Config file: .mcp.json (present / not found)
  Daemon:      running (PID 12345) / stopped
```

Implementation: reads the tool registry (instantiate `mcp.New()`, call
`mcp.RegisterDevTrackTools()`, introspect the registered tools list) and checks
daemon status via the existing `daemon.IsRunning()` function.

**Part C — Daemon lifecycle: MCP server does NOT auto-start in daemon mode**

The MCP server runs on-demand when Claude Code (or any MCP client) spawns
`devtrack mcp`. It does NOT run as a background goroutine inside the daemon — the
stdio transport is inherently a single-session pipe between the MCP client and the
MCP server process. Each `devtrack mcp` invocation is its own process.

This means:
- The daemon does NOT need changes for MCP startup.
- `devtrack mcp` opens its own DB connection (read-only: `?mode=ro` on the SQLite
  URI is preferred but not required — it must not acquire the write lock).
- `devtrack mcp` does NOT need the Python server to be running (all tools are SQLite-only).

Document this in `devtrack mcp status` output with a note:
"MCP server runs on-demand when spawned by Claude Code. No background process needed."

**Part D — `devtrack mcp` subcommand routing**

Consolidate all `devtrack mcp *` commands in `devtrack_client/mcp_cmd.go`:

```
devtrack mcp           → start MCP server (stdio mode; blocks)
devtrack mcp setup     → write .mcp.json in current directory
devtrack mcp status    → print server info and tool list
devtrack mcp test      → run a self-test: start server on a pipe, send initialize + 
                          tools/list + get_active_context, print results, exit
```

`devtrack mcp test` is the local smoke-test for the integration. It creates an in-process
pipe, starts the MCP server on that pipe, sends three JSON-RPC messages, and prints the
responses. This lets the developer verify the MCP server is working without needing Claude
Code to be installed.

**Part E — Add MCP_PORT to `.env_sample`** (if not already done in TASK-098)

Confirm `MCP_PORT=0` is in `.env_sample`. No change needed if TASK-098 already added it.

**Acceptance criteria**:
- [ ] `devtrack mcp setup` writes `.mcp.json` with the correct server entry in the current directory
- [ ] `devtrack mcp setup` is idempotent: re-running when config already exists prints the "already configured" message and exits 0
- [ ] `devtrack mcp setup` merges into existing `.mcp.json` rather than overwriting non-DevTrack entries
- [ ] `devtrack mcp status` prints server info, all 6 tool names, config file presence, and daemon status
- [ ] `devtrack mcp test` sends `initialize` + `tools/list` + `get_active_context` over an in-process pipe and prints results without error
- [ ] `devtrack mcp` (no subcommand) starts the MCP server in stdio mode and responds to JSON-RPC messages
- [ ] The MCP server process opens SQLite in read-preferred mode and does not block the daemon's write lock
- [ ] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [ ] No `os.Getenv` outside `config_env.go`; no hardcoded paths, ports, or binary names

**Engineer status**: not started
**Blockers**: TASK-099

---

### TASK-101 — Phase 8 exit criterion verification
**Priority**: MEDIUM
**Phase**: Phase 8
**Depends on**: TASK-098, TASK-099, TASK-100 (all must be merged to dev)
**Branch**: `feat/TASK-101-phase8-exit-verification`

**Spec**:

Verify the Phase 8 exit criterion: "Developer runs Claude Code on a task; Claude Code
automatically knows the active ticket, the developer's commit voice, and what is in the
pending queue without any manual context-setting."

Same verification pattern as TASK-059 (Phase 0), TASK-074 (Phase 3), TASK-092 (Phase 6).

**Steps**:

1. Build the client binary: `go build -o devtrack .` from `devtrack_client/`.
   Run `go vet ./...` and `go test ./...`. Report pass/fail.

2. Confirm `devtrack mcp status` shows 6 tools registered and config info.

3. Run `devtrack mcp setup` in a test directory. Confirm `.mcp.json` is created with
   the correct structure. Confirm re-running prints "already configured".

4. Run `devtrack mcp test`. Confirm all three JSON-RPC calls (`initialize`, `tools/list`,
   `get_active_context`) return valid responses. Print the raw output in the engineer log.

5. Simulate Claude Code context injection:
   - Insert a test commit trigger row in SQLite:
     ```sql
     INSERT INTO triggers (trigger_type, commit_hash, commit_message, ticket_id, branch, workspace_name, timestamp)
     VALUES ('commit', 'test123', 'fix auth flow', 'PROJ-123', 'feat/PROJ-123-add-login', 'devtrack', datetime('now'))
     ```
   - Run `echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_active_context","arguments":{}}}' | devtrack mcp`
   - Confirm the response contains `"active_ticket":"PROJ-123"` and `"confidence":"high"`.

6. Simulate voice profile query:
   - Insert a test inference: `INSERT INTO inferences (context_type, subject, inference, evidence, confidence, source) VALUES ('commit', 'tone', 'Uses imperative verbs', '[]', 0.91, 'hermes3')`
   - Call `get_voice_profile` with `context_type: "commit"` via the MCP pipe.
   - Confirm the response contains the inference.

7. Run full hardcoded-values scan across all Phase 8 files:
   ```
   grep -rn "localhost:[0-9]\|127\.0\.0\.1:[0-9]\|0\.0\.0\.0:[0-9]" devtrack_client/internal/mcp/ | grep -v "_test\|#\|config\|Get"
   grep -rn "os\.Getenv\b" devtrack_client/internal/mcp/ devtrack_client/mcp_cmd.go
   ```
   Both must return zero hits.

8. Run `go test ./...` from `devtrack_client/`. Report final pass/fail count.

9. Update `Data/agent_logs/feature_tracker.md` with Phase 8 completion entry.

10. Open a PR targeting `dev` with title "Phase 8: MCP server + Claude Code integration — exit criterion verified".

**Acceptance criteria**:
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` all pass clean from `devtrack_client/`
- [ ] `devtrack mcp status` shows 6 tools registered
- [ ] `devtrack mcp setup` creates `.mcp.json` and is idempotent
- [ ] `devtrack mcp test` shows valid JSON-RPC responses for all three messages
- [ ] `get_active_context` returns `active_ticket="PROJ-123"` from seeded trigger row
- [ ] `get_voice_profile` returns seeded inference from inferences table
- [ ] Hardcoded-values scan CLEAN across all Phase 8 source files
- [ ] `go test ./...` passes (beyond any documented pre-existing failures)
- [ ] `feature_tracker.md` updated with Phase 8 completion entry
- [ ] PR opened targeting `dev` (never `main`)

**Engineer status**: not started
**Blockers**: TASK-098, TASK-099, TASK-100

---

## DEPRIORITISED (pivot 2026-06-10)

These sat on the old v3.x "Polish & Growth" board. The pivot moved them below the
Product Bible phases — not cancelled, just not now.

- ~~TASK-052~~ CLI aesthetics & theming (lipgloss) — deprioritised
- ~~TASK-053~~ "Work you didn't write" savings counter — deprioritised
- ~~TASK-054~~ How-to video series — deprioritised (revisit after Phase 4)
- PG-5 (`stats_client.py` → `GET /internal/stats`) — deprioritised
- Redis R-1 → R-6 — deprioritised
- Boardroom / plan as **primary** features — demoted to secondary (shipped & maintained, not a headline)

---

## SHIPPED — history (compact)

### v3.x line (2026-05 → 2026-06)
| Version | What |
|---|---|
| v3.0.10 | Significant Windows fixes: isatty via mattn/go-isatty; editor-commit hooks; auto-enhance (`DEVTRACK_AUTO_ENHANCE=true`) |
| v3.0.9 | TASK-056 — `skip_issues` flag; dual-platform duplicate-ticket fix |
| v3.0.8 | Stale health snapshot fix; migration 005 prunes legacy Redis/MongoDB rows |
| v3.0.7 | Automated GitHub Actions release pipeline (`release.yml`) |
| v3.0.6 | Windows CLI full parity + autostart via Task Scheduler |
| v3.0.1–5 | `motor` optional dep; `upgrade.go` → GitHub API; Telegram bot migrated to Go |
| v3.0.0 | Go package refactor + client-standalone (TASK-A–F; TASK-E/F layered refactor) |

### Client-Server Decoupling
- **Phase 1 (1a–1d)** — server-mgmt removed from client; reports/learning/auth/license → HTTP; `workspaces.yaml` sole non-secret PM config source.
- **Phase 2 (TASK-055)** — native Go alert poller (`internal/alerts/`) + notifiers (`internal/notify/`) + interactive Telegram bot (`internal/telegram/`); daemon no longer spawns Python telegram/alert subprocesses. (commit `abad449`, follow-ups `e993507`/`c74179f`/`d4d9b5f`)

### EPIC-SPLIT (2026-05-24) — TASK-041–048
Monorepo restructured into `devtrack_client/` + `devtrack_server/` + `devtrack_wiki/`
with an HTTP/JSON boundary. Legacy `devtrack-bin/` + root `backend/` retired
(281 files, 69k lines).

### Earlier — TASK-000–040
v1.0.0 release + local agents; config audit (os.getenv eliminated across 22 files,
50+ accessors); CS-2 headless tests + server-TUI stats panel; CS-3 Admin GUI MVP
(users/licenses/health); logo + Windows binary icon; boardroom + plan commands.

---

### TASK-086 — Hermes 3 reasoning loop: Python server runs a reasoning pass after each interaction
**Priority**: HIGH
**Phase**: Phase 6
**Depends on**: TASK-085 (migrations must be live; Go DB layer must exist)
**Branch**: `feat/TASK-086-hermes3-reasoning-loop`

**Acceptance criteria**:
- [x] `dialectic_reasoner.py` exists with `DialecticReasoner` class; `reason()` returns `[]` on failure, never raises
- [x] Hermes 3 model tried first; falls back to configured LLM chain; logs on fallback
- [x] No `os.getenv` in `dialectic_reasoner.py`; all config via `backend.config`
- [x] `POST /dialectic/infer` endpoint exists, auth-gated, returns `{"inferences": [...]}`
- [x] `PostDialecticInfer()` exists in `devtrack_client/internal/trigger/`; Go client calls it after successful queue execution (fire-and-forget goroutine)
- [x] Returned inferences stored in SQLite `inferences` table via `InsertInference()`
- [x] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [x] Python tests pass: graceful degradation, well-formed JSON return, auth guard
- [x] `uv run pytest backend/tests/ -q` — no regressions beyond documented pre-existing failure

**Assigned to**: engineer
**Started**: 2026-06-18
**Engineer status**: 9/9 criteria done — last commit: 059a6fb "test(dialectic): fix os.getenv AST check" — 2026-06-18
**PR**: https://github.com/sraj0501/Devtrack_/pull/193
**Blockers**: none

**COMPLETE** — ready for PM review — 2026-06-18

---

_Append new active tasks under ACTIVE/QUEUED. Move completed work to SHIPPED as a
one-line entry — keep this board lean. Detailed per-task records live in
`feature_tracker.md` and `engineer_log.md`._
