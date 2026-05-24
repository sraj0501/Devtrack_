package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// WorkspaceMonitor pairs a GitMonitor with its workspace routing metadata
type WorkspaceMonitor struct {
	gitMonitor      *GitMonitor
	workspaceName   string
	pmPlatform      string
	pmProject       string
	// Per-workspace PM settings
	pmAssignee      string
	pmIterationPath string
	pmAreaPath      string
	pmMilestone     int
	// Filtering
	ignoreBranches  []string // commits on these branches are silently skipped
}

// IntegratedMonitor combines Git monitoring and time-based scheduling
type IntegratedMonitor struct {
	workspaceMonitors    []*WorkspaceMonitor // one per repo (single-repo has exactly one)
	scheduler            *Scheduler
	config               *Config
	database             *Database
	lastActiveWorkspace  *WorkspaceMonitor
	lastActiveWorkspaceMu sync.Mutex
}

// NewIntegratedMonitor creates a new integrated monitoring system.
// repoPath is used as the single workspace when workspaces.yaml is absent.
func NewIntegratedMonitor(repoPath string) (*IntegratedMonitor, error) {
	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create database connection
	database, err := NewDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Build workspace monitors: prefer workspaces.yaml when present
	var workspaceMonitors []*WorkspaceMonitor
	wsCfg, err := LoadWorkspacesConfig()
	if err != nil {
		log.Printf("Warning: failed to load workspaces.yaml: %v (falling back to single-repo mode)", err)
	}

	if wsCfg != nil && len(wsCfg.GetEnabledWorkspaces()) > 0 {
		log.Printf("Multi-repo mode: loading %d workspace(s) from workspaces.yaml", len(wsCfg.GetEnabledWorkspaces()))
		for _, ws := range wsCfg.GetEnabledWorkspaces() {
			gm, err := NewGitMonitor(ws.Path)
			if err != nil {
				log.Printf("Warning: skipping workspace %q (%s): %v", ws.Name, ws.Path, err)
				continue
			}
			workspaceMonitors = append(workspaceMonitors, &WorkspaceMonitor{
				gitMonitor:      gm,
				workspaceName:   ws.Name,
				pmPlatform:      ws.PMPlatform,
				pmProject:       ws.PMProject,
				pmAssignee:      ws.PMAssignee,
				pmIterationPath: ws.PMIterationPath,
				pmAreaPath:      ws.PMAreaPath,
				pmMilestone:     ws.PMMilestone,
				ignoreBranches:  ws.IgnoreBranches,
			})
			log.Printf("  ✓ Workspace %q → %s (platform: %q)", ws.Name, ws.Path, ws.PMPlatform)
		}
		if len(workspaceMonitors) == 0 {
			return nil, fmt.Errorf("workspaces.yaml found but no valid workspaces could be loaded")
		}
	} else {
		// Single-repo backward-compat mode
		log.Printf("Single-repo mode: monitoring %s", repoPath)
		gm, err := NewGitMonitor(repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create git monitor: %w", err)
		}
		workspaceMonitors = []*WorkspaceMonitor{{gitMonitor: gm}}
	}

	// Create integrated monitor
	monitor := &IntegratedMonitor{
		workspaceMonitors: workspaceMonitors,
		config:            config,
		database:          database,
	}

	// Create scheduler with shared trigger handler
	scheduler := NewScheduler(config, monitor.handleTrigger)
	monitor.scheduler = scheduler

	return monitor, nil
}

// Start begins monitoring both Git commits and time-based triggers
func (im *IntegratedMonitor) Start() error {
	log.Println("Starting integrated monitoring system...")

	// Start Git monitor(s) — one per workspace
	for _, ws := range im.workspaceMonitors {
		wsCopy := ws // capture for closure
		if err := wsCopy.gitMonitor.Start(func(commit CommitInfo) {
			im.handleCommitForWorkspace(commit, wsCopy)
		}); err != nil {
			return fmt.Errorf("failed to start git monitor for %q: %w", wsCopy.workspaceName, err)
		}
		label := wsCopy.workspaceName
		if label == "" {
			label = wsCopy.gitMonitor.repoPath
		}
		log.Printf("✓ Git monitor started for %s", label)
	}

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

	// Notify Python of graceful shutdown (best-effort)
	httpClient := NewHTTPTriggerClient()
	if err := httpClient.SendShutdown(); err != nil {
		log.Printf("Could not send HTTP shutdown to Python: %v", err)
	}

	for _, ws := range im.workspaceMonitors {
		if ws.gitMonitor != nil {
			ws.gitMonitor.Stop()
		}
	}

	if im.scheduler != nil {
		im.scheduler.Stop()
	}

	if im.database != nil {
		im.database.Close()
	}

	log.Println("✓ Monitoring stopped")
}

