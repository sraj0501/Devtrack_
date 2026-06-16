package main

// cli_queue.go — implements `devtrack queue` subcommand group.
//
// Subcommands:
//   devtrack queue [list] [--all]     List pending (default) or all recent actions
//   devtrack queue approve <id>       Approve a pending action — fires /queue/execute immediately
//   devtrack queue reject  <id>       Reject a pending action — marks rejected, will not post
//   devtrack queue edit    <id> <json> Replace payload JSON of a pending action
//   devtrack queue status             Show one-line summary: pending / posted today / rejected today
//
// All DB access goes through internal/db functions — no raw SQL in CLI code.
// The approve subcommand calls UpdatePendingActionStatus then ExecuteQueueAction.
// The edit subcommand validates JSON with json.Valid before writing to the DB.
// Output is plain text; ANSI colour is stripped when stdout is not a TTY so
// the output is pipe-friendly (e.g. devtrack queue list | cat).

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// handleQueue dispatches `devtrack queue` subcommands.
// Bare `devtrack queue` is treated as `devtrack queue list`.
func (cli *CLI) handleQueue() error {
	sub := "list"
	if len(os.Args) > 2 {
		sub = os.Args[2]
	}

	switch sub {
	case "list", "ls":
		return runQueueList()
	case "approve":
		return runQueueApprove()
	case "reject":
		return runQueueReject()
	case "edit":
		return runQueueEdit()
	case "status":
		return runQueueStatus()
	default:
		// Treat bare `devtrack queue` (no subcommand) as `devtrack queue list`.
		// But if os.Args[2] looks like a number, warn the user.
		if _, err := strconv.ParseInt(sub, 10, 64); err == nil {
			fmt.Printf("devtrack queue: unexpected ID %q — did you mean 'devtrack queue approve %s'?\n", sub, sub)
			printQueueUsage()
			return fmt.Errorf("unknown queue subcommand: %s", sub)
		}
		fmt.Printf("devtrack queue: unknown subcommand %q\n\n", sub)
		printQueueUsage()
		return fmt.Errorf("unknown queue subcommand: %s", sub)
	}
}

// ── list ──────────────────────────────────────────────────────────────────────

// runQueueList implements `devtrack queue list [--all]`.
// Default: shows only pending actions (last 24h window).
// --all: shows all recent actions regardless of status.
func runQueueList() error {
	showAll := false
	for _, arg := range os.Args[3:] {
		if arg == "--all" {
			showAll = true
		}
	}

	d, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("queue list: open database: %w", err)
	}
	defer d.Close()

	var actions []db.PendingAction
	if showAll {
		actions, err = d.ListPendingActionsRecent(24)
	} else {
		actions, err = d.ListPendingActions("pending")
	}
	if err != nil {
		return fmt.Errorf("queue list: %w", err)
	}

	if len(actions) == 0 {
		if showAll {
			fmt.Println("No actions in the last 24 hours.")
		} else {
			fmt.Println("No pending actions. (use --all to see recent history)")
		}
		return nil
	}

	tty := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	printQueueTable(actions, tty)
	return nil
}

// printQueueTable renders pending actions as a fixed-width table.
// Colour is suppressed when tty is false (pipe-friendly mode).
func printQueueTable(actions []db.PendingAction, tty bool) {
	const (
		hdr = "%-6s  %-9s  %-5s  %-16s  %-20s  %-9s\n"
		row = "%-6d  %-9s  %.2f   %-16s  %-20s  %s\n"
	)

	if tty {
		fmt.Printf(hdr, "ID", "STATUS", "CONF", "TYPE", "TARGET", "EXPIRES")
		fmt.Println(strings.Repeat("-", 72))
	} else {
		// Machine-readable header for piped output.
		fmt.Printf("ID\tSTATUS\tCONF\tTYPE\tTARGET\tEXPIRES\n")
	}

	for _, a := range actions {
		exp := expiresLabel(a)
		target := a.Target
		if len(target) > 20 {
			target = target[:17] + "..."
		}
		actionType := a.ActionType
		if len(actionType) > 16 {
			actionType = actionType[:13] + "..."
		}

		if tty {
			fmt.Printf(row, a.ID, padStatus(a.Status), a.Confidence, actionType, target, exp)
		} else {
			fmt.Printf("%d\t%s\t%.2f\t%s\t%s\t%s\n",
				a.ID, a.Status, a.Confidence, a.ActionType, a.Target, exp)
		}
	}
}

// padStatus pads a status string to a fixed width for table alignment.
func padStatus(s string) string {
	return fmt.Sprintf("%-9s", s)
}

// expiresLabel formats the ExpiresAt field as a human-readable string.
// "in Xm" / "in Xs" for future; "expired" for past; absolute time when > 24h away.
func expiresLabel(a db.PendingAction) string {
	if a.Status != "pending" {
		// Non-pending rows: show acted_at if available, else creation time.
		if a.ActedBy != nil && *a.ActedBy != "" {
			return fmt.Sprintf("by %s", *a.ActedBy)
		}
		return a.Status
	}
	remaining := time.Until(a.ExpiresAt)
	if remaining <= 0 {
		return "expired"
	}
	if remaining >= 24*time.Hour {
		return a.ExpiresAt.Local().Format("Jan 02 15:04")
	}
	remaining = remaining.Round(time.Second)
	if remaining >= time.Minute {
		return fmt.Sprintf("in %dm", int(remaining.Minutes()))
	}
	return fmt.Sprintf("in %ds", int(remaining.Seconds()))
}

