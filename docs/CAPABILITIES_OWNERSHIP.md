# DevTrack — Capabilities & Ownership

This file is the **single editable source of truth** for which capabilities are
owned by the **client** (`devtrack_client`, Go) vs the **server**
(`devtrack_server`, Python). Edit the **Owner** column to reassign a capability;
the **Current state** column records how it is implemented *today* so you can see
where reality diverges from the intended ownership.

_Last updated: 2026-06-10 (client-server decoupling Phase 1 + Phase 2 complete)._

## Ownership model (intended)

- **Client owns:** ticket fetching / caching / matching / managing / updating;
  the git flow; notification **delivery** (telegram/slack); local UX, daemon
  lifecycle, workspace/deploy. The client reaches every *server* capability over
  HTTP — it never embeds backend logic.
- **Server owns:** AI / NLP / LLM enhancement pipeline; the learning &
  personalization suite; AI-enhanced report generation; server-management tools
  (server-TUI, admin GUI); webhook ingestion.
- **Managed mode** = deploy/bootstrap convenience (download + run server + client).
  It must not *host* per-feature backend logic in the client.

## Legend

- **Owner**: `Client` / `Server` / `Shared` — edit this to reassign.
- **Current state**:
  - `Go-native` — implemented in Go in the client, no Python.
  - `Python (in client)` — client shells out to `uv run python -m backend.*`
    **(coupling to remove — see decoupling plan)**.
  - `HTTP → server` — client calls the server over HTTP (correct boundary).
  - `Python (server)` — runs in the server process.
- **⚠ Mismatch** in Notes = current state does not match intended Owner.

---

## 1. Daemon lifecycle & process control — Owner: **Client**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Start/stop/restart daemon | `start`, `stop`, `restart` | Go-native | Client | |
| Status / logs | `status`, `logs` | Go-native | Client | `logs --follow` not implemented (stub) |
| Pause/resume/skip triggers | `pause`, `resume`, `skip-next`, `force-trigger`, `on`, `off` | Go-native | Client | |
| Config reload | `reload`, `reload-config`, `settings` | Go-native | Client | |
| DB stats / version / help | `db-stats`, `version`, `help` | Go-native | Client | |
| Telemetry ping | `telemetry` | Go-native | Client | |

## 2. Git flow & AI commit enhancement — Owner: **Client**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| AI-enhanced commit (A/E/R/Q/C) | `git commit` | Go-native (gitsage → LLM) | Client | git-sage is client-owned; calls Ollama/OpenAI **directly** from the client. ⚠ If you want LLM calls routed via the server, change Owner→Server. |
| Stage / history / passthrough | `git add`, `git history`, `git <any>` | Go-native | Client | |
| git-sage agent | `sage`, `ask`, `do`, `interactive` | Go-native (bundled Python git_sage) | Client | git_sage is **client-owned** Python bundled with the client (not the server). |
| Deferred (offline) commits | `commits pending/review/enhance`, `commit-queue` | Go-native | Client | Durable snapshot + 3-way apply |

## 3. Ticket management (PM connectors) — Owner: **Client**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| GitHub issues: check/list/sync/view | `github-check/list/sync/view` | Go-native | Client | `connectors/github` |
| GitLab issues: check/list/sync/view | `gitlab-check/list/sync/view` | Go-native | Client | `connectors/gitlab` |
| Azure work items: check/list/sync/view | `azure-check/list/sync/view` | Go-native | Client | `connectors/azure` |
| Ticket comment / create | (in commit flow) | Go-native | Client | `connectors/*/comment.go`, `create.go` |
| Offline ticket cache | (in commit flow) | Go-native (SQLite) | Client | `ticket_cache` table |
| Smart matching + likelihood | (in commit flow) | Go-native (fuzzy + optional Ollama embeddings) | Client | `internal/match` |
| Jira | — | **None in client** | Server | ⚠ No Go Jira connector; Jira handled only server-side today |
| GitHub ticket sync (legacy) | (internal) | Go-native | Client | ✅ Removed Python `github_ticket_sync.py` call (Phase 1b — d5f8f36); routes to Go-native `github-sync` |

## 4. Work tracking & workspaces — Owner: **Client**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Work sessions | `work` | Go-native (SQLite) | Client | |
| Vacation mode | `vacation` | Go-native | Client | |
| Workspace management | `workspace`, `is-workspace`, `list`, `enable`, `disable`, `remove` | Go-native | Client | |
| Git integration / hooks | `enable-git`, `disable-git`, `install-hooks`, `shell-init` | Go-native | Client | post-commit + pre-push hooks |
| Autostart / init | `autostart-install/uninstall/status`, `launchd-install/uninstall`, `init` | Go-native | Client | |

