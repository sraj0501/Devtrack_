package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// RegisterDevTrackTools registers all six DevTrack MCP tools on the server.
// database must be an open Database connection. All tools are read-only.
func RegisterDevTrackTools(s *Server, database *db.Database) {
	s.Register(Tool{
		Name:        "get_active_context",
		Title:       "Get active work context",
		Annotations: readOnlyAnnotations(),
		Description: "Returns the developer's current active ticket, repo path, today's commit count, and confidence in the ticket mapping. This is the primary context tool — call it first.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: makeGetActiveContext(database),
	})

	s.Register(Tool{
		Name:        "get_today_commits",
		Title:       "Get today's commits",
		Annotations: readOnlyAnnotations(),
		Description: "Returns all commits from today, grouped by ticket ID, with message and metadata.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repo_path": map[string]interface{}{
					"type":        "string",
					"description": "Filter by repository path. Omit for all repos.",
				},
			},
		},
		Handler: makeGetTodayCommits(database),
	})

	s.Register(Tool{
		Name:        "get_pending_actions",
		Title:       "Get pending actions",
		Annotations: readOnlyAnnotations(),
		Description: "Returns the current pending actions queue — actions DevTrack wants to take but hasn't yet. Each action has a confidence score and an expiry time.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: makeGetPendingActions(database),
	})

	s.Register(Tool{
		Name:        "get_voice_profile",
		Title:       "Get voice profile",
		Annotations: readOnlyAnnotations(),
		Description: "Returns the developer's inferred writing style profile for a given context type. Use this to understand how the developer prefers to communicate before generating text on their behalf.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"context_type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"commit", "comment", "report", "task", "ticket_mapping"},
					"description": "The writing context to retrieve style inferences for.",
				},
			},
		},
		Handler: makeGetVoiceProfile(database),
	})

	s.Register(Tool{
		Name:        "get_ticket_context",
		Title:       "Get ticket context",
		Annotations: readOnlyAnnotations(),
		Description: "Returns full context for a named ticket: recent commits, current pending actions targeting it, and its last activity time.",
		InputSchema: map[string]interface{}{
			"type":     "object",
			"required": []string{"ticket_id"},
			"properties": map[string]interface{}{
				"ticket_id": map[string]interface{}{
					"type":        "string",
					"description": "The ticket ID, e.g. PROJ-123 or AB-7",
				},
			},
		},
		Handler: makeGetTicketContext(database),
	})

	s.Register(Tool{
		Name:        "get_eod_summary",
		Title:       "Get end-of-day summary",
		Annotations: readOnlyAnnotations(),
		Description: "Returns today's EOD narrative draft — a summary of the day's commits grouped by ticket, suitable for a standup or daily report. Template-based (no LLM required).",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: makeGetEODSummary(database),
	})
}

func readOnlyAnnotations() map[string]any {
	return map[string]any{
		"readOnlyHint":    true,
		"destructiveHint": false,
		"idempotentHint":  true,
		"openWorldHint":   false,
	}
}

// ---- Tool 1: get_active_context ----

func makeGetActiveContext(database *db.Database) func(context.Context, map[string]interface{}) (interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		recent, err := database.MostRecentCommit()
		if err != nil {
			return nil, fmt.Errorf("get_active_context: %w", err)
		}

		confidence := "none"
		if recent.Hash != "" {
			if recent.TicketID != "" && recent.TicketID != "unlinked" {
				confidence = "high"
			} else {
				confidence = "low"
			}
		}

		todayCount, _ := database.CountTodayCommits()

		pending, err := database.ListPendingActions("pending")
		pendingCount := 0
		if err == nil {
			pendingCount = len(pending)
		}

		return map[string]interface{}{
			"active_ticket":         recent.TicketID,
			"repo_path":             recent.RepoPath,
			"confidence":            confidence,
			"today_commits":         todayCount,
			"pending_actions_count": pendingCount,
			"last_commit_time":      recent.Timestamp,
		}, nil
	}
}

// ---- Tool 2: get_today_commits ----

func makeGetTodayCommits(database *db.Database) func(context.Context, map[string]interface{}) (interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		repoPath, _ := args["repo_path"].(string)

		commits, err := database.ListTodayCommits(repoPath)
		if err != nil {
			return nil, fmt.Errorf("get_today_commits: %w", err)
		}

		grouped := make(map[string][]map[string]interface{})
		for _, c := range commits {
			key := c.TicketID
			if key == "" {
				key = "unlinked"
			}
			grouped[key] = append(grouped[key], map[string]interface{}{
				"hash":      shortHash(c.Hash),
				"message":   c.Message,
				"timestamp": c.Timestamp,
				"repo_path": c.RepoPath,
			})
		}

		return map[string]interface{}{
			"commits_by_ticket": grouped,
			"total_today":       len(commits),
		}, nil
	}
}

// ---- Tool 3: get_pending_actions ----

