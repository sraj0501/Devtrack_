---
name: devtrack upgrade self-update command
description: Self-update command that downloads the latest release binary, applies versioned migrations, and auto-restarts the daemon
type: project
---

`devtrack upgrade` and `devtrack upgrade --check` are implemented in `devtrack-bin/upgrade.go`.

**Why:** Users needed a single command to stay current without manually downloading releases or knowing the install path. The command also runs any pending configuration migrations so the `.env` stays in sync after major version jumps.

**How it works (step by step):**

1. Calls `GET https://api.github.com/repos/sraj0501/Devtrack_/releases/latest`
2. Compares `tag_name` to `GetDevTrackVersion()` (skips if `"dev"` build or already current)
3. Finds the matching asset: `devtrack_{GOOS}_{GOARCH}.tar.gz`
4. Downloads and extracts the `devtrack` binary from the tarball into a temp file
5. Calls `replaceBinary(execPath, tmpFile)`:
   - First tries direct copy + atomic rename (works for user-writable install locations)
   - On `os.ErrPermission` (e.g. `/usr/local/bin`), retries with `sudo cp` — user sees normal password prompt
6. Calls `RunPendingMigrations()` (see `devtrack-bin/migrations.go`)
7. If daemon is currently running (checks PID file + `kill -0`), runs `devtrack restart`

**Versioned migrations (`devtrack-bin/migrations.go`):**

Migrations are idempotent, append-only, and tracked in `~/.devtrack/migrations.json`:

| ID | What it does |
|---|---|
| `001-create-xdg-home` | Creates `~/.local/share/devtrack/data/` subdirectory tree |
| `002-create-workspaces-yaml` | Creates `workspaces.yaml` in XDG home if missing |
| `003-add-workspaces-file-env` | Patches `WORKSPACES_FILE=` into `.env` if absent/empty |
| `004-generate-admin-secret-key` | Generates `ADMIN_SECRET_KEY` in `.env` if empty |

`MarkAllMigrationsApplied()` is called after a fresh `devtrack setup` so migrations never re-run what setup already did.

**`--check` flag:** Prints current vs latest version and exits without downloading anything.

**How to apply:**
- For root-owned installs (`/usr/local/bin`), tell users to run `sudo devtrack upgrade` — the internal `sudo cp` fallback means a second `sudo` prompt but it will work.
- Do not add new hardcoded logic to migrations; only add new `Migration` entries at the end of `allMigrations` in `migrations.go`.
- The migration state file lives at `~/.devtrack/migrations.json` (not in `DATA_DIR`).
