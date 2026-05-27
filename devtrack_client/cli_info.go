package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// handleLogs displays recent log entries
func (cli *CLI) handleLogs() error {
	lines := 50 // Default: last 50 lines

	if len(os.Args) > 2 {
		if os.Args[2] == "-f" || os.Args[2] == "--follow" {
			fmt.Println("❌ Follow mode not yet implemented")
			fmt.Printf("Use: tail -f %s\n", GetLogFilePath())
			return nil
		}
	}

	logs, err := cli.daemon.GetLogs(lines)
	if err != nil {
		fmt.Printf("❌ Failed to read logs: %v\n", err)
		return err
	}

	if len(logs) == 0 {
		fmt.Println("No logs available")
		return nil
	}

	fmt.Printf("📄 Last %d log entries:\n", len(logs))
	fmt.Println("════════════════════════")
	for _, line := range logs {
		fmt.Println(line)
	}

	return nil
}

// handleVersion shows version information
func (cli *CLI) handleVersion() error {
	fmt.Println("DevTrack - Developer Automation Tools")
	fmt.Printf("Version:    %s\n", Version)
	fmt.Printf("Commit:     %s\n", GitCommit)
	fmt.Printf("Built:      %s\n", BuildTime)
	fmt.Println()
	fmt.Println("Components:")
	fmt.Println("  • Git monitoring (go-git)")
	fmt.Println("  • Time-based scheduler (robfig/cron)")
	fmt.Println("  • Background daemon + Python bridge")
	fmt.Println("  • IPC communication, SQLite database")
	fmt.Println("  • NLP task parsing (spaCy)")
	fmt.Println("  • Task matching, email reports")
	fmt.Println("  • AI-enhanced daily reports (Ollama)")
	return nil
}

// handleDBStats shows database statistics
func (cli *CLI) handleDBStats() error {
	fmt.Println("📊 Database Statistics")
	fmt.Println("=" + strings.Repeat("=", 50))
	fmt.Println()

	// Open database
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get statistics
	stats, err := db.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get database stats: %w", err)
	}

	// Get analytics
	analytics, _ := db.GetAnalytics()

	// Display stats
	fmt.Printf("Database Path:    %s\n", stats["database_path"])
	fmt.Println()
	fmt.Printf("Total Triggers:   %d\n", stats["triggers"])
	if today, ok := analytics["triggers_today"].(int); ok {
		fmt.Printf("  Today:          %d\n", today)
	}
	if week, ok := analytics["triggers_this_week"].(int); ok {
		fmt.Printf("  This week:      %d\n", week)
	}
	fmt.Printf("Total Responses:  %d\n", stats["responses"])
	fmt.Printf("Task Updates:     %d\n", stats["task_updates"])
	fmt.Printf("Unsynced Updates: %d\n", stats["unsynced_updates"])
	fmt.Printf("Log Entries:      %d\n", stats["logs"])
	if top, ok := analytics["top_projects"].([]map[string]interface{}); ok && len(top) > 0 {
		fmt.Println()
		fmt.Println("Top Projects (last 30 days):")
		fmt.Println("─" + strings.Repeat("─", 50))
		for i, p := range top {
			if proj, ok := p["project"].(string); ok {
				switch cnt := p["count"].(type) {
				case int:
					fmt.Printf("  %d. %s (%d updates)\n", i+1, proj, cnt)
				case int64:
					fmt.Printf("  %d. %s (%d updates)\n", i+1, proj, cnt)
				}
			}
		}
	}
	fmt.Println()

	// Get recent triggers
	triggers, err := db.GetRecentTriggers(5)
	if err == nil && len(triggers) > 0 {
		fmt.Println("Recent Triggers (last 5):")
		fmt.Println("─" + strings.Repeat("─", 50))
		for i, t := range triggers {
			fmt.Printf("%d. [%s] %s at %s\n",
				i+1,
				t.TriggerType,
				t.Source,
				t.Timestamp.Format("2006-01-02 15:04:05"))
			if t.CommitMessage != "" {
				fmt.Printf("   %s\n", t.CommitMessage)
			}
		}
		fmt.Println()
	}

	// Get unsynced updates
	unsynced, err := db.GetUnsyncedTaskUpdates()
	if err == nil && len(unsynced) > 0 {
		fmt.Println("Unsynced Task Updates:")
		fmt.Println("─" + strings.Repeat("─", 50))
		for i, u := range unsynced {
			fmt.Printf("%d. [%s] %s - %s\n",
				i+1,
				u.Project,
				u.TicketID,
				u.Status)
			if u.UpdateText != "" {
				fmt.Printf("   %s\n", u.UpdateText)
			}
		}
		fmt.Println()
	}

	return nil
}

