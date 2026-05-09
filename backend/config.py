"""
Centralized configuration for DevTrack backend.

Reads entirely from the process environment. No file loading.
Inject secrets via whatever mechanism you use (op run, direnv, launchd
EnvironmentVariables, Docker env, etc.) before starting the process.

All backend modules should use backend.config.get() instead of os.getenv() directly.
"""

import os
from pathlib import Path
from typing import Optional


def _find_project_root() -> Path:
    """Find project root without loading any .env file.

    Priority:
    1. PROJECT_ROOT env var
    2. Walk up from this file looking for .git
    3. Parent of backend/
    """
    proot = os.getenv("PROJECT_ROOT")
    if proot:
        p = Path(proot).expanduser()
        if p.exists():
            return p

    current = Path(__file__).resolve().parent  # backend/
    for _ in range(6):
        if (current / ".git").exists():
            return current
        parent = current.parent
        if parent == current:
            break
        current = parent

    return Path(__file__).resolve().parent.parent


def get(key: str, default: Optional[str] = None) -> str:
    """Get a configuration value from the process environment."""
    return os.getenv(key, default or "")


def get_int(key: str, default: int = 0) -> int:
    """Get config value as integer."""
    val = get(key)
    if not val:
        return default
    try:
        return int(val.strip())
    except ValueError:
        return default


def get_bool(key: str, default: bool = False) -> bool:
    """Get config value as boolean."""
    val = get(key, "").lower().strip()
    if not val:
        return default
    return val in ("true", "1", "yes", "on")


def get_path(key: str, default: Optional[str] = None) -> Path:
    """Get config value as expanded path."""
    val = get(key, default)
    if not val:
        return Path(default or ".").expanduser()
    return Path(val).expanduser()


# --- Project paths ---
def project_root() -> Path:
    """Project root directory."""
    return _find_project_root()


def database_path() -> Path:
    """Full path to SQLite database. From .env: DATABASE_DIR, DATA_DIR, or PROJECT_ROOT/Data/db."""
    db_dir = get("DATABASE_DIR")
    db_file = get("DATABASE_FILE_NAME") or "devtrack.db"
    if db_dir:
        return get_path("DATABASE_DIR") / db_file
    data_dir = get("DATA_DIR")
    if data_dir:
        return get_path("DATA_DIR") / "db" / db_file
    proot = get("PROJECT_ROOT")
    if proot:
        return get_path("PROJECT_ROOT") / "Data" / "db" / db_file
    return project_root() / "Data" / "db" / db_file


def reports_dir() -> Path:
    """Directory for saved reports. From .env: DATA_DIR or PROJECT_ROOT/Data."""
    data_dir = get("DATA_DIR")
    if data_dir:
        return get_path("DATA_DIR") / "reports"
    proot = get("PROJECT_ROOT")
    if proot:
        return get_path("PROJECT_ROOT") / "Data" / "reports"
    return project_root() / "Data" / "reports"


def learning_dir() -> Path:
    """Directory for learning data. From .env: LEARNING_DIR_PATH or PROJECT_ROOT/Data/learning."""
    learning_path = get("LEARNING_DIR_PATH")
    if learning_path:
        return get_path("LEARNING_DIR_PATH")
    proot = get("PROJECT_ROOT")
    if proot:
        return get_path("PROJECT_ROOT") / "Data" / "learning"
    return project_root() / "Data" / "learning"


def rag_chroma_dir() -> Path:
    """ChromaDB persistence directory for RAG personalization.
    From .env: PERSONALIZATION_CHROMA_DIR or DATA_DIR/learning/chroma."""
    custom = get("PERSONALIZATION_CHROMA_DIR")
    if custom:
        return get_path("PERSONALIZATION_CHROMA_DIR")
    return learning_dir() / "chroma"


def rag_embed_model() -> str:
    """Ollama model used for RAG embeddings. From .env: PERSONALIZATION_EMBED_MODEL."""
    return get("PERSONALIZATION_EMBED_MODEL", "nomic-embed-text") or "nomic-embed-text"


def rag_k() -> int:
    """Number of few-shot examples to retrieve per RAG query.
    From .env: PERSONALIZATION_RAG_K (default 3)."""
    return get_int("PERSONALIZATION_RAG_K", 3) or 3


def rag_enabled() -> bool:
    """Enable RAG personalization. From .env: PERSONALIZATION_RAG_ENABLED (default true)."""
    return get_bool("PERSONALIZATION_RAG_ENABLED", True)


def log_dir() -> Path:
    """Directory for log files. From .env: LOG_DIR or PROJECT_ROOT/Data/logs."""
    log_path = get("LOG_DIR")
    if log_path:
        return get_path("LOG_DIR")
    proot = get("PROJECT_ROOT")
    if proot:
        return get_path("PROJECT_ROOT") / "Data" / "logs"
    return project_root() / "Data" / "logs"


# --- Ollama ---
def ollama_host() -> str:
    """Ollama API host URL."""
    return get("OLLAMA_HOST", "http://localhost:11434")


def ollama_model() -> str:
    """Default Ollama model for AI tasks."""
    return get("OLLAMA_MODEL", "llama3.2")


# --- IPC ---
def ipc_host() -> str:
    """IPC server host."""
    return get("IPC_HOST", "127.0.0.1")


def ipc_port() -> str:
    """IPC server port."""
    return get("IPC_PORT", "35893")


# --- Azure DevOps (supports both AZURE_DEVOPS_PAT and AZURE_API_KEY for compatibility) ---
def azure_pat() -> str:
    """Azure DevOps Personal Access Token."""
    return get("AZURE_DEVOPS_PAT") or get("AZURE_API_KEY", "")


def azure_org() -> str:
    """Azure DevOps organization. Reads AZURE_ORGANIZATION, falls back to ORGANIZATION."""
    return get("AZURE_ORGANIZATION") or get("ORGANIZATION", "")


def azure_project() -> str:
    """Azure DevOps project. Reads AZURE_PROJECT, falls back to PROJECT."""
    return get("AZURE_PROJECT") or get("PROJECT", "")


# --- Task generation defaults (from env, no hardcoded personal data) ---
def azure_parent_work_item_id() -> str:
    """Parent work item ID for task hierarchy."""
    return get("AZURE_PARENT_WORK_ITEM_ID", "")


def azure_starting_work_item_id() -> int:
    """Starting work item ID for new tasks."""
    return get_int("AZURE_STARTING_WORK_ITEM_ID", 0)


# --- Sentiment / Chat analysis ---
def sentiment_target_sender() -> str:
    """Target sender name for responsiveness analysis."""
    return get("SENTIMENT_TARGET_SENDER", "")


