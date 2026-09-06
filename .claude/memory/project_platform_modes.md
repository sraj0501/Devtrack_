---
name: Platform modes and host portability
description: Actual managed/lightweight/external resolution and cross-platform working rules
type: project
---

The three documented configuration values are `managed`, `lightweight`, and `external`. The implementation has only `ServerModeManaged` and `ServerModeExternal`: `GetServerMode()` maps both the `lightweight` and `external` strings to `ServerModeExternal`. Preserve that distinction between the documented user-facing value and the current Go type.

- **Centralize mode checks:** use `GetServerMode()`/`IsExternalServer()` from `internal/config/server_config.go`; do not duplicate direct `DEVTRACK_SERVER_MODE` parsing in new code. There is currently no `IsLightweightMode()` helper.
- **Current guard behavior:** `requiresManagedMode()` is used by selected AI/server commands, but it only errors for external/lightweight mode when `GetServerURL()` is empty. `GetServerURL()` currently falls back to loopback, so do not claim that every Python-dependent command is categorically blocked in lightweight mode without changing and testing the implementation.
- PM connectors, gitsage, alerts, local SQLite, queueing, scheduling, and MCP are Go-native. Managed mode spawns the Python server; external/lightweight mode does not.
- **Validated Managed bootstrap:** the Windows validation path uses a non-blocking sparse checkout followed by `uv sync --extra ai`, prepares a generation-capable Ollama model and `nomic-embed-text`, and then performs bounded first-run voice seeding. This passed locally, but a clean Windows account/machine remains a release gate; do not treat one contributor-machine pass as clean-install evidence.
- **Core cross-platform E2E:** `scripts/e2e.ps1` and `scripts/e2e.sh` exercise an isolated lightweight client/daemon/SQLite/MCP flow with PM, telemetry, server sync, and automatic delivery disabled. The Windows launcher prefers WSL when Go is installed and otherwise uses a disposable Go Docker image without modifying the WSL distribution. Local Windows/Linux runs and hosted Windows/Ubuntu End-to-end run `34045590767` passed at `ed0f571`. This is portability evidence, not proof of the Managed PostgreSQL/Python/LLM boundary.
- **Host portability:** do not record one contributor machine as the permanent development host. This repository is currently being audited from native Windows, while Linux and macOS remain supported. Prefer cross-platform commands and paths; isolate unavoidable Windows/Unix differences behind the existing platform-specific files and tests.
