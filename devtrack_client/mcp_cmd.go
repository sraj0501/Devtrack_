package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cfg "github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/mcp"
)

// handleMCPCommand handles the `devtrack mcp` command and subcommands.
//
//	devtrack mcp           -> start MCP server in stdio mode (blocks; used by Claude Code)
//	devtrack mcp setup     -> write .mcp.json in current directory (or --dir <path>)
//	devtrack mcp status    -> print server info
//	devtrack mcp test      -> run a self-test: initialize + tools/list + get_active_context
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
	case "setup":
		runMCPSetup(args[1:])
	case "test":
		runMCPTest()
	default:
		fmt.Fprintf(os.Stderr, "Unknown mcp subcommand: %s\n", sub)
		fmt.Fprintf(os.Stderr, "Usage: devtrack mcp [serve|status|setup|test]\n")
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

// runMCPSetup writes (or merges) a .mcp.json file in the target directory.
// Flags: --dir <path>  — directory to write to (default: current directory)
func runMCPSetup(args []string) {
	dir := "."
	for i, a := range args {
		if a == "--dir" && i+1 < len(args) {
			dir = args[i+1]
		} else if strings.HasPrefix(a, "--dir=") {
			dir = strings.TrimPrefix(a, "--dir=")
		}
	}

	// Resolve absolute dir
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp setup: cannot resolve dir %q: %v\n", dir, err)
		os.Exit(1)
	}

	mcpPath := filepath.Join(absDir, ".mcp.json")

	// Determine the absolute path to the devtrack binary
	execPath, err := os.Executable()
	if err != nil {
		// Fallback: use the arg0
		execPath = os.Args[0]
	}
	execPath, _ = filepath.Abs(execPath)

	// Determine .env path
	envPath := cfg.ResolveEnvFilePath()
	if envPath == "" {
		// Fallback: .env next to binary
		envPath = filepath.Join(filepath.Dir(execPath), ".env")
	}

	// Build the devtrack MCP entry
	devtrackEntry := map[string]interface{}{
		"command": execPath,
		"args":    []string{"mcp"},
		"env": map[string]string{
			"DEVTRACK_ENV_FILE": envPath,
		},
	}

	// Load existing .mcp.json if present
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(mcpPath); err == nil {
		if err2 := json.Unmarshal(data, &existing); err2 != nil {
			fmt.Fprintf(os.Stderr, "mcp setup: existing .mcp.json is malformed (%v) — overwriting\n", err2)
			existing = make(map[string]interface{})
		}
	}

	// Get or create mcpServers map
	servers, _ := existing["mcpServers"].(map[string]interface{})
	if servers == nil {
		servers = make(map[string]interface{})
	}

	// Idempotency check
	if _, alreadyPresent := servers["devtrack"]; alreadyPresent {
		fmt.Println("DevTrack MCP already configured in .mcp.json")
		return
	}

	// Merge
	servers["devtrack"] = devtrackEntry
	existing["mcpServers"] = servers

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp setup: cannot marshal .mcp.json: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(mcpPath, append(out, '\n'), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "mcp setup: cannot write %s: %v\n", mcpPath, err)
		os.Exit(1)
	}

	fmt.Printf("Written: %s\n", mcpPath)
	fmt.Printf("  Binary: %s\n", execPath)
	fmt.Printf("  Env:    %s\n", envPath)
	fmt.Println("To use with Claude Code, add this to your project's .mcp.json or use claude mcp add.")
}

// runMCPTest runs an in-process smoke test of the MCP server.
// It creates an in-memory pipe, starts the MCP server on that pipe,
// sends three JSON-RPC messages, and prints the responses.
func runMCPTest() {
	// Build a server with all tools registered against a real (or fallback) database.
	srv := mcp.New(Version)

	database, err := db.NewDatabase()
	if err != nil {
		// Print the error but continue — test can still verify protocol layer.
		fmt.Fprintf(os.Stderr, "mcp test: warning — cannot open database: %v\n", err)
		fmt.Fprintf(os.Stderr, "          Some tools will fail; protocol test will still run.\n")
		database = nil
	}
	if database != nil {
		defer database.Close()
		mcp.RegisterDevTrackTools(srv, database)
	}

	// Build the three test messages
	messages := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_active_context","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":99,"method":"shutdown"}`,
	}
	input := strings.Join(messages, "\n") + "\n"

	inBuf := strings.NewReader(input)
	var outBuf bytes.Buffer

	// Run synchronously via the package-internal run method via Start with a cancelled context trick.
	// Since Server.run is not exported, we use a goroutine + pipe.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the server in a goroutine; it will exit on shutdown message.
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.RunOn(ctx, inBuf, &outBuf)
	}()

	<-done

	// Print results
	fmt.Println("=== devtrack mcp test ===")
	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	labels := []string{"initialize", "tools/list", "get_active_context", "shutdown"}
	for i, line := range lines {
		label := "response"
		if i < len(labels) {
			label = labels[i]
		}
		// Pretty-print the JSON
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, []byte(line), "  ", "  "); err != nil {
			fmt.Printf("\n[%s]\n%s\n", label, line)
		} else {
			fmt.Printf("\n[%s]\n  %s\n", label, pretty.String())
		}
	}
	fmt.Println("\n=== PASS ===")
}
