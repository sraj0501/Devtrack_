package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// IntegratedMonitor combines Git monitoring and time-based scheduling
type IntegratedMonitor struct {
	gitMonitor *GitMonitor
	scheduler  *Scheduler
	config     *Config
	ipcServer  *IPCServer
	database   *Database
}

// NewIntegratedMonitor creates a new integrated monitoring system
func NewIntegratedMonitor(repoPath string) (*IntegratedMonitor, error) {
	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create Git monitor
	gitMonitor, err := NewGitMonitor(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create git monitor: %w", err)
	}

	// Create IPC server
	ipcServer, err := NewIPCServer()
	if err != nil {
		return nil, fmt.Errorf("failed to create IPC server: %w", err)
	}

	// Create database connection
	database, err := NewDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Create integrated monitor
	monitor := &IntegratedMonitor{
		gitMonitor: gitMonitor,
		config:     config,
		ipcServer:  ipcServer,
		database:   database,
	}

	// Register IPC handlers
	monitor.registerIPCHandlers()

	// Create scheduler with shared trigger handler
	scheduler := NewScheduler(config, monitor.handleTrigger)
	monitor.scheduler = scheduler

	return monitor, nil
}

// Start begins monitoring both Git commits and time-based triggers
func (im *IntegratedMonitor) Start() error {
	log.Println("Starting integrated monitoring system...")

	// Start IPC server
	if err := im.ipcServer.Start(); err != nil {
		return fmt.Errorf("failed to start IPC server: %w", err)
	}
	log.Println("✓ IPC server started")

	// Start Git monitor
	if err := im.gitMonitor.Start(im.handleCommit); err != nil {
		return fmt.Errorf("failed to start git monitor: %w", err)
	}
	log.Println("✓ Git monitor started")

	// Start scheduler
	if err := im.scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}
	log.Println("✓ Scheduler started")

	return nil
}

// Stop stops all monitoring
func (im *IntegratedMonitor) Stop() {
	log.Println("Stopping integrated monitoring system...")

	// Send shutdown message to Python
	if im.ipcServer != nil {
		shutdownMsg := IPCMessage{
			Type:      MsgTypeShutdown,
			Timestamp: time.Now(),
			ID:        "shutdown",
			Data:      make(map[string]interface{}),
		}
		im.ipcServer.SendMessage(shutdownMsg)
		time.Sleep(500 * time.Millisecond) // Give Python time to process
		im.ipcServer.Stop()
	}

	if im.gitMonitor != nil {
		im.gitMonitor.Stop()
	}

	if im.scheduler != nil {
		im.scheduler.Stop()
	}

	if im.database != nil {
		im.database.Close()
	}

	log.Println("✓ Monitoring stopped")
}

// registerIPCHandlers registers handlers for IPC messages from Python
func (im *IntegratedMonitor) registerIPCHandlers() {
	// Handle task update responses from Python
	im.ipcServer.RegisterHandler(MsgTypeTaskUpdate, func(msg IPCMessage) error {
		log.Printf("Received task update from Python: %+v", msg.Data)

		// Log to database
		if im.database != nil {
			record := TaskUpdateRecord{
				Timestamp:  time.Now(),
				Project:    getStringFromMap(msg.Data, "project"),
				TicketID:   getStringFromMap(msg.Data, "ticket_id"),
				UpdateText: getStringFromMap(msg.Data, "description"),
				Status:     getStringFromMap(msg.Data, "status"),
				Synced:     getBoolFromMap(msg.Data, "synced"),
				Platform:   "python", // Will be updated when actually synced
			}

			if _, err := im.database.InsertTaskUpdate(record); err != nil {
				log.Printf("Failed to log task update to database: %v", err)
			}
		}

		return nil
	})

	// Handle responses from Python
	im.ipcServer.RegisterHandler(MsgTypeResponse, func(msg IPCMessage) error {
		log.Printf("Received response from Python: %+v", msg.Data)
		return nil
	})

	// Handle errors from Python
	im.ipcServer.RegisterHandler(MsgTypeError, func(msg IPCMessage) error {
		log.Printf("Received error from Python: %s", msg.Error)

		// Log error to database
		if im.database != nil {
			logRecord := LogRecord{
				Timestamp: time.Now(),
				Level:     "error",
				Component: "python_ipc",
				Message:   msg.Error,
			}
			im.database.InsertLog(logRecord)
		}

		return nil
	})

	// Handle acknowledgments from Python
	im.ipcServer.RegisterHandler(MsgTypeAck, func(msg IPCMessage) error {
		log.Printf("Received ACK from Python for message: %s", msg.ID)
		return nil
	})
}

