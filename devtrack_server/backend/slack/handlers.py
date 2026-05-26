"""DevTrack Slack command handlers.

All commands are accessed via a single Slack slash command:
    /devtrack <subcommand> [args]

Or via @devtrack mention:
    @devtrack <subcommand> [args]

Business logic is shared with the Telegram handlers — same SQLite DB,
same PM clients, same WorkSessionStore.
"""
from __future__ import annotations

import logging
import os
import subprocess
from datetime import date, datetime, timezone
from typing import TYPE_CHECKING, Callable

if TYPE_CHECKING:
    from backend.slack.bot import SlackBot

logger = logging.getLogger(__name__)

# ── Subcommand registry ────────────────────────────────────────────────────────

_SUBCOMMANDS: dict[str, Callable] = {}


def _cmd(name: str):
    """Decorator to register a subcommand handler."""
    def decorator(fn: Callable) -> Callable:
        _SUBCOMMANDS[name] = fn
        return fn
    return decorator


def dispatch(text: str, respond: Callable, bot: "SlackBot") -> None:
    """Parse <subcommand> [args] and call the matching handler."""
    parts = text.split(None, 1)
    subcommand = parts[0].lower() if parts else "help"
    args_str = parts[1].strip() if len(parts) > 1 else ""

    handler = _SUBCOMMANDS.get(subcommand)
    if handler is None:
        respond(
            f":question: Unknown subcommand *{subcommand}*.\n"
            "Use `/devtrack help` to see available commands."
        )
        return

    try:
        handler(args_str, respond, bot)
    except Exception as e:
        logger.exception(f"Error in /devtrack {subcommand}: {e}")
        respond(f":x: Error: {e}")


# ── Helpers ────────────────────────────────────────────────────────────────────

def _devtrack_bin() -> str:
    from backend.config import get_project_root
    root = get_project_root()
    if root:
        candidate = os.path.join(root, "devtrack")
        if os.path.isfile(candidate) and os.access(candidate, os.X_OK):
            return candidate
    return "devtrack"


def _run(args: list[str], timeout: int = 10) -> str:
    """Run devtrack CLI and return stdout."""
    env = os.environ.copy()
    root = env.get("PROJECT_ROOT", "")
    if root and "DEVTRACK_ENV_FILE" not in env:
        env["DEVTRACK_ENV_FILE"] = os.path.join(root, ".env")
    try:
        result = subprocess.run(
            [_devtrack_bin()] + args,
            capture_output=True,
            text=True,
            timeout=timeout,
            env=env,
        )
        return (result.stdout + result.stderr).strip()
    except subprocess.TimeoutExpired:
        return "Command timed out."
    except FileNotFoundError:
        return "devtrack binary not found."


def _fmt_duration(minutes: int) -> str:
    h, m = divmod(max(0, minutes), 60)
    return f"{h}h {m}m" if h else f"{m}m"


# ── Commands ───────────────────────────────────────────────────────────────────

@_cmd("help")
def cmd_help(args: str, respond: Callable, bot: "SlackBot") -> None:
    """Show available commands."""
    respond(
        "*DevTrack Slack Commands* (`/devtrack <subcommand>`)\n\n"
        "*Daemon*\n"
        "  `status`         — Daemon status\n"
        "  `logs`           — Recent log lines\n"
        "  `trigger`        — Force immediate work update\n"
        "  `health`         — Service health check\n"
        "\n*Work Sessions*\n"
        "  `workstart [ticket-ref]`   — Start a work session\n"
        "  `workstop`                 — Stop active session\n"
        "  `workadjust <minutes>`     — Override session time\n"
        "  `workstatus`               — Today's sessions\n"
        "  `workreport [--email addr]` — Generate EOD report\n"
        "\n*Integrations*\n"
        "  `commits`        — Recent commits\n"
        "  `github`         — GitHub issues assigned to you\n"
        "  `gitlab`         — GitLab issues assigned to you\n"
        "\n*Other*\n"
        "  `help`           — This message"
    )


