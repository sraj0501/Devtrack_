package infra

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/pm"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	trigger "github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// TriggerType represents the type of trigger event
type TriggerType string

const (
	TriggerTypeTimer  TriggerType = "timer"
	TriggerTypeCommit TriggerType = "commit"
	TriggerTypeManual TriggerType = "manual"
)

// TriggerEvent represents an event that triggers a prompt
type TriggerEvent struct {
	Type      TriggerType
	Timestamp time.Time
	Source    string
	Data      interface{}
	// Workspace context (populated for commit triggers in multi-repo mode)
	RepoPath      string
	WorkspaceName string
	TicketID      string // extracted from branch name (or "" if unlinked)
	// TicketConfidence reflects the extraction strategy that produced TicketID:
	// 0.95 branch name, 0.85 commit message, 0.60 active-ticket fallback.
	TicketConfidence float64
	// IsMergeToDefault is true when the commit is a merge commit that landed on
	// the repository's default branch — the "merged to main → Done" signal.
	IsMergeToDefault bool
	PMPlatform       string
	PMProject        string
	// Per-workspace PM settings (override global .env defaults)
	PMAssignee      string
	PMIterationPath string
	PMAreaPath      string
	PMMilestone     int
	// PMInProgressLabel is the GitHub/GitLab in-progress label convention
	// (TASK-129); "" = default "in-progress", "none" = disabled.
	PMInProgressLabel string
}

// Scheduler manages time-based triggers and scheduling
type Scheduler struct {
	cron          *cron.Cron
	config        *config.Config
	intervalID    cron.EntryID
	isPaused      bool
	lastTrigger   time.Time
	onTrigger     func(TriggerEvent)
	mu            sync.RWMutex
	stopChan      chan bool
	nextTrigger   time.Time
	triggerCount  int
	pauseDuration time.Duration
}

// NewScheduler creates a new scheduler instance
func NewScheduler(config *config.Config, onTrigger func(TriggerEvent)) *Scheduler {
	c := cron.New(cron.WithSeconds())

	return &Scheduler{
		cron:      c,
		config:    config,
		isPaused:  false,
		onTrigger: onTrigger,
		stopChan:  make(chan bool),
	}
}

// Start begins the scheduler with the configured interval
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return fmt.Errorf("scheduler config is nil")
	}

	// Get interval from config (env-driven)
	intervalMinutes := s.config.Settings.PromptInterval
	if intervalMinutes <= 0 {
		return fmt.Errorf("invalid prompt interval in configuration: %d", intervalMinutes)
	}

	log.Printf("Starting scheduler with %d minute interval", intervalMinutes)

	// Create cron expression for interval
	// Run every N minutes: "0 */N * * * *" (seconds, minutes, hours, day, month, weekday)
	cronExpr := fmt.Sprintf("0 */%d * * * *", intervalMinutes)

	// Add the scheduled job
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.triggerPrompt()
	})

	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.intervalID = entryID
	s.cron.Start()

	// Calculate next trigger time
	s.updateNextTrigger()

	log.Printf("✓ Scheduler started. Next trigger at: %s", s.nextTrigger.Format(time.RFC1123))

	// Optional: auto EOD report at configured hour
	s.scheduleEODReport()

	// Optional: auto-stop idle sessions
	s.scheduleIdleSessionStop()

	// Drain the offline PM update queue periodically
	s.schedulePMQueueFlush()

	// Retry AI enhancement of queued (deferred) commits when the LLM is back
	s.scheduleDeferredEnhance()

	// Background voice sync: poll PM platforms for PR descriptions and issue comments
	s.scheduleVoiceSync()

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
		log.Println("✓ Scheduler stopped")
	}

	close(s.stopChan)
}

// Pause temporarily pauses the scheduler
func (s *Scheduler) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isPaused {
		log.Println("Scheduler is already paused")
		return
	}

	s.isPaused = true
	s.pauseDuration = time.Since(s.lastTrigger)
	log.Println("✓ Scheduler paused")
}

