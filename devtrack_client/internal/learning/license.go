package learning

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ── License / Auth CLI handlers ───────────────────────────────────────────────

// HandleLogin runs the interactive login flow via Python auth module.
func HandleLogin(projectRoot string) error {
	return runPythonAuthCommand(projectRoot, "login")
}

// HandleLogout clears the local session.
func HandleLogout(projectRoot string) error {
	return runPythonAuthCommand(projectRoot, "logout")
}

// HandleWhoami shows the current session info.
func HandleWhoami(projectRoot string) error {
	return runPythonAuthCommand(projectRoot, "whoami")
}

// HandleLicense shows the current licence status and tier.
func HandleLicense(projectRoot string, args []string) error {
	if len(args) > 0 && args[0] == "--accept" {
		return runPythonAuthCommand(projectRoot, "terms", "--accept")
	}
	return runPythonAuthCommand(projectRoot, "license")
}

// HandleTerms shows the Terms of Service and optionally prompts acceptance.
func HandleTerms(projectRoot string, args []string) error {
	if len(args) > 0 && args[0] == "--accept" {
		return runPythonAuthCommand(projectRoot, "terms", "--accept")
	}
	return runPythonAuthCommand(projectRoot, "terms")
}

// HandleTelemetry enables or disables telemetry.
func HandleTelemetry(projectRoot string, args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: devtrack telemetry [on|off|status]")
		return nil
	}
	return runPythonAuthCommand(projectRoot, "telemetry", args...)
}

// ── First-run T&C check ───────────────────────────────────────────────────────

// EnsureTermsAccepted checks if T&C have been accepted, prompting if not.
// Returns false if the user declines — caller should exit.
// Never fails on errors (offline-safe).
func EnsureTermsAccepted(projectRoot string, cmdArgs []string) bool {
	// Skip for non-interactive commands where T&C don't apply
	if len(cmdArgs) > 1 {
		cmd := cmdArgs[1]
		switch cmd {
		case "terms", "license", "help", "version", "shell-init":
			return true
		}
	}

	// Check via Python license_manager
	accepted, err := checkTermsAccepted(projectRoot)
	if err != nil || accepted {
		return true // On error, don't block — offline safety
	}

	// Not yet accepted — show prompt
	return promptTermsAcceptance(projectRoot)
}

// checkTermsAccepted returns true if terms are already accepted locally.
func checkTermsAccepted(projectRoot string) (bool, error) {
	uvPath, err := exec.LookPath("uv")
	if err != nil {
		return true, nil // uv not available, don't block
	}

	cmd := exec.Command(uvPath, "run", "python", "-c",
		"from backend.license_manager import is_accepted; print('yes' if is_accepted() else 'no')")
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(),
		"PROJECT_ROOT="+projectRoot,
		"DEVTRACK_ENV_FILE="+filepath.Join(projectRoot, ".env"),
	)

	out, err := cmd.Output()
	if err != nil {
		return true, nil // On error, don't block
	}

	return strings.TrimSpace(string(out)) == "yes", nil
}

// promptTermsAcceptance runs the interactive T&C prompt.
func promptTermsAcceptance(projectRoot string) bool {
	uvPath, err := exec.LookPath("uv")
	if err != nil {
		return true // uv not available, don't block
	}

	nonInteractive := "False"
	if os.Getenv("DEVTRACK_AUTO_ACCEPT_TERMS") == "1" {
		nonInteractive = "True"
	}

	cmd := exec.Command(uvPath, "run", "python", "-c",
		fmt.Sprintf(
			"from backend.license_manager import ensure_accepted; import sys; sys.exit(0 if ensure_accepted(non_interactive=%s) else 1)",
			nonInteractive,
		),
	)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(),
		"PROJECT_ROOT="+projectRoot,
		"DEVTRACK_ENV_FILE="+filepath.Join(projectRoot, ".env"),
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	return err == nil
}

// ── Python bridge helpers ────────────────────────────────────────────────────

