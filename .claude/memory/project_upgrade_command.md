---
name: devtrack upgrade self-update command
description: Self-update command that downloads the latest release binary from GitLab, applies versioned migrations, and auto-restarts the daemon
type: project
---

`devtrack upgrade` and `devtrack upgrade --check` are implemented in `devtrack-bin/upgrade.go`.

**Why:** Users needed a single command to stay current without manually downloading releases or knowing the install path. The command also runs any pending configuration migrations so the `.env` stays in sync after major version jumps.

**How it works (step by step):**

1. Calls `GET https://gitlab.com/api/v4/projects/devtrack3_cloud%2Fdevtrack_client/releases?per_page=1&order_by=released_at&sort=desc` — returns an array; `[0]` is the latest release.
2. Compares `tag_name` to `GetDevTrackVersion()` (skips if `"dev"` build or already current)
3. Finds the matching asset by name from `assets.links[]`: `devtrack_{GOOS}_{GOARCH}.zip` on Windows or `devtrack_{GOOS}_{GOARCH}.tar.gz` on Linux/macOS
4. Downloads the archive and extracts the `devtrack` binary into a temp file (`extractFromZip` for Windows, `extractFromTarGz` for others)
5. Calls `replaceBinary(execPath, tmpFile)`:
   - First tries direct copy + atomic rename (works for user-writable install locations)
   - On `os.ErrPermission`, delegates to `elevatedReplace(dst, src)` — platform-specific via build tags:
     - **Unix** (`upgrade_unix.go`, `//go:build !windows`): retries with `sudo cp` — user sees normal password prompt
     - **Windows** (`upgrade_windows.go`, `//go:build windows`): returns a "re-run as Administrator" guidance message (no sudo on Windows)
6. Calls `RunPendingMigrations()` (see `devtrack-bin/migrations.go`)
7. If daemon is currently running (checks PID file + `isProcessAlive`), runs `devtrack restart`

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
- Releases are on GitLab (`devtrack3_cloud/devtrack_client`), not GitHub. Do not change the API base back to GitHub.
- For root-owned installs (`/usr/local/bin`) on Unix, the internal `sudo cp` fallback handles it automatically.
- On Windows, if the binary is in a protected location, tell users to re-run as Administrator.
- Do not add new hardcoded logic to migrations; only add new `Migration` entries at the end of `allMigrations` in `migrations.go`.
- The migration state file lives at `~/.devtrack/migrations.json` (not in `DATA_DIR`).
