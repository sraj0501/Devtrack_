---
name: Telemetry Plan
description: Planned telemetry system — download stats, usage events, registered user tracking, real-time admin dashboard
type: project
---

Telemetry is fully designed but not yet implemented. Plan document at: `docs/TELEMETRY_PLAN.md`

**Why:** User wants download counts, usage stats, and registered user (CS-4 signup) growth visible in real-time in the admin console.

**How to apply:** When the user says "implement telemetry" or "build the analytics dashboard", start from Phase 1 of the plan doc. Do not redesign — the architecture is already agreed.

## Three Tiers

1. **GitHub download counts** — free, no code; poll GitHub Releases API every 15 min via background task
2. **Anonymous opt-in usage** (Go binary) — `devtrack telemetry on/off/status`; fires `install`, `command`, `heartbeat`, `signup` events as non-blocking goroutine POSTs to `/telemetry/event`; install_id = random UUID, never linked to identity; default OFF
3. **Registered users** (CS-4) — `api_keys` table extended with `last_seen_at` + `install_id`; updated on every authenticated request; no extra binary work needed

## Real-Time Delivery

- FastAPI SSE endpoint `GET /admin/telemetry/stream`
- Two named SSE events: `counters` (pushed every 5s, aggregate stats) and `live_event` (pushed immediately on each telemetry INSERT via asyncio.Queue)
- Admin dashboard uses HTMX SSE extension (`hx-ext="sse"`) — server returns HTML fragments, no JS needed
- GitHub counts served from in-memory cache (refreshed every 15 min); shown with "last updated" timestamp

## Key Files to Create

**Go**: `devtrack-bin/telemetry.go` — TelemetryConfig (~/.devtrack/telemetry.json chmod 0600), Track() goroutine POST, handleTelemetry() CLI

**Python**: `backend/telemetry/` — store.py (telemetry_events SQLite table), router.py (/telemetry/event POST + SSE GET), github_stats.py (background poller), stream.py (asyncio.Queue + SSE generator)

**Modified**: cli.go (Track per command), cloud.go (Track signup), daemon.go (Track heartbeat), webhook_server.py (mount router + start poller), admin/routes.py + dashboard.html (SSE panel)

## Data Model

```sql
telemetry_events(id, install_id, event, props JSON, received_at)
api_keys: + install_id TEXT, + last_seen_at TEXT
```

No materialized aggregate tables — all DAU/MAU/histogram queries run live against telemetry_events with indexes on install_id, event, received_at.

## Implementation Order (4 phases, ~3.5 days total)

1. Server-side foundation — store + /telemetry/event endpoint + GitHub poller
2. Real-time dashboard — SSE stream + admin template updates
3. Go binary — telemetry.go + wire into cli/cloud/daemon
4. Polish — first-run prompt, status command, retention cohorts
