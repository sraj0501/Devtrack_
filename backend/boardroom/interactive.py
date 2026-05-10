"""
Interactive boardroom session.

Manages multi-turn conversation between the user and 7 AI personas.
Each turn:
  1. A lightweight moderator call selects which persona(s) should respond.
  2. Selected personas generate in-character replies.
  3. History is returned to the caller so the Go CLI can persist it between HTTP calls.

This module is stateless — all state (history, plan_text) is passed in and
returned on every call so the webhook endpoint stays stateless too.
"""

from __future__ import annotations

import json
import logging
import re
from typing import Optional

from backend.boardroom.personas import PERSONAS, Persona
from backend.llm.base import LLMOptions

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Data types (plain dicts for easy JSON serialisation over HTTP)
# ---------------------------------------------------------------------------

# History entry shape:
#   {"role": "user",    "content": "..."}
#   {"role": "persona", "persona_id": "...", "persona_name": "...", "content": "..."}
#   {"role": "system",  "content": "..."}   ← used for the initial boardroom summary

HistoryEntry = dict  # typed alias for readability

# ---------------------------------------------------------------------------
# Persona personality snippets — make each voice distinct
# ---------------------------------------------------------------------------

_PERSONA_VOICE: dict[str, str] = {
    "architect": (
        "You are precise and structured. You draw diagrams in words. "
        "You ask about component boundaries, data flows, and contract definitions. "
        "You are collegial but will firmly push back on design choices you consider unsound."
    ),
    "security": (
        "You are measured and methodical. You never catastrophise — "
        "you quantify risk. You ask about threat models, auth flows, and data classification. "
        "When you disagree, you cite real breach patterns, not hypotheticals."
    ),
    "pm": (
        "You are user-obsessed and timeline-aware. You think in epics and sprints. "
        "You push back on scope creep and ask 'what does the user actually need?' "
        "You're optimistic but realistic about delivery."
    ),
    "devil": (
        "You are intellectually playful and deliberately provocative. "
        "You enjoy puncturing assumptions. You ask uncomfortable 'what if' questions. "
        "You don't enjoy being wrong but you admit it gracefully when you are."
    ),
    "engineer": (
        "You are practical and implementation-focused. You think in sprint points and "
        "test coverage. You appreciate elegant solutions and hate tech debt. "
        "You'll call out unrealistic timelines directly."
    ),
    "analyst": (
        "You think in numbers, ROI, and business outcomes. You like KPIs and metrics. "
        "You translate technical decisions into business risk and opportunity. "
        "You're not afraid to say a feature isn't worth the cost."
    ),
    "scalability": (
        "You've seen applications fail at 10x load. You are calm and evidence-driven. "
        "You probe for single points of failure, connection pool limits, and cache stampedes. "
        "You celebrate when a design is truly stateless. You are wary of premature optimisation "
        "but merciless about ignoring known scaling failure modes."
    ),
}

# ---------------------------------------------------------------------------
# Moderator: select which personas respond
# ---------------------------------------------------------------------------

_SELECT_PROMPT = """\
You are a boardroom moderator. A user is discussing a software plan with {n} expert personas.

Personas available:
{persona_list}

The conversation so far (last {tail} turns):
{history_snippet}

User's latest message: "{user_message}"
Addressed to: {addressed_to}

Select 1–3 personas who should respond. Prioritise:
- The persona most directly addressed (if any)
- Personas whose domain is most relevant to the message
- A persona who would naturally disagree or add tension to the discussion

Respond ONLY with JSON: {{"responders": ["id1", "id2"]}}
No prose, no fences.
"""


def select_responders(
    provider,
    history: list[HistoryEntry],
    user_message: str,
    addressed_to: Optional[str],
) -> list[str]:
    """Return a list of persona IDs that should respond to this message."""

    persona_list = "\n".join(
        f"  - {p.id}: {p.name} ({p.role})" for p in PERSONAS
    )
    history_snippet = _format_history_snippet(history, tail=6)

    prompt = _SELECT_PROMPT.format(
        n=len(PERSONAS),
        persona_list=persona_list,
        tail=6,
        history_snippet=history_snippet,
        user_message=user_message,
        addressed_to=addressed_to or "nobody specific (open question to the group)",
    )

    opts = LLMOptions(temperature=0.2, max_tokens=60)
    raw = provider.generate(prompt, opts, timeout=20) or ""

    data = _parse_json(raw)
    responders = data.get("responders", [])

    # Validate: only known persona IDs, cap at 3
    valid_ids = {p.id for p in PERSONAS}
    responders = [r for r in responders if r in valid_ids][:3]

    # Fall back to the addressed persona or the first two
    if not responders:
        if addressed_to and addressed_to in valid_ids:
            responders = [addressed_to]
        else:
            responders = [PERSONAS[0].id, PERSONAS[3].id]

    return responders


