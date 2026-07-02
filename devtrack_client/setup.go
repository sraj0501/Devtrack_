package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// devtrackServerRepoURL is the GitHub repository that contains devtrack_server/.
const devtrackServerRepoURL = "https://github.com/sraj0501/Devtrack_.git"

// devtrackServerBranch is the branch used for sparse-checkout cloning.
const devtrackServerBranch = "main"

// DevTrackMode represents the operating mode chosen during setup.
type DevTrackMode string

const (
	ModeExternal DevTrackMode = "external"
	ModeManaged  DevTrackMode = "managed"
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
	fmt.Println("  [1] Managed   (default) — daemon spawns Python server automatically.")
	fmt.Println("                            Full AI features: reports, commit enhancement, integrations.")
	fmt.Println("                            Python must be installed on this machine.")
	fmt.Println("  [2] External            — daemon connects to a Python server elsewhere.")
	fmt.Println("                            Same machine (separate process), LAN, or cloud.")
	fmt.Println("                            Set DEVTRACK_SERVER_URL and DEVTRACK_API_KEY in .env,")
	fmt.Println("                            or use: devtrack cloud login --url URL --key KEY")
	fmt.Println()
	fmt.Println("  Note: git monitoring, scheduling, git-sage, and connector sync work in both modes.")
	fmt.Println("        AI enrichment (NLP, reports, boardroom) requires the Python server.")
	fmt.Println()
	fmt.Print("Choice [1]: ")
	modeChoice := readLine(reader)
	fmt.Println()

	var selectedMode DevTrackMode
	switch modeChoice {
	case "2":
		selectedMode = ModeExternal
	default:
		selectedMode = ModeManaged
	}

	// ── 1. Detect PROJECT_ROOT ────────────────────────────────────────────────
	var projectRoot string
	if selectedMode == ModeManaged {
		root, err := detectProjectRoot()
		if err != nil {
			// Python server not found — offer automatic sparse-checkout clone.
			xdgHome, xdgErr := devtrackDataHome()
			if xdgErr != nil {
				return fmt.Errorf("could not determine DevTrack home directory: %w", xdgErr)
			}
			fmt.Println()
			fmt.Println("Python server not found. DevTrack can clone it automatically (sparse checkout, ~5MB).")
			fmt.Printf("Install location: %s\n", filepath.Join(xdgHome, "server"))
			fmt.Print("Clone and install now? [Y/n]: ")
			cloneAnswer := readLine(reader)
			fmt.Println()
			if cloneAnswer == "" || strings.ToLower(cloneAnswer) == "y" {
				clonedRoot, cloneErr := cloneAndInstallServer(xdgHome)
				if cloneErr != nil {
					return fmt.Errorf("server install failed: %w", cloneErr)
				}
				projectRoot = clonedRoot
			} else {
				return fmt.Errorf("setup cancelled: managed mode requires the Python server.\n" +
					"Re-run 'devtrack setup' and choose to clone, or set PROJECT_ROOT to an existing devtrack_server/ path")
			}
		} else {
			projectRoot = root
		}
	} else {
		execPath, err := os.Executable()
		if err == nil {
			projectRoot = filepath.Dir(execPath)
		} else {
			projectRoot, _ = os.Getwd()
		}
	}

		// ── 1b. XDG data home ────────────────────────────────────────────────────
	xdgHome, err := devtrackDataHome()
	if err != nil {
		return fmt.Errorf("could not determine data home: %w", err)
	}
	envPath := filepath.Join(xdgHome, ".env")

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
		DataDir:     filepath.Join(xdgHome, "data"),
		Mode:        selectedMode,
	}

	// ── 3. Prerequisites check ────────────────────────────────────────────────
	fmt.Println("─── Checking prerequisites ───────────────────────────────────────")
	checkCommonPrereqs()
	if cfg.Mode == ModeManaged {
		if err := checkPythonBackend(projectRoot); err != nil {
			return fmt.Errorf("Python backend check failed: %w", err)
		}
	} else {
		fmt.Println("[" + string(cfg.Mode) + " mode] Python backend not required — skipping.")
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
		if err := offerGitInit(ws); err != nil {
			// User declined or init failed — warn and continue; they can fix it later.
			fmt.Printf("  Note: %s is not a git repository. Update DEVTRACK_WORKSPACE in .env when ready.\n", ws)
		}
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

	// ── 8. Create directories and workspaces.yaml ────────────────────────────
	fmt.Println("─── Creating directories ─────────────────────────────────────────")
	if err := createDataDirectories(cfg.DataDir); err != nil {
		return fmt.Errorf("failed to create data directories: %w", err)
	}
	wsPath := filepath.Join(xdgHome, "workspaces.yaml")
	if err := createWorkspacesFile(wsPath, cfg.WorkspacePath, cfg.PMPlatform); err != nil {
		fmt.Printf("  Warning: could not create workspaces.yaml: %v\n", err)
	} else {
		fmt.Printf("  ✓ %s\n", wsPath)
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
	installShellIntegration()

	// ── 11. Autostart ─────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("─── Autostart ────────────────────────────────────────────────────")
	fmt.Println("Autostart keeps DevTrack running automatically after login.")
	fmt.Print("Set up autostart now? [Y/n]: ")
	autostartAnswer := readLine(reader)
	if autostartAnswer == "" || strings.ToLower(autostartAnswer) == "y" {
		printAutostartInstructions()
	}

	// ── Done ──────────────────────────────────────────────────────────────────
	// Record all current migrations as applied — setup already did everything they do.
	MarkAllMigrationsApplied()

	printSetupComplete(projectRoot, cfg.Mode)
	return nil
}

// detectProjectRoot finds the DevTrack server installation root (devtrack_server/).
// Check order:
//  1. PROJECT_ROOT env var (explicit override, set in .env)
//  2. DEVTRACK_SERVER_DIR env var (user-specified alternate location)
//  3. Standard managed-install path: $XDG_DATA_HOME/devtrack/server/devtrack_server/
//  4. Walk up from binary looking for backend/ (developer mode)
//  5. Current working directory backend/ (developer mode)
func detectProjectRoot() (string, error) {
	// 1. Explicit PROJECT_ROOT env var (set by previously generated .env)
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		return root, nil
	}

	// 2. Explicit DEVTRACK_SERVER_DIR override
	if serverDir := os.Getenv("DEVTRACK_SERVER_DIR"); serverDir != "" {
		if _, err := os.Stat(filepath.Join(serverDir, "backend")); err == nil {
			return serverDir, nil
		}
		return "", fmt.Errorf("DEVTRACK_SERVER_DIR=%q does not contain a backend/ directory", serverDir)
	}

	// 3. Standard managed-install location: devtrackDataHome/server/devtrack_server/
	if xdgHome, err := devtrackDataHome(); err == nil {
		standardPath := filepath.Join(xdgHome, "server", "devtrack_server")
		if _, err := os.Stat(filepath.Join(standardPath, "backend")); err == nil {
			return standardPath, nil
		}
	}

	// 4. Walk up from binary (developer mode — binary next to devtrack_server/backend/)
	execPath, err := os.Executable()
	if err == nil {
		execPath, _ = filepath.Abs(execPath)
		searchDir := filepath.Dir(execPath)
		for range 6 {
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

	// 5. Current working directory (developer mode)
	cwd, err := os.Getwd()
	if err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "backend")); err == nil {
			return cwd, nil
		}
	}

	return "", fmt.Errorf("Python server not found. Run 'devtrack setup' to install it automatically, " +
		"or set PROJECT_ROOT / DEVTRACK_SERVER_DIR to point at an existing devtrack_server/ directory")
}

