"""
LLM-powered project spec generator.

Takes requirements text, team composition, and workload snapshot → generates a
structured YAML spec (features, stories, sprints, capacity analysis, risks).

Also supports iterative revision from PM feedback.

All methods are async. LLM is accessed via get_provider() with graceful fallback.
"""

import json
import logging
import uuid
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Dict, List, Optional

import yaml

from backend.llm.base import LLMOptions
from backend.llm.provider_factory import get_provider

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Data model
# ---------------------------------------------------------------------------

@dataclass
class ProjectSpec:
    """In-memory representation of a project spec."""

    spec_id: str
    status: str                         # draft | pending_review | approved | in_progress | completed
    pm_platform: str
    pm_email: str
    created_at: str
    review_url: str
    project: Dict[str, Any]            # name, deadline, goals, non_functional
    team: Dict[str, Any]               # developers list with skills/capacity
    workload_snapshot: Dict[str, Any]  # pulled_at + per-developer computed data
    features: List[Dict[str, Any]]     # features → stories → story_points, sprint, etc.
    sprints: List[Dict[str, Any]]      # sprint schedule
    capacity_analysis: Dict[str, Any]  # risks, on_track, buffer_days
    approval: Dict[str, Any]           # status, iterations

    def to_dict(self) -> Dict[str, Any]:
        return {
            "spec_meta": {
                "version": 1,
                "spec_id": self.spec_id,
                "status": self.status,
                "pm_platform": self.pm_platform,
                "pm_email": self.pm_email,
                "created_at": self.created_at,
                "review_url": self.review_url,
            },
            "project": self.project,
            "team": self.team,
            "workload_snapshot": self.workload_snapshot,
            "features": self.features,
            "sprints": self.sprints,
            "capacity_analysis": self.capacity_analysis,
            "approval": self.approval,
        }

    def to_yaml(self) -> str:
        return yaml.dump(self.to_dict(), default_flow_style=False, allow_unicode=True, sort_keys=False)

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "ProjectSpec":
        meta = d.get("spec_meta", {})
        return cls(
            spec_id=meta.get("spec_id", str(uuid.uuid4())),
            status=meta.get("status", "draft"),
            pm_platform=meta.get("pm_platform", ""),
            pm_email=meta.get("pm_email", ""),
            created_at=meta.get("created_at", datetime.utcnow().isoformat() + "Z"),
            review_url=meta.get("review_url", ""),
            project=d.get("project", {}),
            team=d.get("team", {}),
            workload_snapshot=d.get("workload_snapshot", {}),
            features=d.get("features", []),
            sprints=d.get("sprints", []),
            capacity_analysis=d.get("capacity_analysis", {}),
            approval=d.get("approval", {"status": "pending", "iterations": []}),
        )

    @classmethod
    def from_yaml(cls, yaml_str: str) -> "ProjectSpec":
        d = yaml.safe_load(yaml_str)
        return cls.from_dict(d)


# ---------------------------------------------------------------------------
# Generator
# ---------------------------------------------------------------------------

_SYSTEM_PROMPT = """\
You are a senior software architect and agile coach generating a detailed project plan.
Output ONLY valid JSON matching the exact schema provided — no prose, no markdown, no code fences.
Be realistic about timelines and capacity. Surface genuine risks clearly.
"""

_GENERATE_PROMPT = """\
Generate a complete project spec as JSON with this exact structure:

{{
  "project": {{
    "name": "<derive from requirements>",
    "deadline": "{deadline}",
    "goals": ["<goal 1>", "..."],
    "non_functional": {{"performance": "...", "security": "..."}}
  }},
  "features": [
    {{
      "id": "F1",
      "title": "...",
      "description": "...",
      "skill_required": ["backend", "..."],
      "stories": [
        {{
          "id": "S1",
          "title": "...",
          "story_points": 5,
          "assigned_to": "<developer name or null>",
          "sprint": 1,
          "depends_on": [],
          "acceptance_criteria": ["..."]
        }}
      ]
    }}
  ],
  "sprints": [
    {{
      "number": 1,
      "name": "Sprint 1 — <theme>",
      "start": "<YYYY-MM-DD>",
      "end": "<YYYY-MM-DD>",
      "goal": "...",
      "stories": ["S1", "S2"]
    }}
  ],
  "capacity_analysis": {{
    "total_available_days": {{{capacity_json}}},
    "estimated_effort_days": <number>,
    "on_track": <true|false>,
    "buffer_days": <number>,
    "risks": [
      {{
        "type": "capacity|skill_gap|dependency|timeline",
        "severity": "low|medium|high",
        "message": "...",
        "recommendation": "..."
      }}
    ]
  }},
  "approval": {{
    "status": "pending",
    "iterations": []
  }}
}}

PROJECT REQUIREMENTS:
{requirements}

TEAM & SKILLS:
{team_context}

WORKLOAD (current commitments — auto-computed from {platform}):
{workload_context}

Sprint length: {sprint_days} working days.
Today: {today}.
Project deadline: {deadline}.

Rules:
- Assign stories to developers whose skills match. Leave assigned_to null if unclear.
- Sprints run {sprint_days} working days each, starting from the first Monday after today.
- Highlight capacity deficits and skill gaps in risks.
- If total estimated effort > total available days, set on_track=false and explain in risks.
- story_points should reflect realistic effort (1 pt ≈ 0.5 days of focused work).
"""

