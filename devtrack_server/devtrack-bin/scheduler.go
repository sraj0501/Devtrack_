package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
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
}

// Scheduler manages time-based triggers and scheduling
type Scheduler struct {
	cron          *cron.Cron
	config        *Config
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
func NewScheduler(config *Config, onTrigger func(TriggerEvent)) *Scheduler {
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