// cloneAndInstallServer sparse-clones only devtrack_server/ from the monorepo
// into <devtrackHome>/server/ and then runs `uv sync` to install Python deps.
// It streams all git and uv output to stdout so the user sees progress.
// Returns the absolute path to the installed devtrack_server/ directory.
func cloneAndInstallServer(devtrackHome string) (string, error) {
	targetDir := filepath.Join(devtrackHome, "server")

	fmt.Printf("\nCloning Python server into %s ...\n\n", targetDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("create server dir %s: %w", targetDir, err)
	}

	// runGit runs a git command, streaming its output to stdout/stderr.
	runGit := func(args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return nil
	}

	// Sparse-checkout: only devtrack_server/ (~5 MB vs full repo).
	if err := runGit("init", targetDir); err != nil {
		return "", err
	}
	if err := runGit("-C", targetDir, "remote", "add", "origin", devtrackServerRepoURL); err != nil {
		return "", err
	}
	if err := runGit("-C", targetDir, "sparse-checkout", "init", "--cone"); err != nil {
		return "", err
	}
	if err := runGit("-C", targetDir, "sparse-checkout", "set", "devtrack_server"); err != nil {
		return "", err
	}
	if err := runGit("-C", targetDir, "fetch", "--depth", "1", "origin", devtrackServerBranch); err != nil {
		return "", err
	}
	if err := runGit("-C", targetDir, "checkout", devtrackServerBranch); err != nil {
		return "", err
	}

	serverPath := filepath.Join(targetDir, "devtrack_server")

	// Install Python dependencies.
	fmt.Println("\nRunning uv sync...")
	syncCmd := exec.Command("uv", "sync")
	syncCmd.Dir = serverPath
	syncCmd.Stdout = os.Stdout
	syncCmd.Stderr = os.Stderr
	if err := syncCmd.Run(); err != nil {
		return "", fmt.Errorf("uv sync failed in %s: %w", serverPath, err)
	}

	fmt.Printf("\nPython server installed at: %s\n\n", serverPath)
	return serverPath, nil
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
	// DEVTRACK_HOME is the XDG data home ($XDG_DATA_HOME/devtrack), not a path
	// derived from PROJECT_ROOT — in managed mode PROJECT_ROOT points at the
	// cloned devtrack_server/ directory, which has no devtrack_client/ sibling.
	b.WriteString("DEVTRACK_HOME=" + filepath.Dir(dataDir) + "\n")
	b.WriteString("DEVTRACK_WORKSPACE=" + cfg.WorkspacePath + "\n")
	b.WriteString("WORKSPACES_FILE=" + filepath.Join(filepath.Dir(dataDir), "workspaces.yaml") + "\n")
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
	b.WriteString("DEVTRACK_SERVER_HTTP_PORT=35894\n\n")

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
	b.WriteString("HTTP_TIMEOUT_SHORT=10\n")      // Python server reads this name
	b.WriteString("HTTP_TIMEOUT_SHORT_SECS=10\n") // Go client reads this name
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
	b.WriteString("# Secret + alert user only — owner/repo/api_url go in workspaces.yaml\n")
	b.WriteString("GITHUB_TOKEN=" + cfg.GitHubToken + "\n")
	b.WriteString("GITHUB_USER=" + cfg.GitHubOwner + "\n\n") // Go alert poller user ID (falls back to EMAIL)

	b.WriteString("## AZURE DEVOPS\n")
	b.WriteString("# Secrets + identity — org/project/api_url go in workspaces.yaml\n")
	b.WriteString("AZURE_DEVOPS_PAT=" + cfg.AzurePAT + "\n")
	b.WriteString("AZURE_API_KEY=\n")                                     // alias for AZURE_DEVOPS_PAT accepted by Python server + cli_info
	b.WriteString("AZURE_ORGANIZATION=" + cfg.AzureOrganization + "\n")  // Go health check + devtrack settings + Python server
	b.WriteString("AZURE_PROJECT=" + cfg.AzureProject + "\n")            // Go health check + devtrack settings + Python server
	b.WriteString("AZURE_EMAIL=" + cfg.UserEmail + "\n")                  // alert poller: skip own comments
	b.WriteString("AZURE_SYNC_ENABLED=false\n")
	b.WriteString("AZURE_SYNC_AUTO_COMMENT=true\n")
	b.WriteString("AZURE_SYNC_AUTO_TRANSITION=false\n")
	b.WriteString("AZURE_SYNC_CREATE_ON_NO_MATCH=false\n")
	b.WriteString("AZURE_SYNC_MATCH_THRESHOLD=0.7\n")
	b.WriteString("AZURE_SYNC_WINDOW_HOURS=0\n")
	b.WriteString("AZURE_SYNC_STATES=New,Active,In Progress\n")
	b.WriteString("AZURE_SYNC_WORK_ITEM_TYPE=Task\n")
	b.WriteString("AZURE_POLL_INTERVAL_MINS=5\n\n")

	b.WriteString("## GITLAB\n")
	b.WriteString("# Secret only — all other config goes in workspaces.yaml\n")
	b.WriteString("GITLAB_PAT=\n")
	b.WriteString("GITLAB_API_KEY=\n") // alias for GITLAB_PAT accepted by Python server
	b.WriteString("GITLAB_URL=https://gitlab.com\n")
	b.WriteString("GITLAB_PROJECT_ID=\n")
	b.WriteString("GITLAB_SYNC_ENABLED=false\n")
	b.WriteString("GITLAB_AUTO_COMMENT=true\n")
	b.WriteString("GITLAB_AUTO_TRANSITION=false\n")
	b.WriteString("GITLAB_CREATE_ON_NO_MATCH=false\n")
	b.WriteString("GITLAB_MATCH_THRESHOLD=0.6\n")
	b.WriteString("GITLAB_DONE_STATE=closed\n")
	b.WriteString("GITLAB_SYNC_LABEL=devtrack\n")
	b.WriteString("GITLAB_AUTO_UPDATE_DESCRIPTION=false\n")
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
	b.WriteString("ADMIN_SECRET_KEY=" + generateSecret(32) + "\n")
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
	b.WriteString("TELEGRAM_CHAT_ID=\n")        // Go native notifier: chat to send to
	b.WriteString("TELEGRAM_NOTIFY_HEALTH=true\n\n")

	b.WriteString("## SLACK\n")
	b.WriteString("SLACK_WEBHOOK_URL=\n")        // Go native notifier: incoming webhook URL
	b.WriteString("SLACK_ENABLED=false\n")       // Python bot
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

// checkCommonPrereqs verifies prerequisites required by all modes.
func checkCommonPrereqs() {
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Println("  ✗ git not found — required for repository monitoring")
		fmt.Println("    Install git: https://git-scm.com/downloads")
	} else {
		fmt.Println("  ✓ git found")
	}
}

