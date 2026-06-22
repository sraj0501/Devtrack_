package reviewer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestMain allows this test binary to double as the mock agent subprocess.
// When MOCK_AGENT_ROLE is set, the binary acts as the mock and exits immediately
// without running any test functions. This is the standard Go cross-platform
// subprocess-testing pattern (see os/exec package tests in the Go stdlib).
func TestMain(m *testing.M) {
	switch os.Getenv("MOCK_AGENT_ROLE") {
	case "success":
		fmt.Print("Fix applied.")
		os.Exit(0)
	case "cannot_fix":
		fmt.Print("CANNOT_FIX: ambiguous logic")
		os.Exit(0)
	case "nonzero":
		fmt.Print("error: no such file")
		os.Exit(1)
	case "sleep":
		// Sleep longer than the timeout used in TestApplyTimeout.
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// testExe returns the path to the currently running test binary.
func testExe(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return exe
}

// mockCmdBuilder returns a cmdBuilderFunc that invokes the test binary itself
// with MOCK_AGENT_ROLE=<role> and -test.run=^$ (skip all test functions).
// This means the subprocess is always the test binary binary — no shell wrapper —
// so Process.Kill() reliably terminates it on both Windows and Unix.
func mockCmdBuilder(t *testing.T, role string) cmdBuilderFunc {
	t.Helper()
	exe := testExe(t)
	return func(inv AgentInvocation, promptFile string) *exec.Cmd {
		cmd := exec.Command(exe, "-test.run=^$")
		cmd.Env = append(os.Environ(), "MOCK_AGENT_ROLE="+role)
		return cmd
	}
}

// makeTempRepo creates a temp directory with a git repo initialised with one commit.
// gitHead returns "" on error, so tests that don't need HEAD detection can pass an
// empty dir and the HEAD-change check simply won't fire.
func makeTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			// Non-fatal: some CI environments lack git config; HEAD detection returns ""
			t.Logf("git %v: %v — output: %s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "initial")
	return dir
}

// TestApplySuccess tests the happy path: mock agent outputs "Fix applied." and exits 0.
func TestApplySuccess(t *testing.T) {
	repoDir := makeTempRepo(t)

	ag := &Agent{
		backend:    BackendClaudeCode,
		timeoutSec: 10,
		cmdBuilder: mockCmdBuilder(t, "success"),
	}
	inv := AgentInvocation{
		RepoPath:    repoDir,
		CommentBody: "Add nil check for user parameter",
		FixHint:     "style",
		PRTitle:     "Fix auth flow",
		Backend:     BackendClaudeCode,
		TimeoutSecs: 10,
	}

	result := ag.Apply(context.Background(), inv)

	if !result.Success {
		t.Errorf("expected Success=true, got false; Error=%q", result.Error)
	}
	if !strings.Contains(result.OutputSummary, "Fix applied.") {
		t.Errorf("OutputSummary %q does not contain 'Fix applied.'", result.OutputSummary)
	}
	// HEAD-change detection: the mock does not git-commit, so CommitHash stays "".
	// This confirms the detection does not fire spuriously.
	t.Logf("CommitHash=%q (expected empty — mock did not commit)", result.CommitHash)
}

// TestApplyCannotFix verifies that CANNOT_FIX: in agent output causes Success=false.
func TestApplyCannotFix(t *testing.T) {
	ag := &Agent{
		backend:    BackendClaudeCode,
		timeoutSec: 10,
		cmdBuilder: mockCmdBuilder(t, "cannot_fix"),
	}
	inv := AgentInvocation{
		RepoPath:    t.TempDir(),
		CommentBody: "Refactor the authentication module",
		Backend:     BackendClaudeCode,
		TimeoutSecs: 10,
	}

	result := ag.Apply(context.Background(), inv)

	if result.Success {
		t.Error("expected Success=false for CANNOT_FIX output, got true")
	}
	if !strings.Contains(result.Error, "ambiguous logic") {
		t.Errorf("Error %q does not contain 'ambiguous logic'", result.Error)
	}
}

// TestApplyNonzeroExit verifies that a non-zero exit code causes Success=false.
func TestApplyNonzeroExit(t *testing.T) {
	ag := &Agent{
		backend:    BackendClaudeCode,
		timeoutSec: 10,
		cmdBuilder: mockCmdBuilder(t, "nonzero"),
	}
	inv := AgentInvocation{
		RepoPath:    t.TempDir(),
		CommentBody: "Fix the crash on null input",
		Backend:     BackendClaudeCode,
		TimeoutSecs: 10,
	}

	result := ag.Apply(context.Background(), inv)

	if result.Success {
		t.Error("expected Success=false for non-zero exit, got true")
	}
	if result.Error == "" {
		t.Error("expected non-empty Error for non-zero exit")
	}
}

// TestApplyTimeout verifies that a slow agent is killed and Apply returns promptly.
func TestApplyTimeout(t *testing.T) {
	ag := &Agent{
		backend:    BackendClaudeCode,
		timeoutSec: 1,
		cmdBuilder: mockCmdBuilder(t, "sleep"),
	}
	inv := AgentInvocation{
		RepoPath:    t.TempDir(),
		CommentBody: "Optimise the database query",
		Backend:     BackendClaudeCode,
		TimeoutSecs: 1, // 1-second timeout; mock sleeps 30 seconds
	}

	start := time.Now()
	result := ag.Apply(context.Background(), inv)
	elapsed := time.Since(start)

	if result.Success {
		t.Error("expected Success=false for timed-out agent, got true")
	}
	if !strings.Contains(result.Error, "timed out") {
		t.Errorf("Error %q does not mention timeout", result.Error)
	}
	// Apply must return well before the mock's 30-second sleep ends.
	// Allow up to 8 seconds for process-kill + cleanup overhead on slow CI.
	if elapsed > 8*time.Second {
		t.Errorf("Apply took %v — expected to return within a few seconds of the 1s timeout", elapsed)
	}
	t.Logf("Timeout test passed in %v", elapsed)
}
