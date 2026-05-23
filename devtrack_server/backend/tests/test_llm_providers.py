"""
Tests for the LLM provider abstraction layer.

No network calls are made — all external calls are mocked.
"""

import os
import pytest
from unittest.mock import MagicMock, patch


# ---------------------------------------------------------------------------
# Helpers / fixtures
# ---------------------------------------------------------------------------

class _AlwaysAvailableProvider:
    """Test double: always available, returns a fixed response."""
    def __init__(self, response="mock response"):
        self._response = response

    @property
    def provider_name(self): return "mock"

    @property
    def model_name(self): return "mock-model"

    def is_available(self): return True

    def generate(self, prompt, options=None, timeout=30):
        return self._response


class _NeverAvailableProvider:
    """Test double: never available."""
    @property
    def provider_name(self): return "unavail"

    @property
    def model_name(self): return "unavail"

    def is_available(self): return False

    def generate(self, prompt, options=None, timeout=30):
        pytest.fail("generate() called on unavailable provider")


class _FailingProvider:
    """Test double: available but always returns None (simulate LLM failure)."""
    @property
    def provider_name(self): return "failing"

    @property
    def model_name(self): return "failing"

    def is_available(self): return True

    def generate(self, prompt, options=None, timeout=30):
        return None


# ---------------------------------------------------------------------------
# ProviderChain tests
# ---------------------------------------------------------------------------

class TestProviderChain:
    def _make_chain(self, *providers):
        from backend.llm.provider_factory import ProviderChain
        return ProviderChain(list(providers))

    def test_returns_first_successful_response(self):
        chain = self._make_chain(_AlwaysAvailableProvider("first"))
        assert chain.generate("hi") == "first"

    def test_skips_unavailable_provider_tries_next(self):
        chain = self._make_chain(_NeverAvailableProvider(), _AlwaysAvailableProvider("second"))
        assert chain.generate("hi") == "second"

    def test_skips_failing_provider_tries_next(self):
        chain = self._make_chain(_FailingProvider(), _AlwaysAvailableProvider("fallback"))
        assert chain.generate("hi") == "fallback"

    def test_returns_none_when_all_fail(self):
        chain = self._make_chain(_FailingProvider(), _FailingProvider())
        assert chain.generate("hi") is None

    def test_returns_none_when_all_unavailable(self):
        chain = self._make_chain(_NeverAvailableProvider(), _NeverAvailableProvider())
        assert chain.generate("hi") is None

    def test_primary_property(self):
        p1 = _AlwaysAvailableProvider("first")
        p2 = _AlwaysAvailableProvider("second")
        chain = self._make_chain(p1, p2)
        assert chain.primary is p1

    def test_raises_on_empty_providers(self):
        from backend.llm.provider_factory import ProviderChain
        with pytest.raises(ValueError):
            ProviderChain([])


# ---------------------------------------------------------------------------
# OllamaProvider tests
# ---------------------------------------------------------------------------

class TestOllamaProvider:
    def test_delegates_generate_to_ollama_client(self):
        from backend.llm.ollama_provider import OllamaProvider
        provider = OllamaProvider(host="http://localhost:11434", model="llama3.2")
        with patch("backend.ai.ollama_client.generate", return_value="ollama response") as mock_gen:
            result = provider.generate("test prompt")
            assert result == "ollama response"
            mock_gen.assert_called_once()
            call_kwargs = mock_gen.call_args
            assert call_kwargs.kwargs.get("model") == "llama3.2" or call_kwargs.args[1] == "llama3.2"

    def test_is_available_delegates_to_ollama_client(self):
        from backend.llm.ollama_provider import OllamaProvider
        provider = OllamaProvider(host="http://localhost:11434", model="llama3.2")
        with patch("backend.ai.ollama_client.is_available", return_value=True):
            assert provider.is_available() is True
        with patch("backend.ai.ollama_client.is_available", return_value=False):
            assert provider.is_available() is False

    def test_is_available_returns_false_on_exception(self):
        from backend.llm.ollama_provider import OllamaProvider
        provider = OllamaProvider(host="http://localhost:11434", model="llama3.2")
        with patch("backend.ai.ollama_client.is_available", side_effect=Exception("network error")):
            assert provider.is_available() is False

    def test_generate_returns_none_on_exception(self):
        from backend.llm.ollama_provider import OllamaProvider
        provider = OllamaProvider(host="http://localhost:11434", model="llama3.2")
        with patch("backend.ai.ollama_client.generate", side_effect=Exception("timeout")):
            assert provider.generate("test") is None

    def test_provider_name(self):
        from backend.llm.ollama_provider import OllamaProvider
        assert OllamaProvider(host="h", model="m").provider_name == "ollama"

    def test_model_name(self):
        from backend.llm.ollama_provider import OllamaProvider
        assert OllamaProvider(host="h", model="llama3.2").model_name == "llama3.2"


