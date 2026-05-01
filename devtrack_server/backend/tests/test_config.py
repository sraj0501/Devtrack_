"""
Tests for backend.config module.
"""
import pytest


def test_config_load_env():
    """Test that _load_env can be called without error."""
    from backend.config import _load_env
    _load_env()


def test_database_path_returns_path():
    """Test database_path returns a Path-like object."""
    from backend.config import _load_env, database_path
    _load_env()
    path = database_path()
    assert path is not None
    assert str(path).endswith(".db") or "devtrack" in str(path).lower() or "daemon" in str(path).lower()


def test_ollama_host_returns_string():
    """Test ollama_host returns a non-empty string."""
    from backend.config import _load_env, ollama_host
    _load_env()
    host = ollama_host()
    assert isinstance(host, str)
    assert len(host) > 0
    assert "localhost" in host or "127.0.0.1" in host or "http" in host


def test_ollama_model_returns_string():
    """Test ollama_model returns a non-empty string."""
    from backend.config import _load_env, ollama_model
    _load_env()
    model = ollama_model()
    assert isinstance(model, str)
    assert len(model) > 0


def test_ipc_host_port_return_values():
    """Test IPC host and port return sensible values."""
    from backend.config import _load_env, ipc_host, ipc_port
    _load_env()
    host = ipc_host()
    port = ipc_port()
    assert host in ("127.0.0.1", "localhost") or len(host) > 0
    assert port.isdigit() and int(port) > 0
