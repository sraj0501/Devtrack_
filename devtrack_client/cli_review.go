package main

// cli_review.go — implements `devtrack review` command.
//
// Queries the local pr_review_comments SQLite table for new and classified
// review comments, groups them by (Platform, PRID), and prints a summary.
//
// Usage:
//   devtrack review

import (
	"fmt"
	"sort"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// handleReview prints the current PR review queue grouped by PR.
func (cli *CLI) handleReview() error {
	database, err := db.NewDatabase()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	// Collect "new" and "classified" comments.
	newComments, err := database.ListPRReviewCommentsByStatus("new")
	if err != nil {
		return fmt.Errorf("list new review comments: %w", err)
	}
	classifiedComments, err := database.ListPRReviewCommentsByStatus("classified")
	if err != nil {
		return fmt.Errorf("list classified review comments: %w", err)
	}

	all := append(newComments, classifiedComments...)

	if len(all) == 0 {
		fmt.Println("No PR review comments detected. Ensure ALERT_GITHUB_ENABLED=true and PRs are open.")
		return nil
	}

	// Group by (Platform + PRID).
	type prKey struct{ platform, prID string }
	type prStats struct {
		AutoFixable  int
		NeedsHuman   int
		Unclassified int
		Workspace    string
	}

	grouped := make(map[prKey]*prStats)
	keyOrder := []prKey{}

	for _, c := range all {
		k := prKey{platform: c.Platform, prID: c.PRID}
		if _, ok := grouped[k]; !ok {
			grouped[k] = &prStats{Workspace: c.Workspace}
			keyOrder = append(keyOrder, k)
		}
		stats := grouped[k]
		switch c.ClassifiedAs {
		case "auto_fixable":
			stats.AutoFixable++
		case "needs_human":
			stats.NeedsHuman++
		default:
			stats.Unclassified++
		}
	}

	// Sort by platform then PR ID for stable output.
	sort.Slice(keyOrder, func(i, j int) bool {
		ki, kj := keyOrder[i], keyOrder[j]
		if ki.platform != kj.platform {
			return ki.platform < kj.platform
		}
		return ki.prID < kj.prID
	})

	fmt.Println("PR Review Queue")
	fmt.Println("---------------")

	for _, k := range keyOrder {
		stats := grouped[k]
		total := stats.AutoFixable + stats.NeedsHuman + stats.Unclassified

		parts := ""
		if stats.AutoFixable > 0 {
			parts += fmt.Sprintf("%d auto_fixable", stats.AutoFixable)
		}
		if stats.NeedsHuman > 0 {
			if parts != "" {
				parts += ", "
			}
			parts += fmt.Sprintf("%d needs_human", stats.NeedsHuman)
		}
		if stats.Unclassified > 0 {
			if parts != "" {
				parts += ", "
			}
			parts += fmt.Sprintf("%d unclassified", stats.Unclassified)
		}

		commentWord := "comment"
		if total != 1 {
			commentWord = "comments"
		}

		fmt.Printf("PR #%-6s %-8s %-18s %d %s (%s)\n",
			k.prID,
			k.platform,
			stats.Workspace,
			total,
			commentWord,
			parts,
		)
	}

	return nil
}
