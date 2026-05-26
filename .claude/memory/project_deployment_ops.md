---
name: Deployment operations
description: devtrack upgrade command and devtrack-server bash CLI for External-mode deployments
type: project
---

## `devtrack upgrade`

Fetches from **GitLab** Releases API (`devtrack3_cloud/devtrack_client`) — not GitHub. Assets: `devtrack_{GOOS}_{GOARCH}.zip` (Windows) / `.tar.gz` (others). Auto-runs versioned migrations then restarts daemon.

**Migrations** (`~/.devtrack/migrations.json`): idempotent, append-only. Add new entries only at end of `allMigrations` in `migrations.go`.

## `devtrack-server` (Bash CLI)

Manages tarball-deployed Python backend for External/Lightweight users. Installed to `~/.local/bin/devtrack-server`; home: `~/.local/share/devtrack-server/` (override via `DEVTRACK_SERVER_HOME`). `upgrade` preserves `.env` and `workspaces.yaml`. Debug External-mode: `devtrack-server status` + `devtrack-server logs` first.
