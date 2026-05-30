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
	fmt.Println("  • Background daemon + HTTP boundary to AI server")
	fmt.Println("  • SQLite database, ticket cache, alert poller")
	fmt.Println("  • Native Go connectors: GitHub, GitLab, Azure DevOps")
	fmt.Println("  • Notifications: terminal, Telegram, Slack, OS")
	fmt.Println("  • AI features (NLP, LLM, reports) via devtrack_server")
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
	fmt.Println("DevTrack — developer automation: git monitoring, AI commits, PM sync")
	fmt.Println()
	fmt.Println("SETUP:")
	fmt.Println("  devtrack setup         First-run wizard (creates .env + workspaces.yaml)")
	fmt.Println()
	fmt.Println("DAEMON:")
	fmt.Println("  devtrack start         Start the daemon")
	fmt.Println("  devtrack stop          Stop the daemon")
	fmt.Println("  devtrack restart       Restart the daemon")
	fmt.Println("  devtrack status        Show daemon, workspaces, and service status")
	fmt.Println("  devtrack logs          Show recent log entries")
	fmt.Println()
	fmt.Println("SCHEDULER:")
	fmt.Println("  devtrack pause         Pause scheduled triggers (git monitoring continues)")
	fmt.Println("  devtrack resume        Resume scheduler")
	fmt.Println("  devtrack skip-next     Skip the next scheduled trigger")
	fmt.Println("  devtrack force-trigger Fire an immediate trigger")
	fmt.Println("  devtrack reload-config Reload .env + workspaces.yaml without restart")
	fmt.Println()
	fmt.Println("GIT:")
	fmt.Println("  devtrack git commit -m '<msg>'   AI-enhanced commit")
	fmt.Println("    A  Accept  |  E  Enhance further  |  R  Regenerate  |  Q  Defer")
	fmt.Println("    → Ticket picker links commit to an open PM issue")
	fmt.Println("    → 'Log this work?' posts commit as a PM comment")
	fmt.Println("    → 'Push to origin/<branch>?' pushes with one keystroke")
	fmt.Println("  devtrack git history [n]         Last n AI-enhanced commits (default 10)")
	fmt.Println("  devtrack git <cmd>               Pass-through to git (add, push, pull, …)")
	fmt.Println()
	fmt.Println("  devtrack sage ask '<question>'       Ask git-sage a question")
	fmt.Println("  devtrack sage do  '<task>'           git-sage autonomous task execution")
	fmt.Println("  devtrack sage interactive            Interactive git-sage session")
	fmt.Println()
	fmt.Println("SHELL INTEGRATION:")
	fmt.Println(`  eval "$(devtrack shell-init)"    Add to ~/.zshrc / ~/.bashrc (one-time)`)
	fmt.Println("  devtrack enable-git              Opt this repo in  (git config devtrack.enabled)")
	fmt.Println("  devtrack disable-git             Opt this repo out")
	fmt.Println("  GIT_NO_DEVTRACK=1 git commit     Bypass DevTrack for one command")
	fmt.Println()
	fmt.Println("WORKSPACES  (workspaces.yaml is the sole source for paths, org, project, username)")
	fmt.Println("  devtrack workspace list                               List configured workspaces")
	fmt.Println("  devtrack workspace add <name> <path> --pm <platform>  Add a workspace")
	fmt.Println("    --pm: azure | github | gitlab | jira | none")
	fmt.Println("    If <path> is not a git repo you will be offered to initialize it.")
	fmt.Println("  devtrack workspace remove <name>                      Remove a workspace")
	fmt.Println("  devtrack workspace enable|disable <name>              Toggle enabled state")
	fmt.Println("  devtrack workspace reload                             Reload in running daemon")
	fmt.Println("  devtrack workspace install-hooks                      Install post-commit hooks")
	fmt.Println()
	fmt.Println("  workspaces.yaml fields (per workspace):")
	fmt.Println("    pm_platform:        azure | github | gitlab | jira | none")
	fmt.Println("    pm_project:         owner/repo (GitHub) · project ID (GitLab) · project name (Azure)")
	fmt.Println("    pm_org:             Azure org name (e.g. mycompany)")
	fmt.Println("    pm_username:        your login/email for assignee filtering")
	fmt.Println("    pm_api_url:         self-hosted base URL (GitHub Ent / GitLab / ADO Server)")
	fmt.Println("    pm_milestone:       GitHub milestone number or GitLab milestone_id")
	fmt.Println("    pm_iteration_path:  Azure sprint path (e.g. MyProject\\Sprint 5)")
	fmt.Println("    pm_area_path:       Azure area path")
	fmt.Println()
	fmt.Println("  Token/key secrets only (.env):  GITHUB_TOKEN · GITLAB_PAT · AZURE_DEVOPS_PAT")
	fmt.Println()
	fmt.Println("INTEGRATIONS:")
	fmt.Println()
	fmt.Println("  GitHub  (token: GITHUB_TOKEN in .env — all other config in workspaces.yaml)")
	fmt.Println("    devtrack github-check                    Verify connectivity")
	fmt.Println("    devtrack github-list                     Open issues assigned to you")
	fmt.Println("    devtrack github-view <owner/repo> <N>    Issue details")
	fmt.Println("    devtrack github-view <N>                 Issue details (uses pm_project)")
	fmt.Println("    devtrack github-sync                     Sync issues to local SQLite cache")
	fmt.Println()
	fmt.Println("  GitLab  (token: GITLAB_PAT in .env — all other config in workspaces.yaml)")
	fmt.Println("    devtrack gitlab-check                    Verify connectivity")
	fmt.Println("    devtrack gitlab-list                     Open issues assigned to you")
	fmt.Println("    devtrack gitlab-view <project> <iid>     Issue details")
	fmt.Println("    devtrack gitlab-sync                     Sync issues to local SQLite cache")
	fmt.Println()
	fmt.Println("  Azure DevOps  (token: AZURE_DEVOPS_PAT in .env — all other config in workspaces.yaml)")
	fmt.Println("    devtrack azure-check                     Verify connectivity")
	fmt.Println("    devtrack azure-list                      Work items assigned to you")
	fmt.Println("    devtrack azure-view <id>                 Work item details")
	fmt.Println("    devtrack azure-sync                      Sync work items to local SQLite cache")
	fmt.Println()
	fmt.Println("  Jira  (JIRA_API_TOKEN, JIRA_URL, JIRA_EMAIL, JIRA_PROJECT_KEY in .env)")
	fmt.Println("    Handled via the AI server (managed/external mode).")
	fmt.Println()
	fmt.Println("TICKET ALERTS  (background polling for events assigned to you):")
	fmt.Println("  devtrack alerts            Show unread notifications (last 24h)")
	fmt.Println("  devtrack alerts --all      All notifications")
	fmt.Println("  devtrack alerts --clear    Mark all as read")
	fmt.Println("  Configure in .env: ALERT_ENABLED, ALERT_POLL_INTERVAL_SECS")
	fmt.Println()
	fmt.Println("PM AGENT  (requires AI server):")
	fmt.Println("  devtrack plan \"<problem>\"         Epic → Story → Task decomposition + create on PM")
	fmt.Println("  devtrack plan --file <plan.md>    Load from file")
	fmt.Println("  devtrack plan --folder <dir/>     Process all .md files in folder")
	fmt.Println()
	fmt.Println("BOARDROOM  (requires AI server):")
	fmt.Println("  devtrack boardroom \"<problem>\"              7 AI personas review in parallel")
	fmt.Println("  devtrack boardroom --file <plan.md>         Review a plan file")
	fmt.Println("  devtrack boardroom --file <p.md> --output <r.md>  Save report")
	fmt.Println("  devtrack boardroom --interactive            Stay in chat after review")
	fmt.Println("  Personas: Architect · Security · PM · Devil · Engineer · Analyst · Scalability")
	fmt.Println()
	fmt.Println("REPORTS  (requires AI server):")
	fmt.Println("  devtrack preview-report [YYYY-MM-DD]    Preview daily report")
	fmt.Println("  devtrack send-report <email> [date]     Email the report")
	fmt.Println("  devtrack save-report [date]             Save report to file")
	fmt.Println("  devtrack send-summary                   Generate daily summary now")
	fmt.Println()
	fmt.Println("DEFERRED COMMITS:")
	fmt.Println("  devtrack commits pending    List commits queued for AI enhancement")
	fmt.Println("  devtrack commits review     Review and apply enhanced commits")
	fmt.Println()
	fmt.Println("PERSONALIZED AI  (requires AI server):")
	fmt.Println("  devtrack enable-learning [days]    Enable learning from communications")
	fmt.Println("  devtrack learning-sync             Delta sync (new messages only)")
	fmt.Println("  devtrack learning-sync --full      Force full re-sync")
	fmt.Println("  devtrack learning-status           Learning status and sample count")
	fmt.Println("  devtrack learning-reset            Wipe all learning data")
	fmt.Println("  devtrack show-profile              Learned communication profile")
	fmt.Println("  devtrack test-response <text>      Test a personalized response")
	fmt.Println("  devtrack revoke-consent            Revoke consent and delete data")
	fmt.Println("  devtrack learning-setup-cron       Install daily sync cron")
	fmt.Println("  devtrack learning-remove-cron      Remove sync cron")
	fmt.Println("  devtrack learning-cron-status      Show cron status")
	fmt.Println()
	fmt.Println("ACCOUNT  (requires AI server):")
	fmt.Println("  devtrack login             Authenticate with DevTrack cloud")
	fmt.Println("  devtrack logout            Clear session")
	fmt.Println("  devtrack whoami            Show current session")
	fmt.Println("  devtrack license           Show license status and tier")
	fmt.Println("  devtrack terms             Show Terms of Service")
	fmt.Println("  devtrack terms --accept    Accept Terms of Service")
	fmt.Println("  devtrack telemetry [on|off|status]  Manage telemetry")
	fmt.Println()
	fmt.Println("AUTO-START:")
	fmt.Println("  devtrack autostart-install      macOS launchd / Linux systemd / WSL")
	fmt.Println("  devtrack autostart-uninstall    Remove auto-start")
	fmt.Println("  devtrack autostart-status       Show auto-start status")
	fmt.Println()
	fmt.Println("INFO:")
	fmt.Println("  devtrack status        Daemon, workspaces, and services")
	fmt.Println("  devtrack version       Version and build info")
	fmt.Println("  devtrack settings      Config paths and key env settings")
	fmt.Println("  devtrack db-stats      Database statistics and recent activity")
	fmt.Println("  devtrack help          This message")
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