# ---------------------------------------------------------------------------
# Persona response generation
# ---------------------------------------------------------------------------

_PERSONA_REPLY_PROMPT = """\
You are {name}, a {role} in an active boardroom discussion about a software plan.

Your personality: {voice}

Context — the plan being discussed:
{plan_text}

Conversation history:
{history}

The user just said: "{user_message}"

Respond as {first_name} — in first person, conversational, 2–4 sentences max.
Be direct. Show your personality. You can disagree, ask a question, or build on
what another persona said. Do NOT use bullet points or headers — this is a conversation.
Do NOT introduce yourself or explain your role — just speak.
"""


def generate_persona_response(
    provider,
    persona: Persona,
    history: list[HistoryEntry],
    user_message: str,
    plan_text: str,
) -> str:
    """Generate one persona's in-character response."""
    opts = LLMOptions(temperature=persona.temperature + 0.1, max_tokens=200)
    first_name = persona.name.split()[0]
    prompt = _PERSONA_REPLY_PROMPT.format(
        name=persona.name,
        role=persona.role,
        voice=_PERSONA_VOICE.get(persona.id, ""),
        plan_text=plan_text[:800],
        history=_format_history_snippet(history, tail=10),
        user_message=user_message,
        first_name=first_name,
    )
    raw = provider.generate(prompt, opts, timeout=40) or ""
    return raw.strip()


# ---------------------------------------------------------------------------
# Final synthesis after user gives their verdict
# ---------------------------------------------------------------------------

_FINAL_PROMPT = """\
You are the boardroom moderator writing up the final minutes.

The group has just completed an active discussion of a software plan.
The project owner has given their final decision.

Original plan:
{plan_text}

Full discussion:
{history}

Project owner's final say: "{final_say}"

Write a concise closing summary (3–5 sentences) that:
1. Acknowledges the owner's decision
2. Captures the key points of consensus and disagreement from the discussion
3. Lists 2–3 concrete next steps the team agreed on (or that follow logically)

Write in a professional but human tone — this is meeting minutes, not a report.
"""


def generate_final_summary(
    provider,
    plan_text: str,
    history: list[HistoryEntry],
    final_say: str,
) -> str:
    """Generate a closing summary after the user gives their final say."""
    opts = LLMOptions(temperature=0.3, max_tokens=400)
    prompt = _FINAL_PROMPT.format(
        plan_text=plan_text[:800],
        history=_format_history_full(history),
        final_say=final_say,
    )
    raw = provider.generate(prompt, opts, timeout=60) or ""
    return raw.strip()


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _format_history_snippet(history: list[HistoryEntry], tail: int = 6) -> str:
    recent = history[-tail:] if len(history) > tail else history
    lines = []
    for entry in recent:
        role = entry.get("role", "")
        content = entry.get("content", "")
        if role == "user":
            lines.append(f"User: {content}")
        elif role == "persona":
            lines.append(f"{entry.get('persona_name', 'Persona')}: {content}")
        elif role == "system":
            lines.append(f"[Session context: {content[:100]}...]")
    return "\n".join(lines) if lines else "(no prior conversation)"


def _format_history_full(history: list[HistoryEntry]) -> str:
    lines = []
    for entry in history:
        role = entry.get("role", "")
        content = entry.get("content", "")
        if role == "user":
            lines.append(f"User: {content}")
        elif role == "persona":
            lines.append(f"{entry.get('persona_name', 'Persona')}: {content}")
    return "\n".join(lines) if lines else "(no conversation)"


def _parse_json(raw: str) -> dict:
    text = re.sub(r"^```(?:json)?\s*", "", raw.strip())
    text = re.sub(r"\s*```$", "", text).strip()
    start, end = text.find("{"), text.rfind("}")
    if start != -1 and end > start:
        text = text[start:end + 1]
    try:
        return json.loads(text)
    except Exception:
        return {}