// workspaceKey returns a string that uniquely identifies a workspace's config.
// If the key changes, the monitor must be restarted.
func workspaceKey(ws WorkspaceConfig) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%s",
		ws.Name, ws.Path, ws.PMPlatform, ws.PMProject,
		ws.PMAssignee, ws.PMIterationPath, ws.PMMilestone,
		strings.Join(ws.IgnoreBranches, ","))
}

// ReloadWorkspaces hot-reloads the workspace configuration.
// Diff-based: only restarts monitors for workspaces that were added, removed, or changed.
// Called from the SIGHUP handler in daemon.go and from the workspaces.yaml file watcher.
func (im *IntegratedMonitor) ReloadWorkspaces() {
	newCfg, err := LoadWorkspacesConfig()
	if err != nil {
		log.Printf("Failed to reload workspaces.yaml: %v", err)
		return
	}
	if newCfg == nil {
		log.Println("workspaces.yaml removed — single-repo mode active on restart")
		return
	}

	// Index current monitors by name for fast lookup
	oldByName := make(map[string]*WorkspaceMonitor, len(im.workspaceMonitors))
	for _, wm := range im.workspaceMonitors {
		oldByName[wm.workspaceName] = wm
	}

	// Index desired workspaces by name
	desired := newCfg.GetEnabledWorkspaces()
	desiredByName := make(map[string]WorkspaceConfig, len(desired))
	for _, ws := range desired {
		desiredByName[ws.Name] = ws
	}

	// Stop monitors for removed or changed workspaces
	for name, wm := range oldByName {
		ws, stillWanted := desiredByName[name]
		if !stillWanted || workspaceKey(ws) != fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%s",
			wm.workspaceName, wm.gitMonitor.repoPath, wm.pmPlatform, wm.pmProject,
			wm.pmAssignee, wm.pmIterationPath, wm.pmMilestone,
			strings.Join(wm.ignoreBranches, ",")) {
			log.Printf("Stopping monitor for workspace %q", name)
			wm.gitMonitor.Stop()
			delete(oldByName, name)
		}
	}

	// Build new monitor list: keep unchanged, add new
	var newMonitors []*WorkspaceMonitor
	added, kept := 0, 0
	for _, ws := range desired {
		if existing, ok := oldByName[ws.Name]; ok {
			// Unchanged — reuse without restarting
			newMonitors = append(newMonitors, existing)
			kept++
			continue
		}
		// New or changed workspace — create and start a fresh monitor
		gm, err := NewGitMonitor(ws.Path)
		if err != nil {
			log.Printf("Warning: skipping workspace %q (%s) on reload: %v", ws.Name, ws.Path, err)
			continue
		}
		wm := &WorkspaceMonitor{
			gitMonitor:      gm,
			workspaceName:   ws.Name,
			pmPlatform:      ws.PMPlatform,
			pmProject:       ws.PMProject,
			pmAssignee:      ws.PMAssignee,
			pmIterationPath: ws.PMIterationPath,
			pmAreaPath:      ws.PMAreaPath,
			pmMilestone:     ws.PMMilestone,
			ignoreBranches:  ws.IgnoreBranches,
		}
		wmCopy := wm
		if err := wmCopy.gitMonitor.Start(func(commit CommitInfo) {
			im.handleCommitForWorkspace(commit, wmCopy)
		}); err != nil {
			log.Printf("Warning: failed to start git monitor for %q on reload: %v", ws.Name, err)
			continue
		}
		newMonitors = append(newMonitors, wm)
		added++
	}

	im.workspaceMonitors = newMonitors
	log.Printf("Workspace reload complete: %d active (%d kept, %d added)", len(newMonitors), kept, added)
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

