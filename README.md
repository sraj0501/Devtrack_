# DevTrack - Developer Automation Tools

> An intelligent system that automates developer timesheet tracking, task management, and progress reporting through Git monitoring and AI-powered natural language processing.

## Monorepo Structure

The codebase is split into two independent services living in separate GitLab repositories. This monorepo exists as a combined working tree during the transition period.

| Folder | GitLab repo | What's inside |
|--------|------------|---------------|
| [`devtrack_server/`](devtrack_server/) | [`devtrack3_cloud/devtrack_server`](https://gitlab.com/devtrack3_cloud/devtrack_server) | Go daemon + Python backend |
| [`devtrack_client/`](devtrack_client/) | [`devtrack3_cloud/devtrack_cli`](https://gitlab.com/devtrack3_cloud/devtrack_cli) | Thin CLI client |
| [`contract/`](contract/) | [`devtrack3_cloud/devtrack_contract`](https://gitlab.com/devtrack3_cloud/devtrack_contract) | HTTP API types — read this first when debugging |

See [CLAUDE.md](CLAUDE.md) for the full architecture overview.

## Documentation

| Purpose | Documentation |
|---------|---|
| **Architecture** | [devtrack_wiki/docs/ARCHITECTURE.md](devtrack_wiki/docs/ARCHITECTURE.md) |
| **Getting Started** | [devtrack_wiki/docs/GETTING_STARTED.md](devtrack_wiki/docs/GETTING_STARTED.md) |
| **Installation** | [devtrack_wiki/docs/INSTALLATION.md](devtrack_wiki/docs/INSTALLATION.md) |
| **Configuration** | [devtrack_wiki/docs/CONFIGURATION.md](devtrack_wiki/docs/CONFIGURATION.md) |
| **LLM Guide** | [devtrack_wiki/docs/LLM_GUIDE.md](devtrack_wiki/docs/LLM_GUIDE.md) |
| **CLI/Server Split (ADR-001)** | [devtrack_wiki/docs/ADR-001-cli-server-split.md](devtrack_wiki/docs/ADR-001-cli-server-split.md) |
| **Git Features** | [devtrack_wiki/docs/GIT_FEATURES.md](devtrack_wiki/docs/GIT_FEATURES.md) |
| **Personalization** | [devtrack_wiki/docs/PERSONALIZATION.md](devtrack_wiki/docs/PERSONALIZATION.md) |
| **All docs** | [devtrack_wiki/docs/INDEX.md](devtrack_wiki/docs/INDEX.md) |

---

## Quick Overview

DevTrack combines background process automation with AI intelligence to:

- Monitor your Git activity and trigger smart prompts at key moments
- Parse natural language updates into structured task data
- Learn your communication style from Teams, Azure DevOps, and Outlook
- Generate responses in YOUR voice using privacy-first local AI
- Integrate with Azure DevOps, GitHub, Jira, and Microsoft Lists
- Update tasks automatically in project tracking systems
- Generate professional reports for managers and stakeholders
- Track time and productivity without manual timesheet entry

### CLI/Server Architecture

The system is split into two binaries that communicate over HTTP:

| Binary | Module | What it does |
|--------|--------|--------------|
| `devtrack-server` | `devtrack_server/` | Go daemon + Python backend; listens on HTTP port 8765 and IPC port 35893 |
| `devtrack-cli` | `devtrack_client/` | Thin HTTP client; proxies commands to the server over HTTP |

The shared API contract (`contract/api.go`) defines all 9 HTTP routes and request/response types. See [ADR-001](devtrack_wiki/docs/ADR-001-cli-server-split.md) for the full architectural rationale.

## Configuration Required

DevTrack requires **explicit configuration** with no hardcoded defaults.

### Server configuration (`devtrack_server/.env`)

Copy `devtrack_server/.env_sample` and set at minimum:

- Timeouts: `IPC_CONNECT_TIMEOUT_SECS`, `HTTP_TIMEOUT_SHORT`, `HTTP_TIMEOUT`, `HTTP_TIMEOUT_LONG`
- Hosts: `OLLAMA_HOST`, `LMSTUDIO_HOST`
- Model: `GIT_SAGE_DEFAULT_MODEL`
- Delays: `IPC_RETRY_DELAY_MS`
- Prompts: `PROMPT_TIMEOUT_SIMPLE_SECS`, `PROMPT_TIMEOUT_WORK_SECS`, `PROMPT_TIMEOUT_TASK_SECS`
- LLM: `LLM_REQUEST_TIMEOUT_SECS`
- Sentiment: `SENTIMENT_ANALYSIS_WINDOW_MINUTES`
- HTTP port: `DEVTRACK_SERVER_HTTP_PORT` (default 8765)
- Optional auth token: `DEVTRACK_API_TOKEN`

LLM provider is selected by `LLM_PROVIDER` (`ollama` | `openai` | `anthropic`). Per-feature temperature and token limits are configurable via `COMMIT_LLM_TEMPERATURE`, `REPORT_LLM_TEMPERATURE`, etc.

### CLI configuration (`devtrack_client/.env`)

```bash
DEVTRACK_SERVER_URL=http://localhost:8765
CLI_APP_NAME=devtrack-cli
DEVTRACK_VERSION=0.1.0
DEVTRACK_API_TOKEN=   # must match server value if set
```

See [Configuration Reference](devtrack_wiki/docs/CONFIGURATION.md) for the complete variable list with examples.

## 30-Second Start

```bash
# 1. Clone and configure the server
cd devtrack_server
cp .env_sample .env
nano .env  # IMPORTANT: Set all required variables!

# 2. Install Python dependencies
uv sync

# 3. Build the server binary
cd devtrack-bin && go build -o devtrack-server .
mv devtrack-server ~/.local/bin/

# 4. Start the server
devtrack-server start &
devtrack-server status

# 5. Build and configure the CLI (separate terminal)
cd ../../devtrack_client
cp .env_sample .env          # set DEVTRACK_SERVER_URL
go build -o devtrack-cli ./cmd/cli
./devtrack-cli status        # proxies to server over HTTP

# 6. Make a commit - see AI magic
git commit -m "Working on auth feature (2h)"
```

**Note**: Server will fail at startup if any required variables are missing (this is intentional for safety). On Windows, use `devtrack-cli.exe` and the `devtrack.bat` compatibility wrapper.

For detailed setup, see [Installation Guide](devtrack_wiki/docs/INSTALLATION.md) and [Configuration Reference](devtrack_wiki/docs/CONFIGURATION.md).

---

## Core Features

### Git Workflow Enhancement (Phases 1-3)

- **Enhanced Commit Messages**: AI-powered context-aware commit messages with branch/PR information
- **Conflict Resolution**: Automatic merge conflict detection and smart resolution
- **Work Update Parsing**: Natural language work updates with PR/issue auto-detection
- **Daily Reports**: AI-enhanced daily and weekly report generation

### AI-Powered Processing

- **Local-First**: 100% offline-capable with Ollama (no external AI required)
- **Hybrid LLM**: Optional integration with OpenAI, Anthropic, or custom LLMs; provider selected by `LLM_PROVIDER` with automatic fallback chain
- **NLP Parsing**: spaCy-based natural language processing for task extraction
- **Personalization**: Learns your communication style from Microsoft Teams chat history and generates responses in your voice (requires MongoDB)

### HTTP API (server → CLI bridge)

The server exposes 9 routes consumed by `devtrack-cli`:

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/health` | Liveness check + version |
| GET | `/status` | Daemon running state, PID, uptime |
| GET | `/logs` | Recent log lines (optional `?tail=N`) |
| GET | `/version` | Binary version and Go version |
| POST | `/start` | Start the daemon |
| POST | `/stop` | Stop the daemon |
| POST | `/pause` | Pause scheduled triggers |
| POST | `/resume` | Resume scheduled triggers |
| POST | `/trigger/force` | Fire an immediate trigger |

All routes accept an optional `X-DevTrack-Token` header when `DEVTRACK_API_TOKEN` is set. See `contract/api.go` for request/response types.

### Integrations

- **Project Management**: Azure DevOps, GitHub, Jira
- **Communication**: Microsoft Teams, Outlook
- **Task Tracking**: Automatic updates to linked tasks and stories

---

## Technology Stack

### Backend (Go)

- Go 1.20+ daemon for monitoring and triggers
- fsnotify for real-time Git repository monitoring
- Cron-based scheduling with configurable intervals
- SQLite for local caching and trigger history
- TCP-based IPC for Go ↔ Python communication

### Intelligence (Python 3.12+)

- **uv** for fast dependency management
- **spaCy** (en_core_web_sm) for NLP and entity recognition
- **OLLAMA** for local LLM processing (privacy-first)
- **sentence-transformers** for semantic task matching
- **python-dotenv** for environment configuration
- Microsoft Graph SDK for Teams/Outlook integrations

### Integrations

- Azure DevOps REST API
- Microsoft Graph API (Teams, Email, Lists)
- GitHub API
- Jira API

### CI/CD

Both repos ship GitLab CI/CD pipelines:

| Repo | Pipeline highlights |
|------|---------------------|
| `devtrack_server` | `go vet`, `go build` (linux/amd64, CGO_ENABLED=0); `pytest` (allow_failure); tag-triggered release |
| `devtrack_client` | `go vet`, `go test`; cross-compile to linux/amd64, darwin/arm64, darwin/amd64, windows/amd64; tag-triggered release with 4 asset links |

The client pipeline clones `devtrack_contract` via `CI_JOB_TOKEN` so the `replace` directive resolves in the split-repo CI environment.

---

## Project Status

**Current Phase**: GitLab migration complete; active feature development
**Overall Progress**: ~95% Complete

### Completed Phases

- **Phase 1**: Enhanced Commit Messages ✅
- **Phase 2**: Conflict Resolution & PR-Aware Parsing ✅
- **Phase 3**: Event-Driven Integration ✅
- **Phase 4**: Personalization / "Talk Like You" ✅
- **Migration**: CLI/server split, HTTP API, GitLab CI/CD, doc URLs ✅
- **Housekeeping**: Docs/wiki moved to `devtrack_wiki` repo, monorepo cleaned up ✅

### Planned

- launchd plist for macOS auto-start
- Ticket Alerter (background polling for Jira/Azure/GitHub events)
- Dashboard and analytics

For detailed phase information, see [Roadmap & Phases](devtrack_wiki/docs/PHASES.md).

---

## Installation Options

### Option 1: Local Installation (Recommended for Development)

Run DevTrack natively on your system without Docker for faster iteration and easier debugging.

**Prerequisites**:

- Go 1.21+ ([Download](https://go.dev/dl/))
- Python 3.12+ with uv package manager ([Install uv](https://github.com/astral-sh/uv))
- Git (for repository monitoring)
- Ollama (optional, for AI features) ([Download](https://ollama.com/download))

**Setup**:

```bash
# Server
cd devtrack_server
cp .env_sample .env   # set PROJECT_ROOT and required vars
uv sync
cd devtrack-bin && go build -o devtrack-server .
mv devtrack-server ~/.local/bin/
devtrack-server start &

# CLI (separate clone or same monorepo)
cd ../../devtrack_client
cp .env_sample .env   # set DEVTRACK_SERVER_URL=http://localhost:8765
go build -o devtrack-cli ./cmd/cli
```

Unix/macOS compatibility wrapper: `devtrack_client/devtrack` (shell script that execs `devtrack-cli`).
Windows compatibility wrapper: `devtrack_client/devtrack.bat`.

See [Installation Guide](devtrack_wiki/docs/INSTALLATION.md) for complete step-by-step instructions.

### Option 2: Containerized Setup (Cross-Platform)

This workflow runs the full stack on macOS, Windows, and Linux with the same commands.

```bash
# Copy .env_sample to .env and configure
cd devtrack_server
cp .env_sample .env

# Start Ollama on host machine
ollama serve

# Launch DevTrack container (includes MongoDB, Redis, PostgreSQL)
DOCKER_BUILDKIT=1 docker compose up devtrack
```

---

## Privacy & Security

DevTrack is built with privacy as a core principle:

- All data stored locally on your machine
- No cloud AI services by default (uses local Ollama)
- Explicit consent required for AI learning features
- Complete transparency about data collection
- Full data deletion option available anytime

---

## Common Commands

Commands are sent via `devtrack-cli` (or the `devtrack` / `devtrack.bat` wrapper) which proxies them to the running server over HTTP.

```bash
# Daemon Control
devtrack-cli start              # Start monitoring
devtrack-cli stop               # Stop daemon
devtrack-cli status             # Show running status + uptime

# Scheduler Control
devtrack-cli pause              # Pause scheduled triggers
devtrack-cli resume             # Resume scheduler
devtrack-cli trigger/force      # Trigger immediately

# Information
devtrack-cli logs               # View recent log lines
devtrack-cli version            # Version information
devtrack-cli health             # Liveness check
```

### Personalization Commands (run on server directly)

```bash
devtrack-server enable-learning       # Consent + initial Teams data collection
devtrack-server learning-sync         # Delta sync (only new messages since last run)
devtrack-server learning-sync --full  # Force full 30-day re-collection
devtrack-server show-profile          # Display learned communication profile
devtrack-server test-response <text>  # Generate a personalized response
devtrack-server learning-status       # Show consent/sample count status
devtrack-server learning-reset        # Wipe all learning data and start fresh
devtrack-server learning-setup-cron   # Install daily cron job
devtrack-server learning-remove-cron  # Remove cron entry
devtrack-server learning-cron-status  # Show cron status
```

---

## Troubleshooting

### Python Version Issues

```bash
# DevTrack requires Python 3.12 or 3.13 (not 3.14+)
python3 --version

# If using Python 3.14, uv will automatically downgrade based on pyproject.toml
```

### spaCy NLP Model Not Found

```bash
uv run python -m spacy download en_core_web_sm
```

### Daemon Won't Start

```bash
# Check logs for errors
tail -50 ~/.devtrack/daemon.log

# Verify .env file exists
cat .env | grep PROJECT_ROOT

# Check if port is already in use
lsof -i :35893
```

See [Troubleshooting Guide](devtrack_wiki/docs/TROUBLESHOOTING.md) for more solutions.

---

## Contributing

1. Fork the relevant repo on GitLab ([server](https://gitlab.com/devtrack3_cloud/devtrack_server) | [CLI](https://gitlab.com/devtrack3_cloud/devtrack_cli))
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Merge Request

For development setup and architecture details, see [CLAUDE.md](CLAUDE.md).

---

## Support

- **Documentation**: [Complete Documentation Index](devtrack_wiki/docs/INDEX.md)
- **Architecture**: [System Architecture](devtrack_wiki/docs/ARCHITECTURE.md)
- **ADR-001**: [CLI/Server Split](devtrack_wiki/docs/ADR-001-cli-server-split.md)
- **Issues**: [GitLab Issues — Server](https://gitlab.com/devtrack3_cloud/devtrack_server/-/issues)
- **Issues**: [GitLab Issues — CLI](https://gitlab.com/devtrack3_cloud/devtrack_cli/-/issues)

---

## License

This project is licensed under the MIT License - see the LICENSE file for details.

---

**Note**: This tool is designed for individual and team productivity enhancement. Ensure you have appropriate licenses and permissions for all integrated services.
