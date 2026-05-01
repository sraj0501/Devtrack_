"""
Ollama LLM provider.

Thin wrapper over the existing backend/ai/ollama_client.py so that
module is preserved untouched (other code may import it directly).
"""

import logging
from typing import Optional

from backend.llm.base import LLMOptions, LLMProvider

logger = logging.getLogger(__name__)


class OllamaProvider(LLMProvider):
    """LLM provider that dispatches to a local Ollama instance."""

    def __init__(self, host: Optional[str] = None, model: Optional[str] = None):
        # Load from config (REQUIRED: OLLAMA_HOST, OLLAMA_MODEL in .env)
        from backend.config import ollama_host, ollama_model
        self._host = host or ollama_host()
        self._model = model or ollama_model()

    @property
    def provider_name(self) -> str:
        return "ollama"

    @property
    def model_name(self) -> str:
        return self._model

    def is_available(self) -> bool:
        try:
            from backend.ai.ollama_client import is_available
            return is_available(self._host)
        except Exception:
            return False

    def generate(
        self,
        prompt: str,
        options: Optional[LLMOptions] = None,
        timeout: int = 30,
    ) -> Optional[str]:
        try:
            from backend.ai.ollama_client import generate
            opts = options or LLMOptions()
            return generate(
                prompt=prompt,
                model=self._model,
                host=self._host,
                options={"temperature": opts.temperature, "num_predict": opts.max_tokens, **opts.extra},
                timeout=timeout,
            )
        except Exception as e:
            logger.warning(f"OllamaProvider.generate failed: {e}")
            return None
