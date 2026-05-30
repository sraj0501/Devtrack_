package main

// license_cli.go — CLI command handlers for auth/license commands.
// All commands route through the devtrack_server HTTP API (Phase 1c).

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ── License / Auth CLI handlers ───────────────────────────────────────────────

// handleLogin runs the interactive magic-link login flow via the server.
func (cli *CLI) handleLogin() error {
	client := NewHTTPTriggerClient()

	fmt.Print("Enter your email address: ")
	reader := bufio.NewReader(os.Stdin)
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email address required")
	}

	msg, err := client.AuthRequestMagicLink(email)
	if err != nil {
		return fmt.Errorf("login: %w (is the server running?)", err)
	}
	fmt.Println(msg)

	fmt.Print("Enter the code from your email: ")
	code, _ := reader.ReadString('\n')
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("verification code required")
	}

	session, err := client.AuthVerifyMagicLink(email, code)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if !session.Success {
		return fmt.Errorf("login failed: %s", session.Message)
	}

	fmt.Printf("Logged in as %s (%s tier)\n", session.Email, session.Tier)
	return nil
}

// handleLogout clears the local session via the server.
func (cli *CLI) handleLogout() error {
	client := NewHTTPTriggerClient()
	msg, err := client.AuthLogout()
	if err != nil {
		return fmt.Errorf("logout: %w (is the server running?)", err)
	}
	fmt.Println(msg)
	return nil
}

// handleWhoami shows the current session info via the server.
func (cli *CLI) handleWhoami() error {
	client := NewHTTPTriggerClient()
	s, err := client.AuthWhoami()
	if err != nil {
		return fmt.Errorf("whoami: %w (is the server running?)", err)
	}
	if !s.LoggedIn {
		fmt.Println("Not logged in.")
		fmt.Println("Run 'devtrack login' to authenticate (optional for personal use).")
		return nil
	}
	fmt.Printf("Email    : %s\n", s.Email)
	fmt.Printf("Name     : %s\n", s.DisplayName)
	fmt.Printf("Tier     : %s\n", s.Tier)
	fmt.Printf("Mode     : %s\n", s.Mode)
	telemetry := "disabled"
	if s.TelemetryEnabled {
		telemetry = "enabled"
	}
	fmt.Printf("Telemetry: %s\n", telemetry)
	if s.TokenExpiresAt != "" {
		fmt.Printf("Expires  : %s\n", s.TokenExpiresAt)
	}
	return nil
}

// handleLicense shows the current licence status and tier.
func (cli *CLI) handleLicense() error {
	args := os.Args[2:]
	if len(args) > 0 && args[0] == "--accept" {
		return cli.handleTermsAccept()
	}
	client := NewHTTPTriggerClient()
	output, err := client.LicenseStatus()
	if err != nil {
		return fmt.Errorf("license: %w (is the server running?)", err)
	}
	fmt.Print(output)
	return nil
}

// handleTerms shows the Terms of Service and optionally prompts acceptance.
func (cli *CLI) handleTerms() error {
	args := os.Args[2:]
	if len(args) > 0 && args[0] == "--accept" {
		return cli.handleTermsAccept()
	}
	client := NewHTTPTriggerClient()
	output, err := client.LicenseTerms()
	if err != nil {
		return fmt.Errorf("terms: %w (is the server running?)", err)
	}
	fmt.Print(output)
	fmt.Println()
	fmt.Println("To accept: devtrack terms --accept")
	return nil
}

func (cli *CLI) handleTermsAccept() error {
	client := NewHTTPTriggerClient()
	msg, err := client.LicenseAccept()
	if err != nil {
		return fmt.Errorf("terms --accept: %w (is the server running?)", err)
	}
	fmt.Println(msg)
	return nil
}

// handleTelemetry enables or disables telemetry.
func (cli *CLI) handleTelemetry() error {
	args := os.Args[2:]
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}
	client := NewHTTPTriggerClient()
	msg, err := client.AuthTelemetry(action)
	if err != nil {
		return fmt.Errorf("telemetry: %w (is the server running?)", err)
	}
	fmt.Println(msg)
	if action == "status" {
		fmt.Println()
		fmt.Println("devtrack telemetry on   — enable")
		fmt.Println("devtrack telemetry off  — disable")
	}
	return nil
}

// ── First-run T&C check ───────────────────────────────────────────────────────

// EnsureTermsAccepted checks if T&C have been accepted, prompting if not.
// Returns false if the user declines — caller should exit.
// Fails open on any error (offline-safe).
func EnsureTermsAccepted(projectRoot string) bool {
	// Skip for non-interactive commands where T&C don't apply.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "terms", "license", "help", "version", "shell-init":
			return true
		}
	}

	client := NewHTTPTriggerClient()
	accepted, err := client.LicenseIsAccepted()
	if err != nil || accepted {
		return true // fail open — offline safety
	}

	// Not yet accepted — prompt.
	return promptTermsAcceptanceHTTP(client)
}

// promptTermsAcceptanceHTTP shows a simple T&C prompt using the HTTP API.
func promptTermsAcceptanceHTTP(client *HTTPTriggerClient) bool {
	if os.Getenv("DEVTRACK_AUTO_ACCEPT_TERMS") == "1" {
		client.LicenseAccept() //nolint
		return true
	}

	terms, err := client.LicenseTerms()
	if err != nil {
		return true // fail open
	}
	fmt.Println(terms)

	fmt.Print("Do you accept the Terms of Service? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("Terms not accepted. Some features may be unavailable.")
		return false
	}

	if _, err := client.LicenseAccept(); err != nil {
		return true // fail open even if accept call fails
	}
	fmt.Println("Terms accepted. Thank you!")
	return true
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// resolveProjectRoot finds the project root from env or binary location.
// Used by cli_daemon.go for the T&C startup check.
func resolveProjectRoot() string {
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		return root
	}
	return "."
}

func printLicenseSection(w *bufio.Writer) {
	fmt.Fprintln(w, "ACCOUNT:   login | logout | whoami | license | terms | telemetry [on|off]")
}

var _ = strings.TrimSpace
