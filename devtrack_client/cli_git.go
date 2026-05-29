package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sraj0501/Devtrack_/devtrack_client/connectors/pm"
	"github.com/sraj0501/Devtrack_/devtrack_client/gitsage"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/tui"
)

// gitCommitHooks builds the PM/push post-commit behaviour injected into the
// Go-native commit flow. All steps are interactive and degrade gracefully:
// they no-op on non-TTY stdin, missing PM creds, pm_platform "none", or errors.
func gitCommitHooks() *gitsage.CommitHooks {
	return &gitsage.CommitHooks{
		BeforeCommit: gitBeforeCommit,
		AfterCommit:  gitAfterCommit,
	}
}

// gitBeforeCommit offers the interactive ticket picker and, if a ticket is
// chosen, appends a "Refs: <id>" trailer. The selected ticket is returned as
// opaque state for gitAfterCommit.
func gitBeforeCommit(repoPath, message string) (string, any) {
	if !gitsage.IsInteractive() {
		return "", nil
	}
	ws, _ := config.ResolveWorkspaceForPath(repoPath)
	if ws == nil || !pm.SupportedPlatform(ws.PMPlatform) {
		return "", nil
	}

	tickets, err := pm.ListOpenTickets(ws)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  (ticket picker skipped: %v)\n", err)
		return "", nil
	}
	if len(tickets) == 0 {
		return "", nil
	}

	items := make([]tui.PickItem, len(tickets))
	for i, t := range tickets {
		items[i] = tui.PickItem{Title: t.Title, Subtitle: t.ID + " · " + t.State, Body: t.Body}
	}
	idx, skip, err := tui.PickTicket(items)
	if err != nil || skip || idx < 0 || idx >= len(tickets) {
		return "", nil
	}
	chosen := tickets[idx]
	return message + "\n\nRefs: " + chosen.ID, chosen
}

// gitAfterCommit runs the time prompt, immediate PM sync (with offline-queue
// fallback), and the auto-push prompt.
func gitAfterCommit(repoPath, hash, branch, message string, state any) {
	if !gitsage.IsInteractive() {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	ticket, hasTicket := state.(pm.Ticket)

	// --- Time tracking ---
	fmt.Print("\n🔔 Log this work? How long did it take? (e.g. 2h, 30m — Enter to skip): ")
	line, _ := reader.ReadString('\n')
	mins, hasMins := parseMinutes(line)

	// --- Local work-session record (best-effort) ---
	if database := openDBQuiet(); database != nil {
		ticketRef := ""
		if hasTicket {
			ticketRef = ticket.ID
		}
		if hasMins || hasTicket {
			wsName := ""
			if ws, _ := config.ResolveWorkspaceForPath(repoPath); ws != nil {
				wsName = ws.Name
			}
			if id, err := database.InsertWorkSession(ticketRef, repoPath, wsName); err == nil {
				_, _ = database.Exec(`UPDATE work_sessions SET commits = ? WHERE id = ?`,
					fmt.Sprintf("[%q]", hash), id)
				if hasMins {
					_ = database.AdjustWorkSessionTime(id, mins)
					_ = database.EndWorkSession(id, time.Now().UTC().Format("2006-01-02 15:04:05"), mins)
				}
			}
		}

		// --- Immediate PM sync with offline-queue fallback ---
		if hasTicket {
			if confirmYN(reader, fmt.Sprintf("→ Post commit update to %s?", ticket.ID), true) {
				body := buildCommentBody(hash, message, mins, hasMins)
				if err := pm.AddComment(ticket, body); err != nil {
					if qErr := pm.EnqueueComment(database, ticket, body, hash); qErr == nil {
						fmt.Printf("  ⚠️  %s unreachable — queued for sync when back online.\n", ticket.Platform)
					} else {
						fmt.Printf("  ✗ Sync failed and could not queue: %v\n", err)
					}
				} else {
					fmt.Printf("  ✓ Posted to %s\n", ticket.ID)
				}
			}
		}
		_ = database.Close()
	} else if hasTicket {
		// No DB available — still attempt an immediate post, no queue fallback.
		if confirmYN(reader, fmt.Sprintf("→ Post commit update to %s?", ticket.ID), true) {
			body := buildCommentBody(hash, message, mins, hasMins)
			if err := pm.AddComment(ticket, body); err != nil {
				fmt.Printf("  ✗ Sync failed: %v\n", err)
			} else {
				fmt.Printf("  ✓ Posted to %s\n", ticket.ID)
			}
		}
	}

	// --- Auto-push ---
	if branch != "" && confirmYN(reader, fmt.Sprintf("🚀 Push to origin/%s?", branch), false) {
		gitPush(repoPath, branch)
	}
}

// gitPush pushes the current branch to origin, suppressing DevTrack's own hooks.
func gitPush(repoPath, branch string) {
	cmd := exec.Command("git", "push", "origin", branch)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_NO_DEVTRACK=1")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("  ✗ Push failed — run 'git push origin %s' to retry.\n", branch)
		return
	}
	fmt.Printf("  ✓ Pushed to origin/%s\n", branch)
}

// --- helpers ---

// openDBQuiet opens the database, returning nil if unavailable (best-effort).
func openDBQuiet() *db.Database {
	database, err := db.NewDatabase()
	if err != nil {
		return nil
	}
	return database
}

// confirmYN prompts a yes/no question. def is the answer for a bare Enter.
func confirmYN(reader *bufio.Reader, prompt string, def bool) bool {
	suffix := " (y/N) "
	if def {
		suffix = " (Y/n) "
	}
	fmt.Print(prompt + suffix)
	line, err := reader.ReadString('\n')
	if err != nil {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "":
		return def
	case "y", "yes":
		return true
	default:
		return false
	}
}

// buildCommentBody composes the PM comment from the commit and optional time.
func buildCommentBody(hash, message string, mins int, hasMins bool) string {
	subject := message
	if i := strings.IndexByte(message, '\n'); i >= 0 {
		subject = message[:i]
	}
	short := hash
	if len(short) > 8 {
		short = short[:8]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Commit %s: %s", short, subject)
	if hasMins {
		fmt.Fprintf(&b, "\n\nTime spent: %s", formatMinutes(mins))
	}
	return b.String()
}

// parseMinutes parses "2h", "30m", "1h30m", or a bare integer (minutes).
func parseMinutes(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n > 0 {
			return n, true
		}
		return 0, false
	}
	total, matched := 0, false
	if i := strings.IndexByte(s, 'h'); i >= 0 {
		if h, err := strconv.Atoi(strings.TrimSpace(s[:i])); err == nil {
			total += h * 60
			matched = true
			s = strings.TrimSpace(s[i+1:])
		}
	}
	if i := strings.IndexByte(s, 'm'); i >= 0 {
		if m, err := strconv.Atoi(strings.TrimSpace(s[:i])); err == nil {
			total += m
			matched = true
		}
	}
	return total, matched && total > 0
}

// formatMinutes renders minutes as e.g. "1h30m" or "45m".
func formatMinutes(mins int) string {
	if mins >= 60 {
		h, m := mins/60, mins%60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", mins)
}
