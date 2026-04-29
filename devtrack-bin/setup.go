package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DevTrackMode represents the operating mode chosen during setup.
type DevTrackMode string

const (
	ModeLightweight DevTrackMode = "lightweight"
	ModeExternal    DevTrackMode = "external"
	ModeManaged     DevTrackMode = "managed"
)

// SetupConfig holds all values collected during the setup wizard.
type SetupConfig struct {
	ProjectRoot   string
	WorkspacePath string
	DataDir       string

	// Mode
	Mode DevTrackMode

	// LLM
	LLMProvider    string
	OllamaHost     string
	OllamaModel    string
	OpenAIKey      string
	OpenAIModel    string
	AnthropicKey   string
	AnthropicModel string
	GroqKey        string
	GroqModel      string

	// User identity
	UserEmail string

	// PM integration
	PMPlatform string // "azure" | "github" | "jira" | "none"

	// Optional: GitHub
	GitHubToken string
	GitHubOwner string

	// Optional: Azure DevOps
	AzurePAT          string
	AzureOrganization string
	AzureProject      string
}

// RunSetup implements `devtrack setup` — interactive first-run wizard.
func RunSetup() error {
	reader := bufio.NewReader(os.Stdin)

	printSetupHeader()

	// ── 0. Mode selection ─────────────────────────────────────────────────────
	fmt.Println("Which mode do you want to run DevTrack in?")
	fmt.Println("  [1] Managed    (default) — full AI features. Requires Python backend/ on this machine.")
	fmt.Println("  [2] Lightweight           — git monitoring + scheduling only. No Python needed.")
	fmt.Println("  [3] External              — daemon only; Python runs on a separate server.")
	fmt.Println()
	fmt.Print("Choice [1]: ")
	modeChoice := readLine(reader)
	fmt.Println()

	var selectedMode DevTrackMode
	switch modeChoice {
	case "2":
		selectedMode = ModeLightweight
	case "3":
		selectedMode = ModeExternal
	default:
		selectedMode = ModeManaged
	}

	// ── 1. Detect PROJECT_ROOT ────────────────────────────────────────────────
	var projectRoot string
	if selectedMode == ModeManaged {
		root, err := detectProjectRoot()
		if err != nil {
			return fmt.Errorf("could not locate DevTrack installation: %w\n\nMake sure the devtrack binary is inside the release package (next to backend/)", err)
		}
		projectRoot = root
	} else {
		execPath, err := os.Executable()
		if err == nil {
			projectRoot = filepath.Dir(execPath)
		} else {
			projectRoot, _ = os.Getwd()
		}
	}

	envPath := filepath.Join(projectRoot, ".env")

	// ── 2. Already configured? ────────────────────────────────────────────────
	if _, err := os.Stat(envPath); err == nil {
		fmt.Printf("Found existing configuration at: %s\n\n", envPath)
		fmt.Print("Re-run setup and overwrite? [y/N]: ")
		answer := readLine(reader)
		if strings.ToLower(answer) != "y" {
			fmt.Println("\nSetup cancelled. Existing configuration kept.")
			fmt.Println("Run 'devtrack start' to start the daemon.")
			return nil
		}
		fmt.Println()
	}

	cfg := &SetupConfig{
		ProjectRoot: projectRoot,
		DataDir:     filepath.Join(projectRoot, "Data"),
		Mode:        selectedMode,
	}

	// ── 3. Python backend check ───────────────────────────────────────────────
	if cfg.Mode == ModeManaged {
		checkPythonBackend(projectRoot)
	} else {
		fmt.Println("─── Checking prerequisites ───────────────────────────────────────")
		fmt.Println("[" + string(cfg.Mode) + " mode] Python backend not required — skipping prerequisite check.")
		fmt.Println()
	}

	// ── 4. Workspace path ─────────────────────────────────────────────────────
	fmt.Println("─── Git Repository to Monitor ───────────────────────────────────")
	fmt.Printf("Which git repository should DevTrack monitor?\n")
	fmt.Printf("Press Enter to use: %s\n", projectRoot)
	fmt.Print("Workspace path: ")
	ws := readLine(reader)
	if ws == "" {
		ws = projectRoot
	}
	ws = expandHomePath(ws)
	if !IsGitRepository(ws) {
		fmt.Printf("\nWarning: %s does not appear to be a git repository.\n", ws)
		fmt.Println("You can update DEVTRACK_WORKSPACE in .env later.")
	}
	cfg.WorkspacePath = ws
	fmt.Println()

	// ── 5. LLM provider ──────────────────────────────────────────────────────
	fmt.Println("─── AI / LLM Provider ───────────────────────────────────────────")
	fmt.Println("DevTrack uses an LLM to enhance commit messages, generate reports,")
	fmt.Println("and parse your work updates.")
	fmt.Println()
	fmt.Println("  1) Ollama  (local, free — recommended for privacy)")
	fmt.Println("  2) OpenAI  (cloud, API key required)")
	fmt.Println("  3) Anthropic / Claude (cloud, API key required)")
	fmt.Println("  4) Groq    (cloud, free tier available)")
	fmt.Println("  5) Skip    (configure later in .env)")
	fmt.Print("\nChoice [1]: ")
	choice := readLine(reader)
	if choice == "" {
		choice = "1"
	}

	switch choice {
	case "1":
		cfg.LLMProvider = "ollama"
		fmt.Print("Ollama host [http://localhost:11434]: ")
		host := readLine(reader)
		if host == "" {
			host = "http://localhost:11434"
		}
		cfg.OllamaHost = host
		fmt.Print("Ollama model [llama3.2]: ")
		model := readLine(reader)
		if model == "" {
			model = "llama3.2"
		}
		cfg.OllamaModel = model

	case "2":
		cfg.LLMProvider = "openai"
		fmt.Print("OpenAI API key: ")
		cfg.OpenAIKey = readLine(reader)
		fmt.Print("OpenAI model [gpt-4o-mini]: ")
		model := readLine(reader)
		if model == "" {
			model = "gpt-4o-mini"
		}
		cfg.OpenAIModel = model

	case "3":
		cfg.LLMProvider = "anthropic"
		fmt.Print("Anthropic API key: ")
		cfg.AnthropicKey = readLine(reader)
		fmt.Print("Anthropic model [claude-haiku-4-5]: ")
		model := readLine(reader)
		if model == "" {
			model = "claude-haiku-4-5"
		}
		cfg.AnthropicModel = model

	case "4":
		cfg.LLMProvider = "groq"
		fmt.Print("Groq API key: ")
		cfg.GroqKey = readLine(reader)
		fmt.Print("Groq model [llama-3.3-70b-versatile]: ")
		model := readLine(reader)
		if model == "" {
			model = "llama-3.3-70b-versatile"
		}
		cfg.GroqModel = model

	default:
		cfg.LLMProvider = "ollama"
		cfg.OllamaHost = "http://localhost:11434"
		cfg.OllamaModel = "llama3.2"
	}
	fmt.Println()

	// ── 6. User email ─────────────────────────────────────────────────────────
	fmt.Println("─── Your Identity ───────────────────────────────────────────────")
	fmt.Println("Used for filtering your own comments in integrations and reports.")
	fmt.Print("Your email address (optional, Enter to skip): ")
	cfg.UserEmail = readLine(reader)
	fmt.Println()

	// ── 7. Project management platform ───────────────────────────────────────
	fmt.Println("─── Project Management Integration ──────────────────────────────")
	fmt.Println("DevTrack can sync work updates to your PM platform.")
	fmt.Println()
	fmt.Println("  1) GitHub Issues")
	fmt.Println("  2) Azure DevOps")
	fmt.Println("  3) Jira")
	fmt.Println("  4) None / skip (configure later)")
	fmt.Print("\nChoice [4]: ")
	pmChoice := readLine(reader)
	if pmChoice == "" {
		pmChoice = "4"
	}

	switch pmChoice {
	case "1":
		cfg.PMPlatform = "github"
		fmt.Print("GitHub personal access token: ")
		cfg.GitHubToken = readLine(reader)
		fmt.Print("GitHub owner (username or org): ")
		cfg.GitHubOwner = readLine(reader)

	case "2":
		cfg.PMPlatform = "azure"
		fmt.Print("Azure DevOps personal access token: ")
		cfg.AzurePAT = readLine(reader)
		fmt.Print("Azure organization name: ")
		cfg.AzureOrganization = readLine(reader)
		fmt.Print("Azure project name: ")
		cfg.AzureProject = readLine(reader)

	case "3":
		cfg.PMPlatform = "jira"
		fmt.Println("(Configure JIRA_* variables in .env after setup)")

	default:
		cfg.PMPlatform = "none"
	}
	fmt.Println()

	// ── 8. Create directories ─────────────────────────────────────────────────
	fmt.Println("─── Creating directories ─────────────────────────────────────────")
	if err := createDataDirectories(cfg.DataDir); err != nil {
		return fmt.Errorf("failed to create data directories: %w", err)
	}

	// ── 9. Write .env ─────────────────────────────────────────────────────────
	fmt.Printf("\nWriting configuration to: %s\n", envPath)
	envContent := generateEnvContent(cfg)
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		return fmt.Errorf("failed to write .env: %w", err)
	}
	fmt.Println("✓ .env written")

	// Register the .env path in ~/.devtrack/devtrack.conf so every subsequent
	// `devtrack` invocation auto-loads it — no manual sourcing needed.
	if err := RegisterEnvFile(envPath); err != nil {
		fmt.Printf("  Warning: could not register .env path: %v\n", err)
		fmt.Printf("  You can still run: source %s && devtrack start\n", envPath)
	} else {
		home, _ := os.UserHomeDir()
		fmt.Printf("✓ Registered at ~/.devtrack/devtrack.conf\n")
		fmt.Printf("  Future 'devtrack' commands auto-load %s\n", filepath.Join(home, devtrackConfFile))
	}

	// ── 10. Shell integration ─────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("─── Shell Integration ────────────────────────────────────────────")
	fmt.Println("Shell integration enables bare 'git commit' to use AI enhancement.")
	fmt.Print("Set up shell integration now? [Y/n]: ")
	shellAnswer := readLine(reader)
	if shellAnswer == "" || strings.ToLower(shellAnswer) == "y" {
		printShellInitInstructions(projectRoot, envPath)
	}

	// ── 11. Autostart ─────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("─── Autostart ────────────────────────────────────────────────────")
	fmt.Println("Autostart keeps DevTrack running automatically after login.")
	fmt.Print("Set up autostart now? [Y/n]: ")
	autostartAnswer := readLine(reader)
	if autostartAnswer == "" || strings.ToLower(autostartAnswer) == "y" {
		printAutostartInstructions(projectRoot, envPath)
	}

	// ── Done ──────────────────────────────────────────────────────────────────
	printSetupComplete(projectRoot, envPath, cfg.Mode)
	return nil
}

