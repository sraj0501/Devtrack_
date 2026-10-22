"""Reporting describes work without estimating, displaying, or exporting time spent."""
from datetime import datetime

from backend.email_reporter import ActivitySummary, DailyReport, EmailReporter
from backend.daily_report_generator import DailyReportGenerator, EnhancedReport, ReportStyle, WeeklyReport


def test_daily_and_weekly_report_formats_contain_no_duration_metrics():
    now = datetime(2026, 10, 21, 12)
    activity = ActivitySummary(timestamp=now, project="Example", ticket_id="EX-1",
                               status="completed", description="Fixed authentication", source="commit")
    daily = DailyReport(date=now, activities=[activity], projects_worked=["Example"],
                        tickets_updated=["EX-1"], completed_count=1, in_progress_count=0, blocked_count=0)
    generator = DailyReportGenerator.__new__(DailyReportGenerator)
    enhanced = EnhancedReport(base_report=daily, ai_insights=None, report_style=ReportStyle.PROFESSIONAL,
                              generated_at=now, is_ai_enhanced=False)
    weekly = WeeklyReport(week_start=now, week_end=now, daily_reports=[enhanced], total_completed=1,
                          total_in_progress=0, total_blocked=0, projects_worked=["Example"],
                          tickets_updated=["EX-1"], daily_breakdown=[{"date":"Wed", "completed":1,"activities":1}],
                          ai_insights=None, generated_at=now, is_ai_enhanced=False)
    reporter = EmailReporter()
    generator.email_reporter = reporter
    formats = [reporter.format_report_text(daily), reporter.format_report_html(daily),
               generator.format_report(enhanced, "terminal"), generator.format_report(enhanced, "html"),
               generator.format_report(enhanced, "text"), generator.format_report(enhanced, "markdown"),
               generator.format_report(enhanced, "json"), generator.format_weekly_report(weekly, "text"),
               generator.format_weekly_report(weekly, "markdown"), generator.format_weekly_report(weekly, "json"),
               generator.format_weekly_report(weekly, "html")]
    for output in formats:
        assert "hour" not in output.lower()
        assert "time_spent" not in output
        assert "duration_minutes" not in output


def test_model_schema_does_not_request_or_return_duration_estimates():
    from backend.llm_task_parser import StructuredTask, _PROMPT_TEMPLATE
    assert "time_spent" not in StructuredTask(raw_text="Fixed authentication").to_dict()
    assert "time_estimate" not in StructuredTask(raw_text="Fixed authentication").to_dict()
    assert '"time_spent"' not in _PROMPT_TEMPLATE
    assert '"time_estimate"' not in _PROMPT_TEMPLATE


def test_commit_report_without_ai_never_invokes_a_model():
    from unittest.mock import patch
    generator = DailyReportGenerator.__new__(DailyReportGenerator)
    with patch.object(generator, "_generate_ticket_narrative", side_effect=AssertionError("model called")):
        report = generator.generate_eod_narrative("2026-10-21", [
            {"ticket_id": "EX-1", "commit_message": "Fixed authentication"}
        ], include_ai=False)
    assert "Fixed authentication" in report