# --- GitHub ---
def github_token() -> str:
    """GitHub Personal Access Token."""
    return get("GITHUB_TOKEN", "")


def timezone() -> str:
    """Default timezone for date handling."""
    return get("TIMEZONE", "Asia/Kolkata")


def log_file_path() -> Path:
    """Path for GitHub analysis log file."""
    return log_dir() / "github_branch_analysis.log"


# --- Semantic model ---
def semantic_model_name() -> str:
    """Sentence transformer model for task matching."""
    return get("SEMANTIC_MODEL_NAME", "all-MiniLM-L6-v2")


# --- Excel / Azure updator ---
def azure_excel_file() -> Path:
    """Path to Excel file for Azure task import."""
    excel = get("AZURE_EXCEL_FILE")
    if excel:
        return get_path("AZURE_EXCEL_FILE")
    return project_root() / "backend" / "data" / "tasks.xlsx"


def azure_excel_sheet() -> str:
    """Sheet name in Excel file."""
    return get("AZURE_EXCEL_SHEET", "my_tasks")


# --- Multi-Provider LLM ---
def llm_provider() -> str:
    """Primary LLM provider: 'ollama' | 'openai' | 'anthropic'. Default: ollama."""
    return get("LLM_PROVIDER", "ollama").lower().strip()


def openai_api_key() -> str:
    """OpenAI API key. Optional - only needed if LLM_PROVIDER=openai or as fallback."""
    return get("OPENAI_API_KEY", "")


def openai_model() -> str:
    """OpenAI model name."""
    return get("OPENAI_MODEL", "gpt-4o-mini")


def anthropic_api_key() -> str:
    """Anthropic API key. Optional - only needed if LLM_PROVIDER=anthropic or as fallback."""
    return get("ANTHROPIC_API_KEY", "")


def anthropic_model() -> str:
    """Anthropic model name."""
    return get("ANTHROPIC_MODEL", "claude-haiku-4-5")


# --- Jira ---
def jira_url() -> str:
    """Jira base URL, e.g. https://yourorg.atlassian.net"""
    return get("JIRA_URL", "")


def jira_email() -> str:
    """Jira account email (used with API token for Basic auth)."""
    return get("JIRA_EMAIL", "")


def jira_api_token() -> str:
    """Jira API token."""
    return get("JIRA_API_TOKEN", "")


def jira_project_key() -> str:
    """Default Jira project key (e.g. PROJ)."""
    return get("JIRA_PROJECT_KEY", "")


# --- GitHub PR Analysis ---
def github_owner() -> str:
    """GitHub org/user name (for PR queries)."""
    return get("GITHUB_OWNER", "")


def github_repo() -> str:
    """Default GitHub repo name (for PR queries). Leave empty to query all repos."""
    return get("GITHUB_REPO", "")


# --- Timeouts & Delays (NO DEFAULTS - REQUIRED ENV VARS) ---

