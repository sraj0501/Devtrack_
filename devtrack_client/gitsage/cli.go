package gitsage

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ApprovalMode controls how the agent executes commands in "do" mode.
type ApprovalMode string

const (
	// ApprovalAuto executes commands without prompting.
	ApprovalAuto ApprovalMode = "auto"
	// ApprovalReview prompts before each command.
	ApprovalReview ApprovalMode = "review"
	// ApprovalSuggestOnly shows what the agent would do, but doesn't execute.
	ApprovalSuggestOnly ApprovalMode = "suggest-only"
)

// maxFollowUps is the maximum number of follow-up questions after a do task completes.
const maxFollowUps = 5

// ShowApprovalDialog shows a simple terminal prompt asking the user to choose
// how the agent should operate. Returns the chosen ApprovalMode.
// Falls back to Auto if stdin is not a TTY or SAGE_AUTO=1 is set.
func ShowApprovalDialog(task string) ApprovalMode {
	// Non-interactive / scripted override
	if os.Getenv("SAGE_AUTO") == "1" {
		return ApprovalAuto
	}

	fmt.Printf("\nsage: task — %s\n\n", task)
	fmt.Println("How should sage operate?")
	fmt.Println("  [1] auto         — execute all commands without prompting (default)")
	fmt.Println("  [2] review        — prompt before each command")
	fmt.Println("  [3] suggest-only  — show plan only, do not execute")
	fmt.Print("\nChoice [1]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ApprovalAuto
	}

	switch strings.TrimSpace(input) {
	case "2", "review":
		return ApprovalReview
	case "3", "suggest-only", "suggest":
		return ApprovalSuggestOnly
	default:
		return ApprovalAuto
	}
}

