---
name: Deployment operations
description: Release script, devtrack-server bash CLI, and versioned migrations
type: project
---

## Releases

`scripts/release.ps1 [-Bump patch|minor|major]` — local release script. Builds 5 cross-compile targets, creates GitHub release via `gh`, updates wiki version badge, pushes to main. No CI/CD.

**Migrations** (`~/.devtrack/migrations.json`): idempotent, append-only. Add new entries only at end of `allMigrations` in `migrations.go`.

## `devtrack-server` (Bash CLI)

Manages tarball-deployed Python backend for External/Lightweight users. Installed to `~/.local/bin/devtrack-server`; home: `~/.local/share/devtrack-server/` (override via `DEVTRACK_SERVER_HOME`). `upgrade` preserves `.env` and `workspaces.yaml`. Debug External-mode: `devtrack-server status` + `devtrack-server logs` first.