// handleSettings shows all configuration paths and key env settings
func (cli *CLI) handleSettings() error {
	LoadEnvConfig()

	fmt.Println("DevTrack Settings")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Println()

	fmt.Println("Files & Paths:")
	fmt.Printf("  Config:      %s\n", GetConfigPath())
	fmt.Printf("  Log file:    %s\n", GetLogFilePath())
	fmt.Printf("  PID file:    %s\n", GetPIDFilePath())
	fmt.Printf("  Database:    %s\n", GetDatabasePath())
	fmt.Println()

	fmt.Println("IPC:")
	fmt.Printf("  Host:        %s\n", getEnvOrDefault("IPC_HOST", "127.0.0.1"))
	fmt.Printf("  Port:        %s\n", getEnvOrDefault("IPC_PORT", "35893"))
	fmt.Println()

	fmt.Println("Azure DevOps:")
	fmt.Printf("  Org:         %s\n", maskEmpty(os.Getenv("AZURE_ORGANIZATION")))
	fmt.Printf("  Project:     %s\n", maskEmpty(os.Getenv("AZURE_PROJECT")))
	pat := os.Getenv("AZURE_DEVOPS_PAT")
	if pat == "" {
		pat = os.Getenv("AZURE_API_KEY")
	}
	fmt.Printf("  PAT:         %s\n", maskSecret(pat))
	fmt.Printf("  Sync:        %s\n", getEnvOrDefault("AZURE_SYNC_ENABLED", "false"))
	fmt.Println()

	fmt.Println("LLM:")
	fmt.Printf("  Provider:    %s\n", getEnvOrDefault("LLM_PROVIDER", "ollama"))
	fmt.Printf("  Ollama host: %s\n", getEnvOrDefault("OLLAMA_HOST", "(not set)"))
	fmt.Printf("  Sage model:  %s\n", getEnvOrDefault("GIT_SAGE_DEFAULT_MODEL", "(not set)"))
	fmt.Println()

	fmt.Println("Telegram:")
	enabled := os.Getenv("TELEGRAM_ENABLED")
	if enabled == "" {
		enabled = "false"
	}
	fmt.Printf("  Enabled:     %s\n", enabled)
	if enabled == "true" {
		fmt.Printf("  Bot token:   %s\n", maskSecret(os.Getenv("TELEGRAM_BOT_TOKEN")))
		fmt.Printf("  Allowed IDs: %s\n", maskEmpty(os.Getenv("TELEGRAM_ALLOWED_CHAT_IDS")))
	}
	fmt.Println()

	fmt.Println("Webhook:")
	webhookEnabled := getEnvOrDefault("WEBHOOK_ENABLED", "false")
	fmt.Printf("  Enabled:     %s\n", webhookEnabled)
	if webhookEnabled == "true" {
		fmt.Printf("  Listen:      %s:%s\n",
			getEnvOrDefault("WEBHOOK_HOST", "0.0.0.0"),
			getEnvOrDefault("WEBHOOK_PORT", "8089"))
	}
	fmt.Println()

	return nil
}

