"""
LLM provider factory with fallback chain.

Usage:
    from backend.llm import get_provider
    result = get_provider().generate("your prompt")

The factory reads LLM_PROVIDER from config (default: 'ollama') and builds an
ordered provider chain. On each generate() call, providers are tried in order;
the first non-None response wins. Ollama is always the final fallback since it
requires no API key.

The chain is cached as a module-level singleton (built once per process).
Call reset_provider_cache() in tests that need to change the provider mid-test.
"""

import logging
from typing import List, Optional

from backend.llm.base import LLMOptions, LLMProvider

logger = logging.getLogger(__name__)

_provider_cache: Optional["ProviderChain"] = None


class ProviderChain:
    """Ordered list of providers with automatic fallback on failure or unavailability."""

    def __init__(self, providers: List[LLMProvider]):
        if not providers:
            raise ValueError("ProviderChain requires at least one provider")
        self._providers = providers

    @property
    def primary(self) -> LLMProvider:
        return self._providers[0]

    @property
    def providers(self) -> List[LLMProvider]:
        return list(self._providers)

    def generate(
        self,
        prompt: str,
        options: Optional[LLMOptions] = None,
        timeout: int = 30,
    ) -> Optional[str]:
        """Try each provider in order. Return the first non-None result."""
        for provider in self._providers:
            if not provider.is_available():
                logger.debug(f"LLM provider '{provider.provider_name}' unavailable, skipping")
                continue
            result = provider.generate(prompt, options, timeout)
            if result is not None:
                if provider is not self._providers[0]:
                    logger.info(
                        f"Used fallback LLM provider: '{provider.provider_name}' "
                        f"(primary '{self._providers[0].provider_name}' failed)"
                    )
                return result
        logger.warning("All LLM providers failed or unavailable — returning None")
        return None


def _build_chain() -> ProviderChain:
    """Build provider chain from configuration."""
    from backend.llm.ollama_provider import OllamaProvider
    from backend.llm.openai_provider import OpenAIProvider
    from backend.llm.anthropic_provider import AnthropicProvider

    try:
        from backend.config import llm_provider, openai_api_key, anthropic_api_key
        primary_name = llm_provider()
        has_openai = bool(openai_api_key())
        has_anthropic = bool(anthropic_api_key())
    except Exception:
        primary_name = "ollama"
        has_openai = False
        has_anthropic = False

    provider_map = {
        "ollama": OllamaProvider,
        "openai": OpenAIProvider,
        "anthropic": AnthropicProvider,
    }

    ordered: List[LLMProvider] = []

    # Primary provider first
    primary_cls = provider_map.get(primary_name, OllamaProvider)
    ordered.append(primary_cls())

    # Add other providers that have credentials as fallbacks
    if primary_name != "openai" and has_openai:
        ordered.append(OpenAIProvider())
    if primary_name != "anthropic" and has_anthropic:
        ordered.append(AnthropicProvider())
    # Ollama is always the final free fallback (no API key required)
    if primary_name != "ollama":
        ordered.append(OllamaProvider())

    names = [p.provider_name for p in ordered]
    logger.debug(f"LLM provider chain built: {names}")
    return ProviderChain(ordered)


def get_provider() -> ProviderChain:
    """Get (or lazily create) the global provider chain.

    Thread-safe at the module-import level; the singleton is set once.
    For test isolation use reset_provider_cache() before and after each test.
    """
    global _provider_cache
    if _provider_cache is None:
        _provider_cache = _build_chain()
    return _provider_cache


def reset_provider_cache() -> None:
    """Force rebuild on next get_provider() call. Use in tests only."""
    global _provider_cache
    _provider_cache = None
