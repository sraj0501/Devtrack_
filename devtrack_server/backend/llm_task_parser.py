"""LLM-owned structured parsing for developer work updates.

The configured provider extracts optional descriptive enrichment only. Ticket
routing remains owned by the Go client and is deliberately absent from this
schema. Provider failures and invalid responses return raw text with empty
optional fields so parsing can never block the commit flow.
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

from backend import config
from backend.llm import LLMOptions, get_provider

logger = logging.getLogger(__name__)

try:
    from backend.work_update_enhancer import get_work_context
except ImportError:  # Optional git/PR context enrichment.
    get_work_context = None


_ALLOWED_STATUSES = {
    "completed",
    "in_progress",
    "started",
    "blocked",
    "waiting",
    "in_review",
    "testing",
}
_SCHEMA_FIELDS = {
    "project",
    "action_verb",
    "status",
    "time_spent",
    "time_estimate",
    "description",
    "confidence",
}

_PROMPT_TEMPLATE = """\
You extract optional descriptive enrichment from a developer's commit message.
Ticket routing is already resolved elsewhere. Do not return or infer a ticket ID.
Return exactly one JSON object with all fields below and no markdown or prose.

Schema:
{{
  "project": "string or null",
  "action_verb": "string or null",
  "status": "completed | in_progress | started | blocked | waiting | in_review | testing | null",
  "time_spent": "string or null",
  "time_estimate": "string or null",
  "description": "non-empty string",
  "confidence": 0.0
}}

Set confidence to a JSON number from 0.0 to 1.0.

Commit message: {text}
"""


class InvalidTaskParse(ValueError):
    """Raised internally when a provider response violates the strict schema."""


@dataclass(frozen=True)
class StructuredTask:
    """Validated descriptive enrichment for one work update."""

    raw_text: str
    project: Optional[str] = None
    action_verb: Optional[str] = None
    time_estimate: Optional[str] = None
    time_spent: Optional[str] = None
    status: Optional[str] = None
    description: str = ""
    confidence: float = 0.0
    git_context: Dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> Dict[str, Any]:
        return {
            "raw_text": self.raw_text,
            "project": self.project,
            "action_verb": self.action_verb,
            "time_estimate": self.time_estimate,
            "time_spent": self.time_spent,
            "status": self.status,
            "description": self.description,
            "confidence": self.confidence,
            "git_context": self.git_context,
        }

    def get(self, field_name: str, default: Any = None) -> Any:
        """Provide mapping-style reads for the trigger enrichment boundary."""
        return self.to_dict().get(field_name, default)


def _optional_string(data: Dict[str, Any], field_name: str) -> Optional[str]:
    value = data[field_name]
    if value is None:
        return None
    if not isinstance(value, str):
        raise InvalidTaskParse(f"{field_name} must be a string or null")
    value = value.strip()
    return value or None


def _validate_response(raw_response: str) -> Dict[str, Any]:
    """Decode a provider response and enforce the exact enrichment schema."""
    try:
        data = json.loads(raw_response.strip())
    except (AttributeError, json.JSONDecodeError) as exc:
        raise InvalidTaskParse("response is not a JSON object") from exc

    if not isinstance(data, dict):
        raise InvalidTaskParse("response must be a JSON object")
    if set(data) != _SCHEMA_FIELDS:
        raise InvalidTaskParse("response fields do not match the task schema")

    confidence = data["confidence"]
    if isinstance(confidence, bool) or not isinstance(confidence, (int, float)):
        raise InvalidTaskParse("confidence must be numeric")
    confidence = float(confidence)
    if not 0.0 <= confidence <= 1.0:
        raise InvalidTaskParse("confidence must be between 0.0 and 1.0")

    status = _optional_string(data, "status")
    if status is not None and status not in _ALLOWED_STATUSES:
        raise InvalidTaskParse("status is not an allowed value")

    description = data["description"]
    if not isinstance(description, str) or not description.strip():
        raise InvalidTaskParse("description must be a non-empty string")

    return {
        "project": _optional_string(data, "project"),
        "action_verb": _optional_string(data, "action_verb"),
        "status": status,
        "time_spent": _optional_string(data, "time_spent"),
        "time_estimate": _optional_string(data, "time_estimate"),
        "description": description.strip(),
        "confidence": confidence,
    }


class LLMTaskParser:
    """Parse work-update text through the configured LLM provider."""

    def __init__(self, provider: Any = None):
        self._provider = provider

    def _git_context(self, repo_path: str) -> Dict[str, Any]:
        if get_work_context is None:
            return {}
        try:
            return get_work_context(repo_path) or {}
        except Exception as exc:
            logger.debug("Work context unavailable during task parsing: %s", exc)
            return {}

    def _fallback(self, text: str, git_context: Dict[str, Any]) -> StructuredTask:
        return StructuredTask(
            raw_text=text,
            description=text,
            confidence=0.0,
            git_context=git_context,
        )

    def parse(self, text: str, repo_path: str = ".") -> StructuredTask:
        """Return validated enrichment or a non-blocking raw-text fallback."""
        git_context = self._git_context(repo_path)
        if not text:
            return self._fallback(text, git_context)

        try:
            provider = self._provider or get_provider()
            raw_response = provider.generate(
                _PROMPT_TEMPLATE.format(text=json.dumps(text)),
                LLMOptions(temperature=0.1, max_tokens=300, extra={"format": "json"}),
                timeout=config.llm_request_timeout(),
            )
            if not raw_response:
                return self._fallback(text, git_context)
            parsed = _validate_response(raw_response)
        except Exception as exc:
            logger.debug("LLM task enrichment unavailable; using raw text: %s", exc)
            return self._fallback(text, git_context)

        return StructuredTask(raw_text=text, git_context=git_context, **parsed)

    def parse_batch(self, texts: List[str]) -> List[StructuredTask]:
        return [self.parse(text) for text in texts]


def parse_task(text: str, provider: Any = None) -> StructuredTask:
    """Parse one work update through the configured provider."""
    return LLMTaskParser(provider=provider).parse(text)