# ---------------------------------------------------------------------------
# OpenAIProvider tests
# ---------------------------------------------------------------------------

class TestOpenAIProvider:
    def test_not_available_without_key(self):
        from backend.llm.openai_provider import OpenAIProvider
        provider = OpenAIProvider(api_key="", model="gpt-4o-mini")
        assert provider.is_available() is False

    def test_available_with_key(self):
        from backend.llm.openai_provider import OpenAIProvider
        provider = OpenAIProvider(api_key="sk-test", model="gpt-4o-mini")
        assert provider.is_available() is True

    def test_generate_returns_none_without_key(self):
        from backend.llm.openai_provider import OpenAIProvider
        provider = OpenAIProvider(api_key="", model="gpt-4o-mini")
        assert provider.generate("test") is None

    def test_generate_returns_none_when_import_fails(self):
        from backend.llm.openai_provider import OpenAIProvider
        provider = OpenAIProvider(api_key="sk-test", model="gpt-4o-mini")
        with patch.dict("sys.modules", {"openai": None}):
            # ImportError path
            result = provider.generate("test")
            assert result is None

    def test_generate_delegates_to_openai_when_available(self):
        from backend.llm.openai_provider import OpenAIProvider
        from backend.llm.base import LLMOptions

        provider = OpenAIProvider(api_key="sk-test", model="gpt-4o-mini")

        mock_openai = MagicMock()
        mock_client_instance = MagicMock()
        mock_openai.OpenAI.return_value = mock_client_instance
        mock_response = MagicMock()
        mock_response.choices[0].message.content = "openai response"
        mock_client_instance.chat.completions.create.return_value = mock_response

        with patch.dict("sys.modules", {"openai": mock_openai}):
            result = provider.generate("test prompt", LLMOptions(temperature=0.5, max_tokens=100))

        assert result == "openai response"
        mock_client_instance.chat.completions.create.assert_called_once()

    def test_generate_returns_none_on_api_exception(self):
        from backend.llm.openai_provider import OpenAIProvider
        provider = OpenAIProvider(api_key="sk-test", model="gpt-4o-mini")

        mock_openai = MagicMock()
        mock_client_instance = MagicMock()
        mock_openai.OpenAI.return_value = mock_client_instance
        mock_client_instance.chat.completions.create.side_effect = Exception("rate limit")

        with patch.dict("sys.modules", {"openai": mock_openai}):
            result = provider.generate("test")
        assert result is None

    def test_provider_name(self):
        from backend.llm.openai_provider import OpenAIProvider
        assert OpenAIProvider(api_key="", model="m").provider_name == "openai"


# ---------------------------------------------------------------------------
# AnthropicProvider tests
# ---------------------------------------------------------------------------

class TestAnthropicProvider:
    def test_not_available_without_key(self):
        from backend.llm.anthropic_provider import AnthropicProvider
        provider = AnthropicProvider(api_key="", model="claude-haiku-4-5")
        assert provider.is_available() is False

    def test_available_with_key(self):
        from backend.llm.anthropic_provider import AnthropicProvider
        provider = AnthropicProvider(api_key="sk-ant-test", model="claude-haiku-4-5")
        assert provider.is_available() is True

    def test_generate_returns_none_without_key(self):
        from backend.llm.anthropic_provider import AnthropicProvider
        provider = AnthropicProvider(api_key="", model="claude-haiku-4-5")
        assert provider.generate("test") is None

    def test_generate_returns_none_on_api_exception(self):
        from backend.llm.anthropic_provider import AnthropicProvider
        provider = AnthropicProvider(api_key="sk-ant-test", model="claude-haiku-4-5")

        mock_anthropic = MagicMock()
        mock_client = MagicMock()
        mock_anthropic.Anthropic.return_value = mock_client
        mock_client.messages.create.side_effect = Exception("overloaded")

        with patch.dict("sys.modules", {"anthropic": mock_anthropic}):
            result = provider.generate("test")
        assert result is None

    def test_provider_name(self):
        from backend.llm.anthropic_provider import AnthropicProvider
        assert AnthropicProvider(api_key="", model="m").provider_name == "anthropic"


