# DevTrack HTTP API contract

This document defines the stable Go-client-to-Python-server boundary. The server implementation is
`devtrack_server/backend/webhook_server.py`; cross-boundary contract tests live in
`devtrack_client/internal/trigger/api_contract_test.go` and
`devtrack_server/backend/tests/test_api_contract.py`.

## Authentication

When `DEVTRACK_API_KEY` is configured, clients send it as `X-DevTrack-API-Key`. Health and version
probes remain available for local capability detection. JSON requests use `Content-Type:
application/json`.

## Health and lifecycle

| Method and path | Request | Successful response |
|---|---|---|
| `GET /health` | none | `{"status":"ok","service":"devtrack-webhooks"}` |
| `GET /version` | none | version and service metadata |
| `GET /status` | none | server capability/status object |
| `POST /trigger/ping` | `{}` | `{"status":"ok","pong":true}` |
| `POST /trigger/workspace_reload` | `{"source":"cli"}` | `{"status":"ok","message":"..."}` |

## Core triggers

### `POST /trigger/commit`

Required client fields are `commit_hash`, `commit_message`, `author`, and `branch`. The client may
also provide repository, timestamp, changed-file, workspace-routing, and PM-specific fields. The
deterministic ticket fields are `ticket_id` (canonical reference), `ticket_external_id`,
`ticket_source`, `ticket_state`, `ticket_confidence`, and `ticket_conflict`. A conflicted mapping may
be retained locally but the server must not stage an outbound PM action from it. A successful
response includes `status`, `actions`, and `commit_hash`.

### `POST /trigger/timer`

The normal payload contains `timestamp`, `interval_mins`, and `trigger_count`, with optional
workspace-routing fields. An empty JSON object is accepted for compatibility. A successful response
includes `status` and `trigger_count`.

Work-duration session endpoints have been removed. Active-ticket selection remains local to the
client; it does not start a timer or record durations.

## Boardroom

`POST /trigger/boardroom` accepts either `plan_text` or `markdown` and an `output_format` of
`terminal` or `markdown`. Its response contains `report`, `verdict`, `verdict_summary`, vote counts,
`pros`, and `cons`. Missing boardroom dependencies return HTTP 503; missing plan content returns
HTTP 400.

`POST /trigger/boardroom/chat` accepts `plan_text`, `history`, and optional `user_message`,
`addressed_to`, or `final_say`. It returns persona responses, updated history, and session-closing
state.

## Queue and client-event synchronization

`POST /trigger/client_events` carries a client ID and replay-keyed event records. The response
reports the number accepted. `POST /queue/execute_staged` carries the full local pending-action
identity and returns `posted` or an error state. Both routes require the API-key header when server
authentication is enabled.

`GET /queue/history` reads server-owned queue rows across all statuses, ordered by creation time
descending with ID as the tie breaker. Parameters: `action_type` (optional exact match), `hours`
(default 24; 0 means all history), `limit` (1–1000, default 1000), and `offset` (default 0).
Response: `{"actions":[<complete queue rows>]}`. Unavailable persistence returns HTTP 503 rather
than an empty success. The CLI pages through history and labels these independent PostgreSQL
identities `server:<id>`; bare numeric IDs continue to identify local SQLite actions.

`POST /queue/review` accepts `{"action_id":123,"operation":"reject"}` or
`{"action_id":123,"operation":"edit","payload":{"narrative":"..."}}`. Edits preserve pending
status. Both operations require the row still to be pending; otherwise HTTP 409. Approval uses
the existing `POST /queue/execute` atomic claim and execution path. These authenticated reads
and review operations do not enable client-event synchronization.

## End-of-day report

`POST /reports/eod` accepts `email`, `date`, `workspace`, and an optional `commits` array. Each
explicit commit row contains `ticket_id`, `commit_message`, `commit_hash`, and `timestamp`. The Go
client includes only the selected day's local commit summaries when the user runs an EOD command or
enables the EOD scheduler. This makes reports work in PostgreSQL-backed Managed mode without
silently enabling continuous client-event synchronization. The response contains `output`,
`success`, and the staged `action_id` when queue staging is available.

`devtrack eod status` and `devtrack eod show` consult server history as well as legacy local
report actions. The most recently created report wins, including rejected or posted reports.
An unavailable server is reported explicitly; an existing local report remains readable with
a warning. Generating a report prints its qualified queue ID for subsequent review.

Changes to a request or response shape must update both contract-test suites and this document in
the same change.
