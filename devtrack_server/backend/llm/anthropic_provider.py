"""
Anthropic Claude LLM provider.

Requires the 'anthropic' package (optional dependency).
Install: pip install anthropic  OR  uv add anthropic
If the package is absent, is_available() returns False and generate() returns None.
"""

import logging
from typing import Optional

from backend.llm.base import LLMOptions, LLMProvider

logger = logging.getLogger(__name__)


class AnthropicProvider(LLMProvider):
    """LLM provider that dispatches to the Anthropic Claude API."""

    def __init__(self, api_key: Optional[str] = None, model: Optional[str] = None):
        _key = api_key
        _model = model
        if _key is None or _model is None:
            try:
                from backend.config import anthropic_api_key, anthropic_model
                _key = _key if _key is not None else anthropic_api_key()
                _model = _model or anthropic_model()
            except Exception:
                _key = _key or ""
                _model = _model or "claude-haiku-4-5"
        self._api_key = _key
        self._model = _model

    @property
    def provider_name(self) -> str:
        return "anthropic"

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
            import anthropic
        except ImportError:
            logger.debug("anthropic package not installed; AnthropicProvider unavailable")
            return None
        try:
            opts = options or LLMOptions()
            client = anthropic.Anthropic(api_key=self._api_key, timeout=timeout)
            msg = client.messages.create(
                model=self._model,
                max_tokens=opts.max_tokens,
                temperature=opts.temperature,
                messages=[{"role": "user", "content": prompt}],
            )
            text = msg.content[0].text if msg.content else None
            return text.strip() if text else None
        except Exception as e:
            logger.warning(f"AnthropicProvider.generate failed: {e}")
            return None
