package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	cli "gitlab.com/devtrack3_cloud/devtrack_cli"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "help", "--help", "-h":
		printUsage()
		return
	case "start", "stop", "status", "logs", "pause", "resume",
		"force-trigger", "version", "health":
		// valid — fall through to load env and dispatch
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", cmd)
		fmt.Fprintln(os.Stderr, "Run 'devtrack-cli help' for usage.")
		os.Exit(1)
	}

	loadEnvFile()

	serverURL := os.Getenv("DEVTRACK_SERVER_URL")
	if serverURL == "" {
		die("DEVTRACK_SERVER_URL is required (e.g. http://localhost:8765)")
	}
	token := os.Getenv("DEVTRACK_API_TOKEN")
	client := cli.NewCLIClient(serverURL, token)

	switch cmd {
	case "start":
		resp, err := client.Start()
		check(err)
		fmt.Println(resp.Message)

	case "stop":
		resp, err := client.Stop()
		check(err)
		fmt.Println(resp.Message)

	case "status":
		resp, err := client.Status()
		check(err)
		if resp.Running {
			fmt.Printf("running  pid=%d  uptime=%s", resp.PID, resp.Uptime)
			if resp.Paused {
				fmt.Print("  [paused]")
			}
			fmt.Println()
		} else {
			fmt.Println("stopped")
		}

	case "logs":
		tail := 50
		if len(os.Args) > 2 {
			if n, err := strconv.Atoi(os.Args[2]); err == nil {
				tail = n
			}
		}
		resp, err := client.Logs(tail)
		check(err)
		for _, line := range resp.Lines {
			fmt.Println(line)
		}

	case "pause":
		resp, err := client.Pause()
		check(err)
		fmt.Println(resp.Message)

	case "resume":
		resp, err := client.Resume()
		check(err)
		fmt.Println(resp.Message)

	case "force-trigger":
		resp, err := client.ForceTrigger()
		check(err)
		fmt.Println(resp.Message)

	case "version":
		resp, err := client.Version()
		check(err)
		fmt.Printf("%s (go %s)\n", resp.Version, resp.GoVersion)

	case "health":
		resp, err := client.Health()
		check(err)
		if resp.OK {
			fmt.Printf("ok  version=%s\n", resp.Version)
		} else {
			fmt.Println("unhealthy")
			os.Exit(1)
		}

	}
}

func check(err error) {
	if err != nil {
		die(err.Error())
	}
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

// loadEnvFile loads DEVTRACK_ENV_FILE (or .env in the working directory) if present.
// Already-set environment variables are never overwritten.
func loadEnvFile() {
	path := os.Getenv("DEVTRACK_ENV_FILE")
	if path == "" {
		path = ".env"
	}
	// godotenv.Overload would overwrite existing vars; Load does not.
	_ = godotenv.Load(path)
}

func printUsage() {
	fmt.Println("devtrack-cli — thin HTTP client for devtrack-server")
	fmt.Println()
	fmt.Println("Usage: devtrack-cli <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  start           check server is running")
	fmt.Println("  stop            stop the daemon")
	fmt.Println("  status          show daemon status")
	fmt.Println("  logs [N]        show last N log lines (default 50)")
	fmt.Println("  pause           pause the scheduler")
	fmt.Println("  resume          resume the scheduler")
	fmt.Println("  force-trigger   fire an immediate trigger")
	fmt.Println("  version         show server version")
	fmt.Println("  health          check server health")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  DEVTRACK_SERVER_URL   server base URL (required)")
	fmt.Println("  DEVTRACK_API_TOKEN    auth token (optional)")
	fmt.Println("  DEVTRACK_ENV_FILE     path to .env file (default: .env)")
}
