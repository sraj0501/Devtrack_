"""
InferenceRetriever — retrieves top-k reasoned inferences from the Go daemon's
/dialectic/query internal HTTP endpoint and presents them as plain dicts for
injection into LLM prompts via inject_style().

Never calls os.getenv directly — all config via backend.config.
Returns [] on any failure (network error, daemon unavailable, JSON parse failure).
"""
from __future__ import annotations

import logging
from typing import Optional

logger = logging.getLogger(__name__)

# Minimum confidence for an inference to be injected into a prompt.
# This is a module constant — NOT a config var.
INFERENCE_MIN_CONFIDENCE: float = 0.4


class InferenceRetriever:
    """
    Retrieves top-k inferences from the inferences SQLite table via the Go
    client's /dialectic/query HTTP endpoint. Falls back to [] if unavailable.

    Config (via backend.config, never os.getenv):
      IPC_HOST                  — Go daemon bind host (default: 127.0.0.1)
      DEVTRACK_SERVER_HTTP_PORT — Go daemon internal HTTP port (default: 35894)
      DEVTRACK_API_KEY          — API key sent as X-DevTrack-API-Key header
    """

    _TIMEOUT: float = 1.0  # seconds — keep fast so callers are never blocked

    def get_top_inferences(
        self,
        context_type: str,
        query_text: str,
        top_k: int = 5,
    ) -> list[dict]:
        """Return up to top_k inference dicts sorted by confidence DESC.

        Each dict has keys: "subject", "inference", "confidence"
        (and optionally "id" from the DB).

        Returns [] on ANY failure — never raises.
        """
        try:
            return self._fetch(context_type, query_text, top_k)
        except Exception as exc:
            logger.debug("inference_retriever: get_top_inferences failed: %s", exc)
            return []

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _build_url(self, context_type: str, query_text: str, limit: int) -> str:
        """Build the /dialectic/query URL from config."""
        from backend import config  # noqa: PLC0415 — local import keeps top-level import-free
        import urllib.parse  # noqa: PLC0415

        host = config.ipc_host()
        port = config.get_devtrack_control_port()
        base = f"http://{host}:{port}/dialectic/query"
        params = {
            "context_type": context_type,
            "q": query_text,
            "limit": str(limit),
        }
        return base + "?" + urllib.parse.urlencode(params)

    def _fetch(self, context_type: str, query_text: str, top_k: int) -> list[dict]:
        """Perform the HTTP GET and return parsed inference list."""
        import json  # noqa: PLC0415
        import urllib.request  # noqa: PLC0415
        from backend import config  # noqa: PLC0415

        url = self._build_url(context_type, query_text, top_k)
        api_key = config.get_devtrack_api_key()

        req = urllib.request.Request(url)
        if api_key:
            req.add_header("X-DevTrack-API-Key", api_key)

        with urllib.request.urlopen(req, timeout=self._TIMEOUT) as resp:
            if resp.status != 200:
                logger.debug(
                    "inference_retriever: /dialectic/query returned HTTP %d", resp.status
                )
                return []
            data = json.loads(resp.read().decode("utf-8"))

        raw_inferences: list[dict] = data.get("inferences", [])

        # Normalise: keep only the keys we need and sort by confidence DESC.
        result: list[dict] = []
        for item in raw_inferences:
            try:
                result.append(
                    {
                        "id": item.get("id"),
                        "subject": str(item.get("subject", "")),
                        "inference": str(item.get("inference", "")),
                        "confidence": float(item.get("confidence", 0.0)),
                    }
                )
            except (TypeError, ValueError):
                continue  # skip malformed rows

        result.sort(key=lambda x: x["confidence"], reverse=True)
        return result[:top_k]
