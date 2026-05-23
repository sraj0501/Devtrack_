"""
Groq LLM provider (OpenAI-compatible API).

Requires the 'openai' package (same as OpenAI provider — Groq uses the same SDK).
Install: uv add openai
If the package is absent or GROQ_API_KEY is unset, is_available() returns False.
"""

import logging
from typing import Optional

from backend.llm.base import LLMOptions, LLMProvider

logger = logging.getLogger(__name__)


class GroqProvider(LLMProvider):
    """LLM provider that dispatches to the Groq cloud API (OpenAI-compatible)."""

    def __init__(
        self,
        api_key: Optional[str] = None,
        model: Optional[str] = None,
        base_url: Optional[str] = None,
    ):
        _key = api_key
        _model = model
        _url = base_url
        try:
            from backend.config import groq_api_key, groq_model, groq_host
            _key   = _key   or groq_api_key()
            _model = _model or groq_model()
            _url   = _url   or groq_host()
        except Exception:
            _key   = _key   or ""
            _model = _model or "llama-3.3-70b-versatile"
            _url   = _url   or "https://api.groq.com/openai/v1"

        self._api_key  = _key
        self._model    = _model
        self._base_url = _url

    @property
    def provider_name(self) -> str:
        return "groq"

    @property
    def model_name(self) -> str:
        return self._model

    def is_available(self) -> bool:
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
            logger.debug("openai package not installed; GroqProvider unavailable")
            return None
        try:
            opts = options or LLMOptions()
            client = openai.OpenAI(
                api_key=self._api_key,
                base_url=self._base_url,
                timeout=timeout,
            )
            resp = client.chat.completions.create(
                model=self._model,
                messages=[{"role": "user", "content": prompt}],
                temperature=opts.temperature,
                max_tokens=opts.max_tokens,
            )
            text = resp.choices[0].message.content
            return text.strip() if text else None
        except Exception as e:
            logger.warning(f"GroqProvider.generate failed: {e}")
            return None
