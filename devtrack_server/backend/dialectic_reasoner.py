"""
DialecticReasoner — Phase 6 dialectic self-improvement.

After each developer interaction (commit, approval, rejection, edit), this module
runs a local reasoning pass via Hermes 3 (Ollama) to produce structured inferences
about the developer's writing style and work patterns.

Falls back to the configured LLM chain (provider_factory) if Hermes 3 is unavailable.
Returns [] on any LLM failure — never raises (Non-Negotiable #8).

All config via backend.config.get() / get_int() — no os.getenv.
"""

import json
import logging
import urllib.request

from backend import config

logger = logging.getLogger("devtrack.dialectic_reasoner")

# ---------------------------------------------------------------------------
# Module-level constants
# ---------------------------------------------------------------------------

HERMES_MODEL = "adrienbrault/nous-hermes2pro-llama3-8b:q8_0"

# REASONING_PROMPT_TEMPLATE is used as the skeleton for every reasoning call.
# Placeholders: {interaction_type}, {context_type}, {before_text}, {after_text}.
REASONING_PROMPT_TEMPLATE = """\
You are analyzing a developer's interaction to infer writing patterns and preferences.

Interaction type: {interaction_type}
Context: {context_type}

Original text: {before_text}
Final text (after developer action): {after_text}

Based on this interaction, produce up to 3 structured inferences about the developer's \
style or preferences. Each inference must be:
- Specific and actionable (not generic)
- Grounded in the evidence above
- Expressed in one sentence

Return as JSON: {{"inferences": [{{"subject": "...", "inference": "...", "confidence": 0.0}}]}}\
"""


class DialecticReasoner:
    """
    Runs a local reasoning pass via Hermes 3 (Ollama) after each developer interaction.
    Produces structured inferences about the developer's writing style and work patterns.
    Falls back to the configured LLM chain (provider_factory) if Hermes 3 is unavailable.
    """

    def __init__(self):
        self._fallback_provider = None  # lazy-init on first use

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def reason(
        self,
        interaction_type: str,
        context_type: str,
        before_text: str,
        after_text: str,
        metadata: dict,
    ) -> list[dict]:
        """
        Returns a list of inference dicts:
        [
          {
            "subject": "commit tone",
            "inference": "Developer uses imperative mood in commit messages.",
            "confidence": 0.75
          },
          ...
        ]
        Returns [] on LLM failure (graceful degradation — never raises).

        Args:
            interaction_type: "commit" | "approval" | "rejection" | "edit"
            context_type:     "commit" | "comment" | "report" | "task" | "ticket_mapping"
            before_text:      original generated text (empty for approvals)
            after_text:       final text after edit (same as before for non-edits)
            metadata:         ticket_id, workspace, action_id, etc.
        """
        try:
            prompt = REASONING_PROMPT_TEMPLATE.format(
                interaction_type=interaction_type,
                context_type=context_type,
                before_text=before_text or "(none)",
                after_text=after_text or "(none)",
            )

            raw_json = self._call_hermes3(prompt)
            if raw_json is None:
                logger.info(
                    "dialectic: Hermes 3 unavailable — falling back to configured LLM chain"
                )
                raw_json = self._call_fallback(prompt)

            if raw_json is None:
                logger.warning(
                    "dialectic: both Hermes 3 and fallback LLM failed — returning []"
                )
                return []

            return self._parse_inferences(raw_json)

        except Exception as exc:
            logger.warning("dialectic: reason() raised unexpectedly — returning []: %s", exc)
            return []

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _hermes3_available(self) -> bool:
        """Check whether the Hermes 3 model is listed in GET {OLLAMA_HOST}/api/tags."""
        ollama_host = self._ollama_host()
        if not ollama_host:
            return False
        try:
            url = f"{ollama_host}/api/tags"
            with urllib.request.urlopen(url, timeout=3) as resp:
                data = json.loads(resp.read().decode())
            models = data.get("models", [])
            return any(
                m.get("name", "") == HERMES_MODEL or m.get("model", "") == HERMES_MODEL
                for m in models
            )
        except Exception as exc:
            logger.debug("dialectic: Ollama tags check failed: %s", exc)
            return False

    def _call_hermes3(self, prompt: str) -> str | None:
        """
        Call Hermes 3 directly via Ollama /api/generate with format=json.
        Returns the raw JSON string or None on any failure.
        """
        if not self._hermes3_available():
            return None

        ollama_host = self._ollama_host()
        timeout = config.get_int("LLM_REQUEST_TIMEOUT_SECS", 60)

        payload = json.dumps({
            "model": HERMES_MODEL,
            "prompt": prompt,
            "format": "json",
            "stream": False,
        }).encode()

        try:
            req = urllib.request.Request(
                f"{ollama_host}/api/generate",
                data=payload,
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                result = json.loads(resp.read().decode())
            return result.get("response", "")
        except Exception as exc:
            logger.warning("dialectic: Hermes 3 generate call failed: %s", exc)
            return None

    def _call_fallback(self, prompt: str) -> str | None:
        """
        Call the configured LLM chain via provider_factory with JSON output requested.
        Returns the raw text/JSON string or None on any failure.
        """
        try:
            if self._fallback_provider is None:
                from backend.llm import get_provider
                self._fallback_provider = get_provider()

            timeout = config.get_int("LLM_REQUEST_TIMEOUT_SECS", 60)

            from backend.llm.base import LLMOptions
            result = self._fallback_provider.generate(
                prompt=prompt,
                options=LLMOptions(temperature=0.3, max_tokens=512),
                timeout=timeout,
            )
            return result
        except Exception as exc:
            logger.warning("dialectic: fallback LLM call failed: %s", exc)
            return None

    @staticmethod
    def _parse_inferences(raw: str) -> list[dict]:
        """
        Parse the LLM JSON response into a list of inference dicts.
        Expected shape: {"inferences": [{"subject": "...", "inference": "...", "confidence": 0.0}]}
        Returns [] on any parse failure.
        """
        if not raw:
            return []
        try:
            data = json.loads(raw)
        except json.JSONDecodeError:
            # Model may have returned prose wrapping JSON — try to extract the first {...}.
            try:
                start = raw.index("{")
                end = raw.rindex("}") + 1
                data = json.loads(raw[start:end])
            except (ValueError, json.JSONDecodeError) as exc:
                logger.warning("dialectic: cannot parse LLM response as JSON: %s", exc)
                return []

        items = data.get("inferences", [])
        if not isinstance(items, list):
            return []

        result = []
        for item in items:
            if not isinstance(item, dict):
                continue
            subject = item.get("subject", "")
            inference = item.get("inference", "")
            try:
                confidence = float(item.get("confidence", 0.5))
            except (TypeError, ValueError):
                confidence = 0.5
            # Clamp confidence to [0.0, 1.0].
            confidence = max(0.0, min(1.0, confidence))
            if subject and inference:
                result.append({
                    "subject": subject,
                    "inference": inference,
                    "confidence": confidence,
                })
        return result

    @staticmethod
    def _ollama_host() -> str:
        """Return the normalised OLLAMA_HOST value (never 0.0.0.0)."""
        host = config.get("OLLAMA_HOST", "").rstrip("/")
        if host in ("http://0.0.0.0", "0.0.0.0"):
            host = "http://localhost:11434"
        return host
