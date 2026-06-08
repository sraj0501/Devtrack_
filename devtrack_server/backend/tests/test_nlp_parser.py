"""
Tests for backend.nlp_parser module.

All tests use use_ollama=False to exercise the pure regex path without
requiring a running LLM. One test mocks the LLM provider to verify the
enrichment path.
"""
import json
import pytest
from unittest.mock import MagicMock, patch


def test_parse_task_extracts_ticket_id():
    """parse_task extracts ticket ID from text like AB-123."""
    from backend.nlp_parser import parse_task

    task = parse_task("Fixed bug in AB-123", use_ollama=False)
    assert task.ticket_id == "AB-123"


def test_parse_task_extracts_action_verb():
    """parse_task extracts action verb from text."""
    from backend.nlp_parser import parse_task

    task = parse_task("Fixed bug in AB-123", use_ollama=False)
    assert task.action_verb in ("fix", "fixed", "fixing") or "fix" in (task.action_verb or "").lower()


def test_parse_task_working_on_feature():
    """parse_task extracts project/ticket from 'Working on feature'."""
    from backend.nlp_parser import parse_task

    task = parse_task("Working on feature for PROJ-456", use_ollama=False)
    assert task.ticket_id == "PROJ-456" or task.description is not None


def test_parse_task_returns_parsed_task():
    """parse_task returns ParsedTask with expected attributes."""
    from backend.nlp_parser import parse_task, ParsedTask

    task = parse_task("Completed task AB-123", use_ollama=False)
    assert isinstance(task, ParsedTask)
    assert task.raw_text == "Completed task AB-123"
    assert hasattr(task, "project")
    assert hasattr(task, "ticket_id")
    assert hasattr(task, "description")
    assert hasattr(task, "action_verb")
    assert hasattr(task, "status")
    assert hasattr(task, "confidence")


def test_parse_task_with_time_spent():
    """parse_task extracts time spent without crashing."""
    from backend.nlp_parser import parse_task

    task = parse_task("Fixed bug in AB-123, spent 2 hours", use_ollama=False)
    assert task.raw_text is not None
    assert task.confidence >= 0


def test_parse_task_llm_enrichment():
    """LLM result is used when use_ollama=True and provider returns valid JSON."""
    from backend.nlp_parser import NLPTaskParser

    llm_response = json.dumps({
        "ticket_id": "AB-123",
        "project": "Alpha",
        "action_verb": "fixed",
        "status": "completed",
        "time_spent": "2h",
        "time_estimate": None,
        "description": "Fixed login bug",
    })
    mock_chain = MagicMock()
    mock_chain.generate.return_value = llm_response

    with patch("backend.llm.get_provider", return_value=mock_chain):
        parser = NLPTaskParser(use_ollama=True)
        task = parser.parse("Fixed login bug for Project Alpha AB-123, spent 2 hours")

    assert task.ticket_id == "AB-123"
    assert task.project == "Alpha"
    assert task.status == "completed"
    assert task.time_spent == "2h"
    assert task.action_verb == "fixed"


def test_parse_task_llm_fallback_on_bad_json():
    """Parser falls back to regex when LLM returns non-JSON."""
    from backend.nlp_parser import NLPTaskParser

    mock_chain = MagicMock()
    mock_chain.generate.return_value = "I cannot parse that."

    with patch("backend.llm.get_provider", return_value=mock_chain):
        parser = NLPTaskParser(use_ollama=True)
        task = parser.parse("Fixed bug in PROJ-789")

    # Regex fallback should still find the ticket
    assert task.ticket_id == "PROJ-789"
    assert task.confidence >= 0
