# Plan: Decouple `devtrack_client` from the Python backend (client/server split)

> Companion to [CAPABILITIES_OWNERSHIP.md](CAPABILITIES_OWNERSHIP.md), which is
> the editable source of truth for what each side owns. This file is the
> execution plan for closing the gaps that doc marks with ⚠.
>
> _Status: **Phase 1 complete** (branch: `feat/client-server-decoupling`). Phase 2 pending._
> _Phase 1a+1b: d5f8f36. Phase 1c+1d: 3dd17f8. Connector config refactor: 8d3113a._
> _Workspace git-init helper: ed11bba. Help/status rewrite: e3726e0._

## Context

The `devtrack_client` is meant to be a standalone Go binary whose only link to
`devtrack_server` is the HTTP/JSON boundary (CLAUDE.md: "the only interface is
HTTPS POST to `/trigger/*` … no shared compiled artefact"). In reality the
client still invokes the Python backend directly in ~15 places via
`uv run python -m backend.*`, and the daemon spawns backend feature modules as
subprocesses. This couples the client to a Python install + `PROJECT_ROOT`,
contradicts the standalone-client goal, and mixes server-only programs
(server-tui, admin GUI) into the client.

**Ownership model (decided with the user):**
- **Client owns:** ticket fetching / caching / matching / managing / updating;
  notification delivery (telegram/slack); the git flow; local UX. The client
  reaches every *server* capability over HTTP — it never embeds backend logic.
- **Server owns:** AI / NLP / LLM enhancement; the learning suite; AI-enhanced
  report generation. Server-management tools (server-TUI, admin GUI) are
  server-only and have no place in the client.
- **Managed mode** is a deploy/bootstrap convenience (download server components,
  deploy server + client, optionally launch the `webhook_server` entry). It must
  **not** carry/host per-feature backend logic.

**Sequencing:** This lands on a **separate branch**, *after* the current
uncommitted git-flow work (ticket creation, smart matching, deferred commits) is
E2E-tested and committed. Do not entangle the two.

## Outcome

After Phase 1, the only `python`/`uv` reference left in the client is the
managed-mode `webhook_server` launch. After Phase 2, alerts + telegram/slack are
native Go and the daemon spawns no per-feature Python modules.

---

## Phase 1 completion summary (2026-05-31)

All four sub-phases landed on `feat/client-server-decoupling`.

| Sub-phase | Commit | What was done |
|---|---|---|
| 1a | d5f8f36 | Deleted `server-tui` and `admin-start` commands from the client CLI. |
| 1b | d5f8f36 | Removed redundant `github_ticket_sync.py` invocation; routes to Go-native `handleGitHubSync`. |
| 1c | 3dd17f8 | Reports, learning, auth, and license commands now call `devtrack_server` over HTTP (`/reports/*`, `/learning/*`, `/auth/*`, `/license/*`). New HTTP methods added to `devtrack_client/internal/trigger/http_trigger.go`; corresponding FastAPI routes added to `devtrack_server/backend/webhook_server.py`. Only remaining `uv run` calls in the client: managed-mode `webhook_server` launch; `backend.alert_poller` and `backend.telegram`/`slack` (Phase 2). |
| 1d | 3dd17f8 | Daemon comments updated to state managed mode = deploy + run server, not feature host. |
| extra | 8d3113a | `workspaces.yaml` is now the sole non-secret PM connector config source. `.env` holds secrets only (`GITHUB_TOKEN`, `GITLAB_PAT`, `AZURE_DEVOPS_PAT`). All connector `NewClient()` signatures take explicit workspace params; free functions converted to methods on `*Client`. Single-repo env-var fallback removed. |
| extra | ed11bba | `devtrack workspace add` now offers to run `git init` + initial commit when the path is not a git repo (`offerGitInit()` in `cli_workspace.go`; also called from the setup wizard). |
| extra | e3726e0 | `devtrack help` fully rewritten. `devtrack status` now shows workspaces list, PM token presence, and AI server ping. |

**Verification**: `cd devtrack_client && go build ./... && go vet ./... && go test ./...` green after every commit above.

---

## Phase 1 — Remove server-mgmt commands + convert server-owned features to HTTP

Two-sided: add HTTP endpoints in `devtrack_server` (thin wrappers over existing
modules) and convert the client commands to thin callers.

### 1a. Remove server-management tools from the client (pure deletions) ✅ d5f8f36
- `server-tui`, `admin-start`/`admin` — delete the routes in
  `devtrack_client/cli.go` and the handlers in `devtrack_client/cli_work.go`
  (the `uv run … backend.server_tui` / `backend.admin` calls). Update help text
  in `cli_info.go`. These are managed only on the server.

### 1b. Delete redundant Python ticket-sync (client already has Go-native sync) ✅ d5f8f36
- Remove the `backend/github_ticket_sync.py` invocation in `cli.go` (~line 479)
  and route any sync entry to the existing Go-native `github-sync`
  (`connectors/github/sync.go`, `handleGitHubSync`).

### 1c. Convert server-owned, user-facing commands to HTTP ✅ 3dd17f8
Server side — add FastAPI routes in `devtrack_server/backend/webhook_server.py`
that wrap existing modules (no new logic):
- `POST /reports/preview|send|save` → `backend/email_reporter.py`,
  `backend/daily_report_generator.py`, `backend/work_tracker/eod_report_generator.py`.
- `POST /learning/{enable,sync,status,reset,cron,...}`, `GET /learning/profile`,
  `POST /learning/test-response`, `POST /learning/revoke` → `backend/learning_integration.py`, `backend/personalized_ai.py`.
- `POST /auth/login|logout`, `GET /auth/whoami` → `backend/auth/cloud_auth.py`,
  `backend/auth/session.py`; `GET /license`, `POST /license/accept` →
  `backend/license_manager.py`.

Client side — replace each `uv run python` handler with a thin call using the
existing `HTTPTriggerClient` (`devtrack_client/http_trigger.go`, already used by
`cli_boardroom.go`/`cli_plan.go` via `NewHTTPTriggerClient`):
- `cli_reports.go` (`preview/send/save-report`, `send-summary`).
- `internal/learning/learning.go` + `internal/learning/license.go` +
  `learning_shim.go` + `license_cli.go` (learning suite, login/logout/whoami,
  license/terms).
- `internal/infra/scheduler.go` EOD job (~line 394) → call the `/reports` HTTP
  endpoint instead of `uv run`.
- Add the new methods (`SendReportPreview`, `Learning*`, `AuthLogin`, …) to
  `HTTPTriggerClient` in `http_trigger.go`, mirroring `SendBoardroom`.

All client commands degrade gracefully when `DEVTRACK_SERVER_URL` is
unreachable (clear message), the same way triggers/boardroom already do.

### 1d. Managed-mode reframe (minimal in Phase 1) ✅ 3dd17f8
- `internal/daemon/daemon.go`: keep the `webhook_server` launch as the
  managed/deploy convenience (already gated by `DEVTRACK_SERVER_MODE=external`).
  Leave the `telegram`/`slack`/`alert_poller` subprocess spawns **untouched in
  Phase 1** (they're replaced in Phase 2 to avoid a functionality gap). Update
  comments to state managed mode = deploy + run server, not feature host.

---

## Phase 2 — Native-Go ports of client-owned features ✅ COMPLETE (2026-05-31)

### 2a. Ticket alerts → native Go ✅
- `internal/alerts/` — `Poller` goroutine polls GitHub (`ListNotificationsSince` via
  GitHub Notifications API) and Azure (`ListWorkItemsChangedAfter` WIQL delta) on
  `ALERT_POLL_INTERVAL_SECS` (default 300 s). Writes new records to SQLite
  `notifications` table via `InsertNotificationNew`; reads `alert_state` for delta tracking.
- `handleAlerts` in `cli.go` reads directly from SQLite (no Python call).
  `--all` / `--clear` flags retained.
- Daemon starts `alerts.Poller` via `startAlertPoller()` — no subprocess.
- Jira: out of scope (no Go Jira connector); alerts continue to be absent for Jira.

### 2b. Telegram / Slack delivery → native Go ✅
- `internal/notify/` — `Notifier` interface; `Multi` fan-out; `Terminal` stdout;
  `Telegram` (Bot API POST `/sendMessage`); `Slack` (incoming webhook POST);
  `OS` (platform-split: PowerShell balloon on Windows, osascript/notify-send on Unix).
- `NewTelegramFromConfig()` / `NewSlackFromConfig()` return nil when unconfigured — safe to pass to `Multi`.
- Daemon no longer spawns `backend.telegram`, `backend.slack`, `backend.azure.assignment_poller`,
  or `backend.gitlab.assignment_poller`. All four subprocess fields and start/restart
  functions removed from `internal/daemon/daemon.go`.
- Health monitor: `checkTelegramBot()` now checks `TELEGRAM_BOT_TOKEN` + `TELEGRAM_CHAT_ID`
  are set (config-based) rather than checking a subprocess PID.

---

## Reuse (do not reinvent)
- `devtrack_client/http_trigger.go` `HTTPTriggerClient` + `NewHTTPTriggerClient`
  — the established client→server HTTP path (boardroom/plan use it).
- Go connectors `connectors/{github,gitlab,azure}` — already provide ticket
  fetching/listing/commenting/creating for the Phase 2 alerter.
- `internal/db` `ticket_cache` + SQLite — storage for Phase 2 alerts.
- Server modules already exist (`email_reporter`, `daily_report_generator`,
  `learning_integration`, `license_manager`, `auth/*`); Phase 1 only wraps them
  in HTTP routes.

## Verification
1. After each Phase-1 change: `cd devtrack_client && go build ./... && go vet ./... && go test ./...` stays green.
2. **No leakage:** `grep -rn "uv run\|python -m backend\|backend/.*\.py" devtrack_client --include=*.go` returns only the managed-mode `webhook_server` launch (Phase 1) and nothing (Phase 2).
3. **Server endpoints:** start `devtrack_server`; `curl` each new route
   (`/reports/preview`, `/learning/status`, `/auth/whoami`, `/license`) and
   confirm 200 + expected payload.
4. **Client→server E2E (external mode):** with the server running and
   `DEVTRACK_SERVER_URL` set, run `devtrack preview-report`, `devtrack
   learning-status`, `devtrack whoami` and confirm they hit the server and
   render. With the server stopped, confirm a clean "server unreachable" message
   (no `uv`/Python error).
5. **Removed commands:** `devtrack server-tui` / `devtrack admin-start` are gone
   from help and routing.
6. **Phase 2:** unit-test the Go alerter against a temp/mocked connector;
   integration-test telegram/slack delivery against a test bot/webhook; confirm
   the daemon no longer spawns `telegram`/`slack`/`alert_poller`.

## Out of scope / notes
- This plan does not change the git-flow work already done on
  `feat/go-native-git-commit-flow`; that is committed first, separately.
- `boardroom`, `plan`, commit/timer triggers already use the HTTP boundary — no
  change.
- A native Go Jira connector is optional (Phase 2 decision); without it, Jira
  alerts remain server-side or unsupported and must be documented.
