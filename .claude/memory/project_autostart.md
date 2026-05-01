---
name: launchd/systemd Env-First Autostart
description: OS-agnostic auto-start for the DevTrack daemon on login, baking current .env vars at install time
type: feature
---

# launchd/systemd Env-First Autostart

**Shipped**: April 4, 2026
**Commit**: `f1d5f6c feat: launchd/systemd env-first autostart — bake env vars at install time`

## Why

launchd (macOS) and systemd (Linux) start processes with a minimal environment — no shell profile, no `.env` file loading. The solution is to bake all current devtrack env vars directly into the service definition file at install time. Re-running `autostart-install` after `.env` changes re-bakes with fresh values.

## CLI Commands

```bash
devtrack autostart-install      # OS-agnostic: picks launchd / systemd / profile fallback
devtrack autostart-uninstall    # Remove installed autostart entry
devtrack autostart-status       # Show whether autostart is configured and active

# Explicit macOS aliases (backward compat)
devtrack launchd-install
devtrack launchd-uninstall
```

## OS Detection (`detectOSType`)

Implemented in `devtrack-bin/cli.go`:

| Platform | Strategy |
|---|---|
| macOS | `~/Library/LaunchAgents/com.devtrack.plist` with `EnvironmentVariables` dict |
| Linux + systemd | `~/.config/systemd/user/devtrack.service` with `Environment=KEY=VALUE` lines |
| WSL without systemd | Appends `export KEY=VALUE` + `devtrack start &` to `~/.profile` |
| Linux without systemd | Same as WSL-nosystemd |

`hasSystemd()` checks `/run/systemd/private`, `cat /proc/1/comm`, then `pidof systemd`.

## Env-Baking Logic

`launchdEnvVarPrefixes` (and its systemd equivalent) capture env vars by prefix:

```
DEVTRACK_, IPC_, HTTP_, OLLAMA_, LMSTUDIO_, GIT_SAGE_,
PROMPT_TIMEOUT_, LLM_, SENTIMENT_, MONGODB_, JIRA_,
AZURE_, GITHUB_, GROQ_, ALERT_, DATA_DIR, DATABASE_DIR,
LEARNING_, PROJECT_ROOT, DEVTRACK_ENV_FILE, PATH
```

All currently set vars matching those prefixes are written into the plist/unit at install time. `PATH` is always included to find the binary.

## How to apply

- After any `.env` change that affects startup: re-run `devtrack autostart-install`
- Uninstall before re-install is NOT required — the command overwrites the existing file
- Check status with `devtrack autostart-status` to confirm the service is loaded
- Log output on macOS goes to `Data/logs/launchd.log`; on Linux to `Data/logs/systemd.log`

## Files

- `devtrack-bin/cli.go`: `handleLaunchdInstall`, `handleLaunchdUninstall`, `buildLaunchdPlist`, `buildSystemdService`, `detectOSType`, `handleAutostartInstall`, `handleAutostartUninstall`, `handleAutostartStatus`
