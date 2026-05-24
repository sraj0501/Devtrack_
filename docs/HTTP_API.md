# DevTrack HTTP API — v1

_Last updated: 2026-05-24 | TASK-044 | Branch: features/SPLIT-001-monorepo-restructure_

## Overview

The devtrack client (Go binary) communicates with devtrack-server exclusively over HTTPS.
There is no shared code, module, or compiled artefact between them. The entire boundary
is this HTTP/JSON contract.

All client-to-server traffic is HTTPS. The Go daemon generates a self-signed certificate
at startup and cert-pins it in the HTTP client so no InsecureSkipVerify is required.
In cloud/managed mode, the system CA roots are used instead.

**Transport**: HTTPS only. Plain HTTP is not supported in production.

**Base URL**: configured via `DEVTRACK_SERVER_URL` env var (e.g. `https://localhost:8089`).

---

## Versioning

API version: **v1**.

No version prefix is used in URL paths. When a breaking change is required, new paths
will be prefixed `/v2/`. The `/v1/` prefix will not be added retroactively.

---

## Authentication

### Trigger endpoints (`/trigger/*`, `/health`, `/version`, `/status`)

All `/trigger/*` endpoints, plus `/health`, `/version`, and `/status`, validate the
`X-DevTrack-API-Key` request header.

The key is set by `DEVTRACK_API_KEY` on both client and server. When the env var is
not set on the server, auth is skipped entirely (dev/testing mode). The client reads
the key via the same `DEVTRACK_API_KEY` var.

```
X-DevTrack-API-Key: <value of DEVTRACK_API_KEY>
```

Response when key is missing or wrong: `HTTP 403`
```json
{"detail": "Invalid or missing X-DevTrack-API-Key"}
```

### Admin console (`/admin/*`)

Admin routes use JWT cookie auth. The credentials are validated against
`ADMIN_USERNAME` and `ADMIN_PASSWORD` env vars. The JWT is issued as an
`HttpOnly` cookie on `POST /admin/login`. No API key is used for admin routes.

### Inbound webhooks (`/webhooks/*`)

Each inbound webhook platform uses its own mechanism:

- `/webhooks/azure-devops` — HTTP Basic Auth (`WEBHOOK_AZURE_USERNAME` / `WEBHOOK_AZURE_PASSWORD`)
- `/webhooks/github` — HMAC-SHA256 signature via `X-Hub-Signature-256` header (`WEBHOOK_GITHUB_SECRET`)
- `/webhooks/gitlab` — `X-Gitlab-Token` header (`WEBHOOK_GITLAB_SECRET`)
- `/webhooks/jira` — no auth (rely on network/firewall controls)

---

## Standard Error Format

All endpoints return errors in FastAPI's standard format:

```json
{"detail": "<human-readable error message>"}
```

For server-side failures the Go client also handles:

```json
{"error": "<message>", "detail": "<optional detail>"}
```

HTTP status codes used:
- `200` — success
- `400` — bad request (missing or invalid field)
- `401` — authentication missing
- `403` — authentication present but invalid
- `404` — resource not found
- `500` — internal server error
- `503` — feature dependency not installed (e.g. PM agent, boardroom)

---

## Endpoint Reference

### GET /health

**Caller**: Go client (daemon startup, `devtrack status`)
**Auth**: `X-DevTrack-API-Key` (optional — checked if key is configured)
**Description**: Liveness check. Returns immediately. Used by the client to determine
whether the server is reachable before sending triggers.

**Request**: No body.

**Response** `200 OK`:
```json
{
  "status": "ok",
  "service": "devtrack-webhooks"
}
```

---

### GET /version

**Caller**: Go client (`devtrack cloud status`)
**Auth**: `X-DevTrack-API-Key`
**Description**: Returns the server's version string.

**Request**: No body.

**Response** `200 OK`:
```json
{
  "version": "1.0",
  "service": "devtrack-webhooks"
}
```

