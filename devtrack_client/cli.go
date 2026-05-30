package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gitsage "github.com/sraj0501/Devtrack_/devtrack_client/gitsage"
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
			cmd == "login" || cmd == "logout" || cmd == "whoami" || cmd == "license" || cmd == "terms" || cmd == "telemetry" ||
			cmd == "reload-config" || cmd == "plan" || cmd == "boardroom" || cmd == "sage" ||
			cmd == "azure-list" || cmd == "azure-sync" || cmd == "azure-view" ||
			cmd == "gitlab-list" || cmd == "gitlab-sync" || cmd == "gitlab-view" ||
			cmd == "github-list" || cmd == "github-sync" || cmd == "github-view" {
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
	case "reload-config":
		return cli.handleReloadConfig()
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
	case "sage":
		return cli.handleSage()
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
	case "init":
		return cli.handleInit()
	case "plan":
		return cli.handlePlan()
	case "boardroom":
		return cli.handleBoardroom()
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
// current server mode does not include a local Python backend.
func requiresManagedMode(command string) error {
	if IsExternalServer() {
		url := GetServerURL()
		if url == "" {
			fmt.Printf("'%s' requires a Python backend server.\n", command)
			fmt.Println("Either re-run 'devtrack setup' and choose Managed mode,")
			fmt.Println("or set DEVTRACK_SERVER_URL to an external DevTrack server.")
			return fmt.Errorf("command unavailable: no server configured")
		}
	}
	return nil
}

// handleSage dispatches git-sage subcommands.
//
// Usage:
//
//	devtrack sage ask "<question>"       — one-shot Q&A about the repository
//	devtrack sage do "<task>" [--verbose] — agentic task execution with approval dialog
//	devtrack sage pr                     — show current branch PR info
//	devtrack sage interactive            — explicit interactive multi-turn chat
//	devtrack sage                        — interactive multi-turn chat
func (cli *CLI) handleSage() error {
	repoPath, err := os.Getwd()
	if err != nil {
		repoPath = "."
	}

	args := os.Args
	if len(args) < 3 {
		return gitsage.RunInteractive(repoPath)
	}

	sub := args[2]
	switch sub {
	case "ask":
		if len(args) < 4 {
			fmt.Println("Usage: devtrack sage ask \"<question>\"")
			return fmt.Errorf("missing question")
		}
		question := strings.Join(args[3:], " ")
		return gitsage.RunAsk(repoPath, question)

	case "do":
		if len(args) < 4 {
			fmt.Println("Usage: devtrack sage do \"<task>\"")
			return fmt.Errorf("missing task")
		}
		// Strip --verbose flag; pass remaining tokens as task
		verbose := false
		var taskParts []string
		for _, a := range args[3:] {
			if a == "--verbose" || a == "-v" {
				verbose = true
			} else {
				taskParts = append(taskParts, a)
			}
		}
		task := strings.Join(taskParts, " ")
		return gitsage.RunDoVerbose(repoPath, task, verbose)

	case "pr":
		info, err := gitsage.FindPR(repoPath)
		if err != nil {
			return fmt.Errorf("sage pr: %w", err)
		}
		fmt.Println(info.Format())
		return nil

	case "interactive":
		return gitsage.RunInteractive(repoPath)

	default:
		// Treat anything else as a question
		question := strings.Join(args[2:], " ")
		return gitsage.RunAsk(repoPath, question)
	}
}

// handleAlerts shows ticket alert notifications or marks them as read.
// Reads directly from SQLite — no Python subprocess required.
//
// Usage:
//
//	devtrack alerts           — show unread notifications
//	devtrack alerts --all     — show all notifications (read + unread)
//	devtrack alerts --clear   — mark all as read
func (cli *CLI) handleAlerts() error {
	d, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}
	defer d.Close()

	showAll := false
	markRead := false
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--all":
			showAll = true
		case "--clear":
			markRead = true
		}
	}

	if markRead {
		if err := d.MarkAllNotificationsRead(); err != nil {
			return fmt.Errorf("could not mark notifications read: %w", err)
		}
		fmt.Println("All notifications marked as read.")
		return nil
	}

	var records []NotificationRecord
	if showAll {
		records, err = d.GetAllNotifications(100)
	} else {
		records, err = d.GetUnreadNotifications(50)
	}
	if err != nil {
		return fmt.Errorf("could not fetch notifications: %w", err)
	}

	if len(records) == 0 {
		if showAll {
			fmt.Println("No notifications found.")
		} else {
			fmt.Println("No unread notifications. (Use --all to see everything)")
		}
		return nil
	}

	fmt.Printf("\n  %-8s %-20s %-16s %s\n", "Source", "Event", "Time", "Title")
	fmt.Println("  " + strings.Repeat("-", 72))
	for _, r := range records {
		dot := "○"
		if !r.Read {
			dot = "●"
		}
		ts := r.CreatedAt.Format("01/02 15:04")
		title := r.Title
		if len(title) > 38 {
			title = title[:35] + "..."
		}
		fmt.Printf("  %s %-8s %-20s %-16s %s\n", dot, r.Source, r.EventType, ts, title)
		if r.URL != "" {
			fmt.Printf("    %s\n", r.URL)
		}
	}
	fmt.Printf("\n  %d notification(s)\n", len(records))
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

// handleInit runs one-time DevTrack initialisation for the current repository.
// It warms the ticket cache via the Go-native GitHub sync (connectors/github,
// SQLite) — no Python backend involved. Sync failures are non-fatal — init
// always succeeds.
func (cli *CLI) handleInit() error {
	if os.Getenv("GITHUB_TOKEN") == "" {
		fmt.Println("GITHUB_TOKEN not set — skipping ticket sync")
		return nil
	}

	fmt.Println("Syncing tickets from GitHub...")

	// Reuse the Go-native sync (writes to the github_issues SQLite table).
	if err := cli.handleGitHubSync(); err != nil {
		fmt.Printf("Ticket sync failed (non-fatal): %v\n", err)
		// Do not return an error — init continues regardless of sync outcome.
	} else {
		fmt.Println("Ticket sync complete.")
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
