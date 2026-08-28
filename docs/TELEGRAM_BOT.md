# Telegram bot

The Telegram bot is Go-native and starts with the daemon. It provides daemon visibility and
pending-action correction parity; it is not a separate Python process or a full mirror of every CLI
integration.

## Setup

1. Create a bot with BotFather and obtain its token.
2. Send `/start` to the bot to see your numeric chat ID.
3. Add the following secrets to the registered DevTrack environment file:

```bash
TELEGRAM_ENABLED=true
TELEGRAM_BOT_TOKEN=<bot-token>
TELEGRAM_CHAT_ID=<notification-chat-id>
TELEGRAM_ALLOWED_CHAT_IDS=<authorized-id>[,<authorized-id>...]
```

4. Restart DevTrack and verify:

```bash
devtrack restart
devtrack telegram-status
```

`/start` and `/help` are available before authorization. Every other command requires the chat ID to
appear in `TELEGRAM_ALLOWED_CHAT_IDS`.

## Commands

| Command | Behavior |
|---|---|
| `/start` | Show the current chat ID and authorization instructions |
| `/help` | Show the implemented command list |
| `/status` | Daemon uptime and trigger summary |
| `/logs` | Last 20 daemon log lines |
| `/health` | Current daemon health snapshot |
| `/trigger` | Fire an immediate trigger |
| `/pause` / `/resume` | Pause or resume the scheduler |
| `/stop` / `/restart` | Control the daemon |
| `/reload` | Reload configuration |
| `/commits` | Show five recent commits |
| `/queue` | Show the pending-action summary and actions with correction buttons |

Commands previously documented for issue browsing, PM planning, vacation mode, and work-session
tracking are not implemented by the current Go bot. Use the corresponding `devtrack` CLI commands.

## Pending-action corrections

Low-confidence actions can produce proactive Telegram messages with **Approve**, **Reject**, and
**Edit** buttons. Approve dispatches through the Python queue endpoint; reject prevents posting; edit
prompts for replacement content. These actions update the same local pending-actions state used by
the CLI and TUI.

## Notifications

The daemon can deliver alert notifications to `TELEGRAM_CHAT_ID` or the configured allowed IDs.
Telegram does not change whether the underlying daemon capability runs; it is a visibility and
correction channel only.

## Troubleshooting

- Check `devtrack telegram-status` and `devtrack logs -f`.
- Verify the bot token without printing it into logs or issue reports.
- Confirm the chat ID is numeric and present in `TELEGRAM_ALLOWED_CHAT_IDS`.
- Only one process should poll a Telegram bot token at a time.
- An approved action still needs a reachable configured Python server to execute; failed dispatches
  remain visible for retry.
