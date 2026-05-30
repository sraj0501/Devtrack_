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
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/match"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/tui"
)

// gitCommitHooks builds the PM/push post-commit behaviour injected into the
// Go-native commit flow. All steps are interactive and degrade gracefully:
// they no-op on non-TTY stdin, missing PM creds, pm_platform "none", or errors.
func gitCommitHooks() *gitsage.CommitHooks {
	return &gitsage.CommitHooks{
		BeforeCommit:  gitBeforeCommit,
		AfterCommit:   gitAfterCommit,
		QueueForLater: gitQueueForLater,
	}
}

// gitQueueForLater stores the staged change for later AI enhancement in the
// deferred_commits outbox (offline-first hold-diff model). The work is not
// committed; `devtrack commits review` applies it once enhanced.
func gitQueueForLater(repoPath, message, branch, diffPatch string, files []string) (bool, error) {
	database := openDBQuiet()
	if database == nil {
		return false, fmt.Errorf("database unavailable")
	}
	defer database.Close()

	mgr := NewDeferredCommitManager(database)
	if _, err := mgr.QueueCommit(message, diffPatch, branch, repoPath, files); err != nil {
		return false, err
	}
	return true, nil
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

	database := openDBQuiet()
	if database != nil {
		defer database.Close()
	}

	tickets, fromCache, err := pm.ListOpenTicketsCached(ws, database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  (ticket picker skipped: %v)\n", err)
		return "", nil
	}
	if fromCache {
		fmt.Fprintln(os.Stderr, "  (PM API unreachable — using locally cached tickets)")
	}

	if len(tickets) == 0 {
		// No open tickets — offer to create one when create-on-no-match is enabled.
		if !createOnNoMatchEnabled() {
			return "", nil
		}
		if t, ok := createTicketInteractive(ws, repoPath, message); ok {
			return message + "\n\nRefs: " + t.ID, t
		}
		return "", nil
	}

	// Rank tickets by how likely they relate to this commit, most-likely first.
	tickets, scores := rankTickets(repoPath, message, tickets)

	// Auto-link the top match when it clears the configured confidence threshold.
	if thr := matchThreshold(); thr > 0 && scores[0] >= thr {
		top := tickets[0]
		fmt.Printf("🎯 Auto-linked %s (%d%% match) — %s\n", top.ID, pct(scores[0]), top.Title)
		return message + "\n\nRefs: " + top.ID, top
	}

	items := make([]tui.PickItem, len(tickets))
	for i, t := range tickets {
		items[i] = tui.PickItem{
			Title:    t.Title,
			Subtitle: fmt.Sprintf("%3d%% · %s · %s", pct(scores[i]), t.ID, t.State),
			Body:     t.Body,
		}
	}
	idx, skip, createNew, err := tui.PickTicket(items, 0) // pre-select the top match
	if err != nil {
		return "", nil
	}
	if createNew {
		if t, ok := createTicketInteractive(ws, repoPath, message); ok {
			return message + "\n\nRefs: " + t.ID, t
		}
		return "", nil
	}
	if skip || idx < 0 || idx >= len(tickets) {
		return "", nil
	}
	chosen := tickets[idx]
	return message + "\n\nRefs: " + chosen.ID, chosen
}

// rankTickets reorders tickets by likelihood against the commit signal (branch,
// subject, staged files) and returns them with their parallel match scores.
func rankTickets(repoPath, message string, tickets []pm.Ticket) ([]pm.Ticket, []float64) {
	g := gitsage.NewGitOps(repoPath)
	branch, _ := g.CurrentBranch()
	files, _ := g.StagedFiles()

	sig := match.Signal{
		Branch:  branch,
		Subject: commitSubject(message),
		Files:   files,
		Refs:    parseTicketRefs(message),
	}
	docs := make([]match.Doc, len(tickets))
	for i, t := range tickets {
		docs[i] = match.Doc{ID: t.ID, Title: t.Title, Body: t.Body}
	}

	results := match.RankHybrid(sig, docs, match.NewOllamaEmbedder())
	ordered := make([]pm.Ticket, len(results))
	scores := make([]float64, len(results))
	for i, r := range results {
		ordered[i] = tickets[r.Index]
		scores[i] = r.Score
	}
	return ordered, scores
}

// matchThreshold reads PM_MATCH_THRESHOLD, the minimum score (0..1, or a 0..100
// percentage) at which the top match is auto-linked without showing the picker.
// Returns 0 (disabled) when unset or invalid.
func matchThreshold() float64 {
	v := strings.TrimSpace(os.Getenv("PM_MATCH_THRESHOLD"))
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		return 0
	}
	if f > 1 {
		f /= 100 // allow "85" to mean 0.85
	}
	return f
}

