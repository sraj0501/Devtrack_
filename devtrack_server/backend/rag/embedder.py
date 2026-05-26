"""
Ollama embedding calls for RAG personalization.

Uses Ollama's /api/embed endpoint with a dedicated embedding model
(default: nomic-embed-text) that is separate from the chat model.

All public functions return None on failure so callers can skip RAG
gracefully without crashing.
"""
from __future__ import annotations

import logging
from typing import Optional

import requests

logger = logging.getLogger(__name__)

# ── config helpers ────────────────────────────────────────────────────────────

def _ollama_host() -> str:
    from backend.config import ollama_host
    return ollama_host().rstrip("/")


def _embed_model() -> str:
    try:
        from backend.config import get
        return get("PERSONALIZATION_EMBED_MODEL", "nomic-embed-text") or "nomic-embed-text"
    except Exception:
        return "nomic-embed-text"


def _http_timeout() -> int:
    try:
        from backend.config import http_timeout_short
        return http_timeout_short()
    except Exception:
        return 10


# ── public API ────────────────────────────────────────────────────────────────

def embed(text: str) -> Optional[list[float]]:
    """
    Embed *text* using the configured Ollama embedding model.

    Returns the embedding vector, or None if Ollama is unavailable or the
    model is not pulled.  Callers should treat None as "skip RAG".
    """
    host = _ollama_host()
    model = _embed_model()
    timeout = _http_timeout()

    # Try the newer /api/embed endpoint first (Ollama >= 0.1.26)
    try:
        resp = requests.post(
            f"{host}/api/embed",
            json={"model": model, "input": text},
            timeout=timeout,
        )
        if resp.status_code == 200:
            data = resp.json()
            # /api/embed returns {"embeddings": [[...]], ...}
            embeddings = data.get("embeddings") or data.get("embedding")
            if embeddings:
                vec = embeddings[0] if isinstance(embeddings[0], list) else embeddings
                return vec
    except requests.RequestException:
        pass

    # Fallback to legacy /api/embeddings endpoint
    try:
        resp = requests.post(
            f"{host}/api/embeddings",
            json={"model": model, "prompt": text},
            timeout=timeout,
        )
        if resp.status_code == 200:
            data = resp.json()
            vec = data.get("embedding")
            if vec:
                return vec
    except requests.RequestException as exc:
        logger.debug("RAG embedder: Ollama unavailable (%s)", exc)

    return None


def embed_batch(texts: list[str]) -> list[Optional[list[float]]]:
    """Embed multiple texts.  Returns a list aligned with *texts*."""
    return [embed(t) for t in texts]


def model_available() -> bool:
    """Return True if the embedding model is pulled and ready."""
    host = _ollama_host()
    model = _embed_model()
    try:
        resp = requests.get(f"{host}/api/tags", timeout=5)
        if resp.status_code == 200:
            names = [m.get("name", "") for m in resp.json().get("models", [])]
            return any(model in n for n in names)
    except requests.RequestException:
        pass
    return False
