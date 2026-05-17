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

`IsExternalServer()` = true for Lightweight + External (skips Python spawn). `IsLightweightMode()` is the guard used by `requiresManagedMode()` in `cli.go`.

**How to apply:** Default to `ServerModeManaged` for new features. Check `IsLightweightMode()` before calling Python helpers. Any new command requiring Python backend must be added to the `requiresManagedMode()` guard list. Never read `DEVTRACK_SERVER_MODE` directly — use `GetServerMode()`.

## Windows Gap

Go daemon does not compile natively on Windows. Three hard errors:
- `cli.go` — `Setsid: true` (Unix-only)
- `cli.go` + `daemon.go` — `syscall.SIGUSR2` (undefined on Windows)

**Fix plan:** Split into `daemon_unix.go`/`daemon_windows.go` with build tags. Replace `Setsid` with `CREATE_NEW_PROCESS_GROUP`, `SIGUSR2` with named pipe or HTTP signal, SIGTERM-stop with `os.Process.Kill()`. Add Windows Service/Task Scheduler autostart path.

**Current workaround:** WSL2 (linux_amd64 binary runs natively).