// Resume resumes the scheduler after being paused
func (s *Scheduler) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isPaused {
		log.Println("Scheduler is not paused")
		return
	}

	s.isPaused = false
	s.pauseDuration = 0
	s.updateNextTrigger()
	log.Printf("✓ Scheduler resumed. Next trigger at: %s", s.nextTrigger.Format(time.RFC1123))
}

// IsPaused returns whether the scheduler is currently paused
func (s *Scheduler) IsPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isPaused
}

// GetNextTrigger returns the time of the next scheduled trigger
func (s *Scheduler) GetNextTrigger() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextTrigger
}

// GetTimeUntilNextTrigger returns the duration until the next trigger
func (s *Scheduler) GetTimeUntilNextTrigger() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.isPaused {
		return 0
	}

	return time.Until(s.nextTrigger)
}

// ForceImmediate forces an immediate trigger
func (s *Scheduler) ForceImmediate() {
	log.Println("Forcing immediate trigger")
	// Run asynchronously to avoid blocking
	go s.triggerPrompt()
}

// SkipNext skips the next scheduled trigger
func (s *Scheduler) SkipNext() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Calculate next trigger after the one we're skipping
	intervalMinutes := s.config.Settings.PromptInterval
	if intervalMinutes <= 0 {
		intervalMinutes = 180
	}

	s.nextTrigger = time.Now().Add(time.Duration(intervalMinutes*2) * time.Minute)
	log.Printf("✓ Skipped next trigger. New next trigger at: %s", s.nextTrigger.Format(time.RFC1123))
}

// SetInterval changes the trigger interval (in minutes)
func (s *Scheduler) SetInterval(minutes int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if minutes <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	// Stop current scheduler
	if s.cron != nil {
		s.cron.Remove(s.intervalID)
	}

	// Update config
	s.config.Settings.PromptInterval = minutes

	// Create new cron expression
	cronExpr := fmt.Sprintf("0 */%d * * * *", minutes)

	// Add the new scheduled job
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.triggerPrompt()
	})

	if err != nil {
		return fmt.Errorf("failed to update cron job: %w", err)
	}

	s.intervalID = entryID
	s.updateNextTrigger()

	log.Printf("✓ Interval updated to %d minutes. Next trigger at: %s", minutes, s.nextTrigger.Format(time.RFC1123))

	return nil
}

// GetStats returns scheduler statistics
func (s *Scheduler) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"is_paused":        s.isPaused,
		"trigger_count":    s.triggerCount,
		"last_trigger":     s.lastTrigger,
		"next_trigger":     s.nextTrigger,
		"interval_minutes": s.config.Settings.PromptInterval,
		"time_until_next":  s.GetTimeUntilNextTrigger().String(),
	}
}

// triggerPrompt is called when a scheduled trigger occurs
func (s *Scheduler) triggerPrompt() {
	s.mu.Lock()

	// Check if paused
	if s.isPaused {
		s.mu.Unlock()
		log.Println("Trigger skipped (scheduler is paused)")
		return
	}

	// Check work hours if enabled
	if s.config.Settings.WorkHoursOnly {
		now := time.Now()
		hour := now.Hour()

		if hour < s.config.Settings.WorkStartHour || hour >= s.config.Settings.WorkEndHour {
			s.mu.Unlock()
			log.Printf("Trigger skipped (outside work hours: %d-%d)",
				s.config.Settings.WorkStartHour, s.config.Settings.WorkEndHour)
			return
		}
	}

	s.lastTrigger = time.Now()
	s.triggerCount++
	s.updateNextTrigger()

	event := TriggerEvent{
		Type:      TriggerTypeTimer,
		Timestamp: s.lastTrigger,
		Source:    "scheduler",
		Data: map[string]interface{}{
			"trigger_count":    s.triggerCount,
			"interval_minutes": s.config.Settings.PromptInterval,
		},
	}

	s.mu.Unlock()

	// Call the trigger callback
	if s.onTrigger != nil {
		log.Printf("🔔 Timer trigger #%d at %s", s.triggerCount, s.lastTrigger.Format(time.RFC1123))
		s.onTrigger(event)
	}
}