_REVISE_PROMPT = """\
Here is the current project spec (YAML):

{current_yaml}

The project manager has requested the following changes:

{feedback}

Return the FULL updated spec as JSON (same schema as before, all sections present).
Apply only the requested changes; keep everything else identical.
"""


class SpecGenerator:
    """Generate and revise project specs via LLM."""

    def __init__(self, sprint_length_days: int = 10):
        self._sprint_days = sprint_length_days

    async def generate(
        self,
        requirements: str,
        developers: List[Any],          # List[Developer]
        workload: Any,                  # WorkloadSnapshot
        deadline: Optional[Any],        # date or None
        platform: str = "",
        pm_email: str = "",
        review_base_url: str = "",
    ) -> Optional[ProjectSpec]:
        """Generate a new project spec from requirements + team + workload.

        Returns a ProjectSpec or None if the LLM fails.
        """
        from datetime import date
        today = date.today()
        deadline_str = deadline.isoformat() if deadline else (today.replace(year=today.year + 1)).isoformat()

        team_context = self._format_team(developers)
        workload_context = self._format_workload(workload)
        capacity_json = ", ".join(
            f'"{d.name}": {d.available_days}'
            for d in workload.developers
        ) if workload else ""

        prompt = _GENERATE_PROMPT.format(
            requirements=requirements,
            team_context=team_context,
            workload_context=workload_context,
            capacity_json=capacity_json,
            platform=platform,
            sprint_days=self._sprint_days,
            today=today.isoformat(),
            deadline=deadline_str,
        )

        raw = await self._call_llm(prompt)
        if not raw:
            return None

        try:
            spec_data = json.loads(raw)
        except json.JSONDecodeError as e:
            logger.warning(f"SpecGenerator: LLM returned invalid JSON: {e}\nRaw: {raw[:300]}")
            return None

        spec_id = str(uuid.uuid4())
        review_url = f"{review_base_url.rstrip('/')}/spec/{spec_id}/review" if review_base_url else ""

        return ProjectSpec(
            spec_id=spec_id,
            status="draft",
            pm_platform=platform,
            pm_email=pm_email,
            created_at=datetime.utcnow().isoformat() + "Z",
            review_url=review_url,
            project=spec_data.get("project", {"name": "Untitled", "deadline": deadline_str, "goals": []}),
            team={
                "developers": [d.to_dict() for d in developers] if developers else [],
            },
            workload_snapshot=workload.to_dict() if workload else {},
            features=spec_data.get("features", []),
            sprints=spec_data.get("sprints", []),
            capacity_analysis=spec_data.get("capacity_analysis", {}),
            approval=spec_data.get("approval", {"status": "pending", "iterations": []}),
        )

    async def revise(self, spec: ProjectSpec, feedback: str) -> Optional[ProjectSpec]:
        """Apply PM feedback to an existing spec and return the updated version."""
        prompt = _REVISE_PROMPT.format(
            current_yaml=spec.to_yaml(),
            feedback=feedback,
        )
        raw = await self._call_llm(prompt)
        if not raw:
            return None
        try:
            spec_data = json.loads(raw)
        except json.JSONDecodeError as e:
            logger.warning(f"SpecGenerator.revise: invalid JSON: {e}")
            return None

        # Preserve metadata; update content sections
        updated = ProjectSpec.from_dict({
            "spec_meta": {
                "version": 1,
                "spec_id": spec.spec_id,
                "status": "pending_review",
                "pm_platform": spec.pm_platform,
                "pm_email": spec.pm_email,
                "created_at": spec.created_at,
                "review_url": spec.review_url,
            },
            **spec_data,
        })
        # Record revision
        updated.approval.setdefault("iterations", []).append({
            "changed_at": datetime.utcnow().isoformat() + "Z",
            "changed_by": spec.pm_email or "pm",
            "summary": feedback[:200],
        })
        return updated

    # -- LLM call -----------------------------------------------------------

    async def _call_llm(self, prompt: str) -> Optional[str]:
        """Synchronous LLM call wrapped for async context."""
        import asyncio
        opts = LLMOptions(temperature=0.2, max_tokens=4000)
        provider = get_provider()
        try:
            from backend.config import get_int
            timeout = get_int("LLM_REQUEST_TIMEOUT_SECS", 120)
        except Exception:
            timeout = 120
        result = await asyncio.to_thread(
            provider.generate,
            f"{_SYSTEM_PROMPT}\n\n{prompt}",
            opts,
            timeout,
        )
        if not result:
            logger.warning("SpecGenerator: LLM returned no output")
        return result

    # -- context formatting -------------------------------------------------

    @staticmethod
    def _format_team(developers: List[Any]) -> str:
        if not developers:
            return "(no developers specified)"
        lines = []
        for d in developers:
            skills = d.skills or {}
            primary = ", ".join(skills.get("primary", [])) or "unspecified"
            secondary = ", ".join(skills.get("secondary", [])) or "none"
            lines.append(f"- {d.name} ({d.email}): primary={primary}, secondary={secondary}")
        return "\n".join(lines)

    @staticmethod
    def _format_workload(workload: Any) -> str:
        if not workload or not workload.developers:
            return "(no workload data)"
        lines = []
        for d in workload.developers:
            n_items = len(d.current_assignments)
            lines.append(
                f"- {d.name}: {n_items} active items, "
                f"committed={d.committed_days}d, available={d.available_days}d "
                f"(source: {d.capacity_source})"
            )
        return "\n".join(lines)
