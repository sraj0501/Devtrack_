"""
Work session tracking and EOD report generation.

Public exports:
    WorkSessionStore       — async SQLite interface for work sessions
    EODReportGenerator     — builds an EOD report for a given date
    EODEmailer             — sends the EOD report via MS Graph email
"""

from backend.work_tracker.session_store import WorkSessionStore
from backend.work_tracker.eod_report_generator import EODReportGenerator, EODReport
from backend.work_tracker.eod_emailer import EODEmailer

__all__ = [
    "WorkSessionStore",
    "EODReportGenerator",
    "EODReport",
    "EODEmailer",
]
