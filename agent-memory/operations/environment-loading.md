---
name: AutoLoadEnv and devtrack setup
description: Env resolution order at daemon startup; setup wizard; test isolation pattern
type: project
---

`AutoLoadEnv()` in `main.go` resolves (first match wins): `DEVTRACK_ENV_FILE` → `~/.devtrack/devtrack.conf` → `.env` beside binary. Never overwrites existing env vars — shell/launchd/CI overrides always win.

`devtrack setup` generates the environment file under the resolved XDG DevTrack data home and writes
its absolute path to `~/.devtrack/devtrack.conf` for auto-load on future starts.

**Test isolation**: tests touching `DATABASE_DIR`-dependent code need `monkeypatch.setenv("DATABASE_DIR", str(tmp_path))` as autouse fixture to prevent cross-test SQLite contamination.
