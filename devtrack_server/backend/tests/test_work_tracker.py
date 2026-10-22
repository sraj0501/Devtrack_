"""The legacy EOD adapter reports commits without work-duration data."""
from unittest.mock import patch
import pytest

@pytest.mark.asyncio
async def test_commit_report_adapter_has_no_time_metrics():
    from backend.work_tracker import EODReportGenerator
    from backend.work_tracker.eod_emailer import _build_html
    with patch("backend.daily_report_generator.DailyReportGenerator.generate_eod_narrative", return_value="EOD Report — 2026-10-21\nFixed authentication") as generate:
        report = await EODReportGenerator().generate("2026-10-21")
    generate.assert_called_once_with("2026-10-21")
    assert "Fixed authentication" in report.as_text()
    assert "Fixed authentication" in _build_html(report)
    assert not hasattr(report, "total_minutes")
    assert not hasattr(report, "sessions")
