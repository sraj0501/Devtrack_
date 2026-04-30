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

Download the pre-built binary for your platform from the [latest release](https://github.com/sraj0501/Devtrack_/releases/latest):

```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH="amd64"; [ "$ARCH" = "aarch64" ] && ARCH="arm64"
curl -fsSL "https://github.com/sraj0501/Devtrack_/releases/latest/download/devtrack_${OS}_${ARCH}.tar.gz" | tar xz
sudo mv devtrack /usr/local/bin/
```

Verify:
```bash
devtrack --version
```

---

## Step 2 — Install the Python backend

Download the server package from the same release page:

```bash
curl -fsSL "https://github.com/sraj0501/Devtrack_/releases/latest/download/devtrack-server-$(curl -fsSL https://api.github.com/repos/sraj0501/Devtrack_/releases/latest | grep tag_name | cut -d'"' -f4 | tr -d v).tar.gz" | tar xz
cd devtrack-server-*
devtrack-server install
```

This copies the backend files to `~/.local/share/devtrack-server`, installs Python dependencies, downloads the spaCy NLP model, and adds `devtrack-server` to your PATH.

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
| `spaCy model not found` | `cd ~/.local/share/devtrack-server && uv run python -m spacy download en_core_web_sm` |
| `IPC connection failed` | Check port: `lsof -i :35893`. Change `IPC_PORT` in `.env` if in use |
| `.env not found` | Run `devtrack-server setup` |
| `missing required environment variables` | Re-run `devtrack-server setup` or check `.env` at `~/.local/share/devtrack-server/.env` |
| `Ollama unreachable` | `ollama serve` in a separate terminal |
| `git commit` still uses plain git | Run `source ~/.zshrc`, check `eval "$(devtrack shell-init)"` is in your shell config |

More: [Troubleshooting Guide](TROUBLESHOOTING.md)

---

**Next:** [Quick Start Guide](QUICK_START.md) — get up and running in 5 minutes.