@_cmd("status")
def cmd_status(args: str, respond: Callable, bot: "SlackBot") -> None:
    out = _run(["status"])
    respond(f"```{out}```" if out else "Daemon is not running.")


@_cmd("logs")
def cmd_logs(args: str, respond: Callable, bot: "SlackBot") -> None:
    lines = args.strip() or "30"
    out = _run(["logs", "--lines", lines] if lines.isdigit() else ["logs"])
    respond(f"```{out[-3000:]}```" if out else "No logs available.")


@_cmd("trigger")
def cmd_trigger(args: str, respond: Callable, bot: "SlackBot") -> None:
    out = _run(["force-trigger"], timeout=5)
    respond(f":zap: Trigger sent.\n```{out}```" if out else ":zap: Trigger sent.")


@_cmd("health")
def cmd_health(args: str, respond: Callable, bot: "SlackBot") -> None:
    try:
        import sqlite3
        from backend.config import database_path
        conn = sqlite3.connect(str(database_path()))
        rows = conn.execute(
            "SELECT service, status, latency_ms, checked_at "
            "FROM health_snapshots "
            "ORDER BY checked_at DESC LIMIT 20"
        ).fetchall()
        conn.close()
        if not rows:
            respond("No health data available yet.")
            return
        seen: dict[str, tuple] = {}
        for row in rows:
            if row[0] not in seen:
                seen[row[0]] = row
        lines = ["*Service Health*"]
        icons = {"up": ":green_circle:", "down": ":red_circle:", "degraded": ":yellow_circle:"}
        for svc, status, latency, checked_at in seen.values():
            icon = icons.get(status, ":white_circle:")
            lat = f"  {latency}ms" if latency else ""
            lines.append(f"{icon} *{svc}*: {status}{lat}")
        respond("\n".join(lines))
    except Exception as e:
        respond(f":x: Health check error: {e}")


@_cmd("commits")
def cmd_commits(args: str, respond: Callable, bot: "SlackBot") -> None:
    out = _run(["commits", "pending"])
    respond(f"```{out}```" if out else "No pending commits.")


# ── Work session commands ─────────────────────────────────────────────────────

@_cmd("workstart")
def cmd_workstart(args: str, respond: Callable, bot: "SlackBot") -> None:
    ticket_ref = args.strip()
    try:
        from backend.work_tracker.session_store import WorkSessionStore
        from backend.config import database_path
        import sqlite3

        store = WorkSessionStore()
        active = store.get_active_session()
        if active:
            msg = f":warning: A session is already active (ID {active['id']}"
            if active.get("ticket_ref"):
                msg += f", ticket: `{active['ticket_ref']}`"
            msg += "). Use `workstop` first."
            respond(msg)
            return

        conn = sqlite3.connect(str(database_path()))
        cur = conn.execute(
            "INSERT INTO work_sessions (started_at, ticket_ref, commits) VALUES (datetime('now'), ?, '[]')",
            (ticket_ref,),
        )
        session_id = cur.lastrowid
        conn.commit()
        conn.close()

        msg = f":white_check_mark: Work session started (ID {session_id})"
        if ticket_ref:
            msg += f" for *{ticket_ref}*"
        msg += "\nUse `workstop` when you're done."
        respond(msg)
    except Exception as e:
        respond(f":x: Error starting session: {e}")


@_cmd("workstop")
def cmd_workstop(args: str, respond: Callable, bot: "SlackBot") -> None:
    try:
        from backend.work_tracker.session_store import WorkSessionStore
        store = WorkSessionStore()
        active = store.get_active_session()
        if not active:
            respond("No active session found.")
            return

        store.end_session(active["id"])

        started_at = active.get("started_at", "")
        duration_str = ""
        try:
            start = datetime.fromisoformat(started_at)
            elapsed = int((datetime.now(timezone.utc).replace(tzinfo=None) - start).total_seconds() / 60)
            duration_str = _fmt_duration(elapsed)
        except Exception:
            pass

        msg = f":white_check_mark: Session {active['id']} stopped."
        if duration_str:
            msg += f" Duration: *{duration_str}*"
        if active.get("ticket_ref"):
            msg += f" (ticket: `{active['ticket_ref']}`)"
        msg += "\nUse `workadjust <minutes>` to correct the time."
        respond(msg)
    except Exception as e:
        respond(f":x: Error stopping session: {e}")


