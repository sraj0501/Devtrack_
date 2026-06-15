package daemon

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/telegram"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// startTelegramBot starts the interactive Telegram bot if TELEGRAM_ENABLED=true.
// The bot handles inbound commands and outbound push notifications, replacing
// the separate Python telegram bot process.
//
// After the bot starts, it registers its NotifyPendingAction method as the
// queue executor's notification callback so that new low-confidence pending
// actions trigger a Telegram push within the next poll interval.
func (d *Daemon) startTelegramBot() {
	bot := telegram.New(d.buildTelegramHooks())
	if bot == nil {
		log.Println("Telegram bot disabled or not configured (TELEGRAM_ENABLED / TELEGRAM_BOT_TOKEN)")
		return
	}
	if err := bot.Start(); err != nil {
		log.Printf("Warning: Telegram bot failed to start: %v", err)
		return
	}
	d.telegramBot = bot

	// Wire the bot's notification callback into the queue executor so that
	// every new low-confidence pending action triggers a Telegram message.
	if d.monitor != nil {
		d.monitor.SetQueueNotifyFn(bot.NotifyPendingAction)
		log.Println("✓ Telegram queue notifications wired to queue executor")
	}
}

// buildTelegramHooks wires daemon state into the bot without import cycles.
func (d *Daemon) buildTelegramHooks() telegram.Hooks {
	// QueueHooks gives the bot direct access to the pending-actions DB and the
	// HTTP trigger client needed to call POST /queue/execute on approval.
	var queueHooks telegram.QueueHooks
	if d.monitor != nil {
		queueHooks.Database = d.monitor.Database()
		queueHooks.TriggerClient = trigger.NewHTTPTriggerClient()
	}

	return telegram.Hooks{
		Queue: queueHooks,
		StatusText: func() string {
			status, err := d.Status()
			if err != nil || status == nil {
				return "Status unavailable."
			}
			if !status.Running {
				return "*Daemon*: stopped"
			}
			uptime := status.Uptime.Round(time.Second)
			text := fmt.Sprintf("*Daemon*: running\nUptime: %s\nTriggers: %d", uptime, status.TriggerCount)
			if !status.LastTrigger.IsZero() {
				text += fmt.Sprintf("\nLast trigger: %s ago", time.Since(status.LastTrigger).Round(time.Second))
			}
			return text
		},

		LogLines: func(n int) string {
			lines, err := tailFile(d.logFile, n)
			if err != nil {
				return "Could not read log file: " + err.Error()
			}
			if len(lines) == 0 {
				return "Log is empty."
			}
			return "```\n" + strings.Join(lines, "\n") + "\n```"
		},

		HealthText: func() string {
			database, err := db.NewDatabase()
			if err != nil {
				return "DB unavailable."
			}
			defer database.Close()
			snaps, err := database.GetLatestHealthSnapshots()
			if err != nil || len(snaps) == 0 {
				return "No health data."
			}
			var sb strings.Builder
			sb.WriteString("*Health*\n")
			for _, s := range snaps {
				icon := statusIcon(s.Status)
				fmt.Fprintf(&sb, "%s %s: %s\n", icon, s.Service, s.Status)
			}
			return strings.TrimRight(sb.String(), "\n")
		},

		ForceTrigger: func() error {
			if d.monitor == nil || d.monitor.Scheduler() == nil {
				return fmt.Errorf("scheduler not available")
			}
			d.monitor.Scheduler().ForceImmediate()
			return nil
		},

		Pause: func() error { return d.Pause() },
		Resume: func() error { return d.Resume() },

		Stop: func() {
			log.Println("Telegram bot requested daemon stop")
			if d.cancel != nil {
				d.cancel()
			}
		},

		Restart: func() {
			log.Println("Telegram bot requested daemon restart")
			go func() {
				if err := d.Restart(); err != nil {
					log.Printf("Telegram-triggered restart failed: %v", err)
				}
			}()
		},

		ReloadConfig: func() error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("reload failed: %w", err)
			}
			d.config = cfg
			if d.monitor != nil {
				d.monitor.ReloadWorkspaces()
				go func() {
					if err := trigger.NewHTTPTriggerClient().SendWorkspaceReload(); err != nil {
						log.Printf("Could not notify Python of workspace reload: %v", err)
					}
				}()
			}
			log.Println("Config reloaded via Telegram")
			return nil
		},

		RecentCommits: func(n int) string {
			database, err := db.NewDatabase()
			if err != nil {
				return "DB unavailable."
			}
			defer database.Close()
			records, err := database.GetRecentTriggers(n)
			if err != nil || len(records) == 0 {
				return "No recent commits."
			}
			var sb strings.Builder
			sb.WriteString("*Recent commits*\n")
			for _, r := range records {
				if r.TriggerType != "commit" {
					continue
				}
				msg := r.CommitMessage
				if len(msg) > 60 {
					msg = msg[:60] + "..."
				}
				hash := r.CommitHash
				if len(hash) > 7 {
					hash = hash[:7]
				}
				fmt.Fprintf(&sb, "`%s` %s\n", hash, msg)
			}
			result := strings.TrimRight(sb.String(), "\n")
			if result == "*Recent commits*" {
				return "No commit triggers found."
			}
			return result
		},
	}
}

// tailFile returns the last n lines of a file.
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func statusIcon(status string) string {
	switch status {
	case "up":
		return "✓"
	case "down":
		return "✗"
	case "unconfigured":
		return "-"
	default:
		return "?"
	}
}
