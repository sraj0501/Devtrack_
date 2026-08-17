# Quick Start Guide

Get DevTrack up and running in 15 minutes.

---

## Prerequisites Checklist

Before starting, ensure you have:

- [ ] Go 1.21+ — `go version`
- [ ] Git — `git --version`
- [ ] Ollama (optional, for AI) — `ollama serve`
- [ ] Python 3.12 or 3.13 (only if installing the server) — `python3 --version`
- [ ] uv (only if installing the server) — `uv --version`

Missing something? See [Installation Guide](INSTALLATION.md) for detailed setup.

---

## 5-Minute Setup (Client Only)

This gets the `devtrack` binary running. The server (`devtrack_server`) is optional.

### Step 1: Clone and Build (2 min)

```bash
# Clone the client repo
cd ~/Documents  # or your preferred location
git clone https://github.com/sraj0501/Devtrack_.git
cd devtrack_client

# Build the binary
go build -o devtrack .

# Optional: install globally
mkdir -p ~/.local/bin
mv devtrack ~/.local/bin/
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

**Windows (PowerShell):**
```powershell
git clone https://github.com/sraj0501/Devtrack_.git
cd devtrack_client
go build -o devtrack.exe .
Move-Item devtrack.exe "$env:GOPATH\bin\devtrack.exe"
```

### Step 2: Configure (1 min)

```bash
cp .env_sample .env
nano .env  # or your preferred editor
```

**Minimum required variables:**

```bash
# Paths
PROJECT_ROOT=/Users/yourname/Documents/devtrack_client
DEVTRACK_WORKSPACE=/Users/yourname/Documents/myproject
DATA_DIR=${PROJECT_ROOT}/Data

# IPC
IPC_HOST=127.0.0.1
IPC_PORT=35893
IPC_CONNECT_TIMEOUT_SECS=5
IPC_RETRY_DELAY_MS=2000

# HTTP timeouts
HTTP_TIMEOUT_SHORT=10
HTTP_TIMEOUT=30
HTTP_TIMEOUT_LONG=60

# LLM
LLM_PROVIDER=ollama
OLLAMA_HOST=http://localhost:11434
GIT_SAGE_DEFAULT_MODEL=llama3

# Prompts
PROMPT_TIMEOUT_SIMPLE_SECS=30
PROMPT_TIMEOUT_WORK_SECS=120
PROMPT_TIMEOUT_TASK_SECS=60
LLM_REQUEST_TIMEOUT_SECS=60
SENTIMENT_ANALYSIS_WINDOW_MINUTES=60
```

Leave credential variables empty for now (OPENAI_API_KEY, AZURE_DEVOPS_TOKEN, etc.).

### Step 3: Start (1 min)

```bash
# Source .env so all vars are in the environment
source .env

# Start background daemon
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

## Adding the Server (Optional)

If you want LLM task enrichment, the boardroom feature, admin UI, or PM integrations:

```bash
# Clone the server
git clone https://github.com/sraj0501/Devtrack_.git
cd devtrack_server

# Install dependencies with uv (never pip)
uv sync                    # core deps always required
uv sync --extra ai         # optional: ChromaDB RAG for personalization

# Start the server
source .env
uv run python -m backend.webhook_server
```

Then add to your client `.env`:
```bash
DEVTRACK_SERVER_URL=https://localhost:8089
DEVTRACK_API_KEY=your-api-key-here
```

Restart the daemon: `devtrack stop && devtrack start`

---

## First Run

### View Logs

```bash
devtrack logs

# Or follow the log file directly
tail -f Data/logs/daemon.log
```

---

## Try It Out

### Option A: Trigger with a Git Commit

```bash
# Navigate to monitored repository
cd ${DEVTRACK_WORKSPACE}

# Make a test commit
git add .
git commit -m "Test commit for DevTrack (1h)"

# Watch DevTrack process it
devtrack logs
```

Expected log entries:
```
[INFO] Commit detected: Test commit for DevTrack
[INFO] AI enhancement: Generated improved description
[INFO] Task update queued
```

### Option B: Force a Timer Trigger

```bash
devtrack force-trigger
# Interactive prompt: "What are you working on?"
# Type: "Working on PR #42 - fixing auth bug (2 hours)"
```

---

## Common Commands

```bash
# Daemon management
devtrack start          # Start monitoring
devtrack stop           # Stop daemon
devtrack status         # Show daemon status
devtrack logs           # View logs
devtrack health         # Health check

# Scheduler control
devtrack pause          # Pause scheduled triggers
devtrack resume         # Resume scheduler
devtrack force-trigger  # Trigger immediately

# Git features
devtrack git commit     # AI-enhanced commit
git-sage do "squash my last 3 commits"  # git-sage agentic mode
git-sage ask "how do I rebase onto main?"

# AI / boardroom
devtrack boardroom "should we rewrite the auth module?"
devtrack plan "add OAuth2 support"

# Information
devtrack version        # Show version
devtrack help           # Show all commands
```

---

## What's Running in the Background

After `devtrack start`:

```
├─ Go daemon (PID: 12345)
│  ├─ Git file monitor (fsnotify — watches for commits)
│  ├─ Cron scheduler (periodic triggers)
│  ├─ IPC server (127.0.0.1:35893)
│  └─ SQLite database (trigger history)
│
└─ Python webhook server (subprocess, :8089) — only if DEVTRACK_SERVER_URL set
   ├─ FastAPI HTTP server (receives HTTPS POST from Go daemon)
   ├─ LLM task enrichment (strict JSON; raw-text fallback)
   ├─ LLM client (Ollama / OpenAI / Anthropic)
   ├─ PM integrations (Azure, GitHub, Teams, Jira)
   └─ Admin UI (if ADMIN_EMBED=true)
```

All communication is local. No data leaves your machine unless you configure external integrations.

---

## Verify Everything Works

```bash
# 1. Daemon is running
devtrack status

# 2. Health check
devtrack health

# 3. Ollama running (if using AI)
curl http://localhost:11434/api/tags
```

---

## Stopping DevTrack

```bash
devtrack stop

# Verify stopped
devtrack status  # Should show: "DevTrack daemon is not running"
```

---

## Next Steps

- [Getting Started](GETTING_STARTED.md) — concepts and architecture
- [Configuration Reference](CONFIGURATION.md) — all env variables
- [LLM Guide](LLM_GUIDE.md) — configure AI providers
- [Git Features](GIT_FEATURES.md) — enhanced commits, git-sage
- [Troubleshooting](TROUBLESHOOTING.md) — common problems
- [Full Documentation](INDEX.md) — complete reference
