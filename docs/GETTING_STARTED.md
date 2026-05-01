# Getting Started with DevTrack

Welcome to DevTrack! This guide will help you understand what DevTrack is and get you up and running.

---

## What is DevTrack?

DevTrack is a developer automation tool that:

1. **Monitors your Git activity** - Watches your repository for commits and scheduled time intervals
2. **Prompts you for work updates** - Asks you what you're working on at key moments
3. **Processes with AI** - Uses natural language processing to understand your updates
4. **Updates your tasks** - Automatically updates Azure DevOps, GitHub, and other systems
5. **Generates reports** - Creates daily/weekly reports of your work

Think of it as your personal developer assistant that runs locally on your machine, learns your communication style, and helps you stay organized without manual data entry.

---

## Core Concepts

### 1. Git Monitoring
DevTrack runs as a background daemon that watches your Git repositories. When you make a commit, DevTrack:
- Analyzes the commit message and changes
- Uses AI to enhance your commit message with context
- Extracts task information from the message
- Updates your project management system

### 2. Scheduled Updates
On a schedule (configurable), DevTrack:
- Prompts you for work updates through a terminal interface
- Parses your natural language description
- Detects PR/issue numbers automatically
- Updates tasks in your project management system
- Enriches context with git information (branch, recent commits, etc.)

### 3. AI Processing
DevTrack uses AI for:
- **Natural Language Understanding** - Extracts tasks, time tracking, and status from your text
- **Context Enrichment** - Adds branch info, PR numbers, and commit history
- **Description Enhancement** - Improves clarity and consistency of task descriptions
- **Learning** - Learns your communication style for better future updates

All AI processing happens locally with Ollama by default (100% offline). You can optionally configure it to use OpenAI or Anthropic for better quality.

### 4. Integration
DevTrack integrates with:
- **Azure DevOps** - Work items, sprints, repositories
- **GitHub** - Issues, pull requests, repositories
- **Microsoft Teams** - Chat notifications
- **Outlook** - Email reports
- **Jira** - Issues and tasks

---

## System Architecture

```
Your Git Repo
      │
      ├─ Commit detected
      ├─ Timer triggers
      └─ Manual commands
            │
            ▼
       Go Daemon
      (devtrack-bin/)
    ├─ Git monitoring (fsnotify)
    ├─ Scheduling (cron)
    ├─ IPC server (TCP)
    └─ Local database (SQLite)
            │
    TCP/IP (JSON messages)
            │
            ▼
    Python Bridge
    (python_bridge.py)
    ├─ NLP parsing (spaCy)
    ├─ LLM enhancement (Ollama)
    ├─ TUI prompts
    └─ API integrations
            │
            ├──────────────┬──────────────┬──────────────┐
            ▼              ▼              ▼              ▼
        Azure DevOps    GitHub         Teams         Outlook
      Work Items      Issues/PRs      Chat          Email
```

---

## Installation Overview

DevTrack has two installation options:

### Option 1: Local Installation (Recommended)
Run DevTrack natively on your system. This is faster for development and easier to debug.

**Requires**:
- Go 1.20+
- Python 3.12+
- Git
- Ollama (optional, for AI)

**Time**: ~15 minutes for complete setup

### Option 2: Docker Installation
Run DevTrack in a container for consistency across platforms (Windows, macOS, Linux).

**Requires**:
- Docker and Docker Compose
- Ollama (runs on host machine)

**Time**: ~10 minutes for complete setup

Choose **Option 1** if you're developing on DevTrack or want easier debugging. Choose **Option 2** if you want consistent setup across multiple machines.

For detailed instructions, see [Installation Guide](INSTALLATION.md).

---

## First Run Checklist

After installation, here's what to do:

### 1. Verify Installation
```bash
# Check Go daemon is installed
devtrack --version

# Check Python dependencies
uv run python -c "import spacy; spacy.load('en_core_web_sm')"

# (Optional) Check Ollama is running
curl http://localhost:11434/api/tags
```

### 2. Configure .env
```bash
# Copy example configuration
cp .env_sample .env

# Edit with your values
nano .env
```

