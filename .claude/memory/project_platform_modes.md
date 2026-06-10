---
name: Platform modes and Windows gap
description: Managed/Lightweight/External modes; Windows compile errors and workaround
type: project
---

## Operating Modes (`devtrack_client/internal/config/config.go`)

| Mode | `DEVTRACK_SERVER_MODE` | Behaviour |
|------|----------------------|-----------|
| Managed (default) | `managed` | Spawns Python `webhook_server.py` as subprocess (AI features) |
| Lightweight | `lightweight` | Git monitoring + scheduling + Go-native commands only; AI/server commands blocked |
| External | `external` | Daemon only; Python on separate server via `DEVTRACK_SERVER_URL` |

`IsLightweightMode()` guards `requiresManagedMode()` in `cli.go`. Never read `DEVTRACK_SERVER_MODE` directly — use `GetServerMode()`. New commands needing Python must be added to the guard list. (Post-decoupling, PM connectors, gitsage, and alerts are Go-native and work in all modes.)

## Windows

Go daemon Unix signals (`Setsid`, `SIGUSR2`) not present on Windows — daemon targets linux_amd64 via WSL2 on developer machine. ARM64 `.syso` fix already shipped (see CLAUDE.md).