// ── approve ───────────────────────────────────────────────────────────────────

// runQueueApprove implements `devtrack queue approve <id>`.
// It marks the action approved in SQLite, then calls POST /queue/execute immediately.
func runQueueApprove() error {
	id, err := parseQueueID("approve")
	if err != nil {
		return err
	}

	d, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("queue approve: open database: %w", err)
	}
	defer d.Close()

	if err := d.UpdatePendingActionStatus(id, "approved", "cli"); err != nil {
		return fmt.Errorf("queue approve: update status: %w", err)
	}

	// Fire the action immediately via the Python queue endpoint.
	tc := trigger.NewHTTPTriggerClient()
	resp, err := tc.ExecuteQueueAction(id)
	if err != nil {
		// Action is already marked approved in the DB; the queue executor will
		// handle it on the next poll. Surface the error so the user knows.
		fmt.Fprintf(os.Stderr, "warning: approved locally but server execution failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "         The action will be dispatched on the next queue poll.\n")
		return nil
	}
	if resp.Status == "failed" {
		return fmt.Errorf("approved: action %d dispatched but server reported failure: %s", id, resp.Error)
	}

	fmt.Printf("approved: action %d dispatched\n", id)
	return nil
}

// ── reject ────────────────────────────────────────────────────────────────────

// runQueueReject implements `devtrack queue reject <id>`.
func runQueueReject() error {
	id, err := parseQueueID("reject")
	if err != nil {
		return err
	}

	d, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("queue reject: open database: %w", err)
	}
	defer d.Close()

	if err := d.UpdatePendingActionStatus(id, "rejected", "cli"); err != nil {
		return fmt.Errorf("queue reject: %w", err)
	}

	fmt.Printf("rejected: action %d will not be dispatched\n", id)
	return nil
}

// ── edit ──────────────────────────────────────────────────────────────────────

// runQueueEdit implements `devtrack queue edit <id> <json>`.
// The JSON argument replaces the action's payload. It must be valid JSON.
// After a successful edit the action is NOT automatically approved — the user
// must run `devtrack queue approve <id>` separately if desired.
func runQueueEdit() error {
	args := os.Args[3:] // everything after "edit"
	if len(args) < 2 {
		fmt.Println("Usage: devtrack queue edit <id> <json>")
		return fmt.Errorf("queue edit: missing arguments")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("queue edit: invalid ID %q: %w", args[0], err)
	}

	rawJSON := strings.Join(args[1:], " ")
	if !json.Valid([]byte(rawJSON)) {
		return fmt.Errorf("queue edit: invalid JSON: %s", rawJSON)
	}

	d, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("queue edit: open database: %w", err)
	}
	defer d.Close()

	if err := d.UpdatePendingActionPayload(id, rawJSON); err != nil {
		return fmt.Errorf("queue edit: %w", err)
	}

	fmt.Printf("updated: action %d payload replaced (run 'devtrack queue approve %d' to dispatch)\n", id, id)
	return nil
}

// ── status ────────────────────────────────────────────────────────────────────

// runQueueStatus implements `devtrack queue status`.
// Prints a one-line summary: Pending: N | Posted today: N | Rejected today: N
func runQueueStatus() error {
	d, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("queue status: open database: %w", err)
	}
	defer d.Close()

	pending, posted, rejected, err := d.CountPendingActionsRecent()
	if err != nil {
		return fmt.Errorf("queue status: %w", err)
	}

	fmt.Printf("Pending: %d | Posted today: %d | Rejected today: %d\n", pending, posted, rejected)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// parseQueueID extracts and parses the integer ID argument for approve/reject subcommands.
// subCmd is used only to compose a useful error message.
func parseQueueID(subCmd string) (int64, error) {
	if len(os.Args) < 4 {
		fmt.Printf("Usage: devtrack queue %s <id>\n", subCmd)
		return 0, fmt.Errorf("queue %s: missing action ID", subCmd)
	}
	id, err := strconv.ParseInt(os.Args[3], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("queue %s: invalid ID %q: %w", subCmd, os.Args[3], err)
	}
	return id, nil
}

// printQueueUsage prints a short usage block for the queue command group.
func printQueueUsage() {
	fmt.Println("Usage:")
	fmt.Println("  devtrack queue [list] [--all]    List pending (or all recent) actions")
	fmt.Println("  devtrack queue approve <id>      Approve and immediately dispatch action")
	fmt.Println("  devtrack queue reject  <id>      Reject action (will not be dispatched)")
	fmt.Println("  devtrack queue edit    <id> <json> Replace payload JSON of action")
	fmt.Println("  devtrack queue status            Show summary: pending / posted / rejected")
}