// generateSecret returns a cryptographically random hex string of n bytes.
func generateSecret(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback: timestamp-based (not cryptographic, but better than empty)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// checkPythonBackend verifies the Python backend and uv are present, then runs uv sync.
// Returns an error if backend/ is missing, uv is not in PATH, or uv sync fails.
func checkPythonBackend(projectRoot string) error {
	// Check backend directory
	backendDir := filepath.Join(projectRoot, "backend")
	if _, err := os.Stat(backendDir); err != nil {
		fmt.Println("  ✗ backend/ directory not found at " + backendDir)
		fmt.Println("    Run 'devtrack setup' to clone the Python server automatically.")
		return fmt.Errorf("backend/ directory not found at %s", backendDir)
	}
	fmt.Println("  ✓ backend/ directory found")

	// Check uv
	if _, err := exec.LookPath("uv"); err != nil {
		fmt.Println("  ✗ uv not found — Python dependency manager required")
		fmt.Println("    Install uv: https://docs.astral.sh/uv/getting-started/installation/")
		fmt.Println("    Or: curl -LsSf https://astral.sh/uv/install.sh | sh")
		return fmt.Errorf("uv not found in PATH — install from https://docs.astral.sh/uv/getting-started/installation/")
	}
	fmt.Println("  ✓ uv found")

	// Check Python (non-fatal — uv can manage its own Python interpreter)
	pythonBin := "python3"
	if runtime.GOOS == "windows" {
		pythonBin = "python"
	}
	if _, err := exec.LookPath(pythonBin); err != nil {
		fmt.Printf("  ~ %s not in PATH (uv will use its managed interpreter)\n", pythonBin)
	} else {
		fmt.Printf("  ✓ %s found\n", pythonBin)
	}

	// Run uv sync to install/update Python dependencies
	fmt.Println()
	fmt.Println("  Installing Python dependencies (uv sync)...")
	syncCmd := exec.Command("uv", "sync")
	syncCmd.Dir = projectRoot
	syncCmd.Stdout = os.Stdout
	syncCmd.Stderr = os.Stderr
	if err := syncCmd.Run(); err != nil {
		return fmt.Errorf("uv sync failed: %w", err)
	}
	fmt.Println("  ✓ Python dependencies installed")

	fmt.Println()
	return nil
}

// installShellIntegration appends the devtrack eval line to the active shell RC file.
// It is idempotent: a second run will not add a duplicate line.
// On Windows it targets the PowerShell profile; on Unix it targets bash/zsh/fish.
func installShellIntegration() {
	if runtime.GOOS == "windows" {
		installShellIntegrationWindows()
	} else {
		installShellIntegrationUnix()
	}
}

// installShellIntegrationWindows writes the PowerShell shim to $PROFILE.
func installShellIntegrationWindows() {
	profilePath := resolvePowerShellProfile()
	if profilePath == "" {
		fmt.Println("  ✗ Could not locate PowerShell profile; add integration manually.")
		fmt.Println("    Add this line to your $PROFILE:")
		fmt.Println("      devtrack shell-init --powershell | Out-String | Invoke-Expression")
		return
	}

	evalLine := `devtrack shell-init --powershell | Out-String | Invoke-Expression`

	if data, err := os.ReadFile(profilePath); err == nil {
		if strings.Contains(string(data), "devtrack shell-init") {
			fmt.Printf("  ✓ Shell integration already present in %s\n", profilePath)
			return
		}
	}

	// Ensure the profile's parent directory exists (profile may not exist yet).
	if err := os.MkdirAll(filepath.Dir(profilePath), 0755); err != nil {
		fmt.Printf("  ✗ Could not create profile directory: %v\n", err)
		return
	}

	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("  ✗ Could not write to %s: %v\n", profilePath, err)
		fmt.Printf("    Add manually: %s\n", evalLine)
		return
	}
	defer f.Close()

	if _, err := f.WriteString("\n# DevTrack shell integration\n" + evalLine + "\n"); err != nil {
		fmt.Printf("  ✗ Write error on %s: %v\n", profilePath, err)
		return
	}

	fmt.Printf("  ✓ Added to %s\n", profilePath)
	fmt.Println("    Reload with:  . $PROFILE   (or open a new terminal)")
}

