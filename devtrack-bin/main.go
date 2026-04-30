package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	// Auto-load .env before any command processing.
	// Existing env vars (shell exports, CI, secret managers) are never overridden.
	AutoLoadEnv()

	// Check if CLI command is provided
	if len(os.Args) > 1 {
		cmd := os.Args[1]

		// Delegate "git" to devtrack-git-wrapper.sh for AI-enhanced commits
		if cmd == "git" {
			runGitWrapper()
			return
		}

		// Delegate "sage" to the Python git-sage module
		if cmd == "sage" {
			runGitSage()
			return
		}

		// devtrack setup — interactive first-run configuration wizard
		if cmd == "setup" {
			if err := RunSetup(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// devtrack upgrade [--check] — self-update binary and run migrations
		if cmd == "upgrade" {
			checkOnly := len(os.Args) > 2 && os.Args[2] == "--check"
			if err := RunUpgrade(checkOnly); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// devtrack migrate — run any pending config/filesystem migrations
		if cmd == "migrate" {
			RunPendingMigrations()
			return
		}

		// devtrack install — server architecture info
		if cmd == "install" {
			if err := RunInstall(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Handle test commands (but not CLI commands that start with "test-")
		if strings.HasPrefix(cmd, "test-") && cmd != "test-response" {
			RunDemo()
			return
		}

		// Auth / license commands
		if cmd == "login" || cmd == "logout" || cmd == "whoami" ||
			cmd == "license" || cmd == "terms" || cmd == "telemetry" {
			cli, err := NewCLI()
			if err != nil {
				fmt.Printf("Error initializing CLI: %v\n", err)
				os.Exit(1)
			}
			if err := cli.Execute(); err != nil {
				os.Exit(1)
			}
			return
		}

		// Run pending migrations silently on every daemon start.
		// All migrations are idempotent — already-applied ones are skipped instantly.
		if cmd == "start" {
			RunPendingMigrations()
		}

		// Handle daemon commands
		if cmd == "start" || cmd == "stop" || cmd == "restart" ||
			cmd == "status" || cmd == "pause" || cmd == "resume" ||
			cmd == "logs" || cmd == "version" || cmd == "help" ||
			cmd == "db-stats" || cmd == "stats" || cmd == "enable-learning" || cmd == "show-profile" ||
			cmd == "test-response" || cmd == "revoke-consent" || cmd == "learning-status" ||
			cmd == "preview-report" || cmd == "send-report" || cmd == "save-report" ||
			cmd == "force-trigger" || cmd == "send-summary" || cmd == "skip-next" ||
			cmd == "learning-sync" || cmd == "learning-setup-cron" ||
			cmd == "learning-remove-cron" || cmd == "learning-cron-status" ||
			cmd == "learning-reset" ||
			cmd == "commit-queue" || cmd == "commits" || cmd == "queue" ||
			cmd == "telegram-status" || cmd == "azure-check" || cmd == "azure-list" || cmd == "azure-sync" || cmd == "azure-view" || cmd == "settings" ||
			cmd == "workspace" ||
			cmd == "shell-init" || cmd == "is-workspace" || cmd == "enable-git" || cmd == "disable-git" ||
			cmd == "launchd-install" || cmd == "launchd-uninstall" ||
			cmd == "alerts" ||
			cmd == "work" ||
			cmd == "github-check" || cmd == "github-list" || cmd == "github-sync" ||
			cmd == "github-view" || cmd == "github-create" ||
			cmd == "gitlab-check" || cmd == "gitlab-list" || cmd == "gitlab-sync" ||
			cmd == "gitlab-view" || cmd == "gitlab-create" ||
			cmd == "jira-check" || cmd == "jira-list" || cmd == "jira-view" ||
			cmd == "newproject" {
			cli, err := NewCLI()
			if err != nil {
				fmt.Printf("Error initializing CLI: %v\n", err)
				os.Exit(1)
			}

			if err := cli.Execute(); err != nil {
				os.Exit(1)
			}
			return
		}

		// Unknown command - show help
		fmt.Printf("Unknown command: %s\n\n", cmd)
	}

	// No command or unknown command: show help
	printBasicUsage()
}

// printBasicUsage prints help without requiring config (for no-arg or unknown command)
func printBasicUsage() {
	fmt.Println("DevTrack - Developer Automation Tools")
	fmt.Println("======================================")
	fmt.Println()
	fmt.Println("Usage: devtrack <command> [options]")
	fmt.Println()
	fmt.Println("SETUP:      setup                             (first-run configuration wizard)")
	fmt.Println("DAEMON:     start | stop | restart | status")
	fmt.Println("SCHEDULER:  pause | resume | force-trigger | skip-next | send-summary")
	fmt.Println("INFO:       logs | db-stats | stats | version | help")
	fmt.Println("GIT:        git add | git commit -m 'msg'    (AI-enhanced; shell-init required)")
	fmt.Println("SAGE:       sage ask '<question>' | sage do '<task>' | sage interactive")
	fmt.Println("WORKSPACES: workspace list                    (show monitored repos)")
	fmt.Println("            workspace add <name> <path>       (add repo to workspaces.yaml)")
	fmt.Println("            workspace remove <name>           (remove a repo)")
	fmt.Println("            workspace enable|disable <name>   (toggle without removing)")
	fmt.Println("            workspace reload                   (hot-reload running daemon)")
	fmt.Println("COMMITS:    commits pending | commits review")
	fmt.Println("ALERTS:     alerts | alerts --all | alerts --clear")
	fmt.Println("REPORTS:    preview-report | send-report | save-report")
	fmt.Println("ACCOUNT:    login | logout | whoami | license | terms | telemetry [on|off]")
	fmt.Println()
	fmt.Println("UPDATE:     upgrade                          (download latest binary + apply migrations)")
	fmt.Println("            upgrade --check                  (check for updates without installing)")
	fmt.Println()
	fmt.Println("New install? Run: devtrack setup")
	fmt.Println("Run 'devtrack help' for full usage.")
	fmt.Println()
}

// runGitSage delegates to the Python git-sage module, forwarding all args after "sage"
func runGitSage() {
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		execPath, err := os.Executable()
		if err != nil {
			execPath = os.Args[0]
		}
		execPath, _ = filepath.Abs(execPath)
		searchDir := filepath.Dir(execPath)
		for i := 0; i < 6; i++ {
			if _, err := os.Stat(filepath.Join(searchDir, "backend", "git_sage")); err == nil {
				projectRoot = searchDir
				break
			}
			parent := filepath.Dir(searchDir)
			if parent == searchDir {
				break
			}
			searchDir = parent
		}
	}

	if projectRoot == "" {
		fmt.Println("Error: Could not find backend/git_sage directory")
		fmt.Println("Set PROJECT_ROOT to the automation_tools path")
		os.Exit(1)
	}

	// Forward all args after "sage" to the Python module
	sageArgs := append([]string{"run", "python", "-m", "backend.git_sage"}, os.Args[2:]...)

	env := os.Environ()
	env = append(env, "PROJECT_ROOT="+projectRoot)
	env = append(env, "DEVTRACK_ENV_FILE="+filepath.Join(projectRoot, ".env"))

	cmd := exec.Command("uv", sageArgs...)
	cmd.Dir = projectRoot
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

// runGitWrapper execs devtrack-git-wrapper.sh for AI-enhanced git commits
func runGitWrapper() {
	projectRoot := os.Getenv("PROJECT_ROOT")
	// Validate PROJECT_ROOT has the wrapper; if not, search from binary
	if projectRoot != "" {
		if _, err := os.Stat(filepath.Join(projectRoot, "devtrack-git-wrapper.sh")); err != nil {
			projectRoot = ""
		}
	}
	if projectRoot == "" {
		// Find project root by walking up from binary, looking for devtrack-git-wrapper.sh
		execPath, err := os.Executable()
		if err != nil {
			execPath = os.Args[0]
		}
		execPath, _ = filepath.Abs(execPath)
		searchDir := filepath.Dir(execPath)
		for i := 0; i < 6; i++ {
			wrapperPath := filepath.Join(searchDir, "devtrack-git-wrapper.sh")
			if _, err := os.Stat(wrapperPath); err == nil {
				projectRoot = searchDir
				break
			}
			parent := filepath.Dir(searchDir)
			if parent == searchDir {
				break
			}
			searchDir = parent
		}
	}

	if projectRoot == "" {
		fmt.Println("Error: Could not find devtrack-git-wrapper.sh")
		fmt.Println("Set PROJECT_ROOT to automation_tools path, or run: ./devtrack-bin/devtrack git commit -m 'message'")
		os.Exit(1)
	}

	wrapperPath := filepath.Join(projectRoot, "devtrack-git-wrapper.sh")

	// Set PROJECT_ROOT and DEVTRACK_ENV_FILE so wrapper finds .env
	env := os.Environ()
	env = append(env, "PROJECT_ROOT="+projectRoot)
	env = append(env, "DEVTRACK_ENV_FILE="+filepath.Join(projectRoot, ".env"))

	cmd := exec.Command("bash", append([]string{wrapperPath}, os.Args[1:]...)...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
