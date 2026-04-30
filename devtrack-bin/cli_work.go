package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// handleWork dispatches devtrack work <subcommand>
func (cli *CLI) handleWork() error {
	if len(os.Args) < 3 {
		printWorkUsage()
		return nil
	}

	sub := os.Args[2]
	switch sub {
	case "start":
		return cli.handleWorkStart()
	case "stop":
		return cli.handleWorkStop()
	case "adjust":
		return cli.handleWorkAdjust()
	case "status":
		return cli.handleWorkStatus()
	case "report":
		return cli.handleWorkReport()
	default:
		fmt.Printf("Unknown work subcommand: %s\n\n", sub)
		printWorkUsage()
		return fmt.Errorf("unknown work subcommand: %s", sub)
	}
}

func printWorkUsage() {
	fmt.Println(`Usage: devtrack work <subcommand>

Subcommands:
  start [ticket-ref]          Start a work session (optionally tied to a ticket/PR)
  stop                        Stop the active work session (auto-measures duration)
  adjust <minutes>            Override time on the active or last session
  status                      Show the active session and today's completed sessions
  report [--email addr]       Generate the EOD report; optionally email it`)
}

// handleWorkStart starts a new work session
func (cli *CLI) handleWorkStart() error {
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	// Check for already-active session
	active, err := db.GetActiveWorkSession()
	if err != nil {
		return fmt.Errorf("could not check active session: %w", err)
	}
	if active != nil {
		fmt.Printf("⚠️  A session is already active (started %s", active.StartedAt)
		if active.TicketRef != "" {
			fmt.Printf(", ticket: %s", active.TicketRef)
		}
		fmt.Println(")")
		fmt.Println("Run `devtrack work stop` first.")
		return nil
	}

	// Optional ticket ref: devtrack work start AUTH-42
	ticketRef := ""
	if len(os.Args) >= 4 {
		ticketRef = os.Args[3]
	}

	repoPath := "."
	workspaceName := ""

	id, err := db.InsertWorkSession(ticketRef, repoPath, workspaceName)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	msg := fmt.Sprintf("✅ Work session started (ID %d)", id)
	if ticketRef != "" {
		msg += fmt.Sprintf(" for %s", ticketRef)
	}
	fmt.Println(msg)
	fmt.Println("   Run `devtrack work stop` when you're done.")
	return nil
}

// handleWorkStop stops the active work session and records auto-measured duration
func (cli *CLI) handleWorkStop() error {
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	active, err := db.GetActiveWorkSession()
	if err != nil {
		return fmt.Errorf("could not check active session: %w", err)
	}
	if active == nil {
		fmt.Println("No active work session found.")
		return nil
	}

	// Compute duration
	startTime, err := time.Parse("2006-01-02 15:04:05", active.StartedAt)
	if err != nil {
		// Try RFC3339 fallback
		startTime, err = time.Parse(time.RFC3339, active.StartedAt)
		if err != nil {
			startTime = time.Now()
		}
	}
	durationMins := int(time.Since(startTime).Minutes())
	if durationMins < 0 {
		durationMins = 0
	}

	endedAt := time.Now().UTC().Format("2006-01-02 15:04:05")
	if err := db.EndWorkSession(active.ID, endedAt, durationMins); err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}

	hours := durationMins / 60
	mins := durationMins % 60
	durationStr := fmt.Sprintf("%dm", durationMins)
	if hours > 0 {
		durationStr = fmt.Sprintf("%dh %dm", hours, mins)
	}

	fmt.Printf("✅ Session stopped. Duration: %s", durationStr)
	if active.TicketRef != "" {
		fmt.Printf(" (ticket: %s)", active.TicketRef)
	}
	fmt.Println()
	fmt.Println("   Tip: `devtrack work adjust <minutes>` to correct the recorded time.")
	return nil
}

// handleWorkAdjust overrides the time on the most recent (or active) session
func (cli *CLI) handleWorkAdjust() error {
	if len(os.Args) < 4 {
		fmt.Println("Usage: devtrack work adjust <minutes>")
		return fmt.Errorf("missing minutes argument")
	}

	minutes, err := strconv.Atoi(os.Args[3])
	if err != nil || minutes < 0 {
		return fmt.Errorf("invalid minutes value: %s (must be a non-negative integer)", os.Args[3])
	}

	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	// Try active session first, then today's last session
	active, err := db.GetActiveWorkSession()
	if err != nil {
		return fmt.Errorf("could not check active session: %w", err)
	}

	var targetID int64
	if active != nil {
		targetID = active.ID
	} else {
		today := time.Now().Format("2006-01-02")
		sessions, err := db.GetWorkSessionsForDate(today)
		if err != nil || len(sessions) == 0 {
			fmt.Println("No active or recent session found to adjust.")
			return nil
		}
		targetID = sessions[len(sessions)-1].ID
	}

	if err := db.AdjustWorkSessionTime(targetID, minutes); err != nil {
		return fmt.Errorf("failed to adjust session: %w", err)
	}

	hours := minutes / 60
	mins := minutes % 60
	durationStr := fmt.Sprintf("%dm", minutes)
	if hours > 0 {
		durationStr = fmt.Sprintf("%dh %dm", hours, mins)
	}
	fmt.Printf("✅ Session %d time adjusted to %s (auto-measured time preserved in DB for audit).\n", targetID, durationStr)
	return nil
}

