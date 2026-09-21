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

## Ticket correction

Commit-time ticket selection is not part of the DevTrack workflow. Ticket resolution runs in the
background; unlinked results are corrected later through explicit queue, work-session, or status
surfaces. TASK-160 will add the strict deterministic provenance and conflict contract.

## Python server TUI

Server operators can run:

```bash
cd devtrack_server
uv run python -m backend.server_tui
```

This Textual process monitor is server-owned and is not exposed as `devtrack server-tui`. In
PostgreSQL mode, its trigger stats come from the Go daemon's internal HTTP stats endpoint rather than
from Go-owned SQLite tables.