---

### GET /status

**Caller**: Go client (`devtrack status`)
**Auth**: `X-DevTrack-API-Key`
**Description**: Returns the server's operational status including which integrations
are enabled.

**Request**: No body.

**Response** `200 OK`:
```json
{
  "service": "devtrack-webhooks",
  "azure_devops": false,
  "webhook_enabled": true,
  "notify_os": true,
  "notify_terminal": true
}
```

| Field | Type | Description |
|---|---|---|
| `service` | string | Always `"devtrack-webhooks"` |
| `azure_devops` | bool | Whether Azure DevOps sync is enabled (`AZURE_SYNC_ENABLED`) |
| `webhook_enabled` | bool | Whether inbound webhooks are enabled (`WEBHOOK_ENABLED`) |
| `notify_os` | bool | Whether OS notifications are enabled (`WEBHOOK_NOTIFY_OS`) |
| `notify_terminal` | bool | Whether terminal notifications are enabled (`WEBHOOK_NOTIFY_TERMINAL`) |

---

### POST /trigger/commit

**Caller**: Go client (on every Git commit detected by `git_monitor.go`)
**Auth**: `X-DevTrack-API-Key`
**Description**: Notifies the server of a new Git commit. The server parses the commit
message with NLP, optionally syncs to the PM platform, and links the commit to any
active work session.

