"""
Automated Email Report Generation

This module generates professional daily/weekly status reports by compiling
activities from the SQLite database and sending them via Microsoft Graph API.
"""

import os
import sys
import json
import sqlite3
from datetime import datetime, timedelta
from pathlib import Path
from typing import List, Dict, Optional
from dataclasses import dataclass

# Add project root for backend imports
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))


@dataclass
class ActivitySummary:
    """Summary of a single activity"""
    timestamp: datetime
    project: str
    ticket_id: str
    status: str
    description: str
    time_spent: float
    source: str  # 'commit', 'manual', 'timer'


@dataclass
class DailyReport:
    """Complete daily report data"""
    date: datetime
    activities: List[ActivitySummary]
    total_hours: float
    projects_worked: List[str]
    tickets_updated: List[str]
    completed_count: int
    in_progress_count: int
    blocked_count: int


class EmailReporter:
    """Generates and sends automated email reports"""
    
    def __init__(self, db_path: Optional[str] = None):
        """
        Initialize email reporter
        
        Args:
            db_path: Path to SQLite database (auto-detected if None)
        """
        if db_path is None:
            try:
                from backend.config import database_path
                db_path = str(database_path())
            except ImportError:
                try:
                    from backend.utils.paths import fallback_path
                    db_path = fallback_path("Data", "db", "devtrack.db")
                except ImportError:
                    cur = Path(__file__).resolve().parent.parent
                    db_path = str(cur / "Data" / "db" / "devtrack.db")
        self.db_path = db_path
        self.graph_client = None
    
    def initialize_graph(self, graph_client):
        """
        Initialize with Microsoft Graph client
        
        Args:
            graph_client: Instance of msgraph_python.graph.Graph
        """
        self.graph_client = graph_client
    
    def get_daily_activities(self, date: Optional[datetime] = None) -> List[ActivitySummary]:
        """
        Get all activities for a specific day
        
        Args:
            date: Date to get activities for (default: today)
            
        Returns:
            List of activity summaries
        """
        if date is None:
            date = datetime.now()
        
        start_of_day = date.replace(hour=0, minute=0, second=0, microsecond=0)
        end_of_day = date.replace(hour=23, minute=59, second=59, microsecond=999999)
        
        activities = []
        
        try:
            conn = sqlite3.connect(self.db_path)
            cursor = conn.cursor()
            
            # Query task updates for the day (schema: Go devtrack uses timestamp, project,
            # ticket_id, status, update_text; no time_estimate/source - use 0 and 'manual')
            cursor.execute("PRAGMA table_info(task_updates)")
            columns = [r[1] for r in cursor.fetchall()]
            has_time_estimate = "time_estimate" in columns
            has_source = "source" in columns

            if has_time_estimate and has_source:
                query = """
                    SELECT timestamp, project, ticket_id, status, update_text,
                           time_estimate, source
                    FROM task_updates
                    WHERE timestamp >= ? AND timestamp <= ?
                    ORDER BY timestamp ASC
                """
            else:
                query = """
                    SELECT timestamp, project, ticket_id, status, update_text
                    FROM task_updates
                    WHERE timestamp >= ? AND timestamp <= ?
                    ORDER BY timestamp ASC
                """

            cursor.execute(query, (start_of_day.isoformat(), end_of_day.isoformat()))

            for row in cursor.fetchall():
                if has_time_estimate and has_source:
                    timestamp_str, project, ticket_id, status, description, time_est, source = row
                else:
                    timestamp_str, project, ticket_id, status, description = row
                    time_est, source = 0, "manual"
                
                activities.append(ActivitySummary(
                    timestamp=datetime.fromisoformat(timestamp_str),
                    project=project or "Unknown",
                    ticket_id=ticket_id or "",
                    status=status or "in_progress",
                    description=description or "",
                    time_spent=float(time_est or 0),
                    source=source or "manual"
                ))
            
            conn.close()
            
        except Exception as e:
            print(f"Error fetching activities: {e}")
        
        return activities
    
    def generate_daily_report(self, date: Optional[datetime] = None) -> DailyReport:
        """
        Generate a complete daily report
        
        Args:
            date: Date to generate report for (default: today)
            
        Returns:
            DailyReport object with all statistics
        """
        if date is None:
            date = datetime.now()
        
        activities = self.get_daily_activities(date)
        
        # Calculate statistics
        total_hours = sum(act.time_spent for act in activities)
        projects_worked = list(set(act.project for act in activities if act.project))
        tickets_updated = list(set(act.ticket_id for act in activities if act.ticket_id))
        
        completed_count = sum(1 for act in activities if act.status == 'completed')
        in_progress_count = sum(1 for act in activities if act.status == 'in_progress')
        blocked_count = sum(1 for act in activities if act.status == 'blocked')
        
        return DailyReport(
            date=date,
            activities=activities,
            total_hours=total_hours,
            projects_worked=projects_worked,
            tickets_updated=tickets_updated,
            completed_count=completed_count,
            in_progress_count=in_progress_count,
            blocked_count=blocked_count
        )
    
    def format_report_text(self, report: DailyReport, style: str = 'professional') -> str:
        """
        Format report as text
        
        Args:
            report: DailyReport to format
            style: 'professional', 'casual', or 'detailed'
            
        Returns:
            Formatted report text
        """
        date_str = report.date.strftime('%A, %B %d, %Y')
        
        if style == 'professional':
            return self._format_professional(report, date_str)
        elif style == 'casual':
            return self._format_casual(report, date_str)
        else:
            return self._format_detailed(report, date_str)
    
    def _format_professional(self, report: DailyReport, date_str: str) -> str:
        """Format in professional business style"""
        lines = []
        
        lines.append(f"Daily Status Report - {date_str}")
        lines.append("=" * 70)
        lines.append("")
        
        # Summary section
        lines.append("SUMMARY")
        lines.append("-" * 70)
        lines.append(f"Total Hours Logged: {report.total_hours:.1f}h")
        lines.append(f"Tasks Completed: {report.completed_count}")
        lines.append(f"Tasks In Progress: {report.in_progress_count}")
        if report.blocked_count > 0:
            lines.append(f"Tasks Blocked: {report.blocked_count}")
        lines.append(f"Projects: {', '.join(report.projects_worked)}")
        lines.append("")
        
        # Activities by project
        if report.activities:
            lines.append("ACTIVITIES")
            lines.append("-" * 70)
            
            # Group by project
            by_project: Dict[str, List[ActivitySummary]] = {}
            for act in report.activities:
                if act.project not in by_project:
                    by_project[act.project] = []
                by_project[act.project].append(act)
            
            for project, activities in sorted(by_project.items()):
                lines.append(f"\n{project}:")
                for act in activities:
                    status_emoji = {
                        'completed': '✓',
                        'in_progress': '→',
                        'blocked': '✗'
                    }.get(act.status, '•')
                    
                    ticket_str = f"[{act.ticket_id}] " if act.ticket_id else ""
                    time_str = f" ({act.time_spent:.1f}h)" if act.time_spent > 0 else ""
                    
                    lines.append(f"  {status_emoji} {ticket_str}{act.description}{time_str}")
        
        lines.append("")
        lines.append("-" * 70)
        lines.append(f"Report generated at {datetime.now().strftime('%I:%M %p')}")
        
        return "\n".join(lines)
    
    def _format_casual(self, report: DailyReport, date_str: str) -> str:
        """Format in casual friendly style"""
        lines = []
        
        lines.append(f"📊 Daily Update - {date_str}")
        lines.append("")
        
        # Quick summary
        lines.append("Quick Summary:")
        lines.append(f"⏱️  {report.total_hours:.1f} hours logged today")
        lines.append(f"✅ {report.completed_count} tasks completed")
        if report.in_progress_count > 0:
            lines.append(f"🔄 {report.in_progress_count} tasks in progress")
        if report.blocked_count > 0:
            lines.append(f"🚫 {report.blocked_count} tasks blocked (need help!)")
        lines.append("")
        
        # What I worked on
        if report.activities:
            lines.append("What I worked on:")
            
            by_project: Dict[str, List[ActivitySummary]] = {}
            for act in report.activities:
                if act.project not in by_project:
                    by_project[act.project] = []
                by_project[act.project].append(act)
            
            for project, activities in sorted(by_project.items()):
                lines.append(f"\n{project}:")
                for act in activities:
                    status_emoji = {
                        'completed': '✅',
                        'in_progress': '🔄',
                        'blocked': '🚫'
                    }.get(act.status, '📌')
                    
                    ticket_str = f"{act.ticket_id} - " if act.ticket_id else ""
                    lines.append(f"  {status_emoji} {ticket_str}{act.description}")
        
        lines.append("")
        lines.append("Let me know if you have any questions!")
        
        return "\n".join(lines)
    
    def _format_detailed(self, report: DailyReport, date_str: str) -> str:
        """Format with detailed timestamps and metadata"""
        lines = []
        
        lines.append(f"Detailed Activity Report - {date_str}")
        lines.append("=" * 70)
        lines.append("")
        
        # Statistics
        lines.append("STATISTICS")
        lines.append("-" * 70)
        lines.append(f"Total Activities: {len(report.activities)}")
        lines.append(f"Total Hours: {report.total_hours:.2f}h")
        lines.append(f"Projects: {len(report.projects_worked)}")
        lines.append(f"Tickets: {len(report.tickets_updated)}")
        lines.append(f"Completed: {report.completed_count}")
        lines.append(f"In Progress: {report.in_progress_count}")
        lines.append(f"Blocked: {report.blocked_count}")
        lines.append("")
        
        # Detailed activities
        if report.activities:
            lines.append("DETAILED ACTIVITIES")
            lines.append("-" * 70)
            
            for i, act in enumerate(report.activities, 1):
                time_str = act.timestamp.strftime('%I:%M %p')
                status_str = act.status.replace('_', ' ').title()
                
                lines.append(f"\n{i}. {time_str} - {act.project}")
                if act.ticket_id:
                    lines.append(f"   Ticket: {act.ticket_id}")
                lines.append(f"   Status: {status_str}")
                lines.append(f"   Description: {act.description}")
                if act.time_spent > 0:
                    lines.append(f"   Time: {act.time_spent:.1f}h")
                lines.append(f"   Source: {act.source}")
        
        lines.append("")
        lines.append("=" * 70)
        
        return "\n".join(lines)
    
    def format_report_html(self, report: DailyReport) -> str:
        """
        Format report as HTML for email
        
        Args:
            report: DailyReport to format
            
        Returns:
            HTML formatted report
        """
        date_str = report.date.strftime('%A, %B %d, %Y')
        
        html = f"""
<!DOCTYPE html>
<html>
<head>
    <style>
        body {{
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }}
        .container {{
            background-color: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }}
        h1 {{
            color: #2c3e50;
            border-bottom: 3px solid #3498db;
            padding-bottom: 10px;
        }}
        h2 {{
            color: #34495e;
            margin-top: 25px;
        }}
        .summary {{
            background-color: #ecf0f1;
            padding: 15px;
            border-radius: 5px;
            margin: 20px 0;
        }}
        .summary-item {{
            margin: 8px 0;
            font-size: 16px;
        }}
        .project-section {{
            margin: 20px 0;
            padding: 15px;
            background-color: #f8f9fa;
            border-left: 4px solid #3498db;
        }}
        .project-title {{
            font-weight: bold;
            color: #2c3e50;
            font-size: 18px;
            margin-bottom: 10px;
        }}
        .activity {{
            margin: 10px 0;
            padding: 10px;
            background-color: white;
            border-radius: 4px;
        }}
        .status-completed {{ color: #27ae60; }}
        .status-progress {{ color: #f39c12; }}
        .status-blocked {{ color: #e74c3c; }}
        .footer {{
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #ecf0f1;
            color: #7f8c8d;
            font-size: 14px;
        }}
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 Daily Status Report</h1>
        <p style="color: #7f8c8d; font-size: 18px;">{date_str}</p>
        
        <div class="summary">
            <h2>Summary</h2>
            <div class="summary-item">⏱️ <strong>Total Hours:</strong> {report.total_hours:.1f}h</div>
            <div class="summary-item">✅ <strong>Completed:</strong> {report.completed_count} tasks</div>
            <div class="summary-item">🔄 <strong>In Progress:</strong> {report.in_progress_count} tasks</div>
"""
        
        if report.blocked_count > 0:
            html += f"""
            <div class="summary-item">🚫 <strong>Blocked:</strong> {report.blocked_count} tasks</div>
"""
        
        html += f"""
            <div class="summary-item">📁 <strong>Projects:</strong> {', '.join(report.projects_worked)}</div>
        </div>
        
        <h2>Activities</h2>
"""
        
        # Group by project
        by_project: Dict[str, List[ActivitySummary]] = {}
        for act in report.activities:
            if act.project not in by_project:
                by_project[act.project] = []
            by_project[act.project].append(act)
        
        for project, activities in sorted(by_project.items()):
            html += f"""
        <div class="project-section">
            <div class="project-title">{project}</div>
"""
            
            for act in activities:
                status_class = {
                    'completed': 'completed',
                    'in_progress': 'progress',
                    'blocked': 'blocked'
                }.get(act.status, 'progress')
                
                status_emoji = {
                    'completed': '✓',
                    'in_progress': '→',
                    'blocked': '✗'
                }.get(act.status, '•')
                
                ticket_str = f'<strong>[{act.ticket_id}]</strong> ' if act.ticket_id else ''
                time_str = f' <em>({act.time_spent:.1f}h)</em>' if act.time_spent > 0 else ''
                
                html += f"""
            <div class="activity">
                <span class="status-{status_class}">{status_emoji}</span>
                {ticket_str}{act.description}{time_str}
            </div>
"""
            
            html += """
        </div>
"""
        
        html += f"""
        <div class="footer">
            Report generated automatically at {datetime.now().strftime('%I:%M %p')}
        </div>
    </div>
</body>
</html>
"""
        
        return html
    
    async def send_email_report(
        self,
        recipient: str,
        report: DailyReport,
        subject: Optional[str] = None,
        format: str = 'html'
    ) -> bool:
        """
        Send email report via Microsoft Graph
        
        Args:
            recipient: Email address to send to
            report: DailyReport to send
            subject: Email subject (auto-generated if None)
            format: 'html' or 'text'
            
        Returns:
            True if sent successfully
        """
        if not self.graph_client:
            print("Error: Graph client not initialized")
            return False
        
        # Generate subject
        if subject is None:
            date_str = report.date.strftime('%B %d, %Y')
            subject = f"Daily Status Report - {date_str}"
        
        # Generate body
        if format == 'html':
            body = self.format_report_html(report)
        else:
            body = self.format_report_text(report, style='professional')
        
        try:
            await self.graph_client.send_mail(subject, body, recipient)
            print(f"✓ Report sent to {recipient}")
            return True
        except Exception as e:
            print(f"✗ Failed to send report: {e}")
            return False
    
    def save_report(self, report: DailyReport, output_path: Optional[str] = None):
        """
        Save report to file
        
        Args:
            report: DailyReport to save
            output_path: Path to save to (auto-generated if None)
        """
        if output_path is None:
            date_str = report.date.strftime('%Y-%m-%d')
            try:
                from backend.config import reports_dir as get_reports_dir
                reports_dir = str(get_reports_dir())
            except ImportError:
                try:
                    from backend.utils.paths import fallback_path
                    reports_dir = fallback_path("Data", "reports")
                except ImportError:
                    cur = Path(__file__).resolve().parent.parent
                    reports_dir = str(cur / "Data" / "reports")
            os.makedirs(reports_dir, exist_ok=True)
            output_path = os.path.join(reports_dir, f"report-{date_str}.txt")
        
        content = self.format_report_text(report, style='detailed')
        
        with open(output_path, 'w') as f:
            f.write(content)
        
        print(f"✓ Report saved to {output_path}")


