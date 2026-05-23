"""
Shared Ollama client for AI operations.

Provides a centralized interface for Ollama API calls used across
description_enhancer, git_diff_analyzer, daily_report_generator,
commit_message_enhancer, sentiment_analysis, personalized_ai, and create_tasks.
"""

import logging
from typing import Any, Dict, Optional

logger = logging.getLogger(__name__)

# Lazy import to avoid loading config at module level before env is loaded
_config = None


def _get_config():
    global _config
    if _config is None:
        # Ensure project root is in path for config import
        import sys
        from pathlib import Path
        _backend_dir = Path(__file__).resolve().parent.parent
        _project_root = _backend_dir.parent
        if str(_project_root) not in sys.path:
            sys.path.insert(0, str(_project_root))
        try:
            from backend.config import ollama_host, ollama_model
        except ImportError:
            from config import ollama_host, ollama_model
        _config = {"host": ollama_host(), "model": ollama_model()}
    return _config


def get_ollama_host() -> str:
    """Get configured Ollama host URL."""
    return _get_config()["host"]


def get_ollama_model(model_override: Optional[str] = None) -> str:
    """Get configured Ollama model, with optional override."""
    if model_override:
        return model_override
    return _get_config()["model"]


def is_available(host: Optional[str] = None) -> bool:
    """Check if Ollama is available and responsive."""
    import urllib.request
    from backend.config import http_timeout_short
    h = host or get_ollama_host()
    try:
        req = urllib.request.Request(f"{h}/api/tags")
        with urllib.request.urlopen(req, timeout=http_timeout_short()) as response:
            return response.status == 200
    except Exception:
        return False


def generate(
    prompt: str,
    model: Optional[str] = None,
    host: Optional[str] = None,
    stream: bool = False,
    options: Optional[Dict[str, Any]] = None,
    timeout: int = 30
) -> Optional[str]:
    """
    Call Ollama generate API.

    Args:
        prompt: The prompt to send
        model: Model name (uses config default if None)
        host: Ollama host URL (uses config if None)
        stream: Whether to stream response
        options: Additional options (temperature, num_predict, etc.)
        timeout: Request timeout in seconds

    Returns:
        Response text or None on failure
    """
    import urllib.request
    import json

    h = host or get_ollama_host()
    m = get_ollama_model(model)

    payload = {
        "model": m,
        "prompt": prompt,
        "stream": stream,
        "options": options or {"temperature": 0.3, "num_predict": 300}
    }

    try:
        data = json.dumps(payload).encode()
        req = urllib.request.Request(
            f"{h}/api/generate",
            data=data,
            headers={"Content-Type": "application/json"}
        )
        with urllib.request.urlopen(req, timeout=timeout) as response:
            result = json.loads(response.read().decode())
            return result.get("response", "").strip()
    except Exception as e:
        logger.warning(f"Ollama generate failed: {e}")
        return None


def generate_with_requests(
    prompt: str,
    model: Optional[str] = None,
    host: Optional[str] = None,
    stream: bool = False,
    options: Optional[Dict[str, Any]] = None,
    timeout: int = 30
) -> Optional[str]:
    """
    Call Ollama generate API using requests library.
    Use when requests is already imported in the module.
    """
    try:
        import requests
    except ImportError:
        return generate(prompt, model, host, stream, options, timeout)

    h = host or get_ollama_host()
    m = get_ollama_model(model)

    try:
        response = requests.post(
            f"{h}/api/generate",
            json={
                "model": m,
                "prompt": prompt,
                "stream": stream,
                "options": options or {"temperature": 0.3, "num_predict": 300}
            },
            timeout=timeout
        )
        if response.status_code != 200:
            return None
        return response.json().get("response", "").strip()
    except Exception as e:
        logger.warning(f"Ollama generate failed: {e}")
        return None
