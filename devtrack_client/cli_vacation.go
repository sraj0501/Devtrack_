package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// VacationCommands handles the `devtrack vacation` subcommands.
type VacationCommands struct {
	db *Database
}

func NewVacationCommands() (*VacationCommands, error) {
	db, err := NewDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return &VacationCommands{db: db}, nil
}

// On enables vacation mode.
//
//	devtrack vacation on [--until YYYY-MM-DD] [--threshold 0.7] [--no-submit]
func (vc *VacationCommands) On(args []string) error {
	until := ""
	threshold := 0.7
	autoSubmit := true

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--until":
			if i+1 < len(args) {
				i++
				until = args[i]
				// Validate date format
				if _, err := time.Parse("2006-01-02", until); err != nil {
					return fmt.Errorf("invalid date %q — use YYYY-MM-DD", until)
				}
			}
		case "--threshold":
			if i+1 < len(args) {
				i++
				v, err := strconv.ParseFloat(args[i], 64)
				if err != nil || v < 0 || v > 1 {
					return fmt.Errorf("threshold must be a number between 0 and 1")
				}
				threshold = v
			}
		case "--no-submit":
			autoSubmit = false
		}
	}

	if err := vc.db.SetVacationMode(true, until, threshold, autoSubmit); err != nil {
		return fmt.Errorf("failed to enable vacation mode: %w", err)
	}

	msg := "✈️  Vacation mode ON"
	if until != "" {
		msg += fmt.Sprintf(" (until %s)", until)
	} else {
		msg += " (indefinite — run 'devtrack vacation off' to disable)"
	}
	msg += fmt.Sprintf("\n   Confidence threshold : %.0f%%", threshold*100)
	if autoSubmit {
		msg += "\n   Auto-submit          : enabled (updates posted automatically when confident)"
	} else {
		msg += "\n   Auto-submit          : disabled (updates logged but not posted)"
	}
	fmt.Println(msg)
	return nil
}

// Off disables vacation mode.
func (vc *VacationCommands) Off() error {
	if err := vc.db.SetVacationMode(false, "", 0.7, true); err != nil {
		return fmt.Errorf("failed to disable vacation mode: %w", err)
	}
	fmt.Println("✓ Vacation mode OFF — normal prompting resumed")
	return nil
}

// Status prints the current vacation mode state.
func (vc *VacationCommands) Status() error {
	state, err := vc.db.GetVacationState()
	if err != nil {
		return fmt.Errorf("failed to read vacation state: %w", err)
	}

	if !state.Enabled {
		fmt.Println("Vacation mode: OFF")
		return nil
	}

	// Check if expired
	if state.Until != "" {
		if until, err := time.Parse("2006-01-02", state.Until); err == nil {
			if time.Now().After(until.Add(24 * time.Hour)) {
				fmt.Println("Vacation mode: EXPIRED (run 'devtrack vacation off' to clear)")
				return nil
			}
		}
	}

	lines := []string{
		"Vacation mode: ON ✈️",
		fmt.Sprintf("  Enabled at  : %s", state.EnabledAt),
	}
	if state.Until != "" {
		lines = append(lines, fmt.Sprintf("  Until       : %s", state.Until))
	} else {
		lines = append(lines, "  Until       : indefinite")
	}
	lines = append(lines, fmt.Sprintf("  Threshold   : %.0f%% confidence required to auto-submit", state.ConfidenceThreshold*100))
	if state.AutoSubmit {
		lines = append(lines, "  Auto-submit : enabled")
	} else {
		lines = append(lines, "  Auto-submit : disabled (logging only)")
	}
	fmt.Println(strings.Join(lines, "\n"))
	return nil
}