# CLI interface
async def main():
    """CLI for email reporter"""
    import sys
    from configparser import ConfigParser
    
    # Add msgraph path
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'msgraph_python'))
    from graph import Graph
    
    if len(sys.argv) < 2:
        print("Usage: python email_reporter.py <command> [options]")
        print()
        print("Commands:")
        print("  preview [date]           Preview today's report (or specific date YYYY-MM-DD)")
        print("  send <email> [date]      Send report to email address")
        print("  save [date]              Save report to file")
        print("  html [date]              Generate HTML report")
        print()
        return
    
    command = sys.argv[1]
    
    # Parse date if provided
    date = None
    date_arg_pos = 2 if command == 'preview' or command == 'save' or command == 'html' else 3
    if len(sys.argv) > date_arg_pos:
        try:
            date = datetime.strptime(sys.argv[date_arg_pos], '%Y-%m-%d')
        except ValueError:
            print(f"Invalid date format. Use YYYY-MM-DD")
            return
    
    reporter = EmailReporter()
    
    if command == 'preview':
        report = reporter.generate_daily_report(date)
        print(reporter.format_report_text(report, style='professional'))
    
    elif command == 'send':
        if len(sys.argv) < 3:
            print("Error: Email address required")
            return
        
        recipient = sys.argv[2]
        
        # Initialize Graph
        config = ConfigParser()
        config_path = os.path.join(os.path.dirname(__file__), 'msgraph_python', 'config.cfg')
        
        if not os.path.exists(config_path):
            print("Error: MS Graph config not found")
            return
        
        config.read(config_path)
        graph = Graph(config['azure'])
        reporter.initialize_graph(graph)
        
        report = reporter.generate_daily_report(date)
        await reporter.send_email_report(recipient, report)
    
    elif command == 'save':
        report = reporter.generate_daily_report(date)
        reporter.save_report(report)
    
    elif command == 'html':
        report = reporter.generate_daily_report(date)
        print(reporter.format_report_html(report))
    
    else:
        print(f"Unknown command: {command}")


if __name__ == "__main__":
    import asyncio
    asyncio.run(main())
