package gitsage

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// commitTokenBudget is the baseline max-token budget for generating a commit
// message. Pressing [E] (enhance) doubles it for a single call, producing a
// more detailed message ("Richer AI Messages").
const commitTokenBudget = 512

// ExitCodeError carries a git exit code back to the caller so the binary can
// mirror git's own exit status for pass-through commands.
type ExitCodeError struct{ Code int }

func (e *ExitCodeError) Error() string { return fmt.Sprintf("git exited with code %d", e.Code) }

// CommitHooks lets the host (package main) inject PM/push behaviour around the
// commit without coupling gitsage to connectors. Both fields are optional.
type CommitHooks struct {
	// BeforeCommit runs after the user accepts a message but before `git commit`.
	// It may return a modified message (e.g. with a "Refs:" trailer) and an
	// opaque state passed to AfterCommit. Return ("", nil) to leave it unchanged.
	BeforeCommit func(repoPath, message string) (newMessage string, state any)
	// AfterCommit runs after a successful commit, with the real HEAD hash/branch
	// and the (possibly rewritten) message, plus the state from BeforeCommit.
	AfterCommit func(repoPath, hash, branch, message string, state any)
}

// IsInteractive reports whether stdin is a terminal (exported for hosts/hooks).
func IsInteractive() bool { return isInteractive() }

// RunGit is the Go-native entry point for "devtrack git <args...>".
//
// It replaces the legacy devtrack-git-wrapper.sh, which depended on the server
// backend's Python venv and a PROJECT_ROOT monorepo layout — neither of which
// exists in a standalone client install. Behaviour:
//
//	devtrack git add [paths...]          stage changes (defaults to ".")
//	devtrack git commit [-m msg] [flags] AI-enhanced interactive commit
//	devtrack git history|messages [n]    show recent commits
//	devtrack git <anything-else> ...     transparent pass-through to git
func RunGit(repoPath string, args []string, hooks *CommitHooks) error {
	if len(args) == 0 {
		printGitUsage()
		return nil
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "add":
		return handleGitAdd(repoPath, rest)
	case "commit":
		return handleGitCommit(repoPath, rest, hooks)
	case "history", "messages":
		return handleGitHistory(repoPath, rest)
	default:
		// Everything else (status, log, push, ...) goes straight to git.
		return passthroughGit(repoPath, args)
	}
}

func printGitUsage() {
	fmt.Println("Usage: devtrack git <command> [args...]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  add [paths...]       Stage changes (defaults to 'git add .')")
	fmt.Println("  commit [-m msg]      AI-enhanced commit with interactive refinement")
	fmt.Println("    --dry-run, -n        Preview the AI message without committing")
	fmt.Println("    --no-enhance         Commit with your message as-is, no AI")
	fmt.Println("  history [n]          Show last n commits (default: 10)")
	fmt.Println("  <other> ...          Any other git command is passed straight through")
}

// passthroughGit execs git with the given args, inheriting stdio, and mirrors
// git's exit code via ExitCodeError.
func passthroughGit(repoPath string, args []string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return &ExitCodeError{Code: ee.ExitCode()}
		}
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// handleGitAdd stages the given paths, defaulting to "." when none are given.
func handleGitAdd(repoPath string, paths []string) error {
	if len(paths) == 0 {
		fmt.Println("🔍 No path specified — staging all changes (git add .)")
		paths = []string{"."}
	}
	return passthroughGit(repoPath, append([]string{"add"}, paths...))
}

// handleGitHistory prints the last n commits (default 10).
func handleGitHistory(repoPath string, args []string) error {
	n := 10
	if len(args) > 0 {
		if parsed, err := strconv.Atoi(args[0]); err == nil && parsed > 0 {
			n = parsed
		}
	}

	if !NewGitOps(repoPath).IsRepo() {
		return fmt.Errorf("not a git repository")
	}

	fmt.Printf("📜 DevTrack Commit History — last %d commit(s)\n", n)
	fmt.Println(strings.Repeat("━", 44))
	// Delegate formatting to git itself. Pass-through inherits stdout so colour
	// works, and avoids GitOps.Log's NUL-separated format (rejected by Windows exec).
	return passthroughGit(repoPath, []string{
		"log",
		fmt.Sprintf("-n%d", n),
		"--date=format:%Y-%m-%d %H:%M",
		"--format=%C(yellow)%h%C(reset) %ad %C(cyan)%an%C(reset)%n    %s",
	})
}