// detectProjectRoot finds the DevTrack installation root.
// It walks up from the binary location looking for the backend/ directory.
func detectProjectRoot() (string, error) {
	// 1. Explicit env var
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		return root, nil
	}

	// 2. Walk up from binary
	execPath, err := os.Executable()
	if err == nil {
		execPath, _ = filepath.Abs(execPath)
		searchDir := filepath.Dir(execPath)
		for i := 0; i < 6; i++ {
			if _, err := os.Stat(filepath.Join(searchDir, "backend")); err == nil {
				return searchDir, nil
			}
			parent := filepath.Dir(searchDir)
			if parent == searchDir {
				break
			}
			searchDir = parent
		}
	}

	// 3. Current working directory
	cwd, err := os.Getwd()
	if err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "backend")); err == nil {
			return cwd, nil
		}
	}

	return "", fmt.Errorf("backend/ directory not found near binary")
}

// createDataDirectories creates all required Data/ subdirectories.
func createDataDirectories(dataDir string) error {
	dirs := []string{
		dataDir,
		filepath.Join(dataDir, "db"),
		filepath.Join(dataDir, "logs"),
		filepath.Join(dataDir, "pids"),
		filepath.Join(dataDir, "configs"),
		filepath.Join(dataDir, "learning"),
		filepath.Join(dataDir, "learning", "chroma"),
		filepath.Join(dataDir, "reports"),
		filepath.Join(dataDir, "tls"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		fmt.Printf("  ✓ %s\n", d)
	}
	return nil
}

// generateEnvContent produces a complete .env file from the wizard answers.
func generateEnvContent(cfg *SetupConfig) string {
	dataDir := cfg.DataDir
	now := time.Now().Format("2006-01-02")

	// Determine LLM model for git-sage
	gitSageModel := cfg.OllamaModel
	if gitSageModel == "" {
		switch cfg.LLMProvider {
		case "openai":
			gitSageModel = cfg.OpenAIModel
		case "anthropic":
			gitSageModel = cfg.AnthropicModel
		case "groq":
			gitSageModel = cfg.GroqModel
		default:
			gitSageModel = "llama3.2"
		}
	}

	var b strings.Builder

	b.WriteString("# DevTrack configuration — generated by 'devtrack setup' on " + now + "\n")
	b.WriteString("# Edit this file to customize. Re-run 'devtrack setup' to reset.\n\n")

	b.WriteString("## PATHS\n")
	b.WriteString("PROJECT_ROOT=" + cfg.ProjectRoot + "\n")
	b.WriteString("DEVTRACK_HOME=" + filepath.Join(cfg.ProjectRoot, "devtrack-bin") + "\n")
	b.WriteString("DEVTRACK_WORKSPACE=" + cfg.WorkspacePath + "\n")
	b.WriteString("DATA_DIR=" + dataDir + "\n")
	b.WriteString("DATABASE_DIR=" + filepath.Join(dataDir, "db") + "\n")
	b.WriteString("LOG_DIR=" + filepath.Join(dataDir, "logs") + "\n")
	b.WriteString("PID_DIR=" + filepath.Join(dataDir, "pids") + "\n")
	b.WriteString("CONFIG_DIR_PATH=" + filepath.Join(dataDir, "configs") + "\n")
	b.WriteString("LEARNING_DIR_PATH=" + filepath.Join(dataDir, "learning") + "\n\n")

	b.WriteString("## DAEMON INTERNALS\n")
	b.WriteString("DEVTRACK_SERVER_MODE=" + string(cfg.Mode) + "\n")
	b.WriteString("DEVTRACK_SERVER_URL=\n")
	b.WriteString("DEVTRACK_TLS=true\n")
	b.WriteString("DEVTRACK_API_KEY=\n")
	b.WriteString("IPC_HOST=127.0.0.1\n")
	b.WriteString("IPC_PORT=35893\n")
	b.WriteString("IPC_CONNECT_TIMEOUT_SECS=5\n")
	b.WriteString("IPC_RETRY_DELAY_MS=2000\n")
	b.WriteString("PYTHON_BRIDGE_SCRIPT=python_bridge.py\n")
	b.WriteString("CLI_BINARY_NAME=devtrack\n")
	b.WriteString("CONFIG_FILE_NAME=config.yaml\n")
	b.WriteString("DATABASE_FILE_NAME=devtrack.db\n")
	b.WriteString("PID_FILE_NAME=daemon.pid\n")
	b.WriteString("LOG_FILE_NAME=daemon.log\n")
	b.WriteString("LEARNING_DIR_NAME=learning\n")
	b.WriteString("CONFIG_DIR_NAME=.devtrack\n")
	b.WriteString("CLI_APP_NAME=DevTrack\n")
	b.WriteString("CLI_DAEMON_NAME=devtrack\n")
	b.WriteString("DEVTRACK_COMMIT_ONLY=false\n\n")

	b.WriteString("## LLM PROVIDERS\n")
	b.WriteString("LLM_PROVIDER=" + cfg.LLMProvider + "\n\n")
	b.WriteString("# Ollama (local)\n")
	ollamaHost := cfg.OllamaHost
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}
	ollamaModel := cfg.OllamaModel
	if ollamaModel == "" {
		ollamaModel = "llama3.2"
	}
	b.WriteString("OLLAMA_HOST=" + ollamaHost + "\n")
	b.WriteString("OLLAMA_MODEL=" + ollamaModel + "\n\n")
	b.WriteString("LMSTUDIO_HOST=http://localhost:1234/v1\n\n")
	b.WriteString("# OpenAI\n")
	b.WriteString("OPENAI_API_KEY=" + cfg.OpenAIKey + "\n")
	openAIModel := cfg.OpenAIModel
	if openAIModel == "" {
		openAIModel = "gpt-4o-mini"
	}
	b.WriteString("OPENAI_MODEL=" + openAIModel + "\n\n")
	b.WriteString("# Anthropic\n")
	b.WriteString("ANTHROPIC_API_KEY=" + cfg.AnthropicKey + "\n")
	anthropicModel := cfg.AnthropicModel
	if anthropicModel == "" {
		anthropicModel = "claude-haiku-4-5"
	}
	b.WriteString("ANTHROPIC_MODEL=" + anthropicModel + "\n\n")
	b.WriteString("# Groq\n")
	b.WriteString("GROQ_API_KEY=" + cfg.GroqKey + "\n")
	b.WriteString("GROQ_HOST=https://api.groq.com/openai/v1\n")
	groqModel := cfg.GroqModel
	if groqModel == "" {
		groqModel = "llama-3.3-70b-versatile"
	}
	b.WriteString("GROQ_MODEL=" + groqModel + "\n\n")

	b.WriteString("## GIT-SAGE\n")
	b.WriteString("GIT_SAGE_PROVIDER=" + cfg.LLMProvider + "\n")
	b.WriteString("GIT_SAGE_DEFAULT_MODEL=" + gitSageModel + "\n")
	b.WriteString("GIT_SAGE_BASE_URL=\n")
	b.WriteString("GIT_SAGE_API_KEY=\n\n")

	b.WriteString("## LLM GENERATION PARAMETERS\n")
	b.WriteString("COMMIT_LLM_TEMPERATURE=0.1\n")
	b.WriteString("COMMIT_LLM_MAX_TOKENS=1000\n")
	b.WriteString("REPORT_LLM_TEMPERATURE=0.3\n")
	b.WriteString("REPORT_LLM_MAX_TOKENS=600\n")
	b.WriteString("PERSONALIZATION_LLM_TEMPERATURE=0.7\n")
	b.WriteString("PERSONALIZATION_LLM_MAX_TOKENS=300\n")
	b.WriteString("DESCRIPTION_LLM_TEMPERATURE=0.3\n")
	b.WriteString("DESCRIPTION_LLM_MAX_TOKENS=300\n\n")

	b.WriteString("## TIMEOUTS AND DELAYS\n")
	b.WriteString("HTTP_TIMEOUT_SHORT=10\n")
	b.WriteString("HTTP_TIMEOUT=30\n")
	b.WriteString("HTTP_TIMEOUT_LONG=60\n")
	b.WriteString("LLM_REQUEST_TIMEOUT_SECS=120\n")
	b.WriteString("PROMPT_TIMEOUT_SIMPLE_SECS=30\n")
	b.WriteString("PROMPT_TIMEOUT_WORK_SECS=60\n")
	b.WriteString("PROMPT_TIMEOUT_TASK_SECS=120\n")
	b.WriteString("SENTIMENT_ANALYSIS_WINDOW_MINUTES=120\n\n")

	b.WriteString("## APPLICATION SETTINGS\n")
	b.WriteString("PROMPT_INTERVAL=30\n")
	b.WriteString("WORK_HOURS_ONLY=true\n")
	b.WriteString("WORK_START_HOUR=9\n")
	b.WriteString("WORK_END_HOUR=18\n")
	b.WriteString("TIMEZONE=UTC\n")
	b.WriteString("LOG_LEVEL=info\n")
	b.WriteString("AUTO_SYNC=true\n")
	b.WriteString("OUTPUT_TYPE=both\n")
	b.WriteString("DAILY_REPORT_TIME=18:00\n")
	b.WriteString("WEEKLY_REPORT_DAY=Friday\n")
	b.WriteString("SEND_ON_TRIGGER=false\n")
	b.WriteString("SEND_DAILY_SUMMARY=true\n\n")

	b.WriteString("## IDENTITY\n")
	b.WriteString("EMAIL=" + cfg.UserEmail + "\n")
	b.WriteString("EMAIL_TO_ADDRESSES=" + cfg.UserEmail + "\n")
	b.WriteString("EMAIL_CC_ADDRESSES=\n")
	b.WriteString("EMAIL_MANAGER=\n")
	b.WriteString("EMAIL_SUBJECT=DevTrack Daily Report\n\n")

	b.WriteString("## TEAMS\n")
	b.WriteString("TEAMS_CHANNEL_ID=\n")
	b.WriteString("TEAMS_CHANNEL_NAME=\n")
	b.WriteString("TEAMS_CHAT_ID=\n")
	b.WriteString("TEAMS_CHAT_TYPE=channel\n")
	b.WriteString("TEAMS_WEBHOOK_URL=\n")
	b.WriteString("TEAMS_MENTION_USER=false\n")
	b.WriteString("SENTIMENT_TARGET_SENDER=\n\n")

	b.WriteString("## GITHUB\n")
	b.WriteString("GITHUB_TOKEN=" + cfg.GitHubToken + "\n")
	b.WriteString("GITHUB_OWNER=" + cfg.GitHubOwner + "\n")
	b.WriteString("GITHUB_REPO=\n")
	b.WriteString("GITHUB_API_URL=\n")
	b.WriteString("GITHUB_API_VERSION=2022-11-28\n")
	b.WriteString("GITHUB_SYNC_ENABLED=false\n")
	b.WriteString("GITHUB_AUTO_COMMENT=true\n")
	b.WriteString("GITHUB_AUTO_TRANSITION=false\n")
	b.WriteString("GITHUB_CREATE_ON_NO_MATCH=false\n")
	b.WriteString("GITHUB_MATCH_THRESHOLD=0.6\n")
	b.WriteString("GITHUB_DONE_STATE=closed\n")
	b.WriteString("GITHUB_SYNC_LABEL=devtrack\n")
	b.WriteString("GITHUB_AUTO_UPDATE_DESCRIPTION=false\n")
	b.WriteString("GITHUB_SYNC_WINDOW_HOURS=0\n")
	b.WriteString("GITHUB_LOG_PATH=\n\n")

	b.WriteString("## AZURE DEVOPS\n")
	b.WriteString("AZURE_DEVOPS_PAT=" + cfg.AzurePAT + "\n")
	b.WriteString("AZURE_ORGANIZATION=" + cfg.AzureOrganization + "\n")
	b.WriteString("AZURE_PROJECT=" + cfg.AzureProject + "\n")
	b.WriteString("AZURE_API_KEY=\n")
	b.WriteString("AZURE_API_VERSION=7.1\n")
	b.WriteString("AZURE_EXCEL_FILE=" + filepath.Join(cfg.ProjectRoot, "backend", "data", "tasks.xlsx") + "\n")
	b.WriteString("AZURE_EXCEL_SHEET=my_tasks\n")
	b.WriteString("AZURE_PARENT_WORK_ITEM_ID=\n")
	b.WriteString("AZURE_STARTING_WORK_ITEM_ID=0\n")
	b.WriteString("AZURE_EMAIL=" + cfg.UserEmail + "\n")
	b.WriteString("AZURE_SYNC_ENABLED=false\n")
	b.WriteString("AZURE_SYNC_AUTO_COMMENT=true\n")
	b.WriteString("AZURE_SYNC_AUTO_TRANSITION=false\n")
	b.WriteString("AZURE_SYNC_CREATE_ON_NO_MATCH=false\n")
	b.WriteString("AZURE_SYNC_MATCH_THRESHOLD=0.7\n")
	b.WriteString("AZURE_SYNC_WINDOW_HOURS=0\n")
	b.WriteString("AZURE_POLL_ENABLED=false\n")
	b.WriteString("AZURE_POLL_INTERVAL_MINS=5\n\n")

	b.WriteString("## GITLAB\n")
	b.WriteString("GITLAB_URL=https://gitlab.com\n")
	b.WriteString("GITLAB_PAT=\n")
	b.WriteString("GITLAB_PROJECT_ID=\n")
	b.WriteString("GITLAB_SYNC_ENABLED=false\n")
	b.WriteString("GITLAB_SYNC_WINDOW_HOURS=0\n")
	b.WriteString("GITLAB_AUTO_COMMENT=true\n")
	b.WriteString("GITLAB_AUTO_TRANSITION=false\n")
	b.WriteString("GITLAB_CREATE_ON_NO_MATCH=false\n")
	b.WriteString("GITLAB_MATCH_THRESHOLD=0.6\n")
	b.WriteString("GITLAB_DONE_STATE=closed\n")
	b.WriteString("GITLAB_SYNC_LABEL=devtrack\n")
	b.WriteString("GITLAB_AUTO_UPDATE_DESCRIPTION=false\n")
	b.WriteString("GITLAB_POLL_ENABLED=false\n")
	b.WriteString("GITLAB_POLL_INTERVAL_MINS=5\n\n")

	b.WriteString("## JIRA\n")
	b.WriteString("JIRA_API_TOKEN=\n")
	b.WriteString("JIRA_URL=https://yourorg.atlassian.net\n")
	b.WriteString("JIRA_EMAIL=" + cfg.UserEmail + "\n")
	b.WriteString("JIRA_PROJECT_KEY=PROJ\n\n")

	b.WriteString("## WEBHOOK SERVER\n")
	b.WriteString("WEBHOOK_ENABLED=false\n")
	b.WriteString("WEBHOOK_PORT=8089\n")
	b.WriteString("WEBHOOK_HOST=0.0.0.0\n")
	b.WriteString("WEBHOOK_AZURE_USERNAME=devtrack\n")
	b.WriteString("WEBHOOK_AZURE_PASSWORD=\n")
	b.WriteString("WEBHOOK_GITHUB_SECRET=\n")
	b.WriteString("WEBHOOK_GITLAB_SECRET=\n")
	b.WriteString("GITLAB_PROJECT_IDS=\n")
	b.WriteString("DEVTRACK_WEBHOOK_PUBLIC_URL=\n")
	b.WriteString("WEBHOOK_NOTIFY_OS=true\n")
	b.WriteString("WEBHOOK_NOTIFY_TERMINAL=true\n")
	b.WriteString("SHUTDOWN_GRACE_PERIOD_SECONDS=0.5\n\n")

	b.WriteString("## TICKET ALERTER\n")
	b.WriteString("ALERT_ENABLED=true\n")
	b.WriteString("ALERT_POLL_INTERVAL_SECS=300\n")
	b.WriteString("ALERT_GITHUB_ENABLED=true\n")
	b.WriteString("ALERT_AZURE_ENABLED=true\n")
	b.WriteString("ALERT_NOTIFY_ASSIGNED=true\n")
	b.WriteString("ALERT_NOTIFY_COMMENTS=true\n")
	b.WriteString("ALERT_NOTIFY_STATUS_CHANGES=true\n")
	b.WriteString("ALERT_NOTIFY_REVIEW_REQUESTED=true\n")
	b.WriteString("ALERT_JIRA_ENABLED=true\n\n")

	b.WriteString("## LEARNING AND PERSONALIZATION\n")
	b.WriteString("LEARNING_PYTHON_PATH=python3\n")
	b.WriteString("LEARNING_SCRIPT_PATH=" + filepath.Join(cfg.ProjectRoot, "backend", "learning_integration.py") + "\n")
	b.WriteString("LEARNING_DAILY_SCRIPT_PATH=" + filepath.Join(cfg.ProjectRoot, "backend", "run_daily_learning.py") + "\n")
	b.WriteString("LEARNING_DEFAULT_DAYS=30\n")
	b.WriteString("LEARNING_CRON_ENABLED=false\n")
	b.WriteString("LEARNING_CRON_SCHEDULE=0 20 * * *\n")
	b.WriteString("LEARNING_HISTORY_DAYS=30\n")
	b.WriteString("PERSONALIZATION_RAG_ENABLED=true\n")
	b.WriteString("PERSONALIZATION_EMBED_MODEL=nomic-embed-text\n")
	b.WriteString("PERSONALIZATION_RAG_K=3\n")
	b.WriteString("PERSONALIZATION_CHROMA_DIR=" + filepath.Join(dataDir, "learning", "chroma") + "\n\n")

	b.WriteString("## SEMANTIC MODEL\n")
	b.WriteString("SEMANTIC_MODEL_NAME=all-MiniLM-L6-v2\n\n")

	b.WriteString("## HEALTH AND QUEUE\n")
	b.WriteString("HEALTH_CHECK_INTERVAL_SECS=30\n")
	b.WriteString("HEALTH_AUTO_RESTART_PYTHON=true\n")
	b.WriteString("HEALTH_AUTO_RESTART_WEBHOOK=true\n")
	b.WriteString("HEALTH_MAX_RESTARTS_PER_HOUR=3\n")
	b.WriteString("QUEUE_DRAIN_INTERVAL_SECS=10\n")
	b.WriteString("QUEUE_MAX_RETRIES=10\n")
	b.WriteString("QUEUE_RETENTION_DAYS=7\n")
	b.WriteString("DEFERRED_COMMIT_EXPIRY_HOURS=72\n\n")

	b.WriteString("## ADMIN CONSOLE\n")
	b.WriteString("ADMIN_PORT=8090\n")
	b.WriteString("ADMIN_HOST=0.0.0.0\n")
	b.WriteString("ADMIN_USERNAME=admin\n")
	b.WriteString("ADMIN_PASSWORD=changeme\n")
	b.WriteString("ADMIN_SECRET_KEY=\n")
	b.WriteString("ADMIN_EMBED=false\n")
	b.WriteString("ADMIN_SESSION_HOURS=8\n")
	b.WriteString("SCRYPT_N=16384\n")
	b.WriteString("SCRYPT_R=8\n")
	b.WriteString("SCRYPT_P=1\n")
	b.WriteString("SCRYPT_DKLEN=32\n")
	b.WriteString("STATS_REFRESH_INTERVAL_SECONDS=30\n")
	b.WriteString("PROCESS_REFRESH_INTERVAL_SECONDS=15\n")
	b.WriteString("AUDIT_LOG_LIMIT=200\n")
	b.WriteString("LICENSE_CONTACT_EMAIL=license@devtrack.dev\n\n")

	b.WriteString("## PM AGENT\n")
	b.WriteString("PM_AGENT_MAX_ITEMS_PER_LEVEL=10\n")
	b.WriteString("PM_AGENT_DEFAULT_PLATFORM=" + cfg.PMPlatform + "\n\n")

	b.WriteString("## PROJECT SYNC\n")
	b.WriteString("PROJECT_SYNC_ENABLED=false\n")
	b.WriteString("PROJECT_SYNC_INTERVAL_SECS=300\n\n")

	b.WriteString("## TELEGRAM\n")
	b.WriteString("TELEGRAM_ENABLED=false\n")
	b.WriteString("TELEGRAM_BOT_TOKEN=\n")
	b.WriteString("TELEGRAM_ALLOWED_CHAT_IDS=\n")
	b.WriteString("TELEGRAM_NOTIFY_COMMITS=false\n")
	b.WriteString("TELEGRAM_NOTIFY_TRIGGERS=true\n")
	b.WriteString("TELEGRAM_NOTIFY_HEALTH=true\n")
	b.WriteString("HEALTH_AUTO_RESTART_TELEGRAM=true\n\n")

	b.WriteString("## SLACK\n")
	b.WriteString("SLACK_ENABLED=false\n")
	b.WriteString("SLACK_BOT_TOKEN=\n")
	b.WriteString("SLACK_APP_TOKEN=\n")
	b.WriteString("SLACK_ALLOWED_CHANNEL_IDS=\n\n")

	b.WriteString("## VACATION MODE\n")
	b.WriteString("VACATION_CONFIDENCE_THRESHOLD=0.7\n")
	b.WriteString("VACATION_AUTO_SUBMIT=true\n\n")

	b.WriteString("## WORK SESSION TRACKING\n")
	b.WriteString("EOD_REPORT_HOUR=18\n")
	b.WriteString("EOD_REPORT_EMAIL=" + cfg.UserEmail + "\n")
	b.WriteString("WORK_SESSION_AUTO_STOP_MINUTES=0\n\n")

	b.WriteString("## PROJECT PLANNING\n")
	b.WriteString("NEWPROJECT_ENABLED=true\n")
	b.WriteString("SPEC_REVIEW_BASE_URL=http://localhost:8089\n\n")

	b.WriteString("## COMMIT ENHANCEMENT\n")
	b.WriteString("COMMIT_ENHANCE_MODE=false\n\n")

	b.WriteString("## INFRASTRUCTURE (optional — only needed for MongoDB/Redis/PostgreSQL)\n")
	b.WriteString("MONGO_USER=devtrack\n")
	b.WriteString("MONGO_PASSWORD=devtrack\n")
	b.WriteString("MONGO_PORT=27017\n")
	b.WriteString("MONGODB_DB_NAME=devtrack\n")
	b.WriteString("MONGODB_URI=\n")
	b.WriteString("REDIS_PASSWORD=devtrack\n")
	b.WriteString("REDIS_PORT=6379\n")
	b.WriteString("REDIS_MAX_MEMORY=256mb\n")
	b.WriteString("REDIS_URL=\n")
	b.WriteString("POSTGRES_USER=devtrack\n")
	b.WriteString("POSTGRES_PASSWORD=devtrack\n")
	b.WriteString("POSTGRES_DB=devtrack\n")
	b.WriteString("POSTGRES_PORT=5432\n")
	b.WriteString("POSTGRES_URL=\n\n")

	b.WriteString("## AZURE AD (optional — for MS Graph / Teams / email)\n")
	b.WriteString("AZURE_CLIENT_ID=\n")
	b.WriteString("AZURE_TENANT_ID=\n")
	b.WriteString("AZURE_CLIENT_SECRET=\n\n")

	b.WriteString("## BUILD METADATA\n")
	b.WriteString("DEVTRACK_VERSION=" + GetDevTrackVersion() + "\n")
	b.WriteString("DEVTRACK_BUILD_DATE=" + now + "\n")
	b.WriteString("DEVTRACK_AUTO_ACCEPT_TERMS=false\n")
	b.WriteString("DEVTRACK_API_URL=\n\n")

	return b.String()
}