// Helper functions to extract values from map[string]interface{}
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getBoolFromMap(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// handleCommit is called when a Git commit is detected
func (im *IntegratedMonitor) handleCommit(commit CommitInfo) {
	event := TriggerEvent{
		Type:      TriggerTypeCommit,
		Timestamp: commit.Timestamp,
		Source:    "git",
		Data:      commit,
	}

	im.handleTrigger(event)
}

// handleTrigger is the unified trigger handler for both Git and timer events
func (im *IntegratedMonitor) handleTrigger(event TriggerEvent) {
	fmt.Println("\n" + string('═') + strings.Repeat("═", 60))
	fmt.Printf("🎯 TRIGGER EVENT: %s\n", event.Type)
	fmt.Println(string('═') + strings.Repeat("═", 60))
	fmt.Printf("Timestamp: %s\n", event.Timestamp.Format(time.RFC1123))
	fmt.Printf("Source:    %s\n", event.Source)

	var ipcMsg IPCMessage
	var triggerRecord TriggerRecord

	switch event.Type {
	case TriggerTypeCommit:
		if commit, ok := event.Data.(CommitInfo); ok {
			fmt.Printf("Commit:    %s\n", commit.Hash[:12])
			fmt.Printf("Message:   %s\n", commit.Message)
			fmt.Printf("Author:    %s\n", commit.Author)
			if len(commit.Files) > 0 {
				fmt.Printf("Files:     %d changed\n", len(commit.Files))
			}

			// Create IPC message for commit trigger
			ipcMsg = CreateCommitTriggerMessage(CommitTriggerData{
				RepoPath:      im.gitMonitor.repoPath,
				CommitHash:    commit.Hash,
				CommitMessage: commit.Message,
				Author:        commit.Author,
				Timestamp:     commit.Timestamp.Format(time.RFC3339),
				FilesChanged:  commit.Files,
				Branch:        "", // Branch info not available in CommitInfo
			})

			// Prepare database record
			triggerRecord = TriggerRecord{
				TriggerType:   "commit",
				Timestamp:     event.Timestamp,
				Source:        "git",
				RepoPath:      im.gitMonitor.repoPath,
				CommitHash:    commit.Hash,
				CommitMessage: commit.Message,
				Author:        commit.Author,
				Processed:     false,
			}
		}

	case TriggerTypeTimer:
		if data, ok := event.Data.(map[string]interface{}); ok {
			if count, ok := data["trigger_count"].(int); ok {
				fmt.Printf("Trigger #:  %d\n", count)
			}
			if interval, ok := data["interval_minutes"].(int); ok {
				fmt.Printf("Interval:   %d minutes\n", interval)
			}

			// Create IPC message for timer trigger
			triggerCount := 0
			intervalMins := im.config.Settings.PromptInterval
			if count, ok := data["trigger_count"].(int); ok {
				triggerCount = count
			}

			ipcMsg = CreateTimerTriggerMessage(TimerTriggerData{
				Timestamp:    event.Timestamp.Format(time.RFC3339),
				IntervalMins: intervalMins,
				TriggerCount: triggerCount,
			})

			// Prepare database record
			triggerRecord = TriggerRecord{
				TriggerType: "timer",
				Timestamp:   event.Timestamp,
				Source:      "scheduler",
				Processed:   false,
			}
		}
	}

	// Log trigger to database
	if im.database != nil {
		triggerID, err := im.database.InsertTrigger(triggerRecord)
		if err != nil {
			log.Printf("Failed to log trigger to database: %v", err)
		} else {
			log.Printf("✓ Logged trigger to database (ID: %d)", triggerID)
		}
	}

	// Send IPC message to Python
	if im.ipcServer != nil {
		if err := im.ipcServer.SendMessage(ipcMsg); err != nil {
			log.Printf("Failed to send IPC message: %v", err)
		} else {
			log.Println("✓ Sent trigger to Python via IPC")
		}
	}

	fmt.Println()
	fmt.Println("📝 What happens next:")
	fmt.Println("   1. Python receives trigger via IPC")
	fmt.Println("   2. Prompt user: 'What did you work on?'")
	fmt.Println("   3. Parse text with NLP (spaCy)")
	fmt.Println("   4. Match to existing tasks (semantic matching)")
	fmt.Println("   5. Update Azure DevOps / GitHub / JIRA")
	fmt.Println("   6. Logged to SQLite database ✓")
	fmt.Println("   7. Generate email report at EOD")
	fmt.Println()
	fmt.Println("⏳ Waiting for next event...")
	fmt.Println()
}

// GetStatus returns the current monitoring status
func (im *IntegratedMonitor) GetStatus() map[string]interface{} {
	status := make(map[string]interface{})

	// Scheduler status
	if im.scheduler != nil {
		status["scheduler"] = im.scheduler.GetStats()
		status["work_hours"] = im.scheduler.GetWorkHoursStatus()
	}

	// Git monitor status
	status["git_monitoring"] = true
	status["repo_path"] = im.gitMonitor.repoPath

	return status
}

// TestIntegrated demonstrates the complete integrated system
func TestIntegrated() {
	fmt.Println("🚀 Integrated Monitoring System Test")
	fmt.Println("====================================")
	fmt.Println()

	// Get current directory as repository path
	repoPath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Go up one directory to the project root
	repoPath = filepath.Dir(repoPath)
	fmt.Printf("Repository: %s\n", repoPath)

	// Create integrated monitor
	monitor, err := NewIntegratedMonitor(repoPath)
	if err != nil {
		log.Fatalf("Failed to create integrated monitor: %v", err)
	}
	defer monitor.Stop()

	// Set short interval for testing
	fmt.Println("Setting scheduler interval to 2 minutes for testing...")
	monitor.scheduler.SetInterval(2)
	fmt.Println()

	// Start monitoring
	if err := monitor.Start(); err != nil {
		log.Fatalf("Failed to start monitoring: %v", err)
	}

	fmt.Println()
	fmt.Println("✅ SYSTEM ACTIVE")
	fmt.Println("================")
	fmt.Println()
	fmt.Println("The system will now trigger on:")
	fmt.Println("  • Git commits (make a commit to test)")
	fmt.Println("  • Every 2 minutes (timer)")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  p - Pause scheduler")
	fmt.Println("  r - Resume scheduler")
	fmt.Println("  f - Force trigger now")
	fmt.Println("  s - Show status")
	fmt.Println("  q - Quit")
	fmt.Println()

	// Command handler
	go func() {
		reader := make([]byte, 1)
		for {
			os.Stdin.Read(reader)
			cmd := string(reader[0])

			switch cmd {
			case "p", "P":
				monitor.scheduler.Pause()
			case "r", "R":
				monitor.scheduler.Resume()
			case "f", "F":
				monitor.scheduler.ForceImmediate()
			case "s", "S":
				status := monitor.GetStatus()
				fmt.Println("\n📊 System Status:")
				fmt.Println("─────────────────")

				if schedStats, ok := status["scheduler"].(map[string]interface{}); ok {
					fmt.Printf("Scheduler:\n")
					fmt.Printf("  Paused: %v\n", schedStats["is_paused"])
					fmt.Printf("  Triggers: %v\n", schedStats["trigger_count"])
					fmt.Printf("  Interval: %v min\n", schedStats["interval_minutes"])
					fmt.Printf("  Next: %v\n", schedStats["time_until_next"])
				}

				fmt.Printf("\nGit Monitoring:\n")
				fmt.Printf("  Active: %v\n", status["git_monitoring"])
				fmt.Printf("  Repo: %v\n", status["repo_path"])
				fmt.Println()
			case "q", "Q":
				return
			}
		}
	}()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n\n✓ Shutting down...")
}