// resolvePowerShellProfile returns the path to the current user's PowerShell
// profile by asking pwsh (7+) or powershell.exe (5.x). Falls back to the
// conventional Documents\PowerShell path when neither is available.
func resolvePowerShellProfile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Try pwsh (PowerShell 7+) first, then legacy powershell.exe.
	for _, ps := range []string{"pwsh", "powershell"} {
		out, err := exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", "echo $PROFILE").Output()
		if err == nil {
			p := strings.TrimSpace(string(out))
			if p != "" {
				return p
			}
		}
	}

	// Conventional fallback: PowerShell 7 profile location.
	return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
}

// installShellIntegrationUnix appends the eval line to ~/.zshrc, ~/.bashrc, or fish config.
func installShellIntegrationUnix() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("  ✗ Could not determine home directory; add shell integration manually.")
		fmt.Println(`    eval "$(devtrack shell-init)"`)
		return
	}

	shell := os.Getenv("SHELL")
	var rcFile string
	switch {
	case strings.Contains(shell, "zsh"):
		rcFile = filepath.Join(home, ".zshrc")
	case strings.Contains(shell, "fish"):
		rcFile = filepath.Join(home, ".config", "fish", "config.fish")
	default:
		rcFile = filepath.Join(home, ".bashrc")
	}

	evalLine := `eval "$(devtrack shell-init)"`

	if data, err := os.ReadFile(rcFile); err == nil {
		if strings.Contains(string(data), "devtrack shell-init") {
			fmt.Printf("  ✓ Shell integration already present in %s\n", rcFile)
			return
		}
	}

	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("  ✗ Could not write to %s: %v\n", rcFile, err)
		fmt.Printf("    Add manually: %s\n", evalLine)
		return
	}
	defer f.Close()

	if _, err := f.WriteString("\n# DevTrack shell integration\n" + evalLine + "\n"); err != nil {
		fmt.Printf("  ✗ Write error on %s: %v\n", rcFile, err)
		return
	}

	fmt.Printf("  ✓ Added to %s\n", rcFile)
	fmt.Printf("    Run: source %s  (or open a new terminal)\n", rcFile)
}

