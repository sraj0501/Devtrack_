"""
Tests for TASK-087 — Inference-to-generation injection.

Coverage:
  1. injection_present: inject_style() includes inference text when InferenceRetriever
     returns inferences with confidence > 0.4.
  2. graceful_degradation: inject_style() returns original prompt unchanged when
     InferenceRetriever.get_top_inferences() raises an exception.
  3. confidence_gate: Inference with confidence=0.3 is excluded from the injected section.
  4. get_dialectic_query_endpoint: InferenceRetriever.get_top_inferences() correctly
     parses the JSON response from the mocked Go daemon HTTP endpoint.
"""
from __future__ import annotations

import json
import sys
from io import BytesIO
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

_ROOT = Path(__file__).resolve().parents[2]
if str(_ROOT) not in sys.path:
    sys.path.insert(0, str(_ROOT))


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_GOOD_INFERENCES = [
    {"id": 1, "subject": "commit tone", "inference": "Developer uses imperative mood.", "confidence": 0.82},
    {"id": 2, "subject": "ticket refs", "inference": "Developer always references ticket IDs.", "confidence": 0.71},
]

_LOW_CONF_INFERENCE = [
    {"id": 3, "subject": "style", "inference": "Developer prefers lowercase messages.", "confidence": 0.30},
]


# ---------------------------------------------------------------------------
# Test 1 — injection_present
# ---------------------------------------------------------------------------

class TestInjectionPresent:
    """inject_style() should include inference text when retriever returns valid inferences."""

    def test_inference_text_in_returned_prompt(self, monkeypatch):
        import backend.personalization as p

        # Reset singletons so we start clean.
        p.reset_cache()

        # Build a mock retriever that returns two high-confidence inferences.
        mock_retriever = MagicMock()
        mock_retriever.get_top_inferences.return_value = _GOOD_INFERENCES

        # Patch _get_inference_retriever to return our mock.
        monkeypatch.setattr(p, "_get_inference_retriever", lambda: mock_retriever)

        # Disable Signal 1 and Signal 2 so only Signal 3 is active.
        monkeypatch.setattr(p, "get_style_instruction", lambda ct: "")
        monkeypatch.setattr(p, "_rag_examples", lambda q, ct: "")

        result = p.inject_style("Write a commit message.", "commit", "test query")

        assert "Developer uses imperative mood." in result
        assert "Developer always references ticket IDs." in result
        assert "Inferred developer patterns" in result

    def test_original_prompt_still_present(self, monkeypatch):
        import backend.personalization as p

        p.reset_cache()

        mock_retriever = MagicMock()
        mock_retriever.get_top_inferences.return_value = _GOOD_INFERENCES

        monkeypatch.setattr(p, "_get_inference_retriever", lambda: mock_retriever)
        monkeypatch.setattr(p, "get_style_instruction", lambda ct: "")
        monkeypatch.setattr(p, "_rag_examples", lambda q, ct: "")

        original = "Write a commit message."
        result = p.inject_style(original, "commit", "test query")

        assert original in result, "Original prompt must be present in the returned string"


# ---------------------------------------------------------------------------
# Test 2 — graceful_degradation
# ---------------------------------------------------------------------------

class TestGracefulDegradation:
    """inject_style() must not crash and must return the original prompt unchanged
    when InferenceRetriever.get_top_inferences() raises an Exception."""

    def test_exception_in_retriever_returns_original(self, monkeypatch):
        import backend.personalization as p

        p.reset_cache()

        mock_retriever = MagicMock()
        mock_retriever.get_top_inferences.side_effect = RuntimeError("daemon unavailable")

        monkeypatch.setattr(p, "_get_inference_retriever", lambda: mock_retriever)
        monkeypatch.setattr(p, "get_style_instruction", lambda ct: "")
        monkeypatch.setattr(p, "_rag_examples", lambda q, ct: "")

        original = "Write a commit message."
        result = p.inject_style(original, "commit", "test query")

        # No crash and inference section should NOT be present.
        assert result == original, (
            f"Expected original prompt unchanged, got: {result!r}"
        )

    def test_none_retriever_does_not_crash(self, monkeypatch):
        """If _get_inference_retriever() returns None, inject_style() should not crash."""
        import backend.personalization as p

        p.reset_cache()

        monkeypatch.setattr(p, "_get_inference_retriever", lambda: None)
        monkeypatch.setattr(p, "get_style_instruction", lambda ct: "")
        monkeypatch.setattr(p, "_rag_examples", lambda q, ct: "")

        original = "Write a commit message."
        result = p.inject_style(original, "commit", "test query")
        assert result == original


# ---------------------------------------------------------------------------
# Test 3 — confidence_gate
# ---------------------------------------------------------------------------