// updateNextTrigger calculates the next trigger time
func (s *Scheduler) updateNextTrigger() {
	if s.cron == nil {
		return
	}

	entries := s.cron.Entries()
	for _, entry := range entries {
		if entry.ID == s.intervalID {
			s.nextTrigger = entry.Next
			return
		}
	}
}

// IsWorkingHours checks if the current time is within configured work hours
func (s *Scheduler) IsWorkingHours() bool {
	if !s.config.Settings.WorkHoursOnly {
		return true // Always working hours if not restricted
	}

	now := time.Now()
	hour := now.Hour()

	return hour >= s.config.Settings.WorkStartHour && hour < s.config.Settings.WorkEndHour
}

// scheduleEODReport adds a daily cron job at EOD_REPORT_HOUR:EOD_REPORT_MINUTE
// (hour = 0 disables). When triggered it auto-stops any active work session, then
// calls the Python EOD report generator which emails the report to EOD_REPORT_EMAIL
// if set. Config is read via typed accessors in internal/config — no bare os.Getenv.
func (s *Scheduler) scheduleEODReport() {
	hour := config.GetEODReportHour()
	if hour <= 0 {
		return
	}
	minute := config.GetEODReportMinute()

	// "0 M H * * *" fires at H:M:00 every day (cron with seconds)
	cronExpr := fmt.Sprintf("0 %d %d * * *", minute, hour)
	_, err := s.cron.AddFunc(cronExpr, func() {
		log.Printf("⏰ EOD auto-trigger at %02d:%02d", hour, minute)

		// Auto-stop active session
		database, dbErr := db.NewDatabase()
		var eodCommits []trigger.EODCommit
		if dbErr == nil {
			defer database.Close()
			active, _ := database.GetActiveWorkSession()
			if active != nil {
				endedAt := time.Now().UTC().Format("2006-01-02 15:04:05")
				startTime, parseErr := time.Parse("2006-01-02 15:04:05", active.StartedAt)
				durationMins := 0
				if parseErr == nil {
					durationMins = int(time.Since(startTime).Minutes())
					if durationMins < 0 {
						durationMins = 0
					}
				}
				if stopErr := database.EndWorkSession(active.ID, endedAt, durationMins); stopErr == nil {
					log.Printf("✅ Auto-stopped work session #%d for EOD report", active.ID)
				}
			}
			if localCommits, commitsErr := database.ListTodayCommits(""); commitsErr == nil {
				eodCommits = make([]trigger.EODCommit, 0, len(localCommits))
				for _, commit := range localCommits {
					eodCommits = append(eodCommits, trigger.EODCommit{
						TicketID: commit.TicketID, CommitMessage: commit.Message,
						CommitHash: commit.Hash, Timestamp: commit.Timestamp,
					})
				}
			}
		}

		// Send EOD report via server HTTP API.
		recipient := config.GetEODReportEmail()
		trig := trigger.NewHTTPTriggerClient()
		out, reportErr := trig.ReportEOD(recipient, "", eodCommits)
		if reportErr != nil {
			log.Printf("⚠️  EOD report error: %v", reportErr)
		} else {
			log.Printf("✅ EOD report generated%s", func() string {
				if recipient != "" {
					return " and emailed to " + recipient
				}
				return ""
			}())
			if out != "" {
				log.Printf("EOD report output:\n%s", out)
			}
		}
	})
	if err != nil {
		log.Printf("⚠️  Could not schedule EOD report cron: %v", err)
		return
	}
	log.Printf("✓ EOD auto-report scheduled at %02d:%02d daily", hour, minute)
}