Key variables to set:
- `PROJECT_ROOT` - Path to this repository
- `DEVTRACK_WORKSPACE` - Git repo to monitor (can be same as PROJECT_ROOT)
- `DATA_DIR` - Where to store logs and database
- `LLM_PROVIDER` - Which AI to use (ollama, openai, or anthropic)

### 3. Start the Daemon
```bash
# Start DevTrack
devtrack start

# Check it's running
devtrack status

# View logs
devtrack logs
```

### 4. Make a Commit
```bash
# Make a commit in a monitored repo
git commit -m "Fixed authentication bug (2h)"

# Check logs to see DevTrack processing it
tail -f ~/.devtrack/daemon.log
```

### 5. See a Timer Trigger (Optional)
```bash
# Force an immediate timer trigger to see it in action
devtrack force-trigger

# This will prompt you for a work update in the terminal
```

---

## Common Questions

### Q: Does DevTrack upload my data to the cloud?
**A**: No. All data stays on your machine. Ollama runs locally. If you configure OpenAI or Anthropic, only AI requests are sent to those services - never your full commit messages or personal context.

### Q: What if I'm not ready for full automation?
**A**: You can use individual features:
- Just the enhanced commit message feature (`devtrack git commit`)
- Just the conflict resolver for manual merges
- Just the report generator
- Everything is optional and can be enabled/disabled

### Q: Can I use DevTrack with multiple Git repositories?
**A**: Yes! DevTrack monitors the `DEVTRACK_WORKSPACE` directory. You can:
- Set it to a parent directory that contains multiple repos
- Run multiple DevTrack instances with different `.env` files
- Configure each one independently

### Q: How does the AI learning work?
**A**: DevTrack can learn your communication style from:
- Your Git commit messages
- Your Teams chat history (with permission)
- Your Outlook emails (with permission)

This helps it generate better responses in your voice. Learning is **opt-in** and can be disabled anytime.

### Q: What if Ollama isn't available?
**A**: DevTrack degrades gracefully:
1. If local Ollama isn't running, it will try OpenAI or Anthropic (if configured)
2. If no commercial APIs are configured, features that need AI will be disabled
3. Core functionality (monitoring, parsing, updating tasks) works without AI

### Q: Can I use DevTrack offline?
**A**: Yes! With Ollama configured, DevTrack is 100% offline. Even the project management integrations work offline - they queue updates and sync when connection is restored.

---

## Next Steps

Now that you understand DevTrack:

1. **Ready to install?** → [Installation Guide](INSTALLATION.md)
2. **Want to see it in action?** → [Quick Start Guide](QUICK_START.md)
3. **Curious about architecture?** → [Architecture Overview](ARCHITECTURE.md)
4. **Want to configure AI?** → [LLM Guide](LLM_GUIDE.md)
5. **Interested in Git features?** → [Git Features Guide](GIT_FEATURES.md)

---

## Need Help?

- **Setup issues?** → [Troubleshooting Guide](TROUBLESHOOTING.md)
- **Command reference?** → [Commands Reference](COMMANDS.md)
- **Configuration details?** → [Configuration Reference](CONFIGURATION.md)
- **Still stuck?** → [Known Issues](../KNOWN_ISSUES.md)

---

## Key Features at a Glance

| Feature | What It Does | Requires Setup |
|---------|---|---|
| Git Monitoring | Detects commits and prompts you | Just .env configuration |
| Commit Enhancement | AI-powered commit messages | Ollama (or OpenAI/Anthropic) |
| Work Updates | Prompts you for status at intervals | Just .env configuration |
| NLP Parsing | Extracts tasks from your text | Just Python dependencies |
| Conflict Resolution | Auto-resolves merge conflicts | Just Python dependencies |
| Report Generation | Daily/weekly AI summaries | Ollama (or OpenAI/Anthropic) |
| Azure DevOps Integration | Updates work items automatically | Azure credentials in .env |
| GitHub Integration | Updates issues/PRs automatically | GitHub token in .env |
| Teams Integration | Posts reports and notifications | Teams credentials in .env |
| AI Learning | Learns your communication style | Opt-in, configurable |

---

**Ready?** Start with [Installation Guide](INSTALLATION.md) or jump to [Quick Start Guide](QUICK_START.md) if you're already familiar with similar tools.
