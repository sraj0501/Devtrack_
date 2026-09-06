# Architecture

The canonical architecture reference is [`docs/ARCHITECTURE.md`](../../../docs/ARCHITECTURE.md).
This page is the short user-facing summary.

```text
devtrack_client (Go)                    devtrack_server (Python)
local SQLite                            PostgreSQL (required)
git monitor, queue, MCP   HTTPS JSON    LLM, reports, voice, admin
connectors, alerts, TUI  ------------> webhook_server.py
```

## Ownership rules

- The Go daemon is the product and remains useful without Python.
- The Go client never connects to PostgreSQL.
- The Python server does not read Go-owned SQLite tables in PostgreSQL mode.
- Explicit manual and scheduled EOD requests send only the selected local day's minimal commit
  summaries to the Python server; this keeps PostgreSQL-backed reports complete without enabling
  continuous client-event synchronization.
- New client/server messages use authenticated HTTP/JSON, not legacy TCP IPC.
- Go owns PM connectors, alerts, Telegram, MCP, and `gitsage/`.
- Python owns LLM enrichment, report generation, personalization, boardroom, and the admin UI.
- Every non-administrative server capability must remain reachable through `devtrack`.

## Deployment modes

| Mode | Behavior |
|---|---|
| `managed` | The daemon manages a local Python server; setup bootstraps it in the background. |
| `lightweight` | Go-native monitoring, queue, MCP, scheduling, and connectors only. |
| `external` | The Go client connects to a separately operated Python server. |

PostgreSQL is mandatory for the Python server. Local SQLite remains the client's offline source of
truth in every mode. "Today" in client history and MCP output follows the user's local calendar
date, including timestamps retained from older installations.
