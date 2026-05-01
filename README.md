# DevTrack - Developer Automation Tools

> An intelligent system that automates developer timesheet tracking, task management, and progress reporting through Git monitoring and AI-powered natural language processing.

## Monorepo Structure (Migration Phase)

This repo is being split into two independent services. Code is physically separated into subfolders:

| Folder | Destination | What's inside |
|--------|------------|---------------|
| [`devtrack_server/`](devtrack_server/) | `gitlab.com/devtrack3_cloud/devtrack_server` | Go daemon + Python backend |
| [`devtrack_client/`](devtrack_client/) | `gitlab.com/devtrack3_cloud/devtrack_cli` | Thin CLI client (in progress) |
| [`contract/`](contract/) | Shared by both | HTTP API types — read this first when debugging |

See [CLAUDE.md](CLAUDE.md) for migration task status and the full architecture overview.

## Documentation

| Purpose | Documentation |
|---------|---|
| **Architecture** | [devtrack_server/docs/ARCHITECTURE.md](devtrack_server/docs/ARCHITECTURE.md) |
| **Getting Started** | [devtrack_server/docs/GETTING_STARTED.md](devtrack_server/docs/GETTING_STARTED.md) |
| **Installation** | [devtrack_server/docs/INSTALLATION.md](devtrack_server/docs/INSTALLATION.md) |
| **Configuration** | [devtrack_server/docs/CONFIGURATION.md](devtrack_server/docs/CONFIGURATION.md) |
| **LLM Guide** | [devtrack_server/docs/LLM_GUIDE.md](devtrack_server/docs/LLM_GUIDE.md) |
| **All docs** | [devtrack_server/docs/INDEX.md](devtrack_server/docs/INDEX.md) |

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

## Configuration Required

DevTrack requires **explicit configuration** with no hardcoded defaults. All configuration comes from a single `.env` file with **12 required variables**:

```bash
# Copy and configure (MUST set all variables)
cp .env_sample .env
nano .env  # Edit with YOUR paths and settings
```

**Required variables** (no defaults - must be set):

- Timeouts: `IPC_CONNECT_TIMEOUT_SECS`, `HTTP_TIMEOUT_SHORT`, `HTTP_TIMEOUT`, `HTTP_TIMEOUT_LONG`
- Hosts: `OLLAMA_HOST`, `LMSTUDIO_HOST`
- Model: `GIT_SAGE_DEFAULT_MODEL`
- Delays: `IPC_RETRY_DELAY_MS`
- Prompts: `PROMPT_TIMEOUT_SIMPLE_SECS`, `PROMPT_TIMEOUT_WORK_SECS`, `PROMPT_TIMEOUT_TASK_SECS`
- LLM: `LLM_REQUEST_TIMEOUT_SECS`
- Sentiment: `SENTIMENT_ANALYSIS_WINDOW_MINUTES`

See [Configuration Reference](docs/CONFIGURATION.md) for complete list with examples.

## 30-Second Start

```bash
# 1. Clone and configure (MUST do this)
git clone https://github.com/yourusername/automation_tools.git
cd automation_tools
cp .env_sample .env
nano .env  # IMPORTANT: Set all required variables!

# 2. Install dependencies
uv sync

# 3. Build the Go daemon
cd devtrack-bin && go build -o devtrack .
mv devtrack ~/.local/bin/

# 4. Start monitoring
devtrack start &
devtrack status

# 5. Make a commit - see AI magic
git commit -m "Working on auth feature (2h)"
```

**Note**: Daemon will fail at startup if any required variables are missing (this is intentional for safety).

For detailed setup, see [Installation Guide](docs/INSTALLATION.md) and [Configuration Reference](docs/CONFIGURATION.md).

---

## Core Features

### Git Workflow Enhancement (Phases 1-3)

- **Enhanced Commit Messages**: AI-powered context-aware commit messages with branch/PR information
- **Conflict Resolution**: Automatic merge conflict detection and smart resolution
- **Work Update Parsing**: Natural language work updates with PR/issue auto-detection
- **Daily Reports**: AI-enhanced daily and weekly report generation

### AI-Powered Processing

- **Local-First**: 100% offline-capable with Ollama (no external AI required)
- **Hybrid LLM**: Optional integration with OpenAI, Anthropic, or custom LLMs
- **NLP Parsing**: spaCy-based natural language processing for task extraction
- **Learning**: Personalized AI that learns from your communication style

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

---

## Project Status

**Current Phase**: Phase 3 Complete (Advanced Features - In Progress)
**Overall Progress**: ~85% Complete

### Completed Phases

- **Phase 1**: Enhanced Commit Messages ✅
- **Phase 2**: Conflict Resolution & PR-Aware Parsing ✅
- **Phase 3**: Event-Driven Integration ✅

### Current Phase (Phase 4+)

- Enhanced Integrations and advanced features
- Dashboard and analytics
- Mobile notifications
- Plugin system

For detailed phase information, see [Roadmap & Phases](docs/PHASES.md).

---

## Installation Options

### Option 1: Local Installation (Recommended for Development)

Run DevTrack natively on your system without Docker for faster iteration and easier debugging.

**Prerequisites**:

- Go 1.20+ ([Download](https://go.dev/dl/))
- Python 3.12+ with uv package manager ([Install uv](https://github.com/astral-sh/uv))
- Git (for repository monitoring)
- Ollama (optional, for AI features) ([Download](https://ollama.com/download))

**Setup**:

```bash
git clone https://github.com/yourusername/automation_tools.git
cd automation_tools
cp .env_sample .env
# Edit .env - set PROJECT_ROOT to your repo path
uv sync
cd devtrack-bin && go build -o devtrack .
mv devtrack ~/.local/bin/
devtrack start &
```

See [Installation Guide](docs/INSTALLATION.md) for complete step-by-step instructions.

### Option 2: Containerized Setup (Cross-Platform)

This workflow runs the full stack on macOS, Windows, and Linux with the same commands.

```bash
# Copy .env_sample to .env and configure
cp .env_sample .env

# Start Ollama on host machine
ollama serve

# Launch DevTrack container
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

```bash
# Daemon Control
devtrack start              # Start monitoring
devtrack stop               # Stop daemon
devtrack restart            # Restart with new config
devtrack status             # Show running status

# Scheduler Control
devtrack pause              # Pause scheduled triggers
devtrack resume             # Resume scheduler
devtrack force-trigger      # Trigger immediately
devtrack skip-next          # Skip next scheduled trigger

# Information
devtrack logs               # View recent logs
devtrack db-stats           # Database statistics
devtrack version            # Version information
devtrack help               # Full command list
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

See [Troubleshooting Guide](docs/TROUBLESHOOTING.md) for more solutions.

---

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

For development setup and architecture details, see [CLAUDE.md](CLAUDE.md).

---

## Support

- **Documentation**: [Complete Documentation Index](docs/INDEX.md)
- **Architecture**: [System Architecture](docs/ARCHITECTURE.md)
- **Issues**: [GitHub Issues](https://github.com/sraj0501/automation_tools/issues)
- **Discussions**: [GitHub Discussions](https://github.com/sraj0501/automation_tools/discussions)

---

## License

This project is licensed under the MIT License - see the LICENSE file for details.

---

**Note**: This tool is designed for individual and team productivity enhancement. Ensure you have appropriate licenses and permissions for all integrated services.
