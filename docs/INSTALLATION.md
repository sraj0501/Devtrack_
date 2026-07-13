# DevTrack Installation

## Prerequisites

| Dependency | Required | Notes |
|---|---|---|
| **Git** | Yes | Used for repository monitoring and server clone |
| **uv** | Yes (managed mode) | Python package manager — [install guide](https://docs.astral.sh/uv/getting-started/installation/) |
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
2. **Server clone** (managed mode only) — if the Python server is not already present, DevTrack sparse-clones only `devtrack_server/` (~5 MB) from GitHub into `~/.local/share/devtrack/server/devtrack_server/` and runs `uv sync` to install Python dependencies.
3. **Git repository** — the path DevTrack will monitor for commits.
4. **LLM provider** — Ollama (local, default), OpenAI, Anthropic, Groq, or skip (configure later in `.env`).
5. **Identity** — your email address (used to filter your own comments in integrations).
6. **PM platform** — GitHub Issues, Azure DevOps, Jira, or none.
7. **Directories** — creates `~/.local/share/devtrack/data/{db,logs,pids,reports,...}`.
8. **Config files** — writes `~/.local/share/devtrack/.env` and `~/.local/share/devtrack/workspaces.yaml`.
9. **Shell integration** — appends `eval "$(devtrack shell-init)"` to your shell RC file (`~/.zshrc` or `~/.bashrc`). This transparently routes `git commit`/`add`/`history` through DevTrack for monitored workspaces, honouring per-repo opt-in/out (`devtrack enable-git` / `disable-git`) and the `GIT_NO_DEVTRACK=1` bypass.
10. **Autostart** (optional) — installs a login item that starts the daemon automatically after login.

The `.env` path is registered in `~/.devtrack/devtrack.conf`. Subsequent `devtrack` commands load it automatically — no manual `source .env` is needed.

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

This upgrades the binary and runs `git pull + uv sync` on the Python server in managed mode.

---

## External mode (Python server on a separate host)

If you want to run the Python server separately rather than having the daemon manage it:

On the server host:
```sh
git clone https://github.com/sraj0501/Devtrack_.git
cd Devtrack_/devtrack_server
uv sync
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

**Server not starting**: Run `devtrack logs` and look for Python errors. Common causes: `uv sync` was not run, or a required env var is missing. Re-run `devtrack setup` to reset the configuration.

**Daemon already running**: `devtrack stop` then `devtrack start`.

**Port conflict**: `devtrack status` shows which ports are in use. Change `WEBHOOK_PORT` (default 8089) in `.env` if there is a conflict.