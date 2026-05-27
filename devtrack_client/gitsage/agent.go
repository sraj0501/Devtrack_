package gitsage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const maxSteps = 15

// agentStep is the JSON structure the LLM returns for each agentic step.
type agentStep struct {
	Thought   string   `json:"thought"`
	Commands  []string `json:"commands"`
	Done      bool     `json:"done"`
	Summary   string   `json:"summary"`
}

var agentSystemPrompt = strings.TrimSpace(`
You are git-sage, an autonomous git assistant running inside a terminal.
You help developers manage their git repositories safely and efficiently.

When given a task, respond ONLY with valid JSON in this exact format:
{
  "thought": "brief explanation of what you're doing and why",
  "commands": ["git command 1", "git command 2"],
  "done": false,
  "summary": ""
}

When the task is complete, set "done": true and fill "summary" with a short human-readable result.
When "done" is false, "commands" must contain at least one git or shell command to execute.
Commands must be safe: no force-push to main/master, no destructive resets without explicit request.
Each command is a single shell command string. Use only git, echo, cat, ls, mkdir.
`)

// StepLog records HEAD snapshots taken before each command batch,
// enabling undo of any individual agent step.
type StepLog struct {
	heads []string
}

// Record snapshots the current HEAD into the log.
func (s *StepLog) Record(repoPath string) {
	g := NewGitOps(repoPath)
	head, err := g.HEAD()
	if err == nil && head != "" {
		s.heads = append(s.heads, head)
	}
}

// Undo resets the repository to the HEAD recorded N steps ago.
// n=1 undoes the last step, n=2 the one before that, etc.
func (s *StepLog) Undo(repoPath string, n int) error {
	if n <= 0 || n > len(s.heads) {
		return fmt.Errorf("undo: only %d step(s) recorded (requested %d)", len(s.heads), n)
	}
	target := s.heads[len(s.heads)-n]
	g := NewGitOps(repoPath)
	if _, err := g.ResetToRef(target); err != nil {
		return fmt.Errorf("undo: reset to %s failed: %w", target[:8], err)
	}
	s.heads = s.heads[:len(s.heads)-n]
	fmt.Printf("undo: reset to %s\n", target[:8])
	return nil
}

// UndoStep is the standalone entry point for "devtrack sage undo [N]".
// It re-runs a minimal agentic context to undo N steps using a StepLog
// passed in from the caller. If log is nil, reports no history.
func UndoStep(repoPath string, log *StepLog, n int) error {
	if log == nil || len(log.heads) == 0 {
		return fmt.Errorf("undo: no steps recorded in this session")
	}
	return log.Undo(repoPath, n)
}

// Do executes a task autonomously using an agentic loop.
func Do(repoPath, task string, cfg LLMConfig, verbose bool) error {
	if !cfg.Ping() {
		return fmt.Errorf("Ollama is not running at %s\nStart it with: ollama serve", cfg.Host)
	}

	ctx, err := CollectContext(repoPath)
	if err != nil {
		return fmt.Errorf("failed to collect git context: %w", err)
	}

	systemMsg := agentSystemPrompt
	userMsg := fmt.Sprintf("Git repository context:\n%s\nTask: %s", ctx.Format(), task)

	messages := []Message{
		{Role: "system", Content: systemMsg},
		{Role: "user", Content: userMsg},
	}

	fmt.Printf("sage: starting task — %s\n\n", task)

	for range maxSteps {
		raw, err := cfg.ChatJSON(messages)
		if err != nil {
			return err
		}

		if verbose {
			fmt.Printf("[raw] %s\n\n", raw)
		}

		// Attempt JSON parse; if the model returned prose, show it and stop.
		var parsed agentStep
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			fmt.Println(raw)
			return nil
		}

		if parsed.Thought != "" {
			fmt.Printf("  → %s\n", parsed.Thought)
		}

		if parsed.Done {
			fmt.Println()
			if parsed.Summary != "" {
				fmt.Printf("done: %s\n", parsed.Summary)
			} else {
				fmt.Println("done.")
			}
			return nil
		}

		if len(parsed.Commands) == 0 {
			return fmt.Errorf("sage: model returned no commands and done=false — stopping")
		}

		// Execute each command and collect output
		var execOutput strings.Builder
		for _, cmdStr := range parsed.Commands {
			if err := safetyCheck(cmdStr); err != nil {
				fmt.Printf("  blocked: %v\n", err)
				fmt.Fprintf(&execOutput, "$ %s\nBLOCKED: %v\n", cmdStr, err)
				continue
			}
			fmt.Printf("  $ %s\n", cmdStr)
			out, runErr := runCommand(repoPath, cmdStr)
			if out != "" {
				fmt.Println("   ", strings.ReplaceAll(out, "\n", "\n    "))
				fmt.Fprintf(&execOutput, "$ %s\n%s\n", cmdStr, out)
			}
			if runErr != nil {
				fmt.Fprintf(&execOutput, "$ %s\nERROR: %v\n", cmdStr, runErr)
			}
		}

		// Feed execution results back to model
		messages = append(messages,
			Message{Role: "assistant", Content: raw},
			Message{Role: "user", Content: fmt.Sprintf("Command output:\n%s\nContinue.", execOutput.String())},
		)
		fmt.Println()
	}

	return fmt.Errorf("sage: reached max steps (%d) without completing task", maxSteps)
}