@_cmd("workadjust")
def cmd_workadjust(args: str, respond: Callable, bot: "SlackBot") -> None:
    if not args.strip().isdigit():
        respond("Usage: `/devtrack workadjust <minutes>`  (e.g. `workadjust 90`)")
        return
    minutes = int(args.strip())
    try:
        from backend.work_tracker.session_store import WorkSessionStore
        store = WorkSessionStore()
        active = store.get_active_session()
        if active:
            target_id = active["id"]
        else:
            sessions = store.get_sessions_for_date(date.today().isoformat())
            if not sessions:
                respond("No session found to adjust.")
                return
            target_id = sessions[-1]["id"]

        store.adjust_time(target_id, minutes)
        respond(
            f":white_check_mark: Session {target_id} time adjusted to *{_fmt_duration(minutes)}*.\n"
            "_Auto-measured time preserved in DB for audit._"
        )
    except Exception as e:
        respond(f":x: Error adjusting session: {e}")


@_cmd("workstatus")
def cmd_workstatus(args: str, respond: Callable, bot: "SlackBot") -> None:
    try:
        from backend.work_tracker.session_store import WorkSessionStore
        store = WorkSessionStore()
        active = store.get_active_session()
        today = date.today().isoformat()
        sessions = store.get_sessions_for_date(today)

        lines: list[str] = []
        if active:
            started_at = active.get("started_at", "")
            elapsed_str = ""
            try:
                start = datetime.fromisoformat(started_at)
                elapsed = int((datetime.now(timezone.utc).replace(tzinfo=None) - start).total_seconds() / 60)
                elapsed_str = _fmt_duration(elapsed)
            except Exception:
                pass
            line = f":large_green_circle: *Active* (ID {active['id']}) — running {elapsed_str}"
            if active.get("ticket_ref"):
                line += f"\n   Ticket: `{active['ticket_ref']}`"
            lines.append(line)
        else:
            lines.append(":white_circle: No active session.")

        completed = [s for s in sessions if s.get("ended_at")]
        if completed:
            lines.append(f"\n*Today ({len(completed)} session(s)):*")
            total = 0
            for s in completed:
                dur = WorkSessionStore.effective_duration(s)
                total += dur
                adj = " :pencil2:" if s.get("adjusted_minutes") is not None else ""
                ticket = s.get("ticket_ref") or "(no ticket)"
                lines.append(f"  • {_fmt_duration(dur)}{adj}  {ticket}")
            lines.append(f"  Total: *{_fmt_duration(total)}*")

        respond("\n".join(lines))
    except Exception as e:
        respond(f":x: Error loading work status: {e}")


@_cmd("workreport")
def cmd_workreport(args: str, respond: Callable, bot: "SlackBot") -> None:
    email_to = None
    for part in args.split():
        if "@" in part and "." in part:
            email_to = part
            break

    try:
        from backend.work_tracker.eod_report_generator import EODReportGenerator
        generator = EODReportGenerator()
        report = generator.generate(date.today().isoformat(), include_ai=True)

        lines: list[str] = [f"*EOD Report — {report.date}*\n"]
        for s in report.sessions:
            dur = _fmt_duration(s.get("duration_minutes") or 0)
            ticket = s.get("ticket_ref") or "(no ticket)"
            commits = len(s.get("commits") or [])
            lines.append(f"  • {dur}  {ticket}  ({commits} commit(s))")

        if report.total_minutes:
            lines.append(f"\nTotal: *{_fmt_duration(report.total_minutes)}*")

        if report.ai_narrative:
            narrative = report.ai_narrative[:800]
            lines.append(f"\n_{narrative}_")

        if email_to:
            try:
                from backend.work_tracker.eod_emailer import EODEmailer
                sent = EODEmailer().send(report, email_to)
                if sent:
                    lines.append(f"\n:envelope: Report emailed to {email_to}")
            except Exception as e:
                lines.append(f"\n:warning: Email failed: {e}")

        respond("\n".join(lines))
    except Exception as e:
        respond(f":x: Error generating report: {e}")


