---
name: project_autoload_env
description: AutoLoadEnv() startup env resolution and devtrack setup wizard
type: project
originSessionId: 224dbc1c-de8b-4635-b8b1-b826b4c85afa
---
# Auto-Load .env at Daemon Startup

**Shipped**: April 23, 2026
**PR**: #79 (features/loadEnvs → main, in review)
**Files**: `devtrack-bin/loadenv.go`, `devtrack-bin/setup.go`

## AutoLoadEnv() — loadenv.go

`AutoLoadEnv()` is called at daemon startup before any command runs. It loads `.env` into the process environment so users no longer need to manually `source .env` before `devtrack start`.

**Resolution order** (first match wins):

1. `DEVTRACK_ENV_FILE` env var — explicit path to a `.env` file
2. `~/.devtrack/devtrack.conf` — user-level config written by `devtrack setup`
3. `.env` file adjacent to the binary

**Key rule**: `AutoLoadEnv()` never overwrites variables already present in the process environment. Existing env vars always take precedence (shell exports, launchd/systemd injections, CI overrides).

**Why:** Previously developers had to remember to `source .env` before every `devtrack start` in a fresh shell. Missing this step caused silent config errors. AutoLoadEnv makes startup reliable without changing the env-first architecture.

**How to apply:** The function runs in `main.go` before argument parsing. Do not call it again elsewhere. If a test needs specific env values, use `monkeypatch.setenv` or set them before calling the tested function — AutoLoadEnv will not overwrite them.

## devtrack setup — setup.go

`devtrack setup` is an interactive onboarding wizard for new installs.

**What it does:**

1. Prompts for LLM provider (ollama / openai / anthropic / groq)
2. Collects provider-specific credentials (API key, host URL, model name)
3. Asks for workspace path (the git repo to monitor)
4. Generates a complete `.env` file with all required variables
5. Writes `~/.devtrack/devtrack.conf` pointing to the generated `.env` so future `devtrack start` invocations auto-load it via `AutoLoadEnv()`

**Why:** Replaces the manual "copy .env_sample and fill in values" step. Lowers barrier to first successful run.

**How to apply:** Run once after install. Re-running is safe — it prompts before overwriting existing files. After setup, `devtrack start` works in any shell without sourcing `.env`.

## ~/.devtrack/devtrack.conf

A simple key=value file (one entry: `DEVTRACK_ENV_FILE=/path/to/.env`) created by `devtrack setup`. `AutoLoadEnv()` reads this to find the canonical `.env` path. This allows the binary and `.env` to live in different directories (common when the binary is on `$PATH`).

## Test Fix: TestProjectManager isolation

Commit `c1c05fa` added an `isolate_db` autouse pytest fixture to `TestProjectManager` that calls `monkeypatch.setenv("DATABASE_DIR", str(tmp_path))` before each test. This prevents stale SQLite data from one test polluting the next when tests run in the same process.

**Pattern to reuse:** Any test class that touches `DATABASE_DIR`-dependent code should use a similar `monkeypatch.setenv` fixture in `conftest.py` or as a class-level autouse fixture.
