# Installation Guide

DevTrack is a **client-server tool**: the Go binary (`devtrack`) is the local daemon/client, and the Python backend handles AI, NLP, integrations, and reports. The two components are installed separately.

---

## Prerequisites

| Dependency | Version | Notes |
|---|---|---|
| Python | 3.12+ | Runs the AI/NLP backend |
| uv | latest | Python package manager — installed automatically if missing |
| Ollama | latest | Optional — local LLM. Can use OpenAI/Anthropic/Groq instead |

> Go is **not** required. The `devtrack` binary is distributed as a pre-built executable.

---

## Step 1 — Install the `devtrack` binary

Binaries are published to the **GitLab Package Registry** for `devtrack_client`. Download the archive for your platform and extract it:

```bash
# Fetch the latest tag name, then download the matching archive
GITLAB_PROJECT="devtrack3_cloud%2Fdevtrack_client"
TAG=$(curl -fsSL "https://gitlab.com/api/v4/projects/${GITLAB_PROJECT}/repository/tags?order_by=version&sort=desc&per_page=1" \
      | python3 -c "import sys,json; print(json.load(sys.stdin)[0]['name'])")
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH="amd64"; [ "$ARCH" = "aarch64" ] && ARCH="arm64"
curl -fsSL "https://gitlab.com/api/v4/projects/${GITLAB_PROJECT}/packages/generic/devtrack/${TAG}/devtrack_${OS}_${ARCH}.tar.gz" | tar xz
sudo mv devtrack /usr/local/bin/
```

**Windows** — download the `.zip` from the same URL (replace `.tar.gz` with `.zip` and `OS=windows`), extract, and copy `devtrack.exe` to a directory on your `PATH`. If you need elevated permissions, re-run the copy step as Administrator.

Verify:
```bash
devtrack --version
```

---

## Step 2 — Install the Python backend

Download the server package from the same GitLab release:

```bash
GITLAB_PROJECT="devtrack3_cloud%2Fdevtrack_client"
TAG=$(curl -fsSL "https://gitlab.com/api/v4/projects/${GITLAB_PROJECT}/repository/tags?order_by=version&sort=desc&per_page=1" \
      | python3 -c "import sys,json; print(json.load(sys.stdin)[0]['name'])")
curl -fsSL "https://gitlab.com/api/v4/projects/${GITLAB_PROJECT}/packages/generic/devtrack/${TAG}/devtrack-server-${TAG#v}.tar.gz" | tar xz
cd devtrack-server-*
devtrack-server install
```

This copies the backend files to `~/.local/share/devtrack-server`, installs the **core** Python dependencies, and adds `devtrack-server` to your PATH.

### Two-Tier Dependency Model

The Python backend uses a two-tier dependency model so the server runs lean by default:

| Tier | Installed by | Includes |
|------|-------------|---------|
| **core** (default) | `devtrack-server install` / `uv sync` | Web server, LLM providers, integrations (Azure/GitHub/GitLab/Jira), reports, Telegram, Slack |
| **ai** (optional) | `devtrack-server enable ai` | spaCy NLP parser, RAG personalization (ChromaDB + sentence-transformers), `en_core_web_sm` model |

The server starts and runs fully without the `ai` extra. NLP-powered features (smarter work-update parsing, RAG-based commit message style) are enabled on demand.

#### Check what is installed

```bash
devtrack-server features
```

Example output when `ai` is not yet installed:

```
── DevTrack Server Features ──

  ✓  core    web server, LLM, integrations, reporting
  ✗  ai      NLP parser, RAG personalization (run: devtrack-server enable ai)
```

#### Enable the AI extra

```bash
devtrack-server enable ai
```

This installs spaCy, ChromaDB, sentence-transformers, and the English NLP model into the server virtualenv, then reminds you to restart:

```
── Enabling AI features ──

  →  Installing ai extra into server venv...
  ✓  AI features installed
  !  Restart the server to apply: devtrack-server restart
```

