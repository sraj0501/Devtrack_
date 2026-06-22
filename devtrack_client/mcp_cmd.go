package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/mcp"
)

// handleMCPCommand handles the `devtrack mcp` command and subcommands.
// devtrack mcp          -> start MCP server in stdio mode (blocks; used by Claude Code)
// devtrack mcp status   -> print server info
func handleMCPCommand(args []string) {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "", "serve":
		runMCPServer()
	case "status":
		printMCPStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown mcp subcommand: %s\n", sub)
		fmt.Fprintf(os.Stderr, "Usage: devtrack mcp [serve|status]\n")
		os.Exit(1)
	}
}

func runMCPServer() {
	// MCP server runs on stdio — all logs go to stderr.
	// The Python server does NOT need to be running; all tools are SQLite-backed.
	srv := mcp.New(Version) // Version is the package-level var in version.go

	// Open SQLite database and register all 6 read-only MCP tools (TASK-099).
	database, err := db.NewDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	mcp.RegisterDevTrackTools(srv, database)

	srv.Start(context.Background())
}

func printMCPStatus() {
	fmt.Println("DevTrack MCP Server")
	fmt.Println("  Protocol:  MCP 2024-11-05")
	fmt.Println("  Transport: stdio")
	fmt.Println("  Tools:     6 registered")
	fmt.Println("             get_active_context, get_today_commits, get_pending_actions,")
	fmt.Println("             get_voice_profile, get_ticket_context, get_eod_summary")
	fmt.Println("  Note:      MCP server runs on-demand when spawned by Claude Code.")
	fmt.Println("             No background process needed.")
}
