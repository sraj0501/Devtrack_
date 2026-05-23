package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ── ANSI colours ─────────────────────────────────────────────────────────────

const (
	colReset   = "\033[0m"
	colBold    = "\033[1m"
	colDim     = "\033[2m"
	colCyan    = "\033[96m"
	colRed     = "\033[91m"
	colGreen   = "\033[92m"
	colMagenta = "\033[95m"
	colBlue    = "\033[94m"
	colYellow  = "\033[93m"
	colWhite   = "\033[97m"
	colOrange  = "\033[38;5;208m"
)

// Per-persona terminal colours (matched to persona IDs).
var personaColour = map[string]string{
	"architect":   colCyan,
	"security":    colRed,
	"pm":          colGreen,
	"devil":       colMagenta,
	"engineer":    colBlue,
	"analyst":     colYellow,
	"scalability": colOrange,
}

// handleBoardroom implements `devtrack boardroom` — a multi-persona AI review
// of a plan, producing PROs/CONs, a SWOT matrix, a final verdict, and an
// optional interactive discussion session.
//
// Usage:
//
//	devtrack boardroom "<problem>"
//	devtrack boardroom --file  <plan.md>
//	devtrack boardroom --folder <plans/>
//	devtrack boardroom --file <plan.md> --output <report.md>
//	devtrack boardroom --file <plan.md> --interactive
func (cli *CLI) handleBoardroom() error {
	if err := requiresManagedMode("boardroom"); err != nil {
		return err
	}

	args := os.Args[2:]
	if len(args) == 0 {
		printBoardroomUsage()
		return nil
	}

	var (
		filePath    string
		folderPath  string
		outputPath  string
		autoInteract bool
		problemParts []string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file", "-f":
			if i+1 >= len(args) {
				return fmt.Errorf("--file requires a path argument")
			}
			i++
			filePath = args[i]
		case "--folder", "-d":
			if i+1 >= len(args) {
				return fmt.Errorf("--folder requires a path argument")
			}
			i++
			folderPath = args[i]
		case "--output", "-o":
			if i+1 >= len(args) {
				return fmt.Errorf("--output requires a file path argument")
			}
			i++
			outputPath = args[i]
		case "--interactive", "-i":
			autoInteract = true
		default:
			problemParts = append(problemParts, args[i])
		}
	}

	switch {
	case folderPath != "":
		return cli.runBoardroomFolder(folderPath, outputPath, autoInteract)
	case filePath != "":
		return cli.runBoardroomFile(filePath, outputPath, autoInteract)
	default:
		problem := strings.Join(problemParts, " ")
		if problem == "" {
			printBoardroomUsage()
			return nil
		}
		return cli.runBoardroomInline(problem, outputPath, autoInteract)
	}
}

// ── Entry points ─────────────────────────────────────────────────────────────

func (cli *CLI) runBoardroomInline(problem, outputPath string, autoInteract bool) error {
	return cli.executeBoardroom(BoardroomRequest{
		PlanText:     problem,
		OutputFormat: boardroomOutputFormat(outputPath),
	}, problem, outputPath, autoInteract)
}

func (cli *CLI) runBoardroomFile(path, outputPath string, autoInteract bool) error {
	markdown, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file %q: %w", path, err)
	}
	return cli.executeBoardroom(BoardroomRequest{
		Markdown:     string(markdown),
		OutputFormat: boardroomOutputFormat(outputPath),
	}, string(markdown), outputPath, autoInteract)
}

func (cli *CLI) runBoardroomFolder(dir, outputPath string, autoInteract bool) error {
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
		fileOutput := outputPath
		if outputPath != "" {
			info, statErr := os.Stat(outputPath)
			if statErr == nil && info.IsDir() {
				base := strings.TrimSuffix(filepath.Base(f), ".md")
				fileOutput = filepath.Join(outputPath, base+"-boardroom.md")
			}
		}
		if err := cli.runBoardroomFile(f, fileOutput, autoInteract); err != nil {
			fmt.Printf("  ✗ skipped: %v\n\n", err)
		}
		fmt.Println()
	}
	return nil
}

// ── Core execution ────────────────────────────────────────────────────────────

