---
name: Platform modes and host assumptions
description: Non-obvious managed/lightweight/external rules and current Linux development host
type: project
---

The three modes (`managed` / `lightweight` / `external`) are documented in CLAUDE.md — only the non-obvious parts are recorded here:

- **Never read `DEVTRACK_SERVER_MODE` directly** — use `GetServerMode()` (`internal/config/config.go`). `IsLightweightMode()` guards `requiresManagedMode()` in `cli.go`; **any new command that needs Python must be added to that guard list**, or it fails obscurely in lightweight mode.
- PM connectors, gitsage, and alerts are Go-native and work in **all** modes — only AI/server commands are mode-gated.
- **Development host:** Zorin OS (Ubuntu-based Linux) with `zsh`. Use Linux paths and commands for
  repository work. Windows remains a supported release target with platform-specific locking,
  autostart, and process handling; do not treat WSL or an old Windows memory path as the local host.
