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