// executeBoardroom runs the initial review then optionally enters the REPL.
// planSource is the raw text/markdown sent as context to the interactive session.
func (cli *CLI) executeBoardroom(req BoardroomRequest, planSource, outputPath string, autoInteract bool) error {
	client := NewHTTPTriggerClient()

	fmt.Println("Convening the boardroom... (7 personas analysing in parallel)")
	fmt.Println("This may take 30–120 seconds depending on your LLM provider.")
	fmt.Println()

	result, err := client.SendBoardroom(req)
	if err != nil {
		return fmt.Errorf("boardroom session failed: %w", err)
	}

	fmt.Println(result.Report)

	if outputPath != "" {
		if writeErr := os.WriteFile(outputPath, []byte(result.Report), 0o644); writeErr != nil {
			fmt.Printf("Warning: could not save report to %q: %v\n", outputPath, writeErr)
		} else {
			fmt.Printf("Report saved → %s\n", outputPath)
		}
	}

	// ── Offer / enter interactive mode ────────────────────────────────────
	enterInteractive := autoInteract
	if !autoInteract {
		enterInteractive = confirmPrompt("Enter interactive boardroom discussion?")
	}
	if !enterInteractive {
		return nil
	}

	// Seed history with a system entry summarising the initial review.
	seedHistory := []BoardroomHistoryEntry{
		{
			Role: "system",
			Content: fmt.Sprintf(
				"Initial boardroom review complete. Verdict: %s. Approve: %d, Revise: %d, Reject: %d.",
				result.Verdict, result.Approve, result.Revise, result.Reject,
			),
		},
	}

	// Determine plan_text to pass to the chat endpoint.
	planText := req.PlanText
	if planText == "" {
		planText = planSource // fallback to the raw markdown
	}

	return cli.runInteractiveSession(client, planText, seedHistory)
}

// ── Interactive REPL ─────────────────────────────────────────────────────────

