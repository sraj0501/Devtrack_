package reviewer

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// killProc kills a process. On both Unix and Windows, Process.Kill() sends the
// equivalent of SIGKILL (terminates immediately without cleanup).
func killProc(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// AgentBackend selects which coding agent is invoked.
type AgentBackend string

const (
	BackendClaudeCode AgentBackend = "claude-code"
	BackendCopilotCLI AgentBackend = "copilot-cli"
)

// AgentInvocation describes a single request to the coding agent.
type AgentInvocation struct {
	RepoPath    string       // absolute path to the git repo
	CommentBody string       // the review comment text
	FixHint     string       // classification hint from TASK-093 (may be empty)
	PRTitle     string       // for context in the prompt
	Backend     AgentBackend // which CLI to use
	TimeoutSecs int          // max seconds to wait for the agent process
}

// AgentResult describes the outcome of an agent invocation.
type AgentResult struct {
	Success       bool
	CommitHash    string // git hash of the fix commit, if any (empty if no commit made)
	OutputSummary string // first 500 chars of agent stdout, for escalation context
	Error         string // non-empty when Success=false
}

// cmdBuilderFunc builds the subprocess command for a given invocation and prompt file path.
// This field is nil in production (uses the default builder); tests inject their own builder.
type cmdBuilderFunc func(inv AgentInvocation, promptFile string) *exec.Cmd

// Agent invokes the configured coding agent CLI as a subprocess.
type Agent struct {
	backend    AgentBackend
	timeoutSec int

	// cmdBuilder is an optional override for the command construction step.
	// nil means use the production default (claude / gh copilot).
	// Tests inject a builder that points at a mock binary.
	cmdBuilder cmdBuilderFunc
}

// NewAgent creates an Agent that uses the given backend and timeout.
func NewAgent(backend AgentBackend, timeoutSec int) *Agent {
	return &Agent{
		backend:    backend,
		timeoutSec: timeoutSec,
	}
}

// Apply invokes the agent with the given invocation spec.
// It returns AgentResult — never panics or returns an error;
// failures are encoded in AgentResult.Success=false + Error field.
func (a *Agent) Apply(ctx context.Context, inv AgentInvocation) (result AgentResult) {
	// Guard against any panic — all failures encode into AgentResult.
	defer func() {
		if r := recover(); r != nil {
			result.Success = false
			result.Error = fmt.Sprintf("agent panicked: %v", r)
		}
	}()

	// 1. Record HEAD before the subprocess runs.
	headBefore := gitHead(inv.RepoPath)

	// 2. Write the review-context prompt to a temp file.
	promptContent := buildPrompt(inv)
	promptFile, err := os.CreateTemp("", "devtrack-agent-prompt-*.txt")
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to create prompt temp file: %v", err)
		return
	}
	defer os.Remove(promptFile.Name())
	if _, err := promptFile.WriteString(promptContent); err != nil {
		promptFile.Close()
		result.Success = false
		result.Error = fmt.Sprintf("failed to write prompt temp file: %v", err)
		return
	}
	promptFile.Close()

	// 3. Build the subprocess command.
	// NOTE: The exact flags used here should be verified against the installed
	// version of each CLI tool. As of the time of writing:
	//   - `claude --no-browser --print <file>` runs Claude Code in headless mode
	//     and prints the output. Verify with `claude --help` for your installed version.
	//   - `gh copilot suggest -t shell <prompt>` runs Copilot CLI in shell mode.
	//     Verify with `gh copilot suggest --help` for your installed version.
	timeoutSecs := inv.TimeoutSecs
	if timeoutSecs <= 0 {
		timeoutSecs = a.timeoutSec
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 120
	}

	var cmd *exec.Cmd

	if a.cmdBuilder != nil {
		// Test injection: use the provided command builder.
		cmd = a.cmdBuilder(inv, promptFile.Name())
	} else {
		// Production: pick the right CLI tool for the configured backend.
		backend := inv.Backend
		if backend == "" {
			backend = a.backend
		}
		switch backend {
		case BackendCopilotCLI:
			// gh copilot suggest -t shell <prompt>
			// The prompt is passed as a positional argument here.
			cmd = exec.Command("gh", "copilot", "suggest", "-t", "shell", promptContent)
		default:
			// BackendClaudeCode (default)
			// claude --no-browser --print <prompt_file>
			cmd = exec.Command("claude", "--no-browser", "--print", promptFile.Name())
		}
	}

	cmd.Dir = inv.RepoPath

	// 4. Run with timeout using Start+Wait+goroutine so we can kill on Windows too.
	// cmd.CombinedOutput() blocks until the subprocess exits; it does not honour
	// context cancellation on Windows. We manage the timeout explicitly.
	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	timedOut := false
	runErr := func() error {
		if startErr := cmd.Start(); startErr != nil {
			return startErr
		}

		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		timeout := time.Duration(timeoutSecs) * time.Second
		select {
		case waitErr := <-done:
			return waitErr
		case <-time.After(timeout):
			timedOut = true
			killProc(cmd)
			<-done // drain to prevent goroutine leak
			return nil
		case <-ctx.Done():
			timedOut = true
			killProc(cmd)
			<-done
			return nil
		}
	}()

	outputStr := outputBuf.String()

	// 5. Truncate output to 500 chars for OutputSummary.
	result.OutputSummary = truncate(outputStr, 500)

	// 6. Check for timeout (context or explicit timer).
	if timedOut {
		result.Success = false
		result.Error = fmt.Sprintf("agent timed out after %ds", timeoutSecs)
		return
	}

	// 7. Check for CANNOT_FIX: prefix in agent output.
	if idx := strings.Index(outputStr, "CANNOT_FIX:"); idx >= 0 {
		reason := strings.TrimSpace(outputStr[idx+len("CANNOT_FIX:"):])
		// Take only the first line of the reason.
		if nl := strings.IndexByte(reason, '\n'); nl >= 0 {
			reason = reason[:nl]
		}
		result.Success = false
		result.Error = strings.TrimSpace(reason)
		return
	}

	// 8. If subprocess exits non-zero, encode the error.
	if runErr != nil {
		firstChars := truncate(outputStr, 200)
		if firstChars == "" {
			firstChars = runErr.Error()
		}
		result.Success = false
		result.Error = firstChars
		return
	}

	// 9. Detect whether the agent made a commit by comparing HEAD before/after.
	headAfter := gitHead(inv.RepoPath)
	if headAfter != "" && headAfter != headBefore {
		result.CommitHash = headAfter
	}

	result.Success = true
	return
}