// runPythonAuthCommand delegates auth/license commands to the Python layer.
func runPythonAuthCommand(projectRoot, subcmd string, extraArgs ...string) error {
	if projectRoot == "" {
		projectRoot = resolveProjectRoot()
	}

	uvPath, err := exec.LookPath("uv")
	if err != nil {
		return fmt.Errorf("uv not found — is DevTrack installed correctly?")
	}

	pyCode := buildAuthPyCode(subcmd, extraArgs)

	cmd := exec.Command(uvPath, "run", "python", "-c", pyCode)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(),
		"PROJECT_ROOT="+projectRoot,
		"DEVTRACK_ENV_FILE="+filepath.Join(projectRoot, ".env"),
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

func buildAuthPyCode(subcmd string, args []string) string {
	switch subcmd {
	case "login":
		return `
from backend.auth.cloud_auth import interactive_login
interactive_login()
`
	case "logout":
		return `
from backend.auth.session import clear_session, is_logged_in
if is_logged_in():
    clear_session()
    print("Logged out successfully.")
else:
    print("Not currently logged in.")
`
	case "whoami":
		return `
from backend.auth.session import get_session, is_logged_in
if is_logged_in():
    s = get_session()
    print(f"Email   : {s.email}")
    print(f"Name    : {s.display_name}")
    print(f"Tier    : {s.tier}")
    print(f"Mode    : {s.mode}")
    print(f"Telemetry: {'enabled' if s.telemetry_enabled else 'disabled'}")
    print(f"Expires : {s.token_expires_at}")
else:
    print("Not logged in.")
    print("Run 'devtrack login' to authenticate (optional for personal use).")
`
	case "license":
		return `
from backend.license_manager import show_license_status, detect_tier
show_license_status()
`
	case "terms":
		if len(args) > 0 && args[0] == "--accept" {
			return `
from backend.license_manager import ensure_accepted
import sys
sys.exit(0 if ensure_accepted() else 1)
`
		}
		return `
from backend.license_manager import show_terms
show_terms()
print()
print("To accept: devtrack terms --accept")
print("Full file: TERMS.md")
`
	case "telemetry":
		if len(args) > 0 {
			switch args[0] {
			case "on":
				return `
from backend.auth.session import get_session, set_session, is_logged_in
if not is_logged_in():
    print("Telemetry requires login. Run: devtrack login")
else:
    s = get_session()
    s.telemetry_enabled = True
    s.save()
    print("Telemetry enabled.")
`
			case "off":
				return `
from backend.auth.session import get_session, is_logged_in
if is_logged_in():
    s = get_session()
    s.telemetry_enabled = False
    s.save()
    print("Telemetry disabled.")
else:
    print("Telemetry is already off (not logged in).")
`
			}
		}
		return `
from backend.auth.session import get_session, is_logged_in
if is_logged_in():
    s = get_session()
    status = "enabled" if s.telemetry_enabled else "disabled"
    print(f"Telemetry: {status}")
else:
    print("Telemetry: disabled (not logged in)")
print()
print("devtrack telemetry on   — enable")
print("devtrack telemetry off  — disable")
`
	default:
		return fmt.Sprintf(`print("Unknown auth command: %s")`, subcmd)
	}
}

// resolveProjectRoot finds the project root from env or binary location.
func resolveProjectRoot() string {
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		return root
	}
	execPath, err := os.Executable()
	if err != nil {
		return "."
	}
	execPath, _ = filepath.Abs(execPath)
	searchDir := filepath.Dir(execPath)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(searchDir, "backend")); err == nil {
			return searchDir
		}
		parent := filepath.Dir(searchDir)
		if parent == searchDir {
			break
		}
		searchDir = parent
	}
	return "."
}

// PrintLicenseSection writes the license help line to a bufio.Writer.
func PrintLicenseSection(w *bufio.Writer) {
	fmt.Fprintln(w, "ACCOUNT:   login | logout | whoami | license | terms | telemetry [on|off]")
}

// Ensure bufio import is used
var _ = bufio.NewWriter
var _ = strings.TrimSpace