// handleWorkStatus shows the active session and today's completed sessions
func (cli *CLI) handleWorkStatus() error {
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	active, err := db.GetActiveWorkSession()
	if err != nil {
		return fmt.Errorf("could not check active session: %w", err)
	}

	if active != nil {
		startTime, _ := time.Parse("2006-01-02 15:04:05", active.StartedAt)
		elapsed := int(time.Since(startTime).Minutes())
		hours := elapsed / 60
		mins := elapsed % 60
		elapsedStr := fmt.Sprintf("%dm", elapsed)
		if hours > 0 {
			elapsedStr = fmt.Sprintf("%dh %dm", hours, mins)
		}

		fmt.Printf("🟢 Active session (ID %d) — running for %s\n", active.ID, elapsedStr)
		if active.TicketRef != "" {
			fmt.Printf("   Ticket: %s\n", active.TicketRef)
		}
		if active.RepoPath != "" && active.RepoPath != "." {
			fmt.Printf("   Repo:   %s\n", active.RepoPath)
		}
		fmt.Println()
	} else {
		fmt.Println("⚪ No active session.")
	}

	today := time.Now().Format("2006-01-02")
	sessions, err := db.GetWorkSessionsForDate(today)
	if err != nil {
		return fmt.Errorf("could not load today's sessions: %w", err)
	}

	completed := make([]WorkSessionRecord, 0)
	for _, s := range sessions {
		if s.EndedAt != nil {
			completed = append(completed, s)
		}
	}

	if len(completed) == 0 {
		fmt.Println("No completed sessions today.")
		return nil
	}

	fmt.Printf("Today's completed sessions (%d):\n", len(completed))
	totalMins := 0
	for _, s := range completed {
		dur := 0
		if s.AdjustedMinutes != nil {
			dur = *s.AdjustedMinutes
		} else if s.DurationMinutes != nil {
			dur = *s.DurationMinutes
		}
		totalMins += dur

		hours := dur / 60
		mins := dur % 60
		durationStr := fmt.Sprintf("%dm", dur)
		if hours > 0 {
			durationStr = fmt.Sprintf("%dh %dm", hours, mins)
		}

		ticketStr := s.TicketRef
		if ticketStr == "" {
			ticketStr = "(no ticket)"
		}
		adjNote := ""
		if s.AdjustedMinutes != nil {
			adjNote = " [adjusted]"
		}
		fmt.Printf("  • %s  %s%s\n", durationStr+adjNote, ticketStr, "")
	}
	totalHours := totalMins / 60
	totalMinsRem := totalMins % 60
	totalStr := fmt.Sprintf("%dm", totalMins)
	if totalHours > 0 {
		totalStr = fmt.Sprintf("%dh %dm", totalHours, totalMinsRem)
	}
	fmt.Printf("  ─────────────────────────────\n")
	fmt.Printf("  Total today: %s\n", totalStr)
	return nil
}

// handleWorkReport delegates EOD report generation to the Python layer
func (cli *CLI) handleWorkReport() error {
	config, _ := LoadEnvConfig()
	projectRoot := ""
	if config != nil {
		projectRoot = config.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = os.Getenv("PROJECT_ROOT")
	}

	uvArgs := []string{"run", "--directory", projectRoot, "python", "-m", "backend.work_tracker.eod_report_generator"}

	// Forward --email flag if present
	for i, arg := range os.Args[3:] {
		if arg == "--email" && i+1 < len(os.Args[3:]) {
			uvArgs = append(uvArgs, "--email", os.Args[3+i+1])
			break
		}
	}

	cmd := exec.Command("uv", uvArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// handleServerTUI launches the Textual-based server process monitor.
// Usage: devtrack server-tui
func (cli *CLI) handleServerTUI() error {
	if err := requiresManagedMode("server-tui"); err != nil {
		return err
	}
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		return fmt.Errorf("PROJECT_ROOT is not set — cannot locate Python backend")
	}
	cmd := exec.Command("uv", "run", "--directory", projectRoot, "python", "-m", "backend.server_tui")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// handleAdminStart starts the Admin Console web server (CS-3).
// Usage: devtrack admin-start
func (cli *CLI) handleAdminStart() error {
	if err := requiresManagedMode("admin-start"); err != nil {
		return err
	}
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		return fmt.Errorf("PROJECT_ROOT is not set — cannot locate Python backend")
	}
	adminPort := os.Getenv("ADMIN_PORT")
	if adminPort == "" {
		adminPort = "8090"
	}
	fmt.Printf("Starting Admin Console on http://localhost:%s/admin/\n", adminPort)
	fmt.Println("Press Ctrl+C to stop.")
	cmd := exec.Command("uv", "run", "--directory", projectRoot, "python", "-m", "backend.admin")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
