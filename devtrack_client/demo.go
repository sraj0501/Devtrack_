package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// TestGitMonitor demonstrates the Git monitoring functionality
func TestGitMonitor() {
	fmt.Println("🚀 Git Monitor Test")
	fmt.Println("==================")

	// Get current directory as repository path
	repoPath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Go up one directory to the project root
	repoPath = filepath.Dir(repoPath)
	fmt.Printf("Monitoring repository: %s\n\n", repoPath)

	// Check if it's a git repository
	if !IsGitRepository(repoPath) {
		log.Fatalf("Not a git repository: %s", repoPath)
	}

	// Create Git monitor
	monitor, err := NewGitMonitor(repoPath)
	if err != nil {
		log.Fatalf("Failed to create Git monitor: %v", err)
	}
	defer monitor.Stop()

	// Install post-commit hook
	if err := InstallPostCommitHook(repoPath); err != nil {
		log.Printf("Warning: could not install post-commit hook: %v", err)
	}

	fmt.Println("✓ Git monitor initialized")
	fmt.Println("✓ Watching for commits...")
	fmt.Println("\nMake a commit in another terminal to see it detected!")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Start monitoring with callback
	err = monitor.Start(func(commit CommitInfo) {
		fmt.Println("\n🎉 NEW COMMIT DETECTED!")
		fmt.Println("═══════════════════════")
		fmt.Printf("Hash:      %s\n", commit.Hash[:12])
		fmt.Printf("Author:    %s\n", commit.Author)
		fmt.Printf("Timestamp: %s\n", commit.Timestamp.Format(time.RFC1123))
		fmt.Printf("Message:   %s\n", commit.Message)

		if len(commit.Files) > 0 {
			fmt.Printf("Files:     ")
			if len(commit.Files) <= 3 {
				for i, file := range commit.Files {
					if i > 0 {
						fmt.Print(", ")
					}
					fmt.Print(file)
				}
				fmt.Println()
			} else {
				fmt.Printf("%d files changed\n", len(commit.Files))
			}
		}

		fmt.Println("\n👉 Next steps:")
		fmt.Println("   • Prompt user: 'What did you work on?'")
		fmt.Println("   • Parse response with NLP")
		fmt.Println("   • Update Azure DevOps/GitHub")
		fmt.Println("   • Log to database")
		fmt.Println()
	})

	if err != nil {
		log.Fatalf("Failed to start Git monitor: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n\n✓ Shutting down Git monitor...")
}

// TestConfig demonstrates the configuration functionality
func TestConfig() {
	fmt.Println("⚙️  Configuration Test")
	fmt.Println("=====================")

	// Load or create config
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Config version: %s\n", config.Version)
	fmt.Printf("Config path: %s\n\n", GetConfigPath())

	fmt.Println("Settings:")
	fmt.Printf("  Prompt Interval: %d minutes\n", config.Settings.PromptInterval)
	fmt.Printf("  Work Hours Only: %v\n", config.Settings.WorkHoursOnly)
	fmt.Printf("  Timezone: %s\n", config.Settings.Timezone)
	fmt.Printf("  Auto Sync: %v\n\n", config.Settings.AutoSync)

	fmt.Println("Notifications:")
	fmt.Printf("  Output Type: %s\n", config.Settings.Notifications.OutputType)
	fmt.Printf("  Daily Report: %v at %s\n",
		config.Settings.Notifications.SendDailySummary,
		config.Settings.Notifications.DailyReportTime)

	if config.Settings.Notifications.Email.Enabled {
		fmt.Println("\n  Email Settings:")
		fmt.Printf("    Enabled: %v\n", config.Settings.Notifications.Email.Enabled)
		fmt.Printf("    Recipients: %v\n", config.Settings.Notifications.Email.ToAddresses)
		fmt.Printf("    Manager: %s\n", config.Settings.Notifications.Email.ManagerEmail)
	}

	if config.Settings.Notifications.Teams.Enabled {
		fmt.Println("\n  Teams Settings:")
		fmt.Printf("    Enabled: %v\n", config.Settings.Notifications.Teams.Enabled)
		fmt.Printf("    Type: %s\n", config.Settings.Notifications.Teams.ChatType)
		fmt.Printf("    Channel: %s\n", config.Settings.Notifications.Teams.ChannelName)
	}

	fmt.Println()

	fmt.Println("Repositories:")
	for _, repo := range config.Repositories {
		status := "✓ enabled"
		if !repo.Enabled {
			status = "✗ disabled"
		}
		fmt.Printf("  %s %s\n", status, repo.Name)
		fmt.Printf("    Path: %s\n", repo.Path)
		fmt.Printf("    Project: %s\n", repo.Project)
	}

	fmt.Println("\nIntegrations:")
	fmt.Printf("  Azure DevOps: %v\n", config.Integrations.AzureDevOps.Enabled)
	fmt.Printf("  GitHub: %v\n", config.Integrations.GitHub.Enabled)
	fmt.Printf("  JIRA: %v\n", config.Integrations.JIRA.Enabled)
}

// TestScheduler demonstrates the scheduler functionality
func TestScheduler() {
	fmt.Println("⏰ Scheduler Test")
	fmt.Println("=================")

	// Load config
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set short interval for testing (1 minute)
	fmt.Println("Setting interval to 1 minute for testing...")
	config.Settings.PromptInterval = 1

	// Create scheduler with callback
	scheduler := NewScheduler(config, func(event TriggerEvent) {
		fmt.Println("\n🔔 TRIGGER EVENT!")
		fmt.Println("════════════════")
		fmt.Printf("Type:      %s\n", event.Type)
		fmt.Printf("Timestamp: %s\n", event.Timestamp.Format(time.RFC1123))
		fmt.Printf("Source:    %s\n", event.Source)

		if data, ok := event.Data.(map[string]interface{}); ok {
			if count, ok := data["trigger_count"].(int); ok {
				fmt.Printf("Count:     %d\n", count)
			}
		}

		fmt.Println("\n👉 Next steps:")
		fmt.Println("   • Prompt user: 'What have you been working on?'")
		fmt.Println("   • Send to Python NLP parser")
		fmt.Println("   • Update project management tools")
		fmt.Println()
	})

	// Start scheduler
	if err := scheduler.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	fmt.Println("\n✓ Scheduler started")
	fmt.Println("✓ Triggers every 1 minute (testing mode)")
	fmt.Println()

	// Display work hours info
	workStatus := scheduler.GetWorkHoursStatus()
	fmt.Println("Work Hours Configuration:")
	fmt.Printf("  Enabled: %v\n", workStatus["enabled"])
	if workStatus["enabled"].(bool) {
		fmt.Printf("  Hours: %d:00 - %d:00\n", workStatus["work_start_hour"], workStatus["work_end_hour"])
		fmt.Printf("  Currently in work hours: %v\n", workStatus["is_work_hours"])
	}
	fmt.Println()

	// Interactive commands
	fmt.Println("Commands:")
	fmt.Println("  p - Pause scheduler")
	fmt.Println("  r - Resume scheduler")
	fmt.Println("  f - Force immediate trigger")
	fmt.Println("  s - Skip next trigger")
	fmt.Println("  i - Show stats")
	fmt.Println("  q - Quit")
	fmt.Println()

	// Command loop
	go func() {
		reader := make([]byte, 1)
		for {
			os.Stdin.Read(reader)
			cmd := string(reader[0])

			switch cmd {
			case "p", "P":
				scheduler.Pause()
			case "r", "R":
				scheduler.Resume()
			case "f", "F":
				scheduler.ForceImmediate()
			case "s", "S":
				scheduler.SkipNext()
			case "i", "I":
				stats := scheduler.GetStats()
				fmt.Println("\n📊 Scheduler Stats:")
				fmt.Printf("  Paused: %v\n", stats["is_paused"])
				fmt.Printf("  Trigger count: %v\n", stats["trigger_count"])
				fmt.Printf("  Interval: %v minutes\n", stats["interval_minutes"])
				fmt.Printf("  Time until next: %v\n", stats["time_until_next"])
				if !stats["is_paused"].(bool) {
					fmt.Printf("  Next trigger: %v\n", stats["next_trigger"].(time.Time).Format(time.RFC1123))
				}
				fmt.Println()
			case "q", "Q":
				fmt.Println("\nStopping scheduler...")
				return
			}
		}
	}()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n✓ Shutting down scheduler...")
}

// RunDemo runs the various test demos
func RunDemo() {
	if len(os.Args) > 1 && os.Args[1] == "test-git" {
		TestGitMonitor()
	} else if len(os.Args) > 1 && os.Args[1] == "test-config" {
		TestConfig()
	} else if len(os.Args) > 1 && os.Args[1] == "test-scheduler" {
		TestScheduler()
	} else if len(os.Args) > 1 && os.Args[1] == "test-integrated" {
		TestIntegrated()
	} else {
		fmt.Println("Developer Automation Tools - Demo")
		fmt.Println("==================================")
		fmt.Println()
		fmt.Println("Available commands:")
		fmt.Println("  go run . test-git         - Test Git commit detection")
		fmt.Println("  go run . test-config      - Test configuration system")
		fmt.Println("  go run . test-scheduler   - Test time-based scheduler")
		fmt.Println("  go run . test-integrated  - Test complete system (Git + Scheduler)")
		fmt.Println()
	}
}