// createWorkspacesFile writes workspaces.yaml with the workspace collected
// during setup. Always overwrites — setup is an authoritative write of the
// initial config, so a stale file with path:"." must be replaced.
func createWorkspacesFile(path, workspacePath, pmPlatform string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}
	if pmPlatform == "" || pmPlatform == "none" {
		pmPlatform = "none"
	}
	// Derive a short name from the last path component.
	name := filepath.Base(workspacePath)
	if name == "" || name == "." {
		name = "default"
	}
	content := "# workspaces.yaml — managed by DevTrack\n" +
		"# Add more workspaces with: devtrack workspace add <name> <path> [platform]\n" +
		"# pm_platform options: azure | github | gitlab | jira | none\n\n" +
		"version: \"1\"\nworkspaces:\n" +
		"  - name: \"" + name + "\"\n" +
		"    path: \"" + filepath.ToSlash(workspacePath) + "\"\n" +
		"    pm_platform: \"" + pmPlatform + "\"\n" +
		"    pm_project: \"\"\n" +
		"    enabled: true\n" +
		"    ignore_branches: []\n" +
		"    tags: []\n" +
		"    pm_assignee: \"\"\n" +
		"    pm_iteration_path: \"\"\n" +
		"    pm_area_path: \"\"\n" +
		"    pm_milestone: 0\n"
	return os.WriteFile(path, []byte(content), 0644)
}