// checkPythonBackend verifies the Python backend and uv are present.
func checkPythonBackend(projectRoot string) {
	fmt.Println("─── Checking prerequisites ───────────────────────────────────────")

	// Check backend directory
	backendDir := filepath.Join(projectRoot, "backend")
	if _, err := os.Stat(backendDir); err != nil {
		fmt.Println("  ✗ backend/ directory not found")
		fmt.Println("    Download the full DevTrack release package (not just the binary).")
	} else {
		fmt.Println("  ✓ backend/ directory found")
	}

	// Check uv
	if _, err := exec.LookPath("uv"); err != nil {
		fmt.Println("  ✗ uv not found — Python dependency manager required")
		fmt.Println("    Install uv: https://docs.astral.sh/uv/getting-started/installation/")
		fmt.Println("    Or: curl -LsSf https://astral.sh/uv/install.sh | sh")
	} else {
		fmt.Println("  ✓ uv found")
	}

	// Check Python
	pythonBin := "python3"
	if runtime.GOOS == "windows" {
		pythonBin = "python"
	}
	if _, err := exec.LookPath(pythonBin); err != nil {
		fmt.Printf("  ✗ %s not found — Python 3.9+ required\n", pythonBin)
	} else {
		fmt.Printf("  ✓ %s found\n", pythonBin)
	}

	fmt.Println()
}

