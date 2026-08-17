# DevTrack Installation

## Prerequisites

| Dependency | Required | Notes |
|---|---|---|
| **Git** | Yes | Used for repository monitoring and server clone |
| **uv** | Yes (managed mode) | Python package manager — [install guide](https://docs.astral.sh/uv/getting-started/installation/) |
| **PostgreSQL 16+** | Yes (Python server) | Required server persistence; local Compose or a reachable remote database |
| **Ollama** | No | Local AI inference — any LLM provider works; Ollama is the default |

Install uv (Linux / macOS):
```sh
curl -LsSf https://astral.sh/uv/install.sh | sh
```

Install uv (Windows PowerShell):
```powershell
powershell -ExecutionPolicy ByPass -c "irm https://astral.sh/uv/install.ps1 | iex"
```

---

## Install the DevTrack binary

**Linux / macOS (amd64)**:
```sh
curl -fsSL https://github.com/sraj0501/Devtrack_/releases/latest/download/devtrack_linux_amd64.tar.gz | tar xz
sudo mv devtrack /usr/local/bin/
```

**Linux arm64**:
```sh
curl -fsSL https://github.com/sraj0501/Devtrack_/releases/latest/download/devtrack_linux_arm64.tar.gz | tar xz
sudo mv devtrack /usr/local/bin/
```

**macOS arm64 (Apple Silicon)**:
```sh
curl -fsSL https://github.com/sraj0501/Devtrack_/releases/latest/download/devtrack_darwin_arm64.tar.gz | tar xz
sudo mv devtrack /usr/local/bin/
```

**Windows**: download `devtrack_windows_amd64.zip` from the [releases page](https://github.com/sraj0501/Devtrack_/releases/latest), extract, and add the folder to your `PATH`.

Verify:
```sh
devtrack --version
```

---

## Run setup

```sh
devtrack setup
```

The setup wizard walks through:

1. **Mode** — choose Managed (default) or External.
   - **Managed**: the daemon spawns the Python AI server automatically.
   - **External**: you run the Python server separately (see External mode below).
2. **Background server bootstrap** (managed mode only) — setup records
   `~/.local/share/devtrack/server/devtrack_server/` immediately, then a detached worker
   sparse-clones only `devtrack_server/` (~5 MB), runs `uv sync`, and pulls the configured model when
   the selected provider is local Ollama. The wizard does not wait for these steps.
3. **Server database** (managed mode) — requires a PostgreSQL connection URL and writes it as `POSTGRES_URL`.
4. **Git repository** — the path DevTrack will monitor for commits.
5. **LLM provider** — Ollama (local, default), OpenAI, Anthropic, Groq, or skip (configure later in `.env`).
6. **Identity** — your email address (used to filter your own comments in integrations).
7. **PM platform** — GitHub Issues, Azure DevOps, Jira, or none.
8. **Directories** — creates `~/.local/share/devtrack/data/{db,logs,pids,reports,...}`.
9. **Config files** — writes `~/.local/share/devtrack/.env` and `~/.local/share/devtrack/workspaces.yaml`.
10. **Shell integration** — appends `eval "$(devtrack shell-init)"` to your shell RC file (`~/.zshrc` or `~/.bashrc`). This transparently routes `git commit`/`add`/`history` through DevTrack for monitored workspaces, honouring per-repo opt-in/out (`devtrack enable-git` / `disable-git`) and the `GIT_NO_DEVTRACK=1` bypass.
11. **Autostart** (optional) — installs a login item that starts the daemon automatically after login.

The `.env` path is registered in `~/.devtrack/devtrack.conf`. Subsequent `devtrack` commands load it automatically — no manual `source .env` is needed.

The generated file writes the standard timeout, model, and local-service defaults explicitly so a
fresh install is immediately usable and every value remains visible and editable. Missing non-secret
runtime settings fall back to those same values; invalid overrides and required secrets still fail
with an actionable configuration error.

Go-native Git monitoring, SQLite, scheduling, and MCP remain available during bootstrap. Progress is
written atomically to the DevTrack data home and shown by both commands:

```sh
devtrack doctor             # capability map, active step, error, and bootstrap log
devtrack status             # daemon/service status plus the same capability map
devtrack doctor --repair    # retry safely after a failed or interrupted bootstrap
```

---

## Configure

After setup, open `~/.local/share/devtrack/.env` and fill in any tokens you skipped:

```
# PM platform tokens (secrets only — project config goes in workspaces.yaml)
GITHUB_TOKEN=ghp_...
AZURE_DEVOPS_PAT=...
GITLAB_PAT=...

# LLM provider (ollama | openai | anthropic | groq)
LLM_PROVIDER=ollama

# Ollama settings (if LLM_PROVIDER=ollama)
OLLAMA_HOST=http://localhost:11434
OLLAMA_MODEL=llama3.2
```

See `devtrack_client/.env_sample` for the full list of variables and their defaults.

---

## PostgreSQL deployment

The Python server has no SQLite fallback. It validates `POSTGRES_URL`, connects to PostgreSQL,
and applies Alembic migrations before opening for traffic. The Go client remains SQLite-backed and
continues observing, queueing, and serving MCP context when the server is unavailable.

### Local PostgreSQL for managed mode

The managed daemon runs Python on the host. You can provision only PostgreSQL with the bundled
Compose file:

```sh
cd /path/to/devtrack_server
cp .env_sample .env
# Set POSTGRES_PASSWORD to a strong local password.
# Set POSTGRES_URL using host `postgres` so Compose validation succeeds.
docker compose up -d postgres
docker compose ps postgres
```

When `devtrack setup` asks for the managed-server URL, use the host-facing address:

```text
postgresql://devtrack:<password>@localhost:5432/devtrack
```

The generated `~/.local/share/devtrack/.env` is mode `0600` and supplies that URL to the managed
Python child process.

### Full local Compose deployment

In `devtrack_server/.env`, set matching values and use the Compose service hostname in the URL:

```dotenv
POSTGRES_USER=devtrack
POSTGRES_PASSWORD=<strong-password>
POSTGRES_DB=devtrack
POSTGRES_URL=postgresql://devtrack:<strong-password>@postgres:5432/devtrack
```

Then start the server. Compose waits until `pg_isready` succeeds before launching Python:

```sh
docker compose up -d devtrack_server
docker compose ps
docker compose logs devtrack_server
```

### Remote PostgreSQL

Create a dedicated database and least-privilege role on the PostgreSQL host, allow network access
only from the DevTrack server, and require TLS. On the Python server host:

```dotenv
POSTGRES_URL=postgresql://<user>:<password>@<db-host>:5432/<database>?sslmode=require
```

Verify connectivity and migrate before the first start:

```sh
cd Devtrack_/devtrack_server
uv sync --frozen
uv run python -m backend.db.migrate upgrade
uv run python -m backend.db.migrate current
uv run python -m backend.webhook_server
```

If the URL is missing, malformed, unreachable, or points to a non-PostgreSQL backend, startup fails
with an actionable error instead of silently creating a local SQLite database.

Open `~/.local/share/devtrack/workspaces.yaml` and set project-specific details:

```yaml
workspaces:
  - name: "myproject"
    path: "/home/user/myproject"
    pm_platform: "github"
    pm_project: "owner/repo"
    enabled: true
```

---

## Start

```sh
devtrack start
devtrack status    # verify daemon + server are running
devtrack logs      # tail recent log output
```

---

## Verify

Make a test commit in the monitored repository — DevTrack should log the activity:

```sh
cd /path/to/your/repo
git commit --allow-empty -m "test: devtrack install"
devtrack logs
```

You should see a `trigger: type=commit` line in the output within a few seconds.

---

## Autostart

If you chose not to install autostart during setup:

```sh
devtrack autostart-install
```

This installs a launchd plist (macOS), a systemd user unit (Linux), or a Scheduled Task (Windows) that starts the daemon on login. The `.env` is baked in so no manual sourcing is needed.

---

## Upgrade

```sh
devtrack upgrade
```

This upgrades the binary immediately, then starts the managed Python checkout/update, `uv sync`, and
local Ollama model pull in the same non-blocking bootstrap worker. Follow it with `devtrack doctor`.

---

## External mode (Python server on a separate host)

If you want to run the Python server separately rather than having the daemon manage it:

On the server host:
```sh
git clone https://github.com/sraj0501/Devtrack_.git
cd Devtrack_/devtrack_server
uv sync
export POSTGRES_URL='postgresql://<user>:<password>@<db-host>:5432/<database>?sslmode=require'
uv run python -m backend.db.migrate upgrade
uv run python -m backend.webhook_server
```

On the client machine, set in `.env`:
```
DEVTRACK_SERVER_MODE=external
DEVTRACK_SERVER_URL=https://<host>:8089
DEVTRACK_API_KEY=<shared-secret>
```

Then:
```sh
devtrack start
devtrack status
```

---

## Troubleshooting

**`git` not found**: Install from <https://git-scm.com/downloads>. Required for both repository monitoring and the server clone.

**`uv` not found**: Install from <https://docs.astral.sh/uv/getting-started/installation/>. Required only in managed mode.

**`PROJECT_ROOT` already set**: If you previously set `PROJECT_ROOT` manually in your environment, DevTrack will use that path instead of the managed-install location. Unset it or point it at the `devtrack_server/` directory inside your managed-install path (`~/.local/share/devtrack/server/devtrack_server/`).

**Server not starting**: Run `devtrack doctor` first; it reports the bootstrap step, last error, and
log path while confirming which Go-native features still work. Common causes are missing `uv` or
Ollama, an incomplete model pull, a missing `POSTGRES_URL`, unreachable PostgreSQL, or incorrect
database credentials. Retry installation with `devtrack doctor --repair`. For database failures,
check with `uv run python -m backend.db.migrate current`, then re-run `devtrack setup` if the managed
URL is wrong.

**Daemon already running**: `devtrack stop` then `devtrack start`.

**Port conflict**: `devtrack status` shows which ports are in use. Change `WEBHOOK_PORT` (default 8089) in `.env` if there is a conflict.