// printAutostartInstructions shows the autostart command.
func printAutostartInstructions() {
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
	fmt.Println("This wizard creates ~/.local/share/devtrack/ with all required directories")
	fmt.Println("and registers DevTrack in your shell profile.")
	fmt.Println("You can re-run 'devtrack setup' at any time to reconfigure.")
	fmt.Println()
}

// printSetupComplete prints the completion summary.
func printSetupComplete(projectRoot string, mode DevTrackMode) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Setup complete!                                                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	if mode == ModeManaged {
		fmt.Println("Next steps:")
		fmt.Println()
		fmt.Println("  1. Start DevTrack (env auto-loaded — no sourcing needed):")
		fmt.Println("       devtrack start")
		fmt.Println()
		fmt.Println("  2. Check status:  devtrack status")
		fmt.Println("  3. View logs:     devtrack logs")
		fmt.Println("  4. Add workspace: devtrack workspace add <path>")
		fmt.Println()
		fmt.Println("Edit .env at any time to add integrations (GitHub, Azure, Jira, etc.)")
		fmt.Printf("Python server installed at: %s\n", projectRoot)
	} else {
		fmt.Println("Next steps:")
		fmt.Println("  1. Set DEVTRACK_SERVER_URL in .env to point at your Python server.")
		fmt.Println("     Or run: devtrack cloud login --url https://<host> --key <key>")
		fmt.Println("  2. Start DevTrack:  devtrack start")
		fmt.Println("  3. Check status:    devtrack status")
		fmt.Println()
		fmt.Println("Note: AI features require the Python server to be reachable at DEVTRACK_SERVER_URL.")
		fmt.Println("Without a server URL, git monitoring and scheduling still run normally.")
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
