"""
BoardroomSession — orchestrates parallel persona analysis + moderator synthesis.
"""

from __future__ import annotations

import asyncio
import json
import logging
import re
from dataclasses import dataclass, field
from typing import Optional

try:
    from runtime_narrative import stage as _stage
except ImportError:
    from contextlib import contextmanager as _cm
    @_cm
    def _stage(name):  # type: ignore[misc]
        yield

from backend.boardroom.personas import (
    PERSONAS,
    Persona,
    PERSONA_PROMPT_TEMPLATE,
    MODERATOR_PROMPT_TEMPLATE,
)

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Data model
# ---------------------------------------------------------------------------

@dataclass
class PersonaResult:
    persona: Persona
    stance: str                          # APPROVE | REVISE | REJECT
    stance_reason: str
    observations: list[str]
    risks: list[str]
    recommendations: list[str]
    raw: str = ""                        # raw LLM output for debugging
    error: Optional[str] = None         # set if parsing failed


@dataclass
class SwotMatrix:
    strengths: list[str] = field(default_factory=list)
    weaknesses: list[str] = field(default_factory=list)
    opportunities: list[str] = field(default_factory=list)
    threats: list[str] = field(default_factory=list)


@dataclass
class BoardroomReport:
    plan_text: str
    persona_results: list[PersonaResult]
    pros: list[str]
    cons: list[str]
    swot: SwotMatrix
    implementation_approach: str
    open_questions: list[str]
    verdict: str                         # PROCEED | REVISE | RECONSIDER
    verdict_summary: str
    synthesis_raw: str = ""
    synthesis_error: Optional[str] = None

    @property
    def approve_count(self) -> int:
        return sum(1 for r in self.persona_results if r.stance == "APPROVE")

    @property
    def revise_count(self) -> int:
        return sum(1 for r in self.persona_results if r.stance == "REVISE")

    @property
    def reject_count(self) -> int:
        return sum(1 for r in self.persona_results if r.stance == "REJECT")


# ---------------------------------------------------------------------------
# Session
# ---------------------------------------------------------------------------

class BoardroomSession:
    """
    Runs a full boardroom review:
      1. All personas analyse the plan in parallel.
      2. The moderator synthesises their outputs into a SWOT + verdict.
    """

    def __init__(self, provider=None, timeout_secs: int = 90):
        self._provider = provider
        self._timeout = timeout_secs

    def _get_provider(self):
        if self._provider is None:
            from backend.llm import get_provider
            self._provider = get_provider()
        return self._provider

    def _llm(self, prompt: str, temperature: float = 0.3, max_tokens: int = 600) -> str:
        from backend.llm.base import LLMOptions
        opts = LLMOptions(temperature=temperature, max_tokens=max_tokens)
        result = self._get_provider().generate(prompt, opts, timeout=self._timeout)
        return result or ""

    # -- persona analysis ----------------------------------------------------

    def _run_persona(self, persona: Persona, plan_text: str) -> PersonaResult:
        prompt = PERSONA_PROMPT_TEMPLATE.format(
            name=persona.name,
            role=persona.role,
            focus=persona.focus,
            plan_text=plan_text,
        )
        with _stage(f"Persona: {persona.name}"):
            raw = self._llm(prompt, temperature=persona.temperature, max_tokens=600)
        return _parse_persona_result(persona, raw)

    async def _run_persona_async(self, persona: Persona, plan_text: str) -> PersonaResult:
        return await asyncio.to_thread(self._run_persona, persona, plan_text)

    # -- synthesis ------------------------------------------------------------

    def _run_synthesis(
        self,
        plan_text: str,
        results: list[PersonaResult],
    ) -> tuple[dict, str]:
        analyses_text = _format_analyses_for_moderator(results)
        prompt = MODERATOR_PROMPT_TEMPLATE.format(
            n=len(results),
            plan_text=plan_text,
            analyses=analyses_text,
        )
        with _stage("Moderator synthesis"):
            raw = self._llm(prompt, temperature=0.3, max_tokens=1200)
        return _parse_json_response(raw), raw

    # -- public entry point --------------------------------------------------

    async def run(
        self,
        plan_text: str,
        on_persona_done: Optional[callable] = None,
    ) -> BoardroomReport:
        """
        Run the full boardroom session asynchronously.

        on_persona_done(persona_result) is called as each persona finishes
        (useful for progress display).
        """
        # ── 1. Parallel persona calls ────────────────────────────────────────
        tasks = [
            asyncio.create_task(self._run_persona_async(p, plan_text))
            for p in PERSONAS
        ]

        persona_results: list[PersonaResult] = []
        for coro in asyncio.as_completed(tasks):
            result = await coro
            persona_results.append(result)
            if on_persona_done:
                on_persona_done(result)

        # Restore original order for display consistency
        id_order = {p.id: i for i, p in enumerate(PERSONAS)}
        persona_results.sort(key=lambda r: id_order.get(r.persona.id, 99))

        # ── 2. Synthesis ────────────────────────────────────────────────────
        synth_dict, synth_raw = await asyncio.to_thread(
            self._run_synthesis, plan_text, persona_results
        )

        swot_raw = synth_dict.get("swot", {})
        swot = SwotMatrix(
            strengths=swot_raw.get("strengths", []),
            weaknesses=swot_raw.get("weaknesses", []),
            opportunities=swot_raw.get("opportunities", []),
            threats=swot_raw.get("threats", []),
        )

        return BoardroomReport(
            plan_text=plan_text,
            persona_results=persona_results,
            pros=synth_dict.get("pros", []),
            cons=synth_dict.get("cons", []),
            swot=swot,
            implementation_approach=synth_dict.get("implementation_approach", ""),
            open_questions=synth_dict.get("open_questions", []),
            verdict=synth_dict.get("verdict", "REVISE"),
            verdict_summary=synth_dict.get("verdict_summary", ""),
            synthesis_raw=synth_raw,
            synthesis_error=synth_dict.get("_parse_error"),
        )


