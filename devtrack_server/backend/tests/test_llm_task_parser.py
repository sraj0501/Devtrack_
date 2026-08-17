"""Tests for the strict, non-blocking LLM task-enrichment boundary."""

import json
from unittest.mock import MagicMock, patch

import pytest

from backend.llm_task_parser import LLMTaskParser, StructuredTask, parse_task


def _response(**overrides):
    data = {
        "project": "Alpha",
        "action_verb": "fixed",
        "status": "completed",
        "time_spent": "2h",
        "time_estimate": None,
        "description": "Fixed login bug",
        "confidence": 0.91,
    }
    data.update(overrides)
    return json.dumps(data)


def test_valid_provider_response_returns_structured_enrichment():
    provider = MagicMock()
    provider.generate.return_value = _response()

    task = parse_task("PROJ-123: fixed login bug", provider=provider)

    assert isinstance(task, StructuredTask)
    assert task.description == "Fixed login bug"
    assert task.project == "Alpha"
    assert task.status == "completed"
    assert task.confidence == pytest.approx(0.91)
    assert "ticket_id" not in task.to_dict()


@pytest.mark.parametrize(
    "response",
    [
        "not json",
        "```json\n{}\n```",
        json.dumps([]),
        _response(confidence=1.1),
        _response(confidence="high"),
        _response(status="done"),
        _response(description=""),
        json.dumps({
            "project": None,
            "action_verb": None,
            "status": None,
            "time_spent": None,
            "time_estimate": None,
            "description": "work",
            "confidence": 0.5,
            "ticket_id": "PROJ-999",
        }),
    ],
)
def test_invalid_provider_response_degrades_to_raw_text(response):
    provider = MagicMock()
    provider.generate.return_value = response

    task = parse_task("PROJ-123: fixed login bug", provider=provider)

    assert task.description == "PROJ-123: fixed login bug"
    assert task.project is None
    assert task.action_verb is None
    assert task.status is None
    assert task.time_spent is None
    assert task.time_estimate is None
    assert task.confidence == 0.0


def test_unavailable_provider_degrades_without_raising():
    provider = MagicMock()
    provider.generate.return_value = None

    task = LLMTaskParser(provider=provider).parse("chore: cleanup")

    assert task.raw_text == "chore: cleanup"
    assert task.description == "chore: cleanup"
    assert task.confidence == 0.0


def test_provider_exception_degrades_without_raising():
    provider = MagicMock()
    provider.generate.side_effect = TimeoutError("provider timeout")

    task = LLMTaskParser(provider=provider).parse("chore: cleanup")

    assert task.description == "chore: cleanup"
    assert task.confidence == 0.0


def test_uses_configured_provider_and_timeout():
    provider = MagicMock()
    provider.generate.return_value = _response()

    with (
        patch("backend.llm_task_parser.get_provider", return_value=provider),
        patch("backend.llm_task_parser.config.llm_request_timeout", return_value=17),
    ):
        task = LLMTaskParser().parse("fix login")

    assert task.description == "Fixed login bug"
    assert provider.generate.call_args.kwargs["timeout"] == 17
    assert provider.generate.call_args.args[1].extra == {"format": "json"}


def test_empty_text_returns_explicit_zero_confidence_without_provider_call():
    provider = MagicMock()

    task = LLMTaskParser(provider=provider).parse("")

    provider.generate.assert_not_called()
    assert task.description == ""
    assert task.confidence == 0.0
