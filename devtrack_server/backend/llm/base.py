"""
Abstract base class and data types for LLM provider abstraction.

All LLM providers (Ollama, OpenAI, Anthropic) implement LLMProvider.
Consumer modules call get_provider().generate(prompt, options) instead of
calling Ollama or cloud APIs directly.
"""

from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from typing import Any, Dict, Optional


@dataclass
class LLMOptions:
    """Options to pass to the LLM for a generation request."""
    temperature: float = 0.3
    max_tokens: int = 300
    # Provider-specific extras (e.g. Ollama's num_ctx, Anthropic's top_k)
    extra: Dict[str, Any] = field(default_factory=dict)


class LLMProvider(ABC):
    """Abstract LLM provider interface.

    Implementations must:
    - Never raise exceptions from generate() — return None on failure
    - Return False from is_available() on any error (fast, < 2s)
    - Be safe to call from multiple threads (no shared mutable state)
    """

    @abstractmethod
    def is_available(self) -> bool:
        """Check if this provider can currently accept requests.

        Must be fast (< 2s). Used for health checks before dispatch.
        Must NOT raise — return False on any error.
        """

    @abstractmethod
    def generate(
        self,
        prompt: str,
        options: Optional[LLMOptions] = None,
        timeout: int = 30,
    ) -> Optional[str]:
        """Generate a text completion.

        Returns the response text stripped of whitespace, or None on failure.
        Must NOT raise — catch all exceptions internally and return None.
        """

    @property
    @abstractmethod
    def provider_name(self) -> str:
        """Short string identifier: 'ollama', 'openai', 'anthropic'."""

    @property
    @abstractmethod
    def model_name(self) -> str:
        """The specific model being used, e.g. 'llama3.2', 'gpt-4o-mini'."""
