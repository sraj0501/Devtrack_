---
name: Platform modes and Windows gap
description: The non-obvious bits of managed/lightweight/external modes; Windows signal gap
type: project
---

The three modes (`managed` / `lightweight` / `external`) are documented in CLAUDE.md — only the non-obvious parts are recorded here:

- **Never read `DEVTRACK_SERVER_MODE` directly** — use `GetServerMode()` (`internal/config/config.go`). `IsLightweightMode()` guards `requiresManagedMode()` in `cli.go`; **any new command that needs Python must be added to that guard list**, or it fails obscurely in lightweight mode.
- PM connectors, gitsage, and alerts are Go-native and work in **all** modes — only AI/server commands are mode-gated.
- **Windows:** the Go daemon's Unix signals (`Setsid`, `SIGUSR2`) don't exist on Windows; the daemon targets linux_amd64 via WSL2 on the dev machine. The ARM64 `.syso` fix already shipped.
