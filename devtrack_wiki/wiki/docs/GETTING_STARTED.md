# Getting Started with DevTrack

Welcome to DevTrack! This guide explains what DevTrack is and how to get up and running.

---

## What is DevTrack?

DevTrack is a developer automation tool that:

1. **Monitors your Git activity** — watches your repository for commits and scheduled intervals
2. **Prompts you for work updates** — asks what you're working on at key moments
3. **Processes with AI** — uses the configured LLM to enrich your updates
4. **Updates your tasks** — automatically updates Azure DevOps, GitHub, Jira, and other systems
5. **Generates reports** — creates daily/weekly summaries of your work

Think of it as your personal developer assistant that runs locally, learns your communication style, and handles task tracking without manual data entry.

---

## Core Concepts

### 1. Git Monitoring
DevTrack runs as a background daemon that watches your Git repositories. When you make a commit, DevTrack:
- Analyzes the commit message and diff
- Uses AI to enhance the commit message with context
- Extracts task information from natural language
- Updates your project management system

### 2. Scheduled Updates
On a configurable schedule, DevTrack:
- Prompts you for work updates through a terminal interface
- Parses your natural language description
- Detects PR/issue numbers automatically
- Updates tasks in your PM system
- Enriches context with git information (branch, recent commits, etc.)

### 3. AI Processing
DevTrack uses AI for:
- **Natural Language Understanding** — extracts tasks, time tracking, and status from your text
- **Context Enrichment** — adds branch info, PR numbers, and commit history
- **Description Enhancement** — improves clarity of task descriptions
- **Boardroom** — multi-persona AI review of plans and problems
- **Learning** — learns your communication style for better future updates

All AI processing is local with Ollama by default (100% offline). You can optionally configure OpenAI or Anthropic for higher quality.

### 4. Integrations
DevTrack integrates with:
- **Azure DevOps** — work items, sprints, repositories
- **GitHub** — issues, pull requests, repositories
- **Microsoft Teams** — chat notifications
- **Outlook** — email reports
- **Jira** — issues and tasks

---

## System Architecture

DevTrack is split into two independent components that communicate over HTTPS:

```
devtrack_client/          devtrack_server/
(Go binary + git-sage)    (Python AI pipeline)
        │                          │
        │   HTTPS POST /trigger/*  │
        │ ─────────────────────── ▶│
        │                          │
Git commits / cron timer    LLM enrichment / Admin UI
CLI commands                Azure DevOps / GitHub / Jira
SQLite (local)              Microsoft Graph (Teams/Email)
```

| Component | Language | Role |
|---|---|---|
| `devtrack` binary | Go | Daemon, git monitor, scheduler, CLI, git-sage |
| `devtrack_server` | Python | AI pipeline, admin GUI, PM integrations |

**The binary works standalone** — no server required. It runs git monitoring, git-sage, commit enhancement (via local Ollama), and all CLI commands. The server unlocks richer features: boardroom, validated LLM task enrichment, admin web UI, and full PM integrations.

---

## Installation Overview

### Option 1: Client Only (Recommended to start)

Install just the `devtrack` binary. Works fully offline with local Ollama.

**Requires:** Go 1.21+, Git, Ollama (optional)
**Time:** ~10 minutes

Steps: clone `devtrack_client` → `go build -o devtrack .` → configure `.env` → `devtrack start`

### Option 2: Client + Server

Install both components for the full AI pipeline.

**Requires:** Go 1.21+, Python 3.12/3.13, uv, Git, Ollama (optional)
**Time:** ~20 minutes

Steps: Option 1 + clone `devtrack_server` → `uv sync` → set `DEVTRACK_SERVER_URL` → start server

For detailed instructions, see [Installation Guide](INSTALLATION.md).

---

## First Run Checklist

After installation:

### 1. Verify the Binary
```bash
devtrack version
devtrack health
```

### 2. Configure .env
```bash
# In devtrack_client directory
cp .env_sample .env
nano .env
```

