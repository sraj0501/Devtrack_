---
name: devtrack-server Management CLI
description: Bash script installed alongside the Go binary to manage the tarball-deployed Python backend
type: project
---

`devtrack-server` is a standalone Bash management CLI shipped as a release asset (`devtrack-server-{version}.tar.gz`). It wraps the tarball-deployed Python backend with a familiar lifecycle interface.

**Why:** When users deploy DevTrack in External or Lightweight mode, the Python backend lives in `~/.local/share/devtrack-server/` rather than being spawned by the Go daemon. `devtrack-server` gives those users a consistent way to install, configure, start/stop/restart, and upgrade the backend without having to know the underlying `uv run` invocation.

**Key facts:**

- File: `devtrack-server` (root of repo — Bash script, not a Go binary)
- Installed to `~/.local/bin/devtrack-server` by `devtrack-server install`
- Default server home: `~/.local/share/devtrack-server/` (override via `DEVTRACK_SERVER_HOME`)
- PID file: `$SERVER_HOME/devtrack-server.pid`
- Log file: `$SERVER_HOME/logs/server.log`
- Health check: `GET http://127.0.0.1:<WEBHOOK_PORT>/health`

**Commands:**

| Command | Purpose |
|---|---|
| `install` | Copies `backend/`, `pyproject.toml`, etc. to `$SERVER_HOME`; runs `uv sync`; seeds pip; downloads spaCy model |
| `setup` | Interactive `.env` wizard — sets `PROJECT_ROOT`, `DATA_DIR`, `WEBHOOK_PORT`, `LLM_PROVIDER`, `OLLAMA_HOST`, model |
| `start` | Launches `uv run python -m backend.webhook_server` as background process via `nohup`; waits up to 10s for health |
| `stop` | Sends SIGTERM to PID; force-kills after 10s if still running |
| `restart` | `stop` + `start` |
| `status` | Shows version, home, port, process state, and HTTP health |
| `logs` | `tail -f $SERVER_LOG` |
| `upgrade` | Downloads `devtrack-server-{ver}.tar.gz` from GitHub releases; preserves `.env`/`workspaces.yaml`; re-runs `uv sync`; restarts if was running |
| `uninstall` | Stops server; removes `$SERVER_HOME` and optionally `DATA_DIR` |
| `version` | Prints version from `$SERVER_HOME/VERSION` |

**Install does NOT overwrite user config:** `upgrade` preserves `.env` and `workspaces.yaml` — only code files are replaced.

**How to apply:** When documenting deployment or writing setup instructions, reference `devtrack-server install` (not `setup_local.sh` — that was removed in commit `1ae7966`). When debugging a user's External-mode deployment, check `devtrack-server status` and `devtrack-server logs` first.
