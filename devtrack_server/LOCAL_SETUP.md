# DevTrack Local Setup Guide

Complete step-by-step instructions for running DevTrack locally from scratch (without Docker).

## Prerequisites

Before you begin, ensure you have these installed:

### 1. **Go 1.20+**

```bash
# Check if Go is installed
go version

# If not installed, download from: https://go.dev/dl/
# Or on Linux:
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### 2. **Python 3.12 or 3.13** (NOT 3.14+)

```bash
# Check Python version
python3 --version

# Should be 3.12.x or 3.13.x (3.14+ has spaCy compatibility issues)
```

### 3. **uv Package Manager**

```bash
# Install uv (https://github.com/astral-sh/uv)
curl -LsSf https://astral.sh/uv/install.sh | sh

# Verify installation
uv --version
```

### 4. **Git**

```bash
git --version
```

### 5. **Ollama** (Optional - for AI features)

```bash
# Download from: https://ollama.com/download
# Or on Linux:
curl -fsSL https://ollama.com/install.sh | sh

# Start Ollama server
ollama serve
```

---

## Setup Steps

### Step 1: Clone Repository

```bash
cd ~/Documents/GitHub  # or your preferred location
git clone https://github.com/yourusername/automation_tools.git
cd automation_tools
```

### Step 2: Configure Environment

```bash
# Copy example configuration
cp .env_sample .env

# Edit with your paths
nano .env  # or use your preferred editor
```

**IMPORTANT: All configuration is required (NO defaults)**

Copy `.env_sample` to `.env` and set all variables. Missing any variable causes daemon to fail at startup with clear error message.

**System paths** (required):

- `PROJECT_ROOT` - Absolute path to automation_tools
- `DEVTRACK_WORKSPACE` - Git repo to monitor
- `DATA_DIR` - Runtime data directory

**IPC configuration** (required):

- `IPC_HOST` - IPC server host (usually `127.0.0.1`)
- `IPC_PORT` - IPC server port (usually `35893`)
- `IPC_CONNECT_TIMEOUT_SECS` - IPC timeout in seconds (e.g., `5`)
- `IPC_RETRY_DELAY_MS` - IPC retry delay in milliseconds (e.g., `2000`)

**Timeout configuration** (required - all must be set):

- `HTTP_TIMEOUT_SHORT` - Fast operations (e.g., `10` seconds)
- `HTTP_TIMEOUT` - Standard operations (e.g., `30` seconds)
- `HTTP_TIMEOUT_LONG` - Long operations (e.g., `60` seconds)
- `LLM_REQUEST_TIMEOUT_SECS` - LLM timeout (e.g., `120` seconds)
- `PROMPT_TIMEOUT_SIMPLE_SECS` - Simple prompts (e.g., `30` seconds)
- `PROMPT_TIMEOUT_WORK_SECS` - Work updates (e.g., `60` seconds)
- `PROMPT_TIMEOUT_TASK_SECS` - Task descriptions (e.g., `120` seconds)
- `SENTIMENT_ANALYSIS_WINDOW_MINUTES` - Analysis window (e.g., `120` minutes)

**Host configuration** (required):

- `OLLAMA_HOST` - Ollama server URL (e.g., `http://localhost:11434`)
- `LMSTUDIO_HOST` - LM Studio URL (e.g., `http://localhost:1234/v1`)
- `GIT_SAGE_DEFAULT_MODEL` - Default model (e.g., `llama3`)

**See [CONFIGURATION.md](docs/CONFIGURATION.md)** for complete reference with all variables and descriptions.

### Step 3: Install Python Dependencies

```bash
# From project root
cd automation_tools

# uv creates virtual environment and installs dependencies
uv sync

# Verify spaCy
uv run python -c "import spacy; print(spacy.__version__)"
```

### Step 4: Install spaCy Model

```bash
uv run python -m spacy download en_core_web_sm

# Verify
uv run python -c "import spacy; spacy.load('en_core_web_sm')"
```

### Step 5: Build Go Binary

```bash
cd devtrack-bin
go build -o devtrack .

# Verify
./devtrack version
```

Use devtrack from `devtrack-bin/` directly (no install to ~/.local/bin).

### Step 6: Verify Setup

```bash
# From project root - runs all checks
./scripts/verify_setup.sh
```

---

## Running DevTrack

### Method 1: Using run_devtrack_local.sh

```bash
# From project root
cd automation_tools
chmod +x run_devtrack_local.sh
./run_devtrack_local.sh
```

This script will:

- Load configuration from `.env`
- Validate required variables
- Stop any existing instances
- Start the daemon in background
- Show status and logs

### Method 2: Manual Start

```bash
# Load .env and start from project root
cd automation_tools
set -a && source .env && set +a
export PROJECT_ROOT="$PWD"
export DEVTRACK_WORKSPACE="$PWD"

# Start daemon (runs in background)
./devtrack-bin/devtrack start

# Or if installed globally:
devtrack start
```

### Check Status

```bash
devtrack status
# or
./devtrack-bin/devtrack status
```

---

## Verification & Testing

### Run Test Scripts

```bash
# Full commit flow (daemon + IPC + Python)
./scripts/test_commit_flow.sh

# Force trigger (timer via IPC)
./scripts/test_force_trigger.sh

# IPC connectivity only (daemon must be running first)
devtrack start &
sleep 5
uv run python scripts/test_ipc_manual.py
devtrack stop
```

