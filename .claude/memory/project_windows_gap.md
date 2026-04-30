---
name: Windows Native Support Gap
description: devtrack Go daemon does not compile for Windows; Windows support is the immediate next platform target
type: project
---

The Go daemon (`devtrack-bin/`) does not compile for Windows and is the immediate next platform work item.

**Why:** Three hard compile errors on Windows:
- `cli.go:297` — `Setsid: true` in `SysProcAttr` (Unix-only, detaches daemon process session)
- `cli.go:578` — `syscall.SIGUSR2` (undefined on Windows, used for `devtrack trigger`)
- `daemon.go:380,387` — `syscall.SIGUSR2` (daemon listens for it to force trigger cycle)

Additional functional gaps even if it compiled:
- `SIGTERM`/`SIGHUP` — `devtrack stop` sends SIGTERM; Windows process termination is different
- Autostart — launchd (macOS) and systemd (Linux) have no Windows equivalent; needs Windows Service or Task Scheduler
- `osascript` notifications — macOS only (Python alert notifier)

**How to apply:** When planning Windows support work, scope it as:
1. Split signal/proc code into `daemon_unix.go` / `daemon_windows.go` with build tags
2. Replace `Setsid` with `CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP`
3. Replace `SIGUSR2` with named pipe or HTTP call for in-process signalling
4. Replace `SIGTERM`-based stop with `os.Process.Kill()` or Windows job object
5. Add Windows Service / Task Scheduler path for autostart

**Current workaround**: Use WSL (linux_amd64 binary works natively in WSL2).
