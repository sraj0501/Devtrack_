# devtrack CLI Quickstart

`devtrack` is the unified CLI and daemon binary. It monitors your Git repos, fires AI pipelines, and provides all developer-facing commands. There is a single binary — no separate thin-client package.

## Prerequisites

- Go 1.21+ (to build from source) or download a pre-built binary
- A configured `.env` file (see [Installation Guide](INSTALLATION.md))
- Ollama running locally (optional, for AI features)
- `devtrack_server` running (optional, for boardroom / admin GUI / PM integrations)

## 1. Install the Binary

### Pre-built release (recommended)

Pre-built binaries are published on every version tag:

```
https://github.com/sraj0501/Devtrack_/releases/latest
```

| Platform | Asset |
|---|---|
| Linux x64 | `devtrack_linux_amd64.tar.gz` |
| Linux ARM64 | `devtrack_linux_arm64.tar.gz` |
| macOS Apple Silicon | `devtrack_darwin_arm64.tar.gz` |
| macOS Intel | `devtrack_darwin_amd64.tar.gz` |
| Windows x64 | `devtrack_windows_amd64.exe` |

```bash
# macOS / Linux — detect OS and architecture, then download the right binary
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
[ "$ARCH" = "x86_64" ]  && ARCH="amd64"
[ "$ARCH" = "aarch64" ] && ARCH="arm64"
curl -fsSL "https://github.com/sraj0501/Devtrack_/releases/latest/download/devtrack_${OS}_${ARCH}.tar.gz" | tar xz
sudo mv devtrack /usr/local/bin/
```

### Build from source

```bash
git clone https://github.com/sraj0501/Devtrack_.git
cd Devtrack_/devtrack_client
go build -o devtrack .

# Install globally
mv devtrack ~/.local/bin/
```

## 2. Configure

```bash
# In the devtrack_client directory
cp .env_sample .env
nano .env
```

Minimum required variables:

```bash
PROJECT_ROOT=/path/to/devtrack_client
DEVTRACK_WORKSPACE=/path/to/repo/to/monitor
DATA_DIR=${PROJECT_ROOT}/Data
IPC_HOST=127.0.0.1
IPC_PORT=35893
IPC_CONNECT_TIMEOUT_SECS=5
IPC_RETRY_DELAY_MS=2000
HTTP_TIMEOUT_SHORT=10
HTTP_TIMEOUT=30
HTTP_TIMEOUT_LONG=60
LLM_PROVIDER=ollama
OLLAMA_HOST=http://localhost:11434
GIT_SAGE_DEFAULT_MODEL=llama3
PROMPT_TIMEOUT_SIMPLE_SECS=30
PROMPT_TIMEOUT_WORK_SECS=120
PROMPT_TIMEOUT_TASK_SECS=60
LLM_REQUEST_TIMEOUT_SECS=60
SENTIMENT_ANALYSIS_WINDOW_MINUTES=60

# Optional — only if devtrack_server is running
# DEVTRACK_SERVER_URL=https://localhost:8089
# DEVTRACK_API_KEY=your-api-key
```

## 3. First Commands

```bash
# Source env before starting
source .env

# Start the daemon
devtrack start

# Verify running
devtrack status

# Health check
devtrack health

# View logs
devtrack logs
```

## Full Command Reference

| Command | Description |
|---|---|
| `devtrack start` | Start the daemon (git monitor + scheduler) |
| `devtrack stop` | Stop the daemon gracefully |
| `devtrack status` | Daemon running state, PID, uptime, paused flag |
| `devtrack logs [N]` | Last N log lines (default 50) |
| `devtrack health` | Ping the server health endpoint |
| `devtrack version` | Show version |
| `devtrack pause` | Pause the git monitoring scheduler |
| `devtrack resume` | Resume git monitoring after a pause |
| `devtrack force-trigger` | Fire an immediate timer trigger |
| `devtrack boardroom "<problem>"` | Multi-persona AI review (requires server) |
| `devtrack plan "<problem>"` | Decompose into Epic/Story/Task (requires server) |
| `devtrack enable-learning` | Consent + initial AI learning data collection |
| `devtrack learning-status` | Show learning consent and sample count |
| `devtrack show-profile` | Display learned communication profile |
| `devtrack upgrade` | Download and install latest release |
| `devtrack uninstall` | Remove daemon files and config |
| `devtrack help` | Show all commands |

## Run Tests

```bash
cd devtrack_client
go test ./...
```

## Next Steps

- [Installation Guide](INSTALLATION.md) — full setup including server
- [Quick Start Guide](QUICK_START.md) — first workflow walkthrough
- [Configuration Reference](CONFIGURATION.md) — all env variables
- [Architecture Overview](ARCHITECTURE.md) — how the binary and server interact
