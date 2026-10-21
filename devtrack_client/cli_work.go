package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/ticket"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// Work selects ticket context only. It never starts a timer or a work session.
func (cli *CLI) handleWork() error {
	if len(os.Args) < 3 {
		printWorkUsage()
		return nil
	}
	switch os.Args[2] {
	case "start", "stop", "status":
		return cli.handleWorkTicket(os.Args[2])
	case "report":
		return cli.handleWorkReport()
	default:
		return fmt.Errorf("unknown work subcommand %q; use start <ticket-ref>, stop, status, or report", os.Args[2])
	}
}

func printWorkUsage() {
	fmt.Println(`Usage: devtrack work <subcommand>

  start <ticket-ref>       Select the active ticket for this repository
  stop                    Clear the active ticket selection
  status                  Show the active ticket selection
  report [--email addr]   Generate a commit-based EOD report`)
}

func (cli *CLI) handleWorkTicket(operation string) error {
	ws, err := currentTicketWorkspace()
	if err != nil {
		return err
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("resolve repository: %w", err)
	}
	repoPath := filepath.Clean(strings.TrimSpace(string(out)))
	database, err := NewDatabase()
	if err != nil {
		return err
	}
	defer database.Close()
	switch operation {
	case "start":
		if len(os.Args) != 4 {
			return fmt.Errorf("usage: devtrack work start <ticket-ref>")
		}
		ref := os.Args[3]
		if err := database.SetActiveTicket(ws.Name, repoPath, ref, ticket.ResolverConfig{
			TicketKey: ws.TicketKey, Kinds: ws.TicketKinds, BranchPattern: ws.TicketPattern,
		}); err != nil {
			return err
		}
		fmt.Printf("Active ticket: %s\n", ref)
	case "stop":
		if err := database.ClearActiveTicket(ws.Name, repoPath); err != nil {
			return err
		}
		fmt.Println("Active ticket cleared.")
	case "status":
		ref, err := database.GetActiveTicket(ws.Name, repoPath)
		if err != nil {
			return err
		}
		if ref == "" {
			fmt.Println("No active ticket selected.")
		} else {
			fmt.Printf("Active ticket: %s\n", ref)
		}
	}
	return nil
}

// handleWorkReport runs the EOD report generator via the server.
func (cli *CLI) handleWorkReport() error {
	email := ""
	args := os.Args[3:]
	for i, arg := range args {
		if arg == "--email" && i+1 < len(args) {
			email = args[i+1]
			break
		}
	}

	database, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("work report: open database: %w", err)
	}
	defer database.Close()
	localCommits, err := database.ListTodayCommits("")
	if err != nil {
		return fmt.Errorf("work report: read today's commits: %w", err)
	}
	commits := make([]trigger.EODCommit, 0, len(localCommits))
	for _, commit := range localCommits {
		commits = append(commits, trigger.EODCommit{
			TicketID: commit.TicketID, CommitMessage: commit.Message,
			CommitHash: commit.Hash, Timestamp: commit.Timestamp,
		})
	}

	client := NewHTTPTriggerClient()
	output, err := client.ReportEOD(email, "", commits)
	if err != nil {
		return fmt.Errorf("work report: %w (is the server running?)", err)
	}
	fmt.Print(output)
	return nil
}

// server-tui and admin-start were removed from the client: the server TUI and
// admin web GUI are server-management tools owned by devtrack_server, not the
// standalone client. See docs/CAPABILITIES_OWNERSHIP.md (§11) and the
// client/server-decoupling plan (Phase 1a).
