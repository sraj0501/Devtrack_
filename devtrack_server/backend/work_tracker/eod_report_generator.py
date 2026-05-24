"""
End-of-Day report generator.

Usage (CLI entry point):
    python -m backend.work_tracker.eod_report_generator
    python -m backend.work_tracker.eod_report_generator --date 2026-03-28
    python -m backend.work_tracker.eod_report_generator --email me@org.com

Environment variables:
    EOD_REPORT_EMAIL   — default recipient for --email flag (optional)
"""

import argparse
import asyncio
import json
import logging
import sys
from dataclasses import dataclass, field
from datetime import datetime, date
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)


@dataclass
class SessionSummary:
    session_id: int
    ticket_ref: str
    duration_minutes: int          # effective (adjusted if set, else auto)
    adjusted: bool                 # True when developer overrode the time
    commits: List[str]
    started_at: str
    ended_at: Optional[str]
    description: str


@dataclass
class EODReport:
    date: str
    sessions: List[SessionSummary] = field(default_factory=list)
    total_minutes: int = 0
    ai_narrative: Optional[str] = None
    achievements: List[str] = field(default_factory=list)
    in_progress: List[str] = field(default_factory=list)
    pending_tomorrow: List[str] = field(default_factory=list)

    # ------------------------------------------------------------------
    # Formatting helpers
    # ------------------------------------------------------------------

    def total_str(self) -> str:
        h, m = divmod(self.total_minutes, 60)
        return f"{h}h {m}m" if h else f"{m}m"

    def as_text(self) -> str:
        lines = [f"EOD Report — {self.date}", "=" * 40]
        if self.sessions:
            lines.append("\nSessions:")
            for s in self.sessions:
                h, m = divmod(s.duration_minutes, 60)
                dur = f"{h}h {m}m" if h else f"{m}m"
                adj = " [adjusted]" if s.adjusted else ""
                ticket = s.ticket_ref or "(no ticket)"
                commits_str = f", {len(s.commits)} commit(s)" if s.commits else ""
                lines.append(f"  • {dur}{adj}  {ticket}{commits_str}")
        lines.append(f"\nTotal time: {self.total_str()}")
        if self.ai_narrative:
            lines.append(f"\nSummary:\n{self.ai_narrative}")
        if self.achievements:
            lines.append("\nAchievements:")
            for a in self.achievements:
                lines.append(f"  ✅ {a}")
        if self.in_progress:
            lines.append("\nIn Progress:")
            for p in self.in_progress:
                lines.append(f"  🔄 {p}")
        if self.pending_tomorrow:
            lines.append("\nPending Tomorrow:")
            for p in self.pending_tomorrow:
                lines.append(f"  ⏳ {p}")
        return "\n".join(lines)


class EODReportGenerator:
    """Build an EOD report for a given date from work_sessions data."""

    def __init__(self, include_ai: bool = True) -> None:
        self._include_ai = include_ai

    async def generate(self, target_date: Optional[str] = None) -> EODReport:
        """Generate the report.

        Args:
            target_date: ISO date string (YYYY-MM-DD). Defaults to today.

        Returns:
            Populated EODReport.
        """
        if target_date is None:
            target_date = date.today().isoformat()

        from backend.work_tracker.session_store import WorkSessionStore
        store = WorkSessionStore()
        raw_sessions = await asyncio.to_thread(store.get_sessions_for_date, target_date)

        sessions: List[SessionSummary] = []
        total_minutes = 0

        for s in raw_sessions:
            dur = WorkSessionStore.effective_duration(s)
            commits: List[str] = []
            try:
                commits = json.loads(s.get("commits") or "[]")
            except json.JSONDecodeError:
                pass

            summary = SessionSummary(
                session_id=s["id"],
                ticket_ref=s.get("ticket_ref") or "",
                duration_minutes=dur,
                adjusted=s.get("adjusted_minutes") is not None,
                commits=commits,
                started_at=s.get("started_at") or "",
                ended_at=s.get("ended_at"),
                description=s.get("description") or "",
            )
            sessions.append(summary)
            total_minutes += dur

        report = EODReport(
            date=target_date,
            sessions=sessions,
            total_minutes=total_minutes,
        )

        if self._include_ai and sessions:
            await self._enrich_with_ai(report)

        return report

    async def _enrich_with_ai(self, report: EODReport) -> None:
        """Populate narrative, achievements, in_progress, pending_tomorrow via LLM."""
        try:
            from backend.llm.provider_factory import get_provider
            provider = get_provider()

            session_lines = []
            for s in report.sessions:
                h, m = divmod(s.duration_minutes, 60)
                dur = f"{h}h {m}m" if h else f"{m}m"
                ticket = s.ticket_ref or "(general work)"
                commits_info = f", {len(s.commits)} commit(s)" if s.commits else ""
                session_lines.append(f"- {dur} on {ticket}{commits_info}")

            prompt = (
                f"You are a developer assistant. Based on the following work sessions for {report.date}:\n\n"
                + "\n".join(session_lines)
                + "\n\nProvide a JSON object with these keys:\n"
                '  "narrative": "2-3 sentence summary of the day\'s work",\n'
                '  "achievements": ["completed item 1", "completed item 2"],\n'
                '  "in_progress": ["still working on X"],\n'
                '  "pending_tomorrow": ["plan to do Y tomorrow"]\n'
                "\nRespond with only valid JSON."
            )

            raw = await asyncio.to_thread(
                provider.generate,
                prompt,
                {"max_tokens": 400, "temperature": 0.3},
                30,
            )

            if raw:
                # Strip markdown code fences if present
                text = raw.strip()
                if text.startswith("```"):
                    text = text.split("\n", 1)[-1].rsplit("```", 1)[0].strip()
                data = json.loads(text)
                report.ai_narrative = data.get("narrative")
                report.achievements = data.get("achievements") or []
                report.in_progress = data.get("in_progress") or []
                report.pending_tomorrow = data.get("pending_tomorrow") or []
        except Exception as e:
            logger.debug(f"EOD AI enrichment failed (non-fatal): {e}")


# ---------------------------------------------------------------------------
# CLI entry point: python -m backend.work_tracker.eod_report_generator
# ---------------------------------------------------------------------------

async def _main() -> None:
    parser = argparse.ArgumentParser(description="Generate EOD work report")
    parser.add_argument("--date", default=None, help="Date (YYYY-MM-DD), default today")
    parser.add_argument("--email", default=None, help="Send report to this email address")
    parser.add_argument("--no-ai", action="store_true", help="Skip AI enrichment")
    args = parser.parse_args()

    import os
    import sys

    _script_dir = os.path.dirname(os.path.abspath(__file__))
    _project_root = os.path.dirname(os.path.dirname(_script_dir))
    if _project_root not in sys.path:
        sys.path.insert(0, _project_root)

    try:
        from backend import config as _cfg
        _cfg._load_env()
    except Exception:
        pass

    logging.basicConfig(level=logging.WARNING, format="%(levelname)s: %(message)s")

    generator = EODReportGenerator(include_ai=not args.no_ai)
    report = await generator.generate(args.date)

    print(report.as_text())

    if args.email:
        from backend.work_tracker.eod_emailer import EODEmailer
        emailer = EODEmailer()
        sent = await emailer.send(report, args.email)
        if sent:
            print(f"\n✅ Report emailed to {args.email}")
        else:
            print(f"\n⚠️  Could not send email to {args.email} — check logs.")


if __name__ == "__main__":
    asyncio.run(_main())