After restarting, `devtrack-server features` will show both tiers with a checkmark.

---

## Step 3 — Configure

```bash
devtrack-server setup
```

The interactive wizard configures your `.env`. Three variables are required to start:

| Variable | Example | Description |
|---|---|---|
| `PROJECT_ROOT` | `~/.local/share/devtrack-server` | Where the server files live |
| `DEVTRACK_WORKSPACE` | `/home/you/myproject` | Git repo DevTrack monitors |
| `DATA_DIR` | `~/.local/share/devtrack-server/data` | Where logs, DB, and reports are stored |

See [CONFIGURATION.md](CONFIGURATION.md) for all variables.

---

## Step 4 — Start

```bash
devtrack-server start       # start the Python backend
devtrack start              # start the Go daemon (connects to the backend)
devtrack status             # verify everything is running
```

For persistent auto-start on login:

```bash
devtrack autostart-install   # installs launchd (macOS) or systemd unit (Linux)
```

---

## Optional: Shell Integration

Route `git commit` through DevTrack without the `devtrack` prefix:

```bash
echo 'eval "$(devtrack shell-init)"' >> ~/.zshrc   # or ~/.bashrc
source ~/.zshrc
```

Then opt repos in:

```bash
cd /path/to/your/repo
devtrack enable-git      # sets git config devtrack.enabled=true
```

After this, `git commit` in that repo runs the full AI enhancement flow. Repos listed in `workspaces.yaml` are intercepted automatically.

See [Git Features](GIT_FEATURES.md) for details.

---

## Option B — Docker (Python Backend in a Container)

```bash
# Run the Python backend in Docker (exposes port 8089)
docker run -p 8089:8089 --env-file ~/.local/share/devtrack-server/.env ghcr.io/sraj0501/devtrack-server:latest

# Or start everything (Python server + MongoDB + Redis) with compose
docker compose up
```

Configure the Go binary to connect to it:

```bash
# In .env
DEVTRACK_SERVER_MODE=external
DEVTRACK_SERVER_URL=http://localhost:8089
```

---

## Optional: Ollama (Local AI)

```bash
# Install: https://ollama.com/download
ollama pull llama3
ollama serve

# macOS background service
brew services start ollama
```

Set in `.env`:
```bash
LLM_PROVIDER=ollama
OLLAMA_HOST=http://localhost:11434
GIT_SAGE_DEFAULT_MODEL=llama3
```

To use a cloud provider instead, see [LLM Guide](LLM_GUIDE.md).

---

## Verification

```bash
devtrack status                  # daemon running?
devtrack-server status           # backend running?
devtrack help                    # all commands
```

---

## Uninstall

```bash
devtrack stop
devtrack-server uninstall        # removes backend files and stops the server
sudo rm /usr/local/bin/devtrack  # remove the binary
```

---

## Troubleshooting

| Problem | Fix |
|---|---|
| `devtrack: command not found` | Check `echo $PATH` contains `/usr/local/bin` |
| `devtrack-server: command not found` | Run `export PATH="$HOME/.local/bin:$PATH"` then re-run install |
| `spaCy model not found` | Run `devtrack-server enable ai` — it installs spaCy and the model automatically |
| NLP features not working | Run `devtrack-server features` to check whether the `ai` extra is installed |
| `IPC connection failed` | Check port: `lsof -i :35893`. Change `IPC_PORT` in `.env` if in use |
| `.env not found` | Run `devtrack-server setup` |
| `missing required environment variables` | Re-run `devtrack-server setup` or check `.env` at `~/.local/share/devtrack-server/.env` |
| `Ollama unreachable` | `ollama serve` in a separate terminal |
| `git commit` still uses plain git | Run `source ~/.zshrc`, check `eval "$(devtrack shell-init)"` is in your shell config |

More: [Troubleshooting Guide](TROUBLESHOOTING.md)

---

**Next:** [Quick Start Guide](QUICK_START.md) — get up and running in 5 minutes.
