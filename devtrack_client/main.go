// Windows binary icon and version info.
// Requires: go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
// Regenerate resource.syso after updating versioninfo.json or devtrack.ico:
//
//go:generate goversioninfo -64 -o resource.syso versioninfo.json

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	gitsage "github.com/sraj0501/Devtrack_/devtrack_client/gitsage"
)

func main() {
	// Auto-load .env before any command processing.
	// Existing env vars (shell exports, CI, secret managers) are never overridden.
	AutoLoadEnv()

	// Propagate the ldflags-injected version into internal/config so that
	// GetDevTrackVersion() returns the correct value everywhere.
	SetBuildVersion(Version)

	// Check if CLI command is provided
	if len(os.Args) > 1 {
		cmd := os.Args[1]

		// Go-native AI-enhanced git commands (no Python/bash wrapper dependency).
		if cmd == "git" {
			repoPath, err := os.Getwd()
			if err != nil || repoPath == "" {
				repoPath = "."
			}
			if err := gitsage.RunGit(repoPath, os.Args[2:], gitCommitHooks()); err != nil {
				var ece *gitsage.ExitCodeError
				if errors.As(err, &ece) {
					os.Exit(ece.Code)
				}
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
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

		// devtrack uninstall [--yes] [--keep-data]
		if cmd == "uninstall" {
			keepData := false
			yes := false
			for _, arg := range os.Args[2:] {
				switch arg {
				case "--keep-data":
					keepData = true
				case "--yes", "-y":
					yes = true
				}
			}
			if err := RunUninstall(keepData, yes); err != nil {
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
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
			cmd == "sage" ||
			cmd == "work" ||
			cmd == "github-check" || cmd == "github-list" || cmd == "github-sync" || cmd == "github-view" ||
			cmd == "gitlab-check" || cmd == "gitlab-list" || cmd == "gitlab-sync" || cmd == "gitlab-view" ||
			cmd == "ticket-sync" || cmd == "narrative" ||
			cmd == "newproject" {
			cli, err := NewCLI()
			if err != nil {
				fmt.Printf("Error initializing CLI: %v\n", err)
				os.Exit(1)
			}

			if err := cli.Execute(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
	fmt.Println("SETUP:      setup                              first-run configuration wizard")
	fmt.Println("DAEMON:     start | stop | restart | status")
	fmt.Println("SCHEDULER:  pause | resume | force-trigger | skip-next | send-summary")
	fmt.Println("INFO:       logs | db-stats | stats | version | settings | help")
	fmt.Println()
	fmt.Println("GIT:        git add | git commit -m 'msg'     AI-enhanced; shell-init required")
	fmt.Println("SAGE:       sage ask '<question>'              one-shot Q&A about the repo")
	fmt.Println("            sage do '<task>' [--verbose]       agentic task execution")
	fmt.Println("            sage pr                            show current branch PR info")
	fmt.Println("            sage interactive                   multi-turn chat")
	fmt.Println()
	fmt.Println("GITHUB:     github-check | github-list | github-sync | github-view <number>")
	fmt.Println("GITLAB:     gitlab-check | gitlab-list | gitlab-sync | gitlab-view <proj> <iid>")
	fmt.Println("AZURE:      azure-check  | azure-list  | azure-sync  | azure-view <id>")
	fmt.Println("            (use devtrack help for filter flags and full options)")
	fmt.Println()
	fmt.Println("WORKSPACES: workspace list | add <name> <path> | remove <name>")
	fmt.Println("            workspace enable|disable <name> | workspace reload")
	fmt.Println("COMMITS:    commits pending | commits review")
	fmt.Println("ALERTS:     alerts | alerts --all | alerts --clear")
	fmt.Println("REPORTS:    preview-report | send-report | save-report")
	fmt.Println("CLOUD:      cloud login --url URL --key KEY    connect to external server")
	fmt.Println("            cloud status | cloud logout")
	fmt.Println("ACCOUNT:    login | logout | whoami | license | terms | telemetry [on|off]")
	fmt.Println()
	fmt.Println("UPDATE:     upgrade | upgrade --check")
	fmt.Println("UNINSTALL:  uninstall | uninstall --keep-data")
	fmt.Println()
	fmt.Println("New install? Run: devtrack setup")
	fmt.Println("Run 'devtrack help' for full usage and flags.")
	fmt.Println()
}

