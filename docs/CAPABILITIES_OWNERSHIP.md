# DevTrack — Capabilities & Ownership

This file is the **single editable source of truth** for which capabilities are
owned by the **client** (`devtrack_client`, Go) vs the **server**
(`devtrack_server`, Python). Edit the **Owner** column to reassign a capability;
the **Current state** column records how it is implemented *today* so you can see
where reality diverges from the intended ownership.

_Last updated: 2026-09-25 (client-server decoupling complete; PostgreSQL is mandatory server-side;
TASK-158 and the first Sage structured-distillation boundary are merged on unreleased `dev`)._

## Ownership model (intended)

- **Client owns:** ticket fetching / caching / matching / managing / updating;
  the git flow; notification **delivery** (telegram/slack); local UX, daemon
  lifecycle, workspace/deploy. The client reaches every *server* capability over
  HTTP — it never embeds backend logic.
- **Server owns:** LLM enhancement pipeline; the learning &
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
| Status / logs | `status`, `logs` | Go-native | Client | `logs --follow` implemented (TASK-131) — polls and streams appended lines, survives log rotation |
| Pause/resume/skip triggers | `pause`, `resume`, `skip-next`, `force-trigger`, `on`, `off` | Go-native | Client | |
| Config reload | `reload`, `reload-config`, `settings` | Go-native | Client | |
| DB stats / version / help | `db-stats`, `version`, `help` | Go-native | Client | |
| Telemetry ping | `telemetry` | Go-native | Client | |

## 2. Git flow & AI commit enhancement — Owner: **Client**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| AI-enhanced commit (A/E/R/Q/C) | `devtrack git commit` | Go-native (`internal/gitcmd` → LLM) | Client | Explicit helper only; normal `git commit` is never intercepted. Calls the configured Ollama/OpenAI-compatible provider directly from the client. |
| Stage / history / passthrough | `devtrack git add`, optional `git history`/`git messages` aliases | Go-native | Client | Shell integration leaves native `git add` and `git commit` unchanged. |
| DevTrack Sage | `sage status/pause/resume/search/topics/doctor/harness` | Go-native (`internal/sage/`) | Client | Local command capture and searchable knowledge; legacy repository chat/autonomous Git commands are removed. |
| Deferred (offline) commits | `commits pending/review/enhance`, `commit-queue` | Go-native | Client | Durable snapshot + 3-way apply |

## 3. Ticket management (PM connectors) — Owner: **Client**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| GitHub issues: check/list/sync/view | `github-check/list/sync/view` | Go-native | Client | `connectors/github` |
| GitLab issues: check/list/sync/view | `gitlab-check/list/sync/view` | Go-native | Client | `connectors/gitlab` |
| Azure work items: check/list/sync/view | `azure-check/list/sync/view` | Go-native | Client | `connectors/azure` |
| Ticket comment / create | (background trigger / explicit command) | Go-native | Client | `connectors/*/comment.go`, `create.go`; outbound work stages through `pending_actions`. |
| Offline ticket cache | (background trigger / explicit command) | Go-native (SQLite) | Client | `ticket_cache` table |
| Smart matching + likelihood | (background trigger) | Go-native (fuzzy + optional Ollama embeddings) | Client | `internal/match`; TASK-160 will make suggestions non-authoritative. |
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

## 5. LLM enhancement pipeline — Owner: **Server**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Commit/timer trigger pipeline | (triggers) | HTTP → server | Server | `/trigger/commit`, `/trigger/timer` |
| Structured task enrichment | — | Python (server) | Server | `llm_task_parser.py`; configured provider, validated JSON, explicit confidence, raw-text fallback; Go ticket target stays authoritative |
| Description enhancement | — | Python (server) | Server | `description_enhancer.py` |
| Multi-provider LLM pipeline | — | Python (server) | Server | `backend/llm/` |
| Boardroom (multi-persona review) | `boardroom` | HTTP → server | Server | `/trigger/boardroom` |
| Plan decomposition | `plan` | HTTP → server | Server | `/trigger/plan/*` |