// printShellInitInstructions prints what to add to the shell profile.
func printShellInitInstructions(projectRoot, envPath string) {
	shell := os.Getenv("SHELL")
	profileFile := "~/.bashrc"
	if strings.Contains(shell, "zsh") {
		profileFile = "~/.zshrc"
	} else if strings.Contains(shell, "fish") {
		profileFile = "~/.config/fish/config.fish"
	}

	fmt.Printf("\nAdd the following to %s:\n\n", profileFile)
	fmt.Println("  # DevTrack shell integration (enables bare 'git commit' AI enhancement)")
	fmt.Printf("  eval \"$(devtrack shell-init)\"\n")
	fmt.Println()
	fmt.Println("Note: env vars are auto-loaded by the binary — no 'source .env' needed.")
	fmt.Println("Then restart your shell or run: source " + profileFile)
}

// printAutostartInstructions shows the autostart command.
func printAutostartInstructions(projectRoot, envPath string) {
	fmt.Println()
	fmt.Println("Run the following to install autostart:")
	fmt.Println()
	fmt.Println("  devtrack autostart-install")
	fmt.Println()
	fmt.Println("DevTrack will start automatically after login. The .env is auto-loaded")
	fmt.Println("by the binary, so no manual sourcing is needed.")
}

// printSetupHeader prints the welcome banner.
func printSetupHeader() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║          DevTrack — First-Run Setup Wizard                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("This wizard creates your .env configuration and Data/ directories.")
	fmt.Println("You can re-run 'devtrack setup' at any time to reconfigure.")
	fmt.Println()
}

