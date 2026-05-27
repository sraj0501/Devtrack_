package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

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

	// Ensure local MongoDB/Redis are reachable; auto-start Docker containers if needed.
	// Returns an error only if the user explicitly chose to exit without a working DB.
	if err := EnsureLocalInfra(); err != nil {
		return err
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

	// Detach from parent (platform-specific — see cli_unix.go / cli_windows.go)
	setSetsid(cmd)

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
		} else {
			fmt.Printf("DevTrack Daemon        ● Running (PID: %s)\n", strings.TrimSpace(string(data)))
			fmt.Println()
			fmt.Println("Use 'devtrack status' from repository directory for full details")
		}
		fmt.Println()
		fmt.Println("Config files:")
		if envPath := resolveEnvFilePath(); envPath != "" {
			fmt.Printf("  .env           %s\n", envPath)
		} else {
			fmt.Println("  .env           (not found — run 'devtrack setup')")
		}
		wsPath := GetWorkspacesFilePath()
		if _, err := os.Stat(wsPath); err == nil {
			fmt.Printf("  workspaces     %s\n", wsPath)
		} else {
			fmt.Printf("  workspaces     %s (not found)\n", wsPath)
		}
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

	// Config file locations
	fmt.Println()
	fmt.Println("Config files:")
	if envPath := resolveEnvFilePath(); envPath != "" {
		fmt.Printf("  .env           %s\n", envPath)
	} else {
		fmt.Println("  .env           (not found — run 'devtrack setup')")
	}
	wsPath := GetWorkspacesFilePath()
	if _, err := os.Stat(wsPath); err == nil {
		fmt.Printf("  workspaces     %s\n", wsPath)
	} else {
		fmt.Printf("  workspaces     %s (not found)\n", wsPath)
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
	case "redis":
		return "Redis"
	case "sqlite":
		return "SQLite"
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
		if msg := extractDetailsField(snap.Details, "error"); msg != "" {
			if url := extractDetailsField(snap.Details, "url"); url != "" {
				return fmt.Sprintf("Down — %s (%s)", url, msg)
			}
			return fmt.Sprintf("Down — %s", msg)
		}
		return "Down"
	case "degraded":
		if msg := extractDetailsField(snap.Details, "error"); msg != "" {
			return fmt.Sprintf("Degraded — %s", msg)
		}
		return "Degraded"
	case "unconfigured":
		return "Not configured"
	default:
		return snap.Status
	}
}

// extractDetailsField pulls a single string field out of a JSON details blob.
// Returns "" if the field is missing or the blob isn't valid JSON.
func extractDetailsField(details, field string) string {
	if details == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(details), &m); err != nil {
		return ""
	}
	v, _ := m[field].(string)
	return v
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

	// Platform-specific: Unix sends SIGUSR2; Windows uses HTTP trigger (see cli_unix.go / cli_windows.go)
	if err := sendForceTriggerSignal(process); err != nil {
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

// handleReloadConfig signals the running daemon to reload .env + YAML config
// without restarting. On Unix this sends SIGHUP; on Windows it calls the
// daemon's internal HTTP /internal/reload-config endpoint.
func (cli *CLI) handleReloadConfig() error {
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

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("❌ Could not find daemon process: %v\n", err)
		return err
	}

	fmt.Println("🔄 Reloading config...")

	if err := sendReloadConfigSignal(process); err != nil {
		fmt.Printf("❌ Could not send reload signal to daemon: %v\n", err)
		return err
	}

	fmt.Println("✓ Config reload signal sent")
	fmt.Println("\nCheck logs for reload status:")
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

	fmt.Println("Run 'devtrack logs' or check the SQLite database for today's activity.")
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