## 6. Learning & personalization — Owner: **Server**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Learning status | `learning-status` | HTTP → server | Server | ⚠ `/learning/status` calls absent `LearningIntegration.get_status`; unavailable until repaired. |
| Enable / sync learning | `enable-learning`, `learning-sync` | HTTP → server | Server | ⚠ `/learning/enable` and `/learning/sync` call adapter methods absent from `LearningIntegration`; unavailable until repaired. |
| Cron management | `learning-setup-cron`, `learning-remove-cron`, `learning-cron-status` | HTTP → server | Server | ⚠ `/learning/cron/*` calls adapter methods absent from `LearningIntegration`. |
| Reset | `learning-reset` | HTTP → server | Server | ⚠ `/learning/reset` calls an adapter method absent from `LearningIntegration`. |
| Profile / test response / revoke | `show-profile`, `test-response`, `revoke-consent` | HTTP → server | Server | ⚠ Profile calls the existing method before initialization; test/revoke use absent method names. Treat the group as unsupported pending adapter repair. |

## 7. Reporting — Owner: **Server** (AI-enhanced generation)

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Preview / send / save report | `preview-report`, `send-report`, `save-report` | HTTP → server | Server | `/reports/preview`, `/reports/send`, `/reports/save` |
| Summary delivery | `send-summary` | HTTP → server | Server | `/reports/eod` |
| End-of-day report (scheduled) | (scheduler) | HTTP → server | Server | Scheduler calls `/reports/eod` |

## 8. Ticket alerts — Owner: **Client** (ticket events = client)

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Alert polling (GitHub/Azure) | (daemon) | Go-native | Client | ✅ `internal/alerts/` reuses Go connectors + SQLite; Python `alert_poller.py` retired. Jira alert parity remains staged rollout work. |
| Read / clear alerts | `alerts` | Go-native | Client | ✅ Reads from SQLite via `internal/db/` |

## 9. Notifications (delivery) — Owner: **Client**

| Capability | Commands | Current state | Owner | Notes |
|---|---|---|---|---|
| Telegram delivery | `telegram-status` | Go-native | Client | ✅ Phase 2 complete: `internal/telegram/` — Go-native bot, starts with daemon when `TELEGRAM_ENABLED=true` |
| Slack delivery | (daemon) | Go-native | Client | ✅ Phase 2 complete: `internal/notify/` — `NewSlackFromConfig` |

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

Phase 1 (remove + convert-to-HTTP) and Phase 2 (port alerts/notify delivery to Go) are both
**complete**. Ownership boundaries match, but the communication-learning HTTP adapter has a known
functional gap: most handlers call methods that `LearningIntegration` does not implement.

| Capability | Intended owner | Status |
|---|---|---|
| `server-tui`, `admin-start` | Server | ✅ Removed from client (Phase 1a — d5f8f36) |
| GitHub ticket sync (`github_ticket_sync.py`) | Client | ✅ Python call removed (Phase 1b — d5f8f36) |
| Reports (preview/send/save/summary/EOD) | Server | ✅ HTTP → server (Phase 1c) |
| Learning suite (all `learning-*`, profile, test-response, revoke) | Server | ⚠ HTTP boundary exists; status is inspection-only and the remaining adapter methods require repair |
| Cloud/auth/license (`login/logout/whoami/license/terms`) | Server | ✅ HTTP → server (Phase 1c) |
| Ticket alerts (`alerts`, poller) | Client | ✅ Ported to Go (Phase 2 — `internal/alerts/`) |
| Telegram / Slack delivery | Client | ✅ Ported to Go (Phase 2 — `internal/telegram/`, `internal/notify/`) |

Everything matches its intended owner; ownership alignment must not be read as end-to-end feature
availability for the learning suite.

## How to change ownership

1. Edit the **Owner** column for the capability above.
2. If you flip something to **Client**, it must be implemented in Go in
   `devtrack_client` (reuse `connectors/*`, `internal/db`, `internal/match`).
3. If you flip something to **Server**, the client keeps only a thin
   `HTTP → server` command (reuse `HTTPTriggerClient` in
   `devtrack_client/http_trigger.go`) and the logic lives in `devtrack_server`.
4. The invariant: **no `uv run python -m backend.*` in the client** except the
   managed-mode `webhook_server` launch.
