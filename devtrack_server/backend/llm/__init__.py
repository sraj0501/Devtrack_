"""
Multi-provider LLM abstraction for DevTrack.

Quick start:
    from backend.llm import get_provider
    result = get_provider().generate("summarize this commit")

Provider selection is driven by LLM_PROVIDER in .env (default: 'ollama').
Supported: 'ollama', 'openai', 'anthropic'.

The project can run entirely on Ollama with no cloud dependencies.
Cloud providers are optional — add keys to .env to enable them as fallbacks.
"""

from backend.llm.base import LLMOptions, LLMProvider
from backend.llm.provider_factory import ProviderChain, get_provider, reset_provider_cache

__all__ = [
    "LLMProvider",
    "LLMOptions",
    "ProviderChain",
    "get_provider",
    "reset_provider_cache",
]