Key variables:
- `PROJECT_ROOT` — absolute path to `devtrack_client` directory
- `DEVTRACK_WORKSPACE` — Git repo to monitor
- `DATA_DIR` — where to store logs and database
- `LLM_PROVIDER` — `ollama` (default), `openai`, or `anthropic`
- `DEVTRACK_SERVER_URL` — set this only if you have `devtrack_server` running

### 3. Start the Daemon
```bash
# Source env first
source .env

# Start DevTrack
devtrack start

# Check it's running
devtrack status

# View logs
devtrack logs
```

### 4. Make a Commit
```bash
# In a monitored repo
git commit -m "Fixed authentication bug (2h)"

# Watch DevTrack process it
devtrack logs
```

### 5. Try a Timer Trigger
```bash
devtrack force-trigger
# Interactive prompt: "What are you working on?"
```

---

## Common Questions

### Q: Does DevTrack upload my data to the cloud?
**A:** No. All data stays on your machine. Ollama runs locally. If you configure OpenAI or Anthropic, only AI requests go to those services — never your raw commit messages or personal context.

### Q: Do I need the server to use DevTrack?
**A:** No. The `devtrack` binary works fully standalone. The server (`devtrack_server`) is optional — it unlocks the boardroom feature, validated LLM task enrichment, the admin web UI, and the full PM integrations.

### Q: Can I use DevTrack with multiple Git repositories?
**A:** Yes. Set `DEVTRACK_WORKSPACE` to a parent directory containing multiple repos, or run multiple daemon instances with different `.env` files.

### Q: What if Ollama isn't available?
**A:** DevTrack degrades gracefully:
1. If Ollama isn't running, it tries OpenAI or Anthropic (if configured)
2. If no commercial API is configured, AI-dependent features are disabled
3. Core monitoring and PM updates work without any AI

### Q: Can I use DevTrack fully offline?
**A:** Yes. With Ollama configured and no commercial keys set, DevTrack is 100% offline.

### Q: How does the AI learning work?
**A:** DevTrack can learn your communication style from Git commits, Teams chat history (with permission), and Outlook emails (with permission). This is **opt-in** and can be disabled at any time with `devtrack learning-reset`.

---

## Key Features at a Glance

| Feature | What It Does | Requires |
|---|---|---|
| Git Monitoring | Detects commits, triggers pipeline | `.env` only |
| Commit Enhancement | AI-powered commit messages | Ollama or cloud LLM |
| Work Updates | Prompts for status on schedule | `.env` only |
| git-sage | Agentic git operations via CLI | Ollama or cloud LLM |
| LLM task enrichment | Adds validated descriptive fields with raw-text fallback | `devtrack_server` core |
| Conflict Resolution | Auto-resolves merge conflicts | `devtrack_server` + AI tier |
| Report Generation | Daily/weekly AI summaries | Ollama or cloud LLM |
| Boardroom / Plan | Multi-persona AI plan review | `devtrack_server` |
| Admin UI | Web interface for users/keys/health | `devtrack_server` |
| Azure DevOps Integration | Updates work items automatically | Azure credentials |
| GitHub Integration | Updates issues/PRs | GitHub token |
| Teams Integration | Posts reports and notifications | Teams credentials |
| AI Learning | Learns your communication style | Opt-in, `devtrack_server` |

---

## Next Steps

1. **Install** → [Installation Guide](INSTALLATION.md)
2. **First commands** → [Quick Start Guide](QUICK_START.md)
3. **Architecture details** → [Architecture Overview](ARCHITECTURE.md)
4. **Configure AI** → [LLM Guide](LLM_GUIDE.md)
5. **Git features** → [Git Features Guide](GIT_FEATURES.md)

---

## Need Help?

- **Setup issues** → [Troubleshooting Guide](TROUBLESHOOTING.md)
- **Command reference** → [Quick Start Guide](QUICK_START.md)
- **Configuration details** → [Configuration Reference](CONFIGURATION.md)
- **Known bugs** → [Known Issues](KNOWN_ISSUES.md)