// PromptCommandApproval asks the user whether to execute a specific command.
// Returns true if the user approves (or presses Enter).
func PromptCommandApproval(cmd string) bool {
	fmt.Printf("  run: %s  [Y/n] ", cmd)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return true
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

// CommandHistory tracks commands run during a sage session.
type CommandHistory struct {
	entries []string
}

// Add records a command in history.
func (h *CommandHistory) Add(cmd string) {
	h.entries = append(h.entries, cmd)
}

// All returns all recorded commands.
func (h *CommandHistory) All() []string {
	return h.entries
}

// Print prints the command history to stdout.
func (h *CommandHistory) Print() {
	if len(h.entries) == 0 {
		fmt.Println("No commands run in this session.")
		return
	}
	fmt.Println("Commands run this session:")
	for i, cmd := range h.entries {
		fmt.Printf("  %d. %s\n", i+1, cmd)
	}
}

// RunFollowUpLoop offers the user up to maxFollowUps follow-up questions
// after a "do" task completes. Uses the existing conversation context
// so the LLM retains awareness of what was done.
func RunFollowUpLoop(repoPath string, cfg LLMConfig, conversationMessages []Message) {
	fmt.Println()
	fmt.Println("sage: task complete. You can ask follow-up questions (or press Enter to exit).")
	fmt.Printf("  (%d follow-ups remaining)\n\n", maxFollowUps)

	reader := bufio.NewReader(os.Stdin)
	remaining := maxFollowUps

	// Refresh context after task completion
	ctx, err := CollectContext(repoPath)
	if err == nil {
		refreshMsg := fmt.Sprintf("The task is complete. Updated repository context:\n%s", ctx.Format())
		conversationMessages = append(conversationMessages,
			Message{Role: "user", Content: refreshMsg},
			Message{Role: "assistant", Content: "Got it. What else can I help you with?"},
		)
	}

	for remaining > 0 {
		fmt.Printf("you> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)
		if input == "" || input == "exit" || input == "quit" || input == "q" {
			break
		}
		if input == "history" {
			// History is managed by the caller; acknowledge
			fmt.Println("sage: history is printed by the parent command.")
			continue
		}

		conversationMessages = append(conversationMessages, Message{Role: "user", Content: input})
		answer, err := cfg.Chat(conversationMessages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sage: LLM error: %v\n", err)
			continue
		}
		conversationMessages = append(conversationMessages, Message{Role: "assistant", Content: answer})
		fmt.Printf("\nsage> %s\n\n", answer)
		remaining--
		if remaining > 0 {
			fmt.Printf("  (%d follow-ups remaining)\n", remaining)
		}
	}
}

// RunAsk is the entry point for "devtrack sage ask <question>".
func RunAsk(repoPath, question string) error {
	cfg := LoadLLMConfig()
	return Ask(repoPath, question, cfg)
}

// RunDo is the entry point for "devtrack sage do <task>".
// Shows the approval dialog, runs the agentic loop with history tracking,
// then offers follow-up questions.
func RunDo(repoPath, task string) error {
	return RunDoVerbose(repoPath, task, false)
}

// RunDoVerbose is the entry point for "devtrack sage do <task> [--verbose]".
func RunDoVerbose(repoPath, task string, verbose bool) error {
	cfg := LoadLLMConfig()

	mode := ShowApprovalDialog(task)
	if mode == ApprovalSuggestOnly {
		fmt.Println("\nsage: suggest-only mode — showing plan without executing.")
	}

	history := &CommandHistory{}

	// For review/suggest-only modes, wrap Do in a way that intercepts commands.
	// For auto mode, use Do directly.
	if mode == ApprovalAuto {
		// Standard agentic loop
		if err := Do(repoPath, task, cfg, verbose); err != nil {
			return err
		}
	} else {
		// Build initial context and messages
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

		// Run the loop manually so we can intercept commands
		fmt.Printf("sage: planning task — %s\n\n", task)
		for range maxSteps {
			raw, err := cfg.ChatJSON(messages)
			if err != nil {
				return err
			}

			var parsed agentStep
			if err := parseAgentStep(raw, &parsed); err != nil {
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
				// Offer follow-ups
				RunFollowUpLoop(repoPath, cfg, messages)
				history.Print()
				return nil
			}

			if len(parsed.Commands) == 0 {
				return fmt.Errorf("sage: model returned no commands and done=false — stopping")
			}

			var execOutput strings.Builder
			for _, cmdStr := range parsed.Commands {
				if mode == ApprovalSuggestOnly {
					fmt.Printf("  would run: %s\n", cmdStr)
					history.Add("[suggest] " + cmdStr)
					fmt.Fprintf(&execOutput, "$ %s\n[not executed — suggest-only mode]\n", cmdStr)
					continue
				}

				if mode == ApprovalReview && !PromptCommandApproval(cmdStr) {
					fmt.Println("  skipped.")
					fmt.Fprintf(&execOutput, "$ %s\n[skipped by user]\n", cmdStr)
					continue
				}

				fmt.Printf("  $ %s\n", cmdStr)
				out, runErr := runCommand(repoPath, cmdStr)
				history.Add(cmdStr)
				if out != "" {
					fmt.Println("   ", strings.ReplaceAll(out, "\n", "\n    "))
					fmt.Fprintf(&execOutput, "$ %s\n%s\n", cmdStr, out)
				}
				if runErr != nil {
					fmt.Fprintf(&execOutput, "$ %s\nERROR: %v\n", cmdStr, runErr)
				}
			}

			messages = append(messages,
				Message{Role: "assistant", Content: raw},
				Message{Role: "user", Content: fmt.Sprintf("Command output:\n%s\nContinue.", execOutput.String())},
			)
			fmt.Println()
		}
		return fmt.Errorf("sage: reached max steps (%d) without completing task", maxSteps)
	}

	// After auto Do, offer follow-ups too
	ctx, _ := CollectContext(repoPath)
	systemMsg := "You are git-sage, a helpful git assistant. Answer questions about the repository concisely."
	userMsg := ""
	if ctx != nil {
		userMsg = fmt.Sprintf("Repository context after completing task:\n%s", ctx.Format())
	}
	RunFollowUpLoop(repoPath, cfg, []Message{
		{Role: "system", Content: systemMsg},
		{Role: "user", Content: userMsg},
		{Role: "assistant", Content: "Task complete. What else can I help you with?"},
	})
	history.Print()
	return nil
}

// RunInteractive is the entry point for "devtrack sage" (no subcommand).
func RunInteractive(repoPath string) error {
	cfg := LoadLLMConfig()
	return Interactive(repoPath, cfg)
}

// parseAgentStep wraps the JSON unmarshal with an import-free approach.
func parseAgentStep(raw string, out *agentStep) error {
	// Reuse the existing json.Unmarshal via the agent package
	// (agent.go already imports encoding/json)
	return unmarshalAgentStep(raw, out)
}
