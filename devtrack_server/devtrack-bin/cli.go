package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		if cmd == "help" || cmd == "version" {
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

// handleStart starts the daemon
func (cli *CLI) handleStart() error {
	if cli.daemon.IsRunning() {
		pid, _ := cli.daemon.readPID()
		fmt.Printf("❌ Daemon is already running (PID: %d)\n", pid)
		fmt.Println("\nUse 'devtrack status' to see details")
		fmt.Println("Use 'devtrack restart' to restart")
		return nil
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

// handleStatus shows daemon status
func (cli *CLI) handleStatus() error {
	// Handle case where daemon is nil (status check without repo)
	if cli.daemon == nil {
		pidFile := GetPIDFilePath()

		// Check if daemon is running by PID file
		data, err := os.ReadFile(pidFile)
		if err != nil {
			fmt.Println("📊 DevTrack Daemon Status")
			fmt.Println("═════════════════════════")
			fmt.Println()
			fmt.Println("Status:     ❌ STOPPED")
			fmt.Println()
			fmt.Println("Configuration:")
			fmt.Printf("  Config:   %s\n", GetConfigPath())
			fmt.Printf("  Logs:     %s\n", GetLogFilePath())
			fmt.Printf("  PID file: %s\n", pidFile)
			fmt.Println()
			fmt.Println("Commands:")
			fmt.Println("  devtrack start  - Start the daemon")
			return nil
		}

		fmt.Println("📊 DevTrack Daemon Status")
		fmt.Println("═════════════════════════")
		fmt.Println()
		fmt.Printf("Status:     ✅ RUNNING (PID: %s)\n", strings.TrimSpace(string(data)))
		fmt.Println()
		fmt.Println("Use 'devtrack status' from repository directory for full details")
		return nil
	}

	status, err := cli.daemon.Status()
	if err != nil {
		fmt.Printf("❌ Failed to get status: %v\n", err)
		return err
	}

	fmt.Println("📊 DevTrack Daemon Status")
	fmt.Println("═════════════════════════")
	fmt.Println()

	if status.Running {
		fmt.Println("Status:     ✅ RUNNING")
		fmt.Printf("PID:        %d\n", status.PID)

		if !status.StartTime.IsZero() {
			fmt.Printf("Uptime:     %s\n", formatDuration(status.Uptime))
			fmt.Printf("Started:    %s\n", status.StartTime.Format(time.RFC1123))
		}

		if status.TriggerCount > 0 {
			fmt.Printf("Triggers:   %d\n", status.TriggerCount)
		}

		if !status.LastTrigger.IsZero() {
			fmt.Printf("Last:       %s\n", status.LastTrigger.Format(time.RFC1123))
		}
	} else {
		fmt.Println("Status:     ❌ STOPPED")
	}

	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Config:   %s\n", status.ConfigPath)
	fmt.Printf("  Logs:     %s\n", status.LogPath)
	fmt.Printf("  PID file: %s\n", status.PIDPath)
	fmt.Println()

	if status.Running {
		// Show monitoring details
		if cli.daemon.monitor != nil && cli.daemon.monitor.scheduler != nil {
			stats := cli.daemon.monitor.scheduler.GetStats()
			workStatus := cli.daemon.monitor.scheduler.GetWorkHoursStatus()

			fmt.Println("Scheduler:")
			fmt.Printf("  Paused:       %v\n", stats["is_paused"])
			fmt.Printf("  Interval:     %v minutes\n", stats["interval_minutes"])
			fmt.Printf("  Next trigger: %v\n", stats["time_until_next"])

			fmt.Println()
			fmt.Println("Work Hours:")
			fmt.Printf("  Enabled:      %v\n", workStatus["enabled"])
			if workStatus["enabled"].(bool) {
				fmt.Printf("  Hours:        %d:00 - %d:00\n",
					workStatus["work_start_hour"], workStatus["work_end_hour"])
				fmt.Printf("  In hours:     %v\n", workStatus["is_work_hours"])
			}
		}

		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  devtrack stop          - Stop the daemon")
		fmt.Println("  devtrack restart       - Restart the daemon")
		fmt.Println("  devtrack pause         - Pause scheduler")
		fmt.Println("  devtrack resume        - Resume scheduler")
		fmt.Println("  devtrack force-trigger - Force immediate trigger")
		fmt.Println("  devtrack skip-next     - Skip next trigger")
		fmt.Println("  devtrack send-summary  - Generate summary now")
		fmt.Println("  devtrack logs          - View recent logs")
	} else {
		fmt.Println("Commands:")
		fmt.Println("  devtrack start  - Start the daemon")
	}

	return nil
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
	fmt.Printf("Version: %s\n", GetDevTrackVersion())
	fmt.Printf("Build date: %s\n", GetDevTrackBuildDate())
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
	days := GetLearningDefaultDays()
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &days)
	}

	learning := NewLearningCommands()
	return learning.EnableLearning(days)
}

// handleShowProfile shows the learning profile
func (cli *CLI) handleShowProfile() error {
	learning := NewLearningCommands()
	return learning.ShowProfile()
}

// handleTestResponse tests generating a response
func (cli *CLI) handleTestResponse() error {
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
	learning := NewLearningCommands()
	return learning.RevokeConsent()
}

// handleLearningStatus shows learning status
func (cli *CLI) handleLearningStatus() error {
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
	learning := NewLearningCommands()
	return learning.ResetLearning()
}

// handleLearningSetupCron installs the crontab entry from LEARNING_CRON_SCHEDULE
func (cli *CLI) handleLearningSetupCron() error {
	learning := NewLearningCommands()
	return learning.SetupCron()
}

// handleLearningRemoveCron removes the DevTrack learning crontab entry
func (cli *CLI) handleLearningRemoveCron() error {
	learning := NewLearningCommands()
	return learning.RemoveCron()
}

// handleLearningCronStatus shows cron entry status
func (cli *CLI) handleLearningCronStatus() error {
	learning := NewLearningCommands()
	return learning.CronStatus()
}

// handleLearningSync runs a delta (or full) sync immediately
func (cli *CLI) handleLearningSync() error {
	full := len(os.Args) > 2 && os.Args[2] == "--full"
	learning := NewLearningCommands()
	return learning.SyncNow(full)
}

// handlePreviewReport previews today's email report
func (cli *CLI) handlePreviewReport() error {
	date := ""
	if len(os.Args) > 2 {
		date = os.Args[2]
	}

	fmt.Println("📊 Generating daily report preview...")
	fmt.Println()

	scriptPath := GetEmailReporterPath()
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

	scriptPath := GetEmailReporterPath()
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
	date := ""
	if len(os.Args) > 2 {
		date = os.Args[2]
	}

	fmt.Println("💾 Saving report to file...")
	fmt.Println()

	scriptPath := GetEmailReporterPath()
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
	fmt.Println("INFO COMMANDS:")
	fmt.Println("  devtrack logs          Show recent log entries")
	fmt.Println("  devtrack db-stats      Show database statistics")
	fmt.Println("  devtrack stats         Alias for db-stats (with analytics)")
	fmt.Println("  devtrack version       Show version information")
	fmt.Println("  devtrack help          Show this help message")
	fmt.Println()
	fmt.Println("PERSONALIZED AI LEARNING:")
	fmt.Println("  devtrack enable-learning [days]  Enable learning from communications (default 30 days)")
	fmt.Println("  devtrack learning-reset          Wipe all learning data and start fresh")
	fmt.Println("  devtrack learning-sync           Run delta sync now (only new messages since last run)")
	fmt.Println("  devtrack learning-sync --full    Force full re-sync (ignore delta state)")
	fmt.Println("  devtrack learning-status         Show learning status and statistics")
	fmt.Println("  devtrack show-profile            Show learned communication profile")
	fmt.Println("  devtrack test-response <text>    Test generating personalized response")
	fmt.Println("  devtrack revoke-consent          Revoke learning consent and delete data")
	fmt.Println()
	fmt.Println("LEARNING CRON (configure LEARNING_CRON_SCHEDULE in .env):")
	fmt.Println("  devtrack learning-setup-cron     Install/update daily cron entry from .env")
	fmt.Println("  devtrack learning-cron-status    Show cron entry and .env schedule settings")
	fmt.Println("  devtrack learning-remove-cron    Remove the cron entry")
	fmt.Println()
	fmt.Println("EMAIL REPORTS:")
	fmt.Println("  devtrack preview-report [date]   Preview today's report (or YYYY-MM-DD)")
	fmt.Println("  devtrack send-report <email>     Send daily report to email address")
	fmt.Println("  devtrack save-report [date]      Save report to file")
	fmt.Println()
	fmt.Println("TEST COMMANDS:")
	fmt.Println("  go run . test-git         Test Git commit detection")
	fmt.Println("  go run . test-scheduler   Test time-based scheduler")
	fmt.Println("  go run . test-config      Test configuration")
	fmt.Println("  go run . test-integrated  Test complete system")
	fmt.Println()
	fmt.Println("OTHER COMMANDS:")
	fmt.Println("  devtrack version       Show version information")
	fmt.Println("  devtrack help          Show this help message")
	fmt.Println()
	fmt.Println("CONFIGURATION:")
	fmt.Printf("  Config file: %s\n", GetConfigPath())
	fmt.Printf("  Log file:    %s\n", GetLogFilePath())
	fmt.Printf("  PID file:    %s\n", GetPIDFilePath())
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
