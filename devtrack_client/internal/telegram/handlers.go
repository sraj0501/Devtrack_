package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const helpText = `*DevTrack Bot Commands*

*Daemon*
/status — Uptime and trigger count
/logs — Last 20 log lines
/health — Service health check
/trigger — Force an immediate trigger
/pause — Pause the scheduler
/resume — Resume the scheduler
/stop — Stop the daemon
/restart — Restart the daemon
/reload — Reload configuration

*Activity*
/commits — Last 5 commits`

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	cmd := msg.Command()

	// No-auth commands
	switch cmd {
	case "start":
		b.reply(msg, fmt.Sprintf(
			"*DevTrack Bot*\n\nYour chat ID: `%d`\n\nAdd it to `TELEGRAM_ALLOWED_CHAT_IDS` in `.env` to authorize.\n\nUse /help for available commands.",
			msg.Chat.ID,
		))
		return
	case "help":
		b.reply(msg, helpText)
		return
	}

	if !b.isAuthorized(msg.Chat.ID) {
		b.reply(msg, fmt.Sprintf("Unauthorized. Your chat ID `%d` is not in the allowed list.", msg.Chat.ID))
		return
	}

	switch cmd {
	case "status":
		if b.hooks.StatusText != nil {
			b.reply(msg, b.hooks.StatusText())
		} else {
			b.reply(msg, "Status unavailable.")
		}

	case "logs":
		if b.hooks.LogLines != nil {
			b.reply(msg, b.hooks.LogLines(20))
		} else {
			b.reply(msg, "Logs unavailable.")
		}

	case "health":
		if b.hooks.HealthText != nil {
			b.reply(msg, b.hooks.HealthText())
		} else {
			b.reply(msg, "Health unavailable.")
		}

	case "trigger":
		if b.hooks.ForceTrigger == nil {
			b.reply(msg, "Trigger unavailable.")
			return
		}
		if err := b.hooks.ForceTrigger(); err != nil {
			b.reply(msg, "Failed: "+err.Error())
		} else {
			b.reply(msg, "Trigger fired.")
		}

	case "pause":
		if b.hooks.Pause == nil {
			b.reply(msg, "Pause unavailable.")
			return
		}
		if err := b.hooks.Pause(); err != nil {
			b.reply(msg, "Failed: "+err.Error())
		} else {
			b.reply(msg, "Scheduler paused.")
		}

	case "resume":
		if b.hooks.Resume == nil {
			b.reply(msg, "Resume unavailable.")
			return
		}
		if err := b.hooks.Resume(); err != nil {
			b.reply(msg, "Failed: "+err.Error())
		} else {
			b.reply(msg, "Scheduler resumed.")
		}

	case "stop":
		b.reply(msg, "Stopping daemon...")
		if b.hooks.Stop != nil {
			go b.hooks.Stop()
		}

	case "restart":
		b.reply(msg, "Restarting daemon...")
		if b.hooks.Restart != nil {
			go b.hooks.Restart()
		}

	case "reload":
		if b.hooks.ReloadConfig == nil {
			b.reply(msg, "Reload unavailable.")
			return
		}
		if err := b.hooks.ReloadConfig(); err != nil {
			b.reply(msg, "Failed: "+err.Error())
		} else {
			b.reply(msg, "Config reloaded.")
		}

	case "commits":
		if b.hooks.RecentCommits != nil {
			b.reply(msg, b.hooks.RecentCommits(5))
		} else {
			b.reply(msg, "Commits unavailable.")
		}

	default:
		b.reply(msg, "Unknown command. Use /help.")
	}
}
