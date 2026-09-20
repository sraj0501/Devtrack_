---
name: Platform modes and host portability
description: Mode resolution and cross-platform working rules
type: project
---

The documented configuration values are `managed`, `lightweight`, and `external`. The implementation has only `ServerModeManaged` and `ServerModeExternal`: `GetServerMode()` maps both `lightweight` and `external` to `ServerModeExternal`. Preserve the distinction between the user-facing value and current Go type.

- Centralize mode checks through `GetServerMode()` and `IsExternalServer()` in `internal/config/server_config.go`; do not duplicate direct `DEVTRACK_SERVER_MODE` parsing.
- `requiresManagedMode()` only errors for external/lightweight mode when `GetServerURL()` is empty. `GetServerURL()` falls back to loopback, so do not claim every Python-dependent command is categorically blocked without changing and testing the implementation.
- PM connectors, legacy Sage helpers, alerts, local SQLite, queueing, scheduling, and MCP are Go-native. Managed mode spawns the Python server; external/lightweight mode does not.
- Do not record one contributor machine as the permanent development host. Windows, Linux, and macOS remain supported. Prefer cross-platform commands and paths; isolate unavoidable differences behind existing platform-specific files and tests.