# ---------------------------------------------------------------------------
# Provider factory tests
# ---------------------------------------------------------------------------

class TestProviderFactory:
    def setup_method(self):
        from backend.llm.provider_factory import reset_provider_cache
        reset_provider_cache()

    def teardown_method(self):
        from backend.llm.provider_factory import reset_provider_cache
        reset_provider_cache()

    def test_defaults_to_ollama_when_env_not_set(self):
        from backend.llm.provider_factory import _build_chain
        env = {"LLM_PROVIDER": "ollama", "OPENAI_API_KEY": "", "ANTHROPIC_API_KEY": ""}
        with patch.dict(os.environ, env, clear=False):
            chain = _build_chain()
        assert chain.primary.provider_name == "ollama"

    def test_primary_is_openai_when_configured(self):
        from backend.llm.provider_factory import _build_chain
        env = {"LLM_PROVIDER": "openai", "OPENAI_API_KEY": "sk-test", "ANTHROPIC_API_KEY": ""}
        with patch.dict(os.environ, env, clear=False):
            chain = _build_chain()
        assert chain.primary.provider_name == "openai"

    def test_primary_is_anthropic_when_configured(self):
        from backend.llm.provider_factory import _build_chain
        env = {"LLM_PROVIDER": "anthropic", "OPENAI_API_KEY": "", "ANTHROPIC_API_KEY": "sk-ant"}
        with patch.dict(os.environ, env, clear=False):
            chain = _build_chain()
        assert chain.primary.provider_name == "anthropic"

    def test_ollama_included_as_fallback_when_primary_is_openai(self):
        from backend.llm.provider_factory import _build_chain
        env = {"LLM_PROVIDER": "openai", "OPENAI_API_KEY": "sk-test", "ANTHROPIC_API_KEY": ""}
        with patch.dict(os.environ, env, clear=False):
            chain = _build_chain()
        names = [p.provider_name for p in chain.providers]
        assert "ollama" in names
        assert names[0] == "openai"

    def test_get_provider_returns_cached_instance(self):
        from backend.llm.provider_factory import get_provider
        with patch.dict(os.environ, {"LLM_PROVIDER": "ollama"}, clear=False):
            p1 = get_provider()
            p2 = get_provider()
        assert p1 is p2

    def test_reset_cache_rebuilds_chain(self):
        from backend.llm.provider_factory import get_provider, reset_provider_cache
        with patch.dict(os.environ, {"LLM_PROVIDER": "ollama"}, clear=False):
            p1 = get_provider()
        reset_provider_cache()
        with patch.dict(os.environ, {"LLM_PROVIDER": "ollama"}, clear=False):
            p2 = get_provider()
        assert p1 is not p2


# ---------------------------------------------------------------------------
# Config integration tests
# ---------------------------------------------------------------------------

class TestConfigIntegration:
    def test_llm_provider_defaults_to_ollama(self):
        from backend.config import llm_provider
        saved = os.environ.pop("LLM_PROVIDER", None)
        try:
            result = llm_provider()
            assert result == "ollama"
        finally:
            if saved is not None:
                os.environ["LLM_PROVIDER"] = saved

    def test_openai_api_key_returns_empty_when_not_set(self):
        from backend.config import openai_api_key
        saved = os.environ.pop("OPENAI_API_KEY", None)
        try:
            assert openai_api_key() == ""
        finally:
            if saved is not None:
                os.environ["OPENAI_API_KEY"] = saved

    def test_anthropic_api_key_returns_empty_when_not_set(self):
        from backend.config import anthropic_api_key
        saved = os.environ.pop("ANTHROPIC_API_KEY", None)
        try:
            assert anthropic_api_key() == ""
        finally:
            if saved is not None:
                os.environ["ANTHROPIC_API_KEY"] = saved
