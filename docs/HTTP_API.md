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
also provide repository, timestamp, changed-file, resolved-ticket, confidence, workspace-routing,
and PM-specific fields. A successful response includes `status`, `actions`, and `commit_hash`.

### `POST /trigger/timer`

The normal payload contains `timestamp`, `interval_mins`, and `trigger_count`, with optional
workspace-routing fields. An empty JSON object is accepted for compatibility. A successful response
includes `status` and `trigger_count`.

### Work sessions

| Method and path | Request | Successful response |
|---|---|---|
| `POST /trigger/work_session_start` | `{"session_id":42,"ticket_ref":"GH-123"}` | status plus `session_id` |
| `POST /trigger/work_session_stop` | `{"session_id":42}` | status plus `session_id` |

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

Changes to a request or response shape must update both contract-test suites and this document in
the same change.
