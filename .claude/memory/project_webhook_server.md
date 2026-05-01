---
name: project_webhook_server
description: FastAPI webhook server + alert poller for DevTrack — handles inbound webhook events from Azure DevOps, GitHub, GitLab, Jira and outbound trigger calls from the Go daemon
type: feature
---

# Webhook Server + Alert Poller

**Completed**: April 5, 2026
**Commit**: af71b43 (added alerter poll and webhook server)

## What Was Built

Three new Python modules forming the inbound/outbound webhook layer:

### backend/webhook_server.py
- FastAPI server started as a subprocess (like `python_bridge.py`)
- Handles **inbound** webhook events from Azure DevOps, GitHub, GitLab, Jira via POST endpoints
- Handles **outbound** trigger calls from Go daemon (`commit_trigger`, `timer_trigger`, full IPC message vocabulary) via `/trigger/*` endpoints
- All `/trigger/*` endpoints protected by `X-DevTrack-API-Key` header
- Go side uses mutual cert-pinning (HTTPS) when calling these endpoints
- Run: `python -m backend.webhook_server`

### backend/webhook_handlers.py
- `WebhookEventHandler` class — decoupled from HTTP routing for testability
- Azure DevOps event types handled:
  - `workitem.updated` — fields changed (state, assignment, etc.)
  - `workitem.commented` — new comment added
  - `workitem.created` — new work item created
  - `workitem.deleted` — work item deleted
- GitHub and Jira routing also wired (handlers TBD)
- Accepts optional `ipc_client`, `notifier`, and `project_sync` (Azure) dependencies

### backend/webhook_notifier.py
- `WebhookNotifier` class — delivers notifications for incoming webhook events
- Channels: macOS `osascript` OS notification + terminal print
- Config: `WEBHOOK_NOTIFY_OS` (default true), `WEBHOOK_NOTIFY_TERMINAL` (default true)
- Platform-aware: OS notification only fires on macOS (`platform.system() == "Darwin"`)

### backend/alert_poller.py
- Full async background polling loop for GitHub + Azure DevOps
- User ID resolution priority: `GITHUB_USER` env → `consent.json` email → `EMAIL` env → empty string
- SQLite fallback: `_sqlite_load_last_checked` / `_sqlite_save_last_checked` persist delta timestamps when MongoDB unavailable
- CLI modes:
  - `python -m backend.alert_poller` — run poll loop
  - `--show` — show unread notifications (last 24 h)
  - `--all` — show all notifications
  - `--clear` — mark all as read

## Configuration

```env
# Webhook server
WEBHOOK_NOTIFY_OS=true
WEBHOOK_NOTIFY_TERMINAL=true

# Alert poller (shared with alerters)
ALERT_ENABLED=true
ALERT_POLL_INTERVAL_SECS=300
ALERT_GITHUB_ENABLED=true
ALERT_AZURE_ENABLED=true
ALERT_NOTIFY_ASSIGNED=true
ALERT_NOTIFY_COMMENTS=true
ALERT_NOTIFY_STATUS_CHANGES=true
ALERT_NOTIFY_REVIEW_REQUESTED=true
```

## Architecture Note

The webhook server coexists with `python_bridge.py`:
- `python_bridge.py` — handles **Go → Python IPC** over TCP (existing path)
- `webhook_server.py` — handles **external systems → DevTrack** over HTTP webhooks AND **Go → Python** over HTTPS for trigger calls (new path)

Both can run simultaneously. The HTTPS trigger path in `webhook_server.py` is an alternative to the TCP IPC path, useful for cloud/remote deployments.

## Why

**Why:** External systems (Azure DevOps, GitHub, Jira) push events via webhooks rather than polling. The webhook server receives these push events in real time and routes them to notifications and project sync, complementing the pull-based alert poller.

## How to Apply

- Start the webhook server alongside the daemon: `python -m backend.webhook_server &`
- Configure webhook URLs in Azure DevOps service hooks / GitHub webhook settings pointing to `https://<host>/webhooks/azure`, `/webhooks/github`, etc.
- Set `X-DevTrack-API-Key` in Go daemon config so outbound trigger calls authenticate correctly
- The alert poller runs standalone or as a subprocess of the daemon; set `ALERT_ENABLED=true` and relevant source flags
