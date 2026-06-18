package main

// cli_voice.go — implements `devtrack voice` subcommand group (Phase 5).
//
// Subcommands:
//   devtrack voice seed   Call POST /voice/seed for each enabled workspace,
//                         print per-repo embedded count.

import (
	"fmt"
	"os"

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

// ── helpers ───────────────────────────────────────────────────────────────────

// printVoiceUsage prints a short usage block for the voice command group.
func printVoiceUsage() {
	fmt.Println("Usage:")
	fmt.Println("  devtrack voice seed     Seed voice corpus from git commit history for all workspaces")
	fmt.Println("  devtrack voice profile  Generate Developer Voice Profile from corpus (profile.md)")
}