def ipc_connect_timeout_secs() -> int:
    """IPC connection timeout in seconds. REQUIRED: IPC_CONNECT_TIMEOUT_SECS."""
    val = get("IPC_CONNECT_TIMEOUT_SECS")
    if not val:
        raise ValueError("IPC_CONNECT_TIMEOUT_SECS environment variable required (e.g., 5)")
    try:
        secs = int(val)
        if secs <= 0:
            raise ValueError(f"IPC_CONNECT_TIMEOUT_SECS must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"IPC_CONNECT_TIMEOUT_SECS must be integer: {e}")


def http_timeout_short() -> int:
    """HTTP timeout for quick requests in seconds. REQUIRED: HTTP_TIMEOUT_SHORT."""
    val = get("HTTP_TIMEOUT_SHORT")
    if not val:
        raise ValueError("HTTP_TIMEOUT_SHORT environment variable required (e.g., 10)")
    try:
        secs = int(val)
        if secs <= 0:
            raise ValueError(f"HTTP_TIMEOUT_SHORT must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"HTTP_TIMEOUT_SHORT must be integer: {e}")


def http_timeout() -> int:
    """HTTP timeout for standard requests in seconds. REQUIRED: HTTP_TIMEOUT."""
    val = get("HTTP_TIMEOUT")
    if not val:
        raise ValueError("HTTP_TIMEOUT environment variable required (e.g., 30)")
    try:
        secs = int(val)
        if secs <= 0:
            raise ValueError(f"HTTP_TIMEOUT must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"HTTP_TIMEOUT must be integer: {e}")


def http_timeout_long() -> int:
    """HTTP timeout for long-running requests in seconds. REQUIRED: HTTP_TIMEOUT_LONG."""
    val = get("HTTP_TIMEOUT_LONG")
    if not val:
        raise ValueError("HTTP_TIMEOUT_LONG environment variable required (e.g., 60)")
    try:
        secs = int(val)
        if secs <= 0:
            raise ValueError(f"HTTP_TIMEOUT_LONG must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"HTTP_TIMEOUT_LONG must be integer: {e}")


def ipc_retry_delay_ms() -> int:
    """IPC retry delay in milliseconds. REQUIRED: IPC_RETRY_DELAY_MS."""
    val = get("IPC_RETRY_DELAY_MS")
    if not val:
        raise ValueError("IPC_RETRY_DELAY_MS environment variable required (e.g., 2000)")
    try:
        ms = int(val)
        if ms < 0:
            raise ValueError(f"IPC_RETRY_DELAY_MS must be >= 0, got {ms}")
        return ms
    except ValueError as e:
        raise ValueError(f"IPC_RETRY_DELAY_MS must be integer: {e}")


def llm_request_timeout() -> int:
    """LLM request timeout in seconds. REQUIRED: LLM_REQUEST_TIMEOUT_SECS."""
    val = get("LLM_REQUEST_TIMEOUT_SECS")
    if not val:
        raise ValueError("LLM_REQUEST_TIMEOUT_SECS environment variable required (e.g., 120)")
    try:
        secs = int(val)
        if secs <= 0:
            raise ValueError(f"LLM_REQUEST_TIMEOUT_SECS must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"LLM_REQUEST_TIMEOUT_SECS must be integer: {e}")


def sentiment_analysis_window_minutes() -> int:
    """Sentiment analysis time window in minutes. REQUIRED: SENTIMENT_ANALYSIS_WINDOW_MINUTES."""
    val = get("SENTIMENT_ANALYSIS_WINDOW_MINUTES")
    if not val:
        raise ValueError("SENTIMENT_ANALYSIS_WINDOW_MINUTES environment variable required (e.g., 120)")
    try:
        mins = int(val)
        if mins <= 0:
            raise ValueError(f"SENTIMENT_ANALYSIS_WINDOW_MINUTES must be > 0, got {mins}")
        return mins
    except ValueError as e:
        raise ValueError(f"SENTIMENT_ANALYSIS_WINDOW_MINUTES must be integer: {e}")


# --- LM Studio & Ollama Hosts (NO DEFAULTS) ---

def lmstudio_host() -> str:
    """LM Studio API base URL. REQUIRED: LMSTUDIO_HOST."""
    val = get("LMSTUDIO_HOST")
    if not val:
        raise ValueError("LMSTUDIO_HOST environment variable required (e.g., http://localhost:1234/v1)")
    return val


# Remove defaults from ollama_host - make it required
def _orig_ollama_host_with_default() -> str:
    """Original with default - kept for backward compat during migration."""
    return get("OLLAMA_HOST", "http://localhost:11434")


# --- Model Names (NO DEFAULTS) ---

def git_sage_default_model() -> str:
    """Git Sage default model name. REQUIRED: GIT_SAGE_DEFAULT_MODEL."""
    val = get("GIT_SAGE_DEFAULT_MODEL")
    if not val:
        raise ValueError("GIT_SAGE_DEFAULT_MODEL environment variable required (e.g., llama3)")
    return val


def prompt_timeout_simple() -> int:
    """Simple prompt timeout in seconds. REQUIRED: PROMPT_TIMEOUT_SIMPLE_SECS."""
    val = get("PROMPT_TIMEOUT_SIMPLE_SECS")
    if not val:
        raise ValueError("PROMPT_TIMEOUT_SIMPLE_SECS environment variable required (e.g., 30)")
    try:
        secs = int(val)
        if secs <= 0:
            raise ValueError(f"PROMPT_TIMEOUT_SIMPLE_SECS must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"PROMPT_TIMEOUT_SIMPLE_SECS must be integer: {e}")


def prompt_timeout_work() -> int:
    """Work update prompt timeout in seconds. REQUIRED: PROMPT_TIMEOUT_WORK_SECS."""
    val = get("PROMPT_TIMEOUT_WORK_SECS")
    if not val:
        raise ValueError("PROMPT_TIMEOUT_WORK_SECS environment variable required (e.g., 60)")
    try:
        secs = int(val)
        if secs <= 0:
            raise ValueError(f"PROMPT_TIMEOUT_WORK_SECS must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"PROMPT_TIMEOUT_WORK_SECS must be integer: {e}")


def prompt_timeout_task() -> int:
    """Task description prompt timeout in seconds. REQUIRED: PROMPT_TIMEOUT_TASK_SECS."""
    val = get("PROMPT_TIMEOUT_TASK_SECS")
    if not val:
        raise ValueError("PROMPT_TIMEOUT_TASK_SECS environment variable required (e.g., 120)")
    try:
        secs = int(val)
        if secs <= 0:
            raise ValueError(f"PROMPT_TIMEOUT_TASK_SECS must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"PROMPT_TIMEOUT_TASK_SECS must be integer: {e}")


# --- MongoDB ---

def mongodb_uri() -> str:
    """MongoDB connection URI. Optional - if set, learning data is stored in MongoDB.
    Leave empty to use file-based storage. MONGODB_URI."""
    return get("MONGODB_URI", "")


def mongodb_db_name() -> str:
    """MongoDB database name. MONGODB_DB_NAME (default: devtrack)."""
    return get("MONGODB_DB_NAME", "devtrack")


# --- Learning cron settings ---

def learning_cron_enabled() -> bool:
    """Whether automatic daily learning collection is enabled. LEARNING_CRON_ENABLED."""
    return get_bool("LEARNING_CRON_ENABLED", False)


def learning_cron_schedule() -> str:
    """Cron schedule for learning collection. LEARNING_CRON_SCHEDULE (default: '0 8 * * *')."""
    return get("LEARNING_CRON_SCHEDULE", "0 8 * * *")


def learning_history_days() -> int:
    """History window in days for initial collection. LEARNING_HISTORY_DAYS (default: 30)."""
    return get_int("LEARNING_HISTORY_DAYS", 30)


# --- LLM generation parameters ---

def commit_llm_temperature() -> float:
    """Temperature for commit message generation. Lower = more focused/deterministic.
    COMMIT_LLM_TEMPERATURE (default: 0.2)."""
    val = get("COMMIT_LLM_TEMPERATURE", "0.2")
    return float(val)


def commit_llm_max_tokens() -> int:
    """Max tokens for commit message generation. COMMIT_LLM_MAX_TOKENS (default: 400)."""
    return get_int("COMMIT_LLM_MAX_TOKENS", 400)


def report_llm_temperature() -> float:
    """Temperature for report generation. REPORT_LLM_TEMPERATURE (default: 0.3)."""
    val = get("REPORT_LLM_TEMPERATURE", "0.3")
    return float(val)


def report_llm_max_tokens() -> int:
    """Max tokens for report generation. REPORT_LLM_MAX_TOKENS (default: 600)."""
    return get_int("REPORT_LLM_MAX_TOKENS", 600)


def personalization_llm_temperature() -> float:
    """Temperature for personalized response generation. PERSONALIZATION_LLM_TEMPERATURE (default: 0.7)."""
    val = get("PERSONALIZATION_LLM_TEMPERATURE", "0.7")
    return float(val)


def personalization_llm_max_tokens() -> int:
    """Max tokens for personalized response generation. PERSONALIZATION_LLM_MAX_TOKENS (default: 300)."""
    return get_int("PERSONALIZATION_LLM_MAX_TOKENS", 300)


def description_llm_temperature() -> float:
    """Temperature for description enhancement. DESCRIPTION_LLM_TEMPERATURE (default: 0.3)."""
    val = get("DESCRIPTION_LLM_TEMPERATURE", "0.3")
    return float(val)


def description_llm_max_tokens() -> int:
    """Max tokens for description enhancement. DESCRIPTION_LLM_MAX_TOKENS (default: 300)."""
    return get_int("DESCRIPTION_LLM_MAX_TOKENS", 300)


# --- Groq ---

def groq_api_key() -> str:
    return get("GROQ_API_KEY", "")

def groq_host() -> str:
    return get("GROQ_HOST", "https://api.groq.com/openai/v1")

def groq_model() -> str:
    return get("GROQ_MODEL", "llama-3.3-70b-versatile")


# --- git-sage provider resolution ---

def git_sage_provider() -> str:
    return get("GIT_SAGE_PROVIDER", llm_provider()).lower().strip()

def git_sage_model() -> str:
    provider = git_sage_provider()
    if provider == "openai":
        return openai_model()
    if provider == "groq":
        return groq_model()
    if provider == "anthropic":
        return anthropic_model()
    return get("GIT_SAGE_DEFAULT_MODEL", "") or ollama_model()

def git_sage_api_key() -> str:
    explicit = get("GIT_SAGE_API_KEY", "")
    if explicit:
        return explicit
    provider = git_sage_provider()
    if provider == "openai":   return openai_api_key()
    if provider == "groq":     return groq_api_key()
    if provider == "anthropic": return anthropic_api_key()
    return ""

def git_sage_base_url() -> str:
    explicit = get("GIT_SAGE_BASE_URL", "")
    if explicit:
        return explicit
    provider = git_sage_provider()
    if provider == "groq":     return groq_host()
    if provider == "lmstudio": return lmstudio_host()
    if provider in ("openai", "anthropic"): return ""
    return ollama_host()


# --- Azure DevOps Sync ---

def is_azure_sync_enabled() -> bool:
    """Check if bidirectional Azure DevOps sync is enabled. AZURE_SYNC_ENABLED (default: false)."""
    return get_bool("AZURE_SYNC_ENABLED", False)


def is_azure_auto_comment() -> bool:
    """Auto-add comments on matched work items. AZURE_SYNC_AUTO_COMMENT (default: true)."""
    return get_bool("AZURE_SYNC_AUTO_COMMENT", True)


def is_azure_auto_transition() -> bool:
    """Auto-transition work item state on status change. AZURE_SYNC_AUTO_TRANSITION (default: false)."""
    return get_bool("AZURE_SYNC_AUTO_TRANSITION", False)


def is_azure_create_on_no_match() -> bool:
    """Create new work item when no match found. AZURE_SYNC_CREATE_ON_NO_MATCH (default: false)."""
    return get_bool("AZURE_SYNC_CREATE_ON_NO_MATCH", False)


# --- GitHub Ticket Sync (NO DEFAULTS — raise ConfigError on missing) ---

class ConfigError(ValueError):
    """Raised when a required config variable is absent or invalid."""

    def __init__(self, var_name: str, message: str) -> None:
        super().__init__(message)
        self.var_name = var_name
        self.message = message


def get_github_token() -> str:
    """GitHub PAT with repo + issues:read scope. REQUIRED: GITHUB_TOKEN."""
    val = get("GITHUB_TOKEN")
    if not val:
        raise ConfigError(
            "GITHUB_TOKEN",
            "GITHUB_TOKEN environment variable required (PAT with repo + issues:read scope)",
        )
    return val


def get_github_default_repo() -> str:
    """Default GitHub repo in owner/repo format. REQUIRED: GITHUB_DEFAULT_REPO."""
    val = get("GITHUB_DEFAULT_REPO")
    if not val:
        raise ConfigError(
            "GITHUB_DEFAULT_REPO",
            "GITHUB_DEFAULT_REPO environment variable required (e.g. acme/backend)",
        )
    return val


def get_github_assignee() -> str:
    """GitHub username to filter assigned issues. REQUIRED: GITHUB_ASSIGNEE."""
    val = get("GITHUB_ASSIGNEE")
    if not val:
        raise ConfigError(
            "GITHUB_ASSIGNEE",
            "GITHUB_ASSIGNEE environment variable required (your GitHub username)",
        )
    return val


def get_ticket_sync_max_age_hours() -> int:
    """Max age of cached tickets (hours) before re-sync. REQUIRED: TICKET_SYNC_MAX_AGE_HOURS."""
    val = get("TICKET_SYNC_MAX_AGE_HOURS")
    if not val:
        raise ConfigError(
            "TICKET_SYNC_MAX_AGE_HOURS",
            "TICKET_SYNC_MAX_AGE_HOURS environment variable required (e.g. 24)",
        )
    try:
        hours = int(val)
    except ValueError:
        raise ConfigError(
            "TICKET_SYNC_MAX_AGE_HOURS",
            f"TICKET_SYNC_MAX_AGE_HOURS must be an integer, got: {val!r}",
        )
    if hours <= 0:
        raise ConfigError(
            "TICKET_SYNC_MAX_AGE_HOURS",
            f"TICKET_SYNC_MAX_AGE_HOURS must be > 0, got {hours}",
        )
    return hours


def get_azure_match_threshold() -> float:
    """Minimum confidence for auto-matching tasks (0.0-1.0). AZURE_SYNC_MATCH_THRESHOLD (default: 0.7)."""
    val = get("AZURE_SYNC_MATCH_THRESHOLD", "0.7")
    try:
        threshold = float(val)
    except (ValueError, TypeError):
        raise ValueError(
            f"AZURE_SYNC_MATCH_THRESHOLD must be a float, got: {val!r}"
        )
    if not 0.0 <= threshold <= 1.0:
        raise ValueError(
            f"AZURE_SYNC_MATCH_THRESHOLD must be between 0.0 and 1.0, got {threshold}"
        )
    return threshold


def get_azure_sync_states() -> list:
    """Work item states to fetch for matching. AZURE_SYNC_STATES (default: New,Active,In Progress)."""
    val = get("AZURE_SYNC_STATES", "New,Active,In Progress")
    return [s.strip() for s in val.split(",") if s.strip()]


def get_azure_done_state() -> str:
    """Target state when transitioning done items. AZURE_SYNC_DONE_STATE (default: Done)."""
    return get("AZURE_SYNC_DONE_STATE", "Done")


def get_azure_api_version() -> str:
    """Azure DevOps API version. AZURE_API_VERSION (default: 7.1)."""
    return get("AZURE_API_VERSION", "7.1")


# --- Webhook Server ---

def is_webhook_enabled() -> bool:
    """Whether the webhook server is enabled. WEBHOOK_ENABLED (default: false)."""
    return get_bool("WEBHOOK_ENABLED", False)


def get_webhook_port() -> int:
    """Webhook server listen port. WEBHOOK_PORT (default: 8089)."""
    return get_int("WEBHOOK_PORT", 8089)


def get_webhook_host() -> str:
    """Webhook server listen host. WEBHOOK_HOST (default: 0.0.0.0)."""
    return get("WEBHOOK_HOST", "0.0.0.0")


def get_webhook_azure_username() -> str:
    """Basic-auth username for Azure DevOps webhooks. WEBHOOK_AZURE_USERNAME (default: devtrack)."""
    return get("WEBHOOK_AZURE_USERNAME", "devtrack")


def get_webhook_azure_password() -> str:
    """Basic-auth password for Azure DevOps webhooks. REQUIRED: WEBHOOK_AZURE_PASSWORD."""
    val = get("WEBHOOK_AZURE_PASSWORD")
    if not val:
        raise ValueError("WEBHOOK_AZURE_PASSWORD environment variable required")
    return val


def get_webhook_github_secret() -> str:
    """GitHub webhook HMAC secret (optional). WEBHOOK_GITHUB_SECRET."""
    return get("WEBHOOK_GITHUB_SECRET", "")


def get_webhook_gitlab_secret() -> str:
    """GitLab webhook token (optional). WEBHOOK_GITLAB_SECRET."""
    return get("WEBHOOK_GITLAB_SECRET", "")


def is_webhook_notify_os() -> bool:
    """Send OS notifications for webhook events. WEBHOOK_NOTIFY_OS (default: true)."""
    return get_bool("WEBHOOK_NOTIFY_OS", True)


def is_webhook_notify_terminal() -> bool:
    """Print webhook events to terminal. WEBHOOK_NOTIFY_TERMINAL (default: true)."""
    return get_bool("WEBHOOK_NOTIFY_TERMINAL", True)


# --- Project Sync ---

def is_project_sync_enabled() -> bool:
    """Whether periodic project sync is enabled. PROJECT_SYNC_ENABLED (default: false)."""
    return get_bool("PROJECT_SYNC_ENABLED", False)


def get_project_sync_interval() -> int:
    """Interval in seconds between project sync runs. PROJECT_SYNC_INTERVAL_SECS (default: 300)."""
    return get_int("PROJECT_SYNC_INTERVAL_SECS", 300)


def get_azure_sync_work_item_type() -> str:
    """Work item type to create/query in Azure. AZURE_SYNC_WORK_ITEM_TYPE (default: Task)."""
    return get("AZURE_SYNC_WORK_ITEM_TYPE", "Task")


def get_azure_sync_default_area_path() -> str:
    """Default area path for new work items. AZURE_SYNC_DEFAULT_AREA_PATH."""
    return get("AZURE_SYNC_DEFAULT_AREA_PATH", "")


def get_azure_sync_default_iteration_path() -> str:
    """Default iteration path for queries/new items. AZURE_SYNC_DEFAULT_ITERATION_PATH."""
    return get("AZURE_SYNC_DEFAULT_ITERATION_PATH", "")


def get_azure_sync_tag() -> str:
    """Tag applied to work items managed by DevTrack. AZURE_SYNC_TAG (default: devtrack-managed)."""
    return get("AZURE_SYNC_TAG", "devtrack-managed")


def get_azure_sync_state_setup() -> str:
    """Azure state for SETUP projects. AZURE_SYNC_STATE_SETUP (default: New)."""
    return get("AZURE_SYNC_STATE_SETUP", "New")


def get_azure_sync_state_active() -> str:
    """Azure state for ACTIVE projects. AZURE_SYNC_STATE_ACTIVE (default: Active)."""
    return get("AZURE_SYNC_STATE_ACTIVE", "Active")


def get_azure_sync_state_closed() -> str:
    """Azure state for CLOSED projects. AZURE_SYNC_STATE_CLOSED (default: Done)."""
    return get("AZURE_SYNC_STATE_CLOSED") or get_azure_done_state()


def get_azure_sync_goal_state_pending() -> str:
    """Azure state for pending goals. AZURE_SYNC_GOAL_STATE_PENDING (default: New)."""
    return get("AZURE_SYNC_GOAL_STATE_PENDING", "New")


def get_azure_sync_goal_state_in_progress() -> str:
    """Azure state for in-progress goals. AZURE_SYNC_GOAL_STATE_IN_PROGRESS (default: Active)."""
    return get("AZURE_SYNC_GOAL_STATE_IN_PROGRESS", "Active")


def get_azure_sync_goal_state_completed() -> str:
    """Azure state for completed goals. AZURE_SYNC_GOAL_STATE_COMPLETED (default: Done)."""
    return get("AZURE_SYNC_GOAL_STATE_COMPLETED") or get_azure_done_state()


# ── GitLab sync ───────────────────────────────────────────────────────────────

def is_gitlab_sync_enabled() -> bool:
    return get_bool("GITLAB_SYNC_ENABLED", default=False)

def is_gitlab_auto_comment() -> bool:
    return get_bool("GITLAB_AUTO_COMMENT", default=True)

def is_gitlab_auto_transition() -> bool:
    return get_bool("GITLAB_AUTO_TRANSITION", default=False)

def is_gitlab_create_on_no_match() -> bool:
    return get_bool("GITLAB_CREATE_ON_NO_MATCH", default=False)

def get_gitlab_match_threshold() -> float:
    val = get("GITLAB_MATCH_THRESHOLD")
    try:
        return float(val) if val else 0.6
    except ValueError:
        return 0.6

def get_gitlab_done_state() -> str:
    return get("GITLAB_DONE_STATE") or "closed"

def get_gitlab_sync_label() -> str:
    return get("GITLAB_SYNC_LABEL") or "devtrack"

def get_gitlab_default_project_id() -> Optional[int]:
    val = get("GITLAB_PROJECT_ID")
    try:
        return int(val) if val else None
    except ValueError:
        return None


def is_gitlab_auto_update_description() -> bool:
    """Append latest commit info to matched issue description. GITLAB_AUTO_UPDATE_DESCRIPTION (default: false)."""
    return get_bool("GITLAB_AUTO_UPDATE_DESCRIPTION", False)


# ── GitHub sync ───────────────────────────────────────────────────────────────

def is_github_sync_enabled() -> bool:
    """Whether bidirectional GitHub issue sync is enabled. GITHUB_SYNC_ENABLED (default: false)."""
    return get_bool("GITHUB_SYNC_ENABLED", False)


def is_github_auto_comment() -> bool:
    """Auto-add comments on matched GitHub issues. GITHUB_AUTO_COMMENT (default: true)."""
    return get_bool("GITHUB_AUTO_COMMENT", True)


def is_github_auto_transition() -> bool:
    """Auto-close/reopen GitHub issues on status change. GITHUB_AUTO_TRANSITION (default: false)."""
    return get_bool("GITHUB_AUTO_TRANSITION", False)


def is_github_create_on_no_match() -> bool:
    """Create a new GitHub issue when no match found. GITHUB_CREATE_ON_NO_MATCH (default: false)."""
    return get_bool("GITHUB_CREATE_ON_NO_MATCH", False)


def get_github_match_threshold() -> float:
    """Minimum confidence for auto-matching tasks to GitHub issues (0.0-1.0).
    GITHUB_MATCH_THRESHOLD (default: 0.6)."""
    val = get("GITHUB_MATCH_THRESHOLD")
    try:
        return float(val) if val else 0.6
    except ValueError:
        return 0.6


def get_github_done_state() -> str:
    """GitHub issue state used when marking tasks done. GITHUB_DONE_STATE (default: closed)."""
    return get("GITHUB_DONE_STATE") or "closed"


def get_github_sync_label() -> str:
    """Label applied to GitHub issues managed by DevTrack. GITHUB_SYNC_LABEL (default: devtrack)."""
    return get("GITHUB_SYNC_LABEL") or "devtrack"


def is_github_auto_update_description() -> bool:
    """Append latest commit info to matched issue description. GITHUB_AUTO_UPDATE_DESCRIPTION (default: false)."""
    return get_bool("GITHUB_AUTO_UPDATE_DESCRIPTION", False)


# ── Telegram bot ─────────────────────────────────────────────────────────────

def is_telegram_enabled() -> bool:
    """Whether the Telegram bot is enabled. TELEGRAM_ENABLED."""
    return get_bool("TELEGRAM_ENABLED", False)


def get_telegram_bot_token() -> str:
    """Telegram bot token from @BotFather. TELEGRAM_BOT_TOKEN (required if enabled)."""
    return get("TELEGRAM_BOT_TOKEN", "")


def get_telegram_allowed_chat_ids() -> list:
    """Comma-separated list of authorized Telegram chat IDs. TELEGRAM_ALLOWED_CHAT_IDS."""
    raw = get("TELEGRAM_ALLOWED_CHAT_IDS", "")
    if not raw:
        return []
    ids = []
    for part in raw.split(","):
        part = part.strip()
        if part:
            try:
                ids.append(int(part))
            except ValueError:
                pass
    return ids


def is_telegram_notify_commits() -> bool:
    """Send commit notifications to Telegram. TELEGRAM_NOTIFY_COMMITS."""
    return get_bool("TELEGRAM_NOTIFY_COMMITS", True)


def is_telegram_notify_triggers() -> bool:
    """Send timer trigger notifications to Telegram. TELEGRAM_NOTIFY_TRIGGERS."""
    return get_bool("TELEGRAM_NOTIFY_TRIGGERS", True)


def is_telegram_notify_health() -> bool:
    """Send health alert notifications to Telegram. TELEGRAM_NOTIFY_HEALTH."""
    return get_bool("TELEGRAM_NOTIFY_HEALTH", True)


# ── Ticket Alerter ────────────────────────────────────────────────────────────

def is_alert_enabled() -> bool:
    """Whether the ticket alerter polling loop is active. ALERT_ENABLED (default: true)."""
    return get_bool("ALERT_ENABLED", True)


def get_alert_poll_interval() -> int:
    """Seconds between alert poll cycles. ALERT_POLL_INTERVAL_SECS (default: 300)."""
    return get_int("ALERT_POLL_INTERVAL_SECS", 300)


def is_alert_github_enabled() -> bool:
    """Whether GitHub alert polling is enabled. ALERT_GITHUB_ENABLED (default: true)."""
    return get_bool("ALERT_GITHUB_ENABLED", True)


def is_alert_notify_assigned() -> bool:
    """Deliver notifications for assignment events. ALERT_NOTIFY_ASSIGNED (default: true)."""
    return get_bool("ALERT_NOTIFY_ASSIGNED", True)


def is_alert_notify_comments() -> bool:
    """Deliver notifications for comment events. ALERT_NOTIFY_COMMENTS (default: true)."""
    return get_bool("ALERT_NOTIFY_COMMENTS", True)


def is_alert_notify_status_changes() -> bool:
    """Deliver notifications for status-change events. ALERT_NOTIFY_STATUS_CHANGES (default: true)."""
    return get_bool("ALERT_NOTIFY_STATUS_CHANGES", True)


def is_alert_notify_review_requested() -> bool:
    """Deliver notifications for review-request events. ALERT_NOTIFY_REVIEW_REQUESTED (default: true)."""
    return get_bool("ALERT_NOTIFY_REVIEW_REQUESTED", True)


# --- Core paths ---

def get_data_dir() -> str:
    """Root data directory. DATA_DIR (default: "")."""
    return get("DATA_DIR", "")


def get_project_root() -> str:
    """Project root directory. PROJECT_ROOT (default: "")."""
    return get("PROJECT_ROOT", "")


def get_database_dir() -> str:
    """Database directory. DATABASE_DIR (default: "")."""
    return get("DATABASE_DIR", "")


# --- DevTrack server / cloud ---

def get_devtrack_api_url() -> str:
    """DevTrack cloud API URL. DEVTRACK_API_URL (default: "")."""
    return get("DEVTRACK_API_URL", "")


def get_devtrack_api_key() -> str:
    """DevTrack API key for /trigger/* requests. DEVTRACK_API_KEY (default: "")."""
    return get("DEVTRACK_API_KEY", "")


def get_devtrack_version() -> str:
    """DevTrack build version string. DEVTRACK_VERSION (default: "unknown")."""
    return get("DEVTRACK_VERSION", "unknown")


def get_devtrack_tls_enabled() -> bool:
    """Whether TLS is enabled on the Go<->Python channel. DEVTRACK_TLS (default: True)."""
    return get_bool("DEVTRACK_TLS", True)


def get_devtrack_tls_cert() -> str:
    """Path to TLS certificate file. DEVTRACK_TLS_CERT (default: "")."""
    return get("DEVTRACK_TLS_CERT", "")


def get_devtrack_tls_key() -> str:
    """Path to TLS private key file. DEVTRACK_TLS_KEY (default: "")."""
    return get("DEVTRACK_TLS_KEY", "")


def get_devtrack_auto_accept_terms() -> bool:
    """Auto-accept terms of service on install. DEVTRACK_AUTO_ACCEPT_TERMS (default: False)."""
    return get_bool("DEVTRACK_AUTO_ACCEPT_TERMS", False)


# --- Admin console ---

def get_admin_port() -> int:
    """Admin console listen port. ADMIN_PORT (default: 8090)."""
    return get_int("ADMIN_PORT", 8090)


def get_admin_host() -> str:
    """Admin console listen host. ADMIN_HOST (default: '0.0.0.0')."""
    return get("ADMIN_HOST", "0.0.0.0")


def get_admin_username() -> str:
    """Admin console username. ADMIN_USERNAME (default: 'admin')."""
    return get("ADMIN_USERNAME", "admin")


def get_admin_password() -> str:
    """Admin console password. ADMIN_PASSWORD (default: "")."""
    return get("ADMIN_PASSWORD", "")


def get_admin_embed() -> bool:
    """Mount admin UI on main webhook server. ADMIN_EMBED (default: false).

    When true, the admin FastAPI router is mounted directly on the main
    webhook_server app at /admin, removing the need for a separate process.
    """
    return get_bool("ADMIN_EMBED", False)


def get_admin_session_hours() -> int:
    """Admin console session lifetime in hours. REQUIRED: ADMIN_SESSION_HOURS."""
    val = get("ADMIN_SESSION_HOURS")
    if not val:
        raise ValueError("ADMIN_SESSION_HOURS environment variable required (e.g., 8)")
    try:
        hours = int(val)
        if hours <= 0:
            raise ValueError(f"ADMIN_SESSION_HOURS must be > 0, got {hours}")
        return hours
    except ValueError as e:
        raise ValueError(f"ADMIN_SESSION_HOURS must be a positive integer: {e}")


def get_scrypt_n() -> int:
    """Scrypt CPU/memory cost factor. REQUIRED: SCRYPT_N (must be a power of 2, >= 2)."""
    val = get("SCRYPT_N")
    if not val:
        raise ValueError("SCRYPT_N environment variable required (e.g., 16384)")
    try:
        n = int(val)
    except ValueError:
        raise ValueError(f"SCRYPT_N must be an integer, got: {val!r}")
    if n < 2 or (n & (n - 1)) != 0:
        raise ValueError(f"SCRYPT_N must be a power of 2 and >= 2, got {n}")
    return n


def get_scrypt_r() -> int:
    """Scrypt block size. REQUIRED: SCRYPT_R (must be > 0)."""
    val = get("SCRYPT_R")
    if not val:
        raise ValueError("SCRYPT_R environment variable required (e.g., 8)")
    try:
        r = int(val)
        if r <= 0:
            raise ValueError(f"SCRYPT_R must be > 0, got {r}")
        return r
    except ValueError as e:
        raise ValueError(f"SCRYPT_R must be a positive integer: {e}")


def get_scrypt_p() -> int:
    """Scrypt parallelisation factor. REQUIRED: SCRYPT_P (must be > 0)."""
    val = get("SCRYPT_P")
    if not val:
        raise ValueError("SCRYPT_P environment variable required (e.g., 1)")
    try:
        p = int(val)
        if p <= 0:
            raise ValueError(f"SCRYPT_P must be > 0, got {p}")
        return p
    except ValueError as e:
        raise ValueError(f"SCRYPT_P must be a positive integer: {e}")


def get_scrypt_dklen() -> int:
    """Scrypt derived key length in bytes. REQUIRED: SCRYPT_DKLEN (must be > 0)."""
    val = get("SCRYPT_DKLEN")
    if not val:
        raise ValueError("SCRYPT_DKLEN environment variable required (e.g., 32)")
    try:
        dklen = int(val)
        if dklen <= 0:
            raise ValueError(f"SCRYPT_DKLEN must be > 0, got {dklen}")
        return dklen
    except ValueError as e:
        raise ValueError(f"SCRYPT_DKLEN must be a positive integer: {e}")


def get_shutdown_grace_period_seconds() -> float:
    """Seconds to wait before sending SIGTERM on shutdown. REQUIRED: SHUTDOWN_GRACE_PERIOD_SECONDS."""
    val = get("SHUTDOWN_GRACE_PERIOD_SECONDS")
    if not val:
        raise ValueError(
            "SHUTDOWN_GRACE_PERIOD_SECONDS environment variable required (e.g., 0.5)"
        )
    try:
        secs = float(val)
        if secs <= 0:
            raise ValueError(f"SHUTDOWN_GRACE_PERIOD_SECONDS must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"SHUTDOWN_GRACE_PERIOD_SECONDS must be a positive number: {e}")


def get_stats_refresh_interval_seconds() -> int:
    """How often the trigger stats panel polls in seconds. REQUIRED: STATS_REFRESH_INTERVAL_SECONDS."""
    val = get("STATS_REFRESH_INTERVAL_SECONDS")
    if not val:
        raise ValueError(
            "STATS_REFRESH_INTERVAL_SECONDS environment variable required (e.g., 30)"
        )
    try:
        secs = int(val)
        if secs <= 0:
            raise ValueError(f"STATS_REFRESH_INTERVAL_SECONDS must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"STATS_REFRESH_INTERVAL_SECONDS must be a positive integer: {e}")


def get_process_refresh_interval_seconds() -> int:
    """How often the process table polls in seconds. REQUIRED: PROCESS_REFRESH_INTERVAL_SECONDS."""
    val = get("PROCESS_REFRESH_INTERVAL_SECONDS")
    if not val:
        raise ValueError(
            "PROCESS_REFRESH_INTERVAL_SECONDS environment variable required (e.g., 15)"
        )
    try:
        secs = int(val)
        if secs <= 0:
            raise ValueError(f"PROCESS_REFRESH_INTERVAL_SECONDS must be > 0, got {secs}")
        return secs
    except ValueError as e:
        raise ValueError(f"PROCESS_REFRESH_INTERVAL_SECONDS must be a positive integer: {e}")


def get_audit_log_limit() -> int:
    """Maximum number of audit log entries returned by the admin UI. REQUIRED: AUDIT_LOG_LIMIT."""
    val = get("AUDIT_LOG_LIMIT")
    if not val:
        raise ValueError("AUDIT_LOG_LIMIT environment variable required (e.g., 200)")
    try:
        limit = int(val)
        if limit <= 0:
            raise ValueError(f"AUDIT_LOG_LIMIT must be > 0, got {limit}")
        return limit
    except ValueError as e:
        raise ValueError(f"AUDIT_LOG_LIMIT must be a positive integer: {e}")


def get_license_contact_email() -> str:
    """Support email shown on the license page. REQUIRED: LICENSE_CONTACT_EMAIL."""
    val = get("LICENSE_CONTACT_EMAIL")
    if not val:
        raise ValueError(
            "LICENSE_CONTACT_EMAIL environment variable required (e.g., license@devtrack.dev)"
        )
    return val


# --- GitHub (call-site use) ---

def get_github_token() -> str:
    """GitHub Personal Access Token. GITHUB_TOKEN (default: "")."""
    return get("GITHUB_TOKEN", "")


def get_github_owner() -> str:
    """GitHub org/user owner. GITHUB_OWNER (default: "")."""
    return get("GITHUB_OWNER", "")


def get_github_repo() -> str:
    """GitHub repository name. GITHUB_REPO (default: "")."""
    return get("GITHUB_REPO", "")


def get_github_api_url() -> str:
    """GitHub API base URL (empty = github.com). GITHUB_API_URL (default: "")."""
    return get("GITHUB_API_URL", "")


def get_github_api_version() -> str:
    """GitHub REST API version header. GITHUB_API_VERSION (default: '2022-11-28')."""
    return get("GITHUB_API_VERSION", "2022-11-28")


def get_github_sync_window_hours() -> int:
    """GitHub sync window in hours (0 = full resync). GITHUB_SYNC_WINDOW_HOURS (default: 0)."""
    return get_int("GITHUB_SYNC_WINDOW_HOURS", 0)


def get_github_create_on_no_match() -> bool:
    """Create GitHub issue when no match found. GITHUB_CREATE_ON_NO_MATCH (default: False)."""
    return get_bool("GITHUB_CREATE_ON_NO_MATCH", False)


def get_github_log_path() -> str:
    """Path for GitHub analysis log output. GITHUB_LOG_PATH (default: "")."""
    return get("GITHUB_LOG_PATH", "")


# --- Azure DevOps (call-site use, supplements existing) ---

def get_azure_organization() -> str:
    """Azure DevOps organization name. AZURE_ORGANIZATION (default: "")."""
    return get("AZURE_ORGANIZATION") or get("ORGANIZATION", "")


def get_azure_project_name() -> str:
    """Azure DevOps project name. AZURE_PROJECT (default: "")."""
    return get("AZURE_PROJECT") or get("PROJECT", "")


def get_azure_poll_interval_mins() -> int:
    """Azure assignment poll interval in minutes. AZURE_POLL_INTERVAL_MINS (default: 5)."""
    return get_int("AZURE_POLL_INTERVAL_MINS", 5)


def get_azure_sync_window_hours() -> int:
    """Azure sync window in hours (0 = full resync). AZURE_SYNC_WINDOW_HOURS (default: 0)."""
    return get_int("AZURE_SYNC_WINDOW_HOURS", 0)


def get_azure_client_id() -> str:
    """Azure AD application (client) ID. AZURE_CLIENT_ID (default: "")."""
    return get("AZURE_CLIENT_ID", "")


def get_azure_tenant_id() -> str:
    """Azure AD tenant ID. AZURE_TENANT_ID (default: "")."""
    return get("AZURE_TENANT_ID", "")


def get_azure_client_secret() -> str:
    """Azure AD client secret. AZURE_CLIENT_SECRET (default: "")."""
    return get("AZURE_CLIENT_SECRET", "")


# --- GitLab (call-site use, supplements existing) ---

def get_gitlab_url() -> str:
    """GitLab instance base URL. GITLAB_URL (default: 'https://gitlab.com')."""
    return get("GITLAB_URL", "https://gitlab.com")


def get_gitlab_pat() -> str:
    """GitLab Personal Access Token. GITLAB_PAT or GITLAB_API_KEY (default: "")."""
    return get("GITLAB_PAT") or get("GITLAB_API_KEY", "")


def get_gitlab_project_id_str() -> str:
    """GitLab project ID as string. GITLAB_PROJECT_ID (default: "")."""
    return get("GITLAB_PROJECT_ID", "")


def get_gitlab_poll_interval_mins() -> int:
    """GitLab assignment poll interval in minutes. GITLAB_POLL_INTERVAL_MINS (default: 5)."""
    return get_int("GITLAB_POLL_INTERVAL_MINS", 5)


def get_gitlab_sync_window_hours() -> int:
    """GitLab sync window in hours (0 = full resync). GITLAB_SYNC_WINDOW_HOURS (default: 0)."""
    return get_int("GITLAB_SYNC_WINDOW_HOURS", 0)


# --- Slack ---

def get_slack_enabled() -> bool:
    """Whether the Slack bot is enabled. SLACK_ENABLED (default: False)."""
    return get_bool("SLACK_ENABLED", False)


def get_slack_bot_token() -> str:
    """Slack bot OAuth token (xoxb-...). SLACK_BOT_TOKEN (default: "")."""
    return get("SLACK_BOT_TOKEN", "")


def get_slack_app_token() -> str:
    """Slack app-level token for Socket Mode (xapp-...). SLACK_APP_TOKEN (default: "")."""
    return get("SLACK_APP_TOKEN", "")


def get_slack_allowed_channel_ids() -> list:
    """Comma-separated Slack channel IDs to respond in. SLACK_ALLOWED_CHANNEL_IDS (default: [])."""
    raw = get("SLACK_ALLOWED_CHANNEL_IDS", "")
    if not raw:
        return []
    return [c.strip() for c in raw.split(",") if c.strip()]


# --- Telegram flags (supplements existing token/ids) ---

def get_telegram_notify_commits() -> bool:
    """Send commit notifications to Telegram. TELEGRAM_NOTIFY_COMMITS (default: False)."""
    return get_bool("TELEGRAM_NOTIFY_COMMITS", False)


def get_telegram_notify_triggers() -> bool:
    """Send timer trigger notifications to Telegram. TELEGRAM_NOTIFY_TRIGGERS (default: True)."""
    return get_bool("TELEGRAM_NOTIFY_TRIGGERS", True)


def get_telegram_notify_health() -> bool:
    """Send health alert notifications to Telegram. TELEGRAM_NOTIFY_HEALTH (default: True)."""
    return get_bool("TELEGRAM_NOTIFY_HEALTH", True)


# --- Email / reporting ---

def get_eod_report_email() -> str:
    """Default recipient for auto EOD reports. EOD_REPORT_EMAIL (default: "")."""
    return get("EOD_REPORT_EMAIL", "")


# --- Notifications ---

def get_notification_enabled() -> bool:
    """Whether desktop notifications are enabled. NOTIFICATION_ENABLED (default: true)."""
    return get_bool("NOTIFICATION_ENABLED", True)


# --- Misc ---

def get_commit_enhance_mode() -> bool:
    """Whether AI commit message enhancement is active. COMMIT_ENHANCE_MODE (default: False)."""
    return get_bool("COMMIT_ENHANCE_MODE", False)


def get_workspaces_file() -> str:
    """Path to workspaces YAML file. WORKSPACES_FILE (default: "")."""
    return get("WORKSPACES_FILE", "")


def get_spec_review_base_url() -> str:
    """Base URL for spec review web form. SPEC_REVIEW_BASE_URL (default: "")."""
    return get("SPEC_REVIEW_BASE_URL", "")


def get_mongodb_uri() -> str:
    """MongoDB connection URI. MONGODB_URI (default: "")."""
    return get("MONGODB_URI", "")


def get_mongodb_db() -> str:
    """MongoDB database name. MONGODB_DB (default: 'devtrack')."""
    return get("MONGODB_DB", "") or get("MONGODB_DB_NAME", "devtrack")


def get_notification_enabled() -> bool:
    """Whether cross-platform desktop notifications are enabled. NOTIFICATION_ENABLED (default: true)."""
    return get_bool("NOTIFICATION_ENABLED", default=True)
