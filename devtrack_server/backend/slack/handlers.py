"""DevTrack Slack command handlers.

All commands are accessed via a single Slack slash command:
    /devtrack <subcommand> [args]

Or via @devtrack mention:
    @devtrack <subcommand> [args]

Business logic is shared with the Telegram handlers — same SQLite DB,
same PM clients, commit-based reports.
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
        "\n*Work Reports*\n"
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
        from backend.health_snapshots_store import get_recent_snapshots
        rows = get_recent_snapshots(limit=20)
        if not rows:
            respond("No health data available yet.")
            return
        seen: dict[str, dict] = {}
        for row in rows:
            if row["service"] not in seen:
                seen[row["service"]] = row
        lines = ["*Service Health*"]
        icons = {"up": ":green_circle:", "down": ":red_circle:", "degraded": ":yellow_circle:"}
        for row in seen.values():
            icon = icons.get(row["status"], ":white_circle:")
            lat = f"  {row['latency_ms']}ms" if row["latency_ms"] else ""
            lines.append(f"{icon} *{row['service']}*: {row['status']}{lat}")
        respond("\n".join(lines))
    except Exception as e:
        respond(f":x: Health check error: {e}")


@_cmd("commits")
def cmd_commits(args: str, respond: Callable, bot: "SlackBot") -> None:
    out = _run(["commits", "pending"])
    respond(f"```{out}```" if out else "No pending commits.")


# ── Commit-based work reports ─────────────────────────────────────────────────









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
        import asyncio
        report = asyncio.run(generator.generate(date.today().isoformat()))

        lines = [report.as_text()]

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
        from backend.vacation.auto_responder import get_vacation_state, set_vacation_state

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
            if not set_vacation_state(True, until=until, confidence_threshold=threshold, auto_submit=auto_submit):
                respond(":x: Could not enable vacation mode (not available yet in this server mode).")
                return
            msg = ":airplane: Vacation mode *ON*"
            if until:
                msg += f" (until {until})"
            respond(msg)
        elif sub == "off":
            if not set_vacation_state(False):
                respond(":x: Could not disable vacation mode (not available yet in this server mode).")
                return
            respond(":white_check_mark: Vacation mode *OFF* — normal prompting resumed")
        else:
            respond("Usage: `/devtrack vacation [on|off|status] [--until YYYY-MM-DD] [--threshold 0.7] [--no-submit]`")
    except Exception as e:
        respond(f":x: Error: {e}")