// commitFlags holds the parsed flags for "devtrack git commit".
type commitFlags struct {
	message   string
	dryRun    bool
	noEnhance bool
	all       bool
	amend     bool
	passthru  []string // remaining args handed verbatim to git commit
}

// parseCommitArgs splits commit args into DevTrack-handled flags and git pass-through.
func parseCommitArgs(args []string) commitFlags {
	var f commitFlags
	skipNext := false
	for i, a := range args {
		if skipNext {
			f.message = a
			skipNext = false
			continue
		}
		switch {
		case a == "-m" || a == "--message":
			skipNext = true
		case strings.HasPrefix(a, "--message="):
			f.message = strings.TrimPrefix(a, "--message=")
		case strings.HasPrefix(a, "-m") && len(a) > 2:
			f.message = a[2:] // git's -mMessage form
		case a == "--dry-run" || a == "-n":
			f.dryRun = true
		case a == "--no-enhance":
			f.noEnhance = true
		case a == "-a" || a == "--all":
			f.all = true
			f.passthru = append(f.passthru, a)
		case a == "--amend":
			f.amend = true
			f.passthru = append(f.passthru, a)
		default:
			f.passthru = append(f.passthru, args[i])
		}
	}
	return f
}

// handleGitCommit runs the AI-enhanced commit flow natively in Go.
func handleGitCommit(repoPath string, args []string, hooks *CommitHooks) error {
	f := parseCommitArgs(args)
	g := NewGitOps(repoPath)
	if !g.IsRepo() {
		return fmt.Errorf("not a git repository")
	}

	// Collect the diff that describes what is being committed.
	diff := collectCommitDiff(g, f)

	// Guard against an empty commit (unless --amend, where git allows message-only).
	if strings.TrimSpace(diff) == "" && !f.amend {
		fmt.Println("No changes staged for commit.")
		fmt.Println("Use 'devtrack git add <files>' to stage changes first.")
		return &ExitCodeError{Code: 1}
	}

	// --no-enhance: commit immediately with the user's message (or git's editor).
	if f.noEnhance {
		return commitWith(repoPath, f, f.message, hooks)
	}

	cfg := LoadConfig().LLM

	// If the LLM is unreachable, degrade gracefully to a plain commit rather
	// than blocking the user. A standalone client has no server fallback.
	if !cfg.Ping() {
		if !f.dryRun {
			fmt.Println("⚠️  AI provider unreachable — committing with your message as-is.")
		} else {
			fmt.Println("⚠️  AI provider unreachable — cannot preview an enhanced message.")
			return nil
		}
		return commitWith(repoPath, f, f.message, hooks)
	}

	branch, _ := g.CurrentBranch()

	message, err := generateCommitMessage(cfg, diff, f.message, branch)
	if err != nil || strings.TrimSpace(message) == "" {
		// Generation failed — fall back rather than abort.
		if f.dryRun {
			return fmt.Errorf("AI generation failed: %w", err)
		}
		fmt.Printf("⚠️  AI generation failed (%v) — committing with your message as-is.\n", err)
		return commitWith(repoPath, f, f.message, hooks)
	}

	if f.dryRun {
		fmt.Println("🔍 AI-enhanced commit message (preview, no commit):")
		fmt.Println(strings.Repeat("━", 44))
		fmt.Println(indent(message, "  "))
		fmt.Println(strings.Repeat("━", 44))
		fmt.Println("Run without --dry-run to commit with interactive refinement.")
		return nil
	}

	// Non-interactive (piped stdin / CI): accept the first generated message.
	if !isInteractive() {
		fmt.Println("🤖 AI-enhanced commit message:")
		fmt.Println(indent(message, "  "))
		return commitWith(repoPath, f, message, hooks)
	}

	return interactiveCommit(repoPath, cfg, f, diff, branch, message, hooks)
}

