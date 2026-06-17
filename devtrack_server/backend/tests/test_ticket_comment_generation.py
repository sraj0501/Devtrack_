"""
Tests for generate_ticket_comment() in commit_message_enhancer.py.

Three tests as specified by TASK-072:
  1. LLM available  — returns a non-empty string distinct from raw commit message;
                      ticket_id appears in the prompt sent to the LLM.
  2. LLM unavailable — falls back to a templated string; no exception propagates.
  3. inject_style called — assert inject_style was invoked with context_type="comment".
"""
from __future__ import annotations

import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

# Ensure project root on sys.path
_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Shared fixtures
# ---------------------------------------------------------------------------

COMMIT_MSG = "fix: auth bug"
DIFF_TEXT   = "--- a/auth.py\n+++ b/auth.py\n@@ -1 +1 @@\n-old\n+new"
FILES       = ["auth.py"]
TICKET_ID   = "PROJ-42"


def _make_provider_mock(return_value: str) -> MagicMock:
    """Build a mock LLM provider whose .generate() returns *return_value*."""
    provider = MagicMock()
    provider.generate.return_value = return_value
    return provider


def _llm_patches(mock_provider: MagicMock):
    """Return a context manager that patches the LLM provider and all config
    accessors used inside generate_ticket_comment(), so tests don't require
    real environment variables.

    The config functions (http_timeout etc.) are imported inside the function
    body at call time, so we patch them on their source module (backend.config).
    """
    from contextlib import ExitStack
    stack = ExitStack()
    stack.enter_context(patch(
        "backend.commit_message_enhancer.CommitMessageEnhancer._get_provider",
        return_value=mock_provider,
    ))
    # Patch config accessors at source — generate_ticket_comment imports them
    # fresh from backend.config on each call.
    stack.enter_context(patch("backend.config.http_timeout", return_value=30))
    stack.enter_context(patch("backend.config.commit_llm_temperature", return_value=0.3))
    stack.enter_context(patch("backend.config.commit_llm_max_tokens", return_value=256))
    return stack


# ---------------------------------------------------------------------------
# Test 1 — LLM available
# ---------------------------------------------------------------------------

class TestGenerateTicketCommentLLMAvailable:
    """When the LLM is reachable, generate_ticket_comment() should:
    - return a non-empty string
    - NOT return the raw commit message verbatim (i.e. the LLM response was used)
    - have called the LLM with a prompt that contains the ticket_id
    """

    def test_returns_nonempty_string(self):
        mock_provider = _make_provider_mock("Fixed the authentication issue in this commit.")

        with _llm_patches(mock_provider):
            from backend.commit_message_enhancer import generate_ticket_comment
            result = generate_ticket_comment(
                commit_message=COMMIT_MSG,
                diff=DIFF_TEXT,
                files=FILES,
                ticket_id=TICKET_ID,
                repo_path=None,
            )

        assert result, "Result must be a non-empty string"
        assert isinstance(result, str)

    def test_result_distinct_from_raw_commit_message(self):
        mock_provider = _make_provider_mock("Fixed the authentication issue in this commit.")

        with _llm_patches(mock_provider):
            from backend.commit_message_enhancer import generate_ticket_comment
            result = generate_ticket_comment(
                commit_message=COMMIT_MSG,
                diff=DIFF_TEXT,
                files=FILES,
                ticket_id=TICKET_ID,
                repo_path=None,
            )

        assert result != COMMIT_MSG, (
            "LLM was called and returned a custom response — result must differ from raw msg"
        )

    def test_ticket_id_present_in_prompt_sent_to_llm(self):
        """The prompt construction path must embed the ticket_id so the LLM
        knows which ticket the comment is for.  We assert this by inspecting
        the prompt passed to provider.generate()."""
        mock_provider = _make_provider_mock("Resolved auth timeout on PROJ-42.")

        with _llm_patches(mock_provider):
            from backend.commit_message_enhancer import generate_ticket_comment
            generate_ticket_comment(
                commit_message=COMMIT_MSG,
                diff=DIFF_TEXT,
                files=FILES,
                ticket_id=TICKET_ID,
                repo_path=None,
            )

        assert mock_provider.generate.called, "provider.generate must have been called"
        call_kwargs = mock_provider.generate.call_args
        # The prompt is passed as the first positional arg or as keyword 'prompt'
        prompt_arg: str = (
            call_kwargs.kwargs.get("prompt")
            or (call_kwargs.args[0] if call_kwargs.args else "")
        )
        assert TICKET_ID in prompt_arg, (
            f"ticket_id '{TICKET_ID}' must appear in the prompt sent to the LLM; "
            f"got prompt: {prompt_arg[:300]!r}"
        )


