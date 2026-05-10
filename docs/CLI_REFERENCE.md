# DevTrack CLI Reference

Complete reference for all `devtrack` commands.

---

## Daemon Control

```bash
devtrack start          # Start the background daemon
devtrack stop           # Stop the daemon gracefully
devtrack restart        # Stop and restart (kills all sub-services, respawns clean)
devtrack status         # Show daemon status, uptime, PID, and service health
devtrack logs           # Show last 50 log lines
devtrack logs -f        # Follow log output in real time (like tail -f)
```

**Tip:** `devtrack restart` fully tears down and respawns the Python bridge, webhook server, Telegram bot, Azure assignment poller, and GitLab assignment poller.

---

## Scheduler

```bash
devtrack pause           # Pause timed work-update triggers (git monitoring continues)
devtrack resume          # Resume the scheduler
devtrack force-trigger   # Fire a work-update prompt immediately (no waiting)
devtrack skip-next       # Skip the next scheduled trigger
devtrack send-summary    # Generate and send the daily summary now
```

---

## Git (AI-Enhanced Commits)

```bash
devtrack git commit -m "message"           # AI-enhanced commit (up to 5 refinement attempts)
devtrack git commit -m "message" --dry-run # Preview AI suggestion without committing
devtrack git history                       # Show recent commit history
devtrack git messages                      # Alias for git history
```

**Refinement loop — Enhance token budget:** In the interactive refinement loop, choosing **E** (Enhance/improve message) doubles the AI model's token allowance to `COMMIT_LLM_MAX_TOKENS × 2` for that single call, producing richer and more detailed messages. The budget resets to the normal limit for any subsequent iteration unless you press **E** again.

See [GIT_COMMIT_WORKFLOW.md](GIT_COMMIT_WORKFLOW.md) for the full interactive workflow.

### Work Logging Flow (after commit accepted)

After a commit is created and you answer **y** to `Log this work? (y/n):`, DevTrack runs a two-step work logging sequence.

#### Step 1 — Time spent prompt

```
How long did this take? (e.g. 2h, 30m) [Enter to skip]:
```

Type a duration (`2h`, `30m`, `1h30m`) or press **Enter** to skip. The value is attached to the comment posted on the linked issue.

#### Step 2 — Interactive ticket picker

A curses split-pane picker opens so you can link the commit to an existing issue without leaving the terminal.

```
┌─────────────────────────────┬──────────────────────────────────────────┐
│  Open Issues          [3]   │  #42 Fix login redirect loop             │
│ ─────────────────────────── │ ──────────────────────────────────────── │
│ ▶ #42  Fix login redirect   │  Users are stuck in a redirect loop      │
│   #38  Add dark mode toggle │  when the session cookie is missing.     │
│   #31  Update README        │                                          │
│                             │  Labels: bug, auth                       │
│  / to filter  n new  q skip │  Opened 2026-03-20 by sraj               │
└─────────────────────────────┴──────────────────────────────────────────┘
```

**Left pane** — scrollable list of your open issues from the workspace's configured PM platform (GitHub, GitLab, or Azure DevOps).

**Right pane** — full body of the currently highlighted issue, updated as you move.

**Keyboard controls:**

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | Select highlighted issue and post comment |
| `/` | Open filter bar — type to narrow the list by title |
| `n` | Create a new issue on the PM platform and link it |
| `Esc` / `q` | Skip — do not link to any issue |

When an issue is selected, DevTrack posts a comment containing:

- Commit hash (short SHA)
- Commit message
- Time spent (if provided in Step 1)

#### Step 3 — Auto-push prompt

After the ticket picker closes (whether an issue was linked or skipped), DevTrack asks whether to push the commit:

```
🚀 Push to origin/<branch>? (y/n)
y
→ Pushing...
✓ Pushed to origin/bot_automation
```

Pressing `n` prints a reminder and skips the push:

```
Skipped. Run 'git push origin <branch>' when ready.
```

#### Non-TTY / no-terminal fallback

When DevTrack is running without an interactive terminal (e.g. inside a CI pipeline, over SSH without a PTY, or when the curses library is unavailable), the picker falls back to a numbered list printed to stdout:

```
Open issues:
  1. #42  Fix login redirect loop
  2. #38  Add dark mode toggle
  3. #31  Update README
  0. Skip (do not link)

Select issue number:
```

Type the number and press **Enter**, or **0** to skip.

---

## Shell Integration

Skip the `devtrack` prefix — type `git commit` directly for monitored repos.

### Setup (one time)

```bash
# Add to ~/.zshrc or ~/.bashrc, then reload your shell
eval "$(devtrack shell-init)"
source ~/.zshrc
```

`eval "$(devtrack shell-init)"` installs two shell functions:

| Function | Purpose |
|----------|---------|
| `git()` | Intercepts `git commit`, `git history`, and `git messages` for DevTrack workspaces; passes everything else straight to the real git binary |
| `devtrack()` | Wraps the devtrack binary; after `devtrack start`, `devtrack restart`, or `devtrack enable-git` it automatically re-evals `shell-init` so the `git()` function is always current |

The setup is **self-maintaining**: after the one-time addition to your rc file, you never need to run `eval "$(devtrack shell-init)"` again manually. Rebuilding or updating the binary? Just run `devtrack restart` and the shell functions refresh silently in the current session.

### Per-Repo Opt-In

```bash
devtrack enable-git    # Opt this repo in  — sets git config devtrack.enabled=true
devtrack disable-git   # Opt this repo out — removes git config devtrack.enabled
```

### Workspace Detection

```bash
devtrack is-workspace  # Exit 0 if CWD is a DevTrack workspace (used internally)
```

This command is called automatically by the shell `git()` function to check `workspaces.yaml`. Repos already in `workspaces.yaml` are intercepted without `devtrack enable-git`.

### Bypass

```bash
GIT_NO_DEVTRACK=1 git commit -m "message"   # Skip DevTrack for this one command
command git commit -m "message"             # Always calls real git
```

### What gets intercepted

| Command | Intercepted? |
|---------|-------------|
| `git commit` | Yes — routed through DevTrack AI enhancement |
| `git add` (no args) | Yes — automatically stages all changes (`git add .`) |
| `git history` | Yes — shows DevTrack commit history |
| `git messages` | Yes — alias for git history |
| `git push`, `git pull`, `git status`, `git log`, … | No — always real git |

### `git add` with No Arguments

When shell integration is active, running `git add` with no path prints a notice and stages everything:

```
$ git add
No path specified — staging all changes (git add .)
```

Providing one or more paths passes them straight to the real `git add` with no change in behaviour:

```bash
git add src/auth.js        # stages only that file
git add src/ tests/        # multiple paths, unchanged
```

---

## git-sage Agent

```bash
# These commands run the git-sage LLM agent directly
uv run python -m backend.git_sage ask "how do I undo my last commit?"
uv run python -m backend.git_sage do  "squash my last 3 commits"
uv run python -m backend.git_sage interactive   # Persistent session (ask + do loop)
```

See [GIT_SAGE.md](GIT_SAGE.md) for full documentation.

---

## Azure DevOps

```bash
devtrack azure-check                      # Test config and connectivity
devtrack azure-list                       # List open work items assigned to you
devtrack azure-list --all                 # All work items (no state filter)
devtrack azure-list --state "Active,New"  # Filter by comma-separated states
devtrack azure-view <id>                  # Show full details for a work item
devtrack azure-sync                       # Full resync (clears cache, fetches all)
devtrack azure-sync --full               # Explicit full resync
devtrack azure-sync --hours 24           # Only items changed in last 24 h
```

See [AZURE_DEVOPS.md](AZURE_DEVOPS.md) for setup and configuration.

---

## GitLab

```bash
devtrack gitlab-check                        # Test config and connectivity
devtrack gitlab-list                         # List open issues assigned to you
devtrack gitlab-list --closed               # Include closed issues
devtrack gitlab-list --state <state>        # Filter by state (opened, closed, all)
devtrack gitlab-view <project_id> <iid>     # Show full details for an issue
devtrack gitlab-sync                         # Full resync (fetches all open issues)
devtrack gitlab-sync --full                 # Explicit full resync
devtrack gitlab-sync --hours 24            # Only issues updated in last 24 h
```

See [GITLAB.md](GITLAB.md) for setup and configuration.

---

## GitHub

```bash
devtrack github-check                        # Test config and connectivity
devtrack github-list                         # List open issues assigned to you
devtrack github-list --closed               # Include closed issues
devtrack github-list --state <state>        # Filter by state (open, closed, all)
devtrack github-view <number>               # Show full details for an issue
devtrack github-sync                         # Full resync (fetches all open issues)
devtrack github-sync --full                 # Explicit full resync
devtrack github-sync --hours 24            # Only issues updated in last 24 h
```

See [GITHUB.md](GITHUB.md) for setup and configuration.

---

## Workspace Management

Manage the multi-repo `workspaces.yaml` without editing the file by hand. All mutation commands (`add`, `remove`, `enable`, `disable`) automatically send a reload signal to the running daemon so changes take effect immediately — no `devtrack restart` needed.

```bash
devtrack workspace list                              # List all configured workspaces and their status
devtrack workspace add <name> <path> --pm <platform> # Add a new workspace
devtrack workspace remove <name>                     # Remove a workspace by name
devtrack workspace enable <name>                     # Set enabled: true for a workspace
devtrack workspace disable <name>                    # Set enabled: false for a workspace
devtrack workspace reload                            # Send reload signal to the daemon (manual)
```