# ── GitHub / GitLab ───────────────────────────────────────────────────────────

@_cmd("github")
def cmd_github(args: str, respond: Callable, bot: "SlackBot") -> None:
    try:
        from backend.github.client import GitHubClient
        import asyncio
        client = GitHubClient()
        if not client.is_configured():
            respond(":warning: GitHub is not configured.")
            return
        issues = asyncio.run(client.get_my_issues(max_results=10))
        if not issues:
            respond("No open GitHub issues assigned to you.")
            return
        lines = ["*GitHub Issues assigned to you:*"]
        for issue in issues:
            lines.append(f"  • #{issue.number} — {issue.title[:70]}")
        respond("\n".join(lines))
    except Exception as e:
        respond(f":x: GitHub error: {e}")


@_cmd("gitlab")
def cmd_gitlab(args: str, respond: Callable, bot: "SlackBot") -> None:
    try:
        from backend.gitlab.client import GitLabClient
        import asyncio
        client = GitLabClient()
        if not client.is_configured():
            respond(":warning: GitLab is not configured.")
            return
        issues = asyncio.run(client.get_my_issues(max_results=10))
        if not issues:
            respond("No open GitLab issues assigned to you.")
            return
        lines = ["*GitLab Issues assigned to you:*"]
        for issue in issues:
            lines.append(f"  • !{issue.iid} — {issue.title[:70]}")
        respond("\n".join(lines))
    except Exception as e:
        respond(f":x: GitLab error: {e}")


@_cmd("vacation")
def cmd_vacation(args: str, respond: Callable, bot: "SlackBot") -> None:
    """vacation [on|off|status] — toggle vacation auto-responder mode."""
    parts = args.split() if args else []
    sub = parts[0].lower() if parts else "status"

    try:
        from backend.vacation.auto_responder import get_vacation_state
        import sqlite3
        from backend.config import get_path, get

        def _db():
            db_path = get_path("DATABASE_DIR") / get("DATABASE_FILE_NAME")
            return sqlite3.connect(str(db_path))

        if sub == "status":
            state = get_vacation_state()
            if not state or not state.enabled:
                respond(":airplane: Vacation mode: *OFF*")
            else:
                until = f"until {state.until}" if state.until else "indefinite"
                respond(
                    f":airplane: Vacation mode: *ON* ({until})\n"
                    f"Confidence threshold: {state.confidence_threshold:.0%} | "
                    f"Auto-submit: {'yes' if state.auto_submit else 'no'}"
                )
        elif sub == "on":
            until = ""
            threshold = 0.7
            auto_submit = True
            i = 1
            while i < len(parts):
                if parts[i] == "--until" and i + 1 < len(parts):
                    i += 1; until = parts[i]
                elif parts[i] == "--threshold" and i + 1 < len(parts):
                    i += 1; threshold = float(parts[i])
                elif parts[i] == "--no-submit":
                    auto_submit = False
                i += 1
            from datetime import datetime, timezone
            enabled_at = datetime.now(timezone.utc).isoformat()
            conn = _db()
            conn.execute(
                "UPDATE vacation_mode SET enabled=1, enabled_at=?, until=?, confidence_threshold=?, auto_submit=? WHERE id=1",
                (enabled_at, until, threshold, int(auto_submit)),
            )
            conn.commit(); conn.close()
            msg = ":airplane: Vacation mode *ON*"
            if until:
                msg += f" (until {until})"
            respond(msg)
        elif sub == "off":
            conn = _db()
            conn.execute("UPDATE vacation_mode SET enabled=0 WHERE id=1")
            conn.commit(); conn.close()
            respond(":white_check_mark: Vacation mode *OFF* — normal prompting resumed")
        else:
            respond("Usage: `/devtrack vacation [on|off|status] [--until YYYY-MM-DD] [--threshold 0.7] [--no-submit]`")
    except Exception as e:
        respond(f":x: Error: {e}")
