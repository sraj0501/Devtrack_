---
name: Platform modes and host portability
description: Mode resolution and cross-platform working rules
type: project
---

The documented configuration values are `managed`, `lightweight`, and `external`. The implementation has only `ServerModeManaged` and `ServerModeExternal`: `GetServerMode()` maps both `lightweight` and `external` to `ServerModeExternal`. Preserve the distinction between the user-facing value and current Go type.

- Centralize mode checks through `GetServerMode()` and `IsExternalServer()` in `internal/config/server_config.go`; do not duplicate direct `DEVTRACK_SERVER_MODE` parsing.
- `requiresManagedMode()` only errors for external/lightweight mode when `GetServerURL()` is empty. `GetServerURL()` falls back to loopback, so do not claim every Python-dependent command is categorically blocked without changing and testing the implementation.
- PM connectors, Sage capture and command-knowledge processing, alerts, local SQLite, queueing, scheduling, and MCP are Go-native. Managed mode spawns the Python server; external/lightweight mode does not.
- In external/lightweight mode with no `DEVTRACK_SERVER_URL`, `GetServerURL()` still tries the loopback
  default. An unavailable server degrades server-backed commands; the mode alone does not disable
  those calls. External mode also skips automatic first-run voice mining.
- On Windows, an isolated full-capability evaluation can use a hybrid external-mode layout: run
  PostgreSQL and the Python server in Docker, but keep the Go client native for host filesystem
  notifications, Git hooks, login-time autostart, and IPC. A Linux server container cannot resolve
  a Windows `repo_path` unless that repository is mounted and its path translated; without that
  mapping, diff enrichment degrades while commit-message processing and queue staging continue.
- Do not record one contributor machine as the permanent development host. Windows, Linux, and macOS remain supported. Prefer cross-platform commands and paths; isolate unavoidable differences behind existing platform-specific files and tests.
