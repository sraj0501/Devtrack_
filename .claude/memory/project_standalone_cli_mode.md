---
name: Standalone-CLI mode (CS-standalone)
description: Three operating modes — Managed/Lightweight/External — selected at setup time; daemon guards backend-dependent commands in Lightweight mode
type: project
---

CS-standalone (TASK-021 through TASK-024, PR #82) introduced a three-mode deployment model so DevTrack can be used without a local Python backend.

**Why:** Developers on resource-constrained machines, Windows (WSL not viable), or shared CI boxes only need git monitoring and scheduling — they should not be forced to run a Python service. External mode supports teams that run a shared Python backend on a separate server.

**Modes (defined in `devtrack-bin/server_config.go`):**

| Mode | Constant | `DEVTRACK_SERVER_MODE` | Behaviour |
|---|---|---|---|
| Managed | `ServerModeManaged` | `managed` (default) | Daemon spawns Python backend as subprocess |
| Lightweight | `ServerModeLightweight` | `lightweight` | Git monitoring + scheduling only; no Python spawned; 28 backend commands blocked with clear error |
| External | `ServerModeExternal` | `external` | Daemon only; Python backend runs on a separate server; connects via `DEVTRACK_SERVER_URL` |
| Cloud | `ServerModeCloud` | n/a | Remote cloud-hosted backend; credentials in `~/.devtrack/cloud.json`; takes priority over env var |

`IsExternalServer()` returns `true` for Lightweight, External, and Cloud — all three skip Python subprocess spawning. `IsLightweightMode()` is the more specific check used by `requiresManagedMode()`.

**Mode selection in setup wizard (`devtrack-bin/setup.go`):**

Step 0 of `RunSetup()` prompts:
```
[1] Managed    (default) — full AI features
[2] Lightweight           — git monitoring + scheduling only
[3] External              — daemon only; Python runs on a separate server
```
The chosen mode is written as `DEVTRACK_SERVER_MODE=<mode>` in the generated `.env`.

**Capability guards (`devtrack-bin/cli.go` — `requiresManagedMode()`):**

28 CLI commands are guarded. In Lightweight mode they print:
```
This command requires Managed mode (DEVTRACK_SERVER_MODE=managed).
Current mode: lightweight
Re-run 'devtrack setup' and choose [1] Managed to enable AI features.
```
Guarded command groups: learning/*, report, server, azure, gitlab, github, and admin routes.

**Non-fatal path helpers (TASK-024):**

`GetEmailReporterPath()`, `GetLearningDailyScriptPath()`, and `GetPythonBridgePath()` were changed from `os.Exit(1)` to `(string, error)` returns. All callers handle the error gracefully — no more hard-exit on missing paths in non-managed modes.

**XDG home + shell integration (related setup hardening):**

`devtrack setup` also:
- Creates `~/.local/share/devtrack/` as the XDG data home (`devtrackDataHome()`) with all data subdirs
- Writes `workspaces.yaml` to `~/.local/share/devtrack/workspaces.yaml` (not repo-adjacent)
- Sets `WORKSPACES_FILE=` in `.env` pointing to that path
- Appends `eval "$(devtrack shell-init)"` to `.bashrc`/`.zshrc`/`config.fish` (idempotent)
- Auto-generates `ADMIN_SECRET_KEY` (random 32-byte hex)
- Checks `git` as a prerequisite for all modes

**How to apply:**
- Default to `ServerModeManaged` when writing new feature code — never assume a specific mode.
- Check `IsLightweightMode()` before calling Python-dependent helpers.
- When adding a new CLI command that requires the Python backend, add it to the `requiresManagedMode()` guard list in `cli.go`.
- Never access `DEVTRACK_SERVER_MODE` directly — use `GetServerMode()` which also handles the cloud credential priority.
