---
name: Deployment operations
description: devtrack upgrade command and devtrack-server bash CLI for External-mode deployments
type: project
---

## `devtrack upgrade`

Fetches latest from **GitLab** Releases API (`devtrack3_cloud/devtrack_client`) — not GitHub. Asset names: `devtrack_{GOOS}_{GOARCH}.zip` (Windows) or `.tar.gz` (Linux/macOS). Replace strategy: direct rename → falls back to `sudo cp` (Unix) or "re-run as Administrator" guidance (Windows). Auto-runs versioned migrations then restarts daemon if running.

**Migrations** (`~/.devtrack/migrations.json`): idempotent, append-only. Current: XDG home dirs, workspaces.yaml creation, `WORKSPACES_FILE` env patch, `ADMIN_SECRET_KEY` generation. Add new entries only at end of `allMigrations` in `migrations.go`.

## `devtrack-server` (Bash CLI)

Manages the tarball-deployed Python backend for External/Lightweight-mode users. Installed to `~/.local/bin/devtrack-server`. Server home: `~/.local/share/devtrack-server/` (override via `DEVTRACK_SERVER_HOME`).

Key commands: `install`, `setup` (interactive .env wizard), `start`, `stop`, `restart`, `status`, `logs`, `upgrade`, `uninstall`.

`upgrade` preserves `.env` and `workspaces.yaml` — only code files replaced. When debugging External-mode deployments, check `devtrack-server status` and `devtrack-server logs` first.
