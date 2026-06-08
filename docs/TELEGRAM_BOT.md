# Telegram Bot

The DevTrack Telegram bot lets you monitor and control the daemon, browse assigned issues, trigger syncs, and plan work — all from your phone.

---

## Setup

### 1. Create a bot with BotFather

Open Telegram, start a chat with **@BotFather**, and run `/newbot`. Copy the token it gives you.

### 2. Get your chat ID

Add the bot token to `.env`, start the bot (see below), then send `/start` to the bot in Telegram. It will reply with your chat ID before any auth is applied.

### 3. Configure `.env`

```env
TELEGRAM_ENABLED=true
TELEGRAM_BOT_TOKEN=123456789:AAFxxx...          # from BotFather
TELEGRAM_ALLOWED_CHAT_IDS=987654321             # comma-separated for multiple users

# Optional notification filters
TELEGRAM_NOTIFY_COMMITS=true
TELEGRAM_NOTIFY_TRIGGERS=true
TELEGRAM_NOTIFY_HEALTH=true
```

### 4. Start the bot

```bash
# Standalone (from devtrack_server/)
uv run python -m backend.telegram

# Or — the daemon auto-starts the bot when TELEGRAM_ENABLED=true
devtrack start
```

---

## Commands

### Daemon

| Command | What it does |
|---|---|
| `/status` | Daemon status, workspace list, service health |
| `/logs [N]` | Last N log lines (default 20, max 50) |
| `/stop` | Stop the daemon |
| `/restart` | Restart the daemon |
| `/pause` | Pause the scheduler (git monitoring continues) |
| `/resume` | Resume the scheduler |
| `/skipnext` | Skip the next scheduled trigger |
| `/trigger` | Fire an immediate work-update trigger |
| `/reloadconfig` | Reload `.env` + `workspaces.yaml` without restart |
| `/health` | Detailed per-service health from the DB |
| `/queue` | Message queue statistics |

### GitHub

| Command | What it does |
|---|---|
| `/github` | Open issues assigned to you (live query) |
| `/githubissue <number>` | Full details for a single issue |
| `/githubcreate [bug\|feature] <title>` | Create a new issue |
| `/githubsync` | Sync issues to local cache + AI server |
| `/githubcheck` | Verify GitHub connectivity and token |

### GitLab

| Command | What it does |
|---|---|
| `/gitlab` | Issues assigned to you (from local cache) |
| `/gitlabissue <project_id> <iid>` | Fetch a single issue live |
| `/gitlabcreate <title>` | Create a new issue (with milestone picker) |
| `/gitlabsync` | Sync issues to local cache + AI server |
| `/gitlabcheck` | Verify GitLab connectivity and token |

### Azure DevOps

| Command | What it does |
|---|---|
| `/issues` | Work items assigned to you (from local cache) |
| `/issue <id>` | Full work item details (live) |
| `/create [type] <title>` | Create a work item (with sprint picker). Types: `bug`, `task`, `feature`, `epic`, `story`, `pbi` |
| `/azuresync` | Sync work items to local cache + AI server |
| `/azurecheck` | Verify Azure DevOps connectivity and PAT |

### Ticket sync (all platforms)

| Command | What it does |
|---|---|
| `/ticketsync` | Sync all enabled PM platforms at once |
| `/ticketsync force` | Force drop-and-reload of the AI server cache |

### Ticket alerts

| Command | What it does |
|---|---|
| `/alerts` | Unread notifications from the last 24h |
| `/alertsall` | All notifications (read + unread) |
| `/alertsclear` | Mark all notifications as read |

### Work session tracking

| Command | What it does |
|---|---|
| `/workstart [ticket-ref]` | Start timing work on a ticket |
| `/workstop` | Stop the active session (auto-measures duration) |
| `/workadjust <minutes>` | Override time on the active or last session |
| `/workstatus` | Active session + today's completed sessions |
| `/workreport [--email addr]` | Generate EOD report; optionally email it |

### PM Agent

| Command | What it does |
|---|---|
| `/plan <problem>` | Decompose into Epic → Stories → Tasks and create them in your PM platform (platform picker inline) |
| `/newproject` | Full AI project planning wizard: platform → requirements → team picker → workload analysis → spec approval → sprint creation |

### Vacation

| Command | What it does |
|---|---|
| `/vacation status` | Show vacation mode state |
| `/vacation on [--until YYYY-MM-DD]` | Enable vacation auto-responder |
| `/vacation off` | Disable vacation mode |

### Info

| Command | What it does |
|---|---|
| `/commits` | Deferred commit queue by status |
| `/help` | Show the command list |
| `/start` | Show your chat ID (no auth required) |

---

## Live push notifications

When the daemon fires a trigger or receives a webhook event, the bot pushes a message to all authorized chat IDs automatically. Controlled by `.env` flags:

| Variable | Default | What it gates |
|---|---|---|
| `TELEGRAM_NOTIFY_COMMITS` | `true` | Push on every git commit detected |
| `TELEGRAM_NOTIFY_TRIGGERS` | `true` | Push on timer and report triggers |
| `TELEGRAM_NOTIFY_HEALTH` | `true` | Push on inbound webhook events |

---

## Multi-user

Add multiple chat IDs to `TELEGRAM_ALLOWED_CHAT_IDS` as a comma-separated list:

```env
TELEGRAM_ALLOWED_CHAT_IDS=111111111,222222222,333333333
```

Live event notifications are broadcast to **all** authorized chat IDs. Each user's commands are independent.

---

## Troubleshooting

**Bot doesn't respond**
- Verify `TELEGRAM_ENABLED=true` and `TELEGRAM_BOT_TOKEN` is set in `.env`.
- Check the bot is running: `devtrack status` should show `telegram_bot: UP`.
- Confirm your chat ID is in `TELEGRAM_ALLOWED_CHAT_IDS` (send `/start` to see your ID).

**Sync commands time out**
- The sync commands shell out to the `devtrack` binary. Ensure `PROJECT_ROOT` in `.env` points to the correct directory and the `devtrack` binary is on `PATH`.

**`/github` shows nothing after `/githubsync`**
- The `/github` command queries the GitHub API live; it doesn't read the local cache. Confirm `GITHUB_TOKEN` is set and `GITHUB_OWNER`/`GITHUB_REPO` (or `pm_project` in `workspaces.yaml`) are correct.

**`/issues` shows stale data**
- Run `/azuresync` to refresh the Azure cache, then `/issues` again.