# ---------------------------------------------------------------------------
# Test 2 — LLM unavailable
# ---------------------------------------------------------------------------

class TestGenerateTicketCommentLLMUnavailable:
    """When the LLM raises any exception, generate_ticket_comment() must:
    - not propagate the exception
    - return a non-empty fallback string
    - fallback string must contain the original commit message
    """

    def test_no_exception_propagates(self):
        mock_provider = MagicMock()
        mock_provider.generate.side_effect = ConnectionError("Ollama unreachable")

        with patch(
            "backend.commit_message_enhancer.CommitMessageEnhancer._get_provider",
            return_value=mock_provider,
        ):
            from backend.commit_message_enhancer import generate_ticket_comment
            # Must not raise
            result = generate_ticket_comment(
                commit_message=COMMIT_MSG,
                diff=DIFF_TEXT,
                files=FILES,
                ticket_id=TICKET_ID,
                repo_path=None,
            )

        assert result is not None

    def test_returns_nonempty_fallback(self):
        mock_provider = MagicMock()
        mock_provider.generate.side_effect = RuntimeError("model not loaded")

        with patch(
            "backend.commit_message_enhancer.CommitMessageEnhancer._get_provider",
            return_value=mock_provider,
        ):
            from backend.commit_message_enhancer import generate_ticket_comment
            result = generate_ticket_comment(
                commit_message=COMMIT_MSG,
                diff=DIFF_TEXT,
                files=FILES,
                ticket_id=TICKET_ID,
                repo_path=None,
            )

        assert isinstance(result, str) and result.strip(), "Fallback must be a non-empty string"

    def test_fallback_contains_commit_message(self):
        mock_provider = MagicMock()
        mock_provider.generate.side_effect = OSError("connection refused")

        with patch(
            "backend.commit_message_enhancer.CommitMessageEnhancer._get_provider",
            return_value=mock_provider,
        ):
            from backend.commit_message_enhancer import generate_ticket_comment
            result = generate_ticket_comment(
                commit_message=COMMIT_MSG,
                diff=DIFF_TEXT,
                files=FILES,
                ticket_id=TICKET_ID,
                repo_path=None,
            )

        assert COMMIT_MSG in result, (
            f"Fallback must contain the original commit message; got: {result!r}"
        )


# ---------------------------------------------------------------------------
# Test 3 — inject_style called with context_type="comment"
# ---------------------------------------------------------------------------

class TestGenerateTicketCommentInjectStyle:
    """inject_style() must be called with context_type='comment' so that
    personalization is applied to the ticket comment prompt."""

    def test_inject_style_called_with_comment_context(self):
        mock_provider = _make_provider_mock("Auth fix applied to resolve ticket.")

        # Patch inject_style at the location it is bound in the module
        with (
            _llm_patches(mock_provider),
            patch(
                "backend.commit_message_enhancer._inject_style",
                wraps=lambda p, context_type="general", query_text=None: p,
            ) as mock_inject,
        ):
            from backend.commit_message_enhancer import generate_ticket_comment
            generate_ticket_comment(
                commit_message=COMMIT_MSG,
                diff=DIFF_TEXT,
                files=FILES,
                ticket_id=TICKET_ID,
                repo_path=None,
            )

        assert mock_inject.called, "_inject_style must have been called"
        _, inject_kwargs = mock_inject.call_args
        assert inject_kwargs.get("context_type") == "comment", (
            f"inject_style must be called with context_type='comment'; "
            f"got: {inject_kwargs.get('context_type')!r}"
        )

    def test_inject_style_receives_commit_message_as_query_text(self):
        """The query_text passed to inject_style should be the commit message,
        so RAG retrieves examples of how the user wrote about similar work."""
        mock_provider = _make_provider_mock("Auth fix applied.")

        with (
            _llm_patches(mock_provider),
            patch(
                "backend.commit_message_enhancer._inject_style",
                wraps=lambda p, context_type="general", query_text=None: p,
            ) as mock_inject,
        ):
            from backend.commit_message_enhancer import generate_ticket_comment
            generate_ticket_comment(
                commit_message=COMMIT_MSG,
                diff=DIFF_TEXT,
                files=FILES,
                ticket_id=TICKET_ID,
                repo_path=None,
            )

        _, inject_kwargs = mock_inject.call_args
        assert inject_kwargs.get("query_text") == COMMIT_MSG, (
            f"inject_style query_text should be the commit message; "
            f"got: {inject_kwargs.get('query_text')!r}"
        )