// scheduleIdleSessionStop adds a periodic check that auto-stops sessions idle
// for longer than WORK_SESSION_AUTO_STOP_MINUTES (0 = disabled).
// Config is read via the typed accessor in internal/config — no bare os.Getenv.
func (s *Scheduler) scheduleIdleSessionStop() {
	idleMins := config.GetWorkSessionAutoStopMinutes()
	if idleMins <= 0 {
		return
	}

	// Check every minute whether the active session has been idle too long.
	_, err := s.cron.AddFunc("0 * * * * *", func() {
		database, dbErr := db.NewDatabase()
		if dbErr != nil {
			return
		}
		active, fetchErr := database.GetActiveWorkSession()
		if fetchErr != nil || active == nil {
			return
		}

		startTime, parseErr := time.Parse("2006-01-02 15:04:05", active.StartedAt)
		if parseErr != nil {
			return
		}
		elapsedMins := int(time.Since(startTime).Minutes())
		if elapsedMins >= idleMins {
			endedAt := time.Now().UTC().Format("2006-01-02 15:04:05")
			if stopErr := database.EndWorkSession(active.ID, endedAt, elapsedMins); stopErr == nil {
				// Mark auto_stopped flag
				database.Exec("UPDATE work_sessions SET auto_stopped = 1 WHERE id = ?", active.ID) //nolint:errcheck
				log.Printf("⏱️  Work session #%d auto-stopped after %d idle minutes", active.ID, elapsedMins)
			}
		}
	})
	if err != nil {
		log.Printf("⚠️  Could not schedule idle session check: %v", err)
		return
	}
	log.Printf("✓ Work session idle auto-stop enabled (%d minutes)", idleMins)
}

// schedulePMQueueFlush periodically drains the offline PM update queue
// (pm_update_queue), retrying ticket comments that failed to post at commit
// time. Runs every 5 minutes and is silent when there is nothing to send.
func (s *Scheduler) schedulePMQueueFlush() {
	_, err := s.cron.AddFunc("0 */5 * * * *", func() {
		database, dbErr := db.NewDatabase()
		if dbErr != nil {
			return
		}
		defer database.Close()
		sent, failed := pm.FlushQueue(database)
		if sent > 0 || failed > 0 {
			log.Printf("📤 PM queue flush: %d sent, %d still pending", sent, failed)
		}
	})
	if err != nil {
		log.Printf("⚠️  Could not schedule PM queue flush: %v", err)
		return
	}
	log.Printf("✓ PM update queue flusher enabled (every 5 min)")
}

// scheduleDeferredEnhance periodically retries AI enhancement of queued
// (deferred) commits. Commits get queued when the LLM was unreachable at commit
// time; this drains them once the provider is back, promoting them to "enhanced"
// for review. Runs every 30 minutes and is silent when there is nothing pending
// or the provider is still down — so by end of day a reachable LLM will have
// processed the backlog.
func (s *Scheduler) scheduleDeferredEnhance() {
	_, err := s.cron.AddFunc("0 */30 * * * *", func() {
		database, dbErr := db.NewDatabase()
		if dbErr != nil {
			return
		}
		defer database.Close()
		if n, eErr := EnhanceDeferredCommits(database); eErr == nil && n > 0 {
			log.Printf("✨ Enhanced %d queued commit(s) — run 'devtrack commits review'", n)
		}
		// Prune commits left queued past their expiry, removing their snapshot refs.
		if n, xErr := ExpireDeferredCommits(database, config.GetDeferredCommitExpiryHours()); xErr == nil && n > 0 {
			log.Printf("🧹 Expired %d stale queued commit(s) and pruned their snapshot refs", n)
		}
	})
	if err != nil {
		log.Printf("⚠️  Could not schedule deferred-commit enhancer: %v", err)
		return
	}
	log.Printf("✓ Deferred-commit enhancer enabled (every 30 min)")
}

