package main

// cli_voice.go — implements `devtrack voice` subcommand group (Phase 5).
//
// Subcommands:
//   devtrack voice seed              Call POST /voice/seed for each enabled workspace,
//                                    print per-repo embedded count.
//   devtrack voice profile           Generate Developer Voice Profile from corpus (profile.md).
//   devtrack voice add <text>        Inject a manual writing example into ChromaDB.
//     --context <type>               Optional context type (default: commit).
//   devtrack voice status            Print corpus statistics table.

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// handleVoice dispatches `devtrack voice` subcommands.
func (cli *CLI) handleVoice() error {
	sub := "seed"
	if len(os.Args) > 2 {
		sub = os.Args[2]
	}

	switch sub {
	case "seed":
		return runVoiceSeed()
	case "profile":
		return runVoiceProfile()
	case "add":
		return runVoiceAdd()
	case "status":
		return runVoiceStatus()
	case "sync":
		return runVoiceSync()
	default:
		fmt.Printf("devtrack voice: unknown subcommand %q\n\n", sub)
		printVoiceUsage()
		return fmt.Errorf("unknown voice subcommand: %s", sub)
	}
}

// ── seed ──────────────────────────────────────────────────────────────────────

// runVoiceSeed implements `devtrack voice seed`.
// Reads all enabled workspaces from workspaces.yaml and calls POST /voice/seed
// for each one, printing the embedded count per workspace.
func runVoiceSeed() error {
	sinceMonths := config.GetVoiceSeedMonths()

	wsCfg, err := config.LoadWorkspacesConfig()
	if err != nil {
		return fmt.Errorf("voice seed: load workspaces: %w", err)
	}

	tc := trigger.NewHTTPTriggerClient()

	if wsCfg == nil || len(wsCfg.Workspaces) == 0 {
		// Single-repo mode: use the configured workspace path.
		repoPath := config.ExpandWorkspacePath(os.Getenv("DEVTRACK_WORKSPACE"))
		if repoPath == "" {
			return fmt.Errorf("voice seed: no workspaces configured and DEVTRACK_WORKSPACE is not set")
		}
		return seedRepo(tc, repoPath, sinceMonths)
	}

	var lastErr error
	for _, ws := range wsCfg.GetEnabledWorkspaces() {
		repoPath := config.ExpandWorkspacePath(ws.Path)
		if repoPath == "" {
			fmt.Printf("voice seed: workspace %q has no path — skipping\n", ws.Name)
			continue
		}
		if err := seedRepo(tc, repoPath, sinceMonths); err != nil {
			fmt.Printf("voice seed: workspace %q: %v\n", ws.Name, err)
			lastErr = err
		}
	}
	return lastErr
}

// seedRepo calls POST /voice/seed for a single repository and prints the result.
func seedRepo(tc *trigger.HTTPTriggerClient, repoPath string, sinceMonths int) error {
	fmt.Printf("Seeding voice corpus from %s...", repoPath)
	resp, err := tc.VoiceSeed(trigger.VoiceSeedRequest{
		RepoPath:    repoPath,
		SinceMonths: sinceMonths,
		Force:       false,
	})
	if err != nil {
		fmt.Println(" error")
		return fmt.Errorf("POST /voice/seed: %w", err)
	}
	fmt.Printf(" %d messages embedded.\n", resp.Embedded)
	return nil
}

// ── profile ───────────────────────────────────────────────────────────────────

