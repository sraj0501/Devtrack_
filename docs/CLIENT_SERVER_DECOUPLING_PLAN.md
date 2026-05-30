# Plan: Decouple `devtrack_client` from the Python backend (client/server split)

> Companion to [CAPABILITIES_OWNERSHIP.md](CAPABILITIES_OWNERSHIP.md), which is
> the editable source of truth for what each side owns. This file is the
> execution plan for closing the gaps that doc marks with ⚠.
>
> _Status: Phase 1 complete (branch: `feat/client-server-decoupling`). Phase 2 pending._
> _Phase 1a+1b committed d5f8f36. Phase 1c+1d committed on the same branch (see git log)._

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

## Phase 1 — Remove server-mgmt commands + convert server-owned features to HTTP

Two-sided: add HTTP endpoints in `devtrack_server` (thin wrappers over existing
modules) and convert the client commands to thin callers.

### 1a. Remove server-management tools from the client (pure deletions)
- `server-tui`, `admin-start`/`admin` — delete the routes in
  `devtrack_client/cli.go` and the handlers in `devtrack_client/cli_work.go`
  (the `uv run … backend.server_tui` / `backend.admin` calls). Update help text
  in `cli_info.go`. These are managed only on the server.

### 1b. Delete redundant Python ticket-sync (client already has Go-native sync)
- Remove the `backend/github_ticket_sync.py` invocation in `cli.go` (~line 479)
  and route any sync entry to the existing Go-native `github-sync`
  (`connectors/github/sync.go`, `handleGitHubSync`).

### 1c. Convert server-owned, user-facing commands to HTTP
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

### 1d. Managed-mode reframe (minimal in Phase 1)
- `internal/daemon/daemon.go`: keep the `webhook_server` launch as the
  managed/deploy convenience (already gated by `DEVTRACK_SERVER_MODE=external`).
  Leave the `telegram`/`slack`/`alert_poller` subprocess spawns **untouched in
  Phase 1** (they're replaced in Phase 2 to avoid a functionality gap). Update
  comments to state managed mode = deploy + run server, not feature host.

---

## Phase 2 — Native-Go ports of client-owned features (later)

### 2a. Ticket alerts → native Go
- New `internal/alerts/` (Go): poll GitHub/Azure/Jira for assigned/comment/
  state-change events, **reusing** `connectors/{github,gitlab,azure}`
  (`ListIssues`/`ListWorkItems`, `View*`) and persisting to **SQLite**
  (`internal/db`, alongside `ticket_cache`) instead of MongoDB. Go notifier
  (OS notification + terminal).
- Rewire `handleAlerts` (`cli.go`) to read from SQLite (Go), remove the
  `backend.alert_poller` call; run polling from the daemon scheduler
  (`internal/infra/scheduler.go`) like the existing PM-queue flusher / deferred
  enhancer. Remove the `alert_poller` daemon spawn.
- Jira note: a Go Jira connector does not exist yet; either add a minimal one or
  keep Jira alerts out of scope (document it).

### 2b. Telegram / Slack delivery → native Go
- New `internal/notify/` (Go): send notifications via the Telegram Bot API /
  Slack webhook over HTTP (simple `net/http`). Wire into the alert notifier.
- Remove `backend.telegram` / `backend.slack` daemon spawns
  (`internal/daemon/daemon.go`) and the `telegram-status` Python call. Any
  AI-conversational telegram processing (if ever needed) calls the server over
  HTTP — not implemented client-side.

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