// scheduleVoiceSync adds a periodic cron job that calls POST /voice/sync to
// embed new PR descriptions and issue comments from configured PM platforms
// into ChromaDB (Phase 5 — Tier 1). Interval is read from
// VOICE_SYNC_INTERVAL_HOURS via config.GetVoiceSyncIntervalHours().
// A startup delay of 60 seconds lets the Python server come up before the first
// call fires. Returns immediately (non-blocking) if the interval is 0 or panics.
func (s *Scheduler) scheduleVoiceSync() {
	var intervalHours int
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️  Voice sync scheduler: VOICE_SYNC_INTERVAL_HOURS not set — voice sync disabled (%v)", r)
				intervalHours = 0
			}
		}()
		intervalHours = config.GetVoiceSyncIntervalHours()
	}()

	if intervalHours <= 0 {
		return
	}

	// "0 0 */H * * *" fires every H hours at minute 0.
	cronExpr := fmt.Sprintf("0 0 */%d * * *", intervalHours)

	// Add the recurring job.
	_, err := s.cron.AddFunc(cronExpr, func() {
		log.Printf("🔄 Voice sync: starting background PR/comment sync (interval=%dh)", intervalHours)
		trig := trigger.NewHTTPTriggerClient()
		counts, syncErr := trig.VoiceSync(nil)
		if syncErr != nil {
			log.Printf("⚠️  Voice sync failed: %v", syncErr)
			return
		}
		log.Printf(
			"✅ Voice sync complete: github=%d azure=%d gitlab=%d total=%d",
			counts["github"], counts["azure"], counts["gitlab"], counts["total"],
		)
	})
	if err != nil {
		log.Printf("⚠️  Could not schedule voice sync cron: %v", err)
		return
	}

	// Fire once at startup after a 60-second delay so the Python server has time
	// to come up. This goroutine exits immediately after the one-shot call.
	go func() {
		startupDelay := 60 * time.Second
		log.Printf("✓ Voice sync scheduled every %d hours (first run in %v)", intervalHours, startupDelay)
		time.Sleep(startupDelay)
		log.Printf("🔄 Voice sync: initial startup run")
		trig := trigger.NewHTTPTriggerClient()
		counts, syncErr := trig.VoiceSync(nil)
		if syncErr != nil {
			log.Printf("⚠️  Voice sync startup run failed (non-fatal): %v", syncErr)
			return
		}
		log.Printf(
			"✅ Voice sync startup run complete: github=%d azure=%d gitlab=%d total=%d",
			counts["github"], counts["azure"], counts["gitlab"], counts["total"],
		)
	}()
}

// GetWorkHoursStatus returns current work hours status
func (s *Scheduler) GetWorkHoursStatus() map[string]interface{} {
	now := time.Now()
	hour := now.Hour()
	isWorkHours := s.IsWorkingHours()

	status := map[string]interface{}{
		"enabled":         s.config.Settings.WorkHoursOnly,
		"current_hour":    hour,
		"work_start_hour": s.config.Settings.WorkStartHour,
		"work_end_hour":   s.config.Settings.WorkEndHour,
		"is_work_hours":   isWorkHours,
	}

	if !isWorkHours && s.config.Settings.WorkHoursOnly {
		var nextWorkStart time.Time
		if hour < s.config.Settings.WorkStartHour {
			// Same day
			nextWorkStart = time.Date(now.Year(), now.Month(), now.Day(),
				s.config.Settings.WorkStartHour, 0, 0, 0, now.Location())
		} else {
			// Next day
			tomorrow := now.Add(24 * time.Hour)
			nextWorkStart = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
				s.config.Settings.WorkStartHour, 0, 0, 0, now.Location())
		}
		status["next_work_start"] = nextWorkStart
		status["time_until_work"] = time.Until(nextWorkStart).String()
	}

	return status
}
