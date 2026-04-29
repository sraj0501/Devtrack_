package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// CLI provides command-line interface for daemon management
type CLI struct {
	daemon *Daemon
}

// NewCLI creates a new CLI instance
func NewCLI() (*CLI, error) {
	// For status/help commands, we don't need a full daemon
	if len(os.Args) > 1 {
		cmd := os.Args[1]
		if cmd == "help" || cmd == "version" || cmd == "commit-queue" || cmd == "commits" || cmd == "queue" || cmd == "telegram-status" || cmd == "azure-check" || cmd == "gitlab-check" || cmd == "github-check" || cmd == "workspace" || cmd == "shell-init" || cmd == "is-workspace" || cmd == "enable-git" || cmd == "disable-git" || cmd == "launchd-install" || cmd == "launchd-uninstall" || cmd == "autostart-install" || cmd == "autostart-uninstall" || cmd == "autostart-status" || cmd == "alerts" || cmd == "cloud" || cmd == "tui" ||
			cmd == "login" || cmd == "logout" || cmd == "whoami" || cmd == "license" || cmd == "terms" || cmd == "telemetry" {
			return &CLI{}, nil
		}
	}

	repoPath, err := resolveRepoPath()
	if err != nil {
		// For status command, still allow but with limited info
		if len(os.Args) > 1 && os.Args[1] == "status" {
			return &CLI{}, nil
		}
		return nil, err
	}

	daemon, err := NewDaemon(repoPath)
	if err != nil {
		return nil, err
	}

	return &CLI{daemon: daemon}, nil
}

func resolveRepoPath() (string, error) {
	wsCfg, err := LoadWorkspacesConfig()
	if err == nil && wsCfg != nil && len(wsCfg.GetEnabledWorkspaces()) > 0 {
		return "", nil
	}

	workspacePath := strings.TrimSpace(os.Getenv("DEVTRACK_WORKSPACE"))
	if workspacePath != "" {
		workspacePath = filepath.Clean(workspacePath)
		if IsGitRepository(workspacePath) {
			return workspacePath, nil
		}

		parentPath := filepath.Dir(workspacePath)
		if IsGitRepository(parentPath) {
			return parentPath, nil
		}

		return "", fmt.Errorf("DEVTRACK_WORKSPACE is not a git repository: %s", workspacePath)
	}

	repoPath, err := os.Getwd()
	if err != nil {
		repoPath = "."
	}

	if IsGitRepository(repoPath) {
		return repoPath, nil
	}

	parentPath := filepath.Dir(repoPath)
	if IsGitRepository(parentPath) {
		return parentPath, nil
	}

	return "", fmt.Errorf("not in a git repository and DEVTRACK_WORKSPACE is not set")
}

