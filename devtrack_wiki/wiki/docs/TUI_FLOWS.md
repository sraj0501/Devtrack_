# TUI flows

The Go TUI is a visibility and correction surface. It does not host capabilities that disappear when
the TUI is closed, and it never prompts from the daemon's normal commit or timer flow.

## Client TUI

```bash
devtrack tui
```

It shows overview, activity, pending actions, alerts, and workspaces. Queue actions can be approved,
rejected, or edited. Equivalent correction commands remain available through the CLI and supported
notification channels.

## Ticket picker

The ticket picker belongs only to the explicitly invoked interactive `devtrack git commit` wrapper.
It is not part of silent commit observation and does not gate the daemon.

## Python server TUI

Server operators can run:

```bash
cd devtrack_server
uv run python -m backend.server_tui
```

This Textual process monitor is server-owned and is not exposed as `devtrack server-tui`. In
PostgreSQL mode, its trigger stats come from the Go daemon's internal HTTP stats endpoint rather than
from Go-owned SQLite tables.