### Manual Verification

```bash
# 1. Check daemon status
devtrack status

# 2. Check processes
ps aux | grep devtrack
ps aux | grep python_bridge.py

# 3. Check logs (path from .env: DATA_DIR/logs/daemon.log)
tail -50 Data/logs/daemon.log | grep -E "NLP|spaCy|IPC|Git monitor"

# Expected:
# - ✓ IPC server started
# - ✓ Git monitor started
# - ✓ Python bridge started
# - ✅ Connected to IPC server
```

### Test Commit Detection

```bash
# In your monitored repository
cd $DEVTRACK_WORKSPACE

# Make a test commit
git commit --allow-empty -m "Working on #PROJ-123 - Fixed auth bug (2h)"

# Check logs
tail -30 Data/logs/daemon.log
# Should see: COMMIT TRIGGER, Parsing commit message with NLP...
```

---

## Troubleshooting

### "spaCy model not found"

```bash
uv run python -m spacy download en_core_web_sm
```

### "Binary not found"

```bash
# Build from devtrack-bin
cd devtrack-bin && go build -o devtrack .

# Or add to PATH
export PATH="$PWD/devtrack-bin:$PATH"
```

### ".env file not found" or "Configuration missing"

```bash
cp .env_sample .env
# Edit .env - ALL VARIABLES ARE REQUIRED
# Run verify to check all variables are set:
./scripts/verify_setup.sh

# Common missing variables:
# - IPC_CONNECT_TIMEOUT_SECS
# - HTTP_TIMEOUT_SHORT, HTTP_TIMEOUT, HTTP_TIMEOUT_LONG
# - OLLAMA_HOST, LMSTUDIO_HOST
# - All PROMPT_TIMEOUT_* variables
# See docs/CONFIGURATION.md for all 12+ required variables
```

### "Configuration missing [VARIABLE_NAME]"

This means a required environment variable is not set in `.env`. The error message tells you which one.

```bash
# 1. Check .env has the variable
grep VARIABLE_NAME .env

# 2. If missing, add it from .env_sample
cat .env_sample | grep VARIABLE_NAME >> .env

# 3. Edit to set the correct value
nano .env

# 4. Try again
devtrack start
```

### "IPC connection failed" / Port 35893 in use

```bash
# Check what's using the port
lsof -i :35893

# Kill stale processes
lsof -ti :35893 | xargs kill -9
sleep 2
devtrack start
```

### "Git commits not detected"

```bash
# 1. Verify daemon is running
devtrack status

# 2. Ensure you're in the monitored repo (DEVTRACK_WORKSPACE)
pwd

# 3. Check daemon log
tail Data/logs/daemon.log | grep "Git repository"
```

### "Daemon is not running" when stopping

This is normal if the daemon was already stopped (e.g. by the test script trap). The test scripts now handle this correctly.

---

## Directory Structure

```
automation_tools/
├── .env                    # Your configuration (DO NOT commit)
├── .env_sample             # Template
├── devtrack-bin/
│   └── devtrack            # Build with: go build -o devtrack .
├── python_bridge.py        # Python IPC bridge
├── backend/                # Python modules
├── scripts/                # Test and verification scripts
│   ├── verify_setup.sh
│   ├── test_commit_flow.sh
│   ├── test_force_trigger.sh
│   └── test_ipc_manual.py
├── run_devtrack_local.sh   # Quick start script
└── pyproject.toml

Data/                       # Runtime data (from .env DATA_DIR)
├── db/                     # SQLite database
├── logs/                   # Daemon logs
├── pids/                   # Process ID file
└── configs/                # User configuration
```

---

## Regular Usage

### Daily Workflow

```bash
# Morning: Start daemon
./run_devtrack_local.sh
# or: devtrack start

# During work: Make commits as normal
git commit -m "Working on #TICKET-123 - Feature (2h)"

# Check status
devtrack status

# Evening: Stop daemon (optional)
devtrack stop
```

### Useful Commands

```bash
./devtrack-bin/devtrack status    # Check if running
./devtrack-bin/devtrack stop      # Stop daemon
./devtrack-bin/devtrack git commit -m "msg"   # AI-enhanced commit
./devtrack-bin/devtrack force-trigger         # Manual timer trigger
tail -f Data/logs/daemon.log      # Watch logs
```

**Note:** Use devtrack from `devtrack-bin/` directly (no install to ~/.local/bin). Run `./devtrack-bin/devtrack start`, `./devtrack-bin/devtrack git commit -m 'msg'`, etc.

---

## Updating DevTrack

```bash
cd automation_tools
git pull origin dev

# Update Python dependencies
uv sync

# Rebuild Go binary
cd devtrack-bin
go build -o devtrack .

# Restart daemon
devtrack stop
devtrack start
```

---

## Getting Help

- **Configuration**: [docs/CONFIGURATION.md](docs/CONFIGURATION.md) - All 12+ required variables
- **Usage Guide**: [USAGE_GUIDE.md](USAGE_GUIDE.md) - Feature usage documentation
- **Troubleshooting**: [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) - Common issues and solutions
- **Wiki/Documentation**: [docs/INDEX.md](docs/INDEX.md) - Complete documentation index
- **Architecture**: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - System design overview
- **Logs**: `Data/logs/daemon.log` (or path from `LOG_DIR` in .env)
