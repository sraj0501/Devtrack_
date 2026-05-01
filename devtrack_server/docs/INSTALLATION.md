# DevTrack Installation Guide

Complete step-by-step installation instructions for all platforms.

---

## Which Installation Path?

Choose one:

1. **[Local Installation](#local-installation-recommended)** (Recommended for development and Linux/macOS)
   - Direct installation on your system
   - Faster development iteration
   - Native performance
   - ~15 minutes setup

2. **[Docker Installation](#docker-installation-cross-platform)** (Recommended for Windows)
   - Consistent across platforms
   - No system-wide dependencies
   - Easier cleanup
   - ~10 minutes setup

3. **[macOS Only: Homebrew](#macos-homebrew-easiest)** (Easiest for macOS users)
   - One command setup
   - Automatic updates
   - System integration
   - ~5 minutes setup

---

## Prerequisites Overview

All paths require:
- [ ] Git
- [ ] 2GB free disk space
- [ ] Internet connection (first time only; can work offline after)

**Local/Homebrew additionally require**:
- [ ] Go 1.20+
- [ ] Python 3.12+ (NOT 3.14+)
- [ ] uv package manager

**Docker additionally requires**:
- [ ] Docker
- [ ] Docker Compose

---

## Local Installation (Recommended)

### Step 1: Install Prerequisites

#### macOS

```bash
# Install Homebrew (if not already installed)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install Go
brew install go

# Install Python (3.13 recommended for compatibility)
brew install python@3.13
python3.13 --version  # Should be 3.13.x

# Install uv
curl -LsSf https://astral.sh/uv/install.sh | sh

# Verify all installed
go version      # 1.20 or higher
python3.13 --version  # 3.13.x (not 3.14+)
uv --version    # Latest version
git --version   # 2.0 or higher
```

#### Linux (Ubuntu/Debian)

```bash
# Update package manager
sudo apt update

# Install Go
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Install Python 3.12 or 3.13
sudo apt install python3.13 python3.13-venv
python3.13 --version

# Install uv
curl -LsSf https://astral.sh/uv/install.sh | sh

# Verify all installed
go version
python3.13 --version
uv --version
```

#### Linux (Fedora/RHEL)

```bash
# Install Go
sudo dnf install golang

# Install Python
sudo dnf install python3.13

# Install uv
curl -LsSf https://astral.sh/uv/install.sh | sh

# Verify
go version
python3.13 --version
uv --version
```

#### Windows (PowerShell)

```powershell
# Install Scoop (Windows package manager)
iex (New-Object System.Net.WebClient).DownloadString('https://get.scoop.sh')

# Install Go
scoop install go

# Install Python 3.13
scoop install python@3.13
python --version  # Should be 3.13.x

# Install uv
scoop install uv

# Verify
go version
python --version
uv --version
```

### Step 2: Clone Repository

```bash
# Choose a location
cd ~/Documents  # or your preferred location

# Clone DevTrack
git clone https://github.com/yourusername/automation_tools.git
cd automation_tools
```

### Step 3: Configure Environment

```bash
# Copy example configuration
cp .env_sample .env

# Edit configuration
nano .env  # or use vim, code, VS Code, etc.
```

**Must set these variables**:

| Variable | Value |
|----------|-------|
| `PROJECT_ROOT` | Absolute path to automation_tools directory |
| `DEVTRACK_WORKSPACE` | Path to Git repo to monitor (can be same as PROJECT_ROOT) |
| `DATA_DIR` | Where to store logs/db (e.g., `${PROJECT_ROOT}/Data`) |

**Example .env** (adjust paths for your system):

```bash
# Paths (REQUIRED - use absolute paths)
PROJECT_ROOT=/home/user/Documents/automation_tools
DEVTRACK_WORKSPACE=${PROJECT_ROOT}
DEVTRACK_HOME=${PROJECT_ROOT}/devtrack-bin
DATA_DIR=${PROJECT_ROOT}/Data
DATABASE_DIR=${DATA_DIR}/db
LOG_DIR=${DATA_DIR}/logs

# IPC Configuration (defaults are fine)
IPC_HOST=127.0.0.1
IPC_PORT=35893

# AI/LLM Configuration
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral

# Optional: Add credentials later for integrations
# OPENAI_API_KEY=
# ANTHROPIC_API_KEY=
# AZURE_DEVOPS_TOKEN=
# GITHUB_TOKEN=
# TEAMS_BOT_ID=
```

### Step 4: Install Dependencies

```bash
# From automation_tools directory
cd /path/to/automation_tools

# Install Python dependencies via uv
uv sync

# Install spaCy NLP model
uv run python -m spacy download en_core_web_sm

# Verify spaCy installation
uv run python -c "import spacy; nlp = spacy.load('en_core_web_sm'); print('NLP OK')"
```

### Step 5: Build Go Daemon

```bash
# Navigate to Go source directory
cd devtrack-bin

# Build the binary
go build -o devtrack .

# Verify build succeeded
ls -la devtrack  # Should see the binary file

# Optional: Install globally
mkdir -p ~/.local/bin
mv devtrack ~/.local/bin/

# Add to PATH (if not already)
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Verify it's in PATH
which devtrack  # Should show ~/.local/bin/devtrack
```

### Step 6: Verify Installation

```bash
# Navigate back to project root
cd /path/to/automation_tools

# Check Go daemon
devtrack --version

# Check Python dependencies
uv run python -c "import spacy; print('NLP: OK')"

# Check environment
echo $PROJECT_ROOT
cat .env | grep "PROJECT_ROOT\|DEVTRACK_WORKSPACE"

# Optional: Check Ollama
curl http://localhost:11434/api/tags
```

### Step 7: Start Ollama (Optional but Recommended)

```bash
# In a separate terminal
ollama serve

# Or as background service
brew services start ollama  # macOS
sudo systemctl start ollama  # Linux
```

### Step 8: Start DevTrack

```bash
# From automation_tools directory
cd /path/to/automation_tools

# Start the daemon
devtrack start

# Verify it's running
devtrack status

# View logs
devtrack logs
```

Expected output:
```
DevTrack daemon is running (PID: 12345)
Monitoring: /path/to/automation_tools
IPC Server: 127.0.0.1:35893
```

---

## macOS Homebrew (Easiest)

Coming soon! Homebrew formula is in development.

```bash
# When available:
brew tap sraj0501/automation_tools
brew install devtrack
devtrack setup  # Interactive setup wizard
```

---

## Docker Installation (Cross-Platform)

### Prerequisites

- Docker Desktop (macOS, Windows)
- Docker Engine + Docker Compose (Linux)
- [Download Docker](https://www.docker.com/products/docker-desktop)

### Step 1: Verify Docker Installation

```bash
docker --version      # Should be 20.10+
docker compose --version  # Should be 2.0+
```

### Step 2: Clone Repository

```bash
cd ~/Documents
git clone https://github.com/yourusername/automation_tools.git
cd automation_tools
```

### Step 3: Configure Environment

```bash
cp .env_sample .env
nano .env
```

**Key variables for Docker**:

```bash
# Paths - use relative paths (defaults work for Docker)
PROJECT_ROOT=.
DEVTRACK_WORKSPACE=${PROJECT_ROOT}
DATA_DIR=${PROJECT_ROOT}/Data
DEVTRACK_DOCKER=true    # Enable Docker mode

# Rest same as local installation
```

### Step 4: Configure Ollama

DevTrack container talks to Ollama running on your host:

```bash
# macOS/Linux - Ollama runs on host
# Windows - Can use WSL2 or dedicated Ollama installation

# Terminal 1: Start Ollama
ollama serve

# Terminal 2: Pull a model (if not already downloaded)
ollama pull mistral
```

### Step 5: Build and Start Container

```bash
# From automation_tools directory
cd automation_tools

# Build image (with BuildKit for speed)
DOCKER_BUILDKIT=1 docker compose build

# Start container
docker compose up devtrack -d

# Verify it's running
docker compose ps

# View logs
docker compose logs devtrack -f
```

### Step 6: Verify Installation

```bash
# Check daemon is running in container
docker compose exec devtrack devtrack status

# View logs
docker compose logs devtrack

# Check it can reach Ollama
docker compose exec devtrack curl http://host.docker.internal:11434/api/tags
```

### Managing Docker Container

```bash
# View logs
docker compose logs devtrack -f

# Stop container
docker compose down

# Restart
docker compose restart devtrack

# Shell into container (for debugging)
docker compose exec devtrack /bin/bash
```

---

## Verification Checklist

After installation, verify everything works:

- [ ] `devtrack --version` shows version
- [ ] `devtrack status` shows daemon running
- [ ] NLP works: `uv run python -c "import spacy; spacy.load('en_core_web_sm')"`
- [ ] Ollama running: `curl http://localhost:11434/api/tags`
- [ ] `.env` variables set correctly
- [ ] Data directory created: `ls Data/`
- [ ] Git can be monitored: `git status` in a repo

---

## Configuration After Installation

### Enable Project Integration (Optional)

Add credentials to `.env` for Azure DevOps, GitHub, or Teams:

#### Azure DevOps

```bash
AZURE_DEVOPS_ORG=your-organization
AZURE_DEVOPS_PROJECT=your-project
AZURE_DEVOPS_TOKEN=your-pat-token
```

[Create Azure DevOps PAT](https://dev.azure.com)

#### GitHub

```bash
GITHUB_TOKEN=your-github-token
GITHUB_REPO=username/repo
```

[Create GitHub PAT](https://github.com/settings/tokens)

#### Microsoft Teams

```bash
TEAMS_BOT_ID=your-bot-id
TEAMS_BOT_PASSWORD=your-bot-password
TEAMS_CHANNEL_ID=your-channel-id
```

### Configure AI Provider (Optional)

Change LLM provider in `.env`:

```bash
# Local Ollama (default, free)
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral

# Or use OpenAI
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-4

# Or Anthropic
LLM_PROVIDER=anthropic
ANTHROPIC_API_KEY=sk-ant-...
ANTHROPIC_MODEL=claude-3-opus
```

See [LLM Guide](LLM_GUIDE.md) for details.

---

## Troubleshooting Installation

### "devtrack: command not found"

```bash
# Check if binary exists
ls ~/.local/bin/devtrack

# Check if in PATH
echo $PATH | grep .local/bin

# Try full path
~/.local/bin/devtrack status

# Or rebuild and reinstall
cd devtrack-bin
go build -o devtrack .
cp devtrack ~/.local/bin/
```

### "Python 3.14 detected, but 3.12/3.13 required"

```bash
# uv should auto-downgrade based on pyproject.toml
# If not, explicitly use Python 3.13

uv --python python3.13 sync

# Or set PYTHON env var
export PYTHON=python3.13
uv sync
```

### "spaCy model not found"

```bash
uv run python -m spacy download en_core_web_sm

# Verify
uv run python -c "import spacy; spacy.load('en_core_web_sm'); print('OK')"
```

### "Ollama not running / not found"

```bash
# Check if running
curl http://localhost:11434/api/tags

# Start Ollama
ollama serve  # Run in background or separate terminal

# macOS: Use Homebrew service
brew services start ollama
brew services status ollama

# Linux: Use systemd
sudo systemctl start ollama
sudo systemctl status ollama
```

### ".env not found"

```bash
# Check current directory
ls -la .env

# Make sure you're in automation_tools root
pwd

# Copy from sample if missing
cp .env_sample .env
nano .env
```

### "IPC connection failed"

```bash
# Check port not in use
lsof -i :35893

# Or change port in .env
IPC_PORT=35894

# Restart daemon
devtrack restart

# Check firewall on some systems
# May need to allow 127.0.0.1:35893 through firewall
```

---

## Next Steps

1. **[Quick Start Guide](QUICK_START.md)** - Get started using DevTrack
2. **[Configuration Reference](CONFIGURATION.md)** - All .env variables
3. **[LLM Setup](LLM_GUIDE.md)** - Configure AI providers
4. **[Troubleshooting](TROUBLESHOOTING.md)** - Solve problems
5. **[Getting Started](GETTING_STARTED.md)** - Learn DevTrack concepts

---

## Getting Help

If installation fails:

1. Check [Troubleshooting Guide](TROUBLESHOOTING.md)
2. Review [Known Issues](../KNOWN_ISSUES.md)
3. Check [CLAUDE.md](../CLAUDE.md) for debugging patterns
4. Open an [Issue on GitHub](https://github.com/sraj0501/automation_tools/issues)

---

## Uninstallation

### Local Installation

```bash
# Remove binary
rm ~/.local/bin/devtrack

# Remove data directory (backup first!)
rm -rf /path/to/automation_tools/Data

# Remove git clone
rm -rf /path/to/automation_tools
```

### Docker Installation

```bash
# Stop and remove containers
docker compose down -v

# Remove image
docker compose rm
docker image rm automation_tools-devtrack

# Remove git clone
rm -rf /path/to/automation_tools
```

---

## Manual Installation Troubleshooting

If standard installation doesn't work, try manual steps:

```bash
# 1. Clone manually
git clone https://github.com/yourusername/automation_tools.git
cd automation_tools

# 2. Create directories
mkdir -p Data/db Data/logs

# 3. Create .env manually
cat > .env << 'EOF'
PROJECT_ROOT=/absolute/path/to/automation_tools
DEVTRACK_WORKSPACE=${PROJECT_ROOT}
DATA_DIR=${PROJECT_ROOT}/Data
DATABASE_DIR=${DATA_DIR}/db
LOG_DIR=${DATA_DIR}/logs
IPC_HOST=127.0.0.1
IPC_PORT=35893
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral
EOF

# 4. Install Python
python3.13 -m venv venv
source venv/bin/activate  # or venv\Scripts\activate on Windows
pip install -r requirements.txt

# 5. Build Go (if you have Go installed)
cd devtrack-bin
go mod download
go build -o devtrack .
cd ..

# 6. Test
devtrack --version
```

---

**Installation complete!** You're ready to use DevTrack. See [Quick Start Guide](QUICK_START.md) for next steps.