class TestConfidenceGate:
    """Inferences with confidence <= 0.4 must NOT be included in the injected section."""

    def test_low_confidence_inference_excluded(self, monkeypatch):
        import backend.personalization as p

        p.reset_cache()

        mock_retriever = MagicMock()
        mock_retriever.get_top_inferences.return_value = _LOW_CONF_INFERENCE

        monkeypatch.setattr(p, "_get_inference_retriever", lambda: mock_retriever)
        monkeypatch.setattr(p, "get_style_instruction", lambda ct: "")
        monkeypatch.setattr(p, "_rag_examples", lambda q, ct: "")

        original = "Write a commit message."
        result = p.inject_style(original, "commit", "test query")

        # The low-confidence inference must not appear.
        assert "Developer prefers lowercase messages." not in result
        # The "Inferred developer patterns" header must also be absent when no inferences pass the gate.
        assert "Inferred developer patterns" not in result
        # But the original prompt must still be there.
        assert original in result

    def test_mixed_confidence_only_high_pass(self, monkeypatch):
        """When both high- and low-confidence inferences are returned, only high passes."""
        import backend.personalization as p

        p.reset_cache()

        mixed = [
            {"id": 1, "subject": "tone", "inference": "Uses short sentences.", "confidence": 0.75},
            {"id": 2, "subject": "style", "inference": "Prefers lowercase.", "confidence": 0.30},
        ]

        mock_retriever = MagicMock()
        mock_retriever.get_top_inferences.return_value = mixed

        monkeypatch.setattr(p, "_get_inference_retriever", lambda: mock_retriever)
        monkeypatch.setattr(p, "get_style_instruction", lambda ct: "")
        monkeypatch.setattr(p, "_rag_examples", lambda q, ct: "")

        result = p.inject_style("prompt", "commit", "")

        assert "Uses short sentences." in result
        assert "Prefers lowercase." not in result


# ---------------------------------------------------------------------------
# Test 4 — get_dialectic_query_endpoint
# ---------------------------------------------------------------------------

class TestGetDialecticQueryEndpoint:
    """InferenceRetriever._fetch() correctly parses a mocked HTTP response from
    the Go daemon's /dialectic/query endpoint."""

    def _make_mock_response(self, data: dict):
        """Create a mock urllib response that returns the given data as JSON."""
        body = json.dumps(data).encode("utf-8")
        mock_resp = MagicMock()
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)
        mock_resp.status = 200
        mock_resp.read.return_value = body
        return mock_resp

    def test_parses_two_inferences_correctly(self, monkeypatch):
        from backend.inference_retriever import InferenceRetriever

        retriever = InferenceRetriever()

        response_data = {
            "inferences": [
                {"id": 10, "subject": "commit tone", "inference": "Uses imperative mood.", "confidence": 0.85},
                {"id": 11, "subject": "ticket refs", "inference": "Always includes ticket ID.", "confidence": 0.72},
            ]
        }

        mock_resp = self._make_mock_response(response_data)

        with patch("urllib.request.urlopen", return_value=mock_resp):
            result = retriever.get_top_inferences("commit", "imperative mood", top_k=5)

        assert len(result) == 2
        assert result[0]["inference"] == "Uses imperative mood."
        assert result[0]["confidence"] == 0.85
        assert result[1]["inference"] == "Always includes ticket ID."
        assert result[1]["confidence"] == 0.72

    def test_returns_empty_list_on_network_error(self, monkeypatch):
        """Network errors must return [] without raising."""
        from backend.inference_retriever import InferenceRetriever

        retriever = InferenceRetriever()

        with patch("urllib.request.urlopen", side_effect=OSError("connection refused")):
            result = retriever.get_top_inferences("commit", "test", top_k=5)

        assert result == []

    def test_returns_empty_list_on_malformed_json(self, monkeypatch):
        """Malformed JSON from daemon must return [] without raising."""
        from backend.inference_retriever import InferenceRetriever

        retriever = InferenceRetriever()

        mock_resp = MagicMock()
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)
        mock_resp.status = 200
        mock_resp.read.return_value = b"not json at all"

        with patch("urllib.request.urlopen", return_value=mock_resp):
            result = retriever.get_top_inferences("commit", "test", top_k=5)

        assert result == []

    def test_sorted_by_confidence_desc(self, monkeypatch):
        """Results are returned sorted by confidence descending."""
        from backend.inference_retriever import InferenceRetriever

        retriever = InferenceRetriever()

        response_data = {
            "inferences": [
                {"id": 1, "subject": "a", "inference": "low conf", "confidence": 0.50},
                {"id": 2, "subject": "b", "inference": "high conf", "confidence": 0.90},
                {"id": 3, "subject": "c", "inference": "mid conf", "confidence": 0.70},
            ]
        }
        mock_resp = self._make_mock_response(response_data)

        with patch("urllib.request.urlopen", return_value=mock_resp):
            result = retriever.get_top_inferences("commit", "q", top_k=5)

        assert len(result) == 3
        assert result[0]["confidence"] == 0.90
        assert result[1]["confidence"] == 0.70
        assert result[2]["confidence"] == 0.50

    def test_no_os_getenv_in_module(self):
        """Verify inference_retriever.py uses backend.config, not os.getenv directly."""
        import ast
        import inspect
        from backend import inference_retriever

        source = inspect.getsource(inference_retriever)
        tree = ast.parse(source)
        calls = [
            node
            for node in ast.walk(tree)
            if (
                isinstance(node, ast.Call)
                and isinstance(node.func, ast.Attribute)
                and node.func.attr == "getenv"
                and isinstance(node.func.value, ast.Name)
                and node.func.value.id == "os"
            )
        ]
        assert not calls, (
            f"inference_retriever.py must not call os.getenv() — use backend.config "
            f"(found {len(calls)} call(s))"
        )
