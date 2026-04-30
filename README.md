<div align="center">

# DevTrack

**The developer automation layer that lives between your terminal and your project management tools.**

*Watches your Git activity. Prompts at the right moments. Routes work updates through AI. Keeps Azure DevOps, GitHub, and GitLab in sync — all on your machine.*

[![GitHub Release](https://img.shields.io/github/v/release/sraj0501/Devtrack_?label=release&color=blue)](https://github.com/sraj0501/Devtrack_/releases/latest)
[![Version](https://img.shields.io/badge/version-v2.0.0-blue)](https://github.com/sraj0501/Devtrack_/releases/tag/v2.0.0)
[![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-blue)](https://github.com/sraj0501/Devtrack_/releases/latest)
[![License](https://img.shields.io/badge/license-Community-green)](TERMS.md)

![DevTrack demo](wiki/assets/demo.gif)

</div>

---

## The 30-second pitch

You finish a feature. You type `git commit`. That's where DevTrack wakes up.

It refines your commit message with AI, asks how long the work took, opens a split-pane picker so you can link to an open issue in one keypress, posts a comment on that issue, and offers to push — all before you've left the terminal. When the session ends, it generates an EOD report in your own writing style, learned from your Teams messages.

The Go daemon is a 5 MB binary. The Python backend runs as a subprocess. Nothing leaves your machine unless you want it to.

---

## Install

```bash
# macOS / Linux — detect OS and architecture, then download the right binary
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
[ "$ARCH" = "x86_64" ]  && ARCH="amd64"
[ "$ARCH" = "aarch64" ] && ARCH="arm64"
curl -fsSL "https://github.com/sraj0501/Devtrack_/releases/latest/download/devtrack_${OS}_${ARCH}.tar.gz" | tar xz
sudo mv devtrack /usr/local/bin/

# Interactive setup wizard — choose standalone or full mode
devtrack setup

devtrack start
devtrack status
```

> **Updating?** Run `devtrack upgrade` to download and install the latest binary automatically.
> If the binary is in a root-owned location (e.g. `/usr/local/bin`), run `sudo devtrack upgrade` instead.
> Versioned migrations are applied automatically and the daemon is restarted after a successful upgrade.

> `devtrack setup` generates `.env` and writes `~/.devtrack/devtrack.conf` so the daemon can find it automatically. If you prefer manual setup, copy `.env_sample` to `.env` and fill in the values instead.

> Full walkthrough: [Installation Guide](docs/INSTALLATION.md) · [Quick Start](docs/QUICK_START.md)

#### Moving to a new machine?

Project memory and agent logs are committed to the repo (`.claude/memory/`, `Data/agent_logs/`). After cloning, wire up Claude Code's memory system with one command:

```bash
# Replace <path-key> with the absolute repo path, slashes replaced by hyphens
# e.g. repo at /home/sraj/devtrack → -home-sraj-devtrack
mkdir -p ~/.claude/projects/<path-key>/
ln -s $(pwd)/.claude/memory ~/.claude/projects/<path-key>/memory
```

Claude Code will then read and write memory directly to the repo, keeping it in sync with git.

---

## The core loop

```
You type: git commit -m "fix auth redirect"
                │
                ▼
        DevTrack intercepts
                │
          ┌─────┴─────┐
          │  AI refines │  → Accept / Enhance / Regenerate
          └─────┬─────┘
                │
        "Log this work? (y/n)"
                │
        "How long? (e.g. 2h, 30m)"
                │
        ┌────────────────┐
        │  Ticket picker  │  ↑/↓ to browse · / to filter · Enter to link
        │  (split pane)   │
        └────────┬───────┘
                 │
        Comment posted on issue
        Commit hash attached
                 │
        "Push to origin/branch? (y/n)"
```

Shell setup is one line, done once:

```bash
eval "$(devtrack shell-init)"    # add to ~/.zshrc or ~/.bashrc
devtrack enable-git              # opt this repo in
```

After that, `git commit` routes through DevTrack automatically for monitored repos. Everything else (`git push`, `git pull`, `git status`) goes straight to real git, unmodified. Escape hatch: `GIT_NO_DEVTRACK=1 git commit -m "skip"`.

> AI commit enhancement is only active when the daemon is running. If you stop it, `git commit` passes through with zero delay and no errors.

---

## What it connects to

| Integration | What DevTrack does |
|-------------|-------------------|
| **Azure DevOps** | Post commit comments, transition work item states, create missing items |
| **GitHub** | Comment on issues/PRs, sync recent activity, alert on review requests |
| **GitLab** | Comment on issues, list and view issues assigned to you, alert on assignments/notes/MR reviews |
| **Jira** | Alert on assignments, comments, and status changes |
| **Microsoft Teams** | Learn your communication style for personalized AI output |
| **Outlook / MS Graph** | Send EOD reports by email |
| **Telegram** | Remote control from your phone — `/workstart`, `/workreport`, `/plan` |
| **Slack** | `/devtrack status`, `/devtrack trigger`, and more via Socket Mode |
| **Ollama / OpenAI / Anthropic / Groq** | AI commit messages, reports, conflict resolution, git-sage agent |

---

## Key features

### Multi-repo monitoring

```yaml
# workspaces.yaml
workspaces:
  - name: work-api
    path: ~/work/api
    pm_platform: azure
    pm_assignee: jane@example.com
    pm_iteration_path: "MyProject\\Sprint 5"
    pm_area_path: "MyProject\\Backend"
  - name: oss-lib
    path: ~/oss/my-lib
    pm_platform: github
    pm_milestone: 3
```

Per-workspace PM overrides (`pm_assignee`, `pm_iteration_path`, `pm_area_path`, `pm_milestone`) are applied when DevTrack creates work items or issues for that repo — Azure uses `assigned_to`/`area_path`/`iteration_path`, GitHub/GitLab use `assignees` and `milestone`. Omit any field to use the global default.

```bash
devtrack workspace list
devtrack workspace add my-project ~/code/project --pm github
devtrack workspace install-hooks   # push post-commit hooks to all enabled workspaces
```

### Work session tracking

```bash
devtrack work start AUTH-42    # start timing a ticket
devtrack work stop             # auto-measures duration
devtrack work report           # EOD narrative in terminal
devtrack work report --email me@org.com
```

Every `git commit` while a session is active automatically attaches its hash — no manual logging.

### git-sage — local LLM git agent

![git-sage standup demo](wiki/assets/standup-demo.gif)

```bash
uv run python -m backend.git_sage do "squash my last 5 commits"
uv run python -m backend.git_sage ask "how do I rebase onto main?"
```

Runs an agentic loop: plans operations, executes them, reads output, handles failures with rollback, only asks when genuinely ambiguous. Session approval dialog (auto / review / suggest-only), step history, and interactive undo built in.

### Personalized AI ("Talk Like You")

```bash
devtrack enable-learning        # opt in — learns from Teams messages
devtrack show-profile           # view your inferred writing style
devtrack test-response "Completed auth module"
```

Combines a style profile with ChromaDB RAG (actual examples of how you write) to personalize every commit message, work update, and report the system generates.

### Ticket alerter

```bash
devtrack alerts                 # unread notifications (last 24 h)
devtrack alerts --all
devtrack alerts --clear
```

Background poller watches **GitHub**, **Azure DevOps**, **Jira**, and **GitLab** for assigned issues, new comments, review requests, and status changes. Delivers macOS OS notifications and terminal output.

- **GitHub**: Issue/PR assigned, review requested, comment on involved issue
- **Azure DevOps**: Work item assigned, comment added, state changed
- **Jira**: Assigned to you, new comments, status transitions (via REST API)
- **GitLab**: Issue assigned, new notes (comments), merge request review requested (`ALERT_GITLAB_ENABLED=true`)

Alert state (`last_checked` timestamps per source) persists to **SQLite** when MongoDB is unavailable, so poll continuity survives daemon restarts even without a MongoDB connection.

### AI project planning (via Telegram)

```
/plan Build a payment processing system
→ Decomposes into Epic + Stories + Tasks
→ Creates everything in Azure / GitLab / GitHub

/newproject
→ Pick platform · Enter requirements + deadline
→ AI fetches team workload · Generates sprint YAML
→ PM approves via email link · Sprints created with dependencies
```

### Auto-start at login

One command installs the right service for your OS — no manual plist or unit file editing:

```bash
devtrack autostart-install    # macOS → launchd LaunchAgent
                              # Linux/systemd → ~/.config/systemd/user/devtrack.service
                              # WSL without systemd → shell profile block
devtrack autostart-status     # show current auto-start status
devtrack autostart-uninstall  # remove auto-start
```

All current `.env` variables are baked into the service file at install time so the daemon starts with the correct environment even in a login session without a shell profile. Re-run `autostart-install` after changing `.env`.

### Interactive setup wizard (`devtrack setup`)

New in v2.0.0. Walks through every required setting interactively and writes the result for you:

```bash
devtrack setup
```

What it does:
- Checks Git is installed before proceeding
- Prompts for operating mode (Managed / Lightweight / External) and LLM provider credentials
- Generates `.env` in the repo root with `ADMIN_SECRET_KEY` auto-generated (no manual `openssl rand` step)
- Creates `~/.devtrack/` (XDG home dir) and writes `workspaces.yaml` there
- Writes `WORKSPACES_FILE` into `.env` pointing at the generated file
- Appends `eval "$(devtrack shell-init)"` to `.bashrc` / `.zshrc` automatically
- Writes `~/.devtrack/devtrack.conf` pointing at the generated `.env`

After `devtrack setup` completes, run `devtrack start` — no manual `source .env` needed.

### Automatic `.env` loading

The daemon automatically finds and loads `.env` at startup (v2.0.0+). Resolution order:

1. `DEVTRACK_ENV_FILE` environment variable (explicit path)
2. Path recorded in `~/.devtrack/devtrack.conf` (written by `devtrack setup`)
3. `.env` file next to the `devtrack` binary

You no longer need to manually `source .env` before `devtrack start` for most setups. The env-first rule still applies for `devtrack autostart-install` — run it after `devtrack setup` so the service bakes the correct variables.

### Self-update (`devtrack upgrade`)

```bash
devtrack upgrade          # download and install the latest release binary
sudo devtrack upgrade     # use when the binary is in a root-owned directory (e.g. /usr/local/bin)
```

What happens on upgrade:
1. Downloads the latest binary for your OS/arch from GitHub Releases
2. Applies all versioned migrations that have not yet run (schema changes, config file moves, etc.)
3. Auto-restarts the daemon so the new binary takes effect immediately
4. Falls back to `sudo cp` if the target directory is root-owned and the command wasn't run as root

### Post-commit hooks for all workspaces

```bash
devtrack workspace install-hooks    # install post-commit hook in every enabled workspace
```

Normally DevTrack installs hooks when the daemon starts. Use this command to push hooks to all workspaces at once — useful after adding new repos to `workspaces.yaml`.

### Webhook + Trigger server (HTTP mode)

The Go daemon spawns `backend.webhook_server` as a subprocess in the default managed mode. In external/Docker mode the server runs separately and the Go daemon connects to it over HTTPS. Either way the same FastAPI server handles both:

- **Inbound webhooks** from Azure DevOps, GitHub, GitLab, and Jira at `/webhooks/<source>`
- **Outbound triggers** from the Go daemon at `/trigger/commit` and `/trigger/timer`

```bash
# external/Docker mode only — managed mode starts this automatically
python -m backend.webhook_server
```

All trigger endpoints require the `X-DevTrack-API-Key` header (set `DEVTRACK_API_KEY` in `.env`). Webhook signature verification uses source-specific secrets (`AZURE_WEBHOOK_SECRET`, `GITHUB_WEBHOOK_SECRET`, etc.). GitLab webhooks are registered automatically at startup when `GITLAB_WEBHOOK_URL` is configured.

### AI development agents (Claude Code)

DevTrack ships three Claude Code sub-agents that automate the project's own development workflow. They are invoked inside Claude Code sessions, not from the terminal.

| Agent | Role |
|-------|------|
| **project-vision** | PM agent — breaks plans into tasks, writes the project board (`Data/agent_logs/project_board.md`), dispatches the engineer, enforces no-push-to-main and vision alignment, fires docu-agent after major features |
| **devtrack-engineer** | Engineer agent — always works on a task branch, commits exclusively through `devtrack git commit`, logs every commit to `Data/agent_logs/engineer_log.md`, opens a PR on completion, never pushes directly to `main` |
| **post-generator** | Turns the weekly engineer log into draft dev.to, Hacker News, and LinkedIn posts saved under `Data/agent_logs/posts/` |

Invoke from a Claude Code session:

```
/project-vision   # plan a new phase or ask for status
/devtrack-engineer  # dispatch the engineer on the current board task
/post-generator   # generate this week's posts from the engineer log
```

The PM and engineer agents share `Data/agent_logs/project_board.md` as a contract — PM writes tasks, engineer reads and updates status. All agent activity is captured in `Data/agent_logs/engineer_log.md`.

### Anonymous telemetry

DevTrack sends an anonymous install and daily-active ping (no code, no commit text, no personal data). To opt out:

```bash
devtrack telemetry off      # disable all pings
devtrack telemetry on       # re-enable
devtrack telemetry status   # show current setting
```

The ping sends: a random install UUID, a hashed hardware fingerprint, the event type (`install` / `active`), OS, arch, and version. Nothing else leaves your machine.

### Admin console (CS-3)

A browser-based admin console built with FastAPI + HTMX. Start it with:

```bash
devtrack admin-start       # web admin console at localhost:8090/admin/
```

Sign in with `ADMIN_USERNAME` / `ADMIN_PASSWORD` (set in `.env`). The dashboard shows live trigger-activity stats (triggers today, commits today, last trigger time, errors in the last 24 h) that refresh every 30 seconds via HTMX without a full page reload.

**Pages and capabilities:**

| Page | What you can do |
|------|----------------|
| **Dashboard** | Health overview, trigger throughput stats, quick links |
| **Users** | Create/delete users, change roles (`admin` / `viewer`), disable/enable accounts, reset passwords |
| **API Keys** | Generate and revoke per-user API keys |
| **License** | View current license tier, seat count, and terms acceptance status |
| **Server** | Real-time process table (CPU %, memory, health) with restart/stop/start controls |
| **Audit Log** | Full history of all admin actions |

**Single-process mode (`ADMIN_EMBED`):** By default the admin console runs as a separate process on `ADMIN_PORT` (default `8090`). Set `ADMIN_EMBED=true` to mount the admin router directly on the main webhook server at `/admin` — no extra port, no extra process:

```bash
# .env
ADMIN_EMBED=true          # mount admin at /admin on the webhook server (port 8089)
# or leave false (default) to run on a dedicated port:
ADMIN_PORT=8090
```

**Required `.env` keys for the admin console:**

```bash
ADMIN_USERNAME=admin
ADMIN_PASSWORD=changeme          # plain text (dev) or bcrypt hash ($2b$...)
ADMIN_SECRET_KEY=<random-string> # JWT signing key — generate with: openssl rand -hex 32
ADMIN_PORT=8090                  # ignored when ADMIN_EMBED=true
ADMIN_EMBED=false
```

### Server management

```bash
devtrack server-tui    # live process monitor — CPU%, memory, health + trigger throughput stats
devtrack tui           # full-screen dashboard: overview, activity, workspaces, alerts
```

The `server-tui` panel includes a **trigger throughput stats** pane that reads directly from the SQLite database and shows triggers fired today, commits today, last trigger time (HH:MM), and unprocessed-trigger error count for the last 24 hours. The pane degrades gracefully — it displays zeroes rather than crashing if the database is unavailable.

---

## Deployment modes

| Mode | `DEVTRACK_SERVER_MODE` | How | Use case |
|------|------------------------|-----|----------|
| **Managed** (default) | `managed` | Daemon spawns Python automatically | Local dev — full AI features |
| **Lightweight** | `lightweight` | Go daemon only — no Python | Git monitoring + scheduling without a Python environment |
| **External** | `external` | Python runs on a separate server; set `DEVTRACK_SERVER_URL` | Docker / self-hosted backend |
| **Cloud** | — | `devtrack cloud login --url URL --key KEY` | Remote managed backend |

`devtrack setup` prompts for the mode on first run and writes it to `.env`. In **Lightweight** mode, commands that depend on the Python backend show a clear error rather than crashing.

```bash
docker compose up -d   # starts Python backend + MongoDB, Redis, PostgreSQL
```

### `devtrack-server` — server-side management CLI

For **server / Linux deployments** where only the Python backend is hosted (External mode), a separate `devtrack-server` binary is distributed in the release tarball:

```bash
# Download the server tarball (no Go daemon required on the server)
curl -fsSL "https://github.com/sraj0501/Devtrack_/releases/latest/download/devtrack-server-<version>.tar.gz" | tar xz

devtrack-server start     # start the Python backend (webhook_server.py)
devtrack-server stop      # stop it
devtrack-server status    # show whether the backend process is running
devtrack-server logs      # tail recent log output
```

`devtrack-server` wraps the tarball-deployed Python backend in the same UX as the main CLI so server operators don't need to manage the subprocess manually.

---

## Technology

| Layer | Stack |
|-------|-------|
| Daemon / CLI | Go 1.24+, fsnotify, robfig/cron, modernc/sqlite |
| AI backend | Python 3.12+, uv, spaCy, aiohttp |
| Local LLM | Ollama (default) · OpenAI · Anthropic · Groq · LM Studio |
| Storage | SQLite (app state + projects/backlog/sprints), ChromaDB (RAG), optional MongoDB |
| Remote control | Telegram (python-telegram-bot) · Slack (slack-bolt Socket Mode) |
| PM integrations | Azure DevOps · GitLab · GitHub · Jira REST APIs |
| Admin console | FastAPI + HTMX, JWT auth, bcrypt passwords, SQLite user/audit store |
| Observability | runtime-narrative — structured story/stage traces on every webhook request |
| Config discipline | All Python modules use `backend.config.get()` — no `os.getenv()` calls in business logic |

---

## Documentation

| I want to… | Go to |
|-----------|-------|
| Install DevTrack | [Installation Guide](docs/INSTALLATION.md) |
| Set up interactively (new users) | [`devtrack setup`](#interactive-setup-wizard-devtrack-setup) |
| Run it for the first time | [Quick Start](docs/QUICK_START.md) |
| See all CLI commands | [CLI Reference](docs/CLI_REFERENCE.md) |
| Configure `.env` | [Configuration Reference](docs/CONFIGURATION.md) |
| Set up AI commits / git-sage | [Git Features](docs/GIT_FEATURES.md) · [git-sage](docs/GIT_SAGE.md) |
| Connect Azure DevOps | [Azure DevOps Guide](docs/AZURE_DEVOPS.md) |
| Connect GitLab / GitHub | [GitLab](docs/GITLAB.md) · [GitHub](docs/GITHUB.md) |
| Monitor multiple repos | [Multi-Repo Guide](docs/MULTI_REPO.md) |
| Set up Telegram / Slack | [Telegram](docs/TELEGRAM_BOT.md) · [Slack](docs/SLACK_BOT.md) |
| Set up AI providers | [LLM Guide](docs/LLM_GUIDE.md) |
| Enable "Talk Like You" | [Personalization](docs/PERSONALIZATION.md) |
| Auto-start at login (macOS/Linux/WSL) | [Auto-Start Guide](docs/AUTOSTART.md) |
| Track time / EOD report | [Work Tracker](docs/WORK_TRACKER.md) |
| Get ticket alerts (GitHub / Azure / Jira) | [Ticket Alerter](docs/TICKET_ALERTER.md) |
| Plan a project with AI | [AI Project Planning](docs/PROJECT_PLANNING.md) |
| Manage opt-out telemetry | [Telemetry](docs/TELEMETRY_PLAN.md) |
| Use external/Docker mode (HTTP triggers + webhooks) | [Webhook Server](docs/WEBHOOK_SERVER.md) |
| Deploy only the Python backend on a server | [`devtrack-server`](#devtrack-server--server-side-management-cli) |
| Run without Python (Lightweight mode) | [Deployment modes](#deployment-modes) |
| Update to the latest release | [`devtrack upgrade`](#self-update-devtrack-upgrade) |
| Monitor server health and trigger stats | [Server TUI](docs/SERVER_TUI.md) |
| Manage users, licenses, and API keys in a browser | [Admin Console](#admin-console-cs-3) |
| Use AI agents for development workflow | [`.claude/agents/`](.claude/agents/) |
| Fix a problem | [Troubleshooting](docs/TROUBLESHOOTING.md) |
| Understand the architecture | [Architecture](docs/ARCHITECTURE.md) |
| Full documentation index | [docs/INDEX.md](docs/INDEX.md) |

---

## Testing

```bash
cd devtrack-bin && go test ./...                    # Go layer (20+ tests)
uv run pytest backend/tests/                        # Python backend (492+ tests)
uv run pytest backend/tests/ -k cs1                # CS-1 HTTP trigger suite (28 tests)
uv run pytest backend/tests/test_server_tui.py     # server_tui helpers (37 headless tests)
uv run pytest backend/tests/test_admin_auth.py     # admin auth (19 tests)
uv run pytest backend/tests/test_admin_routes.py   # admin console routes (59+ tests)
uv run pytest backend/tests/test_admin_user_manager.py  # user manager (33+ tests)
uv run pytest backend/tests/test_jira_alerter.py   # Jira alerter (26 tests)
```

The CS-2 config audit enforces that **no Python business-logic module calls `os.getenv()` directly** — all 40+ backend modules were audited (TASK-001 through TASK-007) and now route through `backend.config` typed accessors. Missing required env vars produce a `ConfigError` with the exact variable name rather than a silent `None`.

---

## Privacy

All data stays on your machine. Ollama runs locally. External AI services (OpenAI, Anthropic, Groq) are optional — only prompt text is sent, never commit history or personal context. Learning from Teams requires explicit opt-in and can be deleted at any time with `devtrack learning-reset`.

---

## License

DevTrack Community License — free for personal use and teams up to 10 users. Enterprise (11+ users) requires a paid license.

```bash
devtrack terms          # read the terms
devtrack terms --accept # accept non-interactively (e.g. in CI)
```

Full text: [TERMS.md](TERMS.md)
