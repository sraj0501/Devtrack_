# DevTrack Advanced Features

## AI-Enhanced Daily Reports

**Basic reports** (via `devtrack preview-report`, `save-report`, `send-report`):
- Use `backend/email_reporter.py` – reads from SQLite `task_updates`, formats as text/HTML.

**AI-enhanced reports** (via Python bridge):
- Use `backend/daily_report_generator.py` – Ollama-powered insights (executive summary, accomplishments, recommendations, productivity score).
- Triggered when:
  - End-of-day window (default 6:00 PM ± 15 min) via timer trigger
  - `REPORT_TRIGGER` IPC message (e.g. from future CLI)

To run AI-enhanced report directly:
```bash
uv run python -c "
from backend.daily_report_generator import DailyReportGenerator
gen = DailyReportGenerator()
report = gen.generate_report(include_ai=True)
print(gen.format_report(report, 'terminal'))
"
```

## Analytics (devtrack stats / db-stats)

`devtrack stats` (alias for `db-stats`) shows:
- Total triggers, responses, task updates
- Triggers today and this week
- Top projects by update count (last 30 days)
- Unsynced updates, log entries

## Future Work

- **Dashboard**: Web UI for analytics (planned)
- **Mobile notifications**: Push on triggers (planned)
- **Plugin system**: Extensible integrations (planned)