## 5. AI / NLP / LLM enhancement pipeline — Owner: **Server**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Commit/timer trigger pipeline | (triggers) | HTTP → server | Server | `/trigger/commit`, `/trigger/timer` |
| NLP parsing (spaCy) | — | Python (server) | Server | `nlp_parser.py` |
| Description enhancement | — | Python (server) | Server | `description_enhancer.py` |
| Multi-provider LLM pipeline | — | Python (server) | Server | `backend/llm/` |
| Boardroom (multi-persona review) | `boardroom` | HTTP → server | Server | `/trigger/boardroom` |
| Plan decomposition | `plan` | HTTP → server | Server | `/trigger/plan/*` |

## 6. Learning & personalization — Owner: **Server**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Enable / sync learning | `enable-learning`, `learning-sync`, `learning-status` | HTTP → server | Server | `/learning/enable`, `/learning/sync`, `/learning/status` |
| Cron management | `learning-setup-cron`, `learning-remove-cron`, `learning-cron-status` | HTTP → server | Server | `/learning/cron/*` |
| Reset | `learning-reset` | HTTP → server | Server | `/learning/reset` |
| Profile / test response / revoke | `show-profile`, `test-response`, `revoke-consent` | HTTP → server | Server | `/learning/profile`, `/learning/test-response`, `/learning/revoke` |

## 7. Reporting — Owner: **Server** (AI-enhanced generation)

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Preview / send / save report | `preview-report`, `send-report`, `save-report` | HTTP → server | Server | `/reports/preview`, `/reports/send`, `/reports/save` |
| Summary delivery | `send-summary` | HTTP → server | Server | `/reports/eod` |
| End-of-day report (scheduled) | (scheduler) | HTTP → server | Server | Scheduler calls `/reports/eod` |

## 8. Ticket alerts — Owner: **Client** (ticket events = client)

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Alert polling (GitHub/Azure/Jira) | (daemon) | Python (in client) | Client | ⚠ Phase 2: Port to Go (reuse connectors + SQLite); remove `backend.alert_poller` |
| Read / clear alerts | `alerts` | Python (in client) | Client | ⚠ Phase 2: Read from SQLite natively |

## 9. Notifications (delivery) — Owner: **Client**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Telegram delivery | `telegram-status` | Python (in client) | Client | ⚠ Phase 2: Port delivery to Go (Telegram Bot API). AI-conversational processing → Server |
| Slack delivery | (daemon) | Python (in client) | Client | ⚠ Phase 2: Port delivery to Go (Slack webhook) |

## 10. Cloud / auth / license — Owner: **Server** (cloud), local gating client-side

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Login / logout / whoami | `login`, `logout`, `whoami`, `cloud` | HTTP → server | Server | `/auth/request-magic-link`, `/auth/verify-magic-link`, `/auth/logout`, `/auth/whoami` |
| License / terms | `license`, `terms` | HTTP → server | Server | `/license/status`, `/license/terms`, `/license/accept`, `/license/check` |

## 11. Server-management tools — Owner: **Server** (remove from client)

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Server TUI | `server-tui` | Removed from client | Server | ✅ Deleted from client CLI (Phase 1a — d5f8f36) |
| Admin web GUI | `admin-start` | Removed from client | Server | ✅ Deleted from client CLI (Phase 1a — d5f8f36) |
| Webhook server (managed mode) | (daemon) | Python subprocess | Server | Launched by managed/deploy mode only; external mode connects via HTTP |

---

## Summary of current mismatches (⚠ = decoupling work)

Phase 1 (remove + convert-to-HTTP) is **complete**. Only Phase 2 items remain.

| Capability | Intended owner | Status |
|---|---|---|
| `server-tui`, `admin-start` | Server | ✅ Removed from client (Phase 1a — d5f8f36) |
| GitHub ticket sync (`github_ticket_sync.py`) | Client | ✅ Python call removed (Phase 1b — d5f8f36) |
| Reports (preview/send/save/summary/EOD) | Server | ✅ HTTP → server (Phase 1c) |
| Learning suite (all `learning-*`, profile, test-response, revoke) | Server | ✅ HTTP → server (Phase 1c) |
| Cloud/auth/license (`login/logout/whoami/license/terms`) | Server | ✅ HTTP → server (Phase 1c) |
| Ticket alerts (`alerts`, poller) | Client | ⚠ **Port to Go** (Phase 2) |
| Telegram / Slack delivery | Client | ⚠ **Port to Go** (Phase 2) |

Everything not marked ⚠ already matches its intended owner.

## How to change ownership

1. Edit the **Owner** column for the capability above.
2. If you flip something to **Client**, it must be implemented in Go in
   `devtrack_client` (reuse `connectors/*`, `internal/db`, `internal/match`).
3. If you flip something to **Server**, the client keeps only a thin
   `HTTP → server` command (reuse `HTTPTriggerClient` in
   `devtrack_client/http_trigger.go`) and the logic lives in `devtrack_server`.
4. The invariant: **no `uv run python -m backend.*` in the client** except the
   managed-mode `webhook_server` launch.
