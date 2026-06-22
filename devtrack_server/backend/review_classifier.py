"""
review_classifier.py — Phase 7 PR review comment classifier.

Uses the configured LLM to classify a PR review comment as auto-fixable or
needs-human. Always falls back to "needs_human" on any LLM failure — never
auto-fixes without classification confidence.

No os.getenv calls. All config via backend.config.
"""
from __future__ import annotations

import json
import logging

logger = logging.getLogger(__name__)


class ReviewClassifier:
    """
    Uses the configured LLM to classify a review comment as auto-fixable or
    needs-human.  Falls back to "needs_human" on any LLM failure (safe default —
    never auto-fixes without classification confidence).
    """

    AUTO_FIXABLE_CATEGORIES = [
        "formatting",
        "whitespace",
        "line_length",
        "naming_convention",
        "missing_documentation",
        "linting_violation",
        "import_ordering",
        "obvious_simple_logic",
    ]

    _NEEDS_HUMAN_FALLBACK = {
        "classification": "needs_human",
        "reason": "LLM unavailable.",
        "fix_hint": "",
    }

    def __init__(self, provider=None):
        self._provider = provider  # None = lazy init on first use

    def _get_provider(self):
        if self._provider is None:
            from backend.llm import get_provider
            self._provider = get_provider()
        return self._provider

    def classify(self, comment_body: str, pr_title: str, platform: str) -> dict:
        """
        Returns a dict with keys: classification, reason, fix_hint.

        - classification: "auto_fixable" | "needs_human"
        - reason: short explanation
        - fix_hint: short imperative fix instruction (empty for needs_human)

        Never raises.  Falls back to needs_human on any failure.
        """
        try:
            return self._classify_with_llm(comment_body, pr_title, platform)
        except Exception as exc:
            logger.warning(
                "ReviewClassifier LLM call failed: %s — defaulting to needs_human", exc
            )
            return dict(self._NEEDS_HUMAN_FALLBACK)

    def _classify_with_llm(
        self, comment_body: str, pr_title: str, platform: str
    ) -> dict:
        from backend import config

        timeout = config.get_int("LLM_REQUEST_TIMEOUT_SECS", 30)

        prompt = self._build_prompt(comment_body, pr_title, platform)

        provider = self._get_provider()

        # Request JSON-mode response (same approach as commit_message_enhancer.py).
        try:
            raw = provider.complete(
                prompt=prompt,
                format="json",
                timeout=timeout,
            )
        except TypeError:
            # Some provider implementations don't accept format/timeout kwargs.
            raw = provider.complete(prompt)

        if not raw:
            logger.warning("ReviewClassifier: LLM returned empty response")
            return dict(self._NEEDS_HUMAN_FALLBACK)

        return self._parse_response(raw)

    def _build_prompt(self, comment_body: str, pr_title: str, platform: str) -> str:
        return (
            "You are classifying a code review comment. Reply ONLY with valid JSON.\n\n"
            f"PR title: {pr_title}\n"
            f"Platform: {platform}\n"
            f"Review comment: {comment_body}\n\n"
            'Classify as "auto_fixable" if the comment asks for: formatting, whitespace, '
            "line length, naming conventions, missing documentation, linting violations, "
            "import ordering, or obvious simple logic corrections.\n"
            'Classify as "needs_human" for everything else (architecture, design, '
            "business logic, etc.).\n"
            'When in doubt: "needs_human".\n\n'
            "Return exactly:\n"
            '{"classification": "auto_fixable"|"needs_human", '
            '"reason": "...", '
            '"fix_hint": "..."}\n'
            "fix_hint is a short imperative instruction for the fix "
            '(e.g. "Rename variable x to userID").\n'
            'fix_hint is "" when classification is "needs_human".'
        )

    def _parse_response(self, raw: str) -> dict:
        """
        Parse LLM response as JSON.  On any parse failure return the
        needs_human fallback.
        """
        text = raw.strip()

        # Strip markdown code fences if the model wrapped the JSON.
        if text.startswith("```"):
            lines = text.splitlines()
            # Remove opening fence line (``` or ```json).
            lines = lines[1:]
            # Remove closing fence line.
            if lines and lines[-1].strip().startswith("```"):
                lines = lines[:-1]
            text = "\n".join(lines).strip()

        try:
            data = json.loads(text)
        except (json.JSONDecodeError, ValueError) as exc:
            logger.warning(
                "ReviewClassifier: failed to parse LLM JSON (%s). Raw: %r", exc, raw[:200]
            )
            return dict(self._NEEDS_HUMAN_FALLBACK)

        classification = data.get("classification", "needs_human")
        if classification not in ("auto_fixable", "needs_human"):
            logger.warning(
                "ReviewClassifier: unexpected classification %r — defaulting to needs_human",
                classification,
            )
            classification = "needs_human"

        return {
            "classification": classification,
            "reason": str(data.get("reason", "")),
            "fix_hint": str(data.get("fix_hint", "")),
        }
