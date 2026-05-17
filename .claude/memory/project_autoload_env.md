---
name: AutoLoadEnv and devtrack setup
description: Env resolution order at daemon startup; setup wizard; test isolation pattern
type: project
---

`AutoLoadEnv()` runs in `main.go` before argument parsing. Resolution order (first match wins):
1. `DEVTRACK_ENV_FILE` env var
2. `~/.devtrack/devtrack.conf` (written by `devtrack setup`)
3. `.env` adjacent to the binary

**Rule**: never overwrites existing env vars — shell exports, launchd/systemd, CI overrides always win.

`devtrack setup` generates `.env` + writes `~/.devtrack/devtrack.conf` so future starts auto-load without `source .env`.

**Test isolation pattern**: any test touching `DATABASE_DIR`-dependent code needs `monkeypatch.setenv("DATABASE_DIR", str(tmp_path))` as an autouse fixture — prevents stale SQLite data across tests in the same process.
