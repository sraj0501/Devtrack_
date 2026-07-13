package learning

// license.go — Auth/license helpers for the learning package.
// All calls route through the devtrack_server HTTP API (Phase 1c).
// The package-main versions of these handlers live in license_cli.go.

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	trig "github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// HandleLogin runs the interactive magic-link login flow via the server.
func HandleLogin(projectRoot string) error {
	c := trig.NewHTTPTriggerClient()

	fmt.Print("Enter your email address: ")
	reader := bufio.NewReader(os.Stdin)
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email address required")
	}

	msg, err := c.AuthRequestMagicLink(email)
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

	session, err := c.AuthVerifyMagicLink(email, code)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if !session.Success {
		return fmt.Errorf("login failed: %s", session.Message)
	}
	fmt.Printf("Logged in as %s (%s tier)\n", session.Email, session.Tier)
	return nil
}

// HandleLogout clears the session via the server.
func HandleLogout(projectRoot string) error {
	msg, err := trig.NewHTTPTriggerClient().AuthLogout()
	if err != nil {
		return fmt.Errorf("logout: %w (is the server running?)", err)
	}
	fmt.Println(msg)
	return nil
}

// HandleWhoami shows the current session info via the server.
func HandleWhoami(projectRoot string) error {
	s, err := trig.NewHTTPTriggerClient().AuthWhoami()
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

// HandleLicense shows the current licence status and tier.
func HandleLicense(projectRoot string, args []string) error {
	if len(args) > 0 && args[0] == "--accept" {
		msg, err := trig.NewHTTPTriggerClient().LicenseAccept()
		if err != nil {
			return fmt.Errorf("license --accept: %w (is the server running?)", err)
		}
		fmt.Println(msg)
		return nil
	}
	output, err := trig.NewHTTPTriggerClient().LicenseStatus()
	if err != nil {
		return fmt.Errorf("license: %w (is the server running?)", err)
	}
	fmt.Print(output)
	return nil
}

// HandleTerms shows the Terms of Service.
func HandleTerms(projectRoot string, args []string) error {
	if len(args) > 0 && args[0] == "--accept" {
		msg, err := trig.NewHTTPTriggerClient().LicenseAccept()
		if err != nil {
			return fmt.Errorf("terms --accept: %w (is the server running?)", err)
		}
		fmt.Println(msg)
		return nil
	}
	output, err := trig.NewHTTPTriggerClient().LicenseTerms()
	if err != nil {
		return fmt.Errorf("terms: %w (is the server running?)", err)
	}
	fmt.Print(output)
	fmt.Println()
	fmt.Println("To accept: devtrack terms --accept")
	return nil
}

// Telemetry consent is client-local and opt-in: see daemon.SetTelemetryEnabled.
// It is deliberately not routed through the server — the daemon reads the local
// marker directly, so `devtrack telemetry off` works with no server running.

// EnsureTermsAccepted checks if T&C have been accepted, prompting if not.
// Fails open on any error (offline-safe). cmdArgs is os.Args-style.
func EnsureTermsAccepted(projectRoot string, cmdArgs []string) bool {
	if len(cmdArgs) > 1 {
		switch cmdArgs[1] {
		case "terms", "license", "help", "version", "shell-init":
			return true
		}
	}
	c := trig.NewHTTPTriggerClient()
	accepted, _ := c.LicenseIsAccepted()
	return accepted // fails open
}

// PrintLicenseSection writes the license help line to a bufio.Writer.
func PrintLicenseSection(w *bufio.Writer) {
	fmt.Fprintln(w, "ACCOUNT:   login | logout | whoami | license | terms | telemetry [on|off]")
}