// Execute runs the CLI command
func (cli *CLI) Execute() error {
	if len(os.Args) < 2 {
		cli.printUsage()
		return nil
	}

	command := os.Args[1]

	switch command {
	case "start":
		return cli.handleStart()
	case "stop":
		return cli.handleStop()
	case "restart":
		return cli.handleRestart()
	case "status":
		return cli.handleStatus()
	case "pause":
		return cli.handlePause()
	case "resume":
		return cli.handleResume()
	case "logs":
		return cli.handleLogs()
	case "db-stats", "stats":
		return cli.handleDBStats()
	case "enable-learning":
		return cli.handleEnableLearning()
	case "show-profile":
		return cli.handleShowProfile()
	case "test-response":
		return cli.handleTestResponse()
	case "revoke-consent":
		return cli.handleRevokeConsent()
	case "learning-status":
		return cli.handleLearningStatus()
	case "learning-setup-cron":
		return cli.handleLearningSetupCron()
	case "learning-remove-cron":
		return cli.handleLearningRemoveCron()
	case "learning-cron-status":
		return cli.handleLearningCronStatus()
	case "learning-sync":
		return cli.handleLearningSync()
	case "learning-reset":
		return cli.handleLearningReset()
	case "preview-report":
		return cli.handlePreviewReport()
	case "send-report":
		return cli.handleSendReport()
	case "save-report":
		return cli.handleSaveReport()
	case "force-trigger":
		return cli.handleForceTrigger()
	case "send-summary":
		return cli.handleSendSummary()
	case "skip-next":
		return cli.handleSkipNext()
	case "version":
		return cli.handleVersion()
	case "commit-queue":
		return cli.handleCommitQueue()
	case "commits":
		return cli.handleCommits()
	case "queue":
		return cli.handleQueueStats()
	case "telegram-status":
		return cli.handleTelegramStatus()
	case "azure-check":
		return cli.handleAzureCheck()
	case "azure-list":
		return cli.handleAzureList()
	case "azure-sync":
		return cli.handleAzureSync()
	case "azure-view":
		return cli.handleAzureView()
	case "gitlab-check":
		return cli.handleGitLabCheck()
	case "gitlab-list":
		return cli.handleGitLabList()
	case "gitlab-sync":
		return cli.handleGitLabSync()
	case "gitlab-view":
		return cli.handleGitLabView()
	case "github-check":
		return cli.handleGitHubCheck()
	case "github-list":
		return cli.handleGitHubList()
	case "github-sync":
		return cli.handleGitHubSync()
	case "github-view":
		return cli.handleGitHubView()
	case "settings":
		return cli.handleSettings()
	case "workspace":
		return cli.handleWorkspace()
	case "shell-init":
		return cli.handleShellInit()
	case "is-workspace":
		return cli.handleIsWorkspace()
	case "enable-git":
		return cli.handleEnableGit()
	case "disable-git":
		return cli.handleDisableGit()
	case "launchd-install":
		return cli.handleLaunchdInstall()
	case "launchd-uninstall":
		return cli.handleLaunchdUninstall()
	case "autostart-install":
		return cli.handleAutostartInstall()
	case "autostart-uninstall":
		return cli.handleAutostartUninstall()
	case "autostart-status":
		return cli.handleAutostartStatus()
	case "alerts":
		return cli.handleAlerts()
	case "vacation":
		return cli.handleVacation()
	case "work":
		return cli.handleWork()
	case "server-tui":
		return cli.handleServerTUI()
	case "admin-start":
		return cli.handleAdminStart()
	case "cloud":
		return cli.handleCloud()
	case "tui":
		return cli.handleTUI()
	case "login":
		return cli.handleLogin()
	case "logout":
		return cli.handleLogout()
	case "whoami":
		return cli.handleWhoami()
	case "license":
		return cli.handleLicense()
	case "terms":
		return cli.handleTerms()
	case "telemetry":
		return cli.handleTelemetry()
	case "help":
		cli.printUsage()
		return nil
	default:
		// Check if it's a test command
		if strings.HasPrefix(command, "test-") {
			return nil // Let main handle test commands
		}
		fmt.Printf("Unknown command: %s\n\n", command)
		cli.printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

// requiresManagedMode prints a clear error and returns an error when the
// current server mode does not include a Python backend.
func requiresManagedMode(command string) error {
	if IsLightweightMode() {
		fmt.Printf("'%s' requires Managed mode (Python backend).\n", command)
		fmt.Println("Re-run 'devtrack setup' and choose [1] Managed to enable AI features.")
		return fmt.Errorf("command unavailable in Lightweight mode")
	}
	return nil
}

// handleStart starts the daemon
func (cli *CLI) handleStart() error {
	if cli.daemon.IsRunning() {
		pid, _ := cli.daemon.readPID()
		fmt.Printf("❌ Daemon is already running (PID: %d)\n", pid)
		fmt.Println("\nUse 'devtrack status' to see details")
		fmt.Println("Use 'devtrack restart' to restart")
		return nil
	}

	// First-run: ensure Terms of Service are accepted before starting
	// Only check in the parent process (not in the forked daemon child)
	if os.Getenv("DEVTRACK_DAEMON") != "1" {
		projectRoot := resolveProjectRoot()
		if !EnsureTermsAccepted(projectRoot) {
			fmt.Println("\nDevTrack requires acceptance of the Terms of Service to start.")
			fmt.Println("Run 'devtrack terms' to read them, then 'devtrack terms --accept'.")
			return fmt.Errorf("terms not accepted")
		}
	}

	// Check if we're already daemonized (child process)
	if os.Getenv("DEVTRACK_DAEMON") == "1" {
		// We are the daemon process - run it
		if err := cli.daemon.Start(); err != nil {
			fmt.Printf("❌ Failed to start daemon: %v\n", err)
			return err
		}
		return nil
	}

	// Parent process - fork to background
	fmt.Println("🚀 Starting DevTrack daemon...")

	// Get current executable path
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Start ourselves as a background daemon
	cmd := exec.Command(exe, "start")
	cmd.Env = append(os.Environ(), "DEVTRACK_DAEMON=1")

	// Redirect output to log file
	logPath := GetLogFilePath()
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Detach from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Parent exits immediately
	fmt.Println("✓ Daemon started successfully")
	fmt.Printf("   PID: %d\n", cmd.Process.Pid)
	fmt.Printf("   Log: %s\n", logPath)
	fmt.Println("\nUse 'devtrack status' to check status")

	enableGitForWorkspaces()
	SendActivePingIfDue()

	return nil
}

// handleStop stops the daemon
func (cli *CLI) handleStop() error {
	fmt.Println("⏹️  Stopping DevTrack daemon...")

	if !cli.daemon.IsRunning() {
		fmt.Println("❌ Daemon is not running")
		return nil
	}

	// Try graceful stop first
	pidFile := GetPIDFilePath()

	if err := KillDaemon(pidFile); err != nil {
		fmt.Printf("❌ Failed to stop daemon: %v\n", err)
		return err
	}

	fmt.Println("✓ Daemon stopped successfully")
	return nil
}

// handleRestart restarts the daemon
func (cli *CLI) handleRestart() error {
	fmt.Println("🔄 Restarting DevTrack daemon...")

	// Stop if running
	if cli.daemon.IsRunning() {
		fmt.Println("Stopping current instance...")
		if err := cli.handleStop(); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}

	// Start again
	return cli.handleStart()
}

// handleStatus shows daemon status with full health dashboard
func (cli *CLI) handleStatus() error {
	// Handle case where daemon is nil (status check without repo)
	if cli.daemon == nil {
		pidFile := GetPIDFilePath()
		data, err := os.ReadFile(pidFile)
		if err != nil {
			fmt.Println("DevTrack Daemon        ● Stopped")
			return nil
		}

		fmt.Printf("DevTrack Daemon        ● Running (PID: %s)\n", strings.TrimSpace(string(data)))
		fmt.Println()
		fmt.Println("Use 'devtrack status' from repository directory for full details")
		return nil
	}

	status, err := cli.daemon.Status()
	if err != nil {
		fmt.Printf("Failed to get status: %v\n", err)
		return err
	}

	// Header
	if status.Running {
		uptime := ""
		if !status.StartTime.IsZero() {
			uptime = fmt.Sprintf(", uptime %s", formatDuration(status.Uptime))
		}
		fmt.Printf("DevTrack Daemon        ● Running (PID %d%s)\n", status.PID, uptime)
	} else {
		fmt.Println("DevTrack Daemon        ● Stopped")
	}
	fmt.Println()

	// Services section (from health snapshots)
	db, dbErr := NewDatabase()
	if dbErr == nil {
		defer db.Close()

		snapshots, err := db.GetLatestHealthSnapshots()
		if err == nil && len(snapshots) > 0 {
			fmt.Println("Services:")
			for _, snap := range snapshots {
				icon := "●"
				switch snap.Status {
				case "up":
					icon = "\033[32m●\033[0m" // green
				case "down":
					icon = "\033[31m●\033[0m" // red
				case "degraded":
					icon = "\033[33m●\033[0m" // yellow
				case "unconfigured":
					icon = "\033[90m○\033[0m" // gray
				}

				detail := formatHealthDetail(snap)
				fmt.Printf("  %-22s %s %s\n", serviceDisplayName(snap.Service), icon, detail)
			}
			fmt.Println()
		}

		// Queue stats
		pending, failed, sent, err := db.GetMessageQueueStats()
		if err == nil {
			fmt.Printf("Sync Queue:            %d pending, %d failed", pending, failed)
			if sent > 0 {
				fmt.Printf(" (%d sent)", sent)
			}
			fmt.Println()
		}

		// Deferred commit stats
		dcPending, dcEnhanced, dcCommitted, dcExpired, err := db.GetDeferredCommitStats()
		if err == nil && (dcPending+dcEnhanced+dcCommitted+dcExpired) > 0 {
			parts := []string{}
			if dcEnhanced > 0 {
				parts = append(parts, fmt.Sprintf("%d enhanced (ready for review)", dcEnhanced))
			}
			if dcPending > 0 {
				parts = append(parts, fmt.Sprintf("%d pending", dcPending))
			}
			if dcCommitted > 0 {
				parts = append(parts, fmt.Sprintf("%d committed", dcCommitted))
			}
			fmt.Printf("Deferred Commits:      %s\n", strings.Join(parts, ", "))
		}
		fmt.Println()
	}

	if status.Running {
		// Scheduler info
		if cli.daemon.monitor != nil && cli.daemon.monitor.scheduler != nil {
			stats := cli.daemon.monitor.scheduler.GetStats()
			workStatus := cli.daemon.monitor.scheduler.GetWorkHoursStatus()

			interval := stats["interval_minutes"]
			nextTrigger := stats["time_until_next"]
			paused := stats["is_paused"]

			schedLine := fmt.Sprintf("every %vm", interval)
			if p, ok := paused.(bool); ok && p {
				schedLine += " (PAUSED)"
			}
			if d, ok := nextTrigger.(time.Duration); ok && d > 0 {
				schedLine += fmt.Sprintf(", next in %s", formatDuration(d))
			}
			fmt.Printf("Scheduler:             %s\n", schedLine)

			if enabled, ok := workStatus["enabled"].(bool); ok && enabled {
				inHours := workStatus["is_work_hours"]
				startH := workStatus["work_start_hour"]
				endH := workStatus["work_end_hour"]
				hoursStr := "outside"
				if ih, ok := inHours.(bool); ok && ih {
					hoursStr = "active"
				}
				fmt.Printf("Work Hours:            %v:00-%v:00 (%s)\n", startH, endH, hoursStr)
			}
		}

	}

	return nil
}

// serviceDisplayName returns a human-friendly name for a service
func serviceDisplayName(service string) string {
	switch service {
	case "ipc":
		return "Python IPC"
	case "python_bridge":
		return "Python Bridge"
	case "ollama":
		return "Ollama"
	case "azure_devops":
		return "Azure DevOps"
	case "webhook_server":
		return "Webhook Server"
	case "telegram_bot":
		return "Telegram Bot"
	case "mongodb":
		return "MongoDB"
	default:
		return service
	}
}

// formatHealthDetail formats the detail string for a health snapshot
func formatHealthDetail(snap HealthSnapshot) string {
	switch snap.Status {
	case "up":
		if snap.LatencyMs > 0 {
			return fmt.Sprintf("Connected (latency: %dms)", snap.LatencyMs)
		}
		return "Connected"
	case "down":
		return "Down"
	case "degraded":
		return "Degraded"
	case "unconfigured":
		return "Not configured"
	default:
		return snap.Status
	}
}

// handlePause pauses the scheduler
func (cli *CLI) handlePause() error {
	if !cli.daemon.IsRunning() {
		fmt.Println("❌ Daemon is not running")
		return nil
	}

	if err := cli.daemon.Pause(); err != nil {
		fmt.Printf("❌ Failed to pause: %v\n", err)
		return err
	}

	fmt.Println("✓ Scheduler paused")
	fmt.Println("\nGit monitoring is still active")
	fmt.Println("Use 'devtrack resume' to resume scheduler")
	return nil
}

// handleResume resumes the scheduler
func (cli *CLI) handleResume() error {
	if !cli.daemon.IsRunning() {
		fmt.Println("❌ Daemon is not running")
		return nil
	}

	if err := cli.daemon.Resume(); err != nil {
		fmt.Printf("❌ Failed to resume: %v\n", err)
		return err
	}

	fmt.Println("✓ Scheduler resumed")
	return nil
}

// handleForceTrigger forces an immediate trigger by sending SIGUSR2 to the running daemon
func (cli *CLI) handleForceTrigger() error {
	if !cli.daemon.IsRunning() {
		fmt.Println("❌ Daemon is not running")
		fmt.Println("\nStart the daemon first:")
		fmt.Println("  devtrack start")
		return nil
	}

	pid, err := cli.daemon.readPID()
	if err != nil {
		fmt.Printf("❌ Could not read daemon PID: %v\n", err)
		return err
	}

	fmt.Println("⚡ Forcing immediate trigger...")

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("❌ Could not find daemon process: %v\n", err)
		return err
	}

	if err := process.Signal(syscall.SIGUSR2); err != nil {
		fmt.Printf("❌ Could not send signal to daemon: %v\n", err)
		return err
	}

	// Give it a moment to execute
	time.Sleep(500 * time.Millisecond)

	fmt.Println("✓ Trigger initiated successfully")
	fmt.Println("\nThe trigger is executing in the background.")
	fmt.Println("Check logs for details:")
	fmt.Println("  devtrack logs")
	return nil
}

// handleSendSummary generates and sends the daily summary
func (cli *CLI) handleSendSummary() error {
	if err := requiresManagedMode("send-summary"); err != nil {
		return err
	}
	if !cli.daemon.IsRunning() {
		fmt.Println("❌ Daemon is not running")
		fmt.Println("\nStart the daemon first:")
		fmt.Println("  devtrack start")
		return nil
	}

	fmt.Println("📊 Generating daily summary...")
	fmt.Println()

	// Get today's statistics
	stats := map[string]interface{}{
		"date":     time.Now().Format("January 2, 2006"),
		"triggers": 0,
	}

	if cli.daemon.monitor != nil && cli.daemon.monitor.scheduler != nil {
		schedulerStats := cli.daemon.monitor.scheduler.GetStats()
		stats["triggers"] = schedulerStats["trigger_count"]
	}

	fmt.Printf("📅 Summary for %s\n", stats["date"])
	fmt.Println("═══════════════════════════════")
	fmt.Printf("Triggers today:    %v\n", stats["triggers"])
	fmt.Println()

	// TODO: When database is implemented, query actual data
	fmt.Println("⚠️  Full summary generation not yet implemented")
	fmt.Println()
	fmt.Println("Coming soon:")
	fmt.Println("  • Query SQLite database for today's activities")
	fmt.Println("  • Aggregate commits and work items")
	fmt.Println("  • Format as email/Teams message")
	fmt.Println("  • Send to configured recipients")
	fmt.Println()
	fmt.Println("For now, this shows current trigger count.")

	return nil
}

// handleSkipNext skips the next scheduled trigger
func (cli *CLI) handleSkipNext() error {
	if !cli.daemon.IsRunning() {
		fmt.Println("❌ Daemon is not running")
		fmt.Println("\nStart the daemon first:")
		fmt.Println("  devtrack start")
		return nil
	}

	if cli.daemon.monitor == nil || cli.daemon.monitor.scheduler == nil {
		fmt.Println("❌ Scheduler not initialized")
		return fmt.Errorf("scheduler not available")
	}

	// Get current stats to show what's being skipped
	stats := cli.daemon.monitor.scheduler.GetStats()
	nextTrigger := stats["time_until_next"]

	fmt.Printf("⏭️  Skipping next trigger (was due in %v)\n", nextTrigger)

	cli.daemon.monitor.scheduler.SkipNext()

	// Get updated stats
	stats = cli.daemon.monitor.scheduler.GetStats()
	newNextTrigger := stats["time_until_next"]

	fmt.Println("✓ Next trigger skipped")
	fmt.Printf("\nNew next trigger: %v\n", newNextTrigger)

	return nil
}

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

// handleEnableLearning enables personalized AI learning
func (cli *CLI) handleEnableLearning() error {
	if err := requiresManagedMode("enable-learning"); err != nil {
		return err
	}
	days := GetLearningDefaultDays()
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &days)
	}

	learning := NewLearningCommands()
	return learning.EnableLearning(days)
}