// printSetupComplete prints the completion summary.
func printSetupComplete(projectRoot, envPath string, mode DevTrackMode) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Setup complete!                                                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	if mode == ModeManaged {
		fmt.Println("Next steps:")
		fmt.Println()
		fmt.Printf("  1. Install Python dependencies:\n")
		fmt.Printf("       cd %s && uv sync\n", projectRoot)
		fmt.Println()
		fmt.Println("  2. Start DevTrack (env auto-loaded — no sourcing needed):")
		fmt.Println("       devtrack start")
		fmt.Println()
		fmt.Println("  3. Check status:  devtrack status")
		fmt.Println("  4. View logs:     devtrack logs")
		fmt.Println("  5. Add workspace: devtrack workspace add <path>")
		fmt.Println()
		fmt.Println("Edit .env at any time to add integrations (GitHub, Azure, Jira, etc.)")
	} else {
		fmt.Println("Next steps:")
		fmt.Println("  1. Start DevTrack:  devtrack start")
		fmt.Println("  2. Check status:    devtrack status")
		fmt.Println()
		fmt.Println("Note: AI features (reports, integrations, commit enhancement) require Managed mode.")
		fmt.Println("Re-run 'devtrack setup' and choose [1] to enable them.")
	}
	fmt.Println()
}

// expandHomePath expands a leading ~ in a path.
func expandHomePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// readLine reads a line from stdin, trimming whitespace.
func readLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}