### workspace add flags

| Flag | Values | Description |
|------|--------|-------------|
| `--pm` | `azure`, `gitlab`, `github`, `jira`, `none` | PM platform for this workspace (required) |

Example:

```bash
devtrack workspace add work-api ~/work/api --pm azure
devtrack workspace add oss-lib  ~/oss/lib  --pm github
```

After adding, the daemon picks up the new workspace without a restart. If the daemon is not running, the entry is written to `workspaces.yaml` and will be active on the next `devtrack start`.

See [MULTI_REPO.md](MULTI_REPO.md) for full multi-repo documentation.

---

## Boardroom

Multi-persona AI review of a problem or plan. Seven AI personas (architect, security, PM, devil's advocate, engineer, analyst, scalability) each provide PROs/CONs; the session produces a SWOT matrix and a final verdict.

```bash
devtrack boardroom "<problem>"                             # Inline problem — all 7 personas review it
devtrack boardroom --file <plan.md>                        # Review a Markdown plan file
devtrack boardroom --folder <plans/>                       # Review all .md files in a folder
devtrack boardroom --file <plan.md> --output <report.md>  # Save report to file
devtrack boardroom --file <plan.md> --interactive          # Auto-enter interactive chat after review
```

After the review you can optionally continue with a chat session, directing follow-up questions to individual personas or the full group.

---

## Plan

Decompose a problem statement into an Epic → Story → Task hierarchy and create the items on your configured PM platform (GitHub, GitLab, Azure DevOps, Jira).

```bash
devtrack plan "<problem>"          # Inline problem statement
devtrack plan --file <plan.md>     # Load problem from a Markdown file
devtrack plan --folder <plans/>    # Process all .md files in a folder
```

A platform picker is shown if `PM_AGENT_DEFAULT_PLATFORM` is not set.

See [PM_AGENT.md](PM_AGENT.md) for full PM agent documentation.

---

## PM Agent (via Telegram)

The PM Agent is also available via Telegram. Use `/plan` in your Telegram bot:

```
/plan Build a user authentication system
```

This opens a platform picker → LLM decomposes into Epic/Story/Task → shows a preview → confirm to create all items.

See [PM_AGENT.md](PM_AGENT.md) for full documentation.

---

## Offline Resilience

```bash
devtrack queue             # Show message queue stats (pending, processed, failed)
devtrack commits pending   # List deferred commits waiting for AI enhancement
devtrack commits review    # Review AI-enhanced deferred commits interactively
```

---

## Reports

```bash
devtrack preview-report [date]   # Preview today's report (or YYYY-MM-DD)
devtrack send-report <email>     # Email the daily report to an address
devtrack save-report [date]      # Save report to a file
devtrack db-stats                # Database statistics and analytics
devtrack stats                   # Alias for db-stats
```

---

## Personalized AI Learning

```bash
devtrack enable-learning [days]   # Enable learning (default: last 30 days)
devtrack learning-sync            # Delta sync — only new messages since last run
devtrack learning-sync --full     # Force full re-collection (ignore delta state)
devtrack learning-status          # Show consent status and sample count
devtrack show-profile             # Display your learned communication profile
devtrack test-response <text>     # Generate a personalized response (no auth needed)
devtrack revoke-consent           # Delete all learning data and revoke consent
devtrack learning-reset           # Wipe everything and start fresh
```

### Learning Cron

```bash
devtrack learning-setup-cron      # Install daily cron from LEARNING_CRON_SCHEDULE in .env
devtrack learning-cron-status     # Show cron entry and schedule settings
devtrack learning-remove-cron     # Remove the cron entry
```

See [PERSONALIZATION.md](PERSONALIZATION.md) for full documentation.

---

## Telegram Bot

```bash
devtrack telegram-status   # Show whether the Telegram bot process is alive
```

**In Telegram** (all require your chat ID to be in `TELEGRAM_ALLOWED_CHAT_IDS`):

| Command | Description |
|---------|-------------|
| `/start` | Get your chat ID (for initial setup) |
| `/help` | List all bot commands |
| `/status` | Daemon status and service health |
| `/logs [N]` | Last N log lines (default 20, max 50) |
| `/trigger` | Force a work-update trigger |
| `/pause` | Pause the scheduler |
| `/resume` | Resume the scheduler |
| `/queue` | Message queue statistics |
| `/commits` | Deferred commit status |
| `/health` | Per-service health check |
| `/azure` | List Azure work items from cache |
| `/azureissue <id>` | View a specific Azure work item |
| `/azurecreate` | Create a new Azure work item interactively |
| `/gitlab` | List GitLab issues from cache |
| `/gitlabissue <project_id> <iid>` | View a specific GitLab issue |
| `/gitlabcreate` | Create a new GitLab issue interactively |
| `/github` | List open GitHub issues assigned to you |
| `/githubissue <number>` | View a specific GitHub issue |
| `/githubcreate [bug\|feature\|task] <title>` | Create a new GitHub issue |
| `/plan <problem>` | Decompose a problem → Epic/Story/Task → create items |

---

## devtrack-server — Python Backend Management

The `devtrack-server` script manages the Python backend process. It is installed to `~/.local/bin/devtrack-server` by `devtrack-server install`.

### Lifecycle

```bash
devtrack-server install        # Copy backend files, install core deps, add to PATH
devtrack-server setup          # Interactive .env configuration wizard
devtrack-server start          # Start the webhook server in the background
devtrack-server stop           # Stop the running server
devtrack-server restart        # Stop then start
devtrack-server status         # Show process state and HTTP health
devtrack-server logs           # Tail the server log (Ctrl+C to exit)
devtrack-server upgrade        # Download and install the latest release
devtrack-server upgrade --check # Check for updates without installing
devtrack-server uninstall      # Stop the server and remove all installed files
devtrack-server version        # Print installed version
```

### Feature Management

The Python backend uses a two-tier dependency model. The `core` tier is installed by default; the `ai` tier adds NLP and RAG personalization features on demand.

```bash
devtrack-server features       # Show which feature tiers are installed
devtrack-server enable ai      # Install the ai extra (spaCy, ChromaDB, sentence-transformers)
```

#### `devtrack-server features`

Checks whether the optional `ai` extra is installed in the server virtualenv and prints a status table:

```
── DevTrack Server Features ──

  ✓  core    web server, LLM, integrations, reporting
  ✗  ai      NLP parser, RAG personalization (run: devtrack-server enable ai)
```

A `✓` means that feature tier is active. A `✗` means the packages are not installed — the server still runs, but NLP-powered features (smarter work-update parsing, RAG commit style) are unavailable until the `ai` extra is enabled.

#### `devtrack-server enable ai`

Installs the `ai` optional-dependency group into the server virtualenv using `uv pip install "devtrack[ai]"`. This includes:

- `spacy>=3.7.0` — NLP parser for work-update extraction
- `en_core_web_sm` — English spaCy model
- `sentence-transformers>=2.2.0` — embeddings for RAG personalization
- `chromadb>=0.4.0` — local vector store for RAG few-shot retrieval

After the install completes, restart the server for the features to activate:

```bash
devtrack-server enable ai
devtrack-server restart
```

---

## Self-Update

```bash
devtrack upgrade          # Check for a new release and install it
devtrack upgrade --check  # Check for updates without installing
```

`devtrack upgrade` fetches the latest release from the **GitLab Package Registry** (`gitlab.com/devtrack3_cloud/devtrack_client`).

**What it does:**
1. Queries the GitLab Releases API for the newest tag.
2. Downloads the archive for the current OS/arch (`.tar.gz` on Unix, `.zip` on Windows).
3. Replaces the running binary in place.
   - **Unix**: if the destination is root-owned, retries with `sudo cp` (you will be prompted for your password).
   - **Windows**: if permission is denied, prints a message asking you to re-run as Administrator or copy the file manually.
4. Runs pending configuration migrations (`RunPendingMigrations`).
5. Restarts the daemon automatically if it was running.

**Notes:**
- `dev` builds (built from source) are reported as "not a release" and the upgrade is skipped.
- The `--check` flag prints whether an update is available without downloading anything.

---

## Info & Settings

```bash
devtrack version    # Show version information
devtrack help       # Show all commands with brief descriptions
devtrack settings   # Show config paths and key environment settings
```

---

## Environment Variables Quick Reference

All configuration comes from `.env`. Copy `.env_sample` to `.env` and fill in your values. Key groups:

| Group | Variables |
|-------|-----------|
| Paths | `PROJECT_ROOT`, `DEVTRACK_WORKSPACE`, `DATA_DIR` |
| IPC | `IPC_HOST`, `IPC_PORT`, `IPC_CONNECT_TIMEOUT_SECS` |
| LLM | `LLM_PROVIDER`, `OLLAMA_HOST`, `OLLAMA_MODEL` |
| Azure | `AZURE_DEVOPS_PAT`, `AZURE_ORGANIZATION`, `AZURE_PROJECT`, `EMAIL` |
| GitLab | `GITLAB_URL`, `GITLAB_PAT`, `GITLAB_PROJECT_ID` |
| GitHub | `GITHUB_TOKEN`, `GITHUB_OWNER`, `GITHUB_REPO` |
| Telegram | `TELEGRAM_ENABLED`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_ALLOWED_CHAT_IDS` |
| PM Agent | `PM_AGENT_DEFAULT_PLATFORM`, `PM_AGENT_MAX_ITEMS_PER_LEVEL` |

See [CONFIGURATION.md](CONFIGURATION.md) for the complete variable reference.