// pct renders a 0..1 score as a rounded whole percentage.
func pct(score float64) int {
	return int(score*100 + 0.5)
}

// createOnNoMatchEnabled reports whether PM_CREATE_ON_NO_MATCH opts into
// offering to create a ticket when no open ticket exists to link.
func createOnNoMatchEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("PM_CREATE_ON_NO_MATCH"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// createTicketInteractive prompts for a title (defaulting to the commit subject)
// and creates a new ticket on the workspace's PM platform. ok is false on skip
// or failure.
func createTicketInteractive(ws *config.WorkspaceConfig, repoPath, message string) (pm.Ticket, bool) {
	reader := bufio.NewReader(os.Stdin)
	def := commitSubject(message)
	if def != "" {
		fmt.Printf("📝 New %s ticket title [%s]: ", ws.PMPlatform, def)
	} else {
		fmt.Printf("📝 New %s ticket title: ", ws.PMPlatform)
	}
	line, _ := reader.ReadString('\n')
	title := strings.TrimSpace(line)
	if title == "" {
		title = def
	}
	if title == "" {
		fmt.Println("  (no title — skipping ticket creation)")
		return pm.Ticket{}, false
	}
	t, err := pm.CreateTicket(ws, repoPath, title, message)
	if err != nil {
		fmt.Printf("  ✗ Could not create ticket: %v\n", err)
		return pm.Ticket{}, false
	}
	fmt.Printf("  ✓ Created %s: %s\n", t.ID, t.Title)
	return t, true
}

// commitSubject returns the first line of a commit message, capped at 80 chars.
func commitSubject(message string) string {
	subject := message
	if i := strings.IndexByte(message, '\n'); i >= 0 {
		subject = message[:i]
	}
	subject = strings.TrimSpace(subject)
	if len(subject) > 80 {
		subject = strings.TrimSpace(subject[:80])
	}
	return subject
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

	// The PM comment is identical across the DB / no-DB branches below.
	body := buildCommentBody(hash, commitAuthor(repoPath, hash), message, detectStatus(message), mins, hasMins)

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
				ws, _ := config.ResolveWorkspaceForPath(repoPath)
				if err := pm.AddComment(ws, ticket, body); err != nil {
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
			ws, _ := config.ResolveWorkspaceForPath(repoPath)
			if err := pm.AddComment(ws, ticket, body); err != nil {
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

// buildCommentBody composes a structured PM comment from the commit, mirroring
// the legacy log_work.py format: Commit / Author / Message [/ Time spent /
// Status]. Status is included only when it is not the default "in_progress".
func buildCommentBody(hash, author, message, status string, mins int, hasMins bool) string {
	short := hash
	if len(short) > 12 {
		short = short[:12]
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("**Commit**: `%s`", short))
	if author != "" {
		lines = append(lines, fmt.Sprintf("**Author**: %s", author))
	}
	lines = append(lines, fmt.Sprintf("**Message**: %s", strings.TrimSpace(message)))
	if hasMins {
		lines = append(lines, fmt.Sprintf("**Time spent**: %s", formatMinutes(mins)))
	}
	if status != "" && status != "in_progress" {
		lines = append(lines, fmt.Sprintf("**Status**: %s", status))
	}
	return strings.Join(lines, "\n\n")
}

// commitAuthor returns the author name of a commit (best-effort, "" on error).
func commitAuthor(repoPath, hash string) string {
	cmd := exec.Command("git", "show", "-s", "--format=%an", hash)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectStatus derives a ticket status from GitHub-style closing keywords in
// the commit message ("fix"/"close"/"resolve" → "done"), else "in_progress".
func detectStatus(message string) string {
	lower := strings.ToLower(message)
	for _, kw := range []string{"fixes ", "fixed ", "fix ", "closes ", "closed ", "close ", "resolves ", "resolved ", "resolve "} {
		if strings.Contains(lower, kw) {
			return "done"
		}
	}
	return "in_progress"
}

// parseTicketRefs extracts explicit ticket numbers referenced in a commit
// message — "#42", "AB#1234", or "refs/fixes/closes #42" — for match boosting.
func parseTicketRefs(message string) []int {
	var refs []int
	seen := map[int]bool{}
	for i := 0; i < len(message); i++ {
		if message[i] != '#' {
			continue
		}
		j := i + 1
		for j < len(message) && message[j] >= '0' && message[j] <= '9' {
			j++
		}
		if j == i+1 {
			continue
		}
		if n, err := strconv.Atoi(message[i+1 : j]); err == nil && n > 0 && !seen[n] {
			seen[n] = true
			refs = append(refs, n)
		}
		i = j - 1
	}
	return refs
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
