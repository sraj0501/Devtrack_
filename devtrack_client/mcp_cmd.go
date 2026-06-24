package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/mcp"
)

// handleMCPCommand handles the `devtrack mcp` command and subcommands.
// devtrack mcp          -> start MCP server in stdio mode (blocks; used by Claude Code)
// devtrack mcp status   -> print server info (placeholder until TASK-100)
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
	// Tools will be registered in TASK-099 via mcp.RegisterDevTrackTools(srv, db)
	srv.Start(context.Background())
}

func printMCPStatus() {
	fmt.Println("DevTrack MCP Server")
	fmt.Println("  Protocol:  MCP 2024-11-05")
	fmt.Println("  Transport: stdio")
	fmt.Println("  Tools:     0 registered (TASK-099 will add 6)")
	fmt.Println("  Note:      MCP server runs on-demand when spawned by Claude Code.")
	fmt.Println("             No background process needed.")
}
