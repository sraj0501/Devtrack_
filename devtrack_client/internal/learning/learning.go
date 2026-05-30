package learning

import (
	"fmt"
	"strings"

	trig "github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// LearningCommands wraps HTTP calls to the server's /learning/* endpoints.
type LearningCommands struct{}

// NewLearningCommands returns a new LearningCommands.
func NewLearningCommands() *LearningCommands { return &LearningCommands{} }

func client() *trig.HTTPTriggerClient { return trig.NewHTTPTriggerClient() }

func printOutput(label, output string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w (is the server running?)", label, err)
	}
	if strings.TrimSpace(output) != "" {
		fmt.Print(output)
	}
	return nil
}

// SetupCron installs the learning crontab entry.
func (lc *LearningCommands) SetupCron() error {
	fmt.Println("Installing learning cron entry...")
	fmt.Println()
	output, err := client().LearningSetupCron()
	return printOutput("learning-setup-cron", output, err)
}

// RemoveCron removes the learning crontab entry.
func (lc *LearningCommands) RemoveCron() error {
	fmt.Println("Removing learning cron entry...")
	fmt.Println()
	output, err := client().LearningRemoveCron()
	return printOutput("learning-remove-cron", output, err)
}

// CronStatus shows the cron entry status.
func (lc *LearningCommands) CronStatus() error {
	output, err := client().LearningCronStatus()
	return printOutput("learning-cron-status", output, err)
}

// ResetLearning wipes all learning data.
func (lc *LearningCommands) ResetLearning() error {
	output, err := client().LearningReset()
	return printOutput("learning-reset", output, err)
}

// SyncNow runs a delta (or full) sync immediately.
func (lc *LearningCommands) SyncNow(full bool) error {
	fmt.Println("Running learning sync...")
	fmt.Println()
	output, err := client().LearningSync(full)
	return printOutput("learning-sync", output, err)
}

// EnableLearning starts collecting communication data and enables learning.
func (lc *LearningCommands) EnableLearning(days int) error {
	fmt.Println("Enabling personalized AI learning...")
	fmt.Println()
	output, err := client().LearningEnable(days)
	return printOutput("enable-learning", output, err)
}

// ShowProfile displays the current learning profile.
func (lc *LearningCommands) ShowProfile() error {
	output, err := client().LearningProfile()
	return printOutput("show-profile", output, err)
}

// TestResponse tests generating a personalized response.
func (lc *LearningCommands) TestResponse(text string) error {
	fmt.Println("Testing response generation...")
	fmt.Println()
	output, err := client().LearningTestResponse(text)
	return printOutput("test-response", output, err)
}

// RevokeConsent revokes learning consent.
func (lc *LearningCommands) RevokeConsent() error {
	fmt.Println("Revoking personalized learning consent...")
	fmt.Println()
	output, err := client().LearningRevoke()
	return printOutput("revoke-consent", output, err)
}

// GetLearningStatus returns the current learning status.
func (lc *LearningCommands) GetLearningStatus() (*LearningStatus, error) {
	r, err := client().LearningStatus()
	if err != nil {
		return &LearningStatus{}, nil // fail open — status is non-critical
	}
	return &LearningStatus{
		Enabled:      r.Enabled,
		ConsentGiven: r.ConsentGiven,
		SampleCount:  r.SampleCount,
		LastUpdated:  r.LastUpdated,
	}, nil
}

// LearningStatus represents the status of personalized learning.
type LearningStatus struct {
	Enabled      bool   `json:"enabled"`
	ConsentGiven bool   `json:"consent_given"`
	SampleCount  int    `json:"sample_count"`
	LastUpdated  string `json:"last_updated"`
}

// PrintStatus prints the learning status in a formatted way.
func (ls *LearningStatus) PrintStatus() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║          PERSONALIZED AI LEARNING STATUS                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	if ls.ConsentGiven {
		fmt.Println("  Status:        Enabled")
	} else {
		fmt.Println("  Status:        Disabled (consent not given)")
	}
	fmt.Printf("  Samples:       %d\n", ls.SampleCount)
	if ls.LastUpdated != "" {
		fmt.Printf("  Last Updated:  %s\n", ls.LastUpdated)
	} else {
		fmt.Println("  Last Updated:  Never")
	}
	fmt.Println()
	if !ls.ConsentGiven {
		fmt.Println("  To enable learning, run: devtrack enable-learning")
		fmt.Println()
	} else if ls.SampleCount == 0 {
		fmt.Println("  No samples collected yet. Learning in progress...")
		fmt.Println()
	} else {
		fmt.Println("  AI is learning from your communication patterns")
		fmt.Println()
	}
}