# ---------------------------------------------------------------------------
# Parsing helpers
# ---------------------------------------------------------------------------

def _parse_json_response(raw: str) -> dict:
    """Extract and parse JSON from an LLM response, tolerating prose wrapping."""
    text = raw.strip()

    # Strip markdown code fences if present
    text = re.sub(r"^```(?:json)?\s*", "", text)
    text = re.sub(r"\s*```$", "", text)
    text = text.strip()

    # Find the outermost {...} block
    start = text.find("{")
    end = text.rfind("}")
    if start != -1 and end != -1 and end > start:
        text = text[start : end + 1]

    try:
        return json.loads(text)
    except json.JSONDecodeError as exc:
        logger.warning(f"JSON parse error: {exc} — raw: {raw[:200]}")
        return {"_parse_error": str(exc), "_raw": raw}


def _parse_persona_result(persona: Persona, raw: str) -> PersonaResult:
    data = _parse_json_response(raw)
    if "_parse_error" in data:
        return PersonaResult(
            persona=persona,
            stance="REVISE",
            stance_reason="(parse error)",
            observations=[],
            risks=[],
            recommendations=[],
            raw=raw,
            error=data["_parse_error"],
        )
    return PersonaResult(
        persona=persona,
        stance=data.get("stance", "REVISE").upper(),
        stance_reason=data.get("stance_reason", ""),
        observations=data.get("observations", []),
        risks=data.get("risks", []),
        recommendations=data.get("recommendations", []),
        raw=raw,
    )


def _format_analyses_for_moderator(results: list[PersonaResult]) -> str:
    lines = []
    for r in results:
        lines.append(f"### {r.persona.name} ({r.persona.role}) — {r.stance}")
        lines.append(f"Stance reason: {r.stance_reason}")
        if r.observations:
            lines.append("Observations: " + "; ".join(r.observations))
        if r.risks:
            lines.append("Risks: " + "; ".join(r.risks))
        if r.recommendations:
            lines.append("Recommendations: " + "; ".join(r.recommendations))
        lines.append("")
    return "\n".join(lines)
