# Quick Start Guide

Get DevTrack up and running in 15 minutes.

---

## Prerequisites Checklist

Before starting, ensure you have:

- [ ] Go 1.20+ (`go version`)
- [ ] Python 3.12+ or 3.13 (`python3 --version`)
- [ ] uv package manager (`uv --version`)
- [ ] Git (`git --version`)
- [ ] Ollama (optional but recommended) (`ollama serve`)

Missing something? See [Installation Guide](INSTALLATION.md) for detailed setup.

---

## 5-Minute Setup

### Step 1: Clone and Configure (2 min)

```bash
# Clone repository
cd ~/Documents/GitHub  # or your preferred location
git clone https://github.com/yourusername/automation_tools.git
cd automation_tools

# Copy configuration template
cp .env_sample .env

# Edit configuration (open with your editor)
nano .env  # or use vim, code, etc.
```

**Key .env variables to set:**

```bash
PROJECT_ROOT=/Users/yourname/Documents/GitHub/automation_tools
DEVTRACK_WORKSPACE=${PROJECT_ROOT}
DATA_DIR=${PROJECT_ROOT}/Data
IPC_HOST=127.0.0.1
IPC_PORT=35893
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral
```

Leave credential variables empty for now (OPENAI_API_KEY, AZURE_DEVOPS_TOKEN, etc.).

### Step 2: Install Dependencies (2 min)

```bash
# Install Python dependencies
uv sync

# Download spaCy NLP model
uv run python -m spacy download en_core_web_sm
```

### Step 3: Build Go Daemon (1 min)

```bash
cd devtrack-bin
go build -o devtrack .

# Make it globally accessible (optional)
mv devtrack ~/.local/bin/
```

If you moved the binary, add `~/.local/bin` to your PATH:
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

Or run devtrack from the devtrack-bin directory:
```bash
../devtrack-bin/devtrack start
```

---

## First Run

### Start the Daemon

```bash
# Navigate to automation_tools directory
cd /path/to/automation_tools

# Start background daemon
devtrack start

# Verify it's running
devtrack status
```

Expected output:
```
DevTrack daemon is running (PID: 12345)
Monitoring: /path/to/monitored/repo
IPC Server: 127.0.0.1:35893
```

### View Logs

```bash
# Watch logs in real-time
devtrack logs -f

# Or follow specific log file
tail -f Data/logs/daemon.log
```

---

## Try It Out

### Option A: Trigger with a Git Commit

```bash
# Navigate to monitored repository
cd ${DEVTRACK_WORKSPACE}

# Make a test commit
git commit -m "Test commit for DevTrack (1h)"

# Watch logs to see DevTrack processing it
tail -f ~/path/to/Data/logs/daemon.log
```

You should see:
```
[INFO] Commit detected: Test commit for DevTrack
[INFO] NLP parsing: Extracted task info
[INFO] AI enhancement: Generated improved description
[INFO] Task update sent to project management
```

### Option B: Force a Timer Trigger

```bash
# Trigger manually (no waiting for schedule)
devtrack force-trigger

# This will show an interactive prompt asking what you're working on
# Type something like: "Working on user authentication (2 hours)"
# Press Enter and watch it process
```

---

## Common Commands

```bash
# Daemon management
devtrack start                 # Start monitoring
devtrack stop                  # Stop daemon
devtrack restart               # Restart with new config
devtrack status                # Show daemon status
devtrack logs                  # View logs
devtrack logs -f               # Follow logs (like tail -f)

# Scheduler control
devtrack pause                 # Pause scheduled triggers
devtrack resume                # Resume scheduler
devtrack force-trigger         # Trigger immediately (no waiting)
devtrack skip-next             # Skip next scheduled trigger

# Information
devtrack db-stats              # Show database statistics
devtrack version               # Show version
devtrack help                  # Show all commands
```

---

## Verify Everything Works

### 1. Check Daemon is Running

```bash
devtrack status
```

Should show:
- Daemon PID
- Monitoring location
- IPC server status

### 2. Check Python Bridge

```bash
tail Data/logs/python_bridge.log | head -20
```

Should show imports and initialization messages.

### 3. Check NLP Model

```bash
uv run python -c "import spacy; nlp = spacy.load('en_core_web_sm'); print('NLP ready')"
```

Should print: `NLP ready`

### 4. Check Ollama (if using AI)

```bash
curl http://localhost:11434/api/tags
```