func makeGetPendingActions(database *db.Database) func(context.Context, map[string]interface{}) (interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		actions, err := database.ListPendingActions("pending")
		if err != nil {
			return nil, fmt.Errorf("get_pending_actions: %w", err)
		}

		result := make([]map[string]interface{}, 0, len(actions))
		for _, a := range actions {
			preview := a.Payload
			if len(preview) > 120 {
				preview = preview[:120] + "..."
			}
			result = append(result, map[string]interface{}{
				"id":              a.ID,
				"action_type":     a.ActionType,
				"target":          a.Target,
				"platform":        a.Platform,
				"confidence":      a.Confidence,
				"expires_at":      a.ExpiresAt.Format("2006-01-02T15:04:05Z"),
				"payload_preview": preview,
			})
		}

		return map[string]interface{}{
			"pending": result,
			"count":   len(result),
		}, nil
	}
}

// ---- Tool 4: get_voice_profile ----

func makeGetVoiceProfile(database *db.Database) func(context.Context, map[string]interface{}) (interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		contextType, _ := args["context_type"].(string)

		inferences, err := database.ListInferencesByConfidence(contextType, 10)
		if err != nil {
			return nil, fmt.Errorf("get_voice_profile inferences: %w", err)
		}

		allSkills, err := database.ListSkills()
		if err != nil {
			return nil, fmt.Errorf("get_voice_profile skills: %w", err)
		}

		infList := make([]map[string]interface{}, 0, len(inferences))
		for _, inf := range inferences {
			infList = append(infList, map[string]interface{}{
				"subject":    inf.Subject,
				"inference":  inf.Inference,
				"confidence": inf.Confidence,
				"source":     inf.Source,
			})
		}

		// Filter skills to matching context_type (or include all if contextType is "")
		skillList := make([]map[string]interface{}, 0)
		for _, sk := range allSkills {
			if contextType == "" || sk.ContextType == contextType {
				skillList = append(skillList, map[string]interface{}{
					"name":           sk.Name,
					"description":    sk.Description,
					"evidence_count": sk.EvidenceCount,
				})
			}
		}

		result := map[string]interface{}{
			"context_type": contextType,
			"inferences":   infList,
			"skills":       skillList,
		}
		if len(infList) == 0 && len(skillList) == 0 {
			result["note"] = "No voice data yet. Run `devtrack voice status` for details."
		}
		return result, nil
	}
}

// ---- Tool 5: get_ticket_context ----

func makeGetTicketContext(database *db.Database) func(context.Context, map[string]interface{}) (interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		ticketID, _ := args["ticket_id"].(string)
		if ticketID == "" {
			return nil, fmt.Errorf("ticket_id is required")
		}

		commits, err := database.ListTicketCommits(ticketID, 10)
		if err != nil {
			return nil, fmt.Errorf("get_ticket_context commits: %w", err)
		}

		allPending, err := database.ListPendingActions("")
		if err != nil {
			return nil, fmt.Errorf("get_ticket_context pending: %w", err)
		}

		commitList := make([]map[string]interface{}, 0, len(commits))
		lastActivity := ""
		for _, c := range commits {
			commitList = append(commitList, map[string]interface{}{
				"hash":      shortHash(c.Hash),
				"message":   c.Message,
				"repo_path": c.RepoPath,
				"timestamp": c.Timestamp,
			})
			if lastActivity == "" {
				lastActivity = c.Timestamp
			}
		}

		pendingList := make([]map[string]interface{}, 0)
		for _, a := range allPending {
			if a.Target == ticketID {
				pendingList = append(pendingList, map[string]interface{}{
					"id":          a.ID,
					"action_type": a.ActionType,
					"confidence":  a.Confidence,
					"status":      a.Status,
				})
			}
		}

		return map[string]interface{}{
			"ticket_id":       ticketID,
			"recent_commits":  commitList,
			"pending_actions": pendingList,
			"last_activity":   lastActivity,
		}, nil
	}
}

// ---- Tool 6: get_eod_summary ----

func makeGetEODSummary(database *db.Database) func(context.Context, map[string]interface{}) (interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		commits, err := database.ListTodayCommits("")
		if err != nil {
			return nil, fmt.Errorf("get_eod_summary: %w", err)
		}

		byTicket := make(map[string][]string)
		for _, c := range commits {
			key := c.TicketID
			if key == "" {
				key = "unlinked"
			}
			byTicket[key] = append(byTicket[key], c.Message)
		}

		// Build template narrative
		parts := make([]string, 0, len(byTicket))
		byTicketResult := make(map[string]map[string]interface{})
		for ticket, msgs := range byTicket {
			n := len(msgs)
			summary := fmt.Sprintf("%s (%d commit", ticket, n)
			if n != 1 {
				summary += "s"
			}
			summary += ") — " + strings.Join(truncateMessages(msgs, 3), ", ")
			parts = append(parts, summary)
			byTicketResult[ticket] = map[string]interface{}{
				"commits":  n,
				"messages": msgs,
			}
		}

		narrative := "Today: " + strings.Join(parts, ". ")
		if len(commits) == 0 {
			narrative = "No commits today."
		}

		return map[string]interface{}{
			"date":           "today",
			"tickets_worked": len(byTicket),
			"total_commits":  len(commits),
			"summary":        narrative,
			"by_ticket":      byTicketResult,
		}, nil
	}
}

// ---- Helpers ----

func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

func truncateMessages(msgs []string, max int) []string {
	if len(msgs) <= max {
		return msgs
	}
	return msgs[:max]
}
