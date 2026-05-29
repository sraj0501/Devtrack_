package main

import (
	"fmt"
	"os"
	"strings"
)

// handleEnableLearning enables personalized AI learning
func (cli *CLI) handleEnableLearning() error {
	if err := requiresManagedMode("enable-learning"); err != nil {
		return err
	}
	days := GetLearningDefaultDays()
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &days)
	}

	learning := NewLearningCommands()
	return learning.EnableLearning(days)
}

// handleShowProfile shows the learning profile
func (cli *CLI) handleShowProfile() error {
	if err := requiresManagedMode("show-profile"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.ShowProfile()
}

// handleTestResponse tests generating a response
func (cli *CLI) handleTestResponse() error {
	if err := requiresManagedMode("test-response"); err != nil {
		return err
	}
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
	if err := requiresManagedMode("revoke-consent"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.RevokeConsent()
}

// handleLearningStatus shows learning status
func (cli *CLI) handleLearningStatus() error {
	if err := requiresManagedMode("learning-status"); err != nil {
		return err
	}
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
	if err := requiresManagedMode("learning-reset"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.ResetLearning()
}

// handleLearningSetupCron installs the crontab entry from LEARNING_CRON_SCHEDULE
func (cli *CLI) handleLearningSetupCron() error {
	if err := requiresManagedMode("learning-setup-cron"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.SetupCron()
}

// handleLearningRemoveCron removes the DevTrack learning crontab entry
func (cli *CLI) handleLearningRemoveCron() error {
	if err := requiresManagedMode("learning-remove-cron"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.RemoveCron()
}

// handleLearningCronStatus shows cron entry status
func (cli *CLI) handleLearningCronStatus() error {
	if err := requiresManagedMode("learning-cron-status"); err != nil {
		return err
	}
	learning := NewLearningCommands()
	return learning.CronStatus()
}

// handleLearningSync runs a delta (or full) sync immediately
func (cli *CLI) handleLearningSync() error {
	if err := requiresManagedMode("learning-sync"); err != nil {
		return err
	}
	full := len(os.Args) > 2 && os.Args[2] == "--full"
	learning := NewLearningCommands()
	return learning.SyncNow(full)
}
