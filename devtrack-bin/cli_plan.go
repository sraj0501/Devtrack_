package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// handlePlan implements `devtrack plan` — decompose a problem into an
// Epic → Story → Task hierarchy and create items on the chosen platform.
//
// Usage:
//
//	devtrack plan "<problem statement>"
//	devtrack plan --file  <plan.md>
//	devtrack plan --folder <plans/>
func (cli *CLI) handlePlan() error {
	if err := requiresManagedMode("plan"); err != nil {
		return err
	}

	args := os.Args[2:]

	switch {
	case len(args) == 0:
		printPlanUsage()
		return nil

	case args[0] == "--file" || args[0] == "-f":
		if len(args) < 2 {
			return fmt.Errorf("--file requires a path argument")
		}
		return cli.runPlanFile(args[1])

	case args[0] == "--folder" || args[0] == "-d":
		if len(args) < 2 {
			return fmt.Errorf("--folder requires a path argument")
		}
		return cli.runPlanFolder(args[1])

	default:
		// Treat all remaining args as the inline problem statement.
		problem := strings.Join(args, " ")
		return cli.runPlanInline(problem)
	}
}

// runPlanInline handles `devtrack plan "<problem>"`.
func (cli *CLI) runPlanInline(problem string) error {
	platform, err := promptPlatform()
	if err != nil {
		return err
	}
	return cli.executePlan(PlanPreviewRequest{
		Problem:  problem,
		Platform: platform,
	})
}

// runPlanFile handles `devtrack plan --file <path>`.
func (cli *CLI) runPlanFile(path string) error {
	markdown, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file %q: %w", path, err)
	}
	// Platform may come from the ## Meta block; prompt only if not present.
	parsed := extractMetaPlatform(string(markdown))
	platform := parsed
	if platform == "" {
		var promptErr error
		platform, promptErr = promptPlatform()
		if promptErr != nil {
			return promptErr
		}
	} else {
		fmt.Printf("Platform from plan file: %s\n", platform)
	}
	return cli.executePlan(PlanPreviewRequest{
		Markdown: string(markdown),
		Platform: platform,
	})
}

// runPlanFolder handles `devtrack plan --folder <dir>`.
// Each .md file in the folder is treated as a separate plan.
func (cli *CLI) runPlanFolder(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read folder %q: %w", dir, err)
	}

	var mdFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			mdFiles = append(mdFiles, filepath.Join(dir, e.Name()))
		}
	}

	if len(mdFiles) == 0 {
		return fmt.Errorf("no .md files found in %q", dir)
	}

	fmt.Printf("Found %d plan file(s) in %s\n\n", len(mdFiles), dir)

	for i, f := range mdFiles {
		fmt.Printf("─── [%d/%d] %s ───\n", i+1, len(mdFiles), filepath.Base(f))
		if err := cli.runPlanFile(f); err != nil {
			fmt.Printf("  ✗ skipped: %v\n\n", err)
			continue
		}
		fmt.Println()
	}
	return nil
}

// executePlan runs the two-step preview → confirm → create flow.
func (cli *CLI) executePlan(req PlanPreviewRequest) error {
	client := NewHTTPTriggerClient()

	// ── Step 1: decompose & preview ────────────────────────────────────────
	fmt.Println("Decomposing plan...")
	preview, err := client.SendPlanPreview(req)
	if err != nil {
		return fmt.Errorf("plan preview failed: %w", err)
	}

	platformLabel := strings.ToUpper(preview.Platform)
	fmt.Printf("\nPlan Preview  (%s)\n", platformLabel)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println(preview.Preview)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Total: %d item(s) — %d epic(s), %d story/feature(s), %d task(s)\n\n",
		preview.TotalCount, preview.EpicCount, preview.StoryCount, preview.TaskCount)

	// ── Step 2: confirm ────────────────────────────────────────────────────
	if !confirmPrompt(fmt.Sprintf("Create all %d items in %s?", preview.TotalCount, platformLabel)) {
		fmt.Println("Cancelled.")
		return nil
	}

	// ── Step 3: create ─────────────────────────────────────────────────────
	fmt.Printf("\nCreating %d items in %s...\n", preview.TotalCount, platformLabel)
	result, err := client.SendPlanCreate(preview.PlanToken)
	if err != nil {
		return fmt.Errorf("plan creation failed: %w", err)
	}

	fmt.Printf("\nDone — %d created, %d failed\n\n", len(result.Created), len(result.Failed))

	indent := map[int]string{0: "", 1: "  ", 2: "    "}
	for _, item := range result.Created {
		ind := indent[item.Level]
		if ind == "" {
			ind = ""
		}
		urlPart := ""
		if item.PlatformURL != "" {
			urlPart = "  → " + item.PlatformURL
		} else if item.PlatformID != "" {
			urlPart = "  #" + item.PlatformID
		}
		fmt.Printf("  ✓ %s%s: %s%s\n", ind, item.ItemType, item.Title, urlPart)
	}

	if len(result.Failed) > 0 {
		fmt.Printf("\nFailed (%d):\n", len(result.Failed))
		for _, f := range result.Failed {
			fmt.Printf("  ✗ %s: %s\n", f.Title, f.Error)
		}
	}

	return nil
}

// ─── helpers ───────────────────────────────────────────────────────────────

// promptPlatform shows a numbered menu and returns the chosen platform string.
func promptPlatform() (string, error) {
	platforms := []string{"azure", "gitlab", "github"}
	labels := []string{"Azure DevOps", "GitLab", "GitHub"}

	fmt.Println("Choose target platform:")
	for i, l := range labels {
		fmt.Printf("  %d) %s\n", i+1, l)
	}
	fmt.Print("Enter choice [1-3]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}
	input = strings.TrimSpace(input)

	switch input {
	case "1":
		return platforms[0], nil
	case "2":
		return platforms[1], nil
	case "3":
		return platforms[2], nil
	default:
		// Also accept direct names.
		lower := strings.ToLower(input)
		for _, p := range platforms {
			if lower == p {
				return p, nil
			}
		}
		return "", fmt.Errorf("invalid choice %q — enter 1, 2, or 3", input)
	}
}

// confirmPrompt asks a yes/no question and returns true for y/Y/yes.
func confirmPrompt(question string) bool {
	fmt.Printf("%s [y/N]: ", question)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// extractMetaPlatform does a quick scan of markdown content for a
// "platform: <value>" line inside a ## Meta block, without a full parse.
func extractMetaPlatform(content string) string {
	inMeta := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, "## meta") || strings.EqualFold(trimmed, "## Meta") {
			inMeta = true
			continue
		}
		if inMeta {
			if strings.HasPrefix(trimmed, "#") {
				break // left the Meta section
			}
			if strings.HasPrefix(strings.ToLower(trimmed), "platform:") {
				val := strings.TrimSpace(trimmed[len("platform:"):])
				val = strings.ToLower(val)
				switch val {
				case "azure", "gitlab", "github":
					return val
				}
			}
		}
	}
	return ""
}

func printPlanUsage() {
	fmt.Println(`Usage:
  devtrack plan "<problem>"          Decompose a problem statement
  devtrack plan --file <plan.md>    Load a structured plan file
  devtrack plan --folder <dir/>     Process all .md plan files in a folder

Plan file format (structured markdown):
  # Plan: <title>

  ## Meta
  platform: azure   # azure | gitlab | github
  project: MyProject

  ## Goal
  Describe what needs to be built.

  ## Epics  (optional — used as hints for the AI)
  - Epic title
    - Story: story title
      - Task: task title

  ## Notes  (optional)
  - Must comply with SOC2`)
}