// collectCommitDiff returns the diff used as context for message generation,
// accounting for -a (include unstaged tracked) and --amend (include last commit).
func collectCommitDiff(g *GitOps, f commitFlags) string {
	staged, _ := g.DiffCached()
	parts := []string{}
	if staged != "" {
		parts = append(parts, staged)
	}
	if f.all {
		if unstaged, _ := g.DiffFull(); unstaged != "" {
			parts = append(parts, unstaged)
		}
	}
	if f.amend && len(parts) == 0 {
		if prev, _ := g.run("show", "--no-color", "HEAD"); prev != "" {
			parts = append(parts, prev)
		}
	}
	return strings.Join(parts, "\n")
}

// interactiveCommit drives the Accept / Enhance / Regenerate / Cancel loop.
func interactiveCommit(repoPath string, cfg LLMConfig, f commitFlags, diff, branch, message string, hooks *CommitHooks) error {
	const maxAttempts = 5
	reader := bufio.NewReader(os.Stdin)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		fmt.Println()
		fmt.Printf("✨ AI-Enhanced Commit — attempt %d/%d\n", attempt, maxAttempts)
		fmt.Println(strings.Repeat("━", 44))
		fmt.Println(indent(message, "  "))
		fmt.Println(strings.Repeat("━", 44))
		fmt.Println("  [A]ccept and commit   [E]nhance   [R]egenerate   [C]ancel")
		fmt.Print("Choice (A/E/R/C): ")

		input, err := reader.ReadString('\n')
		if err != nil {
			// Lost stdin — accept what we have.
			return commitWith(repoPath, f, message, hooks)
		}
		switch strings.ToLower(strings.TrimSpace(input)) {
		case "a", "accept", "":
			return commitWith(repoPath, f, message, hooks)
		case "e", "enhance":
			if attempt == maxAttempts {
				fmt.Println("✗ Maximum attempts reached — accept or cancel.")
				continue
			}
			improved, err := enhanceCommitMessage(cfg, diff, message, branch, commitTokenBudget*2)
			if err == nil && strings.TrimSpace(improved) != "" {
				message = improved
			} else {
				fmt.Printf("⚠️  Enhance failed (%v) — keeping current message.\n", err)
			}
		case "r", "regenerate":
			if attempt == maxAttempts {
				fmt.Println("✗ Maximum attempts reached — accept or cancel.")
				continue
			}
			regen, err := generateCommitMessage(cfg, diff, f.message, branch)
			if err == nil && strings.TrimSpace(regen) != "" {
				message = regen
			} else {
				fmt.Printf("⚠️  Regenerate failed (%v) — keeping current message.\n", err)
			}
		case "c", "cancel", "q", "quit":
			fmt.Println("✗ Commit cancelled.")
			return nil
		default:
			fmt.Println("Please enter A, E, R, or C.")
			attempt-- // don't count invalid input
		}
	}

	fmt.Printf("✗ Maximum attempts (%d) reached without acceptance. Commit cancelled.\n", maxAttempts)
	return &ExitCodeError{Code: 1}
}

