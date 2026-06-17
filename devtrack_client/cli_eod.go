package main

// cli_eod.go — implements `devtrack eod` subcommand group.
//
// Subcommands:
//   devtrack eod              alias for "devtrack eod generate"
//   devtrack eod generate     POST /reports/eod via the HTTP trigger client;
//                             print the narrative to stdout;
//                             if action_id is in the response print "Queued as action <id>"
//   devtrack eod status       query db.ListPendingActions("") filtered to action_type=="eod_report";
//                             print most recent: date, action_id, status
//   devtrack eod show         same filter, most recent, parse payload JSON for "narrative" and print;
//                             if none: print "No EOD report on record"
//
// Output is pipe-friendly: ANSI colour is stripped when stdout is not a TTY
// (isatty check via github.com/mattn/go-isatty, already in go.mod).

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// handleEOD dispatches `devtrack eod` subcommands.
// Bare `devtrack eod` is treated as `devtrack eod generate`.
func (cli *CLI) handleEOD() error {
	sub := "generate"
	if len(os.Args) > 2 {
		sub = os.Args[2]
	}

	switch sub {
	case "generate", "gen":
		return runEODGenerate()
	case "status":
		return runEODStatus()
	case "show":
		return runEODShow()
	default:
		fmt.Printf("devtrack eod: unknown subcommand %q\n\n", sub)
		printEODUsage()
		return fmt.Errorf("unknown eod subcommand: %s", sub)
	}
}

// ── generate ──────────────────────────────────────────────────────────────────

// runEODGenerate implements `devtrack eod generate`.
// It calls POST /reports/eod, prints the narrative, and prints the action_id
// when the server staged the report in pending_actions.
func runEODGenerate() error {
	tc := trigger.NewHTTPTriggerClient()
	result, err := tc.ReportEODFull("", "")
	if err != nil {
		return fmt.Errorf("eod generate: %w", err)
	}

	tty := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	if tty {
		fmt.Println(result.Narrative)
	} else {
		// Pipe-friendly: just the raw narrative text, no decorators.
		fmt.Print(result.Narrative)
		if !strings.HasSuffix(result.Narrative, "\n") {
			fmt.Println()
		}
	}

	if result.ActionID != nil {
		fmt.Printf("Queued as action %d\n", *result.ActionID)
	}
	return nil
}

// ── status ────────────────────────────────────────────────────────────────────

// runEODStatus implements `devtrack eod status`.
// It finds the most recent eod_report action in pending_actions and prints
// its creation date, action_id, and status.
func runEODStatus() error {
	d, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("eod status: open database: %w", err)
	}
	defer d.Close()

	action, err := latestEODAction(d)
	if err != nil {
		return fmt.Errorf("eod status: %w", err)
	}
	if action == nil {
		fmt.Println("No EOD report on record.")
		return nil
	}

	fmt.Printf("Date:      %s\n", action.CreatedAt.Local().Format("2006-01-02 15:04"))
	fmt.Printf("Action ID: %d\n", action.ID)
	fmt.Printf("Status:    %s\n", action.Status)
	return nil
}

// ── show ──────────────────────────────────────────────────────────────────────

// runEODShow implements `devtrack eod show`.
// It finds the most recent eod_report action, extracts the "narrative" field
// from its payload JSON, and prints it.
func runEODShow() error {
	d, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("eod show: open database: %w", err)
	}
	defer d.Close()

	action, err := latestEODAction(d)
	if err != nil {
		return fmt.Errorf("eod show: %w", err)
	}
	if action == nil {
		fmt.Println("No EOD report on record.")
		return nil
	}

	// Parse the narrative from the JSON payload.
	var payload struct {
		Narrative string `json:"narrative"`
	}
	if err := json.Unmarshal([]byte(action.Payload), &payload); err != nil {
		return fmt.Errorf("eod show: parse payload JSON: %w", err)
	}
	if payload.Narrative == "" {
		fmt.Println("No EOD report on record.")
		return nil
	}

	tty := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	if tty {
		fmt.Println(payload.Narrative)
	} else {
		fmt.Print(payload.Narrative)
		if !strings.HasSuffix(payload.Narrative, "\n") {
			fmt.Println()
		}
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// latestEODAction returns the most recent pending_actions row with action_type=="eod_report",
// or nil if none exist. Uses ListPendingActions("") to search all statuses.
func latestEODAction(d *db.Database) (*db.PendingAction, error) {
	all, err := d.ListPendingActions("")
	if err != nil {
		return nil, err
	}
	// ListPendingActions orders by expires_at ASC; iterate in reverse to find most recent.
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].ActionType == "eod_report" {
			a := all[i]
			return &a, nil
		}
	}
	return nil, nil
}

// printEODUsage prints a short usage block for the eod command group.
func printEODUsage() {
	fmt.Println("Usage:")
	fmt.Println("  devtrack eod [generate]   Generate EOD report (POST /reports/eod) and print narrative")
	fmt.Println("  devtrack eod status        Show most recent EOD report action: date, ID, status")
	fmt.Println("  devtrack eod show          Print narrative of most recent EOD report from queue")
}
