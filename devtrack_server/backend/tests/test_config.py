"""
Tests for backend.config module.
"""
import pytest


def test_config_get_returns_empty_for_missing():
    """get() returns empty string when key is absent and no default given."""
    from backend.config import get
    result = get("_DEVTRACK_TEST_NONEXISTENT_KEY_XYZ_")
    assert result == ""


def test_database_path_returns_path():
    """Test database_path returns a Path-like object."""
    from backend.config import database_path
    path = database_path()
    assert path is not None
    assert str(path).endswith(".db") or "devtrack" in str(path).lower() or "daemon" in str(path).lower()


def test_ollama_host_returns_string():
    """Test ollama_host returns a non-empty string."""
    from backend.config import ollama_host
    host = ollama_host()
    assert isinstance(host, str)
    assert len(host) > 0
    assert "localhost" in host or "127.0.0.1" in host or "http" in host


def test_ollama_model_returns_string():
    """Test ollama_model returns a non-empty string."""
    from backend.config import ollama_model
    model = ollama_model()
    assert isinstance(model, str)
    assert len(model) > 0


def test_ipc_host_port_return_values():
    """Test IPC host and port return sensible values."""
    from backend.config import ipc_host, ipc_port
    host = ipc_host()
    port = ipc_port()
    assert host in ("127.0.0.1", "localhost") or len(host) > 0
    assert port.isdigit() and int(port) > 0


@pytest.mark.parametrize(
    "env_name, getter_name, expected",
    [
        ("IPC_CONNECT_TIMEOUT_SECS", "ipc_connect_timeout_secs", 5),
        ("HTTP_TIMEOUT_SHORT", "http_timeout_short", 10),
        ("HTTP_TIMEOUT", "http_timeout", 30),
        ("HTTP_TIMEOUT_LONG", "http_timeout_long", 60),
        ("IPC_RETRY_DELAY_MS", "ipc_retry_delay_ms", 2000),
        ("LLM_REQUEST_TIMEOUT_SECS", "llm_request_timeout", 120),
        ("SENTIMENT_ANALYSIS_WINDOW_MINUTES", "sentiment_analysis_window_minutes", 120),
        ("LMSTUDIO_HOST", "lmstudio_host", "http://localhost:1234/v1"),
        ("GIT_SAGE_DEFAULT_MODEL", "git_sage_default_model", "llama3.2"),
        ("PROMPT_TIMEOUT_SIMPLE_SECS", "prompt_timeout_simple", 30),
        ("PROMPT_TIMEOUT_WORK_SECS", "prompt_timeout_work", 60),
        ("PROMPT_TIMEOUT_TASK_SECS", "prompt_timeout_task", 120),
    ],
)
@pytest.mark.parametrize("blank", [False, True], ids=["missing", "blank"])
def test_non_secret_setup_settings_have_defaults(monkeypatch, env_name, getter_name, expected, blank):
    import backend.config as config

    if blank:
        monkeypatch.setenv(env_name, "")
    else:
        monkeypatch.delenv(env_name, raising=False)
    assert getattr(config, getter_name)() == expected


@pytest.mark.parametrize(
    "env_name, getter_name, value",
    [
        ("HTTP_TIMEOUT", "http_timeout", "not-a-number"),
        ("PROMPT_TIMEOUT_SIMPLE_SECS", "prompt_timeout_simple", "0"),
        ("IPC_RETRY_DELAY_MS", "ipc_retry_delay_ms", "-1"),
    ],
)
def test_non_secret_setup_settings_reject_invalid_overrides(
    monkeypatch, env_name, getter_name, value
):
    import backend.config as config

    monkeypatch.setenv(env_name, value)
    with pytest.raises(ValueError):
        getattr(config, getter_name)()


@pytest.mark.parametrize(
    "value",
    [
        "postgresql://user:pass@localhost:5432/devtrack",
        "postgresql+psycopg2://user:pass@db/devtrack",
        "postgresql:///devtrack",
    ],
)
def test_require_postgres_url_accepts_postgres(monkeypatch, value):
    from backend.config import require_postgres_url

    monkeypatch.setenv("POSTGRES_URL", value)
    assert require_postgres_url() == value


@pytest.mark.parametrize(
    "value, message",
    [
        (None, "required"),
        ("sqlite:///devtrack.db", "postgresql"),
        ("postgresql://user:pass@localhost", "database name"),
    ],
)
def test_require_postgres_url_rejects_invalid(monkeypatch, value, message):
    from backend.config import ConfigError, require_postgres_url

    if value is None:
        monkeypatch.delenv("POSTGRES_URL", raising=False)
    else:
        monkeypatch.setenv("POSTGRES_URL", value)
    with pytest.raises(ConfigError, match=message):
        require_postgres_url()
