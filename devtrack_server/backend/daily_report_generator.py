"""
Daily Report Generator (Phase 5)

This module generates intelligent daily reports by combining:
- Activity data from the SQLite database (via EmailReporter)
- AI-powered summaries using Ollama
- TUI interactions for user feedback

Integrates with python_bridge.py for end-of-day report generation.
"""

import os
import sys
import sqlite3
from datetime import datetime, time as dt_time, timedelta
from typing import Optional, List, Dict, Any
from dataclasses import dataclass
from enum import Enum

# Add paths for imports
sys.path.insert(0, os.path.dirname(__file__))

from email_reporter import EmailReporter, DailyReport, ActivitySummary

try:
    from backend.personalization import inject_style as _inject_style
except ImportError:
    def _inject_style(prompt: str, context_type: str = "general") -> str:
        return prompt


class ReportStyle(str, Enum):
    """Report formatting styles"""
    PROFESSIONAL = "professional"
    CASUAL = "casual"
    DETAILED = "detailed"
    EXECUTIVE = "executive"


class OutputFormat(str, Enum):
    """Report output formats"""
    TERMINAL = "terminal"
    TEXT = "text"
    HTML = "html"
    MARKDOWN = "markdown"
    JSON = "json"


@dataclass
class AIInsights:
    """AI-generated insights for the report"""
    executive_summary: str
    key_accomplishments: List[str]
    ongoing_work: List[str]
    blockers_risks: List[str]
    recommendations: List[str]
    productivity_score: int  # 1-10 scale
    focus_areas: List[str]


@dataclass
class EnhancedReport:
    """Enhanced daily report with AI insights"""
    base_report: DailyReport
    ai_insights: Optional[AIInsights]
    generated_at: datetime
    report_style: ReportStyle
    is_ai_enhanced: bool