// runVoiceProfile implements `devtrack voice profile`.
// Reads repo paths from all enabled workspaces and calls POST /voice/profile/generate
// to build profile.md from the ChromaDB commit corpus, then prints the result.
func runVoiceProfile() error {
	wsCfg, err := config.LoadWorkspacesConfig()
	if err != nil {
		return fmt.Errorf("voice profile: load workspaces: %w", err)
	}

	tc := trigger.NewHTTPTriggerClient()

	var repoPaths []string
	if wsCfg == nil || len(wsCfg.Workspaces) == 0 {
		repoPath := config.ExpandWorkspacePath(os.Getenv("DEVTRACK_WORKSPACE"))
		if repoPath != "" {
			repoPaths = append(repoPaths, repoPath)
		}
	} else {
		for _, ws := range wsCfg.GetEnabledWorkspaces() {
			repoPath := config.ExpandWorkspacePath(ws.Path)
			if repoPath != "" {
				repoPaths = append(repoPaths, repoPath)
			}
		}
	}

	path, wordCount, err := tc.VoiceProfileGenerate(repoPaths)
	if err != nil {
		return fmt.Errorf("voice profile: %w", err)
	}

	fmt.Printf("Profile generated: %s (%d words).\n", path, wordCount)
	return nil
}

// ── add ───────────────────────────────────────────────────────────────────────

// runVoiceAdd implements `devtrack voice add <text> [--context <type>]`.
// Injects a manual high-weight writing example into ChromaDB and prints the
// assigned ChromaDB document ID.
//
// Usage:
//
//	devtrack voice add "example text"
//	devtrack voice add --context comment "Fixed the null check in auth flow"
func runVoiceAdd() error {
	// Parse positional text and optional --context flag from os.Args.
	// os.Args layout: devtrack voice add [--context <type>] <text>
	args := os.Args[3:] // everything after "voice add"

	contextType := "commit" // default
	var textParts []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--context" && i+1 < len(args) {
			contextType = args[i+1]
			i++ // skip value
		} else {
			textParts = append(textParts, args[i])
		}
	}

	text := strings.Join(textParts, " ")
	if text == "" {
		return fmt.Errorf("voice add: text argument is required\nUsage: devtrack voice add [--context <type>] <text>")
	}

	tc := trigger.NewHTTPTriggerClient()
	id, err := tc.VoiceAdd(text, contextType)
	if err != nil {
		return fmt.Errorf("voice add: %w", err)
	}

	fmt.Printf("Added to voice corpus (context: %s, id: %s).\n", contextType, id)
	return nil
}

// ── status ────────────────────────────────────────────────────────────────────

