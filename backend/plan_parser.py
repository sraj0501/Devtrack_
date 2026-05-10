"""
Parses structured plan markdown files into PMAgent-compatible inputs.

Expected format:
  # Plan: <title>

  ## Meta
  platform: azure
  project: MyProject

  ## Goal
  <problem statement — fed verbatim to the LLM>

  ## Epics  (optional — used as structural hints for the LLM)
  - Epic title
    - Story: story title
      - Task: task title

  ## Notes  (optional — appended as constraints/context)
  - Must comply with SOC2
  - Target: Q3 2026
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional


@dataclass
class ParsedPlan:
    title: str
    platform: Optional[str]        # from ## Meta block; None if not specified
    project: Optional[str]         # from ## Meta block; None if not specified
    goal: str                      # from ## Goal block; required
    epics_hint: str                # from ## Epics block; empty string if absent
    notes: str                     # from ## Notes block; empty string if absent
    source_file: Optional[str] = None

    def to_problem_statement(self) -> str:
        """Build a rich problem statement string to pass to PMAgent.decompose()."""
        parts = [self.goal.strip()]
        if self.epics_hint.strip():
            parts.append(f"\nProposed structure (use as a guide, not a strict template):\n{self.epics_hint.strip()}")
        if self.notes.strip():
            parts.append(f"\nConstraints and notes:\n{self.notes.strip()}")
        return "\n".join(parts)


class PlanParseError(ValueError):
    pass


def parse_plan_file(path: str | Path) -> ParsedPlan:
    """Parse a single structured plan markdown file."""
    p = Path(path)
    if not p.exists():
        raise PlanParseError(f"File not found: {path}")
    content = p.read_text(encoding="utf-8")
    plan = _parse_content(content)
    plan.source_file = str(p)
    return plan


def parse_plan_folder(folder: str | Path) -> list[ParsedPlan]:
    """Parse all .md files in a folder (non-recursive)."""
    folder_path = Path(folder)
    if not folder_path.is_dir():
        raise PlanParseError(f"Not a directory: {folder}")

    md_files = sorted(folder_path.glob("*.md"))
    if not md_files:
        raise PlanParseError(f"No .md files found in: {folder}")

    plans: list[ParsedPlan] = []
    errors: list[str] = []
    for md in md_files:
        try:
            plans.append(parse_plan_file(md))
        except PlanParseError as exc:
            errors.append(f"{md.name}: {exc}")

    if errors and not plans:
        raise PlanParseError("All files failed to parse:\n" + "\n".join(errors))

    return plans


# ---------------------------------------------------------------------------
# Internal parsing
# ---------------------------------------------------------------------------

_SECTION_RE = re.compile(r"^#{1,3}\s+(.+)$", re.MULTILINE)


def _parse_content(content: str) -> ParsedPlan:
    sections = _split_sections(content)

    # --- title ---
    title = _extract_title(sections.get("__preamble__", ""))

    # --- meta block ---
    meta = _parse_meta(sections.get("meta", ""))

    # --- goal (required) ---
    goal = sections.get("goal", "").strip()
    if not goal:
        raise PlanParseError("## Goal section is missing or empty — it is required")

    # --- epics hint (optional) ---
    epics_hint = sections.get("epics", "").strip()

    # --- notes (optional) ---
    notes = sections.get("notes", "").strip()

    return ParsedPlan(
        title=title,
        platform=meta.get("platform"),
        project=meta.get("project"),
        goal=goal,
        epics_hint=epics_hint,
        notes=notes,
    )


def _split_sections(content: str) -> dict[str, str]:
    """Split content into a dict keyed by lowercase section name.

    Only H2 (##) headings delimit sections so that the H1 document title
    stays in __preamble__ for title extraction.
    """
    result: dict[str, str] = {}
    current_key = "__preamble__"
    current_lines: list[str] = []

    for line in content.splitlines(keepends=True):
        # Match exactly H2 (##) but not H1 (#) or H3 (###)
        m = re.match(r"^#{2}\s+(.+)$", line.rstrip())
        if m:
            result[current_key] = "".join(current_lines)
            current_key = m.group(1).lower().strip()
            current_lines = []
        else:
            current_lines.append(line)

    result[current_key] = "".join(current_lines)
    return result


def _extract_title(preamble: str) -> str:
    """Pull the plan title from '# Plan: <title>' or the first heading."""
    for line in preamble.splitlines():
        m = re.match(r"^#\s+[Pp]lan:\s*(.+)$", line.strip())
        if m:
            return m.group(1).strip()
        m = re.match(r"^#\s+(.+)$", line.strip())
        if m:
            return m.group(1).strip()
    return "Untitled Plan"


def _parse_meta(meta_text: str) -> dict[str, str]:
    """Parse 'key: value' lines from the ## Meta block."""
    result: dict[str, str] = {}
    for line in meta_text.splitlines():
        line = line.strip()
        if ":" in line:
            key, _, val = line.partition(":")
            key = key.strip().lower()
            val = val.strip()
            if key and val:
                result[key] = val
    return result