class DailyReportGenerator:
    """
    Generates intelligent daily reports with AI enhancement.
    
    Combines data collection (EmailReporter), AI analysis (Ollama),
    and presentation (TUI) for comprehensive end-of-day reports.
    """
    
    DEFAULT_END_OF_DAY_TIME = dt_time(18, 0)  # 6:00 PM
    
    def __init__(
        self,
        db_path: Optional[str] = None,
        provider=None,
        # Legacy params kept for backwards compatibility but ignored:
        ollama_url: Optional[str] = None,
        model: Optional[str] = None,
    ):
        """Initialize the daily report generator.

        Args:
            db_path: Path to SQLite database (auto-detected if None)
            provider: Optional LLM provider instance (injected for testing).
        """
        try:
            from backend.config import database_path
            self.db_path = str(db_path) if db_path else str(database_path())
        except ImportError:
            if db_path:
                self.db_path = str(db_path)
            else:
                try:
                    from backend.utils.paths import fallback_path
                    self.db_path = fallback_path("Data", "db", "devtrack.db")
                except ImportError:
                    from pathlib import Path
                    cur = Path(__file__).resolve().parent.parent
                    self.db_path = str(cur / "Data" / "db" / "devtrack.db")
        self._provider = provider  # None = lazy init on first use
        self.email_reporter = EmailReporter(self.db_path)
    
    def is_end_of_day(
        self,
        end_time: Optional[dt_time] = None,
        tolerance_minutes: int = 15
    ) -> bool:
        """
        Check if current time is within end-of-day window.
        
        Args:
            end_time: Configured end-of-day time (default: 6:00 PM)
            tolerance_minutes: Minutes before/after to consider as "end of day"
            
        Returns:
            True if within end-of-day window
        """
        if end_time is None:
            end_time = self.DEFAULT_END_OF_DAY_TIME
        
        now = datetime.now().time()
        
        # Calculate window boundaries
        end_dt = datetime.combine(datetime.today(), end_time)
        window_start = (end_dt - timedelta(minutes=tolerance_minutes)).time()
        window_end = (end_dt + timedelta(minutes=tolerance_minutes)).time()
        
        return window_start <= now <= window_end
    
    def _get_provider(self):
        if self._provider is None:
            from backend.llm import get_provider
            self._provider = get_provider()
        return self._provider

    def check_ollama_available(self) -> bool:
        """Check if the LLM provider is available and responsive."""
        try:
            return self._get_provider().primary.is_available()
        except Exception:
            return False

    
    def generate_ai_insights(self, report: DailyReport) -> Optional[AIInsights]:
        """
        Generate AI-powered insights from the daily report.
        
        Args:
            report: Base daily report with activities
            
        Returns:
            AIInsights if Ollama available, None otherwise
        """
        if not self.check_ollama_available():
            return None
        
        if not report.activities:
            return AIInsights(
                executive_summary="No activities recorded for today.",
                key_accomplishments=[],
                ongoing_work=[],
                blockers_risks=[],
                recommendations=["Consider logging work activities throughout the day."],
                productivity_score=0,
                focus_areas=[]
            )
        
        # Build activity context for AI
        activities_text = self._format_activities_for_ai(report)
        
        prompt = f"""Analyze this developer's daily work activities and provide structured insights.

ACTIVITIES:
{activities_text}

STATISTICS:
- Total Hours: {report.total_hours:.1f}
- Tasks Completed: {report.completed_count}
- Tasks In Progress: {report.in_progress_count}
- Tasks Blocked: {report.blocked_count}
- Projects Worked: {', '.join(report.projects_worked)}

Provide a JSON response with these exact fields:
{{
    "executive_summary": "2-3 sentence summary of the day's work",
    "key_accomplishments": ["list of 3-5 key accomplishments"],
    "ongoing_work": ["list of work items still in progress"],
    "blockers_risks": ["any blockers or risks identified"],
    "recommendations": ["1-3 suggestions for tomorrow"],
    "productivity_score": 7,
    "focus_areas": ["main focus areas for the day"]
}}

Keep it professional and constructive. Respond ONLY with valid JSON."""

        prompt = _inject_style(prompt, context_type="report")

        try:
            from backend.llm.base import LLMOptions
            from backend.config import http_timeout, report_llm_temperature, report_llm_max_tokens
            response_text = self._get_provider().generate(
                prompt=prompt,
                options=LLMOptions(
                    temperature=report_llm_temperature(),
                    max_tokens=report_llm_max_tokens(),
                ),
                timeout=http_timeout(),
            )

            if response_text:
                insights_data = self._parse_ai_response(response_text)
                if insights_data:
                    return AIInsights(
                        executive_summary=insights_data.get("executive_summary", ""),
                        key_accomplishments=insights_data.get("key_accomplishments", []),
                        ongoing_work=insights_data.get("ongoing_work", []),
                        blockers_risks=insights_data.get("blockers_risks", []),
                        recommendations=insights_data.get("recommendations", []),
                        productivity_score=int(insights_data.get("productivity_score", 5)),
                        focus_areas=insights_data.get("focus_areas", [])
                    )

        except Exception as e:
            print(f"AI insights generation failed: {e}")

        return None
    
    def _format_activities_for_ai(self, report: DailyReport) -> str:
        """Format activities for AI consumption."""
        lines = []
        for act in report.activities:
            time_str = act.timestamp.strftime('%I:%M %p')
            ticket = f"[{act.ticket_id}] " if act.ticket_id else ""
            hours = f" ({act.time_spent:.1f}h)" if act.time_spent > 0 else ""
            status = act.status.replace("_", " ").title()
            lines.append(f"- {time_str}: {ticket}{act.description}{hours} [{status}]")
        return "\n".join(lines)
    
    def _parse_ai_response(self, response: str) -> Optional[Dict[str, Any]]:
        """Parse JSON from AI response."""
        import json
        
        # Try direct parse first
        try:
            return json.loads(response)
        except json.JSONDecodeError:
            pass
        
        # Try to find JSON block in response
        import re
        json_match = re.search(r'\{[\s\S]*\}', response)
        if json_match:
            try:
                return json.loads(json_match.group())
            except json.JSONDecodeError:
                pass
        
        return None
    
    def generate_report(
        self,
        date: Optional[datetime] = None,
        style: ReportStyle = ReportStyle.PROFESSIONAL,
        include_ai: bool = True
    ) -> EnhancedReport:
        """
        Generate an enhanced daily report.
        
        Args:
            date: Date to generate report for (default: today)
            style: Report formatting style
            include_ai: Whether to include AI insights
            
        Returns:
            EnhancedReport with base data and optional AI insights
        """
        # Get base report from EmailReporter
        base_report = self.email_reporter.generate_daily_report(date)
        
        # Generate AI insights if requested and available
        ai_insights = None
        if include_ai and self.check_ollama_available():
            ai_insights = self.generate_ai_insights(base_report)
        
        return EnhancedReport(
            base_report=base_report,
            ai_insights=ai_insights,
            generated_at=datetime.now(),
            report_style=style,
            is_ai_enhanced=(ai_insights is not None)
        )
    
    def format_report(
        self,
        report: EnhancedReport,
        output_format: OutputFormat = OutputFormat.TERMINAL
    ) -> str:
        """
        Format the report for output.
        
        Args:
            report: Enhanced report to format
            output_format: Desired output format
            
        Returns:
            Formatted report string
        """
        if output_format == OutputFormat.TERMINAL:
            return self._format_terminal(report)
        elif output_format == OutputFormat.TEXT:
            return self.email_reporter.format_report_text(
                report.base_report, 
                report.report_style.value
            )
        elif output_format == OutputFormat.HTML:
            return self._format_html(report)
        elif output_format == OutputFormat.MARKDOWN:
            return self._format_markdown(report)
        elif output_format == OutputFormat.JSON:
            return self._format_json(report)
        else:
            return self._format_terminal(report)
    
    def _format_terminal(self, report: EnhancedReport) -> str:
        """Format report for terminal display with colors."""
        lines = []
        base = report.base_report
        date_str = base.date.strftime('%A, %B %d, %Y')
        
        # Header
        lines.append("")
        lines.append("╔══════════════════════════════════════════════════════════════════════╗")
        lines.append(f"║  📊 DAILY STATUS REPORT - {date_str:<42}║")
        lines.append("╚══════════════════════════════════════════════════════════════════════╝")
        lines.append("")
        
        # AI Enhanced badge
        if report.is_ai_enhanced:
            lines.append("  ✨ AI-Enhanced Report")
            lines.append("")
        
        # Summary section
        lines.append("  ┌─ SUMMARY ─────────────────────────────────────────────────────────┐")
        lines.append(f"  │  ⏱️  Total Hours:     {base.total_hours:.1f}h")
        lines.append(f"  │  ✅ Completed:       {base.completed_count} tasks")
        lines.append(f"  │  🔄 In Progress:     {base.in_progress_count} tasks")
        if base.blocked_count > 0:
            lines.append(f"  │  🚫 Blocked:         {base.blocked_count} tasks")
        lines.append(f"  │  📁 Projects:        {', '.join(base.projects_worked) if base.projects_worked else 'None'}")
        lines.append("  └────────────────────────────────────────────────────────────────────┘")
        lines.append("")
        
        # AI Insights section
        if report.ai_insights:
            ai = report.ai_insights
            lines.append("  ┌─ AI INSIGHTS ──────────────────────────────────────────────────────┐")
            
            # Executive summary
            if ai.executive_summary:
                lines.append(f"  │  💡 {ai.executive_summary[:65]}")
                if len(ai.executive_summary) > 65:
                    lines.append(f"  │     {ai.executive_summary[65:130]}")
            
            # Productivity score
            score_bar = "●" * ai.productivity_score + "○" * (10 - ai.productivity_score)
            lines.append(f"  │  📈 Productivity: {score_bar} {ai.productivity_score}/10")
            
            # Key accomplishments
            if ai.key_accomplishments:
                lines.append("  │")
                lines.append("  │  Key Accomplishments:")
                for acc in ai.key_accomplishments[:3]:
                    lines.append(f"  │   ✓ {acc[:60]}")
            
            # Ongoing work
            if ai.ongoing_work:
                lines.append("  │")
                lines.append("  │  Ongoing Work:")
                for work in ai.ongoing_work[:3]:
                    lines.append(f"  │   → {work[:60]}")
            
            # Blockers
            if ai.blockers_risks:
                lines.append("  │")
                lines.append("  │  ⚠️  Blockers/Risks:")
                for blocker in ai.blockers_risks[:2]:
                    lines.append(f"  │   ✗ {blocker[:60]}")
            
            # Recommendations
            if ai.recommendations:
                lines.append("  │")
                lines.append("  │  📋 Tomorrow's Recommendations:")
                for rec in ai.recommendations[:2]:
                    lines.append(f"  │   • {rec[:60]}")
            
            lines.append("  └────────────────────────────────────────────────────────────────────┘")
            lines.append("")
        
        # Activities section
        if base.activities:
            lines.append("  ┌─ ACTIVITIES ────────────────────────────────────────────────────────┐")
            
            # Group by project
            by_project: Dict[str, List[ActivitySummary]] = {}
            for act in base.activities:
                proj = act.project or "Other"
                if proj not in by_project:
                    by_project[proj] = []
                by_project[proj].append(act)
            
            for project, activities in sorted(by_project.items()):
                lines.append("  │")
                lines.append(f"  │  📂 {project}")
                for act in activities:
                    status_icon = {"completed": "✓", "in_progress": "→", "blocked": "✗"}.get(act.status, "•")
                    ticket = f"[{act.ticket_id}] " if act.ticket_id else ""
                    desc = act.description[:50] if act.description else ""
                    time_str = f" ({act.time_spent:.1f}h)" if act.time_spent > 0 else ""
                    lines.append(f"  │     {status_icon} {ticket}{desc}{time_str}")
            
            lines.append("  └────────────────────────────────────────────────────────────────────┘")
        
        # Footer
        lines.append("")
        gen_time = report.generated_at.strftime('%I:%M %p')
        ai_badge = " | AI-Enhanced" if report.is_ai_enhanced else ""
        lines.append(f"  Generated at {gen_time}{ai_badge}")
        lines.append("")
        
        return "\n".join(lines)
    
    def _format_html(self, report: EnhancedReport) -> str:
        """Format report as enhanced HTML."""
        base_html = self.email_reporter.format_report_html(report.base_report)
        
        if not report.ai_insights:
            return base_html
        
        # Insert AI insights section before activities
        ai = report.ai_insights
        ai_section = f"""
        <div class="ai-insights" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); 
            padding: 20px; border-radius: 8px; margin: 20px 0; color: white;">
            <h2 style="color: white; border-bottom: 2px solid rgba(255,255,255,0.3); padding-bottom: 10px;">
                ✨ AI Insights
            </h2>
            <p style="font-size: 16px; margin: 15px 0;">{ai.executive_summary}</p>
            
            <div style="display: flex; align-items: center; margin: 15px 0;">
                <span style="margin-right: 10px;">📈 Productivity:</span>
                <div style="background: rgba(255,255,255,0.3); border-radius: 10px; padding: 3px 10px;">
                    <span style="font-weight: bold;">{ai.productivity_score}/10</span>
                </div>
            </div>
"""
        
        if ai.key_accomplishments:
            ai_section += """
            <div style="margin: 15px 0;">
                <strong>Key Accomplishments:</strong>
                <ul style="margin: 10px 0;">
"""
            for acc in ai.key_accomplishments:
                ai_section += f"                    <li>{acc}</li>\n"
            ai_section += """                </ul>
            </div>
"""
        
        if ai.recommendations:
            ai_section += """
            <div style="margin: 15px 0;">
                <strong>📋 Tomorrow's Focus:</strong>
                <ul style="margin: 10px 0;">
"""
            for rec in ai.recommendations:
                ai_section += f"                    <li>{rec}</li>\n"
            ai_section += """                </ul>
            </div>
"""
        
        ai_section += "        </div>\n"
        
        # Insert before <h2>Activities</h2>
        return base_html.replace("<h2>Activities</h2>", ai_section + "<h2>Activities</h2>")
    
    def _format_markdown(self, report: EnhancedReport) -> str:
        """Format report as Markdown."""
        lines = []
        base = report.base_report
        date_str = base.date.strftime('%A, %B %d, %Y')
        
        lines.append(f"# Daily Status Report - {date_str}")
        lines.append("")
        
        if report.is_ai_enhanced:
            lines.append("*✨ AI-Enhanced Report*")
            lines.append("")
        
        # Summary
        lines.append("## Summary")
        lines.append("")
        lines.append(f"- **Total Hours:** {base.total_hours:.1f}h")
        lines.append(f"- **Completed:** {base.completed_count} tasks")
        lines.append(f"- **In Progress:** {base.in_progress_count} tasks")
        if base.blocked_count > 0:
            lines.append(f"- **Blocked:** {base.blocked_count} tasks")
        lines.append(f"- **Projects:** {', '.join(base.projects_worked)}")
        lines.append("")
        
        # AI Insights
        if report.ai_insights:
            ai = report.ai_insights
            lines.append("## AI Insights")
            lines.append("")
            lines.append(f"> {ai.executive_summary}")
            lines.append("")
            lines.append(f"**Productivity Score:** {ai.productivity_score}/10")
            lines.append("")
            
            if ai.key_accomplishments:
                lines.append("### Key Accomplishments")
                for acc in ai.key_accomplishments:
                    lines.append(f"- ✅ {acc}")
                lines.append("")
            
            if ai.ongoing_work:
                lines.append("### Ongoing Work")
                for work in ai.ongoing_work:
                    lines.append(f"- 🔄 {work}")
                lines.append("")
            
            if ai.blockers_risks:
                lines.append("### Blockers & Risks")
                for blocker in ai.blockers_risks:
                    lines.append(f"- ⚠️ {blocker}")
                lines.append("")
            
            if ai.recommendations:
                lines.append("### Recommendations for Tomorrow")
                for rec in ai.recommendations:
                    lines.append(f"- 📋 {rec}")
                lines.append("")
        
        # Activities
        if base.activities:
            lines.append("## Activities")
            lines.append("")
            
            by_project: Dict[str, List[ActivitySummary]] = {}
            for act in base.activities:
                proj = act.project or "Other"
                if proj not in by_project:
                    by_project[proj] = []
                by_project[proj].append(act)
            
            for project, activities in sorted(by_project.items()):
                lines.append(f"### {project}")
                for act in activities:
                    status = {"completed": "✅", "in_progress": "🔄", "blocked": "🚫"}.get(act.status, "•")
                    ticket = f"`{act.ticket_id}` " if act.ticket_id else ""
                    time_str = f" *({act.time_spent:.1f}h)*" if act.time_spent > 0 else ""
                    lines.append(f"- {status} {ticket}{act.description}{time_str}")
                lines.append("")
        
        # Footer
        gen_time = report.generated_at.strftime('%I:%M %p')
        lines.append("---")
        lines.append(f"*Generated at {gen_time}*")
        
        return "\n".join(lines)
    
    def _format_json(self, report: EnhancedReport) -> str:
        """Format report as JSON."""
        import json
        
        data = {
            "date": report.base_report.date.isoformat(),
            "generated_at": report.generated_at.isoformat(),
            "style": report.report_style.value,
            "is_ai_enhanced": report.is_ai_enhanced,
            "summary": {
                "total_hours": report.base_report.total_hours,
                "completed_count": report.base_report.completed_count,
                "in_progress_count": report.base_report.in_progress_count,
                "blocked_count": report.base_report.blocked_count,
                "projects_worked": report.base_report.projects_worked,
                "tickets_updated": report.base_report.tickets_updated
            },
            "activities": [
                {
                    "timestamp": act.timestamp.isoformat(),
                    "project": act.project,
                    "ticket_id": act.ticket_id,
                    "status": act.status,
                    "description": act.description,
                    "time_spent": act.time_spent,
                    "source": act.source
                }
                for act in report.base_report.activities
            ]
        }
        
        if report.ai_insights:
            ai = report.ai_insights
            data["ai_insights"] = {
                "executive_summary": ai.executive_summary,
                "key_accomplishments": ai.key_accomplishments,
                "ongoing_work": ai.ongoing_work,
                "blockers_risks": ai.blockers_risks,
                "recommendations": ai.recommendations,
                "productivity_score": ai.productivity_score,
                "focus_areas": ai.focus_areas
            }
        
        return json.dumps(data, indent=2)
    
    # =========================================================================
    # DATABASE STORAGE
    # =========================================================================
    
    def _init_reports_table(self) -> None:
        """Initialize the reports table if it doesn't exist."""
        try:
            conn = sqlite3.connect(self.db_path)
            cursor = conn.cursor()
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS reports (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    report_date DATE NOT NULL,
                    report_type TEXT NOT NULL,
                    format TEXT NOT NULL,
                    content TEXT NOT NULL,
                    summary TEXT,
                    total_hours REAL DEFAULT 0,
                    task_count INTEGER DEFAULT 0,
                    completed_count INTEGER DEFAULT 0,
                    projects_count INTEGER DEFAULT 0,
                    ai_enhanced BOOLEAN DEFAULT 0,
                    email_sent BOOLEAN DEFAULT 0,
                    email_sent_at DATETIME,
                    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
                )
            """)
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_reports_date ON reports(report_date)")
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_reports_type ON reports(report_type)")
            conn.commit()
            conn.close()
        except Exception as e:
            print(f"Warning: Could not initialize reports table: {e}")
    
    def _get_report_summary(self, report: EnhancedReport) -> str:
        """Generate a brief summary of the report for database storage."""
        base = report.base_report
        summary_parts = [
            f"Hours: {base.total_hours:.1f}",
            f"Tasks: {len(base.activities)}",
            f"Done: {base.completed_count}",
            f"Projects: {', '.join(base.projects_worked[:3])}"
        ]
        
        if report.ai_insights:
            summary_parts.append(f"Score: {report.ai_insights.productivity_score}/10")
        
        return " | ".join(summary_parts)
    
    def save_to_database(
        self,
        report: EnhancedReport,
        output_format: OutputFormat = OutputFormat.TEXT,
        report_type: str = "daily"
    ) -> Optional[int]:
        """
        Save report to the SQLite database.
        
        Args:
            report: Report to save
            output_format: Format of the content
            report_type: Type of report ("daily" or "weekly")
            
        Returns:
            Report ID if successful, None otherwise
        """
        try:
            self._init_reports_table()
            
            content = self.format_report(report, output_format)
            summary = self._get_report_summary(report)
            base = report.base_report
            
            conn = sqlite3.connect(self.db_path)
            cursor = conn.cursor()
            
            cursor.execute("""
                INSERT INTO reports (
                    report_date, report_type, format, content, summary,
                    total_hours, task_count, completed_count, projects_count,
                    ai_enhanced, email_sent
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """, (
                base.date.strftime('%Y-%m-%d'),
                report_type,
                output_format.value,
                content,
                summary,
                base.total_hours,
                len(base.activities),
                base.completed_count,
                len(base.projects_worked),
                report.is_ai_enhanced,
                False
            ))
            
            report_id = cursor.lastrowid
            conn.commit()
            conn.close()
            
            return report_id
        except Exception as e:
            print(f"Warning: Could not save report to database: {e}")
            return None
    
    def update_report_email_status(self, report_id: int, sent: bool = True) -> bool:
        """
        Update the email sent status for a report.
        
        Args:
            report_id: ID of the report to update
            sent: Whether email was sent
            
        Returns:
            True if successful
        """
        try:
            conn = sqlite3.connect(self.db_path)
            cursor = conn.cursor()
            
            if sent:
                cursor.execute("""
                    UPDATE reports SET email_sent = 1, email_sent_at = CURRENT_TIMESTAMP
                    WHERE id = ?
                """, (report_id,))
            else:
                cursor.execute("""
                    UPDATE reports SET email_sent = 0, email_sent_at = NULL
                    WHERE id = ?
                """, (report_id,))
            
            conn.commit()
            conn.close()
            return True
        except Exception as e:
            print(f"Warning: Could not update email status: {e}")
            return False
    
    def get_reports_from_database(
        self,
        report_type: Optional[str] = None,
        days: int = 30,
        limit: int = 100
    ) -> List[Dict[str, Any]]:
        """
        Retrieve reports from the database.
        
        Args:
            report_type: Filter by type ("daily", "weekly", or None for all)
            days: Number of days to look back
            limit: Maximum number of reports to return
            
        Returns:
            List of report metadata dictionaries
        """
        try:
            self._init_reports_table()
            conn = sqlite3.connect(self.db_path)
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            
            cutoff = (datetime.now() - timedelta(days=days)).strftime('%Y-%m-%d')
            
            if report_type:
                cursor.execute("""
                    SELECT id, report_date, report_type, format, summary,
                           total_hours, task_count, completed_count, projects_count,
                           ai_enhanced, email_sent, email_sent_at, created_at
                    FROM reports
                    WHERE report_type = ? AND report_date >= ?
                    ORDER BY report_date DESC, created_at DESC
                    LIMIT ?
                """, (report_type, cutoff, limit))
            else:
                cursor.execute("""
                    SELECT id, report_date, report_type, format, summary,
                           total_hours, task_count, completed_count, projects_count,
                           ai_enhanced, email_sent, email_sent_at, created_at
                    FROM reports
                    WHERE report_date >= ?
                    ORDER BY report_date DESC, created_at DESC
                    LIMIT ?
                """, (cutoff, limit))
            
            reports = []
            for row in cursor.fetchall():
                reports.append({
                    "id": row["id"],
                    "date": datetime.strptime(row["report_date"], '%Y-%m-%d'),
                    "report_type": row["report_type"],
                    "format": row["format"],
                    "summary": row["summary"],
                    "total_hours": row["total_hours"],
                    "task_count": row["task_count"],
                    "completed_count": row["completed_count"],
                    "projects_count": row["projects_count"],
                    "ai_enhanced": bool(row["ai_enhanced"]),
                    "email_sent": bool(row["email_sent"]),
                    "email_sent_at": row["email_sent_at"],
                    "created_at": row["created_at"],
                    "source": "database"
                })
            
            conn.close()
            return reports
        except Exception as e:
            print(f"Warning: Could not query reports from database: {e}")
            return []
    
    def get_report_content(self, report_id: int) -> Optional[str]:
        """
        Retrieve full report content from the database.
        
        Args:
            report_id: ID of the report
            
        Returns:
            Report content string or None
        """
        try:
            conn = sqlite3.connect(self.db_path)
            cursor = conn.cursor()
            
            cursor.execute("SELECT content FROM reports WHERE id = ?", (report_id,))
            row = cursor.fetchone()
            conn.close()
            
            return row[0] if row else None
        except Exception as e:
            print(f"Warning: Could not get report content: {e}")
            return None
    
    def save_report(
        self,
        report: EnhancedReport,
        output_format: OutputFormat = OutputFormat.TEXT,
        output_path: Optional[str] = None
    ) -> str:
        """
        Save report to file.
        
        Args:
            report: Report to save
            output_format: Format for the saved file
            output_path: Path to save to (auto-generated if None)
            
        Returns:
            Path where report was saved
        """
        if output_path is None:
            date_str = report.base_report.date.strftime('%Y-%m-%d')
            try:
                from backend.config import reports_dir
                reports_dir_path = str(reports_dir())
            except ImportError:
                try:
                    from backend.utils.paths import fallback_path
                    reports_dir_path = fallback_path("Data", "reports")
                except ImportError:
                    from pathlib import Path
                    cur = Path(__file__).resolve().parent.parent
                    reports_dir_path = str(cur / "Data" / "reports")
            os.makedirs(reports_dir_path, exist_ok=True)
            
            ext = {
                OutputFormat.TEXT: "txt",
                OutputFormat.HTML: "html",
                OutputFormat.MARKDOWN: "md",
                OutputFormat.JSON: "json"
            }.get(output_format, "txt")
            
            ai_suffix = "_ai" if report.is_ai_enhanced else ""
            output_path = os.path.join(reports_dir_path, f"report-{date_str}{ai_suffix}.{ext}")
        
        content = self.format_report(report, output_format)
        
        with open(output_path, 'w') as f:
            f.write(content)
        
        # Also save to database
        self.save_to_database(report, output_format, "daily")
        
        return output_path
    
    def get_report_preview(self, report: EnhancedReport, max_lines: int = 25) -> str:
        """
        Get a compact preview of the report for TUI display.
        
        Args:
            report: Report to preview
            max_lines: Maximum lines in preview
            
        Returns:
            Compact preview string
        """
        lines = []
        base = report.base_report
        
        lines.append("┌─ Report Preview ─────────────────────────┐")
        lines.append(f"│ 📅 {base.date.strftime('%A, %B %d, %Y'):<35}│")
        lines.append("├──────────────────────────────────────────┤")
        lines.append(f"│ ⏱️  Hours: {base.total_hours:<5.1f}  ✅ Done: {base.completed_count:<3}    │")
        lines.append(f"│ 📁 Projects: {len(base.projects_worked):<3}  📝 Tasks: {len(base.activities):<3}  │")
        
        if report.is_ai_enhanced and report.ai_insights:
            ai = report.ai_insights
            lines.append("├──────────────────────────────────────────┤")
            lines.append(f"│ ✨ AI Score: {ai.productivity_score}/10                      │")
            if ai.executive_summary:
                summary = ai.executive_summary[:38]
                lines.append(f"│ {summary:<40}│")
        
        lines.append("└──────────────────────────────────────────┘")
        
        return "\n".join(lines[:max_lines])

    # =========================================================================
    # WEEKLY REPORTS
    # =========================================================================
    
    def generate_weekly_report(
        self,
        week_end_date: Optional[datetime] = None,
        style: ReportStyle = ReportStyle.PROFESSIONAL,
        include_ai: bool = True
    ) -> "WeeklyReport":
        """
        Generate a weekly summary report.
        
        Args:
            week_end_date: End date of the week (default: today)
            style: Report formatting style
            include_ai: Whether to include AI analysis
            
        Returns:
            WeeklyReport with aggregated data and insights
        """
        if week_end_date is None:
            week_end_date = datetime.now()
        
        # Calculate week start (Monday)
        days_since_monday = week_end_date.weekday()
        week_start = week_end_date - timedelta(days=days_since_monday)
        week_start = week_start.replace(hour=0, minute=0, second=0, microsecond=0)
        
        # Generate daily reports for the week
        daily_reports: List[EnhancedReport] = []
        current_date = week_start
        
        while current_date <= week_end_date:
            daily = self.generate_report(
                date=current_date,
                style=style,
                include_ai=False  # AI analysis at weekly level
            )
            daily_reports.append(daily)
            current_date += timedelta(days=1)
        
        # Aggregate statistics
        total_hours = sum(r.base_report.total_hours for r in daily_reports)
        total_completed = sum(r.base_report.completed_count for r in daily_reports)
        total_in_progress = sum(r.base_report.in_progress_count for r in daily_reports)
        total_blocked = sum(r.base_report.blocked_count for r in daily_reports)
        
        # Collect all activities
        all_activities: List[ActivitySummary] = []
        for r in daily_reports:
            all_activities.extend(r.base_report.activities)
        
        # Get unique projects
        all_projects = list(set(
            act.project for r in daily_reports 
            for act in r.base_report.activities 
            if act.project
        ))
        
        # Get unique tickets
        all_tickets = list(set(
            act.ticket_id for r in daily_reports 
            for act in r.base_report.activities 
            if act.ticket_id
        ))
        
        # Calculate daily breakdown
        daily_breakdown = [
            {
                "date": r.base_report.date.strftime("%a %m/%d"),
                "hours": r.base_report.total_hours,
                "completed": r.base_report.completed_count,
                "activities": len(r.base_report.activities)
            }
            for r in daily_reports
        ]
        
        # Generate AI insights for the week
        ai_insights = None
        if include_ai and self.check_ollama_available() and all_activities:
            ai_insights = self._generate_weekly_ai_insights(
                daily_reports, total_hours, total_completed, all_projects
            )
        
        return WeeklyReport(
            week_start=week_start,
            week_end=week_end_date,
            daily_reports=daily_reports,
            total_hours=total_hours,
            total_completed=total_completed,
            total_in_progress=total_in_progress,
            total_blocked=total_blocked,
            projects_worked=all_projects,
            tickets_updated=all_tickets,
            daily_breakdown=daily_breakdown,
            ai_insights=ai_insights,
            generated_at=datetime.now(),
            is_ai_enhanced=(ai_insights is not None)
        )
    
    def _generate_weekly_ai_insights(
        self,
        daily_reports: List[EnhancedReport],
        total_hours: float,
        total_completed: int,
        projects: List[str]
    ) -> Optional[AIInsights]:
        """Generate AI insights for weekly report."""
        
        # Build weekly summary for AI
        summary_parts = []
        for r in daily_reports:
            if r.base_report.activities:
                day_name = r.base_report.date.strftime('%A')
                summary_parts.append(
                    f"{day_name}: {r.base_report.total_hours:.1f}h, "
                    f"{r.base_report.completed_count} completed"
                )
        
        prompt = f"""Analyze this developer's weekly work summary and provide insights.

WEEKLY STATISTICS:
- Total Hours: {total_hours:.1f}
- Tasks Completed: {total_completed}
- Projects: {', '.join(projects)}

DAILY BREAKDOWN:
{chr(10).join(summary_parts)}

Provide a JSON response with these exact fields:
{{
    "executive_summary": "2-3 sentence summary of the week's productivity",
    "key_accomplishments": ["list of 3-5 major accomplishments"],
    "ongoing_work": ["work still in progress"],
    "blockers_risks": ["any patterns of blockers noticed"],
    "recommendations": ["1-3 suggestions for next week"],
    "productivity_score": 7,
    "focus_areas": ["main focus areas for the week"]
}}

Be constructive and highlight patterns. Respond ONLY with valid JSON."""

        prompt = _inject_style(prompt, context_type="report")

        try:
            from backend.llm.base import LLMOptions
            from backend.config import http_timeout_long, report_llm_temperature, report_llm_max_tokens
            response_text = self._get_provider().generate(
                prompt=prompt,
                options=LLMOptions(
                    temperature=report_llm_temperature(),
                    max_tokens=report_llm_max_tokens(),
                ),
                timeout=http_timeout_long(),
            )
            if response_text:
                insights_data = self._parse_ai_response(response_text)
                
                if insights_data:
                    return AIInsights(
                        executive_summary=insights_data.get("executive_summary", ""),
                        key_accomplishments=insights_data.get("key_accomplishments", []),
                        ongoing_work=insights_data.get("ongoing_work", []),
                        blockers_risks=insights_data.get("blockers_risks", []),
                        recommendations=insights_data.get("recommendations", []),
                        productivity_score=int(insights_data.get("productivity_score", 5)),
                        focus_areas=insights_data.get("focus_areas", [])
                    )
        except Exception as e:
            print(f"Weekly AI analysis failed: {e}")
        
        return None
    
    def format_weekly_report(
        self,
        report: "WeeklyReport",
        output_format: OutputFormat = OutputFormat.TERMINAL
    ) -> str:
        """Format weekly report for output."""
        if output_format == OutputFormat.TERMINAL:
            return self._format_weekly_terminal(report)
        elif output_format == OutputFormat.MARKDOWN:
            return self._format_weekly_markdown(report)
        elif output_format == OutputFormat.JSON:
            return self._format_weekly_json(report)
        elif output_format == OutputFormat.HTML:
            return self._format_weekly_html(report)
        else:
            return self._format_weekly_terminal(report)
    
    def _format_weekly_terminal(self, report: "WeeklyReport") -> str:
        """Format weekly report for terminal display."""
        lines = []
        week_range = f"{report.week_start.strftime('%b %d')} - {report.week_end.strftime('%b %d, %Y')}"
        
        lines.append("")
        lines.append("╔══════════════════════════════════════════════════════════════════════╗")
        lines.append(f"║  📊 WEEKLY STATUS REPORT - {week_range:<40}║")
        lines.append("╚══════════════════════════════════════════════════════════════════════╝")
        lines.append("")
        
        if report.is_ai_enhanced:
            lines.append("  ✨ AI-Enhanced Report")
            lines.append("")
        
        # Summary
        lines.append("  ┌─ WEEKLY SUMMARY ──────────────────────────────────────────────────┐")
        lines.append(f"  │  ⏱️  Total Hours:     {report.total_hours:.1f}h")
        lines.append(f"  │  ✅ Tasks Completed: {report.total_completed}")
        lines.append(f"  │  🔄 In Progress:     {report.total_in_progress}")
        lines.append(f"  │  📁 Projects:        {len(report.projects_worked)}")
        lines.append(f"  │  🎫 Tickets:         {len(report.tickets_updated)}")
        lines.append("  └────────────────────────────────────────────────────────────────────┘")
        lines.append("")
        
        # Daily breakdown chart
        lines.append("  ┌─ DAILY BREAKDOWN ─────────────────────────────────────────────────┐")
        max_hours = max((d["hours"] for d in report.daily_breakdown), default=8)
        bar_width = 30
        
        for day in report.daily_breakdown:
            bar_len = int((day["hours"] / max(max_hours, 1)) * bar_width) if day["hours"] > 0 else 0
            bar = "█" * bar_len + "░" * (bar_width - bar_len)
            lines.append(f"  │  {day['date']}: {bar} {day['hours']:.1f}h ({day['completed']} done)")
        
        lines.append("  └────────────────────────────────────────────────────────────────────┘")
        lines.append("")
        
        # AI Insights
        if report.ai_insights:
            ai = report.ai_insights
            lines.append("  ┌─ AI INSIGHTS ──────────────────────────────────────────────────────┐")
            
            if ai.executive_summary:
                lines.append(f"  │  💡 {ai.executive_summary[:65]}")
            
            score_bar = "●" * ai.productivity_score + "○" * (10 - ai.productivity_score)
            lines.append(f"  │  📈 Productivity: {score_bar} {ai.productivity_score}/10")
            
            if ai.key_accomplishments:
                lines.append("  │")
                lines.append("  │  🏆 Key Accomplishments:")
                for acc in ai.key_accomplishments[:4]:
                    lines.append(f"  │     ✓ {acc[:55]}")
            
            if ai.recommendations:
                lines.append("  │")
                lines.append("  │  📋 Next Week Focus:")
                for rec in ai.recommendations[:3]:
                    lines.append(f"  │     • {rec[:55]}")
            
            lines.append("  └────────────────────────────────────────────────────────────────────┘")
        
        lines.append("")
        gen_time = report.generated_at.strftime('%I:%M %p on %b %d')
        lines.append(f"  Generated at {gen_time}")
        lines.append("")
        
        return "\n".join(lines)
    
    def _format_weekly_markdown(self, report: "WeeklyReport") -> str:
        """Format weekly report as Markdown."""
        lines = []
        week_range = f"{report.week_start.strftime('%b %d')} - {report.week_end.strftime('%b %d, %Y')}"
        
        lines.append("# Weekly Status Report")
        lines.append(f"**Week of {week_range}**")
        lines.append("")
        
        if report.is_ai_enhanced:
            lines.append("*✨ AI-Enhanced Report*")
            lines.append("")
        
        lines.append("## Summary")
        lines.append("")
        lines.append("| Metric | Value |")
        lines.append("|--------|-------|")
        lines.append(f"| Total Hours | {report.total_hours:.1f}h |")
        lines.append(f"| Tasks Completed | {report.total_completed} |")
        lines.append(f"| In Progress | {report.total_in_progress} |")
        lines.append(f"| Projects | {len(report.projects_worked)} |")
        lines.append(f"| Tickets | {len(report.tickets_updated)} |")
        lines.append("")
        
        # Daily breakdown
        lines.append("## Daily Breakdown")
        lines.append("")
        lines.append("| Day | Hours | Completed | Activities |")
        lines.append("|-----|-------|-----------|------------|")
        for day in report.daily_breakdown:
            lines.append(f"| {day['date']} | {day['hours']:.1f}h | {day['completed']} | {day['activities']} |")
        lines.append("")
        
        # AI Insights
        if report.ai_insights:
            ai = report.ai_insights
            lines.append("## AI Insights")
            lines.append("")
            lines.append(f"> {ai.executive_summary}")
            lines.append("")
            lines.append(f"**Productivity Score:** {ai.productivity_score}/10")
            lines.append("")
            
            if ai.key_accomplishments:
                lines.append("### Key Accomplishments")
                for acc in ai.key_accomplishments:
                    lines.append(f"- ✅ {acc}")
                lines.append("")
            
            if ai.recommendations:
                lines.append("### Next Week Focus")
                for rec in ai.recommendations:
                    lines.append(f"- 📋 {rec}")
                lines.append("")
        
        # Projects worked
        if report.projects_worked:
            lines.append("## Projects")
            lines.append("")
            for proj in sorted(report.projects_worked):
                lines.append(f"- {proj}")
            lines.append("")
        
        gen_time = report.generated_at.strftime('%I:%M %p on %b %d, %Y')
        lines.append("---")
        lines.append(f"*Generated at {gen_time}*")
        
        return "\n".join(lines)
    
    def _format_weekly_json(self, report: "WeeklyReport") -> str:
        """Format weekly report as JSON."""
        import json
        
        data = {
            "week_start": report.week_start.isoformat(),
            "week_end": report.week_end.isoformat(),
            "generated_at": report.generated_at.isoformat(),
            "is_ai_enhanced": report.is_ai_enhanced,
            "summary": {
                "total_hours": report.total_hours,
                "total_completed": report.total_completed,
                "total_in_progress": report.total_in_progress,
                "total_blocked": report.total_blocked,
                "projects_count": len(report.projects_worked),
                "tickets_count": len(report.tickets_updated)
            },
            "daily_breakdown": report.daily_breakdown,
            "projects": report.projects_worked,
            "tickets": report.tickets_updated
        }
        
        if report.ai_insights:
            ai = report.ai_insights
            data["ai_insights"] = {
                "executive_summary": ai.executive_summary,
                "key_accomplishments": ai.key_accomplishments,
                "ongoing_work": ai.ongoing_work,
                "blockers_risks": ai.blockers_risks,
                "recommendations": ai.recommendations,
                "productivity_score": ai.productivity_score,
                "focus_areas": ai.focus_areas
            }
        
        return json.dumps(data, indent=2)
    
    def _format_weekly_html(self, report: "WeeklyReport") -> str:
        """Format weekly report as HTML."""
        week_range = f"{report.week_start.strftime('%b %d')} - {report.week_end.strftime('%b %d, %Y')}"
        
        html = f"""
<!DOCTYPE html>
<html>
<head>
    <style>
        body {{
            font-family: 'Segoe UI', Tahoma, sans-serif;
            max-width: 900px;
            margin: 0 auto;
            padding: 20px;
            background: #f5f5f5;
        }}
        .container {{
            background: white;
            padding: 30px;
            border-radius: 12px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }}
        h1 {{
            color: #2c3e50;
            border-bottom: 3px solid #3498db;
            padding-bottom: 15px;
        }}
        .stats {{
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 15px;
            margin: 20px 0;
        }}
        .stat-card {{
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            border-radius: 8px;
            text-align: center;
        }}
        .stat-value {{
            font-size: 2em;
            font-weight: bold;
        }}
        .daily-chart {{
            margin: 20px 0;
        }}
        .day-row {{
            display: flex;
            align-items: center;
            margin: 8px 0;
        }}
        .day-label {{
            width: 80px;
            font-weight: 500;
        }}
        .day-bar {{
            height: 24px;
            background: linear-gradient(90deg, #3498db, #2ecc71);
            border-radius: 4px;
            margin: 0 10px;
        }}
        .ai-section {{
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            border-left: 4px solid #667eea;
            margin: 20px 0;
        }}
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 Weekly Report: {week_range}</h1>
        
        <div class="stats">
            <div class="stat-card">
                <div class="stat-value">{report.total_hours:.1f}h</div>
                <div>Total Hours</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{report.total_completed}</div>
                <div>Completed</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{len(report.projects_worked)}</div>
                <div>Projects</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{len(report.tickets_updated)}</div>
                <div>Tickets</div>
            </div>
        </div>

        <h2>Daily Breakdown</h2>
        <div class="daily-chart">
"""
        
        max_hours = max((d["hours"] for d in report.daily_breakdown), default=8)
        for day in report.daily_breakdown:
            width_pct = int((day["hours"] / max(max_hours, 1)) * 100) if day["hours"] > 0 else 0
            html += f"""
            <div class="day-row">
                <span class="day-label">{day['date']}</span>
                <div class="day-bar" style="width: {width_pct}%"></div>
                <span>{day['hours']:.1f}h ({day['completed']} done)</span>
            </div>
"""
        
        html += """
        </div>
"""
        
        if report.ai_insights:
            ai = report.ai_insights
            html += f"""
        <div class="ai-section">
            <h2>✨ AI Insights</h2>
            <p style="font-size: 1.1em;">{ai.executive_summary}</p>
            <p><strong>Productivity:</strong> {ai.productivity_score}/10</p>
"""
            if ai.key_accomplishments:
                html += "<h3>Key Accomplishments</h3><ul>"
                for acc in ai.key_accomplishments:
                    html += f"<li>{acc}</li>"
                html += "</ul>"
            
            if ai.recommendations:
                html += "<h3>Next Week Focus</h3><ul>"
                for rec in ai.recommendations:
                    html += f"<li>{rec}</li>"
                html += "</ul>"
            html += "</div>"
        
        gen_time = report.generated_at.strftime('%I:%M %p on %b %d, %Y')
        html += f"""
        <p style="color: #7f8c8d; margin-top: 30px;">Generated at {gen_time}</p>
    </div>
</body>
</html>
"""
        return html

    def save_weekly_to_database(
        self,
        report: "WeeklyReport",
        output_format: OutputFormat = OutputFormat.TEXT
    ) -> Optional[int]:
        """
        Save weekly report to the SQLite database.
        
        Args:
            report: Weekly report to save
            output_format: Format of the content
            
        Returns:
            Report ID if successful, None otherwise
        """
        try:
            self._init_reports_table()
            
            content = self.format_weekly_report(report, output_format)
            summary = (
                f"Week: {report.week_start.strftime('%Y-%m-%d')} to {report.week_end.strftime('%Y-%m-%d')} | "
                f"Hours: {report.total_hours:.1f} | "
                f"Tasks: {report.total_completed} completed | "
                f"Projects: {len(report.projects_worked)}"
            )
            
            conn = sqlite3.connect(self.db_path)
            cursor = conn.cursor()
            
            cursor.execute("""
                INSERT INTO reports (
                    report_date, report_type, format, content, summary,
                    total_hours, task_count, completed_count, projects_count,
                    ai_enhanced, email_sent
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """, (
                report.week_end.strftime('%Y-%m-%d'),
                "weekly",
                output_format.value,
                content,
                summary,
                report.total_hours,
                report.total_completed + report.total_in_progress + report.total_blocked,
                report.total_completed,
                len(report.projects_worked),
                report.is_ai_enhanced,
                False
            ))
            
            report_id = cursor.lastrowid
            conn.commit()
            conn.close()
            
            return report_id
        except Exception as e:
            print(f"Warning: Could not save weekly report to database: {e}")
            return None

    # =========================================================================
    # EMAIL INTEGRATION
    # =========================================================================
    
    async def send_via_email(
        self,
        report: EnhancedReport,
        recipient: str,
        subject: Optional[str] = None,
        output_format: OutputFormat = OutputFormat.HTML
    ) -> bool:
        """
        Send report via email using Microsoft Graph.
        
        Args:
            report: Report to send
            recipient: Email address
            subject: Custom subject (auto-generated if None)
            output_format: Format for email body
            
        Returns:
            True if sent successfully
        """
        if subject is None:
            date_str = report.base_report.date.strftime('%B %d, %Y')
            subject = f"Daily Status Report - {date_str}"
        
        # Note: body formatting is handled by send_email_report
        return await self.email_reporter.send_email_report(
            recipient=recipient,
            report=report.base_report,
            subject=subject,
            format='html' if output_format == OutputFormat.HTML else 'text'
        )
    
    async def send_weekly_via_email(
        self,
        report: "WeeklyReport",
        recipient: str,
        subject: Optional[str] = None
    ) -> bool:
        """
        Send weekly report via email.
        
        Args:
            report: Weekly report to send
            recipient: Email address
            subject: Custom subject
            
        Returns:
            True if sent successfully
        """
        if subject is None:
            week_range = f"{report.week_start.strftime('%b %d')} - {report.week_end.strftime('%b %d')}"
            subject = f"Weekly Status Report: {week_range}"
        
        body = self._format_weekly_html(report)
        
        if not self.email_reporter.graph_client:
            print("✗ Graph client not initialized. Call email_reporter.initialize_graph() first.")
            return False
        
        try:
            await self.email_reporter.graph_client.send_mail(subject, body, recipient)
            print(f"✓ Weekly report sent to {recipient}")
            return True
        except Exception as e:
            print(f"✗ Failed to send weekly report: {e}")
            return False

    # =========================================================================
    # REPORT HISTORY
    # =========================================================================
    
    def list_saved_reports(self, days: int = 30, include_files: bool = True) -> List[Dict[str, Any]]:
        """
        List saved reports from database and optionally from files.
        
        Args:
            days: Number of days to look back
            include_files: Whether to also check file system for reports
            
        Returns:
            List of report metadata
        """
        # First, query from database
        reports = self.get_reports_from_database(days=days)
        
        # Optionally add file-based reports not in database
        if include_files:
            import glob
            try:
                from backend.config import reports_dir
                reports_dir_path = str(reports_dir())
            except ImportError:
                try:
                    from backend.utils.paths import fallback_path
                    reports_dir_path = fallback_path("Data", "reports")
                except ImportError:
                    from pathlib import Path
                    cur = Path(__file__).resolve().parent.parent
                    reports_dir_path = str(cur / "Data" / "reports")
            if os.path.exists(reports_dir_path):
                cutoff = datetime.now() - timedelta(days=days)
                db_dates = {r["date"].strftime('%Y-%m-%d') for r in reports}
                
                for pattern in ["report-*.txt", "report-*.md", "report-*.html", "report-*.json"]:
                    for path in glob.glob(os.path.join(reports_dir_path, pattern)):
                        filename = os.path.basename(path)
                        try:
                            date_str = filename.split("report-")[1].split("_")[0].split(".")[0]
                            report_date = datetime.strptime(date_str, '%Y-%m-%d')
                            
                            # Skip if already in database results
                            if date_str in db_dates:
                                continue
                            
                            if report_date >= cutoff:
                                stat = os.stat(path)
                                reports.append({
                                    "id": None,
                                    "path": path,
                                    "filename": filename,
                                    "date": report_date,
                                    "report_type": "daily",
                                    "size": stat.st_size,
                                    "format": os.path.splitext(filename)[1][1:],
                                    "ai_enhanced": "_ai" in filename,
                                    "source": "file"
                                })
                        except (ValueError, IndexError):
                            continue
        
        return sorted(reports, key=lambda x: x["date"], reverse=True)
    
    def get_productivity_trend(self, days: int = 7) -> Dict[str, Any]:
        """
        Calculate productivity trend over recent days.
        
        Args:
            days: Number of days to analyze
            
        Returns:
            Dictionary with trend data
        """
        daily_data = []
        current = datetime.now()
        
        for i in range(days):
            date = current - timedelta(days=i)
            report = self.generate_report(date=date, include_ai=False)
            
            if report.base_report.activities:
                daily_data.append({
                    "date": date.strftime('%Y-%m-%d'),
                    "day": date.strftime('%a'),
                    "hours": report.base_report.total_hours,
                    "completed": report.base_report.completed_count,
                    "activities": len(report.base_report.activities)
                })
        
        if not daily_data:
            return {"trend": "no_data", "daily": []}
        
        # Calculate averages and trends
        avg_hours = sum(d["hours"] for d in daily_data) / len(daily_data)
        avg_completed = sum(d["completed"] for d in daily_data) / len(daily_data)
        
        # Simple trend: compare first half to second half
        mid = len(daily_data) // 2
        first_half_hours = sum(d["hours"] for d in daily_data[:mid]) / max(mid, 1)
        second_half_hours = sum(d["hours"] for d in daily_data[mid:]) / max(len(daily_data) - mid, 1)
        
        if second_half_hours > first_half_hours * 1.1:
            trend = "improving"
        elif second_half_hours < first_half_hours * 0.9:
            trend = "declining"
        else:
            trend = "stable"
        
        return {
            "trend": trend,
            "avg_hours": round(avg_hours, 1),
            "avg_completed": round(avg_completed, 1),
            "total_hours": round(sum(d["hours"] for d in daily_data), 1),
            "total_completed": sum(d["completed"] for d in daily_data),
            "daily": list(reversed(daily_data))
        }


@dataclass
class WeeklyReport:
    """Weekly aggregated report."""
    week_start: datetime
    week_end: datetime
    daily_reports: List[EnhancedReport]
    total_hours: float
    total_completed: int
    total_in_progress: int
    total_blocked: int
    projects_worked: List[str]
    tickets_updated: List[str]
    daily_breakdown: List[Dict[str, Any]]
    ai_insights: Optional[AIInsights]
    generated_at: datetime
    is_ai_enhanced: bool


# CLI interface
def main():
    """CLI for daily report generator."""
    import argparse
    
    parser = argparse.ArgumentParser(
        description="Generate AI-enhanced daily and weekly reports",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s generate                    # Generate today's report
  %(prog)s generate --no-ai            # Generate without AI
  %(prog)s weekly                      # Generate weekly report
  %(prog)s history                     # Show saved reports
  %(prog)s trend --days 7              # Show productivity trend
  %(prog)s save --format markdown      # Save as markdown
"""
    )
    
    parser.add_argument(
        "command",
        choices=["generate", "preview", "save", "json", "weekly", "history", "trend"],
        help="Command to execute"
    )
    parser.add_argument("--date", "-d", help="Date in YYYY-MM-DD format")
    parser.add_argument(
        "--style", "-s",
        choices=["professional", "casual", "detailed", "executive"],
        default="professional",
        help="Report style"
    )
    parser.add_argument("--no-ai", action="store_true", help="Disable AI enhancement")
    parser.add_argument(
        "--format", "-f",
        choices=["terminal", "text", "html", "markdown", "json"],
        default="terminal",
        help="Output format"
    )
    parser.add_argument("--output", "-o", help="Output file path")
    parser.add_argument("--days", type=int, default=7, help="Number of days for trend/history")
    
    args = parser.parse_args()
    
    # Create generator
    generator = DailyReportGenerator()
    
    # -------------------------------------------------------------------------
    # WEEKLY REPORT
    # -------------------------------------------------------------------------
    if args.command == "weekly":
        date = None
        if args.date:
            try:
                date = datetime.strptime(args.date, '%Y-%m-%d')
            except ValueError:
                print("Invalid date format. Use YYYY-MM-DD")
                return 1
        
        print("📊 Generating weekly report...")
        weekly_report = generator.generate_weekly_report(
            week_end_date=date,
            include_ai=not args.no_ai
        )
        
        output_format = OutputFormat(args.format)
        print(generator.format_weekly_report(weekly_report, output_format))
        
        # Save to database
        report_id = generator.save_weekly_to_database(weekly_report, output_format)
        if report_id:
            print(f"\n✓ Weekly report saved to database (ID: {report_id})")
        
        return 0
    
    # -------------------------------------------------------------------------
    # HISTORY - Show saved reports
    # -------------------------------------------------------------------------
    if args.command == "history":
        reports = generator.list_saved_reports(days=args.days)
        
        if not reports:
            print("No saved reports found")
            return 0
        
        print(f"📁 Saved Reports (last {args.days} days):")
        print("-" * 70)
        for r in reports:
            ai_badge = " ✨" if r.get("ai_enhanced") or r.get("is_ai_enhanced") else ""
            report_type = r.get("report_type", "daily")
            type_badge = "📅" if report_type == "daily" else "📆"
            source = r.get("source", "database")
            
            if source == "database":
                summary = r.get("summary", "")[:40]
                print(f"  {type_badge} {r['date'].strftime('%Y-%m-%d')} | {report_type:6} | {r.get('format', ''):4}{ai_badge}")
                print(f"    └ {summary}")
            else:
                size_kb = r.get("size", 0) / 1024
                print(f"  {type_badge} {r['date'].strftime('%Y-%m-%d')} | {report_type:6} | {r.get('format', ''):4} | {size_kb:5.1f}KB{ai_badge}")
                print(f"    └ {r.get('path', '')}")
        print("-" * 70)
        print(f"Total: {len(reports)} reports")
        return 0
    
    # -------------------------------------------------------------------------
    # TREND - Show productivity trend
    # -------------------------------------------------------------------------
    if args.command == "trend":
        print(f"📈 Productivity Trend (last {args.days} days):")
        print("-" * 50)
        
        trend = generator.get_productivity_trend(days=args.days)
        
        if trend["trend"] == "no_data":
            print("No activity data found")
            return 0
        
        # Print trend indicator
        trend_icon = {
            "improving": "📈 Improving",
            "declining": "📉 Declining",
            "stable": "➡️  Stable"
        }.get(trend["trend"], "❓ Unknown")
        
        print(f"Trend: {trend_icon}")
        print(f"Avg Hours/Day: {trend['avg_hours']:.1f}h")
        print(f"Avg Completed/Day: {trend['avg_completed']:.1f}")
        print(f"Total Hours: {trend['total_hours']:.1f}h")
        print(f"Total Completed: {trend['total_completed']}")
        print()
        print("Daily Breakdown:")
        
        max_hours = max((d["hours"] for d in trend["daily"]), default=8)
        for day in trend["daily"]:
            bar_len = int((day["hours"] / max(max_hours, 1)) * 20) if day["hours"] > 0 else 0
            bar = "█" * bar_len + "░" * (20 - bar_len)
            print(f"  {day['day']} {day['date']}: {bar} {day['hours']:.1f}h")
        
        return 0
    
    # -------------------------------------------------------------------------
    # DAILY REPORT COMMANDS
    # -------------------------------------------------------------------------
    
    # Parse date
    date = None
    if args.date:
        try:
            date = datetime.strptime(args.date, '%Y-%m-%d')
        except ValueError:
            print("Invalid date format. Use YYYY-MM-DD")
            return 1
    
    # Generate report
    style = ReportStyle(args.style)
    report = generator.generate_report(date, style=style, include_ai=not args.no_ai)
    
    if args.command == "generate" or args.command == "preview":
        output_format = OutputFormat(args.format)
        print(generator.format_report(report, output_format))
    
    elif args.command == "save":
        output_format = OutputFormat(args.format if args.format != "terminal" else "text")
        path = generator.save_report(report, output_format, args.output)
        print(f"✓ Report saved to {path}")
    
    elif args.command == "json":
        print(generator.format_report(report, OutputFormat.JSON))
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