// handleShowProfile shows the learning profile
func (cli *CLI) handleShowProfile() error {
	if err := requiresManagedMode("show-profile"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.ShowProfile()
}

// handleTestResponse tests generating a response
func (cli *CLI) handleTestResponse() error {
	if err := requiresManagedMode("test-response"); err != nil {
		return err
	}
	if len(os.Args) < 3 {
		fmt.Println("❌ Usage: devtrack test-response <text>")
		return fmt.Errorf("missing text argument")
	}

	text := strings.Join(os.Args[2:], " ")
	learning := NewLearningCommands()
	return learning.TestResponse(text)
}

// handleRevokeConsent revokes learning consent
func (cli *CLI) handleRevokeConsent() error {
	if err := requiresManagedMode("revoke-consent"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.RevokeConsent()
}

// handleLearningStatus shows learning status
func (cli *CLI) handleLearningStatus() error {
	if err := requiresManagedMode("learning-status"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	status, err := learning.GetLearningStatus()
	if err != nil {
		fmt.Printf("❌ Failed to get learning status: %v\n", err)
		return err
	}

	status.PrintStatus()
	return nil
}

// handleLearningReset wipes all learning data for a fresh start
func (cli *CLI) handleLearningReset() error {
	if err := requiresManagedMode("learning-reset"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.ResetLearning()
}

// handleLearningSetupCron installs the crontab entry from LEARNING_CRON_SCHEDULE
func (cli *CLI) handleLearningSetupCron() error {
	if err := requiresManagedMode("learning-setup-cron"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.SetupCron()
}

// handleLearningRemoveCron removes the DevTrack learning crontab entry
func (cli *CLI) handleLearningRemoveCron() error {
	if err := requiresManagedMode("learning-remove-cron"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.RemoveCron()
}

// handleLearningCronStatus shows cron entry status
func (cli *CLI) handleLearningCronStatus() error {
	if err := requiresManagedMode("learning-cron-status"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.CronStatus()
}

// handleLearningSync runs a delta (or full) sync immediately
func (cli *CLI) handleLearningSync() error {
	if err := requiresManagedMode("learning-sync"); err != nil {
		return err
	}
	full := len(os.Args) > 2 && os.Args[2] == "--full"
	learning := NewLearningCommands()
	return learning.SyncNow(full)
}

// handlePreviewReport previews today's email report
func (cli *CLI) handlePreviewReport() error {
	if err := requiresManagedMode("preview-report"); err != nil {
		return err
	}
	date := ""
	if len(os.Args) > 2 {
		date = os.Args[2]
	}

	fmt.Println("📊 Generating daily report preview...")
	fmt.Println()

	scriptPath, err := GetEmailReporterPath()
	if err != nil {
		return fmt.Errorf("cannot generate report: %w", err)
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	args := []string{"run", "--directory", projectRoot, "python", scriptPath, "preview"}
	if date != "" {
		args = append(args, date)
	}

	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed to generate report: %v\n", err)
		return err
	}

	return nil
}

// handleSendReport sends email report
func (cli *CLI) handleSendReport() error {
	if err := requiresManagedMode("send-report"); err != nil {
		return err
	}
	if len(os.Args) < 3 {
		fmt.Println("❌ Usage: devtrack send-report <email> [date]")
		return fmt.Errorf("missing email argument")
	}

	email := os.Args[2]
	date := ""
	if len(os.Args) > 3 {
		date = os.Args[3]
	}

	fmt.Printf("📧 Sending report to %s...\n", email)
	fmt.Println()

	scriptPath, err := GetEmailReporterPath()
	if err != nil {
		return fmt.Errorf("cannot send report: %w", err)
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	args := []string{"run", "--directory", projectRoot, "python", scriptPath, "send", email}
	if date != "" {
		args = append(args, date)
	}

	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed to send report: %v\n", err)
		return err
	}

	return nil
}

// handleSaveReport saves report to file
func (cli *CLI) handleSaveReport() error {
	if err := requiresManagedMode("save-report"); err != nil {
		return err
	}
	date := ""
	if len(os.Args) > 2 {
		date = os.Args[2]
	}

	fmt.Println("💾 Saving report to file...")
	fmt.Println()

	scriptPath, err := GetEmailReporterPath()
	if err != nil {
		return fmt.Errorf("cannot save report: %w", err)
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	args := []string{"run", "--directory", projectRoot, "python", scriptPath, "save"}
	if date != "" {
		args = append(args, date)
	}

	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed to save report: %v\n", err)
		return err
	}

	return nil
}

// handleCommitQueue handles the internal commit-queue command (called by git wrapper)
func (cli *CLI) handleCommitQueue() error {
	// Parse flags
	message := ""
	branch := ""
	repoPath := ""
	filesStr := ""

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--message":
			if i+1 < len(os.Args) {
				message = os.Args[i+1]
				i++
			}
		case "--branch":
			if i+1 < len(os.Args) {
				branch = os.Args[i+1]
				i++
			}
		case "--repo":
			if i+1 < len(os.Args) {
				repoPath = os.Args[i+1]
				i++
			}
		case "--files":
			if i+1 < len(os.Args) {
				filesStr = os.Args[i+1]
				i++
			}
		}
	}

	if message == "" {
		return fmt.Errorf("--message is required")
	}

	// Read diff from stdin
	diffPatch := ""
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data := make([]byte, 0, 1024*1024) // 1MB max
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				data = append(data, buf[:n]...)
			}
			if err != nil {
				break
			}
			if len(data) > 1024*1024 {
				break // cap at 1MB
			}
		}
		diffPatch = string(data)
	}

	// Parse files
	var files []string
	if filesStr != "" {
		files = strings.Split(filesStr, ",")
	}

	// Open database and queue
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	mgr := NewDeferredCommitManager(db)
	id, err := mgr.QueueCommit(message, diffPatch, branch, repoPath, files)
	if err != nil {
		return err
	}

	fmt.Printf("Commit queued (ID: %d)\n", id)
	return nil
}

// handleCommits handles commits subcommands (pending, review)
func (cli *CLI) handleCommits() error {
	subCmd := "pending"
	if len(os.Args) > 2 {
		subCmd = os.Args[2]
	}

	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	mgr := NewDeferredCommitManager(db)

	switch subCmd {
	case "pending":
		return mgr.ListPending()
	case "review":
		return mgr.ReviewEnhanced()
	default:
		fmt.Printf("Unknown commits subcommand: %s\n", subCmd)
		fmt.Println("Usage:")
		fmt.Println("  devtrack commits pending  - List deferred commits")
		fmt.Println("  devtrack commits review   - Review enhanced commits")
		return nil
	}
}

// handleQueueStats shows message queue statistics
func (cli *CLI) handleQueueStats() error {
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	pending, failed, sent, err := db.GetMessageQueueStats()
	if err != nil {
		return fmt.Errorf("failed to get queue stats: %w", err)
	}

	fmt.Println("Message Queue")
	fmt.Println(strings.Repeat("═", 40))
	fmt.Printf("  Pending:   %d\n", pending)
	fmt.Printf("  Failed:    %d\n", failed)
	fmt.Printf("  Sent:      %d\n", sent)
	fmt.Println()

	if pending > 0 {
		messages, err := db.GetPendingMessages(10)
		if err == nil && len(messages) > 0 {
			fmt.Println("Pending Messages:")
			fmt.Println(strings.Repeat("─", 40))
			for _, m := range messages {
				fmt.Printf("  [%s] %s (queued: %s, retries: %d)\n",
					m.MessageType, m.MessageID,
					m.CreatedAt.Format("15:04:05"),
					m.RetryCount)
			}
			fmt.Println()
		}
	}

	if failed > 0 {
		fmt.Println("Note: Failed messages will be retried automatically when Python bridge reconnects.")
		fmt.Println()
	}

	return nil
}

// handleAzureCheck verifies Azure DevOps config and connectivity
func (cli *CLI) handleAzureCheck() error {
	if err := requiresManagedMode("azure-check"); err != nil {
		return err
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "azure", "check.py")

	args := []string{"run", "--directory", projectRoot, "python", scriptPath}
	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}


// handleAzureList lists work items assigned to the user
func (cli *CLI) handleAzureList() error {
	if err := requiresManagedMode("azure-list"); err != nil {
		return err
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "azure", "list_items.py")
	args := []string{"run", "--directory", projectRoot, "python", scriptPath}
	args = append(args, os.Args[2:]...) // forward --all, --state flags
	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleAzureSync runs a manual sync with Azure DevOps.
// Passes --full or --hours N through from CLI args.
func (cli *CLI) handleAzureSync() error {
	if err := requiresManagedMode("azure-sync"); err != nil {
		return err
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "azure", "run_sync.py")
	uvArgs := []string{"run", "--directory", projectRoot, "python", scriptPath}

	// Forward any flags after "azure-sync" (e.g. --full, --hours 24)
	if len(os.Args) > 2 {
		uvArgs = append(uvArgs, os.Args[2:]...)
	}

	cmd := exec.Command("uv", uvArgs...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleAzureView shows details for a specific Azure DevOps work item
func (cli *CLI) handleAzureView() error {
	if err := requiresManagedMode("azure-view"); err != nil {
		return err
	}
	if len(os.Args) < 3 {
		fmt.Println("Usage: devtrack azure-view <work-item-id>")
		return fmt.Errorf("missing work item ID")
	}

	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "azure", "view_item.py")
	args := []string{"run", "--directory", projectRoot, "python", scriptPath, os.Args[2]}
	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleGitLabCheck verifies GitLab config and connectivity
func (cli *CLI) handleGitLabCheck() error {
	if err := requiresManagedMode("gitlab-check"); err != nil {
		return err
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "gitlab", "check.py")
	args := []string{"run", "--directory", projectRoot, "python", scriptPath}
	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleGitLabList lists GitLab issues assigned to the user
func (cli *CLI) handleGitLabList() error {
	if err := requiresManagedMode("gitlab-list"); err != nil {
		return err
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "gitlab", "list_items.py")
	args := []string{"run", "--directory", projectRoot, "python", scriptPath}
	args = append(args, os.Args[2:]...) // forward --closed, --state flags
	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleGitLabSync runs a manual sync with GitLab.
// Passes --full or --hours N through from CLI args.
func (cli *CLI) handleGitLabSync() error {
	if err := requiresManagedMode("gitlab-sync"); err != nil {
		return err
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "gitlab", "run_sync.py")
	uvArgs := []string{"run", "--directory", projectRoot, "python", scriptPath}

	// Forward any flags after "gitlab-sync" (e.g. --full, --hours 24)
	if len(os.Args) > 2 {
		uvArgs = append(uvArgs, os.Args[2:]...)
	}

	cmd := exec.Command("uv", uvArgs...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleGitLabView shows details for a specific GitLab issue
func (cli *CLI) handleGitLabView() error {
	if err := requiresManagedMode("gitlab-view"); err != nil {
		return err
	}
	if len(os.Args) < 4 {
		fmt.Println("Usage: devtrack gitlab-view <project_id> <issue_iid>")
		return fmt.Errorf("missing project_id and/or issue_iid")
	}

	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "gitlab", "view_item.py")
	args := []string{"run", "--directory", projectRoot, "python", scriptPath, os.Args[2], os.Args[3]}
	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleGitHubCheck verifies GitHub config and connectivity
func (cli *CLI) handleGitHubCheck() error {
	if err := requiresManagedMode("github-check"); err != nil {
		return err
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "github", "check.py")
	args := []string{"run", "--directory", projectRoot, "python", scriptPath}
	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleGitHubList lists GitHub issues assigned to the user
func (cli *CLI) handleGitHubList() error {
	if err := requiresManagedMode("github-list"); err != nil {
		return err
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "github", "list_items.py")
	args := []string{"run", "--directory", projectRoot, "python", scriptPath}
	args = append(args, os.Args[2:]...) // forward --closed, --state flags
	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleGitHubSync runs a manual sync with GitHub.
// Passes --full or --hours N through from CLI args.
func (cli *CLI) handleGitHubSync() error {
	if err := requiresManagedMode("github-sync"); err != nil {
		return err
	}
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "github", "run_sync.py")
	uvArgs := []string{"run", "--directory", projectRoot, "python", scriptPath}

	// Forward any flags after "github-sync" (e.g. --full, --hours 24)
	if len(os.Args) > 2 {
		uvArgs = append(uvArgs, os.Args[2:]...)
	}

	cmd := exec.Command("uv", uvArgs...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleGitHubView shows details for a specific GitHub issue
func (cli *CLI) handleGitHubView() error {
	if err := requiresManagedMode("github-view"); err != nil {
		return err
	}
	if len(os.Args) < 3 {
		fmt.Println("Usage: devtrack github-view <issue_number>")
		return fmt.Errorf("missing issue_number")
	}

	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	scriptPath := filepath.Join(projectRoot, "backend", "github", "view_item.py")
	args := []string{"run", "--directory", projectRoot, "python", scriptPath, os.Args[2]}
	cmd := exec.Command("uv", args...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleVacation dispatches vacation mode subcommands.
//
// Usage:
//
//	devtrack vacation on [--until YYYY-MM-DD] [--threshold 0.7] [--no-submit]
//	devtrack vacation off
//	devtrack vacation status
func (cli *CLI) handleVacation() error {
	vc, err := NewVacationCommands()
	if err != nil {
		return err
	}
	args := os.Args
	sub := ""
	if len(args) > 2 {
		sub = args[2]
	}
	switch sub {
	case "on":
		return vc.On(args[3:])
	case "off":
		return vc.Off()
	case "status", "":
		return vc.Status()
	default:
		return fmt.Errorf("unknown vacation subcommand %q — use: on | off | status", sub)
	}
}

// handleAlerts shows ticket alert notifications or marks them as read.
//
// Usage:
//
//	devtrack alerts           — show unread notifications (last 24h)
//	devtrack alerts --all     — show all notifications
//	devtrack alerts --clear   — mark all as read
func (cli *CLI) handleAlerts() error {
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	// Build uv run python -m backend.alert_poller [flags]
	uvArgs := []string{"run", "--directory", projectRoot, "python", "-m", "backend.alert_poller"}

	// Forward flags: --all, --clear (default is --show)
	showFlag := true
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--all":
			uvArgs = append(uvArgs, "--show", "--all")
			showFlag = false
		case "--clear":
			uvArgs = append(uvArgs, "--clear")
			showFlag = false
		}
	}
	if showFlag {
		uvArgs = append(uvArgs, "--show")
	}

	cmd := exec.Command("uv", uvArgs...)
	if projectRoot != "" {
		cmd.Dir = projectRoot
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// handleWorkspace dispatches workspace subcommands
func (cli *CLI) handleWorkspace() error {
	subCmd := ""
	if len(os.Args) > 2 {
		subCmd = os.Args[2]
	}

	wc := NewWorkspaceCommands()
	switch subCmd {
	case "list", "":
		return wc.List()
	case "add":
		if len(os.Args) < 5 {
			fmt.Println("Usage: devtrack workspace add <name> <path> [--pm azure|gitlab|github|jira|none]")
			return fmt.Errorf("missing arguments")
		}
		name := os.Args[3]
		path := os.Args[4]
		pmPlatform := ""
		addArgs := os.Args[5:]
		for i := 0; i < len(addArgs); i++ {
			if addArgs[i] == "--pm" && i+1 < len(addArgs) {
				pmPlatform = addArgs[i+1]
				i++
			} else if !strings.HasPrefix(addArgs[i], "--") {
				// backwards-compatible: bare positional platform arg
				pmPlatform = addArgs[i]
			}
		}
		return wc.Add(name, path, pmPlatform)
	case "remove":
		if len(os.Args) < 4 {
			fmt.Println("Usage: devtrack workspace remove <name>")
			return fmt.Errorf("missing name argument")
		}
		return wc.Remove(os.Args[3])
	case "enable":
		if len(os.Args) < 4 {
			fmt.Println("Usage: devtrack workspace enable <name>")
			return fmt.Errorf("missing name argument")
		}
		return wc.Enable(os.Args[3])
	case "disable":
		if len(os.Args) < 4 {
			fmt.Println("Usage: devtrack workspace disable <name>")
			return fmt.Errorf("missing name argument")
		}
		return wc.Disable(os.Args[3])
	case "reload":
		return wc.Reload()
	case "install-hooks":
		return wc.InstallHooks()
	default:
		fmt.Printf("Unknown workspace subcommand: %s\n", subCmd)
		fmt.Println("Usage:")
		fmt.Println("  devtrack workspace list                         List configured workspaces")
		fmt.Println("  devtrack workspace add <name> <path> [--pm azure|gitlab|github|jira|none]  Add a workspace")
		fmt.Println("  devtrack workspace remove <name>                Remove a workspace")
		fmt.Println("  devtrack workspace enable <name>                Enable a workspace")
		fmt.Println("  devtrack workspace disable <name>               Disable a workspace")
		fmt.Println("  devtrack workspace reload                        Reload workspaces in running daemon")
		fmt.Println("  devtrack workspace install-hooks                Install post-commit hooks in all workspaces")
		return fmt.Errorf("unknown workspace subcommand: %s", subCmd)
	}
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
		fmt.Printf("  Chat ID:     %s\n", maskEmpty(os.Getenv("TELEGRAM_CHAT_ID")))
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

// handleTelegramStatus shows Telegram bot status
func (cli *CLI) handleTelegramStatus() error {
	// Ensure .env is loaded (this command skips NewDaemon)
	LoadEnvConfig() // ignore error — IsTelegramEnabled will just read os.Getenv
	if !IsTelegramEnabled() {
		fmt.Println("Telegram bot is disabled (TELEGRAM_ENABLED is not true)")
		return nil
	}
	fmt.Println("Telegram Bot Status")
	fmt.Println("===================")
	fmt.Println("Enabled: true")

	// Check health from DB
	db, err := NewDatabase()
	if err != nil {
		fmt.Println("Status: unknown (database unavailable)")
		return nil
	}
	defer db.Close()

	snapshots, err := db.GetLatestHealthSnapshots()
	if err != nil {
		fmt.Println("Status: unknown")
		return nil
	}

	for _, snap := range snapshots {
		if snap.Service == "telegram_bot" {
			fmt.Printf("Status: %s\n", snap.Status)
			if snap.Details != "" {
				fmt.Printf("Details: %s\n", snap.Details)
			}
			fmt.Printf("Last checked: %s\n", snap.CheckedAt.Format(time.RFC3339))
			return nil
		}
	}

	fmt.Println("Status: no health data yet")
	return nil
}

// handleShellInit outputs shell integration code for eval "$(devtrack shell-init)"
// This defines a git() function that transparently routes commit/history/messages
// through DevTrack for monitored workspaces, passing everything else to real git.
func (cli *CLI) handleShellInit() error {
	fmt.Print(`# DevTrack shell integration
# Transparently routes git commands through DevTrack for monitored workspaces.
# Add to ~/.zshrc or ~/.bashrc:
#   eval "$(devtrack shell-init)"

git() {
  # Only intercept when inside a git repo
  if command git rev-parse --git-dir >/dev/null 2>&1; then
    # Honour explicit bypass: GIT_NO_DEVTRACK=1 git commit
    if [ "${GIT_NO_DEVTRACK:-}" = "1" ]; then
      command git "$@"
      return $?
    fi

    local _dt_enabled=""

    # Fast path: per-repo opt-in/out via git config (reads .git/config, no subprocess)
    # 'devtrack enable-git'  sets devtrack.enabled=true  → always intercept
    # 'devtrack disable-git' sets devtrack.enabled=false → never intercept (overrides workspaces.yaml)
    _dt_enabled=$(command git config --local devtrack.enabled 2>/dev/null || true)

    # Explicit opt-out: skip even if this repo is in workspaces.yaml
    if [ "$_dt_enabled" = "false" ]; then
      command git "$@"
      return $?
    fi

    # Slow path: check workspaces.yaml when not explicitly set
    if [ -z "$_dt_enabled" ] && command -v devtrack >/dev/null 2>&1; then
      if devtrack is-workspace 2>/dev/null; then
        _dt_enabled="true"
      fi
    fi

    if [ "$_dt_enabled" = "true" ]; then
      case "$1" in
        commit|history|messages|add)
          devtrack git "$@"
          return $?
          ;;
      esac
    fi
  fi

  command git "$@"
}
`)
	return nil
}

// handleIsWorkspace exits 0 if the current directory is a DevTrack workspace, 1 otherwise.
// Used by the shell-init git() function to decide whether to intercept git commands.
func (cli *CLI) handleIsWorkspace() error {
	// Get the git root of the current directory
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		os.Exit(1) // not a git repo
	}
	gitRoot := strings.TrimSpace(string(out))
	gitRoot, _ = filepath.Abs(gitRoot)

	// Single-repo mode: check DEVTRACK_WORKSPACE
	workspacePath := strings.TrimSpace(os.Getenv("DEVTRACK_WORKSPACE"))
	if workspacePath != "" {
		wsAbs, _ := filepath.Abs(workspacePath)
		if wsAbs == gitRoot || strings.HasPrefix(gitRoot, wsAbs+string(filepath.Separator)) {
			os.Exit(0)
		}
	}

	// Multi-repo mode: check workspaces.yaml
	wsCfg, err := LoadWorkspacesConfig()
	if err != nil || wsCfg == nil {
		os.Exit(1)
	}
	for _, ws := range wsCfg.GetEnabledWorkspaces() {
		wsPath, _ := filepath.Abs(ws.Path)
		if wsPath == gitRoot || strings.HasPrefix(gitRoot, wsPath+string(filepath.Separator)) {
			os.Exit(0)
		}
	}

	os.Exit(1)
	return nil
}

// enableGitForWorkspaces sets devtrack.enabled=true and installs the post-commit
// hook in all enabled workspaces. Called automatically on `devtrack start`.
func enableGitForWorkspaces() {
	cfg, err := LoadWorkspacesConfig()
	if err != nil || cfg == nil {
		return
	}
	for _, ws := range cfg.GetEnabledWorkspaces() {
		cmd := exec.Command("git", "-C", ws.Path, "config", "--local", "devtrack.enabled", "true")
		if err := cmd.Run(); err != nil {
			continue
		}
		if err := InstallPostCommitHook(ws.Path); err != nil {
			fmt.Printf("  ⚠ Git integration enabled for %s but hook install failed: %v\n", ws.Name, err)
		} else {
			fmt.Printf("  ✓ Git integration enabled: %s\n", ws.Name)
		}
	}
}

// handleEnableGit sets git config devtrack.enabled=true in the current repo,
// opting it into DevTrack shell integration without editing workspaces.yaml.
func (cli *CLI) handleEnableGit() error {
	cmd := exec.Command("git", "config", "--local", "devtrack.enabled", "true")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git config: %v\nAre you inside a git repository?", err)
	}
	fmt.Println("✓ DevTrack git integration enabled for this repo.")
	fmt.Println("  git add, git commit, git history will now route through DevTrack.")
	fmt.Println()
	fmt.Println("  Shell integration required — add to ~/.zshrc or ~/.bashrc if not done yet:")
	fmt.Println(`    eval "$(devtrack shell-init)"`)
	fmt.Println()
	fmt.Println("  If already set up, reload your shell function to pick up any updates:")
	fmt.Println(`    eval "$(devtrack shell-init)"`)
	fmt.Println()
	fmt.Println("  To disable: devtrack disable-git")
	return nil
}

// handleDisableGit sets git config devtrack.enabled=false in the current repo.
// Setting false explicitly overrides workspaces.yaml detection in the shell function.
// (Simply unsetting the key would leave workspaces.yaml matching active.)
func (cli *CLI) handleDisableGit() error {
	cmd := exec.Command("git", "config", "--local", "devtrack.enabled", "false")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git config: %v\nAre you inside a git repository?", err)
	}
	fmt.Println("✓ DevTrack git integration disabled for this repo.")
	fmt.Println("  'git commit' will use standard git (even if this repo is in workspaces.yaml).")
	fmt.Println()
	fmt.Println("  To re-enable: devtrack enable-git")
	return nil
}

// launchdPlistTemplatePath returns the path to the bundled plist template.
// It searches relative to the running binary (supports both the dev tree and
// an installed binary whose configs dir lives beside it).
func launchdPlistTemplatePath() (string, error) {
	// 1. Try PROJECT_ROOT env var (set by daemon, set in .env, or by user)
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		// 2. Walk up from binary looking for Data/configs/dev.devtrack.plist
		execPath, err := os.Executable()
		if err != nil {
			execPath = os.Args[0]
		}
		execPath, _ = filepath.Abs(execPath)
		dir := filepath.Dir(execPath)
		for i := 0; i < 6; i++ {
			candidate := filepath.Join(dir, "Data", "configs", "dev.devtrack.plist")
			if _, err := os.Stat(candidate); err == nil {
				// dir is the project root
				projectRoot = dir
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if projectRoot == "" {
		return "", fmt.Errorf("cannot determine project root: set PROJECT_ROOT or run from the DevTrack directory")
	}
	tmplPath := filepath.Join(projectRoot, "Data", "configs", "dev.devtrack.plist")
	if _, err := os.Stat(tmplPath); err != nil {
		return "", fmt.Errorf("plist template not found at %s: %w", tmplPath, err)
	}
	return tmplPath, nil
}

// launchdEnvVarPrefixes lists the env var prefixes that should be captured into
// the launchd plist. launchd starts processes with a minimal environment, so
// all vars devtrack needs must be embedded at install time.
var launchdEnvVarPrefixes = []string{
	"PROJECT_ROOT", "DEVTRACK_", "DATA_", "DATABASE_", "LOG_", "PID_",
	"CONFIG_", "LEARNING_", "WORKSPACES_", "CLI_",
	"IPC_", "WEBHOOK_",
	"LLM_", "OLLAMA_", "LMSTUDIO_", "OPENAI_", "ANTHROPIC_", "GROQ_",
	"GIT_SAGE_", "HTTP_", "PROMPT_",
	"AZURE_", "GITHUB_", "GITLAB_", "JIRA_", "TEAMS_",
	"EMAIL_", "TELEGRAM_", "SLACK_",
	"ALERT_", "ADMIN_",
	"WORK_", "TIMEZONE", "PROMPT_INTERVAL", "AUTO_SYNC", "OUTPUT_TYPE",
	"DAILY_REPORT_TIME", "WEEKLY_REPORT_DAY", "SEND_ON_TRIGGER", "SEND_DAILY_SUMMARY",
	"PYTHON_BRIDGE_SCRIPT",
	"MONGODB_", "REDIS_",
	"NEWPROJECT_", "SPEC_", "VACATION_", "EOD_", "WORK_SESSION_",
	"SENTIMENT_", "IPC_RETRY_", "LLM_REQUEST_", "QUEUE_", "DEFERRED_",
	"HEALTH_",
}

// shouldCaptureForLaunchd returns true if the env var name should be included
// in the launchd plist's EnvironmentVariables.
func shouldCaptureForLaunchd(name string) bool {
	// Always include PATH and HOME so child processes work correctly.
	if name == "PATH" || name == "HOME" || name == "USER" || name == "SHELL" {
		return true
	}
	for _, prefix := range launchdEnvVarPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
		if name == strings.TrimRight(prefix, "_") {
			return true
		}
	}
	return false
}

// xmlEscape escapes special XML characters in an attribute/text value.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// buildLaunchdPlist generates a launchd plist that embeds all current devtrack
// env vars so launchd can start the daemon with the correct environment.
func buildLaunchdPlist(binaryPath, projectRoot string) string {
	// Collect env vars to inject.
	type envEntry struct{ key, val string }
	var entries []envEntry
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		if shouldCaptureForLaunchd(key) {
			entries = append(entries, envEntry{key, val})
		}
	}
	// Sort for deterministic output.
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].key > entries[j].key {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	var envXML strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&envXML, "        <key>%s</key>\n        <string>%s</string>\n",
			xmlEscape(e.key), xmlEscape(e.val))
	}

	logPath := filepath.Join(projectRoot, "Data", "logs", "launchd.log")

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.devtrack</string>

    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>start</string>
    </array>

    <key>EnvironmentVariables</key>
    <dict>
%s    </dict>

    <key>WorkingDirectory</key>
    <string>%s</string>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <false/>

    <key>StandardOutPath</key>
    <string>%s</string>

    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`, xmlEscape(binaryPath), envXML.String(), xmlEscape(projectRoot),
		xmlEscape(logPath), xmlEscape(logPath))
}

// handleLaunchdInstall installs the launchd plist to ~/Library/LaunchAgents
// and loads it with launchctl so DevTrack auto-starts on login.
// All current devtrack env vars are baked into the plist so launchd can start
// the daemon with the correct environment (environment-first config).
func (cli *CLI) handleLaunchdInstall() error {
	// Resolve project root (used as WorkingDirectory and in env paths)
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		config, _ := LoadEnvConfig()
		if config != nil {
			projectRoot = config.ProjectRoot
		}
	}
	if projectRoot == "" {
		// Derive from the binary location
		execPath, err := os.Executable()
		if err != nil {
			execPath = os.Args[0]
		}
		execPath, _ = filepath.Abs(execPath)
		dir := filepath.Dir(execPath)
		for i := 0; i < 6; i++ {
			if _, err := os.Stat(filepath.Join(dir, "Data", "configs")); err == nil {
				projectRoot = dir
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if projectRoot == "" {
		return fmt.Errorf("cannot determine PROJECT_ROOT; export it or run from the DevTrack directory")
	}
	projectRoot, _ = filepath.Abs(projectRoot)

	// Resolve the DevTrack binary path
	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = os.Args[0]
	}
	binaryPath, _ = filepath.Abs(binaryPath)

	// Generate plist content with current env vars embedded.
	plistContent := buildLaunchdPlist(binaryPath, projectRoot)

	// Determine destination: ~/Library/LaunchAgents/dev.devtrack.plist
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}
	destPath := filepath.Join(launchAgentsDir, "dev.devtrack.plist")

	// Unload first if already loaded (ignore errors — it may not be loaded yet)
	_ = exec.Command("launchctl", "unload", destPath).Run()

	// Write the plist
	if err := os.WriteFile(destPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist to %s: %w", destPath, err)
	}

	// Load with launchctl
	loadCmd := exec.Command("launchctl", "load", destPath)
	loadCmd.Stdout = os.Stdout
	loadCmd.Stderr = os.Stderr
	if err := loadCmd.Run(); err != nil {
		return fmt.Errorf("launchctl load failed: %w\nPlist installed at %s — load it manually with: launchctl load %s", err, destPath, destPath)
	}

	fmt.Println("DevTrack launchd service installed.")
	fmt.Printf("  Plist:   %s\n", destPath)
	fmt.Printf("  Binary:  %s\n", binaryPath)
	fmt.Printf("  Root:    %s\n", projectRoot)
	fmt.Println()
	fmt.Println("DevTrack will now start automatically at login.")
	fmt.Println("Tip: re-run 'devtrack autostart-install' after changing env vars.")
	fmt.Println("Use 'devtrack status' to verify it is running.")
	fmt.Println("Use 'devtrack launchd-uninstall' to remove auto-start.")
	return nil
}

// handleLaunchdUninstall unloads and removes the launchd plist.
func (cli *CLI) handleLaunchdUninstall() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "dev.devtrack.plist")

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Println("DevTrack launchd service is not installed.")
		return nil
	}

	// Unload
	unloadCmd := exec.Command("launchctl", "unload", plistPath)
	unloadCmd.Stdout = os.Stdout
	unloadCmd.Stderr = os.Stderr
	if err := unloadCmd.Run(); err != nil {
		fmt.Printf("Warning: launchctl unload returned an error: %v\n", err)
		fmt.Println("Proceeding to remove plist anyway...")
	}

	// Remove plist
	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("failed to remove plist %s: %w", plistPath, err)
	}

	fmt.Println("DevTrack launchd service removed.")
	fmt.Printf("  Removed: %s\n", plistPath)
	fmt.Println()
	fmt.Println("DevTrack will no longer start automatically at login.")
	fmt.Println("The running daemon (if any) was not stopped — use 'devtrack stop' to stop it.")
	return nil
}

// ---------------------------------------------------------------------------
// OS-agnostic autostart (detectOSType / handleAutostart*)
// ---------------------------------------------------------------------------

// osType enumerates the autostart mechanisms we support.
type osType string

const (
	osDarwin      osType = "darwin"
	osLinuxSystemd osType = "linux-systemd"
	osWSLSystemd  osType = "wsl-systemd"
	osWSLNoSystemd osType = "wsl-nosystemd"
)

// detectOSType returns the appropriate autostart mechanism for the current OS.
func detectOSType() osType {
	switch runtime.GOOS {
	case "darwin":
		return osDarwin
	case "linux":
		if isWSL() {
			if hasSystemd() {
				return osWSLSystemd
			}
			return osWSLNoSystemd
		}
		if hasSystemd() {
			return osLinuxSystemd
		}
		// Linux without systemd — fall back to profile-based (same as WSL-nosystemd)
		return osWSLNoSystemd
	default:
		return osDarwin // best guess for unknown OS
	}
}

// isWSL returns true when running inside Windows Subsystem for Linux.
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// hasSystemd returns true when systemd is the active init system.
func hasSystemd() bool {
	// Most reliable: private systemd runtime directory.
	if _, err := os.Stat("/run/systemd/private"); err == nil {
		return true
	}
	// Check PID-1 comm.
	if data, err := os.ReadFile("/proc/1/comm"); err == nil {
		if strings.TrimSpace(string(data)) == "systemd" {
			return true
		}
	}
	// Fall back to pidof.
	if err := exec.Command("pidof", "systemd").Run(); err == nil {
		return true
	}
	return false
}

// resolveProjectRootForAutostart returns the project root, trying PROJECT_ROOT
// env var first and then walking up from the binary.
func resolveProjectRootForAutostart() (string, error) {
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		config, _ := LoadEnvConfig()
		if config != nil {
			projectRoot = config.ProjectRoot
		}
	}
	if projectRoot == "" {
		execPath, err := os.Executable()
		if err != nil {
			execPath = os.Args[0]
		}
		execPath, _ = filepath.Abs(execPath)
		dir := filepath.Dir(execPath)
		for i := 0; i < 6; i++ {
			if _, err := os.Stat(filepath.Join(dir, "Data", "configs")); err == nil {
				projectRoot = dir
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if projectRoot == "" {
		return "", fmt.Errorf("cannot determine PROJECT_ROOT; set it in .env or as an environment variable")
	}
	abs, _ := filepath.Abs(projectRoot)
	return abs, nil
}

// buildSystemdService generates a systemd user unit file that embeds all current
// devtrack env vars as Environment= lines.
func buildSystemdService(binaryPath, projectRoot string) string {
	var envLines strings.Builder
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		if shouldCaptureForLaunchd(key) {
			// systemd requires quoting values that contain special chars.
			// Use double-quotes; escape any existing double-quotes.
			escaped := strings.ReplaceAll(val, `"`, `\"`)
			fmt.Fprintf(&envLines, "Environment=\"%s=%s\"\n", key, escaped)
		}
	}

	logPath := filepath.Join(projectRoot, "Data", "logs", "systemd.log")

	return fmt.Sprintf(`[Unit]
Description=DevTrack Developer Automation
After=default.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=%s start
%sWorkingDirectory=%s
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, binaryPath, envLines.String(), projectRoot, logPath, logPath)
}

// installSystemdService writes the unit file and enables + starts it.
// All current devtrack env vars are embedded as Environment= lines so systemd
// starts the daemon with the correct environment (environment-first config).
func installSystemdService(projectRoot, binaryPath, _ string) error {
	svcContent := buildSystemdService(binaryPath, projectRoot)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	svcDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(svcDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}
	destPath := filepath.Join(svcDir, "devtrack.service")
	if err := os.WriteFile(destPath, []byte(svcContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	for _, args := range [][]string{
		{"--user", "daemon-reload"},
		{"--user", "enable", "devtrack"},
		{"--user", "start", "devtrack"},
	} {
		cmd := exec.Command("systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("systemctl %s failed: %w", strings.Join(args, " "), err)
		}
	}

	fmt.Println("DevTrack systemd user service installed.")
	fmt.Printf("  Unit:    %s\n", destPath)
	fmt.Printf("  Binary:  %s\n", binaryPath)
	fmt.Printf("  Root:    %s\n", projectRoot)
	fmt.Println()
	fmt.Println("DevTrack will now start automatically at login.")
	fmt.Println("Tip: re-run 'devtrack autostart-install' after changing env vars.")
	fmt.Println("Use 'devtrack status' to verify it is running.")
	fmt.Println("Use 'devtrack autostart-uninstall' to remove auto-start.")
	return nil
}

// uninstallSystemdService stops, disables, and removes the unit file.
func uninstallSystemdService() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	svcPath := filepath.Join(homeDir, ".config", "systemd", "user", "devtrack.service")

	if _, err := os.Stat(svcPath); os.IsNotExist(err) {
		fmt.Println("DevTrack systemd user service is not installed.")
		return nil
	}

	for _, args := range [][]string{
		{"--user", "stop", "devtrack"},
		{"--user", "disable", "devtrack"},
	} {
		cmd := exec.Command("systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: systemctl %s returned an error: %v\n", strings.Join(args, " "), err)
		}
	}

	if err := os.Remove(svcPath); err != nil {
		return fmt.Errorf("failed to remove service file %s: %w", svcPath, err)
	}

	// Reload so systemd forgets the unit.
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	fmt.Println("DevTrack systemd user service removed.")
	fmt.Printf("  Removed: %s\n", svcPath)
	fmt.Println()
	fmt.Println("DevTrack will no longer start automatically at login.")
	fmt.Println("The running daemon (if any) was not stopped — use 'devtrack stop' to stop it.")
	return nil
}

// profileShellFile returns the preferred shell profile path for the current
// user (~/.zshrc if it exists, otherwise ~/.bashrc).
func profileShellFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	zshrc := filepath.Join(homeDir, ".zshrc")
	if _, err := os.Stat(zshrc); err == nil {
		return zshrc, nil
	}
	return filepath.Join(homeDir, ".bashrc"), nil
}

const (
	autostartMarkerBegin = "# DevTrack auto-start"
	autostartMarkerEnd   = "# End DevTrack auto-start"
)

// installProfileAutostart appends a startup block to the shell profile.
// The block simply calls 'devtrack start'; env vars must already be set in
// the shell profile (via direnv, manual export, etc.) before this line runs.
func installProfileAutostart(binaryPath, _ string) error {
	profilePath, err := profileShellFile()
	if err != nil {
		return err
	}

	// Read existing profile to check for idempotency.
	existing := ""
	if data, err := os.ReadFile(profilePath); err == nil {
		existing = string(data)
	}
	if strings.Contains(existing, autostartMarkerBegin) {
		fmt.Printf("DevTrack auto-start block already present in %s\n", profilePath)
		fmt.Println("Use 'devtrack autostart-uninstall' first if you want to reinstall.")
		return nil
	}

	block := fmt.Sprintf("\n%s\n%s start 2>/dev/null || true\n%s\n",
		autostartMarkerBegin, binaryPath, autostartMarkerEnd)

	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", profilePath, err)
	}
	defer f.Close()
	if _, err := f.WriteString(block); err != nil {
		return fmt.Errorf("failed to write to %s: %w", profilePath, err)
	}

	fmt.Println("DevTrack auto-start block added.")
	fmt.Printf("  Profile: %s\n", profilePath)
	fmt.Printf("  Binary:  %s\n", binaryPath)
	fmt.Println()
	fmt.Println("DevTrack will start automatically when a new shell session opens.")
	fmt.Printf("Re-source now:  source %s\n", profilePath)
	fmt.Println("Use 'devtrack autostart-uninstall' to remove auto-start.")
	return nil
}

// uninstallProfileAutostart removes the startup block from the shell profile.
func uninstallProfileAutostart() error {
	profilePath, err := profileShellFile()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No shell profile found — nothing to remove.")
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", profilePath, err)
	}

	content := string(data)
	if !strings.Contains(content, autostartMarkerBegin) {
		fmt.Printf("DevTrack auto-start block not found in %s\n", profilePath)
		return nil
	}

	// Remove the block between markers (inclusive).
	scanner := bufio.NewScanner(strings.NewReader(content))
	var lines []string
	inBlock := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, autostartMarkerBegin) {
			inBlock = true
			continue
		}
		if inBlock {
			if strings.Contains(line, autostartMarkerEnd) {
				inBlock = false
			}
			continue
		}
		lines = append(lines, line)
	}

	newContent := strings.Join(lines, "\n")
	// Trim trailing blank lines added by the removal.
	newContent = strings.TrimRight(newContent, "\n") + "\n"

	if err := os.WriteFile(profilePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", profilePath, err)
	}

	fmt.Println("DevTrack auto-start block removed.")
	fmt.Printf("  Profile: %s\n", profilePath)
	fmt.Println()
	fmt.Println("DevTrack will no longer start automatically on new shell sessions.")
	return nil
}

// handleAutostartInstall installs the OS-appropriate auto-start mechanism.
// All required env vars are captured from the current environment and baked
// into the service file / plist at install time.
func (cli *CLI) handleAutostartInstall() error {
	projectRoot, err := resolveProjectRootForAutostart()
	if err != nil {
		return err
	}

	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = os.Args[0]
	}
	binaryPath, _ = filepath.Abs(binaryPath)

	switch detectOSType() {
	case osDarwin:
		return cli.handleLaunchdInstall()
	case osLinuxSystemd, osWSLSystemd:
		return installSystemdService(projectRoot, binaryPath, "")
	case osWSLNoSystemd:
		return installProfileAutostart(binaryPath, "")
	default:
		return cli.handleLaunchdInstall()
	}
}

// handleAutostartUninstall removes the OS-appropriate auto-start mechanism.
func (cli *CLI) handleAutostartUninstall() error {
	switch detectOSType() {
	case osDarwin:
		return cli.handleLaunchdUninstall()
	case osLinuxSystemd, osWSLSystemd:
		return uninstallSystemdService()
	case osWSLNoSystemd:
		return uninstallProfileAutostart()
	default:
		return cli.handleLaunchdUninstall()
	}
}

// handleAutostartStatus shows the status of the OS-appropriate auto-start mechanism.
func (cli *CLI) handleAutostartStatus() error {
	ot := detectOSType()
	switch ot {
	case osDarwin:
		fmt.Println("Auto-start mechanism: launchd (macOS)")
		fmt.Println()
		cmd := exec.Command("launchctl", "list")
		out, err := cmd.Output()
		if err != nil {
			fmt.Println("launchctl list failed — launchd may not be available.")
		} else {
			found := false
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "dev.devtrack") {
					fmt.Printf("  %s\n", strings.TrimSpace(line))
					found = true
				}
			}
			if !found {
				fmt.Println("  Service not registered (devtrack autostart-install to add it).")
			}
		}

	case osLinuxSystemd, osWSLSystemd:
		label := "Linux"
		if ot == osWSLSystemd {
			label = "WSL"
		}
		fmt.Printf("Auto-start mechanism: systemd user service (%s)\n", label)
		fmt.Println()
		cmd := exec.Command("systemctl", "--user", "status", "devtrack")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // non-zero exit when stopped is normal

	case osWSLNoSystemd:
		fmt.Println("Auto-start mechanism: shell profile (WSL without systemd)")
		fmt.Println()
		profilePath, err := profileShellFile()
		if err != nil {
			return err
		}
		data, _ := os.ReadFile(profilePath)
		if strings.Contains(string(data), autostartMarkerBegin) {
			fmt.Printf("  Block present in: %s\n", profilePath)
		} else {
			fmt.Printf("  Block NOT present in: %s\n", profilePath)
			fmt.Println("  Run 'devtrack autostart-install' to add it.")
		}
		fmt.Println()
		// Also show daemon status.
		return cli.handleStatus()
	}

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
	fmt.Println("  uv run python -m backend.git_sage ask '<question>'  Ask git-sage a question")
	fmt.Println("  uv run python -m backend.git_sage do  '<task>'      Let git-sage execute a task")
	fmt.Println("  uv run python -m backend.git_sage interactive        Interactive git-sage session")
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
	fmt.Println("AZURE DEVOPS:")
	fmt.Println("  devtrack azure-check                  Check config and connectivity")
	fmt.Println("  devtrack azure-list                   List work items assigned to you")
	fmt.Println("  devtrack azure-list --all             List all work items (no state filter)")
	fmt.Println("  devtrack azure-list --state <states>  Filter by state (e.g. 'Active,New')")
	fmt.Println("  devtrack azure-view <id>              Show full details for a work item")
	fmt.Println("  devtrack azure-sync                   Full resync (clears cache, fetches all items)")
	fmt.Println("  devtrack azure-sync --full            Explicit full resync")
	fmt.Println("  devtrack azure-sync --hours 24        Only items changed in last 24h (merges)")
	fmt.Println()
	fmt.Println("GITLAB:")
	fmt.Println("  devtrack gitlab-check                    Check GitLab config and connectivity")
	fmt.Println("  devtrack gitlab-list                     List open issues assigned to you")
	fmt.Println("  devtrack gitlab-list --closed            Include closed issues")
	fmt.Println("  devtrack gitlab-list --state <state>     Filter by state (e.g. opened, closed)")
	fmt.Println("  devtrack gitlab-view <project_id> <iid>  Show full details for an issue")
	fmt.Println("  devtrack gitlab-sync                     Full resync (fetches all open issues)")
	fmt.Println("  devtrack gitlab-sync --full              Explicit full resync")
	fmt.Println("  devtrack gitlab-sync --hours 24          Only issues updated in last 24h")
	fmt.Println()
	fmt.Println("GITHUB:")
	fmt.Println("  devtrack github-check                  Check GitHub config and connectivity")
	fmt.Println("  devtrack github-list                   List open issues assigned to you")
	fmt.Println("  devtrack github-list --closed          Include closed issues")
	fmt.Println("  devtrack github-list --state <state>   Filter by state (open, closed, all)")
	fmt.Println("  devtrack github-view <number>          Show full details for an issue")
	fmt.Println("  devtrack github-sync                   Full resync (clears cache, fetches all issues)")
	fmt.Println("  devtrack github-sync --full            Explicit full resync")
	fmt.Println("  devtrack github-sync --hours 24        Only issues updated in last 24h (merges)")
	fmt.Println()
	fmt.Println("TICKET ALERTS (polls GitHub for events relevant to you):")
	fmt.Println("  devtrack alerts          Show unread notifications (last 24h)")
	fmt.Println("  devtrack alerts --all    Show all notifications (no time filter)")
	fmt.Println("  devtrack alerts --clear  Mark all notifications as read")
	fmt.Println("  Configure in .env: ALERT_ENABLED, ALERT_POLL_INTERVAL_SECS,")
	fmt.Println("                     ALERT_GITHUB_ENABLED, ALERT_NOTIFY_ASSIGNED,")
	fmt.Println("                     ALERT_NOTIFY_COMMENTS, ALERT_NOTIFY_REVIEW_REQUESTED")
	fmt.Println()
	fmt.Println("PM AGENT (via Telegram bot):")
	fmt.Println("  /plan <problem>    Decompose a problem into Epic → Story → Task hierarchy")
	fmt.Println("                     Platform picker → LLM preview → confirm to create items")
	fmt.Println("                     Supported platforms: azure, gitlab, github")
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
	fmt.Println("  Bot commands: /status /azure /azureissue /azurecreate")
	fmt.Println("                /gitlab /gitlabissue /gitlabcreate")
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