// handleCommitForWorkspace is called when a Git commit is detected on a specific workspace
func (im *IntegratedMonitor) handleCommitForWorkspace(commit CommitInfo, ws *WorkspaceMonitor) {
	// Honour ignore_branches: silently skip commits on branches the user opted out of
	if commit.Branch != "" {
		for _, ignored := range ws.ignoreBranches {
			if strings.EqualFold(commit.Branch, strings.TrimSpace(ignored)) {
				log.Printf("Skipping commit on ignored branch %q for workspace %q", commit.Branch, ws.workspaceName)
				return
			}
		}
	}

	im.lastActiveWorkspaceMu.Lock()
	im.lastActiveWorkspace = ws
	im.lastActiveWorkspaceMu.Unlock()

	event := TriggerEvent{
		Type:            TriggerTypeCommit,
		Timestamp:       commit.Timestamp,
		Source:          "git",
		Data:            commit,
		RepoPath:        ws.gitMonitor.repoPath,
		WorkspaceName:   ws.workspaceName,
		PMPlatform:      ws.pmPlatform,
		PMProject:       ws.pmProject,
		PMAssignee:      ws.pmAssignee,
		PMIterationPath: ws.pmIterationPath,
		PMAreaPath:      ws.pmAreaPath,
		PMMilestone:     ws.pmMilestone,
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

	var triggerRecord TriggerRecord

	// Typed payload vars — populated inside the switch, sent via HTTP
	var commitData *CommitTriggerData
	var timerData *TimerTriggerData

	switch event.Type {
	case TriggerTypeCommit:
		if commit, ok := event.Data.(CommitInfo); ok {
			fmt.Printf("Commit:    %s\n", commit.Hash[:12])
			fmt.Printf("Message:   %s\n", commit.Message)
			fmt.Printf("Author:    %s\n", commit.Author)
			if len(commit.Files) > 0 {
				fmt.Printf("Files:     %d changed\n", len(commit.Files))
			}
			if event.WorkspaceName != "" {
				fmt.Printf("Workspace: %s (%s)\n", event.WorkspaceName, event.PMPlatform)
			}

			cd := CommitTriggerData{
				RepoPath:        event.RepoPath,
				CommitHash:      commit.Hash,
				CommitMessage:   commit.Message,
				Author:          commit.Author,
				Timestamp:       commit.Timestamp.Format(time.RFC3339),
				FilesChanged:    commit.Files,
				Branch:          commit.Branch,
				WorkspaceName:   event.WorkspaceName,
				PMPlatform:      event.PMPlatform,
				PMProject:       event.PMProject,
				PMAssignee:      event.PMAssignee,
				PMIterationPath: event.PMIterationPath,
				PMAreaPath:      event.PMAreaPath,
				PMMilestone:     event.PMMilestone,
			}
			commitData = &cd

			triggerRecord = TriggerRecord{
				TriggerType:   "commit",
				Timestamp:     event.Timestamp,
				Source:        "git",
				RepoPath:      event.RepoPath,
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

			triggerCount := 0
			intervalMins := im.config.Settings.PromptInterval
			if count, ok := data["trigger_count"].(int); ok {
				triggerCount = count
			}

			td := TimerTriggerData{
				Timestamp:    event.Timestamp.Format(time.RFC3339),
				IntervalMins: intervalMins,
				TriggerCount: triggerCount,
			}

			im.lastActiveWorkspaceMu.Lock()
			lastWS := im.lastActiveWorkspace
			im.lastActiveWorkspaceMu.Unlock()
			if lastWS != nil {
				td.WorkspaceName   = lastWS.workspaceName
				td.PMPlatform      = lastWS.pmPlatform
				td.PMProject       = lastWS.pmProject
				td.PMAssignee      = lastWS.pmAssignee
				td.PMIterationPath = lastWS.pmIterationPath
				td.PMAreaPath      = lastWS.pmAreaPath
				td.PMMilestone     = lastWS.pmMilestone
			}

			timerData = &td

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

	// Route trigger to Python backend via HTTPS POST (CS-1: always HTTP).
	httpClient := NewHTTPTriggerClient()
	var sendErr error
	switch {
	case commitData != nil:
		sendErr = httpClient.SendCommitTrigger(*commitData)
	case timerData != nil:
		sendErr = httpClient.SendTimerTrigger(*timerData)
	}
	if sendErr != nil {
		log.Printf("Warning: HTTP trigger failed (%v) — trigger not delivered", sendErr)
	}

	fmt.Println()
	fmt.Println("📝 What happens next:")
	fmt.Println("   1. Python server receives trigger via HTTPS")
	fmt.Println("   2. NLP parse → PM sync (Azure / GitHub / GitLab / Jira)")
	fmt.Println("   3. Timer triggers → Telegram prompt sent to developer")
	fmt.Println("   4. Logged to SQLite database ✓")
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
	status["workspace_count"] = len(im.workspaceMonitors)
	if len(im.workspaceMonitors) > 0 {
		status["repo_path"] = im.workspaceMonitors[0].gitMonitor.repoPath
	}
	if len(im.workspaceMonitors) > 1 {
		paths := make([]string, len(im.workspaceMonitors))
		for i, ws := range im.workspaceMonitors {
			paths[i] = ws.gitMonitor.repoPath
		}
		status["all_repo_paths"] = paths
	}

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
				fmt.Printf("  Workspaces: %v\n", status["workspace_count"])
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
