"""
Centralized configuration for DevTrack backend.

Loads all configurable values from .env file at project root.
All backend modules should import from this module instead of using os.getenv directly.
"""

import os
from pathlib import Path
from typing import Optional

# Load .env from project root - must be called before any config access
_env_loaded = False


def _find_project_root() -> Path:
    """Find project root by looking for .env or .git in parent directories."""
    # Check DEVTRACK_ENV_FILE or PROJECT_ROOT first
    env_file = os.getenv("DEVTRACK_ENV_FILE")
    if env_file and Path(env_file).exists():
        return Path(env_file).parent

    project_root = os.getenv("PROJECT_ROOT")
    if project_root:
        p = Path(project_root).expanduser()
        if p.exists():
            return p

    # Walk up from this file's location
    current = Path(__file__).resolve().parent
    for _ in range(5):
        if (current / ".env").exists():
            return current
        if (current / ".env_sample").exists():
            return current
        if (current / ".git").exists():
            return current
        parent = current.parent
        if parent == current:
            break
        current = parent

    # Fallback: parent of backend/ is project root
    return Path(__file__).resolve().parent.parent


def _load_env() -> None:
    """Load .env from project root. Idempotent."""
    global _env_loaded
    if _env_loaded:
        return

    try:
        from dotenv import load_dotenv
    except ImportError:
        _env_loaded = True
        return

    project_root = _find_project_root()
    env_paths = [
        project_root / ".env",
        project_root / ".env_sample",
    ]

    for env_path in env_paths:
        if env_path.exists():
            load_dotenv(env_path)
            _env_loaded = True
            return

    _env_loaded = True


def get(key: str, default: Optional[str] = None) -> str:
    """Get config value. Loads .env on first access."""
    _load_env()
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
    """Azure DevOps organization."""
    return get("ORGANIZATION", "")


def azure_project() -> str:
    """Azure DevOps project."""
    return get("PROJECT", "")


# --- Task generation defaults (from env, no hardcoded personal data) ---
def azure_default_assignee() -> str:
    """Default assignee for generated tasks."""
    return get("AZURE_DEFAULT_ASSIGNEE", "")


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