// runVoiceStatus implements `devtrack voice status`.
// Calls GET /voice/status and prints a human-readable table.
// When stdout is not a TTY (pipe/redirect) box-drawing characters are omitted
// and ANSI codes are not emitted.
func runVoiceStatus() error {
	tc := trigger.NewHTTPTriggerClient()
	resp, err := tc.VoiceStatus()
	if err != nil {
		return fmt.Errorf("voice status: %w", err)
	}

	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	var sep string
	if isTTY {
		sep = strings.Repeat("─", 38)
	} else {
		sep = strings.Repeat("-", 38)
	}

	fmt.Println("Voice Corpus Status")
	fmt.Println(sep)
	fmt.Printf("Total entries:    %d\n", resp.TotalEntries)

	// By context — print each known type in a fixed order.
	contextOrder := []string{"commit", "description", "comment", "report", "task"}
	for _, ct := range contextOrder {
		n := resp.ByContext[ct]
		fmt.Printf("  %-14s %d\n", ct+":", n)
	}

	fmt.Println("By source:")
	sourceOrder := []string{"git_history", "pr_sync", "manual"}
	for _, src := range sourceOrder {
		n := resp.BySource[src]
		fmt.Printf("  %-14s %d\n", src+":", n)
	}

	// last_seed
	if resp.LastSeed != nil && *resp.LastSeed != "" {
		// Trim to "YYYY-MM-DD HH:MM" for readability (server returns datetime string).
		ts := *resp.LastSeed
		if len(ts) > 16 {
			ts = ts[:16]
		}
		fmt.Printf("Last seed:        %s\n", ts)
	} else {
		fmt.Println("Last seed:        never")
	}

	// last_sync
	if resp.LastSync != nil && *resp.LastSync != "" {
		ts := *resp.LastSync
		if len(ts) > 16 {
			ts = ts[:16]
		}
		fmt.Printf("Last sync:        %s\n", ts)
	} else {
		fmt.Println("Last sync:        never")
	}

	// profile
	if resp.ProfileExists {
		fmt.Printf("Profile:          exists (%d words)\n", resp.ProfileWordCount)
	} else {
		fmt.Println("Profile:          not generated (run: devtrack voice profile)")
	}

	// ── Phase 6 dialectic sections ───────────────────────────────────────────
	// Each section is printed only when the field is present in the server
	// response (pointer non-nil). This keeps the CLI backward-compatible with
	// older server versions that do not send these fields.

	// Inferences section.
	if resp.Inferences != nil {
		inf := resp.Inferences
		fmt.Println()
		fmt.Println("Dialectic Inferences")
		fmt.Println(strings.Repeat("-", 21))
		fmt.Printf("Total inferred:    %d\n", inf.Total)
		fmt.Printf("Corrections:        %d\n", inf.CorrectionCount)
		if len(inf.TopByConfidence) > 0 {
			fmt.Println("Top inferences (by confidence):")
			for _, entry := range inf.TopByConfidence {
				fmt.Printf("  %-18s %.2f  -- %s\n", entry.Subject, entry.Confidence, entry.Inference)
			}
		} else {
			fmt.Println("Top inferences (by confidence): (none)")
		}
	}

	// Skills section.
	if resp.Skills != nil {
		sk := resp.Skills
		fmt.Println()
		fmt.Printf("Autonomous Skills (%d)\n", sk.Total)
		fmt.Println(strings.Repeat("-", 21))
		if len(sk.Names) > 0 {
			for _, name := range sk.Names {
				fmt.Printf("  %s\n", name)
			}
		} else {
			fmt.Println("  (none)")
		}
	}

	// Thresholds section.
	if resp.Thresholds != nil {
		fmt.Println()
		fmt.Println("Confidence Thresholds")
		fmt.Println(strings.Repeat("-", 21))
		if len(resp.Thresholds) > 0 {
			// Print in sorted order for deterministic output.
			// Collect keys then sort.
			keys := make([]string, 0, len(resp.Thresholds))
			for k := range resp.Thresholds {
				keys = append(keys, k)
			}
			// Simple insertion sort — threshold list is always tiny.
			for i := 1; i < len(keys); i++ {
				for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
					keys[j], keys[j-1] = keys[j-1], keys[j]
				}
			}
			for _, k := range keys {
				t := resp.Thresholds[k]
				fmt.Printf("  %-20s %.2f  (%d approvals / %d rejections)\n",
					k, t.Threshold, t.Approvals, t.Rejections)
			}
		} else {
			fmt.Println("  (none)")
		}
	}

	return nil
}

// ── sync ──────────────────────────────────────────────────────────────────────

// runVoiceSync implements `devtrack voice sync`.
// Calls POST /voice/sync for all configured workspaces and prints per-platform
// counts of newly embedded PR descriptions and issue comments.
func runVoiceSync() error {
	fmt.Print("Syncing voice corpus from PM platforms...")
	tc := trigger.NewHTTPTriggerClient()
	counts, err := tc.VoiceSync(nil)
	if err != nil {
		fmt.Println(" error")
		return fmt.Errorf("voice sync: %w", err)
	}
	fmt.Printf(
		" done.\nSynced: github=%d azure=%d gitlab=%d\n",
		counts["github"], counts["azure"], counts["gitlab"],
	)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// printVoiceUsage prints a short usage block for the voice command group.
func printVoiceUsage() {
	fmt.Println("Usage:")
	fmt.Println("  devtrack voice seed               Seed voice corpus from git commit history")
	fmt.Println("  devtrack voice profile            Generate Developer Voice Profile (profile.md)")
	fmt.Println("  devtrack voice add <text>         Add a manual writing example to corpus")
	fmt.Println("    --context <type>                Context type: commit|description|comment|report|task (default: commit)")
	fmt.Println("  devtrack voice status             Show voice corpus statistics")
	fmt.Println("  devtrack voice sync               Sync PR descriptions and issue comments from PM platforms")
}
