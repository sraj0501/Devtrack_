---
name: Platform modes and Windows gap
description: Managed/Lightweight/External modes; Windows compile errors and workaround
type: project
---

## Operating Modes (`devtrack-bin/server_config.go`)

| Mode | `DEVTRACK_SERVER_MODE` | Behaviour |
|------|----------------------|-----------|
| Managed (default) | `managed` | Spawns Python backend as subprocess |
| Lightweight | `lightweight` | Git monitoring + scheduling only; 28 backend commands blocked |
| External | `external` | Daemon only; Python on separate server via `DEVTRACK_SERVER_URL` |

`IsLightweightMode()` guards `requiresManagedMode()` in `cli.go`. Never read `DEVTRACK_SERVER_MODE` directly — use `GetServerMode()`. New commands needing Python must be added to the guard list.

## Windows Gap

Go daemon does not compile natively on Windows (`Setsid`, `SIGUSR2` are Unix-only).
**Fix plan:** Split `daemon_unix.go`/`daemon_windows.go` with build tags; replace `SIGUSR2` with named pipe or HTTP signal.
**Current workaround:** WSL2 (linux_amd64 binary).