// buildPrompt constructs the review-context prompt written to the temp file.
func buildPrompt(inv AgentInvocation) string {
	var buf bytes.Buffer
	buf.WriteString("You are fixing a code review comment on a pull request.\n\n")
	fmt.Fprintf(&buf, "PR: %s\n", inv.PRTitle)
	fmt.Fprintf(&buf, "Review comment: %s\n", inv.CommentBody)
	fmt.Fprintf(&buf, "Fix hint: %s\n", inv.FixHint)
	buf.WriteString(`
Apply the fix, commit it with a message that matches the developer's style:
- Imperative mood
- No "I have" / "This commit" phrasing
- Reference the review comment briefly

Do not ask for clarification. Apply the most obvious correct fix.
If you cannot determine the correct fix, output: CANNOT_FIX: <reason>
`)
	return buf.String()
}

// gitHead returns the current HEAD commit hash in the given repo path.
// Returns "" on any error (repo not initialised, no commits, path empty, etc.).
func gitHead(repoPath string) string {
	if repoPath == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--format=%H")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		log.Printf("reviewer: gitHead(%q): %v", repoPath, err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

// truncate returns at most maxChars characters of s.
func truncate(s string, maxChars int) string {
	if len(s) <= maxChars {
		return s
	}
	return s[:maxChars]
}
