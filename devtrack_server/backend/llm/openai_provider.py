"""
OpenAI LLM provider.

Requires the 'openai' package (optional dependency).
Install: pip install openai  OR  uv add openai
If the package is absent, is_available() returns False and generate() returns None.
"""

import logging
from typing import Optional

from backend.llm.base import LLMOptions, LLMProvider

logger = logging.getLogger(__name__)


class OpenAIProvider(LLMProvider):
    """LLM provider that dispatches to the OpenAI API."""

    def __init__(self, api_key: Optional[str] = None, model: Optional[str] = None):
        _key = api_key
        _model = model
        if _key is None or _model is None:
            try:
                from backend.config import openai_api_key, openai_model
                _key = _key if _key is not None else openai_api_key()
                _model = _model or openai_model()
            except Exception:
                _key = _key or ""
                _model = _model or "gpt-4o-mini"
        self._api_key = _key
        self._model = _model

    @property
    def provider_name(self) -> str:
        return "openai"

    @property
    def model_name(self) -> str:
        return self._model

    def is_available(self) -> bool:
        """Available if we have an API key. No network check — key presence is sufficient."""
        return bool(self._api_key)

    def generate(
        self,
        prompt: str,
        options: Optional[LLMOptions] = None,
        timeout: int = 30,
    ) -> Optional[str]:
        if not self._api_key:
            return None
        try:
            import openai
        except ImportError:
            logger.debug("openai package not installed; OpenAIProvider unavailable")
            return None
        try:
            opts = options or LLMOptions()
            client = openai.OpenAI(api_key=self._api_key, timeout=timeout)
            resp = client.chat.completions.create(
                model=self._model,
                messages=[{"role": "user", "content": prompt}],
                temperature=opts.temperature,
                max_tokens=opts.max_tokens,
            )
            text = resp.choices[0].message.content
            return text.strip() if text else None
        except Exception as e:
            logger.warning(f"OpenAIProvider.generate failed: {e}")
            return None
