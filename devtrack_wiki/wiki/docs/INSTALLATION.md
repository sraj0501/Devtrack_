# DevTrack Installation Guide

Complete step-by-step installation instructions for all platforms.

---

## Architecture Overview

DevTrack is split into two independent codebases:

| Component | Repo | What it does |
|---|---|---|
| **devtrack** binary | `devtrack_client` | Go daemon — git monitor, scheduler, CLI, git-sage. Works standalone. |
| **devtrack_server** | `devtrack_server` | Python AI pipeline, admin UI, PM integrations. Optional. |

The binary works fully offline with local Ollama. The server unlocks richer AI features (boardroom, validated LLM task enrichment, admin GUI, Azure/GitHub/Jira integrations). Set `DEVTRACK_SERVER_URL` in your `.env` to activate it.

---

## Which Installation Path?

Choose one:

1. **[Client only (offline-first)](#client-only-installation)** — just the `devtrack` binary + Ollama
   - Works without any server
   - Git monitoring, commit enhancement, git-sage, all CLI commands
   - ~10 minutes

2. **[Client + Server](#client--server-installation)** — binary plus the Python AI pipeline
   - Unlocks boardroom, LLM task enrichment, admin GUI, PM integrations (Jira, Azure, GitHub)
   - ~20 minutes

3. **[Docker](#docker-installation)** — containerised server for Windows or consistent cross-platform setup
   - Client binary still runs natively; only the server runs in Docker

---

## Prerequisites Overview

**All paths require:**
- [ ] Git
- [ ] Go 1.21+
- [ ] 2 GB free disk space

**Client + Server additionally requires:**
- [ ] Python 3.12 or 3.13 (NOT 3.14+)
- [ ] uv package manager

**Docker path additionally requires:**
- [ ] Docker and Docker Compose

---

## Client-Only Installation

### Step 1: Install Go

#### macOS
```bash
brew install go
go version  # 1.21 or higher
```

#### Linux (Ubuntu/Debian)
```bash
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

#### Windows (PowerShell)
```powershell
scoop install go
go version
```

### Step 2: Clone and Build the Binary

```bash
git clone https://github.com/sraj0501/Devtrack_.git
cd devtrack_client
go build -o devtrack .

# Verify
./devtrack version
```

#### Install globally (optional but recommended)

**macOS/Linux:**
```bash
mkdir -p ~/.local/bin
mv devtrack ~/.local/bin/
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
which devtrack
```

**Windows (PowerShell):**
```powershell
# Move to a directory already in your PATH, e.g. C:\Users\<you>\go\bin
Move-Item devtrack.exe "$env:GOPATH\bin\devtrack.exe"
```

### Step 3: Configure Environment

```bash
cp .env_sample .env
nano .env  # or your preferred editor
```

**Required variables (client-only):**

| Variable | Example | Notes |
|---|---|---|
| `PROJECT_ROOT` | `/home/user/devtrack_client` | Absolute path to the cloned repo |
| `DEVTRACK_WORKSPACE` | `/home/user/myproject` | Git repo to monitor |
| `DATA_DIR` | `${PROJECT_ROOT}/Data` | Logs, SQLite DB, PIDs |
| `IPC_HOST` | `127.0.0.1` | Leave as default |
| `IPC_PORT` | `35893` | Leave as default |
| `IPC_CONNECT_TIMEOUT_SECS` | `5` | IPC connection timeout |
| `HTTP_TIMEOUT_SHORT` | `10` | Short HTTP timeout (seconds) |
| `HTTP_TIMEOUT` | `30` | Standard HTTP timeout |
| `HTTP_TIMEOUT_LONG` | `60` | Long HTTP timeout |
| `IPC_RETRY_DELAY_MS` | `2000` | IPC retry delay (ms) |
| `LLM_PROVIDER` | `ollama` | `ollama`, `openai`, or `anthropic` |
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama server URL |
| `GIT_SAGE_DEFAULT_MODEL` | `llama3` | Model for git-sage |
| `PROMPT_TIMEOUT_SIMPLE_SECS` | `30` | Simple prompt timeout |
| `PROMPT_TIMEOUT_WORK_SECS` | `120` | Work update prompt timeout |
| `PROMPT_TIMEOUT_TASK_SECS` | `60` | Task prompt timeout |
| `LLM_REQUEST_TIMEOUT_SECS` | `60` | LLM request timeout |
| `SENTIMENT_ANALYSIS_WINDOW_MINUTES` | `60` | Sentiment window |

See [Configuration Reference](CONFIGURATION.md) for the complete variable list.

### Step 4: Start Ollama (Optional, for AI features)

```bash
# macOS
brew services start ollama
ollama pull llama3

# Linux
sudo systemctl start ollama
ollama pull llama3

# Or run in foreground
ollama serve
```

### Step 5: Start DevTrack

```bash
# Source .env so all vars are in the environment
source .env   # or: set -a; source .env; set +a

# Start the daemon
devtrack start

# Verify
devtrack status
```

Expected output:
```
DevTrack daemon is running (PID: 12345)
Monitoring: /path/to/your/repo
IPC Server: 127.0.0.1:35893
```

---

## Client + Server Installation

Do the **Client-Only** steps above first, then continue here.

### Step 1: Clone the Server

```bash
git clone https://github.com/sraj0501/Devtrack_.git
cd devtrack_server
```

### Step 2: Install Python Dependencies

DevTrack server has two dependency tiers:

| Tier | Command | What it includes |
|---|---|---|
| **Core** (always required) | `uv sync` | FastAPI, LLM providers, PM integrations |
| **AI** (optional) | `uv sync --extra ai` | ChromaDB RAG for personalization, semantic matching |

```bash
# Install uv if not already installed
curl -LsSf https://astral.sh/uv/install.sh | sh

# Core (required)
uv sync

# AI tier (optional — adds ChromaDB RAG for personalization)
uv sync --extra ai
```

### Step 3: Configure Server URL in Client .env

In the `devtrack_client/.env` (or your shared `.env`), add:

```bash
# Point client at your local server instance
DEVTRACK_SERVER_URL=https://localhost:8089
DEVTRACK_API_KEY=your-api-key-here
```

The server URL activates the AI pipeline. Without it the binary falls back to local Ollama only.

### Step 4: Start the Server

```bash
# From devtrack_server directory, source its env
cd devtrack_server
source .env

uv run python -m backend.webhook_server
```

Or let the client manage the server as a subprocess (managed mode) — the daemon will spawn it automatically if `DEVTRACK_SERVER_URL` points to localhost.

### Step 5: Verify

```bash
# Check server health
curl http://localhost:8089/health

# Start client daemon (it will connect to the server)
devtrack start
devtrack health
```

---

## Docker Installation

Docker is best for running the **server** component with consistent dependencies, especially on Windows. The `devtrack` binary still runs natively on the host.

### Step 1: Clone the Server Repo

```bash
git clone https://github.com/sraj0501/Devtrack_.git
cd Devtrack_/devtrack_server
cp .env_sample .env
nano .env
```

### Step 2: Start Ollama on Host

The server container calls Ollama running on your host machine:

```bash
ollama serve
ollama pull llama3
```

### Step 3: Build and Run Container

```bash
DOCKER_BUILDKIT=1 docker compose build devtrack_server
docker compose up -d devtrack_server

# Verify
docker compose ps
docker compose logs devtrack_server
```

The server is the only service you need — DevTrack's state lives in local SQLite.

### Step 4: Install Client Binary (on host)

Follow the [Client-Only Installation](#client-only-installation) steps. Then set:

```bash
DEVTRACK_SERVER_URL=http://localhost:8089
```

in your client `.env`.

---

## Verification Checklist

- [ ] `devtrack version` shows version number
- [ ] `devtrack status` shows daemon running
- [ ] `devtrack health` shows green
- [ ] `.env` has all required variables set
- [ ] `Data/` directory created under your project root
- [ ] Ollama running (optional): `curl http://localhost:11434/api/tags`
- [ ] RAG works (optional, AI tier): `uv run python -c "import chromadb; print('OK')"`

---

## Troubleshooting Installation

### "devtrack: command not found"

```bash
# Check binary exists
ls devtrack_client/devtrack   # or devtrack.exe on Windows

# Rebuild
cd devtrack_client
go build -o devtrack .

# Check PATH
echo $PATH | tr ':' '\n' | grep local
```

### "IPC connection failed"

```bash
# Check port is free
lsof -i :35893

# Change port in .env if needed
IPC_PORT=35894

devtrack stop && devtrack start
```

### "ChromaDB not found" (server, AI tier)

```bash
cd devtrack_server
uv sync --extra ai
```

### "Ollama not running"

```bash
curl http://localhost:11434/api/tags  # should list models
ollama serve                           # start if not running
```

### "Python 3.14+ detected"

```bash
# uv resolves this automatically via pyproject.toml
# If not, pin explicitly:
uv --python python3.13 sync
```

### "DEVTRACK_SERVER_URL not set" warning

This warning is informational — the client works without it. Only set this if you have `devtrack_server` running.

---

## Optional: Enable PM Integrations

Add credentials to `.env` for Azure DevOps, GitHub, or Teams:

```bash
# Azure DevOps
AZURE_DEVOPS_ORG=your-organization
AZURE_DEVOPS_PROJECT=your-project
AZURE_DEVOPS_TOKEN=your-pat-token

# GitHub
GITHUB_TOKEN=your-github-token
GITHUB_REPO=username/repo

# Microsoft Teams
TEAMS_BOT_ID=your-bot-id
TEAMS_BOT_PASSWORD=your-bot-password
TEAMS_CHANNEL_ID=your-channel-id
```

See [LLM Guide](LLM_GUIDE.md) to configure AI providers.

---

## Uninstallation

### Client binary
```bash
rm ~/.local/bin/devtrack
rm -rf /path/to/devtrack_client
```

### Server
```bash
rm -rf /path/to/devtrack_server
```

### Docker
```bash
docker compose down -v
docker image rm devtrack-server
```

---

## Next Steps

1. [Quick Start Guide](QUICK_START.md) — first commands and workflow
2. [Configuration Reference](CONFIGURATION.md) — all env variables
3. [LLM Guide](LLM_GUIDE.md) — configure AI providers
4. [Getting Started](GETTING_STARTED.md) — concepts and architecture