// printUsage prints CLI usage information
func (cli *CLI) printUsage() {
	fmt.Println("DevTrack - Developer Automation Tools")
	fmt.Println("======================================")
	fmt.Println()
	fmt.Println("DAEMON COMMANDS:")
	fmt.Println("  devtrack start         Start the daemon")
	fmt.Println("  devtrack stop          Stop the daemon")
	fmt.Println("  devtrack restart       Restart the daemon")
	fmt.Println("  devtrack status        Show daemon status")
	fmt.Println()
	fmt.Println("SCHEDULER COMMANDS:")
	fmt.Println("  devtrack pause         Pause scheduler (keep git monitoring)")
	fmt.Println("  devtrack resume        Resume scheduler")
	fmt.Println("  devtrack force-trigger Force immediate trigger")
	fmt.Println("  devtrack reload-config Reload .env + YAML config without restart")
	fmt.Println("  devtrack skip-next     Skip the next scheduled trigger")
	fmt.Println("  devtrack send-summary  Generate daily summary now")
	fmt.Println()
	fmt.Println("GIT COMMANDS:")
	fmt.Println("  devtrack git commit -m 'msg'   AI-enhanced commit with iterative refinement")
	fmt.Println("                                   A  Accept and commit")
	fmt.Println("                                   E  Enhance further  (2× token budget for richer output)")
	fmt.Println("                                   R  Regenerate from scratch")
	fmt.Println("                                   Q  Queue for later AI enhancement")
	fmt.Println("                                   → Ticket picker links commit to an open issue")
	fmt.Println("                                   → 'Log this work?' syncs commit to PM as a comment")
	fmt.Println("                                   → 'Push to origin/<branch>?' pushes with one keystroke")
	fmt.Println("  devtrack git history [n]        Show last n AI-enhanced commits (default 10)")
	fmt.Println("  devtrack git messages [n]       Alias for history")
	fmt.Println("  devtrack sage ask '<question>'             Ask git-sage a question")
	fmt.Println("  devtrack sage do  '<task>' [--verbose]     Let git-sage execute a task")
	fmt.Println("  devtrack sage interactive                  Interactive git-sage session")
	fmt.Println()
	fmt.Println("SHELL INTEGRATION (intercepts git commands for DevTrack workspaces):")
	fmt.Println(`  eval "$(devtrack shell-init)"  # Add to ~/.zshrc or ~/.bashrc — one-time setup`)
	fmt.Println("  devtrack enable-git            Opt this repo in  (sets git config devtrack.enabled)")
	fmt.Println("  devtrack disable-git           Opt this repo out (explicit override for workspaces.yaml repos)")
	fmt.Println("  devtrack is-workspace          Exit 0 if CWD is a DevTrack workspace (used internally)")
	fmt.Println("  GIT_NO_DEVTRACK=1 git commit   Bypass DevTrack for a single command")
	fmt.Println()
	fmt.Println("  Intercepted commands (when enabled):")
	fmt.Println("    git add              No args → git add .  (stage everything); paths work as normal")
	fmt.Println("    git commit           AI-enhanced commit flow")
	fmt.Println("    git history/messages Show AI-enhanced commit log")
	fmt.Println("    git push/pull/status Pass through to real git unchanged")
	fmt.Println()
	fmt.Println("INTEGRATIONS:")
	fmt.Println()
	fmt.Println("  GitHub:")
	fmt.Println("    devtrack github-check                    Check config and connectivity")
	fmt.Println("    devtrack github-list                     List open issues assigned to you")
	fmt.Println("    devtrack github-list --closed            Include closed issues")
	fmt.Println("    devtrack github-list --state <state>     Filter: open | closed | all")
	fmt.Println("    devtrack github-view <number>            Full details for issue #N")
	fmt.Println("    devtrack github-view <owner/repo> <N>    Full details across a specific repo")
	fmt.Println("    devtrack github-sync                     Resync all issues to local cache")
	fmt.Println("    devtrack github-sync --full              Force full resync (ignore delta)")
	fmt.Println("    devtrack github-sync --hours 24          Only issues updated in last 24h")
	fmt.Println("    Configure: GITHUB_TOKEN, GITHUB_OWNER, GITHUB_REPO in .env")
	fmt.Println()
	fmt.Println("  GitLab:")
	fmt.Println("    devtrack gitlab-check                    Check config and connectivity")
	fmt.Println("    devtrack gitlab-list                     List open issues assigned to you")
	fmt.Println("    devtrack gitlab-list --closed            Include closed issues")
	fmt.Println("    devtrack gitlab-list --state <state>     Filter: opened | closed")
	fmt.Println("    devtrack gitlab-view <project_path> <iid>  Full details for an issue")
	fmt.Println("    devtrack gitlab-sync                     Resync all open issues to cache")
	fmt.Println("    devtrack gitlab-sync --full              Force full resync")
	fmt.Println("    devtrack gitlab-sync --hours 24          Only issues updated in last 24h")
	fmt.Println("    Configure: GITLAB_URL, GITLAB_PAT, GITLAB_PROJECT_ID in .env")
	fmt.Println()
	fmt.Println("  Azure DevOps:")
	fmt.Println("    devtrack azure-check                     Check config and connectivity")
	fmt.Println("    devtrack azure-list                      List work items assigned to you")
	fmt.Println("    devtrack azure-list --all                All work items (no state filter)")
	fmt.Println("    devtrack azure-list --state <states>     Filter by state (e.g. 'Active,New')")
	fmt.Println("    devtrack azure-view <id>                 Full details for a work item")
	fmt.Println("    devtrack azure-sync                      Resync all work items to local cache")
	fmt.Println("    devtrack azure-sync --full               Force full resync")
	fmt.Println("    devtrack azure-sync --hours 24           Only items changed in last 24h")
	fmt.Println("    Configure: AZURE_DEVOPS_PAT, AZURE_ORGANIZATION, AZURE_PROJECT in .env")
	fmt.Println()
	fmt.Println("  Jira: configure JIRA_API_TOKEN, JIRA_URL, JIRA_EMAIL, JIRA_PROJECT_KEY in .env")
	fmt.Println("        Jira CLI commands are handled via the Python server (managed/external mode).")
	fmt.Println()
	fmt.Println("TICKET ALERTS (background polling — events relevant to you):")
	fmt.Println("  devtrack alerts          Show unread notifications (last 24h)")
	fmt.Println("  devtrack alerts --all    Show all notifications (no time filter)")
	fmt.Println("  devtrack alerts --clear  Mark all as read")
	fmt.Println("  Configure: ALERT_ENABLED, ALERT_POLL_INTERVAL_SECS")
	fmt.Println("             ALERT_GITHUB_ENABLED, ALERT_AZURE_ENABLED, ALERT_JIRA_ENABLED")
	fmt.Println("             ALERT_NOTIFY_ASSIGNED, ALERT_NOTIFY_COMMENTS,")
	fmt.Println("             ALERT_NOTIFY_STATUS_CHANGES, ALERT_NOTIFY_REVIEW_REQUESTED")
	fmt.Println()
	fmt.Println("PM AGENT:")
	fmt.Println("  devtrack plan \"<problem>\"       Decompose a problem into Epic → Story → Task hierarchy")
	fmt.Println("  devtrack plan --file <plan.md>  Load a structured plan file")
	fmt.Println("  devtrack plan --folder <dir/>   Process all .md plan files in a folder")
	fmt.Println("                                  Platform picker → LLM preview → confirm to create items")
	fmt.Println("                                  Supported platforms: azure, gitlab, github")
	fmt.Println("  Also available via Telegram bot: /plan <problem>")
	fmt.Println()
	fmt.Println("BOARDROOM (multi-persona AI plan review):")
	fmt.Println("  devtrack boardroom \"<problem>\"               7 AI personas review the plan in parallel")
	fmt.Println("  devtrack boardroom --file <plan.md>          Review a structured plan file")
	fmt.Println("  devtrack boardroom --folder <dir/>           Review all .md plans in folder")
	fmt.Println("  devtrack boardroom --file <p.md> --output <r.md>  Save report as markdown")
	fmt.Println("  Personas: Architect · Security · PM · Devil's Advocate · Engineer · Analyst · Scalability")
	fmt.Println("  Output:   PROs/CONs · SWOT matrix · Implementation approach · PROCEED/REVISE/RECONSIDER")
	fmt.Println()
	fmt.Println("OFFLINE RESILIENCE:")
	fmt.Println("  devtrack queue             Show message queue stats")
	fmt.Println("  devtrack commits pending   List deferred commits and status")
	fmt.Println("  devtrack commits review    Review enhanced deferred commits")
	fmt.Println()
	fmt.Println("EMAIL REPORTS:")
	fmt.Println("  devtrack preview-report [date]   Preview today's report (or YYYY-MM-DD)")
	fmt.Println("  devtrack send-report <email>     Send daily report to email address")
	fmt.Println("  devtrack save-report [date]      Save report to file")
	fmt.Println()
	fmt.Println("PERSONALIZED AI LEARNING:")
	fmt.Println("  devtrack enable-learning [days]  Enable learning from communications (default 30 days)")
	fmt.Println("  devtrack learning-sync           Run delta sync (only new messages since last run)")
	fmt.Println("  devtrack learning-sync --full    Force full re-sync (ignore delta state)")
	fmt.Println("  devtrack learning-status         Show learning status and statistics")
	fmt.Println("  devtrack learning-reset          Wipe all learning data and start fresh")
	fmt.Println("  devtrack show-profile            Show learned communication profile")
	fmt.Println("  devtrack test-response <text>    Test generating a personalized response")
	fmt.Println("  devtrack revoke-consent          Revoke learning consent and delete data")
	fmt.Println()
	fmt.Println("LEARNING CRON (configure LEARNING_CRON_SCHEDULE in .env):")
	fmt.Println("  devtrack learning-setup-cron     Install/update daily cron entry from .env")
	fmt.Println("  devtrack learning-cron-status    Show cron entry and .env schedule settings")
	fmt.Println("  devtrack learning-remove-cron    Remove the cron entry")
	fmt.Println()
	fmt.Println("TELEGRAM:")
	fmt.Println("  devtrack telegram-status  Show Telegram bot status")
	fmt.Println("  Bot commands: /status /azure /azureissue")
	fmt.Println("                /gitlab /gitlabissue")
	fmt.Println("                /plan <problem>  (PM Agent: decompose + create work items)")
	fmt.Println()
	fmt.Println("WORKSPACE (MULTI-REPO):")
	fmt.Println("  devtrack workspace list                          List configured workspaces")
	fmt.Println("  devtrack workspace add <name> <path> [platform]  Add a workspace to workspaces.yaml")
	fmt.Println("  devtrack workspace remove <name>                 Remove a workspace")
	fmt.Println("  devtrack workspace enable <name>                 Enable a workspace")
	fmt.Println("  devtrack workspace disable <name>                Disable a workspace")
	fmt.Println("  devtrack workspace reload                         Signal daemon to reload workspaces.yaml")
	fmt.Println()
	fmt.Println("AUTO-START (OS-aware):")
	fmt.Println("  devtrack autostart-install    Install auto-start for this OS and enable it")
	fmt.Println("                                  macOS        → launchd LaunchAgent (~/Library/LaunchAgents)")
	fmt.Println("                                  Linux/systemd → systemd user service (~/.config/systemd/user)")
	fmt.Println("                                  WSL/systemd  → systemd user service (~/.config/systemd/user)")
	fmt.Println("                                  WSL (no systemd) → shell profile block (~/.zshrc / ~/.bashrc)")
	fmt.Println("  devtrack autostart-uninstall  Remove auto-start for this OS")
	fmt.Println("  devtrack autostart-status     Show auto-start status for this OS")
	fmt.Println()
	fmt.Println("MACOS AUTO-START (launchd, legacy aliases):")
	fmt.Println("  devtrack launchd-install    Install plist to ~/Library/LaunchAgents and load it")
	fmt.Println("                              DevTrack will start automatically at login")
	fmt.Println("  devtrack launchd-uninstall  Unload and remove the launchd plist")
	fmt.Println()
	fmt.Println("SERVER TUI / ADMIN CONSOLE (CS-2 / CS-3):")
	fmt.Println("  devtrack server-tui    Open Textual process monitor for all DevTrack server processes")
	fmt.Println("                           ↑↓ / j k  navigate   r restart   s start   x stop")
	fmt.Println("                           l         toggle log pane for selected process")
	fmt.Println("                           q         quit")
	fmt.Println("  devtrack admin-start   Start Admin Console web UI  (default: http://localhost:8090/admin/)")
	fmt.Println("                           ADMIN_PORT, ADMIN_USERNAME, ADMIN_PASSWORD must be set in .env")
	fmt.Println()
	fmt.Println("INFO COMMANDS:")
	fmt.Println("  devtrack logs          Show recent log entries")
	fmt.Println("  devtrack db-stats      Show database statistics")
	fmt.Println("  devtrack stats         Alias for db-stats (with analytics)")
	fmt.Println("  devtrack version       Show version information")
	fmt.Println("  devtrack help          Show this help message")
	fmt.Println("  devtrack settings      Show configuration paths and key env settings")
	fmt.Println()
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func maskEmpty(v string) string {
	if v == "" {
		return "(not set)"
	}
	return v
}

func maskSecret(v string) string {
	if v == "" {
		return "(not set)"
	}
	if len(v) <= 8 {
		return "****"
	}
	return v[:4] + strings.Repeat("*", len(v)-8) + v[len(v)-4:]
}
