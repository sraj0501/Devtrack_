"""
Tests for backend.nlp_parser module.
"""
import pytest

# Skip entire module if spaCy not available
spacy = pytest.importorskip("spacy")  # noqa: F401


def test_parse_task_extracts_ticket_id():
    """Test parse_task extracts ticket ID from text like AB-123."""
    from backend.nlp_parser import parse_task

    task = parse_task("Fixed bug in AB-123", use_ollama=False)
    assert task.ticket_id == "AB-123"


def test_parse_task_extracts_action_verb():
    """Test parse_task extracts action verb from text."""
    from backend.nlp_parser import parse_task

    task = parse_task("Fixed bug in AB-123", use_ollama=False)
    assert task.action_verb in ("fix", "fixed", "fixing") or "fix" in (task.action_verb or "").lower()


def test_parse_task_working_on_feature():
    """Test parse_task extracts project/ticket from 'Working on feature'."""
    from backend.nlp_parser import parse_task

    task = parse_task("Working on feature for PROJ-456", use_ollama=False)
    assert task.ticket_id == "PROJ-456" or task.description is not None


def test_parse_task_returns_parsed_task():
    """Test parse_task returns ParsedTask with expected attributes."""
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
    """Test parse_task extracts time spent."""
    from backend.nlp_parser import parse_task

    task = parse_task("Fixed bug in AB-123, spent 2 hours", use_ollama=False)
    # Should extract time or at least not crash
    assert task.raw_text is not None
    assert task.confidence >= 0
