<div align="center">

# DevTrack

**Never write a standup again.**

*You commit. Tickets update, EOD reports write themselves — silently, in your voice, entirely on your machine.*

`devtrack` — a single Go binary. Local-first. Offline by default.

[![GitHub Release](https://img.shields.io/github/v/release/sraj0501/Devtrack_?label=release&color=blue)](https://github.com/sraj0501/Devtrack_/releases/latest)
[![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue)](https://github.com/sraj0501/Devtrack_/releases/latest)
[![License](https://img.shields.io/badge/license-Community-green)](TERMS.md)

![DevTrack demo](devtrack_wiki/wiki/assets/demo.gif)

</div>

---

## The 30-second pitch

You write code. DevTrack handles the rest.

A background daemon watches your commits and infers everything around them — which ticket you're on (from the branch name), what you did today, what the standup should say. It drafts the ticket comment and the EOD report **in your writing voice**, learned from your own git history. Your only obligation: name branches with ticket IDs.

**Nothing is sent behind your back.** Every outbound action — a Jira comment, a ticket transition, an email — is *staged* in a review queue first. You approve it, or you let it earn auto-approve over time. The daemon never prompts you, never blocks a commit, and never interrupts.

### And it's the memory your AI agents lack

> **Source preview:** MCP is available on upstream `main`; the latest public release, v3.0.10, does not include it yet.

Coding agents are session-based: they exist while invoked, then forget. DevTrack is always on. One command —

```bash
devtrack mcp setup
```

— and Claude Code knows your active ticket, today's commits, your pending queue, and how you write. Nothing else runs at 6pm, groups the day's commits by ticket, and has the EOD ready before you ask.

### Trust

Local Ollama by default; SQLite on disk. **The default daily path stays on your machine.** If you
configure a PM system, email/chat delivery, an external server, or a cloud LLM, DevTrack sends only
the payload needed for that enabled operation. Anonymous usage telemetry is **opt-in** and off unless
you run `devtrack telemetry on`.

---

## Ten-minute quickstart

The latest public release is **v3.0.10**. It predates the MCP command and the Phase 9 onboarding
work used below, so `releases/latest` cannot run this walkthrough yet. Until a newer release ships,
build the MCP-capable client from upstream `main`:

```bash
git clone --branch main --single-branch https://github.com/sraj0501/Devtrack_.git
cd Devtrack_/devtrack_client
go build -o devtrack .
sudo install -m 0755 devtrack /usr/local/bin/devtrack
```

Verify that this is an MCP-capable build, then run setup from the Git repository you want DevTrack
to watch. Choose `none` when asked for a PM integration if you only want to try the local path.
Managed mode requires a PostgreSQL URL for the Python service; Ollama remains the default LLM and
can finish preparing in the background.

```bash
devtrack mcp status           # must report six registered tools
cd /path/to/the/repo/to/watch
devtrack setup
```

### Minute 2: local context, no Python or model required

The Go binary and local SQLite database are ready as soon as setup finishes. Wire the current
repository into Claude Code, then exercise the same MCP server locally:

```bash
devtrack mcp setup
devtrack mcp test
```

Reload Claude Code after `mcp setup`. Its DevTrack tools can now read the active ticket, today's
commits, pending actions, voice profile, ticket context, and a template EOD summary. The MCP server
runs on demand over stdio; it does not need a background Python process.

### Minutes 3–10: start the silent worker

```bash
devtrack start
devtrack status
devtrack doctor
```

`status` and `doctor` report background Python, PostgreSQL, and LLM readiness without blocking Git
monitoring or MCP. Once the AI server reports ready, create a normal ticket-named branch and commit:

```bash
git switch -c feature/AUTH-42-refresh-token
git commit -m "fix auth redirect"

devtrack logs                # confirm the commit and ticket were detected
devtrack queue               # review local pending actions and confidence
devtrack eod                 # generate a narrative; stages it before any delivery
```

Do not run `queue approve` while evaluating the no-send path. With the workspace PM integration set
to `none` and no `--email` argument, the walkthrough uses no PM credentials and has no external
destination. For a disposable, recorder-friendly version that verifies actual log output instead of
using a canned transcript, see the [demo storyboard](docs/DEMO_STORYBOARD.md).

### What is ready when?

| Capability | Available before AI readiness | Needs the managed/external Python service |
|---|---:|---:|
| Git monitoring, ticket extraction, local SQLite | Yes | No |
| MCP setup, self-test, and local context tools | Yes | No |
| Queue inspection and correction for local actions | Yes | No |
| Voice-aware ticket-comment generation | No | Yes |
| `devtrack eod` generated narrative and staging | No | Yes |

The Python service and model preparation are background work. If they are not ready by minute ten,
keep coding and check `devtrack doctor`; the Go-native path remains usable and commits are not
blocked.

### Update an existing installation

```bash
devtrack upgrade
```

`devtrack upgrade` currently installs the latest public release, v3.0.10. It does not upgrade a
source-built Phase 9/MCP demo installation until that work is included in a newer public release.

The daemon mines enabled local repositories in Managed mode and builds the voice profile once the
local AI server is reachable. `devtrack status` and `devtrack doctor` show the persistent result and
suggest `devtrack work report` when the profile is ready.

Setup also checks Ollama's local model inventory. An existing generation model is used immediately
without another pull. If no usable local model is ready and an OpenAI or Anthropic key is already in
the environment, setup offers that key as a temporary fallback while Ollama downloads; Ollama stays
primary and automatically takes over when the local model becomes available.

> **Updating?** Run `devtrack upgrade` to download and install the latest binary automatically (fetched from GitHub Releases; supports Linux/macOS and Windows).
> If the binary is in a root-owned location (e.g. `/usr/local/bin`), run `sudo devtrack upgrade` instead. On Windows, re-run as Administrator if a permission error occurs.
> Versioned migrations are applied automatically and the daemon is restarted after a successful upgrade.

> `devtrack setup` writes a complete environment file under the DevTrack XDG data directory and registers it in `~/.devtrack/devtrack.conf`. Visible runtime defaults are editable; valid shell, CI, and secret-manager overrides still take precedence.

> Full walkthrough and guides: **[devtrack.cloud](https://devtrack.cloud)**

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

The daemon is **silent**. It does not prompt, block, or interrupt — it observes and stages.

```
You:  git checkout -b feat/AUTH-42-refresh-token
You:  git commit -m "fix auth redirect"
                │
                ▼
        DevTrack observes (background — you are not interrupted)
                │
        ├── infers ticket AUTH-42 from the branch name
        ├── drafts a ticket comment in your voice
        └── STAGES it — nothing is sent yet
                │
                ▼
        devtrack queue        # review what's waiting
        devtrack queue approve <id>
                │
        At 6pm: today's commits, grouped by ticket
                ▼
        devtrack eod          # the standup, already written
```

Review the queue whenever you like — it waits for you:

```bash
devtrack queue                  # what DevTrack wants to send
devtrack queue approve <id>     # send it
devtrack queue reject <id>      # discard it
devtrack eod                    # preview today's EOD report
```

### Optional: AI-enhanced commits

Separately, `devtrack git commit` is an **interactive** wrapper that refines your commit message with AI, offers a ticket picker, and can log time. It is opt-in and never part of the silent daemon path:

```bash
eval "$(devtrack shell-init)"    # add to ~/.zshrc or ~/.bashrc — done once
devtrack enable-git              # opt this repo in
```

After that, `git commit` routes through DevTrack for monitored repos. Everything else (`git push`, `git pull`, `git status`) goes straight to real git, unmodified. Escape hatch: `GIT_NO_DEVTRACK=1 git commit -m "skip"`.

> AI commit enhancement is only active when the daemon is running. If you stop it, `git commit` passes through with zero delay and no errors.

---

## What it connects to

| Integration | What DevTrack does |
|-------------|-------------------|
| **Azure DevOps** | Post commit comments, transition work item states, create missing items; PR approval detection via ADO Pull Requests API (real `IsPRApproved`, vote ≥ 10) |
| **GitHub** | Comment on issues/PRs, sync recent activity, alert on review requests |
| **GitLab** | Comment on issues; list, view, create, and sync issues through the Go connector |
| **Jira** | Server-side webhook and PM support; Go-client connector parity is part of the staged rollout |
| **Microsoft Teams** | Learn your communication style for personalized AI output |
| **Outlook / MS Graph** | Send EOD reports by email |
| **Telegram** | Go-native daemon control, logs, queue review/corrections, and notifications |
| **Slack** | Outbound alert notifications through an incoming webhook |
| **Ollama / OpenAI / Anthropic / Groq** | AI commit messages, reports, conflict resolution, git-sage agent |

---

## Key features

### The pending-actions queue — nothing is sent without review

Every outbound action DevTrack wants to take is **staged first**, never fired blind. This is the trust primitive: one reviewable queue for everything that would otherwise write to your Jira, GitHub, or inbox.

```bash
devtrack queue                   # list pending actions (default)
devtrack queue --all             # include recently posted/rejected
devtrack queue status            # one-line summary: pending / posted today / rejected today
devtrack queue approve <id>      # send it now
devtrack queue reject <id>       # discard — will not post
devtrack queue edit <id> <json>  # fix the payload before it goes
```

Each action carries a confidence score. As you approve a given action type repeatedly, it can earn auto-approve — so DevTrack gets quieter the more you trust it, not louder.

### End-of-day report — the standup, already written

```bash
devtrack eod                # generate today's report
devtrack eod show           # print the most recent narrative
devtrack eod status         # is one staged?
```

Groups the day's commits by ticket and writes the narrative in your voice. It is staged in the queue like anything else — review it, then send.

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
  # Dual-platform: same repo tracked in GitHub (code) + Azure DevOps (PM)
  - name: my-api-github
    path: ~/work/my-api
    pm_platform: github
    pm_org: acme-corp
    pm_username: sraj0501
    skip_issues: true          # code-only: excluded from devtrack issues + ticket sync
  - name: my-api-ado
    path: ~/work/my-api
    pm_platform: azure
    pm_org: acme-corp
    pm_username: jane@acme.com
```

Per-workspace PM overrides (`pm_assignee`, `pm_iteration_path`, `pm_area_path`, `pm_milestone`) are applied when DevTrack creates work items or issues for that repo — Azure uses `assigned_to`/`area_path`/`iteration_path`, GitHub/GitLab use `assignees` and `milestone`. Omit any field to use the global default.

`skip_issues: true` marks a workspace as code-only — it is excluded from `devtrack issues`, ticket sync, and the commit-time ticket picker. Use this when the same repo is tracked in two PM platforms (e.g. GitHub for code review, Azure DevOps for sprint planning) to prevent duplicate ticket lists.

```bash
devtrack workspace list
devtrack workspace add my-project ~/code/project --pm github
devtrack workspace install-hooks   # push post-commit hooks to all enabled workspaces
```

> **Empty repositories**: If a monitored workspace has no commits yet, the daemon watches the folder silently and begins triggering normally once the first commit arrives — no log spam or errors during the empty-repo period.

### Work session tracking

```bash
devtrack work start AUTH-42    # start timing a ticket
devtrack work stop             # auto-measures duration
devtrack work report           # EOD narrative in terminal
devtrack work report --email me@org.com
```

Every `git commit` while a session is active automatically attaches its hash — no manual logging.

### git-sage — local LLM git agent

![git-sage standup demo](devtrack_wiki/wiki/assets/standup-demo.gif)

```bash
devtrack sage do "squash my last 5 commits"
devtrack sage ask "how do I rebase onto main?"
```

Runs an agentic loop: plans operations, executes them, reads output, handles failures with rollback, only asks when genuinely ambiguous. Session approval dialog (auto / review / suggest-only), step history, and interactive undo built in.

### Personalized AI ("Talk Like You")

```bash
devtrack enable-learning        # opt in
devtrack learning-sync          # mine your git history
devtrack show-profile           # view your inferred writing style
devtrack test-response "Completed auth module"
```

Learns your writing voice from **your own git history** — local, automatic, no external service. It combines a style profile with ChromaDB RAG (real examples of how you write) to personalize every commit message, ticket comment, and report the system generates.

On a fresh Managed installation, the daemon automatically seeds Tier 0 voice data from enabled local Git
workspaces and generates the first profile in the background. Completion is saved locally in
`first-run-profile.json`; no PM action is sent and daemon startup never waits for the profile.

Microsoft Teams is an **optional** extra signal (`TEAMS_ENABLED`), not a requirement — the local git-history path is the default and works entirely offline.

### Ticket alerter

```bash
devtrack alerts                 # unread notifications (last 24 h)
devtrack alerts --all
devtrack alerts --clear
```

The Go-native background poller watches **GitHub** and **Azure DevOps** for assigned work, comments,
review requests, and status changes. It can deliver terminal, OS, Telegram, and Slack-webhook
notifications.

- **GitHub**: Issue/PR assigned, review requested, comment on involved issue
- **Azure DevOps**: Work item assigned, comment added, state changed
The poller is **Go-native** and runs inside the daemon — no Python subprocess, no MongoDB. Alert state (`last_checked` per source) and notifications persist to **SQLite**, so poll continuity survives daemon restarts.

### Telegram bot — remote control from your phone

Control the daemon and supervise queued work without opening a terminal:

```
/status | /logs | /health | /trigger
/pause | /resume | /stop | /restart | /reload
/commits
/queue
/approve <id> | /reject <id> | /edit <id> <json>
```

See [Telegram Bot setup guide](docs/TELEGRAM_BOT.md) for full configuration.

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

The daemon enforces a single running instance using an OS-level file lock (`Data/devtrack.lock`). On Windows this is a mandatory lock; on Unix a cooperative flock. Attempting to start a second instance prints a clear error and exits immediately rather than running in parallel and corrupting shared state.

### Interactive setup wizard (`devtrack setup`)

Walks through every required setting interactively and writes the result for you:

```bash
devtrack setup
```

What it does:
- Checks Git is installed before proceeding
- Prompts for operating mode (Managed / External) and LLM provider credentials
- Reuses an installed generation-capable Ollama model without downloading a prescribed model
- When Ollama still needs a model, can retain an already-present OpenAI/Anthropic key as an explicit
  temporary fallback; key values are never displayed and declining keeps setup local-only
- In Managed mode, starts the optional Python checkout, `uv sync`, and any needed local Ollama model
  pull in a detached worker; setup does not wait for them
- Generates the registered XDG environment file with visible runtime defaults and an auto-generated `ADMIN_SECRET_KEY`
- In Managed mode, writes and validates the required PostgreSQL connection configuration
- Creates `~/.devtrack/` (XDG home dir) and writes `workspaces.yaml` there
- Writes `WORKSPACES_FILE` into the generated environment file, pointing at the workspace file
- Appends `eval "$(devtrack shell-init)"` to `.bashrc` / `.zshrc` automatically
- Writes `~/.devtrack/devtrack.conf` pointing at the generated environment file

After `devtrack setup` completes, run `devtrack start` — no manual `source .env` needed. Git
monitoring, local SQLite, scheduling, and MCP are ready while the optional AI server finishes in the
background. Use `devtrack doctor` or `devtrack status` for progress; retry a failed bootstrap with
`devtrack doctor --repair`.

### Automatic `.env` loading

The daemon automatically finds and loads `.env` at startup. Resolution order:

1. `DEVTRACK_ENV_FILE` environment variable (explicit path)
2. Path recorded in `~/.devtrack/devtrack.conf` (written by `devtrack setup`)
3. `.env` file next to the `devtrack` binary

You no longer need to manually `source .env` before `devtrack start` for most setups. The env-first rule still applies for `devtrack autostart-install` — run it after `devtrack setup` so the service bakes the correct variables.

### Uninstall (`devtrack uninstall`)

```bash
devtrack uninstall             # confirm, then remove DevTrack and its data
devtrack uninstall --keep-data # remove DevTrack but preserve the data directory
devtrack uninstall --yes       # skip the confirmation prompt
```

The uninstall command asks once for confirmation unless `--yes` is supplied. It:
- Stops the running daemon (if active)
- Removes the autostart entry (launchd on macOS, systemd on Linux, Task Scheduler on Windows)
- Deletes the configured DevTrack data home, including managed-server files, unless `--keep-data` is supplied
- Removes the `devtrack` binary from `PATH`

The command prints the resolved targets before confirmation. There is no `--dry-run` flag.

### Self-update (`devtrack upgrade`)

```bash
devtrack upgrade          # download and install the latest release binary
sudo devtrack upgrade     # use when the binary is in a root-owned directory (e.g. /usr/local/bin)
```

What happens on upgrade:
1. Downloads the latest binary for your OS/arch from **GitHub Releases** (`sraj0501/Devtrack_`) — Linux/macOS use `.tar.gz`; Windows uses a direct `.exe`
2. Applies all versioned migrations that have not yet run (schema changes, config file moves, etc.)
3. Auto-restarts the daemon so the new binary takes effect immediately
4. On Unix: falls back to `sudo cp` automatically if the target directory is root-owned and the command wasn't run as root
5. On Windows: if a permission error occurs, a message is printed asking you to re-run the command as Administrator

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
cd devtrack_server && uv run python -m backend.webhook_server
```

All trigger endpoints require the `X-DevTrack-API-Key` header (set `DEVTRACK_API_KEY` in `.env`). Webhook signature verification uses source-specific secrets (`AZURE_WEBHOOK_SECRET`, `GITHUB_WEBHOOK_SECRET`, etc.). GitLab webhooks are registered automatically at startup when `GITLAB_WEBHOOK_URL` is configured.

The stable request and response shapes, authentication rules, and matching Go/Python contract tests
are documented in [the HTTP API contract](docs/HTTP_API.md).

### Claude Code / MCP Integration (Phase 8)

DevTrack exposes a Model Context Protocol (MCP) server so Claude Code automatically knows your active ticket, commit voice, and pending queue — no manual context-setting needed.

- **`devtrack mcp`** — starts the MCP server in stdio mode (the transport Claude Code uses)
- **`devtrack mcp serve --database PATH`** — starts it against an explicitly selected `devtrack.db`
  (used by packaged MCPB installs)
- **`devtrack mcp setup`** — writes `.mcp.json` in the current directory so Claude Code discovers the server automatically on next launch
- **`devtrack mcp status`** — shows the registered tools and server info
- **`devtrack mcp test`** — runs an in-process smoke test without starting a full server
- Six SQLite-backed tools: `get_active_context`, `get_today_commits`, `get_pending_actions`,
  `get_voice_profile`, `get_ticket_context`, `get_eod_summary`. Each declares a title and read-only,
  non-destructive, idempotent safety annotations.
- The stdio handshake negotiates finalized MCP versions through `2025-11-25`, retaining older-client
  compatibility. The newer `2026-07-28` per-request protocol is not supported yet.
- Reproducible MCPB 0.3 packaging and manifest validation are wired for Windows, macOS, and Linux in
  the next release pipeline. During bundle installation, select the `devtrack.db` created by
  `devtrack setup`; no MCPB was published with v3.0.10.

```bash
# One-time setup — run from your repo root
devtrack mcp setup    # writes .mcp.json
# Restart Claude Code — it will connect automatically via stdio
devtrack mcp status   # verify tools are registered
devtrack mcp test     # smoke-test the server in-process
```

Source: `devtrack_client/internal/mcp/` (server core) and `devtrack_client/mcp_cmd.go` (CLI).

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

### Anonymous telemetry — opt-in, off by default

DevTrack sends **nothing** unless you explicitly opt in:

```bash
devtrack telemetry status   # DISABLED by default
devtrack telemetry on       # opt in
devtrack telemetry off      # opt back out at any time
```

If (and only if) you opt in, the daemon sends an anonymous install/daily-active ping containing a random install UUID, a hashed hardware fingerprint, the event type (`install` / `active`), OS, arch, and version. Never code, commit text, diffs, ticket contents, or personal data.

The setting is stored locally and read directly by the daemon, so it works in every operating mode — including lightweight, with no server running.

### Admin console (CS-3)

A browser-based admin console built with FastAPI + HTMX. Start it with:

The admin console is server-owned. Run it from `devtrack_server/` with
`uv run python -m backend.admin`, or set `ADMIN_EMBED=true` to mount it on the managed webhook
server at `/admin`. The Go client intentionally has no `admin-start` command.

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

### Runtime visibility

```bash
devtrack status            # daemon, capabilities, and managed-bootstrap progress
devtrack doctor            # configuration and dependency diagnosis
devtrack doctor --repair   # retry a failed managed-server bootstrap
devtrack tui               # full-screen client dashboard
devtrack logs -f           # follow daemon logs
```

The Python server TUI remains available to server operators with
`cd devtrack_server && uv run python -m backend.server_tui`; it is not a Go-client command. Its
trigger-throughput pane reads the Go daemon's internal stats endpoint when PostgreSQL mode is active
and degrades to zero-valued stats when that endpoint is unavailable.

The daemon health subsystem checks these monitored services:

| Check | What is verified |
|-------|-----------------|
| Daemon process | PID file present and process alive |
| Python backend | `/health` HTTP endpoint reachable |
| SQLite | Database file readable and schema valid |
| Ollama | `/api/tags` reachable; response normalised across Ollama versions |
| Ports | Bound ports recorded and checked across restarts |

The last-known port list is persisted so runtime diagnostics can report conflicts across restarts.

---

## Deployment modes

| Mode | `DEVTRACK_SERVER_MODE` | How | Use case |
|------|------------------------|-----|----------|
| **Managed** (default) | `managed` | Daemon spawns Python automatically | Local dev — full AI features |
| **Lightweight** | `lightweight` | Go daemon only — no Python | Git monitoring + scheduling without a Python environment |
| **External** | `external` | Python runs on a separate server; set `DEVTRACK_SERVER_URL` | Docker / self-hosted backend |
| **Cloud** | — | `devtrack cloud login --url URL --key KEY` | Remote managed backend |

`devtrack setup` prompts for the mode on first run and writes it to the generated environment file. In **Lightweight** mode, commands that depend on the Python backend show a clear error rather than crashing.

> DevTrack runs **natively** — a Go binary plus a `uv`-managed Python server. The Go client keeps its
> offline source of truth in local SQLite and does not connect to a database server. PostgreSQL is
> mandatory for Python-server persistence and server-side events; MongoDB remains optional as a
> Teams voice-learning source. Server startup validates PostgreSQL and advances the Alembic schema
> before accepting traffic; there is no server-side SQLite fallback.

### Python AI server

**Managed mode** (default): `devtrack setup` configures the deterministic server location and starts
a background sparse checkout into `~/.local/share/devtrack/server/`, followed by `uv sync` and, for
the local Ollama provider only, a model pull when no usable generation model is already installed.
An opted-in cloud-key fast lane remains a fallback behind Ollama, so local inference takes over as
soon as the model is ready. The wizard does not wait for these steps;
`devtrack doctor` shows durable progress and failures. No manual dependency setup is needed.

**External mode** (server on a separate host): clone the repo on that host,
`cd devtrack_server && uv sync && uv run python -m backend.webhook_server`.
Set `DEVTRACK_SERVER_URL` on the client machine.

See [docs/INSTALLATION.md](docs/INSTALLATION.md) for the full setup walkthrough.

---

## Technology

| Layer | Stack |
|-------|-------|
| Daemon / CLI | Go 1.24+, fsnotify, robfig/cron, modernc/sqlite |
| AI backend | Python 3.12+, uv, aiohttp, LLM-first structured task parsing |
| Local LLM | Ollama (default) · OpenAI · Anthropic · Groq · LM Studio |
| Storage | Client SQLite (offline state), server PostgreSQL (required), ChromaDB (RAG), optional MongoDB |
| Remote control | Go-native Telegram bot · outbound Slack webhook notifier |
| PM integrations | Azure DevOps · GitLab · GitHub · Jira REST APIs |
| Admin console | FastAPI + HTMX, JWT auth, bcrypt passwords, PostgreSQL-backed user/audit data |
| Observability | runtime-narrative — structured story/stage traces on every webhook request |
| Config discipline | All Python modules use `backend.config.get()` — no `os.getenv()` calls in business logic |

---

## Documentation

Full user guides live on the project website: **[devtrack.cloud](https://devtrack.cloud)**.

Key references in this repo:

| I want to… | Go to |
|-----------|-------|
| Understand where the product is going | [**PRODUCT_BIBLE.md**](PRODUCT_BIBLE.md) — the source of truth |
| Install it | [Installation](docs/INSTALLATION.md) |
| Understand the architecture | [Architecture](docs/ARCHITECTURE.md) |
| Maintain the Go↔Python HTTP boundary | [HTTP API contract](docs/HTTP_API.md) |
| Review what DevTrack wants to send | [Pending-actions queue](#the-pending-actions-queue--nothing-is-sent-without-review) |
| See the client↔server split | [Decoupling plan](docs/CLIENT_SERVER_DECOUPLING_PLAN.md) · [Capability ownership](docs/CAPABILITIES_OWNERSHIP.md) |
| Set up the Telegram bot | [Telegram](docs/TELEGRAM_BOT.md) |
| Set up interactively (new users) | [`devtrack setup`](#interactive-setup-wizard-devtrack-setup) |
| Run without Python (Lightweight mode) | [Deployment modes](#deployment-modes) |
| Deploy only the Python backend on a server | [Python AI server](#python-ai-server) |
| Manage users, licenses, and API keys in a browser | [Admin Console](#admin-console-cs-3) |
| Update / remove DevTrack | [`devtrack upgrade`](#self-update-devtrack-upgrade) · [`devtrack uninstall`](#uninstall-devtrack-uninstall) |
| Use AI agents for development workflow | [`.claude/agents/`](.claude/agents/) |
| Connect Claude Code via MCP (Phase 8) | [MCP Integration](#claude-code--mcp-integration-phase-8) |

---

## Releasing

The canonical release pipeline is [`.github/workflows/release.yml`](.github/workflows/release.yml).
It runs when an authorized maintainer pushes a semantic-version tag:

```bash
GIT_NO_DEVTRACK=1 git tag -a vX.Y.Z -m "Release vX.Y.Z"
GIT_NO_DEVTRACK=1 git push origin vX.Y.Z
```

GitHub Actions runs the Go tests, cross-compiles Linux amd64/arm64, macOS amd64/arm64, and Windows
amd64, validates the generated MCPB manifests, then publishes the platform binaries/tarballs and
five matching `.mcpb` bundles. These artifacts remain source-pipeline capability until a maintainer
creates and pushes a new release tag; v3.0.10 contains neither MCP nor MCPB assets. Update
release-facing website copy in the same release change.

The older `scripts/release.ps1` helper is retained for local maintainer workflows, but it is not the
source of truth for published asset names or CI behavior.

---

## Testing

```bash
cd devtrack_client && go test ./...                     # Go client suite
cd devtrack_client && go vet ./...                      # lint

cd devtrack_server && uv sync                           # uv manages the venv — never pip
cd devtrack_server && uv run pytest backend/tests/      # Python server suite
cd devtrack_server && uv run pytest backend/tests/ -k <name>   # filter by name
```

Python business logic must use `backend.config` typed accessors rather than adding direct environment
reads. Missing required variables produce a `ConfigError` with the variable name rather than a
silent `None`.

---

## Privacy

**The default Go + SQLite + Ollama path is local and works without internet.** Configured external
services receive the minimum context required for the operation you enabled.

- **Cloud LLMs are optional.** OpenAI/Anthropic/Groq are used only if configured. The prompt may
  include commit messages, diff context, or work text required by the feature being invoked; do not
  enable a cloud provider if that conflicts with project policy.
- **Nothing is posted without review.** All outbound actions are staged in the pending-actions queue until you approve them.
- **Telemetry is opt-in** and off by default (`devtrack telemetry status`). No pings are sent unless you run `devtrack telemetry on`.
- **Voice learning is local in managed mode by default.** Git-history seeding is local; Teams and
  external-server learning sources require explicit configuration. Learning data can be wiped with
  `devtrack learning-reset`.

---

## License

DevTrack Community License — free for personal use and teams up to 10 users. Enterprise (11+ users) requires a paid license.

```bash
devtrack terms          # read the terms
devtrack terms --accept # accept non-interactively (e.g. in CI)
```

Full text: [TERMS.md](TERMS.md)