Should show available models.

---

## Optional: Configure Project Integrations

To enable task updates to Azure DevOps, GitHub, or Teams, add credentials to `.env`:

### Azure DevOps

```bash
AZURE_DEVOPS_ORG=your-org
AZURE_DEVOPS_PROJECT=your-project
AZURE_DEVOPS_TOKEN=your-personal-access-token
```

[Get an Azure DevOps PAT](https://dev.azure.com)

### GitHub

```bash
GITHUB_TOKEN=your-github-personal-access-token
GITHUB_REPO=username/repo-name
```

[Create a GitHub PAT](https://github.com/settings/tokens)

### Teams

```bash
TEAMS_BOT_ID=your-bot-id
TEAMS_BOT_PASSWORD=your-bot-password
TEAMS_CHANNEL_ID=your-channel-id
```

[Setup Teams bot integration](https://docs.microsoft.com/en-us/azure/bot-service/)

---

## Next Steps

### Want to Learn More?

- **[Getting Started](GETTING_STARTED.md)** - Detailed overview
- **[Architecture Overview](ARCHITECTURE.md)** - How it works
- **[Git Features](GIT_FEATURES.md)** - Enhanced commits and more
- **[Configuration Reference](CONFIGURATION.md)** - All .env variables
- **[LLM Guide](LLM_GUIDE.md)** - AI configuration

### Having Issues?

- **[Troubleshooting Guide](TROUBLESHOOTING.md)** - Common problems
- **[Known Issues](../KNOWN_ISSUES.md)** - Known bugs and workarounds
- **[Full Documentation](INDEX.md)** - Complete reference

### Want to Contribute?

- **[Developer Guide](../CLAUDE.md)** - Architecture and development
- **[Implementation Plan](IMPLEMENTATION_PLAN.md)** - Planned features

---

## Disable Features

If any feature causes issues, you can disable it:

### Disable AI Enhancement

```bash
# In .env, set:
LLM_PROVIDER=none
```

Or stop Ollama and remove OpenAI/Anthropic keys.

### Disable Conflict Auto-Resolution

The conflict resolver gracefully degrades if dependencies are missing. It won't affect other features.

### Disable NLP Parsing

The NLP parser is critical but degrades gracefully:
- If spaCy isn't installed, basic string matching is used
- If en_core_web_sm model is missing, install with: `uv run python -m spacy download en_core_web_sm`

### Disable Integrations

Comment out credentials in `.env`:
```bash
# AZURE_DEVOPS_TOKEN=...
# GITHUB_TOKEN=...
```

Task updates won't be sent to those systems.

---

## What's Running in the Background

After `devtrack start`, here's what's happening:

```
├─ Go daemon (PID: 12345)
│  ├─ Git file monitor (watches for commits)
│  ├─ Cron scheduler (periodic triggers)
│  ├─ IPC server (listens on 127.0.0.1:35893)
│  └─ SQLite database (stores history)
│
└─ Python bridge (subprocess)
   ├─ IPC client (connects to daemon)
   ├─ NLP processor (spaCy)
   ├─ LLM client (Ollama)
   ├─ API integrations (Azure, GitHub, Teams)
   └─ Report generator
```

All communication is local on your machine. No data leaves unless you explicitly configure external integrations.

---

## Stopping DevTrack

```bash
# Stop the daemon gracefully
devtrack stop

# Verify it stopped
devtrack status  # Should show: "DevTrack daemon is not running"

# Force kill (if normal stop doesn't work)
pkill -f "devtrack start" || true
```

---

## Next: Use Cases

Now that DevTrack is running, try these:

### 1. See Commit Enhancement

```bash
cd /path/to/repo
git add .
devtrack git commit -m "Added authentication module"
# Shows AI-enhanced message with options to Accept/Enhance/Regenerate
```

### 2. See Work Update Prompt

```bash
devtrack force-trigger
# Interactive prompt: "What are you working on?"
# Type: "Working on PR #42 - fixing auth bug (2 hours)"
# Watch it extract task, time, and status automatically
```

### 3. See Report Generation

```bash
devtrack stats --today
# Shows summary of today's work
devtrack stats --week
# Shows summary of this week's work
```

---

**All set!** DevTrack is now monitoring your work. Check logs occasionally with `devtrack logs` to see it in action.

For detailed documentation, see the [Complete Documentation Index](INDEX.md).
