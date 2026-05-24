"""
Boardroom report formatters — terminal (ANSI) and markdown.
"""

from __future__ import annotations

from backend.boardroom.session import BoardroomReport, PersonaResult

# Verdict colour codes (terminal)
_VERDICT_COLOUR = {
    "PROCEED":    "\033[92m",   # green
    "REVISE":     "\033[93m",   # yellow
    "RECONSIDER": "\033[91m",   # red
}
_STANCE_COLOUR = {
    "APPROVE": "\033[92m",
    "REVISE":  "\033[93m",
    "REJECT":  "\033[91m",
}
_RESET = "\033[0m"
_BOLD  = "\033[1m"


def format_terminal(report: BoardroomReport) -> str:
    """Return a formatted terminal string (with ANSI colours)."""
    lines: list[str] = []

    def h1(text: str) -> None:
        lines.append(f"\n{_BOLD}{'═' * 60}{_RESET}")
        lines.append(f"{_BOLD}  {text}{_RESET}")
        lines.append(f"{_BOLD}{'═' * 60}{_RESET}")

    def h2(text: str) -> None:
        lines.append(f"\n{_BOLD}{text}{_RESET}")
        lines.append("─" * 50)

    def bullet(items: list[str], prefix: str = "  • ") -> None:
        for item in items:
            lines.append(f"{prefix}{item}")

    h1("BOARDROOM REVIEW")

    # ── Persona vote tally ──────────────────────────────────────────────────
    h2("Votes")
    approve = _colour("APPROVE", f"APPROVE: {report.approve_count}")
    revise  = _colour("REVISE",  f"REVISE:  {report.revise_count}")
    reject  = _colour("REJECT",  f"REJECT:  {report.reject_count}")
    lines.append(f"  {approve}   {revise}   {reject}")

    # ── Per-persona analysis ────────────────────────────────────────────────
    h2("Expert Analyses")
    for r in report.persona_results:
        stance_str = _colour(r.stance, f"[{r.stance}]")
        lines.append(f"\n  {_BOLD}{r.persona.name}{_RESET} {stance_str}")
        lines.append(f"  {r.stance_reason}")
        if r.observations:
            lines.append("  Observations:")
            bullet(r.observations, "    - ")
        if r.risks:
            lines.append("  Risks:")
            bullet(r.risks, "    ⚠ ")
        if r.recommendations:
            lines.append("  Recommendations:")
            bullet(r.recommendations, "    → ")
        if r.error:
            lines.append(f"  (parse error: {r.error})")

    # ── PROs / CONs ─────────────────────────────────────────────────────────
    h2("PROs")
    bullet(report.pros or ["(none identified)"])

    h2("CONs")
    bullet(report.cons or ["(none identified)"])

    # ── SWOT ─────────────────────────────────────────────────────────────────
    h2("SWOT Analysis")
    s = report.swot
    for label, items in [
        ("Strengths",     s.strengths),
        ("Weaknesses",    s.weaknesses),
        ("Opportunities", s.opportunities),
        ("Threats",       s.threats),
    ]:
        lines.append(f"  {_BOLD}{label}{_RESET}")
        bullet(items or ["—"], "    • ")

    # ── Implementation approach ──────────────────────────────────────────────
    h2("Recommended Implementation Approach")
    if report.implementation_approach:
        for para in report.implementation_approach.split("\n"):
            lines.append(f"  {para}")

    # ── Open questions ───────────────────────────────────────────────────────
    if report.open_questions:
        h2("Open Questions / Blockers")
        bullet(report.open_questions, "  ? ")

    # ── Verdict ──────────────────────────────────────────────────────────────
    h2("Verdict")
    verdict_col = _VERDICT_COLOUR.get(report.verdict, "")
    lines.append(f"  {_BOLD}{verdict_col}{report.verdict}{_RESET}")
    if report.verdict_summary:
        lines.append(f"\n  {report.verdict_summary}")

    lines.append("")
    return "\n".join(lines)


def format_markdown(report: BoardroomReport) -> str:
    """Return a full markdown report suitable for saving to a file."""
    lines: list[str] = []

    lines.append("# Boardroom Review\n")

    # Vote tally
    lines.append("## Votes\n")
    lines.append(f"| APPROVE | REVISE | REJECT |")
    lines.append(f"|:-------:|:------:|:------:|")
    lines.append(f"| {report.approve_count} | {report.revise_count} | {report.reject_count} |\n")

    # Per-persona
    lines.append("## Expert Analyses\n")
    for r in report.persona_results:
        lines.append(f"### {r.persona.name} — `{r.stance}`\n")
        lines.append(f"*{r.stance_reason}*\n")
        if r.observations:
            lines.append("**Observations**")
            for o in r.observations:
                lines.append(f"- {o}")
            lines.append("")
        if r.risks:
            lines.append("**Risks**")
            for risk in r.risks:
                lines.append(f"- ⚠ {risk}")
            lines.append("")
        if r.recommendations:
            lines.append("**Recommendations**")
            for rec in r.recommendations:
                lines.append(f"- → {rec}")
            lines.append("")

    # PROs / CONs
    lines.append("## PROs\n")
    for p in report.pros:
        lines.append(f"- {p}")
    lines.append("")

    lines.append("## CONs\n")
    for c in report.cons:
        lines.append(f"- {c}")
    lines.append("")

    # SWOT
    lines.append("## SWOT Analysis\n")
    s = report.swot
    for label, items in [
        ("Strengths",     s.strengths),
        ("Weaknesses",    s.weaknesses),
        ("Opportunities", s.opportunities),
        ("Threats",       s.threats),
    ]:
        lines.append(f"### {label}\n")
        for item in items:
            lines.append(f"- {item}")
        lines.append("")

    # Implementation approach
    lines.append("## Recommended Implementation Approach\n")
    lines.append(report.implementation_approach or "_No recommendation generated._")
    lines.append("")

    # Open questions
    if report.open_questions:
        lines.append("## Open Questions / Blockers\n")
        for q in report.open_questions:
            lines.append(f"- {q}")
        lines.append("")

    # Verdict
    lines.append("## Verdict\n")
    lines.append(f"**{report.verdict}**\n")
    lines.append(report.verdict_summary or "")
    lines.append("")

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _colour(key: str, text: str) -> str:
    col = _STANCE_COLOUR.get(key, "")
    return f"{col}{text}{_RESET}" if col else text
