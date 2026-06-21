package main

// cli_skills.go — implements `devtrack skills` subcommand.
//
// Usage:
//
//	devtrack skills          — list all promoted skills
//
// Skills are autonomous patterns the system has inferred and promoted after
// observing them at least 5 times without developer corrections.

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// handleSkills dispatches `devtrack skills` subcommands.
// Bare `devtrack skills` lists all promoted skills.
func (cli *CLI) handleSkills() error {
	sub := "list"
	if len(os.Args) > 2 {
		sub = os.Args[2]
	}

	switch sub {
	case "list", "ls":
		return runSkillsList()
	default:
		return runSkillsList()
	}
}

// runSkillsList lists all skills promoted by the SkillDetector.
func runSkillsList() error {
	d, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer d.Close()

	skills, err := d.ListSkills()
	if err != nil {
		return fmt.Errorf("list skills: %w", err)
	}

	if len(skills) == 0 {
		fmt.Println("No skills have emerged yet.")
		fmt.Println("Skills emerge automatically from recurring patterns after 5 observations.")
		return nil
	}

	fmt.Printf("Autonomous Skills (%d)\n", len(skills))
	fmt.Println(strings.Repeat("-", 45))

	// Determine column width for skill name (left-pad to longest name).
	maxNameLen := 0
	for _, s := range skills {
		if len(s.Name) > maxNameLen {
			maxNameLen = len(s.Name)
		}
	}

	for _, s := range skills {
		since := s.PromotedAt.Format(time.DateOnly)
		if s.PromotedAt.IsZero() {
			since = "unknown"
		}
		fmt.Printf("%-*s   %-10s  evidence=%-4d  since %s\n",
			maxNameLen,
			s.Name,
			s.ContextType,
			s.EvidenceCount,
			since,
		)
	}
	return nil
}