// commitWith creates the commit. A non-empty message is written to a temp file
// and passed via -F (preserving multi-line bodies); otherwise git's editor opens.
// When hooks are provided, BeforeCommit may rewrite the message and AfterCommit
// runs the post-commit (PM sync / push) flow.
func commitWith(repoPath string, f commitFlags, message string, hooks *CommitHooks) error {
	// BeforeCommit: let the host link a ticket and append a "Refs:" trailer.
	var state any
	if hooks != nil && hooks.BeforeCommit != nil && strings.TrimSpace(message) != "" {
		if newMsg, st := hooks.BeforeCommit(repoPath, message); newMsg != "" {
			message = newMsg
			state = st
		} else {
			state = st
		}
	}

	if strings.TrimSpace(message) == "" {
		// No message and no AI — defer to git (opens editor or uses passthru flags).
		if err := passthroughGit(repoPath, append([]string{"commit"}, f.passthru...)); err != nil {
			return err
		}
	} else {
		tmp, err := os.CreateTemp("", "devtrack-commit-*.txt")
		if err != nil {
			return fmt.Errorf("create temp message file: %w", err)
		}
		defer os.Remove(tmp.Name())
		if _, err := tmp.WriteString(message + "\n"); err != nil {
			tmp.Close()
			return fmt.Errorf("write temp message file: %w", err)
		}
		tmp.Close()

		args := append([]string{"commit", "-F", tmp.Name()}, f.passthru...)
		if err := passthroughGit(repoPath, args); err != nil {
			return err
		}
		fmt.Println("✓ Committed.")
	}

	// AfterCommit: run the post-commit flow (time prompt, PM sync, push).
	if hooks != nil && hooks.AfterCommit != nil {
		g := NewGitOps(repoPath)
		hash, _ := g.HEAD()
		branch, _ := g.CurrentBranch()
		realMsg, _ := g.run("log", "-1", "--format=%B")
		if strings.TrimSpace(realMsg) == "" {
			realMsg = message
		}
		hooks.AfterCommit(repoPath, hash, branch, strings.TrimSpace(realMsg), state)
	}
	return nil
}

// generateCommitMessage asks the LLM for a Conventional-Commits message from the diff.
func generateCommitMessage(cfg LLMConfig, diff, userHint, branch string) (string, error) {
	system := "You are a commit message generator. Given a git diff, write a single " +
		"clear commit message in the Conventional Commits style: a `type(scope): summary` " +
		"subject line of at most 72 characters, optionally followed by a blank line and a " +
		"concise body explaining what changed and why. Output ONLY the commit message — no " +
		"markdown code fences, no quotes, no preamble, no trailing commentary."

	var b strings.Builder
	if branch != "" {
		fmt.Fprintf(&b, "Current branch: %s\n", branch)
	}
	if strings.TrimSpace(userHint) != "" {
		fmt.Fprintf(&b, "The developer's intent/summary: %s\n", userHint)
	}
	fmt.Fprintf(&b, "\nGit diff:\n%s\n", truncate(diff, 8000))

	resp, err := cfg.ChatWithTokens([]Message{
		{Role: "system", Content: system},
		{Role: "user", Content: b.String()},
	}, commitTokenBudget)
	if err != nil {
		return "", err
	}
	return cleanCommitMessage(resp), nil
}

// enhanceCommitMessage asks the LLM to improve an existing message against the diff.
// maxTokens caps the generation length (0 = model default).
func enhanceCommitMessage(cfg LLMConfig, diff, current, branch string, maxTokens int) (string, error) {
	system := "You are a commit message editor. Improve the given commit message so it is " +
		"clearer and more informative while staying faithful to the diff. Keep the Conventional " +
		"Commits style (subject <= 72 chars, optional body). Output ONLY the improved commit " +
		"message — no code fences, no quotes, no preamble."

	var b strings.Builder
	if branch != "" {
		fmt.Fprintf(&b, "Current branch: %s\n", branch)
	}
	fmt.Fprintf(&b, "Current commit message:\n%s\n\nGit diff:\n%s\n", current, truncate(diff, 8000))

	resp, err := cfg.ChatWithTokens([]Message{
		{Role: "system", Content: system},
		{Role: "user", Content: b.String()},
	}, maxTokens)
	if err != nil {
		return "", err
	}
	return cleanCommitMessage(resp), nil
}

// cleanCommitMessage strips markdown fences and stray wrapping the model may add.
func cleanCommitMessage(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}

// truncate caps a string to n bytes, appending a marker when cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n... [diff truncated]"
}

// indent prefixes every line of s with prefix.
func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

// isInteractive reports whether stdin is a terminal.
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