**Request body** (all fields optional except `commit_hash`):
```json
{
  "commit_hash":      "abc123def456",
  "commit_message":   "fix: resolve login timeout issue",
  "repo_path":        "/home/user/myproject",
  "author":           "dev@example.com",
  "timestamp":        "2026-05-24T14:00:00Z",
  "files_changed":    ["backend/auth.py", "backend/config.py"],
  "branch":           "fix/login-timeout",
  "workspace_name":   "myproject",
  "pm_platform":      "github",
  "pm_project":       "myorg/myproject",
  "pm_assignee":      "dev@example.com",
  "pm_iteration_path": "",
  "pm_area_path":     "",
  "pm_milestone":     0
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `commit_hash` | string | yes | Full or short Git commit SHA |
| `commit_message` | string | no | Commit message text for NLP parsing |
| `repo_path` | string | no | Absolute path to the local repository |
| `author` | string | no | Commit author email or name |
| `timestamp` | string | no | ISO 8601 commit timestamp |
| `files_changed` | string[] | no | List of changed file paths |
| `branch` | string | no | Current branch name |
| `workspace_name` | string | no | Workspace name from `workspaces.yaml` |
| `pm_platform` | string | no | PM platform override (`"github"`, `"azure"`, `"gitlab"`, `"jira"`) |
| `pm_project` | string | no | PM project identifier |
| `pm_assignee` | string | no | Assignee for created/updated PM items |
| `pm_iteration_path` | string | no | Azure DevOps iteration path |
| `pm_area_path` | string | no | Azure DevOps area path |
| `pm_milestone` | int | no | Milestone number (GitHub) |

**Response** `200 OK`:
```json
{
  "status": "ok",
  "actions": ["session_linked:3", "pm_sync:github"],
  "commit_hash": "abc123def456"
}
```

| Field | Type | Description |
|---|---|---|
| `status` | string | Always `"ok"` |
| `actions` | string[] | List of actions taken (e.g. `"session_linked:<id>"`, `"pm_sync:<platform>"`) |
| `commit_hash` | string | Echo of the received commit hash |

---

### POST /trigger/timer

**Caller**: Go client (`scheduler.go` — fires on the configured cron schedule)
**Auth**: `X-DevTrack-API-Key`
**Description**: Periodic work-update prompt. In remote/external mode the server
delivers a reminder via Telegram or Slack instead of launching the local TUI.

**Request body**:
```json
{
  "timestamp":      "2026-05-24T14:00:00Z",
  "interval_mins":  60,
  "trigger_count":  3,
  "workspace_name": "myproject",
  "pm_platform":    "github",
  "pm_project":     "myorg/myproject",
  "pm_assignee":    "",
  "pm_iteration_path": "",
  "pm_area_path":   "",
  "pm_milestone":   0
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `timestamp` | string | no | ISO 8601 trigger time |
| `interval_mins` | int | no | Scheduler interval in minutes |
| `trigger_count` | int | no | Cumulative trigger number since daemon start |
| `workspace_name` | string | no | Most-recently-active workspace |
| `pm_platform` | string | no | PM platform |
| `pm_project` | string | no | PM project |
| `pm_assignee` | string | no | PM assignee |
| `pm_iteration_path` | string | no | Azure iteration path |
| `pm_area_path` | string | no | Azure area path |
| `pm_milestone` | int | no | Milestone number |

**Response** `200 OK` (normal path):
```json
{
  "status": "accepted",
  "trigger_count": 3,
  "prompt_channel": "telegram",
  "active_session": true
}
```

**Response** `200 OK` (vacation mode active):
```json
{
  "status": "vacation_auto",
  "trigger_count": 3,
  "confidence": 0.87,
  "submitted": true,
  "skipped_reason": null
}
```

| Field | Type | Description |
|---|---|---|
| `status` | string | `"accepted"` or `"vacation_auto"` |
| `trigger_count` | int | Echo of the received trigger count |
| `prompt_channel` | string | Channels used (`"telegram"`, `"slack"`, `"telegram,slack"`, or `"none"`) |
| `active_session` | bool | Whether a work session is currently active |
| `confidence` | float | (vacation only) Auto-responder confidence score |
| `submitted` | bool | (vacation only) Whether the update was submitted |
| `skipped_reason` | string\|null | (vacation only) Reason if update was skipped |

---

### POST /trigger/workspace_reload

**Caller**: Go client (on `devtrack workspace reload` or config file change)
**Auth**: `X-DevTrack-API-Key`
**Description**: Tells the server to reinitialise the workspace router after
`workspaces.yaml` has been modified.

**Request body**:
```json
{"source": "cli"}
```

**Response** `200 OK`:
```json
{"status": "ok", "message": "workspace router reloaded"}
```

---

### POST /trigger/shutdown

**Caller**: Go client (on `devtrack stop`)
**Auth**: `X-DevTrack-API-Key`
**Description**: Requests a graceful shutdown of the Python server process. The server
schedules a `SIGTERM` to itself after the configured grace period and returns immediately.

**Request body**: `{}` (empty object)

**Response** `200 OK`:
```json
{"status": "ok"}
```

---

### POST /trigger/ping

**Caller**: Go client (health monitor, keepalive)
**Auth**: `X-DevTrack-API-Key`
**Description**: Lightweight liveness check. More efficient than `GET /health` for
frequent polling because it does not serialise the service name.

**Request body**: `{}` (empty object)

**Response** `200 OK`:
```json
{"status": "ok", "pong": true}
```

---

### POST /trigger/work_session_start

**Caller**: Go client (`devtrack work start`)
**Auth**: `X-DevTrack-API-Key`
**Description**: Notifies the server that a work session has started. The server uses
this to link subsequent commits to the session.

**Request body**:
```json
{
  "session_id": 42,
  "ticket_ref": "GH-123"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `session_id` | int | yes | Work session ID from the Go SQLite database |
| `ticket_ref` | string | no | Ticket reference associated with this session |

**Response** `200 OK`:
```json
{"status": "ok", "session_id": 42}
```

---

### POST /trigger/work_session_stop

**Caller**: Go client (`devtrack work stop`)
**Auth**: `X-DevTrack-API-Key`
**Description**: Notifies the server that the current work session has ended.

**Request body**:
```json
{"session_id": 42}
```

**Response** `200 OK`:
```json
{"status": "ok", "session_id": 42}
```

---

### POST /trigger/plan/preview

**Caller**: Go client (`devtrack plan "<problem>"`)
**Auth**: `X-DevTrack-API-Key`
**Description**: Decomposes a problem statement into an Epic → Story → Task hierarchy
and returns a human-readable preview plus a serialised `plan_token` that is passed back
to `/trigger/plan/create` to execute the creation.

**Request body** — inline problem:
```json
{
  "problem":         "Build a login system with OAuth and MFA",
  "platform":        "azure",
  "project_context": "DevTrack backend service",
  "notes":           "Must support Google and GitHub OAuth providers"
}
```

**Request body** — Markdown plan file:
```json
{
  "markdown": "# Plan\n\nBuild a login system...",
  "platform": "azure"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `problem` | string | yes (or `markdown`) | Free-text problem statement |
| `markdown` | string | yes (or `problem`) | Full Markdown plan file contents |
| `platform` | string | no | PM platform (`"azure"`, `"github"`, `"gitlab"`, `"jira"`); default `"azure"` |
| `project_context` | string | no | Project name/context injected into the decomposition prompt |
| `notes` | string | no | Additional constraints appended to the problem statement |

**Response** `200 OK`:
```json
{
  "preview":     "## Epic: Authentication System\n\n### Story: OAuth Integration\n...",
  "plan_token":  "eyJpdGVtcyI6WyJ...",
  "total_count": 12,
  "epic_count":  2,
  "story_count": 5,
  "task_count":  5,
  "platform":    "azure"
}
```

| Field | Type | Description |
|---|---|---|
| `preview` | string | Formatted Markdown or terminal-friendly plan summary |
| `plan_token` | string | Base64-encoded JSON blob; pass to `/trigger/plan/create` |
| `total_count` | int | Total work items in the plan |
| `epic_count` | int | Number of epics |
| `story_count` | int | Number of stories |
| `task_count` | int | Number of tasks |
| `platform` | string | Platform the plan is targeting |

**Error** `400`: `"'problem' or 'markdown' field required"`
**Error** `503`: `"PM agent dependencies not installed"`

---

### POST /trigger/plan/create

**Caller**: Go client (after user confirms the preview from `/trigger/plan/preview`)
**Auth**: `X-DevTrack-API-Key`
**Description**: Executes plan creation on the PM platform using the `plan_token`
returned by `/trigger/plan/preview`. Creates all epics, stories, and tasks in order.

**Request body**:
```json
{"plan_token": "eyJpdGVtcyI6WyJ..."}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `plan_token` | string | yes | Token from `/trigger/plan/preview` response |

**Response** `200 OK`:
```json
{
  "created": [
    {
      "title":        "Authentication System",
      "item_type":    "epic",
      "level":        0,
      "platform_id":  "AB#1001",
      "platform_url": "https://dev.azure.com/org/project/_workitems/edit/1001"
    }
  ],
  "failed": [
    {
      "title": "MFA Setup Story",
      "error": "Azure API rate limit exceeded"
    }
  ],
  "progress": [
    "Authentication System: created",
    "OAuth Integration: created",
    "MFA Setup Story: failed"
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `created` | object[] | Work items successfully created |
| `created[].title` | string | Item title |
| `created[].item_type` | string | `"epic"`, `"story"`, or `"task"` |
| `created[].level` | int | Hierarchy level (0=epic, 1=story, 2=task) |
| `created[].platform_id` | string | ID on the PM platform |
| `created[].platform_url` | string | URL to the created item |
| `failed` | object[] | Work items that could not be created |
| `failed[].title` | string | Item title |
| `failed[].error` | string | Error message |
| `progress` | string[] | Ordered log of creation attempts |

**Error** `400`: `"'plan_token' field required"` or `"Invalid plan_token: ..."`
**Error** `503`: `"PM agent dependencies not installed"`

---

### POST /trigger/boardroom

**Caller**: Go client (`devtrack boardroom "<problem>"`)
**Auth**: `X-DevTrack-API-Key`
**Timeout**: 180 seconds (seven LLM calls + synthesis)
**Description**: Runs a full multi-persona plan review. Seven AI personas (architect,
security, PM, devil's advocate, engineer, analyst, scalability) each evaluate the plan,
then the session synthesises a SWOT matrix and a final verdict.

**Request body** — inline plan:
```json
{
  "plan_text":     "Build a microservices architecture for the payment system",
  "output_format": "terminal"
}
```

**Request body** — Markdown file:
```json
{
  "markdown":      "# Payment System Plan\n\n...",
  "output_format": "markdown"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `plan_text` | string | yes (or `markdown`) | Free-text plan or problem statement |
| `markdown` | string | yes (or `plan_text`) | Full Markdown plan file contents |
| `output_format` | string | no | `"terminal"` (default) or `"markdown"` |

**Response** `200 OK`:
```json
{
  "report":          "## Boardroom Review\n\n### Architect (Alex)...",
  "verdict":         "PROCEED",
  "verdict_summary": "Six of seven personas recommend proceeding with minor revisions.",
  "approve":         6,
  "revise":          1,
  "reject":          0,
  "pros":            ["Well-scoped problem", "Clear success criteria"],
  "cons":            ["No rollback strategy described", "Cost estimate missing"]
}
```

| Field | Type | Description |
|---|---|---|
| `report` | string | Full formatted report (terminal ANSI or Markdown depending on `output_format`) |
| `verdict` | string | `"PROCEED"`, `"REVISE"`, or `"RECONSIDER"` |
| `verdict_summary` | string | One-sentence explanation of the verdict |
| `approve` | int | Number of personas voting PROCEED |
| `revise` | int | Number of personas voting REVISE |
| `reject` | int | Number of personas voting RECONSIDER |
| `pros` | string[] | Consolidated pros from all personas |
| `cons` | string[] | Consolidated cons from all personas |

**Error** `400`: `"'plan_text' or 'markdown' field required"`
**Error** `503`: `"Boardroom dependencies not installed"`

---

### POST /trigger/boardroom/chat

**Caller**: Go client (`devtrack boardroom --interactive` follow-up turns)
**Auth**: `X-DevTrack-API-Key`
**Timeout**: 120 seconds
**Description**: One turn of an interactive boardroom conversation. The client
maintains the `history` array and passes it back on each call. The server selects
which persona(s) respond based on the message content and `addressed_to`.

**Request body** — normal turn:
```json
{
  "plan_text":    "Build a microservices architecture for the payment system",
  "history":      [
    {"role": "user",    "content": "What are the biggest risks?"},
    {"role": "persona", "content": "The main risk is...",
     "persona_id": "security", "persona_name": "Sam (Security)"}
  ],
  "user_message": "How do we handle service discovery?",
  "addressed_to": "architect"
}
```

**Request body** — closing turn (ends session):
```json
{
  "plan_text":  "...",
  "history":    [...],
  "final_say":  "Thank you, I will proceed with the modular approach."
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `plan_text` | string | yes | Original plan/problem (repeated each turn for context) |
| `history` | object[] | yes | Full conversation history so far |
| `history[].role` | string | yes | `"user"`, `"persona"`, or `"system"` |
| `history[].content` | string | yes | Message text |
| `history[].persona_id` | string | no | Persona ID (for `role="persona"` entries) |
| `history[].persona_name` | string | no | Persona display name |
| `user_message` | string | yes (or `final_say`) | What the user typed this turn |
| `addressed_to` | string | no | Persona ID to address directly (e.g. `"security"`) |
| `final_say` | string | yes (or `user_message`) | Closing statement; triggers session close |

**Response** `200 OK` — normal turn:
```json
{
  "responses": [
    {
      "persona_id":   "architect",
      "persona_name": "Alex (Architect)",
      "role":         "architect",
      "content":      "For service discovery I recommend..."
    }
  ],
  "updated_history": [...],
  "session_closed":  false,
  "closing_summary": null
}
```

**Response** `200 OK` — closing turn:
```json
{
  "responses":       [],
  "updated_history": [...],
  "session_closed":  true,
  "closing_summary": "The boardroom concludes that the modular approach is well-suited..."
}
```

| Field | Type | Description |
|---|---|---|
| `responses` | object[] | Persona replies this turn (empty on closing turn) |
| `responses[].persona_id` | string | Persona identifier |
| `responses[].persona_name` | string | Persona display name |
| `responses[].role` | string | Persona role label |
| `responses[].content` | string | Persona's response text |
| `updated_history` | object[] | Full conversation history including this turn |
| `session_closed` | bool | `true` only when `final_say` was provided |
| `closing_summary` | string\|null | Session summary (only when `session_closed=true`) |

**Error** `400`: `"'plan_text' is required"` or `"'user_message' or 'final_say' required"`
**Error** `503`: `"Boardroom dependencies not installed"`

---

## Inbound Webhook Endpoints

These endpoints receive events pushed from external platforms. They are NOT called by
the Go client. The caller is the external service (Azure DevOps, GitHub, GitLab, Jira).
Auth is handled per-platform as described in the Authentication section above.

### POST /webhooks/azure-devops

**Caller**: Azure DevOps service hook
**Auth**: HTTP Basic Auth (`WEBHOOK_AZURE_USERNAME` / `WEBHOOK_AZURE_PASSWORD`)
**Description**: Receives Azure DevOps service hook events (work item assigned, comment
added, state changed, etc.).

**Request body**: Azure DevOps service hook payload (see Azure documentation).
Required fields: `eventType` (string), `resource` (object).

**Response** `200 OK`: handler-specific JSON result.

---

### POST /webhooks/github

**Caller**: GitHub webhook
**Auth**: HMAC-SHA256 signature in `X-Hub-Signature-256` header (`WEBHOOK_GITHUB_SECRET`)
**Description**: Receives GitHub webhook events (issues, pull requests, comments, etc.).
The event type is read from the `X-GitHub-Event` header.

**Request body**: GitHub webhook payload (see GitHub documentation).

**Response** `200 OK`: handler-specific JSON result.

---

### POST /webhooks/gitlab

**Caller**: GitLab webhook
**Auth**: `X-Gitlab-Token` header (`WEBHOOK_GITLAB_SECRET`)
**Description**: Receives GitLab webhook events (issue events, merge request events,
comments, etc.). The event type is read from the `X-Gitlab-Event` header.

**Request body**: GitLab webhook payload (see GitLab documentation).

**Response** `200 OK`: handler-specific JSON result.

---

### POST /webhooks/jira

**Caller**: Jira webhook
**Auth**: None (rely on network controls)
**Description**: Receives Jira webhook events (issue assigned, comment added, status
changed, etc.). The event type is read from the `webhookEvent` field in the body.

**Request body**: Jira webhook payload (see Jira documentation).

**Response** `200 OK`: handler-specific JSON result.

---

## Internal / Browser Endpoints

These endpoints are not called by the Go client. They are either browser-facing
(admin console, spec review) or internal server-to-server.

### GET /admin/* and POST /admin/*

Admin console. Browser-only. Auth: JWT cookie issued by `POST /admin/login`.
Not part of the client-server contract.

### GET /spec/{spec_id}/review

Renders an HTML spec review form for the PM. Browser-only.

### POST /spec/{spec_id}/review

Handles spec approval or change request submitted via the HTML form. Browser-only.

---

_See `docs/split-manifest.md` Section 5 for the original endpoint extraction from source._
_Source of truth for client request shapes: `devtrack-bin/http_trigger.go` (Go structs)._
_Source of truth for server request handling: `backend/webhook_server.py` (FastAPI handlers)._