func (cli *CLI) runInteractiveSession(
	client *HTTPTriggerClient,
	planText string,
	history []BoardroomHistoryEntry,
) error {
	printInteractiveBanner()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("\n%sYou:%s ", colBold, colReset)
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// ── Built-in commands ──────────────────────────────────────────────
		if strings.EqualFold(input, "/exit") || strings.EqualFold(input, "/quit") {
			fmt.Println("\nLeaving the boardroom. Goodbye.")
			break
		}

		if strings.EqualFold(input, "/help") {
			printInteractiveHelp()
			continue
		}

		if strings.EqualFold(input, "/personas") {
			printPersonaList()
			continue
		}

		// /final <text> — end session with user's verdict
		if strings.HasPrefix(strings.ToLower(input), "/final") {
			finalText := strings.TrimSpace(input[6:])
			if finalText == "" {
				fmt.Print("Your final decision: ")
				line2, _ := reader.ReadString('\n')
				finalText = strings.TrimSpace(line2)
			}
			if finalText == "" {
				fmt.Println("(No final say recorded — use /exit to leave.)")
				continue
			}
			if err := cli.sendFinalSay(client, planText, history, finalText); err != nil {
				fmt.Printf("Error recording final say: %v\n", err)
			}
			break
		}

		// ── Parse @mention ─────────────────────────────────────────────────
		addressedTo, cleanMessage := parseAtMention(input)

		// ── Send turn to server ────────────────────────────────────────────
		fmt.Printf("%s(thinking...)%s\n", colDim, colReset)

		resp, err := client.SendBoardroomChat(BoardroomChatRequest{
			PlanText:    planText,
			History:     history,
			UserMessage: cleanMessage,
			AddressedTo: addressedTo,
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// ── Display responses ───────────────────────────────────────────────
		fmt.Println()
		for _, r := range resp.Responses {
			printPersonaMessage(r.PersonaID, r.PersonaName, r.Role, r.Content)
		}

		history = resp.UpdatedHistory
	}

	return nil
}

// sendFinalSay sends the user's closing statement and prints the summary.
func (cli *CLI) sendFinalSay(
	client *HTTPTriggerClient,
	planText string,
	history []BoardroomHistoryEntry,
	finalSay string,
) error {
	fmt.Printf("%s(recording your final say and closing the session...)%s\n", colDim, colReset)

	resp, err := client.SendBoardroomChat(BoardroomChatRequest{
		PlanText: planText,
		History:  history,
		FinalSay: finalSay,
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n%s%s═══ FINAL SAY ═══%s\n", colBold, colGreen, colReset)
	fmt.Printf("%sYou:%s %s\n\n", colBold, colReset, finalSay)

	if resp.ClosingSummary != "" {
		fmt.Printf("%s%s─── Closing Summary ───%s\n", colBold, colCyan, colReset)
		fmt.Println(resp.ClosingSummary)
	}

	fmt.Printf("\n%sThe boardroom is adjourned.%s\n", colBold, colReset)
	return nil
}

// ── Display helpers ───────────────────────────────────────────────────────────

func printPersonaMessage(personaID, personaName, role, content string) {
	col := personaColour[personaID]
	if col == "" {
		col = colWhite
	}
	// Header line: coloured name + dim role
	fmt.Printf("%s%s%s%s %s(%s)%s\n", colBold, col, personaName, colReset, colDim, role, colReset)
	// Wrap content slightly indented
	for _, para := range strings.Split(content, "\n") {
		if strings.TrimSpace(para) != "" {
			fmt.Printf("  %s\n", para)
		}
	}
	fmt.Println()
}

// parseAtMention extracts "@id" or "@name" from the message.
// Returns (addressedTo, cleanedMessage).
func parseAtMention(input string) (string, string) {
	knownIDs := map[string]string{
		"alex": "architect", "architect": "architect",
		"sam": "security", "security": "security",
		"priya": "pm", "pm": "pm",
		"devon": "devil", "devil": "devil", "advocate": "devil",
		"eva": "engineer", "engineer": "engineer",
		"ben": "analyst", "analyst": "analyst",
		"sofia": "scalability", "scalability": "scalability",
	}

	words := strings.Fields(input)
	for i, w := range words {
		if strings.HasPrefix(w, "@") {
			handle := strings.ToLower(strings.TrimPrefix(w, "@"))
			if id, ok := knownIDs[handle]; ok {
				clean := strings.Join(append(words[:i], words[i+1:]...), " ")
				return id, strings.TrimSpace(clean)
			}
		}
	}
	return "", input
}

func printInteractiveBanner() {
	fmt.Printf("\n%s%s╔══════════════════════════════════════════════════╗%s\n", colBold, colCyan, colReset)
	fmt.Printf("%s%s║        BOARDROOM — Interactive Session           ║%s\n", colBold, colCyan, colReset)
	fmt.Printf("%s%s╚══════════════════════════════════════════════════╝%s\n", colBold, colCyan, colReset)
	fmt.Println()
	fmt.Println("  The personas are ready to discuss. Speak your mind.")
	fmt.Println("  Address someone with @alex, @sam, @priya, etc.")
	fmt.Println()
	fmt.Println("  Commands:")
	fmt.Printf("    %s/final <decision>%s  — record your verdict and close the session\n", colBold, colReset)
	fmt.Printf("    %s/personas%s          — list all personas\n", colBold, colReset)
	fmt.Printf("    %s/help%s              — show this help\n", colBold, colReset)
	fmt.Printf("    %s/exit%s              — leave without a final say\n", colBold, colReset)
}

func printInteractiveHelp() {
	fmt.Println()
	fmt.Println("  You can speak freely to the group or address someone directly:")
	fmt.Println("    @alex / @architect    — Alex the Architect")
	fmt.Println("    @sam  / @security     — Sam the Security Lead")
	fmt.Println("    @priya / @pm          — Priya the Product Manager")
	fmt.Println("    @devon / @devil       — Devon the Devil's Advocate")
	fmt.Println("    @eva  / @engineer     — Eva the Lead Engineer")
	fmt.Println("    @ben  / @analyst      — Ben the Business Analyst")
	fmt.Println("    @sofia / @scalability — Sofia the Scalability Engineer")
	fmt.Println()
	fmt.Println("  /final <decision>  Close the session with your verdict")
	fmt.Println("  /personas          Show persona colours and focus areas")
	fmt.Println("  /exit              Leave without a final say")
}

func printPersonaList() {
	type personaInfo struct{ id, name, role, col string }
	personas := []personaInfo{
		{"architect", "Alex the Architect", "system design & trade-offs", colCyan},
		{"security", "Sam the Security Lead", "threats, compliance, attack surface", colRed},
		{"pm", "Priya the Product Manager", "user value, scope, timeline", colGreen},
		{"devil", "Devon the Devil's Advocate", "hidden assumptions & failure modes", colMagenta},
		{"engineer", "Eva the Lead Engineer", "implementation complexity & effort", colBlue},
		{"analyst", "Ben the Business Analyst", "ROI, KPIs, business alignment", colYellow},
		{"scalability", "Sofia the Scalability Engineer", "scaling limits, bottlenecks, load failures", colOrange},
	}
	fmt.Println()
	for _, p := range personas {
		fmt.Printf("  %s%s%s%s — %s%s%s\n",
			colBold, p.col, p.name, colReset,
			colDim, p.role, colReset)
	}
	fmt.Println()
}

// ── Misc ─────────────────────────────────────────────────────────────────────

func boardroomOutputFormat(outputPath string) string {
	if strings.HasSuffix(strings.ToLower(outputPath), ".md") {
		return "markdown"
	}
	return "terminal"
}

func printBoardroomUsage() {
	fmt.Println(`Usage:
  devtrack boardroom "<problem>"                     Run boardroom review on an inline problem
  devtrack boardroom --file <plan.md>               Run boardroom review on a plan file
  devtrack boardroom --folder <dir/>                Review all .md plan files in a folder
  devtrack boardroom --file <plan.md> --output <r.md>  Save report to file
  devtrack boardroom --file <plan.md> --interactive  Auto-enter interactive discussion

Interactive mode (available after every review):
  Speak freely to all 7 personas or address one directly:
    @alex  @sam  @priya  @devon  @eva  @ben  @sofia

  /final <decision>   Record your verdict and close with a summary
  /personas           List all personas with focus areas
  /exit               Leave the boardroom

Personas (7):
  Alex the Architect          — system design & trade-offs
  Sam the Security Lead       — threats, compliance, attack surface
  Priya the Product Manager   — user value, scope, timeline
  Devon the Devil's Advocate  — hidden assumptions & failure modes
  Eva the Lead Engineer       — implementation complexity & effort
  Ben the Business Analyst    — ROI, KPIs, business alignment
  Sofia the Scalability Eng.  — scaling limits, bottlenecks, load failures`)
}
