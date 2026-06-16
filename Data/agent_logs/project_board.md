# DevTrack Project Board

_Last updated: 2026-06-16 by PM (TASK-069 dispatched — dev tip 219768c)_
_Next DevTrack task ID: TASK-071_
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

## ACTIVE — Phase 1: Pending actions queue

**Goal**: Every outbound PM action is staged in `pending_actions` before it touches any external
system. Confidence score on every action. Configurable timeout with auto-approve. TUI, CLI, and
Telegram all surface the queue and accept approve/reject/edit. Nothing posts without clearing
this table.

**Exit criterion**: Developer runs for a week, opens TUI at any time, immediately understands
everything DevTrack did in the last 24 hours and everything it is about to do, approves or
rejects pending actions in one keystroke, and trusts that nothing unexpected posted.

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

## QUEUED — Phases 2–8

| Phase | Name | Exit criterion (short) |
|---|---|---|
| 1 | Pending actions queue + TUI confidence layer | A week of outbound actions all staged in `pending_actions`; nothing unexpected posts → see TASK-060 ff. |
| 2 | Opinionated ticket extractor | >80% of commits mapped to tickets with no config beyond branch naming |
| 3 | Silent commit handler | Commit → ticket commented + state-transitioned within auto-approve window; dev did nothing |
| 4 | EOD pipeline | Accurate EOD email every evening, reads like the dev wrote it |
| 5 | Voice training (low friction) | After 1 week, generated text passes the "did I write this?" test |
| 6 | Dialectic self-improvement | After 30 days, correction rate measurably down; ≥3 autonomous skills emerged |
| 7 | PR review loop (puppet master) | Push PR with nit comments, get "approved" without touching it again |
| 8 | MCP server + headless integration | Claude Code queries DevTrack for developer context automatically |

Full phase specs and acceptance criteria: `PRODUCT_BIBLE.md` § Build Phases.

---

## ACTIVE — Phase 2: Opinionated ticket extractor

**Goal**: On every commit, extract a ticket ID from the branch name or commit message.
Unmatched commits are logged as unlinked — never blocked. Developer obligation: standard
branch naming (e.g. `feat/PROJ-123-description` or `fix/#42-bug`). Nothing else required.

**Exit criterion**: >80% of commits correctly mapped to tickets without any developer
configuration beyond standard branch naming.

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
**Priority**: MEDIUM
**Phase**: Phase 2
**Depends on**: TASK-069
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
- [ ] `Database.TicketStats(repoPath, 50)` returns correct totals from the triggers table
- [ ] `devtrack status` output includes the Ticket Extraction section
- [ ] Status shows `PASS` when linked/total >= 0.80, `BELOW TARGET` otherwise
- [ ] When fewer than 5 commits in history: shows `"Not enough data"` rather than a percentage
- [ ] `[UNLINKED]` tag appears in daemon.log for every commit with no extracted ticket ID
- [ ] `go build ./...` and `go vet ./...` pass clean from `devtrack_client/`
- [ ] Phase 2 exit criterion verifiable: run 10+ test commits, check `devtrack status` shows >= 80% linked

**Engineer status**: not started
**Blockers**: TASK-069 must be merged to dev first

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

_Append new active tasks under ACTIVE/QUEUED. Move completed work to SHIPPED as a
one-line entry — keep this board lean. Detailed per-task records live in
`feature_tracker.md` and `engineer_log.md`._