// runCommand executes a single shell command string in the repo directory.
func runCommand(repoPath, cmdStr string) (string, error) {
	parts := splitCommand(cmdStr)
	if len(parts) == 0 {
		return "", nil
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = repoPath
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

// splitCommand splits a command string into argv respecting quoted strings.
func splitCommand(s string) []string {
	var args []string
	var current strings.Builder
	inQuote := rune(0)

	for _, ch := range s {
		switch {
		case ch == inQuote:
			inQuote = 0
		case inQuote != 0:
			current.WriteRune(ch)
		case ch == '\'' || ch == '"':
			inQuote = ch
		case ch == ' ':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// unmarshalAgentStep decodes raw JSON into an agentStep. Exported for use in cli.go.
func unmarshalAgentStep(raw string, out *agentStep) error {
	return json.Unmarshal([]byte(raw), out)
}

// safetyCheck returns an error if the command would be destructive on protected branches.
func safetyCheck(cmdStr string) error {
	lower := strings.ToLower(cmdStr)
	if strings.Contains(lower, "push") && strings.Contains(lower, "--force") {
		if strings.Contains(lower, "main") || strings.Contains(lower, "master") {
			return fmt.Errorf("blocked: force-push to main/master is not allowed")
		}
	}
	return nil
}

// Ask sends a one-shot question about the repo to the LLM and prints the answer.
func Ask(repoPath, question string, cfg LLMConfig) error {
	if !cfg.Ping() {
		return fmt.Errorf("Ollama is not running at %s\nStart it with: ollama serve", cfg.Host)
	}

	ctx, err := CollectContext(repoPath)
	if err != nil {
		return fmt.Errorf("failed to collect git context: %w", err)
	}

	systemMsg := "You are git-sage, a helpful git assistant. Answer the developer's question about their repository concisely and accurately."
	userMsg := fmt.Sprintf("Git repository context:\n%s\nQuestion: %s", ctx.Format(), question)

	answer, err := cfg.Chat([]Message{
		{Role: "system", Content: systemMsg},
		{Role: "user", Content: userMsg},
	})
	if err != nil {
		return err
	}

	fmt.Println(answer)
	return nil
}

// Interactive runs a multi-turn conversation about the repository.
func Interactive(repoPath string, cfg LLMConfig) error {
	if !cfg.Ping() {
		return fmt.Errorf("Ollama is not running at %s\nStart it with: ollama serve", cfg.Host)
	}

	ctx, err := CollectContext(repoPath)
	if err != nil {
		return fmt.Errorf("failed to collect git context: %w", err)
	}

	messages := []Message{
		{
			Role:    "system",
			Content: "You are git-sage, a helpful git assistant. Answer questions about the developer's repository concisely and accurately.",
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Here is the current git context:\n%s", ctx.Format()),
		},
		{
			Role:    "assistant",
			Content: "Ready. Ask me anything about this repository.",
		},
	}

	fmt.Printf("sage interactive  (model: %s)\n", cfg.Model)
	fmt.Println("Type your question, or 'exit' to quit.")
	fmt.Println()

	stdin := os.Stdin
	buf := make([]byte, 4096)
	for {
		fmt.Print("you> ")
		n, err := stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		input := strings.TrimSpace(string(buf[:n]))
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" || input == "q" {
			break
		}

		messages = append(messages, Message{Role: "user", Content: input})
		answer, err := cfg.Chat(messages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
		messages = append(messages, Message{Role: "assistant", Content: answer})
		fmt.Printf("\nsage> %s\n", answer)
		fmt.Println()
	}
	return nil
}
