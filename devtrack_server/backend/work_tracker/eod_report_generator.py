"""Commit-based EOD report adapter. No work-duration collection or inference."""
import argparse
import asyncio
from dataclasses import dataclass
from datetime import date
from typing import Optional


@dataclass
class EODReport:
    date: str
    narrative: str

    def as_text(self) -> str:
        return self.narrative


class EODReportGenerator:
    def __init__(self, include_ai: bool = True) -> None:
        self._include_ai = include_ai

    async def generate(self, target_date: Optional[str] = None) -> EODReport:
        from backend.daily_report_generator import DailyReportGenerator
        target_date = target_date or date.today().isoformat()
        generator = DailyReportGenerator()
        if self._include_ai:
            narrative = await asyncio.to_thread(generator.generate_eod_narrative, target_date)
        else:
            narrative = await asyncio.to_thread(
                generator.generate_eod_narrative, target_date, include_ai=False
            )
        return EODReport(date=target_date, narrative=narrative)


async def _main() -> None:
    parser = argparse.ArgumentParser(description="Generate a commit-based EOD report")
    parser.add_argument("--date", default=None)
    parser.add_argument("--no-ai", action="store_true")
    parser.add_argument("--email", default=None)
    args = parser.parse_args()
    report = await EODReportGenerator(include_ai=not args.no_ai).generate(args.date)
    print(report.as_text())
    if args.email:
        from backend.work_tracker.eod_emailer import EODEmailer
        await EODEmailer().send(report, args.email)


if __name__ == "__main__":
    asyncio.run(_main())
