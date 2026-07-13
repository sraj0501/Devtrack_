package main

// cli_review.go — implements `devtrack review` and `devtrack review status` commands.
//
// `devtrack review`        — shows new/classified review comments grouped by PR.
// `devtrack review status` — shows per-PR outcome summary from the last 24 hours,
//                            combining pr_review_comments and pending_actions data.
//
// Both commands read from SQLite directly (no daemon required).

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/reviewer"
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

// handleReviewStatus prints the per-PR activity summary from the last 24 hours.
// It combines pr_review_comments (for in-progress / stuck state) and
// pending_actions (for approved / escalated final states staged by the fix loop).
//
// Output format:
//
//	PR Review Activity (last 24h)
//	-----------------------------
//	PR #42   github   my-workspace   APPROVED     2 fixes applied
//	PR #19   github   my-workspace   STUCK        comment needs human: "arch question"
//	PR #7    azure    my-workspace   IN PROGRESS  (fix attempt 1/5)
func (cli *CLI) handleReviewStatus() error {
	database, err := db.NewDatabase()
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	// Collect pr_review_comments from the last 24 hours.
	comments, err := database.ListPRReviewCommentsRecent(24)
	if err != nil {
		return fmt.Errorf("list recent review comments: %w", err)
	}

	// Collect pending_actions from the last 24 hours to capture final outcomes
	// (pr_approved_notify / pr_escalation actions staged by the fix loop).
	pendingActions, err := database.ListPendingActionsRecent(24)
	if err != nil {
		// Non-fatal: proceed without pending action data.
		pendingActions = nil
	}

	// per-PR outcome accumulator.
	type prKey struct{ platform, prID string }
	type prOutcome struct {
		status    string // "APPROVED" | "STUCK" | "IN PROGRESS" | ""
		detail    string
		workspace string
	}

	outcomes := make(map[prKey]*prOutcome)
	keyOrder := []prKey{}
	seen := make(map[prKey]bool)
	addKey := func(k prKey, ws string) {
		if !seen[k] {
			seen[k] = true
			keyOrder = append(keyOrder, k)
			outcomes[k] = &prOutcome{workspace: ws}
		}
	}

	// Pass 1: resolve final outcomes from pending_actions
	// (pr_approved_notify and pr_escalation rows).
	for _, a := range pendingActions {
		if a.ActionType != "pr_approved_notify" && a.ActionType != "pr_escalation" {
			continue
		}
		// Target format: "github:PR #42"
		target := a.Target
		platform := a.Platform
		prIDStr := target
		if idx := strings.Index(target, ":PR #"); idx >= 0 {
			platform = target[:idx]
			prIDStr = target[idx+5:]
		}
		k := prKey{platform: platform, prID: prIDStr}
		addKey(k, a.Workspace)
		o := outcomes[k]

		var payload map[string]any
		_ = json.Unmarshal([]byte(a.Payload), &payload)

		if a.ActionType == "pr_approved_notify" && o.status != "APPROVED" {
			detail := ""
			if payload != nil {
				if n, ok := payload["fixes_applied"]; ok {
					detail = fmt.Sprintf("%v fixes applied", n)
				}
			}
			o.status = "APPROVED"
			o.detail = detail
		} else if a.ActionType == "pr_escalation" && o.status == "" {
			blocker := ""
			if payload != nil {
				if r, ok := payload["blocker_reason"].(string); ok {
					blocker = r
				}
			}
			o.status = "STUCK"
			o.detail = "comment needs human: " + reviewTruncate(blocker, 40)
		}
	}

	// Pass 2: derive IN PROGRESS / STUCK from pr_review_comments for PRs not
	// yet resolved via pending_actions.
	for _, c := range comments {
		k := prKey{platform: c.Platform, prID: c.PRID}
		addKey(k, c.Workspace)
		o := outcomes[k]
		if o.status != "" {
			continue // already resolved
		}
		switch {
		case c.ClassifiedAs == "needs_human" || c.Status == "escalated":
			o.status = "STUCK"
			o.detail = "comment needs human: " + reviewTruncate(c.CommentBody, 40)
		case c.ClassifiedAs == "auto_fixable":
			o.status = "IN PROGRESS"
			o.detail = fmt.Sprintf("(fix attempt %d/%d)", c.AttemptCount, reviewer.MaxAttemptsPerComment)
		}
	}

	if len(keyOrder) == 0 {
		fmt.Println("No PR review activity in the last 24 hours.")
		return nil
	}

	// Sort: platform ascending, then PR ID ascending.
	sort.Slice(keyOrder, func(i, j int) bool {
		ki, kj := keyOrder[i], keyOrder[j]
		if ki.platform != kj.platform {
			return ki.platform < kj.platform
		}
		return ki.prID < kj.prID
	})

	fmt.Println("PR Review Activity (last 24h)")
	fmt.Println("-----------------------------")
	for _, k := range keyOrder {
		o := outcomes[k]
		status := o.status
		if status == "" {
			status = "PENDING"
		}
		fmt.Printf("PR #%-6s  %-8s  %-18s  %-12s  %s\n",
			k.prID, k.platform, reviewTruncate(o.workspace, 18), status, o.detail)
	}
	return nil
}

// reviewTruncate shortens s to at most maxLen runes, appending "…" when trimmed.
func reviewTruncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
