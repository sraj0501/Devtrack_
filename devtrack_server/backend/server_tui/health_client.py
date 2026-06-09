"""
Health client — polls GET /health on the webhook server and Ollama.
"""
from __future__ import annotations

import ssl
from dataclasses import dataclass
from typing import Optional
from urllib import request as urllib_request
from urllib.error import URLError

# Self-signed TLS context for loopback health checks — cert is trusted by the
# process that generated it; we just need encryption, not external CA validation.
_LOCAL_SSL_CTX = ssl.create_default_context()
_LOCAL_SSL_CTX.check_hostname = False
_LOCAL_SSL_CTX.verify_mode = ssl.CERT_NONE


@dataclass
class ServiceHealth:
    name: str
    url: str
    ok: bool
    detail: str = ""


def _webhook_url() -> str:
    from backend.config import get_webhook_port, get_webhook_host, get_bool
    port = get_webhook_port()
    host = get_webhook_host()
    # Normalise 0.0.0.0 → localhost for outbound checks
    if host == "0.0.0.0":
        host = "localhost"
    scheme = "https" if get_bool("DEVTRACK_TLS", False) else "http"
    return f"{scheme}://{host}:{port}/health"


def _ollama_url() -> str:
    from backend.config import ollama_host
    raw = ollama_host()
    return raw.rstrip("/") + "/api/tags"


def _check(url: str, timeout: int = 3) -> tuple[bool, str]:
    try:
        ctx = _LOCAL_SSL_CTX if url.startswith("https://") else None
        with urllib_request.urlopen(url, timeout=timeout, context=ctx) as resp:
            return resp.status == 200, f"HTTP {resp.status}"
    except URLError as e:
        return False, str(e.reason)
    except Exception as e:
        return False, str(e)


def check_all() -> list[ServiceHealth]:
    """Return health status for webhook server and Ollama."""
    results: list[ServiceHealth] = []

    webhook_url = _webhook_url()
    ok, detail = _check(webhook_url)
    results.append(ServiceHealth(name="webhook_server", url=webhook_url, ok=ok, detail=detail))

    ollama_url = _ollama_url()
    ok, detail = _check(ollama_url)
    results.append(ServiceHealth(name="ollama", url=ollama_url, ok=ok, detail=detail))

    return results


def check_webhook() -> Optional[bool]:
    """Quick check — returns True/False/None (None = connection refused, not running)."""
    ok, detail = _check(_webhook_url())
    return ok
