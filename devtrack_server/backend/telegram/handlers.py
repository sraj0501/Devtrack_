"""Telegram bot command handlers for DevTrack."""
import json
import logging
import os
import re
import sqlite3
import subprocess
from typing import TYPE_CHECKING

from telegram import InlineKeyboardButton, InlineKeyboardMarkup, Update
from telegram.ext import Application, CallbackQueryHandler, CommandHandler, ContextTypes

if TYPE_CHECKING:
    from backend.telegram.bot import DevTrackBot

logger = logging.getLogger(__name__)

# Maximum Telegram message length
MAX_MSG_LEN = 4096

# Module-level pending stores for multi-step flows
_PENDING_GITLAB_CREATES: dict = {}  # chat_id -> title
_PENDING_PLANS: dict = {}           # chat_id -> {"problem": str, "plan": ..., "platform": str}
_PENDING_PROJECTS: dict = {}        # chat_id -> ProjectDraft state for /newproject flow


def _devtrack_bin() -> str:
    """Resolve the devtrack binary path from PROJECT_ROOT."""
    from backend.config import get_project_root
    project_root = get_project_root()
    if project_root:
        candidate = os.path.join(project_root, "devtrack")
        if os.path.isfile(candidate) and os.access(candidate, os.X_OK):
            return candidate
    # Fallback: assume it's in PATH
    return "devtrack"


def _devtrack_env() -> dict:
    """Build subprocess environment with DEVTRACK_ENV_FILE set."""
    env = os.environ.copy()
    if "DEVTRACK_ENV_FILE" not in env:
        project_root = env.get("PROJECT_ROOT", "")
        if project_root:
            env["DEVTRACK_ENV_FILE"] = os.path.join(project_root, ".env")
    return env


def register_handlers(app: Application, bot: "DevTrackBot"):
    """Register all command handlers."""
    app.add_handler(CommandHandler("start", _cmd_start))
    app.add_handler(CommandHandler("help", _cmd_help))
    app.add_handler(CommandHandler("status", _make_handler(bot, _cmd_status)))
    app.add_handler(CommandHandler("logs", _make_handler(bot, _cmd_logs)))
    app.add_handler(CommandHandler("trigger", _make_handler(bot, _cmd_trigger)))
    app.add_handler(CommandHandler("pause", _make_handler(bot, _cmd_pause)))
    app.add_handler(CommandHandler("resume", _make_handler(bot, _cmd_resume)))
    app.add_handler(CommandHandler("queue", _make_handler(bot, _cmd_queue)))
    app.add_handler(CommandHandler("commits", _make_handler(bot, _cmd_commits)))
    app.add_handler(CommandHandler("health", _make_handler(bot, _cmd_health)))
    app.add_handler(CommandHandler("issues", _make_handler(bot, _cmd_issues)))
    app.add_handler(CommandHandler("issue", _make_handler(bot, _cmd_issue)))
    app.add_handler(CommandHandler("create", _make_handler(bot, _cmd_create)))
    app.add_handler(CallbackQueryHandler(_on_sprint_selected, pattern=r"^sprint:"))
    app.add_handler(CallbackQueryHandler(_on_view_issue, pattern=r"^view_issue:"))
    # GitLab commands
    app.add_handler(CommandHandler("gitlab", _make_handler(bot, _cmd_gitlab)))
    app.add_handler(CommandHandler("gitlabissue", _make_handler(bot, _cmd_gitlab_issue)))
    app.add_handler(CommandHandler("gitlabcreate", _make_handler(bot, _cmd_gitlab_create)))
    app.add_handler(CallbackQueryHandler(_on_view_gitlab_issue, pattern=r"^view_gitlab_issue:"))
    app.add_handler(CallbackQueryHandler(_on_gitlab_milestone_selected, pattern=r"^gitlab_milestone:"))
    # GitHub commands
    app.add_handler(CommandHandler("github", _make_handler(bot, _cmd_github)))
    app.add_handler(CommandHandler("githubissue", _make_handler(bot, _cmd_github_issue)))
    app.add_handler(CommandHandler("githubcreate", _make_handler(bot, _cmd_github_create)))
    # PM Agent commands
    app.add_handler(CommandHandler("plan", _make_handler(bot, _cmd_plan)))
    app.add_handler(CallbackQueryHandler(_on_plan_platform_selected, pattern=r"^plan_platform:"))
    app.add_handler(CallbackQueryHandler(_on_plan_confirmed, pattern=r"^plan_confirm:"))
    app.add_handler(CallbackQueryHandler(_on_plan_cancelled, pattern=r"^plan_cancel:"))
    # AI Project Planning
    app.add_handler(CommandHandler("newproject", _make_handler(bot, _cmd_newproject)))
    app.add_handler(CallbackQueryHandler(_on_newproject_platform, pattern=r"^np_platform:"))
    app.add_handler(CallbackQueryHandler(_on_newproject_member_toggle, pattern=r"^np_member:"))
    app.add_handler(CallbackQueryHandler(_on_newproject_members_done, pattern=r"^np_done:"))
    app.add_handler(CallbackQueryHandler(_on_newproject_approve, pattern=r"^np_approve:"))
    app.add_handler(CallbackQueryHandler(_on_newproject_revise, pattern=r"^np_revise:"))
    app.add_handler(CallbackQueryHandler(_on_newproject_cancel, pattern=r"^np_cancel:"))
    # Work session tracking
    app.add_handler(CommandHandler("workstart", _make_handler(bot, _cmd_workstart)))
    app.add_handler(CommandHandler("workstop", _make_handler(bot, _cmd_workstop)))
    app.add_handler(CommandHandler("workadjust", _make_handler(bot, _cmd_workadjust)))
    app.add_handler(CommandHandler("workstatus", _make_handler(bot, _cmd_workstatus)))
    app.add_handler(CommandHandler("workreport", _make_handler(bot, _cmd_workreport)))
    app.add_handler(CommandHandler("vacation", _make_handler(bot, _cmd_vacation)))


def _make_handler(bot: "DevTrackBot", handler_fn):
    """Wrap a handler with auth check."""
    async def wrapper(update: Update, context: ContextTypes.DEFAULT_TYPE):
        if not await bot._check_auth(update):
            return
        await handler_fn(update, context, bot)
    return wrapper


async def _cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Handle /start command (no auth needed)."""
    chat_id = update.effective_chat.id
    await update.message.reply_text(
        f"DevTrack Bot\n\n"
        f"Your chat ID: `{chat_id}`\n\n"
        f"Add this to `TELEGRAM_ALLOWED_CHAT_IDS` in your `.env` to authorize.\n\n"
        f"Use /help for available commands.",
        parse_mode="Markdown"
    )


async def _cmd_help(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Handle /help command (no auth needed)."""
    await update.message.reply_text(
        "*DevTrack Bot Commands*\n\n"
        "/status -- Daemon status and service health\n"
        "/logs -- Recent daemon log lines\n"
        "/trigger -- Force an immediate work update trigger\n"
        "/pause -- Pause the scheduler\n"
        "/resume -- Resume the scheduler\n"
        "/queue -- Message queue statistics\n"
        "/commits -- Deferred commit status\n"
        "/health -- Detailed service health\n"
        "/issues -- Azure DevOps work items assigned to you\n"
        "/issue <id> -- Full details of a specific work item\n"
        "/create <title> -- Create a Task (prefix with 'bug' or 'task' to set type)\n"
        "/gitlab -- GitLab issues assigned to me (from cache)\n"
        "/gitlabissue <project\\_id> <iid> -- Fetch a single issue live\n"
        "/gitlabcreate <title> -- Create a new GitLab issue\n"
        "/github -- GitHub issues assigned to me\n"
        "/githubissue <number> -- Full details of a specific issue\n"
        "/githubcreate <title> -- Create a new GitHub issue\n"
        "/plan <problem statement> -- Decompose into work items and create in PM platform\n"
        "/help -- Show this message",
        parse_mode="Markdown"
    )


async def _cmd_status(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /status -- show daemon status."""
    try:
        result = subprocess.run(
            [_devtrack_bin(), "status"],
            capture_output=True, text=True, timeout=10, env=_devtrack_env()
        )
        output = result.stdout or result.stderr or "No output"
        # Truncate if too long
        if len(output) > MAX_MSG_LEN - 20:
            output = output[:MAX_MSG_LEN - 20] + "\n... (truncated)"
        await update.message.reply_text(f"```\n{output}\n```", parse_mode="Markdown")
    except subprocess.TimeoutExpired:
        await update.message.reply_text("Status command timed out")
    except FileNotFoundError:
        await update.message.reply_text("`devtrack` binary not found in PATH")
    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_logs(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /logs -- show recent log lines."""
    try:
        # Get line count from args (default 20)
        lines = 20
        if context.args:
            try:
                lines = min(int(context.args[0]), 50)  # Cap at 50
            except ValueError:
                pass

        result = subprocess.run(
            [_devtrack_bin(), "logs", "-n", str(lines)],
            capture_output=True, text=True, timeout=10, env=_devtrack_env()
        )
        output = result.stdout or result.stderr or "No logs available"
        if len(output) > MAX_MSG_LEN - 20:
            output = output[-(MAX_MSG_LEN - 20):]  # Keep tail
        await update.message.reply_text(f"```\n{output}\n```", parse_mode="Markdown")
    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_trigger(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /trigger -- force immediate work update."""
    try:
        result = subprocess.run(
            [_devtrack_bin(), "force-trigger"],
            capture_output=True, text=True, timeout=10, env=_devtrack_env()
        )
        output = result.stdout or result.stderr or "Trigger sent"
        await update.message.reply_text(output.strip())
    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_pause(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /pause -- pause scheduler."""
    try:
        result = subprocess.run(
            [_devtrack_bin(), "pause"],
            capture_output=True, text=True, timeout=10, env=_devtrack_env()
        )
        output = result.stdout or result.stderr or "Paused"
        await update.message.reply_text(output.strip())
    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_resume(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /resume -- resume scheduler."""
    try:
        result = subprocess.run(
            [_devtrack_bin(), "resume"],
            capture_output=True, text=True, timeout=10, env=_devtrack_env()
        )
        output = result.stdout or result.stderr or "Resumed"
        await update.message.reply_text(output.strip())
    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_queue(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /queue -- show message queue stats."""
    try:
        db_path = _get_db_path()
        if not db_path:
            await update.message.reply_text("Database not found")
            return

        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()

        cursor.execute("SELECT status, COUNT(*) FROM message_queue GROUP BY status")
        rows = cursor.fetchall()
        conn.close()

        if not rows:
            await update.message.reply_text("Message queue is empty")
            return

        text = "*Message Queue*\n\n"
        for status, count in rows:
            text += f"- {status}: {count}\n"

        await update.message.reply_text(text, parse_mode="Markdown")
    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_commits(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /commits -- show deferred commit status."""
    try:
        db_path = _get_db_path()
        if not db_path:
            await update.message.reply_text("Database not found")
            return

        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()

        cursor.execute("SELECT status, COUNT(*) FROM deferred_commits GROUP BY status")
        rows = cursor.fetchall()
        conn.close()

        if not rows:
            await update.message.reply_text("No deferred commits")
            return

        text = "*Deferred Commits*\n\n"
        for status, count in rows:
            text += f"- {status}: {count}\n"

        await update.message.reply_text(text, parse_mode="Markdown")
    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_health(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /health -- show detailed service health."""
    try:
        db_path = _get_db_path()
        if not db_path:
            await update.message.reply_text("Database not found")
            return

        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()

        # Get latest health snapshot per service
        cursor.execute("""
            SELECT service, status, latency_ms, details, checked_at
            FROM health_snapshots h1
            WHERE checked_at = (
                SELECT MAX(checked_at) FROM health_snapshots h2 WHERE h2.service = h1.service
            )
            ORDER BY service
        """)
        rows = cursor.fetchall()
        conn.close()

        if not rows:
            await update.message.reply_text("No health data available yet")
            return

        text = "*Service Health*\n\n"
        status_map = {
            "up": "UP", "down": "DOWN", "degraded": "DEGRADED", "unconfigured": "N/A"
        }

        for service, status, latency_ms, details, checked_at in rows:
            name = _service_display_name(service)
            label = status_map.get(status, status)
            line = f"*{name}*: {label}"
            if latency_ms and latency_ms > 0:
                line += f" ({latency_ms}ms)"
            text += line + "\n"

        await update.message.reply_text(text, parse_mode="Markdown")
    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_issues(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /issues -- list Azure DevOps work items assigned to me (from cache)."""
    try:
        state_file = _get_azure_state_file()
        if not state_file or not os.path.exists(state_file):
            await update.message.reply_text(
                "No Azure sync data found. Run <code>devtrack azure-sync</code> first.",
                parse_mode="HTML"
            )
            return

        with open(state_file) as f:
            data = json.load(f)

        work_items = data.get("work_items", {})
        last_sync = data.get("last_sync", "unknown")
        if last_sync and "T" in last_sync:
            last_sync = last_sync.split("T")[0]

        if not work_items:
            await update.message.reply_text(
                f"No work items found.\n<i>Last synced: {last_sync}</i>\n\n"
                "Run <code>devtrack azure-sync</code> to refresh.",
                parse_mode="HTML"
            )
            return

        by_state: dict = {}
        for item in work_items.values():
            state = item.get("state", "Unknown")
            by_state.setdefault(state, []).append(item)

        blocks = []
        for state in sorted(by_state.keys()):
            state_items = by_state[state]
            lines = [f"<b>{_h(state)}</b> ({len(state_items)})"]
            for item in state_items:
                item_id = item.get("id", "?")
                title = _h(item.get("title", "(no title)"))
                wtype = _h(item.get("type", ""))
                sprint_raw = item.get("iteration_path", "") or ""
                sprint = _h(sprint_raw.split("\\")[-1]) if sprint_raw else ""
                due = _h(item.get("due_date", "") or "")
                desc = _strip_html_entities(item.get("description", "") or "")
                if len(desc) > 150:
                    desc = desc[:150] + "..."

                lines.append(f"\n<code>#{item_id}</code> <b>{title}</b>")
                lines.append(f"  {wtype}  |  {state}")
                if sprint:
                    lines.append(f"  Sprint: {sprint}")
                if due:
                    lines.append(f"  Due: {due}")
                if desc:
                    lines.append(f"  <i>{_h(desc)}</i>")
            blocks.append("\n".join(lines))

        header = f"<b>Azure DevOps — My Issues</b> ({len(work_items)} total)\n\n"
        footer = f"\n\n<i>Synced: {last_sync}</i>  |  /issue &lt;id&gt; for full details"

        current = header
        for block in blocks:
            if len(current) + len(block) + len(footer) + 5 > MAX_MSG_LEN:
                await update.message.reply_text(current, parse_mode="HTML")
                current = block
            else:
                current = current + block + "\n\n"

        await update.message.reply_text(current + footer, parse_mode="HTML")

    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_issue(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /issue <id> -- fetch full work item details live from Azure DevOps."""
    if not context.args:
        await update.message.reply_text("Usage: /issue <work-item-id>")
        return

    try:
        item_id = int(context.args[0])
    except ValueError:
        await update.message.reply_text(f"Invalid ID: `{context.args[0]}`", parse_mode="Markdown")
        return

    await update.message.reply_text(f"Fetching work item #{item_id}...")

    try:
        from backend.azure.client import AzureDevOpsClient

        client = AzureDevOpsClient()
        if not client.is_configured():
            await update.message.reply_text("Azure DevOps is not configured. Check your `.env`.")
            return

        wi = await client.get_work_item(item_id)
        await client.close()

        if not wi:
            await update.message.reply_text(f"Work item #{item_id} not found or not accessible.")
            return

        sprint_name = wi.iteration_path.split("\\")[-1] if wi.iteration_path else ""

        lines = [
            f"<code>#{wi.id}</code> <b>{_h(wi.title)}</b>",
            "",
            f"<b>Type:</b> {_h(wi.work_item_type)}",
            f"<b>State:</b> {_h(wi.state)}",
            f"<b>Assigned:</b> {_h(wi.assigned_to or '(unassigned)')}",
        ]
        if sprint_name:
            lines.append(f"<b>Sprint:</b> {_h(sprint_name)}")
        if wi.due_date:
            lines.append(f"<b>Due:</b> {_h(wi.due_date)}")
        if wi.area_path:
            lines.append(f"<b>Area:</b> {_h(wi.area_path)}")
        if wi.tags:
            lines.append(f"<b>Tags:</b> {_h(', '.join(wi.tags))}")
        if wi.parent_id:
            lines.append(f"<b>Parent:</b> #{wi.parent_id}")
        if wi.url:
            lines.append(f"<b>URL:</b> {wi.url}")

        if wi.description:
            clean = _strip_html_entities(wi.description)
            if clean:
                if len(clean) > 800:
                    clean = clean[:800] + "..."
                lines.append(f"\n<b>Description:</b>\n<i>{_h(clean)}</i>")

        text = "\n".join(lines)
        if len(text) > MAX_MSG_LEN - 20:
            text = text[:MAX_MSG_LEN - 20] + "\n... (truncated)"
        await update.message.reply_text(text, parse_mode="HTML")

    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


_TYPE_KEYWORDS = {
    "bug": "Bug",
    "task": "Task",
    "feature": "Feature",
    "epic": "Epic",
    "story": "Product Backlog Item",
    "pbi": "Product Backlog Item",
}


async def _cmd_create(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /create [type] <title> -- create a new Azure DevOps work item."""
    if not context.args:
        await update.message.reply_text(
            "Usage:\n"
            "<code>/create Fix the login bug</code>\n"
            "<code>/create bug Fix the login bug</code>\n"
            "<code>/create task Review sprint PR</code>\n\n"
            "Type keywords: bug, task, feature, epic, story, pbi",
            parse_mode="HTML"
        )
        return

    first = context.args[0].lower()
    if first in _TYPE_KEYWORDS and len(context.args) > 1:
        work_item_type = _TYPE_KEYWORDS[first]
        title = " ".join(context.args[1:])
    else:
        work_item_type = "Task"
        title = " ".join(context.args)

    try:
        from backend.azure.client import AzureDevOpsClient

        client = AzureDevOpsClient()
        if not client.is_configured():
            await update.message.reply_text(
                "Azure DevOps is not configured. Check your <code>.env</code>.",
                parse_mode="HTML"
            )
            return

        # Fetch sprints in parallel with showing progress
        await update.message.reply_text(
            f"Fetching sprints for <b>{_h(work_item_type)}</b>: <i>{_h(title)}</i>...",
            parse_mode="HTML"
        )
        iterations = await client.get_iterations()
        await client.close()

        # Build sprint picker keyboard
        # Each button encodes: sprint:<work_item_type>|<iteration_path>|<title>
        buttons = []
        for it in iterations:
            name = it.get("name", "")
            path = it.get("path", "") or name
            # Truncate label for button display
            label = name[:30]
            payload = f"sprint:{work_item_type}|{path}|{title}"
            if len(payload) <= 64:  # Telegram callback_data limit
                buttons.append([InlineKeyboardButton(label, callback_data=payload)])

        buttons.append([InlineKeyboardButton("No sprint (backlog)", callback_data=f"sprint:{work_item_type}||{title}")])

        if not buttons:
            # No sprints found — create without sprint
            await _do_create(update, work_item_type, title, iteration_path=None)
            return

        keyboard = InlineKeyboardMarkup(buttons)
        await update.message.reply_text(
            f"Which sprint for <b>{_h(work_item_type)}</b>: <i>{_h(title)}</i>?",
            reply_markup=keyboard,
            parse_mode="HTML"
        )

    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _on_sprint_selected(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback when user picks a sprint from the inline keyboard."""
    query = update.callback_query
    await query.answer()

    try:
        # payload: sprint:<type>|<iteration_path>|<title>
        payload = query.data[len("sprint:"):]
        parts = payload.split("|", 2)
        if len(parts) != 3:
            await query.edit_message_text("Invalid selection.")
            return

        work_item_type, iteration_path, title = parts
        iteration_path = iteration_path or None

        await query.edit_message_text(
            f"Creating <b>{_h(work_item_type)}</b>: <i>{_h(title)}</i>"
            + (f"\nSprint: {_h(iteration_path.split(chr(92))[-1])}" if iteration_path else "\nBacklog (no sprint)")
            + "...",
            parse_mode="HTML"
        )
        await _do_create(query, work_item_type, title, iteration_path)

    except Exception as e:
        await query.edit_message_text(f"Error: {e}")


async def _do_create(ctx, work_item_type: str, title: str, iteration_path):
    """Create the work item and send confirmation."""
    from backend.azure.client import AzureDevOpsClient

    client = AzureDevOpsClient()
    try:
        wi = await client.create_work_item(
            title=title,
            work_item_type=work_item_type,
            iteration_path=iteration_path,
        )
    finally:
        await client.close()

    if not wi:
        text = f"Failed to create {work_item_type}. Check Azure DevOps permissions."
    else:
        sprint_label = ""
        if wi.iteration_path:
            sprint_label = f"\nSprint: {_h(wi.iteration_path.split(chr(92))[-1])}"
        url_label = f'\n<a href="{wi.url}">Open in Azure DevOps</a>' if wi.url else ""
        text = (
            f"✓ Created <b>{_h(wi.work_item_type)}</b> <code>#{wi.id}</code>\n"
            f"<b>{_h(wi.title)}</b>\n"
            f"State: {_h(wi.state)}"
            f"{sprint_label}{url_label}"
        )

    # ctx is either a Message (from direct create) or a CallbackQuery
    if hasattr(ctx, "edit_message_text"):
        await ctx.edit_message_text(text, parse_mode="HTML")
    else:
        await ctx.message.reply_text(text, parse_mode="HTML")


async def _on_view_issue(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: View Full Details button on assignment notifications."""
    query = update.callback_query
    await query.answer()
    try:
        item_id = int(query.data.split(":", 1)[1])
    except (ValueError, IndexError):
        await query.answer("Invalid work item ID", show_alert=True)
        return

    await query.message.reply_text(f"Fetching work item #{item_id}...")

    try:
        from backend.azure.client import AzureDevOpsClient
        client = AzureDevOpsClient()
        if not client.is_configured():
            await query.message.reply_text("Azure DevOps is not configured.")
            return

        wi = await client.get_work_item(item_id)
        await client.close()

        if not wi:
            await query.message.reply_text(f"Work item #{item_id} not found or not accessible.")
            return

        sprint_name = wi.iteration_path.split("\\")[-1] if wi.iteration_path else ""
        lines = [
            f"<code>#{wi.id}</code>  <b>{_h(wi.title)}</b>",
            "",
            f"<b>Type:</b> {_h(wi.work_item_type)}",
            f"<b>State:</b> {_h(wi.state)}",
            f"<b>Assigned:</b> {_h(wi.assigned_to or '(unassigned)')}",
        ]
        if sprint_name:
            lines.append(f"<b>Sprint:</b> {_h(sprint_name)}")
        if wi.due_date:
            lines.append(f"<b>Due:</b> {_h(wi.due_date)}")
        if wi.area_path:
            lines.append(f"<b>Area:</b> {_h(wi.area_path)}")
        if wi.tags:
            lines.append(f"<b>Tags:</b> {_h(', '.join(wi.tags))}")
        if wi.parent_id:
            lines.append(f"<b>Parent:</b> #{wi.parent_id}")
        if wi.url:
            lines.append(f'<a href="{wi.url}">Open in Azure DevOps</a>')
        if wi.description:
            clean = _strip_html_entities(wi.description)
            if clean:
                if len(clean) > 800:
                    clean = clean[:800] + "..."
                lines.append(f"\n<b>Description:</b>\n<i>{_h(clean)}</i>")

        text = "\n".join(lines)
        if len(text) > MAX_MSG_LEN - 20:
            text = text[:MAX_MSG_LEN - 20] + "\n... (truncated)"
        await query.message.reply_text(text, parse_mode="HTML")

    except Exception as e:
        await query.message.reply_text(f"Error: {e}")


def _get_azure_state_file() -> str:
    """Resolve path to Data/azure/sync_state.json."""
    try:
        from backend.config import get_path
        data_dir = get_path("DATA_DIR")
        return os.path.join(data_dir, "azure", "sync_state.json")
    except Exception:
        pass
    from backend.config import get_project_root
    project_root = get_project_root()
    if project_root:
        return os.path.join(project_root, "Data", "azure", "sync_state.json")
    return ""


def _h(text: str) -> str:
    """Escape HTML special characters for Telegram HTML parse mode."""
    return text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def _strip_html_entities(text: str) -> str:
    """Strip HTML tags and decode common entities for display."""
    text = re.sub(r"<br\s*/?>", "\n", text, flags=re.IGNORECASE)
    text = re.sub(r"<[^>]+>", "", text)
    text = text.replace("&nbsp;", " ").replace("&amp;", "&").replace("&lt;", "<").replace("&gt;", ">")
    return " ".join(text.split()).strip()


def _strip_html(text: str) -> str:
    """Remove HTML tags for terminal display."""
    text = re.sub(r"<br\s*/?>", "\n", text, flags=re.IGNORECASE)
    text = re.sub(r"<[^>]+>", "", text)
    return text.strip()


def _get_db_path() -> str:
    """Get the SQLite database path."""
    from backend.config import get_path
    try:
        db_dir = get_path("DATABASE_DIR")
        from backend.config import get as _cfg
        db_file = _cfg("DATABASE_FILE_NAME", "devtrack.db")
        path = os.path.join(db_dir, db_file)
        if os.path.exists(path):
            return path
    except Exception:
        pass
    return ""


def _service_display_name(service: str) -> str:
    """Human-readable service name."""
    names = {
        "ipc": "Python IPC",
        "python_bridge": "Python Bridge",
        "ollama": "Ollama",
        "azure_devops": "Azure DevOps",
        "webhook_server": "Webhook Server",
        "mongodb": "MongoDB",
        "telegram_bot": "Telegram Bot",
    }
    return names.get(service, service)


# ---------------------------------------------------------------------------
# GitLab helpers
# ---------------------------------------------------------------------------

def _get_gitlab_state_file() -> str:
    """Resolve path to Data/gitlab/sync_state.json."""
    try:
        from backend.config import get_path
        data_dir = get_path("DATA_DIR")
        return os.path.join(data_dir, "gitlab", "sync_state.json")
    except Exception:
        pass
    from backend.config import get_project_root
    project_root = get_project_root()
    if project_root:
        return os.path.join(project_root, "Data", "gitlab", "sync_state.json")
    return ""


# ---------------------------------------------------------------------------
# GitLab command handlers
# ---------------------------------------------------------------------------

async def _cmd_gitlab(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /gitlab -- list GitLab issues assigned to me (from cache)."""
    try:
        state_file = _get_gitlab_state_file()
        if not state_file or not os.path.exists(state_file):
            await update.message.reply_text(
                "No GitLab sync data found. Run <code>devtrack gitlab-sync</code> first.",
                parse_mode="HTML"
            )
            return

        with open(state_file) as f:
            data = json.load(f)

        issues = data.get("issues", {})
        last_sync = data.get("last_sync", "unknown")
        if last_sync and "T" in last_sync:
            last_sync = last_sync.split("T")[0]

        if not issues:
            await update.message.reply_text(
                f"No issues found.\n<i>Last synced: {last_sync}</i>\n\n"
                "Run <code>devtrack gitlab-sync</code> to refresh.",
                parse_mode="HTML"
            )
            return

        # Group by state
        by_state: dict = {}
        for item in issues.values():
            state = item.get("state", "opened")
            by_state.setdefault(state, []).append(item)

        blocks = []
        for state in sorted(by_state.keys()):
            state_items = by_state[state]
            lines = [f"<b>{_h(state)}</b> ({len(state_items)})"]
            for item in state_items:
                iid = item.get("iid", "?")
                project_id = item.get("project_id", "?")
                title = _h(item.get("title", "(no title)"))
                labels = _h(", ".join(item.get("labels", [])))
                milestone = _h(item.get("milestone_title", "") or "")
                due = _h(item.get("due_date", "") or "")

                lines.append(f"\n<code>!{iid}</code> <b>{title}</b>")
                lines.append(f"  Project: {project_id}  |  {_h(state)}")
                if milestone:
                    lines.append(f"  Milestone: {milestone}")
                if labels:
                    lines.append(f"  Labels: {labels}")
                if due:
                    lines.append(f"  Due: {due}")
            blocks.append("\n".join(lines))

        header = f"<b>GitLab — My Issues</b> ({len(issues)} total)\n\n"
        footer = f"\n\n<i>Synced: {last_sync}</i>  |  /gitlabissue &lt;project_id&gt; &lt;iid&gt;"

        current = header
        for block in blocks:
            if len(current) + len(block) + len(footer) + 5 > MAX_MSG_LEN:
                await update.message.reply_text(current, parse_mode="HTML")
                current = block
            else:
                current = current + block + "\n\n"

        await update.message.reply_text(current + footer, parse_mode="HTML")

    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_gitlab_issue(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /gitlabissue <project_id> <iid> -- fetch single issue live."""
    if not context.args or len(context.args) < 2:
        await update.message.reply_text("Usage: /gitlabissue <project_id> <issue_iid>")
        return

    try:
        project_id = int(context.args[0])
        issue_iid = int(context.args[1])
    except ValueError:
        await update.message.reply_text("Both project_id and iid must be integers.")
        return

    await update.message.reply_text(f"Fetching GitLab issue !{issue_iid} in project {project_id}...")

    try:
        from backend.gitlab.client import GitLabClient
        client = GitLabClient()
        if not client.is_configured():
            await update.message.reply_text(
                "GitLab is not configured. Check your <code>.env</code>.", parse_mode="HTML"
            )
            return

        issue = await client.get_issue(project_id, issue_iid)
        await client.close()

        if not issue:
            await update.message.reply_text(f"Issue !{issue_iid} not found in project {project_id}.")
            return

        lines = [
            f"<code>!{issue.iid}</code> <b>{_h(issue.title)}</b>",
            "",
            f"<b>State:</b> {_h(issue.state)}",
            f"<b>Project:</b> {issue.project_id}",
            f"<b>Assigned:</b> {_h(issue.assignee or '(unassigned)')}",
        ]
        if issue.milestone_title:
            lines.append(f"<b>Milestone:</b> {_h(issue.milestone_title)}")
        if issue.labels:
            lines.append(f"<b>Labels:</b> {_h(', '.join(issue.labels))}")
        if issue.due_date:
            lines.append(f"<b>Due:</b> {_h(issue.due_date)}")
        if issue.url:
            lines.append(f"<b>URL:</b> {issue.url}")
        if issue.description:
            clean = _strip_html_entities(issue.description)
            if clean:
                if len(clean) > 800:
                    clean = clean[:800] + "..."
                lines.append(f"\n<b>Description:</b>\n<i>{_h(clean)}</i>")

        text = "\n".join(lines)
        if len(text) > MAX_MSG_LEN - 20:
            text = text[:MAX_MSG_LEN - 20] + "\n... (truncated)"
        await update.message.reply_text(text, parse_mode="HTML")

    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_gitlab_create(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /gitlabcreate <title> -- create a GitLab issue with optional milestone picker."""
    if not context.args:
        await update.message.reply_text(
            "Usage: <code>/gitlabcreate Fix the login bug</code>",
            parse_mode="HTML"
        )
        return

    title = " ".join(context.args)

    try:
        from backend.gitlab.client import GitLabClient
        client = GitLabClient()
        if not client.is_configured():
            await update.message.reply_text(
                "GitLab is not configured. Check your <code>.env</code>.",
                parse_mode="HTML"
            )
            return

        await update.message.reply_text(
            f"Fetching milestones for: <i>{_h(title)}</i>...",
            parse_mode="HTML"
        )

        milestones = await client.get_milestones()  # uses GITLAB_PROJECT_ID
        await client.close()

        chat_id = update.effective_chat.id
        _PENDING_GITLAB_CREATES[chat_id] = title

        buttons = []
        for m in milestones:
            mid = m.get("id", "")
            mname = m.get("title", "")
            label = mname[:30]
            payload = f"gitlab_milestone:{mid}|{chat_id}"
            buttons.append([InlineKeyboardButton(label, callback_data=payload)])

        buttons.append([InlineKeyboardButton(
            "No milestone", callback_data=f"gitlab_milestone:0|{chat_id}"
        )])

        if not milestones:
            # No milestones — create directly
            await _do_gitlab_create(update, title, milestone_id=None)
            return

        keyboard = InlineKeyboardMarkup(buttons)
        await update.message.reply_text(
            f"Choose milestone for: <b>{_h(title)}</b>",
            reply_markup=keyboard,
            parse_mode="HTML"
        )

    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _on_gitlab_milestone_selected(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback when user picks a milestone from the GitLab create flow."""
    query = update.callback_query
    await query.answer()

    try:
        payload = query.data[len("gitlab_milestone:"):]
        parts = payload.split("|", 1)
        if len(parts) != 2:
            await query.edit_message_text("Invalid selection.")
            return

        milestone_id_str, chat_id_str = parts
        milestone_id = int(milestone_id_str) if milestone_id_str != "0" else None
        chat_id = int(chat_id_str)

        title = _PENDING_GITLAB_CREATES.pop(chat_id, None)
        if not title:
            await query.edit_message_text("Session expired. Please run /gitlabcreate again.")
            return

        milestone_label = f" (milestone #{milestone_id})" if milestone_id else " (no milestone)"
        await query.edit_message_text(
            f"Creating issue: <i>{_h(title)}</i>{_h(milestone_label)}...",
            parse_mode="HTML"
        )
        await _do_gitlab_create(query, title, milestone_id=milestone_id)

    except Exception as e:
        await query.edit_message_text(f"Error: {e}")


async def _do_gitlab_create(ctx, title: str, milestone_id):
    """Create the GitLab issue and send confirmation."""
    from backend.gitlab.client import GitLabClient
    client = GitLabClient()
    try:
        issue = await client.create_issue(
            title=title,
            milestone_id=milestone_id,
        )
    finally:
        await client.close()

    if not issue:
        text = "Failed to create issue. Check GitLab permissions and GITLAB_PROJECT_ID."
    else:
        url_label = f'\n<a href="{issue.url}">Open in GitLab</a>' if issue.url else ""
        milestone_label = f"\nMilestone: {_h(issue.milestone_title)}" if issue.milestone_title else ""
        text = (
            f"Created issue <code>!{issue.iid}</code>\n"
            f"<b>{_h(issue.title)}</b>\n"
            f"State: {_h(issue.state)}"
            f"{milestone_label}{url_label}"
        )

    if hasattr(ctx, "edit_message_text"):
        await ctx.edit_message_text(text, parse_mode="HTML")
    else:
        await ctx.message.reply_text(text, parse_mode="HTML")


async def _on_view_gitlab_issue(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: View Details on GitLab assignment notifications."""
    query = update.callback_query
    await query.answer()
    try:
        global_id = int(query.data.split(":", 1)[1])
    except (ValueError, IndexError):
        await query.answer("Invalid issue ID", show_alert=True)
        return

    await query.message.reply_text(f"Fetching GitLab issue #{global_id}...")

    try:
        from backend.gitlab.client import GitLabClient
        client = GitLabClient()
        if not client.is_configured():
            await query.message.reply_text("GitLab is not configured.")
            return

        issue = await client.get_issue_by_global_id(global_id)
        await client.close()

        if not issue:
            await query.message.reply_text(f"Issue #{global_id} not found.")
            return

        lines = [
            f"<code>!{issue.iid}</code> <b>{_h(issue.title)}</b>",
            f"<b>State:</b> {_h(issue.state)}",
            f"<b>Assigned:</b> {_h(issue.assignee or '(unassigned)')}",
        ]
        if issue.milestone_title:
            lines.append(f"<b>Milestone:</b> {_h(issue.milestone_title)}")
        if issue.labels:
            lines.append(f"<b>Labels:</b> {_h(', '.join(issue.labels))}")
        if issue.url:
            lines.append(f'<a href="{issue.url}">Open in GitLab</a>')
        if issue.description:
            clean = _strip_html_entities(issue.description)
            if clean and len(clean) > 800:
                clean = clean[:800] + "..."
            if clean:
                lines.append(f"\n<b>Description:</b>\n<i>{_h(clean)}</i>")

        text = "\n".join(lines)
        if len(text) > MAX_MSG_LEN - 20:
            text = text[:MAX_MSG_LEN - 20] + "\n... (truncated)"
        await query.message.reply_text(text, parse_mode="HTML")

    except Exception as e:
        await query.message.reply_text(f"Error: {e}")


# ---------------------------------------------------------------------------
# GitHub command handlers
# ---------------------------------------------------------------------------

async def _cmd_github(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /github -- list GitHub issues assigned to me."""
    await update.message.reply_text("Fetching your GitHub issues...")
    try:
        from backend.github.client import GitHubClient
        client = GitHubClient()
        if not client.is_configured():
            await update.message.reply_text(
                "GitHub is not configured. Set <code>GITHUB_TOKEN</code>, "
                "<code>GITHUB_OWNER</code>, and <code>GITHUB_REPO</code> in your <code>.env</code>.",
                parse_mode="HTML"
            )
            return

        issues = await client.get_my_issues(state="open")
        await client.close()

        if not issues:
            await update.message.reply_text("No open GitHub issues assigned to you.")
            return

        lines = [f"<b>GitHub — My Issues</b> ({len(issues)} open)\n"]
        for issue in issues:
            labels = _h(", ".join(issue.labels)) if issue.labels else ""
            milestone = _h(issue.milestone or "")
            lines.append(f"\n<code>#{issue.number}</code> <b>{_h(issue.title)}</b>")
            lines.append(f"  State: {_h(issue.state)}")
            if milestone:
                lines.append(f"  Milestone: {milestone}")
            if labels:
                lines.append(f"  Labels: {labels}")
            lines.append(f"  <a href='{issue.html_url}'>View on GitHub</a>")

        text = "\n".join(lines)
        # Split if too long
        if len(text) > MAX_MSG_LEN - 50:
            text = text[:MAX_MSG_LEN - 50] + "\n... (truncated)\n\n/githubissue &lt;number&gt; for details"

        await update.message.reply_text(text, parse_mode="HTML", disable_web_page_preview=True)

    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_github_issue(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /githubissue <number> -- fetch single issue live."""
    if not context.args:
        await update.message.reply_text("Usage: /githubissue <issue-number>")
        return

    try:
        number = int(context.args[0])
    except ValueError:
        await update.message.reply_text("Issue number must be an integer.")
        return

    await update.message.reply_text(f"Fetching GitHub issue #{number}...")

    try:
        from backend.github.client import GitHubClient
        client = GitHubClient()
        if not client.is_configured():
            await update.message.reply_text(
                "GitHub is not configured. Check your <code>.env</code>.", parse_mode="HTML"
            )
            return

        issue = await client.get_issue(number)
        await client.close()

        if not issue:
            await update.message.reply_text(f"Issue #{number} not found.")
            return

        lines = [
            f"<code>#{issue.number}</code> <b>{_h(issue.title)}</b>",
            "",
            f"<b>State:</b> {_h(issue.state)}",
        ]
        if issue.assignees:
            lines.append(f"<b>Assigned:</b> {_h(', '.join(issue.assignees))}")
        if issue.milestone:
            lines.append(f"<b>Milestone:</b> {_h(issue.milestone)}")
        if issue.labels:
            lines.append(f"<b>Labels:</b> {_h(', '.join(issue.labels))}")
        if issue.created_at:
            lines.append(f"<b>Created:</b> {_h(issue.created_at[:10])}")
        if issue.html_url:
            lines.append(f"<b>URL:</b> {issue.html_url}")
        if issue.body:
            clean = _strip_html_entities(issue.body)
            if clean:
                if len(clean) > 800:
                    clean = clean[:800] + "..."
                lines.append(f"\n<b>Description:</b>\n<i>{_h(clean)}</i>")

        text = "\n".join(lines)
        if len(text) > MAX_MSG_LEN - 20:
            text = text[:MAX_MSG_LEN - 20] + "\n... (truncated)"
        await update.message.reply_text(text, parse_mode="HTML", disable_web_page_preview=True)

    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


async def _cmd_github_create(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /githubcreate [bug|feature] <title> -- create a GitHub issue."""
    if not context.args:
        await update.message.reply_text(
            "Usage:\n"
            "<code>/githubcreate Fix the login bug</code>\n"
            "<code>/githubcreate bug Fix the login bug</code>\n"
            "<code>/githubcreate feature Add dark mode</code>",
            parse_mode="HTML"
        )
        return

    LABEL_ALIASES = {"bug": "bug", "feature": "enhancement", "task": "task", "enhancement": "enhancement"}
    label = None
    args = list(context.args)

    if args[0].lower() in LABEL_ALIASES:
        label = LABEL_ALIASES[args.pop(0).lower()]

    if not args:
        await update.message.reply_text("Please provide a title.")
        return

    title = " ".join(args)

    await update.message.reply_text(
        f"Creating GitHub issue: <i>{_h(title)}</i>...",
        parse_mode="HTML"
    )

    try:
        from backend.github.client import GitHubClient
        client = GitHubClient()
        if not client.is_configured():
            await update.message.reply_text(
                "GitHub is not configured. Check your <code>.env</code>.", parse_mode="HTML"
            )
            return

        labels = [label] if label else []
        issue = await client.create_issue(title=title, labels=labels)
        await client.close()

        if not issue:
            await update.message.reply_text("Failed to create issue. Check the logs.")
            return

        label_str = f" [{label}]" if label else ""
        await update.message.reply_text(
            f"Created GitHub issue{label_str}\n"
            f"<code>#{issue.number}</code> <b>{_h(issue.title)}</b>\n"
            f"{issue.html_url}",
            parse_mode="HTML",
            disable_web_page_preview=True
        )

    except Exception as e:
        await update.message.reply_text(f"Error: {e}")


# ---------------------------------------------------------------------------
# PM Agent command handlers
# ---------------------------------------------------------------------------

async def _cmd_plan(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """Handle /plan <problem> -- start the PM agent flow."""
    if not context.args:
        await update.message.reply_text(
            "Usage: <code>/plan &lt;problem statement&gt;</code>\n\n"
            "Example: <code>/plan Build a user authentication system with login, "
            "signup, password reset, and session management</code>",
            parse_mode="HTML"
        )
        return

    problem = " ".join(context.args)
    chat_id = update.effective_chat.id
    _PENDING_PLANS[chat_id] = {"problem": problem, "plan": None}

    buttons = [
        [InlineKeyboardButton("Azure DevOps", callback_data=f"plan_platform:azure|{chat_id}")],
        [InlineKeyboardButton("GitLab", callback_data=f"plan_platform:gitlab|{chat_id}")],
        [InlineKeyboardButton("GitHub", callback_data=f"plan_platform:github|{chat_id}")],
        [InlineKeyboardButton("Cancel", callback_data=f"plan_cancel:{chat_id}")],
    ]

    keyboard = InlineKeyboardMarkup(buttons)
    await update.message.reply_text(
        f"<b>PM Agent</b>\n\nProblem: <i>{_h(problem[:200])}</i>\n\nChoose target platform:",
        reply_markup=keyboard,
        parse_mode="HTML"
    )


async def _on_plan_platform_selected(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: platform chosen — run LLM decomposition and show preview."""
    query = update.callback_query
    await query.answer()

    try:
        payload = query.data[len("plan_platform:"):]
        platform, chat_id_str = payload.split("|", 1)
        chat_id = int(chat_id_str)

        pending = _PENDING_PLANS.get(chat_id)
        if not pending:
            await query.edit_message_text("Session expired. Run /plan again.")
            return

        problem = pending["problem"]

        await query.edit_message_text(
            f"Decomposing for <b>{_h(platform)}</b>...\n\n"
            f"<i>{_h(problem[:150])}</i>",
            parse_mode="HTML"
        )

        from backend.pm_agent import PMAgent
        agent = PMAgent(platform=platform)
        plan = agent.decompose(problem)

        _PENDING_PLANS[chat_id]["plan"] = plan
        _PENDING_PLANS[chat_id]["platform"] = platform

        preview = agent.format_preview(plan)
        if len(preview) > 3000:
            preview = preview[:3000] + "\n... (truncated)"

        confirm_buttons = [
            [InlineKeyboardButton(
                f"Create all {plan.total_count} items",
                callback_data=f"plan_confirm:{chat_id}"
            )],
            [InlineKeyboardButton("Cancel", callback_data=f"plan_cancel:{chat_id}")],
        ]
        keyboard = InlineKeyboardMarkup(confirm_buttons)

        await query.edit_message_text(
            f"<b>Plan Preview</b> ({_h(platform)})\n\n<pre>{_h(preview)}</pre>",
            reply_markup=keyboard,
            parse_mode="HTML"
        )

    except ValueError as e:
        await query.edit_message_text(f"Failed to decompose: {_h(str(e))}", parse_mode="HTML")
    except Exception as e:
        await query.edit_message_text(f"Error: {e}")


async def _on_plan_confirmed(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: user confirmed — create all items, report results."""
    query = update.callback_query
    await query.answer()

    try:
        chat_id = int(query.data.split(":", 1)[1])
        pending = _PENDING_PLANS.pop(chat_id, None)
        if not pending or not pending.get("plan"):
            await query.edit_message_text("Session expired. Run /plan again.")
            return

        plan = pending["plan"]
        platform = pending["platform"]

        await query.edit_message_text(
            f"Creating {plan.total_count} items in <b>{_h(platform)}</b>...",
            parse_mode="HTML"
        )

        progress_msg = await query.message.reply_text("Starting...", parse_mode="HTML")
        progress_lines = []

        async def on_progress(node, status):
            progress_lines.append(f"{_h(node.title[:40])}: {_h(status)}")
            if len(progress_lines) % 3 == 0:
                text = "\n".join(progress_lines[-10:])
                try:
                    await progress_msg.edit_text(f"<pre>{text}</pre>", parse_mode="HTML")
                except Exception:
                    pass

        from backend.pm_agent import PMAgent
        agent = PMAgent(platform=platform)
        created, failed = await agent.create_all(plan, on_progress=on_progress)

        lines = [
            f"<b>Done</b> — {len(created)} created, {len(failed)} failed",
            "",
        ]
        for node in created[:20]:
            url_part = f' <a href="{node.platform_url}">#{node.platform_id}</a>' if node.platform_url else ""
            lines.append(f"{_h(node.title[:50])}{url_part}")
        if len(created) > 20:
            lines.append(f"... and {len(created) - 20} more")

        if failed:
            lines.append(f"\n<b>Failed ({len(failed)}):</b>")
            for node, err in failed[:5]:
                lines.append(f"  {_h(node.title[:40])}: {_h(err[:60])}")

        text = "\n".join(lines)
        if len(text) > MAX_MSG_LEN - 20:
            text = text[:MAX_MSG_LEN - 20] + "\n... (truncated)"

        await progress_msg.edit_text(text, parse_mode="HTML", disable_web_page_preview=True)

    except Exception as e:
        await query.message.reply_text(f"Error during creation: {e}")


async def _on_plan_cancelled(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: user cancelled plan."""
    query = update.callback_query
    await query.answer()
    try:
        chat_id = int(query.data.split(":", 1)[1])
        _PENDING_PLANS.pop(chat_id, None)
    except Exception:
        pass
    await query.edit_message_text("Plan cancelled.")


# ---------------------------------------------------------------------------
# AI Project Planning (/newproject)
# ---------------------------------------------------------------------------

def _np_review_url(spec_id: str) -> str:
    """Build the web review URL for a spec."""
    from backend.config import get_spec_review_base_url
    base = get_spec_review_base_url().rstrip("/")
    if not base:
        return ""
    return f"{base}/spec/{spec_id}/review"


async def _cmd_newproject(update: Update, context: ContextTypes.DEFAULT_TYPE, bot):
    """Handle /newproject — start the AI project planning flow.

    The PM provides requirements in plain text; the bot walks through:
    platform selection → team member picker → AI generation → approval.
    """
    chat_id = update.effective_chat.id
    _PENDING_PROJECTS[chat_id] = {"stage": "awaiting_platform"}

    buttons = [
        [InlineKeyboardButton("Azure DevOps", callback_data=f"np_platform:azure|{chat_id}")],
        [InlineKeyboardButton("GitHub", callback_data=f"np_platform:github|{chat_id}")],
        [InlineKeyboardButton("GitLab", callback_data=f"np_platform:gitlab|{chat_id}")],
        [InlineKeyboardButton("Cancel", callback_data=f"np_cancel:{chat_id}")],
    ]
    await update.message.reply_text(
        "*New AI Project Plan*\n\n"
        "Which PM platform should sprints/stories be created in?",
        parse_mode="Markdown",
        reply_markup=InlineKeyboardMarkup(buttons),
    )


async def _on_newproject_platform(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: platform selected → ask for requirements text."""
    query = update.callback_query
    await query.answer()
    try:
        payload, chat_id_str = query.data.split(":", 1)[1].split("|", 1)
        platform = payload
        chat_id = int(chat_id_str)
    except Exception:
        await query.edit_message_text("Invalid state. Run /newproject again.")
        return

    pending = _PENDING_PROJECTS.get(chat_id)
    if not pending:
        await query.edit_message_text("Session expired. Run /newproject again.")
        return

    pending["platform"] = platform
    pending["stage"] = "awaiting_requirements"

    await query.edit_message_text(
        f"Platform: *{platform}*\n\n"
        "Please describe the project in plain text. Include:\n"
        "• What needs to be built\n"
        "• Deadline (e.g. 'by July 31')\n"
        "• Your email address for the spec review\n"
        "• Any key constraints or requirements\n\n"
        "Just send a message with all the details:",
        parse_mode="Markdown",
    )

    # Register a one-shot message handler for this chat
    context.user_data["np_awaiting_text"] = chat_id
    context.application.add_handler(
        _make_np_text_handler(chat_id),
        group=99,
    )


def _make_np_text_handler(target_chat_id: int):
    """Create a MessageHandler that captures the next text message for this chat."""
    from telegram.ext import MessageHandler, filters

    async def handler(update: Update, ctx: ContextTypes.DEFAULT_TYPE):
        if update.effective_chat.id != target_chat_id:
            return
        if not update.message or not update.message.text:
            return
        # Remove this handler after first use
        ctx.application.remove_handler(handler, group=99)
        await _handle_np_requirements(update, ctx, target_chat_id)

    return MessageHandler(filters.TEXT & ~filters.COMMAND, handler)


async def _handle_np_requirements(update: Update, context, chat_id: int):
    """Process the requirements text and show the team member picker."""
    pending = _PENDING_PROJECTS.get(chat_id)
    if not pending:
        return

    text = update.message.text
    pending["requirements"] = text
    pending["stage"] = "awaiting_team_selection"
    pending["selected_members"] = []

    # Parse deadline and PM email from free text
    _extract_deadline_and_email(pending, text)

    platform = pending.get("platform", "azure")
    await update.message.reply_text(f"Fetching team from *{platform}*...", parse_mode="Markdown")

    try:
        from backend.project_spec.developer_roster import DeveloperRoster
        roster = DeveloperRoster()
        members = await roster.list_team_members(platform)
        pending["available_members"] = [m.to_dict() for m in members]
    except Exception as e:
        logger.warning(f"newproject: failed to fetch team: {e}")
        pending["available_members"] = []
        members = []

    if not members:
        await update.message.reply_text(
            "Could not fetch team members from the platform. "
            "Please enter developer names and emails manually (one per line, format: Name, email@org.com):\n\n"
            "Or send /skip to proceed with an empty team."
        )
        pending["stage"] = "awaiting_manual_team"
        return

    await _show_member_picker(update.message, chat_id, pending)


def _extract_deadline_and_email(pending: dict, text: str) -> None:
    """Parse deadline date and PM email from free-text requirements."""
    import re
    # Extract email
    email_match = re.search(r'[\w.+-]+@[\w-]+\.[a-zA-Z]{2,}', text)
    if email_match:
        pending["pm_email"] = email_match.group(0)

    # Extract deadline via dateparser
    try:
        import dateparser
        # Look for date-like phrases
        date_patterns = [
            r'by\s+(\w+ \d{1,2}(?:st|nd|rd|th)?(?:,?\s+\d{4})?)',
            r'deadline[:\s]+([A-Za-z]+ \d{1,2}(?:,?\s+\d{4})?)',
            r'(\d{4}-\d{2}-\d{2})',
        ]
        for pattern in date_patterns:
            m = re.search(pattern, text, re.IGNORECASE)
            if m:
                parsed = dateparser.parse(m.group(1))
                if parsed:
                    pending["deadline"] = parsed.date().isoformat()
                    break
    except Exception:
        pass


async def _show_member_picker(message, chat_id: int, pending: dict) -> None:
    """Display the inline keyboard multi-select for team members."""
    members = pending.get("available_members", [])
    selected = set(pending.get("selected_members", []))

    buttons = []
    for m in members:
        uid = m.get("platform_user_id", m.get("email", ""))
        name = m.get("name") or uid
        checkmark = "✅ " if uid in selected else ""
        buttons.append([InlineKeyboardButton(
            f"{checkmark}{name}",
            callback_data=f"np_member:{uid}|{chat_id}",
        )])

    buttons.append([
        InlineKeyboardButton("✓ Done — Generate Spec", callback_data=f"np_done:{chat_id}"),
        InlineKeyboardButton("✗ Cancel", callback_data=f"np_cancel:{chat_id}"),
    ])

    await message.reply_text(
        "Select team members for this project (tap to toggle ✅):",
        reply_markup=InlineKeyboardMarkup(buttons),
    )


async def _on_newproject_member_toggle(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: toggle a developer's selection."""
    query = update.callback_query
    await query.answer()
    try:
        payload, chat_id_str = query.data.split(":", 1)[1].split("|", 1)
        uid = payload
        chat_id = int(chat_id_str)
    except Exception:
        return

    pending = _PENDING_PROJECTS.get(chat_id)
    if not pending:
        await query.edit_message_text("Session expired. Run /newproject again.")
        return

    selected = pending.setdefault("selected_members", [])
    if uid in selected:
        selected.remove(uid)
    else:
        selected.append(uid)

    # Rebuild buttons to reflect new selection
    members = pending.get("available_members", [])
    selected_set = set(selected)
    buttons = []
    for m in members:
        m_uid = m.get("platform_user_id", m.get("email", ""))
        name = m.get("name") or m_uid
        checkmark = "✅ " if m_uid in selected_set else ""
        buttons.append([InlineKeyboardButton(
            f"{checkmark}{name}",
            callback_data=f"np_member:{m_uid}|{chat_id}",
        )])
    buttons.append([
        InlineKeyboardButton("✓ Done — Generate Spec", callback_data=f"np_done:{chat_id}"),
        InlineKeyboardButton("✗ Cancel", callback_data=f"np_cancel:{chat_id}"),
    ])
    await query.edit_message_reply_markup(reply_markup=InlineKeyboardMarkup(buttons))


async def _on_newproject_members_done(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: team selection complete → generate spec."""
    query = update.callback_query
    await query.answer()
    try:
        chat_id = int(query.data.split(":", 1)[1])
    except Exception:
        return

    pending = _PENDING_PROJECTS.get(chat_id)
    if not pending:
        await query.edit_message_text("Session expired. Run /newproject again.")
        return

    selected_ids = set(pending.get("selected_members", []))
    all_members = pending.get("available_members", [])
    selected_devs_raw = [m for m in all_members if m.get("platform_user_id", m.get("email")) in selected_ids]

    if not selected_devs_raw:
        await query.edit_message_text("No developers selected. Run /newproject again.")
        return

    await query.edit_message_text(
        f"Selected {len(selected_devs_raw)} developer(s).\n"
        "Analyzing workloads and generating spec... this may take a moment."
    )

    platform = pending.get("platform", "azure")
    requirements = pending.get("requirements", "")
    pm_email = pending.get("pm_email", "")
    deadline_str = pending.get("deadline")

    try:
        from datetime import date
        from backend.project_spec.developer_roster import Developer
        from backend.project_spec.workload_analyzer import WorkloadAnalyzer
        from backend.project_spec.spec_generator import SpecGenerator
        from backend.project_spec.spec_store import SpecStore
        from backend.project_spec.spec_emailer import SpecEmailer

        developers = [Developer.from_dict(d) for d in selected_devs_raw]
        deadline = date.fromisoformat(deadline_str) if deadline_str else None

        analyzer = WorkloadAnalyzer()
        workload = await analyzer.analyze(developers, platform, deadline=deadline)

        from backend.config import get_spec_review_base_url
        review_base = get_spec_review_base_url()
        generator = SpecGenerator()
        spec = await generator.generate(
            requirements=requirements,
            developers=developers,
            workload=workload,
            deadline=deadline,
            platform=platform,
            pm_email=pm_email,
            review_base_url=review_base,
        )

        if not spec:
            await query.message.reply_text(
                "AI generation failed. Please check LLM provider config and try again."
            )
            _PENDING_PROJECTS.pop(chat_id, None)
            return

        store = SpecStore()
        spec_id = await store.save(spec)
        pending["spec_id"] = spec_id
        pending["stage"] = "awaiting_approval"

        # Send email to PM
        emailer = SpecEmailer()
        sent = await emailer.send_draft(spec)
        email_note = f"\nSpec also emailed to *{pm_email}*." if sent and pm_email else ""

        # Show truncated preview in Telegram
        ca = spec.capacity_analysis or {}
        on_track = "✅ On track" if ca.get("on_track") else "⚠️ At risk"
        risks = ca.get("risks", [])
        risk_lines = "\n".join(
            f"  • [{r.get('severity', '?').upper()}] {r.get('message', '')}"
            for r in risks[:3]
        )
        project_name = (spec.project or {}).get("name", "Project")
        num_stories = sum(len(f.get("stories", [])) for f in spec.features)
        review_url = spec.review_url

        preview = (
            f"*{project_name}*\n"
            f"Sprints: {len(spec.sprints)} | Stories: {num_stories} | {on_track}\n"
        )
        if risk_lines:
            preview += f"\nRisks:\n{risk_lines}\n"
        if review_url:
            preview += f"\n[Review & edit full spec]({review_url})"
        preview += email_note

        buttons = [
            [InlineKeyboardButton("✅ Approve & Create", callback_data=f"np_approve:{chat_id}")],
            [InlineKeyboardButton("✏️ Request Changes", callback_data=f"np_revise:{chat_id}")],
            [InlineKeyboardButton("✗ Cancel", callback_data=f"np_cancel:{chat_id}")],
        ]
        await query.message.reply_text(
            preview,
            parse_mode="Markdown",
            reply_markup=InlineKeyboardMarkup(buttons),
            disable_web_page_preview=True,
        )

    except Exception as e:
        logger.exception(f"newproject generation error: {e}")
        await query.message.reply_text(f"Error during spec generation: {e}")
        _PENDING_PROJECTS.pop(chat_id, None)


async def _on_newproject_approve(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: PM approved — create all PM artifacts."""
    query = update.callback_query
    await query.answer()
    try:
        chat_id = int(query.data.split(":", 1)[1])
    except Exception:
        return

    pending = _PENDING_PROJECTS.get(chat_id)
    if not pending or not pending.get("spec_id"):
        await query.edit_message_text("Session expired. Run /newproject again.")
        return

    spec_id = pending["spec_id"]
    await query.edit_message_text("Approved! Creating sprints and stories in your PM tool...")

    try:
        from backend.project_spec.spec_store import SpecStore
        from backend.project_spec.project_creator import ProjectCreator

        store = SpecStore()
        spec = await store.load(spec_id)
        if not spec:
            await query.message.reply_text("Could not load spec. It may have expired.")
            return

        progress_msgs = []

        async def on_progress(msg: str) -> None:
            progress_msgs.append(msg)

        creator = ProjectCreator(on_progress=on_progress)
        results = await creator.create(spec)

        await store.update_status(spec_id, "approved")
        _PENDING_PROJECTS.pop(chat_id, None)

        n_sprints = len(results.get("sprints", []))
        n_stories = len(results.get("stories", []))
        errors = results.get("errors", [])

        summary = f"*Done!* Created {n_sprints} sprint(s) and {n_stories} story/stories."
        if errors:
            summary += f"\n\n⚠️ {len(errors)} error(s):\n" + "\n".join(f"• {e}" for e in errors[:5])

        await query.message.reply_text(summary, parse_mode="Markdown")

    except Exception as e:
        logger.exception(f"newproject creation error: {e}")
        await query.message.reply_text(f"Error creating PM items: {e}")
        _PENDING_PROJECTS.pop(chat_id, None)


async def _on_newproject_revise(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: PM requests changes — prompt for feedback text."""
    query = update.callback_query
    await query.answer()
    try:
        chat_id = int(query.data.split(":", 1)[1])
    except Exception:
        return

    pending = _PENDING_PROJECTS.get(chat_id)
    if not pending:
        await query.edit_message_text("Session expired. Run /newproject again.")
        return

    pending["stage"] = "awaiting_revision_feedback"
    await query.edit_message_text(
        "What changes would you like to the spec? Please describe them in a message:"
    )

    # Register one-shot text handler for revision feedback
    context.application.add_handler(
        _make_np_revision_handler(chat_id),
        group=99,
    )


def _make_np_revision_handler(target_chat_id: int):
    """Capture the next text message as revision feedback for this chat."""
    from telegram.ext import MessageHandler, filters

    async def handler(update: Update, ctx: ContextTypes.DEFAULT_TYPE):
        if update.effective_chat.id != target_chat_id:
            return
        if not update.message or not update.message.text:
            return
        ctx.application.remove_handler(handler, group=99)
        await _handle_np_revision(update, ctx, target_chat_id)

    return MessageHandler(filters.TEXT & ~filters.COMMAND, handler)


async def _handle_np_revision(update: Update, context, chat_id: int):
    """Apply PM feedback and regenerate the spec."""
    pending = _PENDING_PROJECTS.get(chat_id)
    if not pending or not pending.get("spec_id"):
        return

    feedback = update.message.text
    spec_id = pending["spec_id"]

    await update.message.reply_text("Applying changes and regenerating spec...")

    try:
        from backend.project_spec.spec_store import SpecStore
        from backend.project_spec.spec_generator import SpecGenerator
        from backend.project_spec.spec_emailer import SpecEmailer

        store = SpecStore()
        spec = await store.load(spec_id)
        if not spec:
            await update.message.reply_text("Could not load spec.")
            return

        generator = SpecGenerator()
        updated = await generator.revise(spec, feedback)
        if not updated:
            await update.message.reply_text("Revision failed. Please try again.")
            return

        new_id = await store.save(updated)
        pending["spec_id"] = new_id
        pending["stage"] = "awaiting_approval"

        emailer = SpecEmailer()
        await emailer.send_draft(updated)

        project_name = (updated.project or {}).get("name", "Project")
        review_url = updated.review_url
        msg = f"*Updated spec for {project_name}*"
        if review_url:
            msg += f"\n[Review full spec]({review_url})"

        buttons = [
            [InlineKeyboardButton("✅ Approve & Create", callback_data=f"np_approve:{chat_id}")],
            [InlineKeyboardButton("✏️ Request More Changes", callback_data=f"np_revise:{chat_id}")],
            [InlineKeyboardButton("✗ Cancel", callback_data=f"np_cancel:{chat_id}")],
        ]
        await update.message.reply_text(
            msg,
            parse_mode="Markdown",
            reply_markup=InlineKeyboardMarkup(buttons),
            disable_web_page_preview=True,
        )

    except Exception as e:
        logger.exception(f"newproject revision error: {e}")
        await update.message.reply_text(f"Error during revision: {e}")


async def _on_newproject_cancel(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Callback: PM cancelled the project planning flow."""
    query = update.callback_query
    await query.answer()
    try:
        chat_id = int(query.data.split(":", 1)[1])
        _PENDING_PROJECTS.pop(chat_id, None)
    except Exception:
        pass
    await query.edit_message_text("Project planning cancelled.")


# ---------------------------------------------------------------------------
# Work session Telegram commands
# ---------------------------------------------------------------------------

async def _cmd_workstart(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """/workstart [ticket-ref] — start a work session."""
    import asyncio
    ticket_ref = " ".join(context.args) if context.args else ""
    try:
        from backend.work_tracker.session_store import WorkSessionStore
        store = WorkSessionStore()
        active = await asyncio.to_thread(store.get_active_session)
        if active:
            msg = f"⚠️ A session is already active (ID {active['id']}"
            if active.get("ticket_ref"):
                msg += f", ticket: {active['ticket_ref']}"
            msg += ").\nUse /workstop first."
            await update.message.reply_text(msg)
            return

        from backend.config import database_path
        import sqlite3
        conn = sqlite3.connect(str(database_path()))
        cur = conn.execute(
            "INSERT INTO work_sessions (started_at, ticket_ref, commits) VALUES (datetime('now'), ?, '[]')",
            (ticket_ref,),
        )
        session_id = cur.lastrowid
        conn.commit()
        conn.close()

        msg = f"✅ Work session started (ID {session_id})"
        if ticket_ref:
            msg += f" for *{ticket_ref}*"
        msg += "\nUse /workstop when you're done."
        await update.message.reply_text(msg, parse_mode="Markdown")
    except Exception as e:
        await update.message.reply_text(f"Error starting session: {e}")


async def _cmd_workstop(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """/workstop — stop the active work session."""
    import asyncio
    from datetime import datetime, timezone
    try:
        from backend.work_tracker.session_store import WorkSessionStore
        store = WorkSessionStore()
        active = await asyncio.to_thread(store.get_active_session)
        if not active:
            await update.message.reply_text("No active session found.")
            return

        await asyncio.to_thread(store.end_session, active["id"])

        started_at = active.get("started_at", "")
        duration_str = ""
        try:
            start = datetime.fromisoformat(started_at)
            elapsed = int((datetime.now(timezone.utc).replace(tzinfo=None) - start).total_seconds() / 60)
            h, m = divmod(max(0, elapsed), 60)
            duration_str = f"{h}h {m}m" if h else f"{m}m"
        except Exception:
            pass

        msg = f"✅ Session {active['id']} stopped."
        if duration_str:
            msg += f" Duration: *{duration_str}*"
        if active.get("ticket_ref"):
            msg += f" (ticket: {active['ticket_ref']})"
        msg += "\nUse `/workadjust <minutes>` to correct the time."
        await update.message.reply_text(msg, parse_mode="Markdown")
    except Exception as e:
        await update.message.reply_text(f"Error stopping session: {e}")


async def _cmd_workadjust(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """/workadjust <minutes> — override time on the active or last session."""
    import asyncio
    if not context.args or not context.args[0].isdigit():
        await update.message.reply_text("Usage: /workadjust <minutes>  (e.g. /workadjust 90)")
        return
    minutes = int(context.args[0])
    try:
        from backend.work_tracker.session_store import WorkSessionStore
        from datetime import date
        store = WorkSessionStore()

        active = await asyncio.to_thread(store.get_active_session)
        if active:
            target_id = active["id"]
        else:
            today = date.today().isoformat()
            sessions = await asyncio.to_thread(store.get_sessions_for_date, today)
            if not sessions:
                await update.message.reply_text("No session found to adjust.")
                return
            target_id = sessions[-1]["id"]

        await asyncio.to_thread(store.adjust_time, target_id, minutes)
        h, m = divmod(minutes, 60)
        dur_str = f"{h}h {m}m" if h else f"{m}m"
        await update.message.reply_text(
            f"✅ Session {target_id} time adjusted to *{dur_str}*.\n"
            "_Auto-measured time is preserved in DB for audit._",
            parse_mode="Markdown",
        )
    except Exception as e:
        await update.message.reply_text(f"Error adjusting session: {e}")


async def _cmd_workstatus(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """/workstatus — show active session and today's completed sessions."""
    import asyncio
    from datetime import date, datetime, timezone
    try:
        from backend.work_tracker.session_store import WorkSessionStore
        store = WorkSessionStore()
        active = await asyncio.to_thread(store.get_active_session)
        today = date.today().isoformat()
        sessions = await asyncio.to_thread(store.get_sessions_for_date, today)

        lines = []
        if active:
            started_at = active.get("started_at", "")
            elapsed_str = ""
            try:
                start = datetime.fromisoformat(started_at)
                elapsed = int((datetime.now(timezone.utc).replace(tzinfo=None) - start).total_seconds() / 60)
                h, m = divmod(max(0, elapsed), 60)
                elapsed_str = f"{h}h {m}m" if h else f"{m}m"
            except Exception:
                pass
            line = f"🟢 *Active* (ID {active['id']}) — running {elapsed_str}"
            if active.get("ticket_ref"):
                line += f"\n   Ticket: {active['ticket_ref']}"
            lines.append(line)
        else:
            lines.append("⚪ No active session.")

        completed = [s for s in sessions if s.get("ended_at")]
        if completed:
            lines.append(f"\n*Today ({len(completed)} session(s)):*")
            total = 0
            for s in completed:
                dur = WorkSessionStore.effective_duration(s)
                total += dur
                h, m = divmod(dur, 60)
                dur_str = f"{h}h {m}m" if h else f"{m}m"
                adj = " ✏️" if s.get("adjusted_minutes") is not None else ""
                ticket = s.get("ticket_ref") or "(no ticket)"
                lines.append(f"  • {dur_str}{adj}  {ticket}")
            th, tm = divmod(total, 60)
            lines.append(f"  Total: *{th}h {tm}m*" if th else f"  Total: *{tm}m*")

        await update.message.reply_text("\n".join(lines), parse_mode="Markdown")
    except Exception as e:
        await update.message.reply_text(f"Error loading work status: {e}")


async def _cmd_workreport(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """/workreport [--email addr] — generate EOD report inline; optionally email it."""
    import asyncio
    from datetime import date

    email_to = None
    if context.args:
        args = context.args
        for i, arg in enumerate(args):
            if arg == "--email" and i + 1 < len(args):
                email_to = args[i + 1]
                break
            elif "@" in arg:
                email_to = arg
                break

    await update.message.reply_text("⏳ Generating EOD report…")
    try:
        from backend.work_tracker.eod_report_generator import EODReportGenerator
        generator = EODReportGenerator(include_ai=True)
        report = await generator.generate(date.today().isoformat())

        text = report.as_text()
        # Telegram max message length is 4096; truncate gracefully
        if len(text) > 3800:
            text = text[:3800] + "\n…(truncated)"

        await update.message.reply_text(f"```\n{text}\n```", parse_mode="Markdown")

        if email_to:
            from backend.work_tracker.eod_emailer import EODEmailer
            emailer = EODEmailer()
            sent = await emailer.send(report, email_to)
            if sent:
                await update.message.reply_text(f"✅ Report emailed to {email_to}")
            else:
                await update.message.reply_text(f"⚠️ Could not send email to {email_to} — check server logs.")
    except Exception as e:
        await update.message.reply_text(f"Error generating report: {e}")


async def _cmd_vacation(update: Update, context: ContextTypes.DEFAULT_TYPE, bot: "DevTrackBot"):
    """/vacation [on|off|status] [--until YYYY-MM-DD] [--threshold 0.7] [--no-submit]"""
    args = context.args or []
    sub = args[0].lower() if args else "status"

    try:
        from backend.vacation.auto_responder import get_vacation_state, is_vacation_active
        import sqlite3
        from backend.config import get_path, get

        def _db():
            db_path = get_path("DATABASE_DIR") / get("DATABASE_FILE_NAME")
            return sqlite3.connect(str(db_path))

        if sub == "status":
            state = get_vacation_state()
            if not state or not state.enabled:
                await update.message.reply_text("✈️ Vacation mode: *OFF*", parse_mode="Markdown")
            else:
                until = f"until {state.until}" if state.until else "indefinite"
                msg = (
                    f"✈️ Vacation mode: *ON* ({until})\n"
                    f"Confidence threshold: {state.confidence_threshold:.0%}\n"
                    f"Auto-submit: {'yes' if state.auto_submit else 'no'}"
                )
                await update.message.reply_text(msg, parse_mode="Markdown")

        elif sub == "on":
            until = ""
            threshold = 0.7
            auto_submit = True
            i = 1
            while i < len(args):
                if args[i] == "--until" and i + 1 < len(args):
                    i += 1
                    until = args[i]
                elif args[i] == "--threshold" and i + 1 < len(args):
                    i += 1
                    threshold = float(args[i])
                elif args[i] == "--no-submit":
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
            msg = f"✈️ Vacation mode *ON*"
            if until:
                msg += f" (until {until})"
            await update.message.reply_text(msg, parse_mode="Markdown")

        elif sub == "off":
            conn = _db()
            conn.execute("UPDATE vacation_mode SET enabled=0 WHERE id=1")
            conn.commit(); conn.close()
            await update.message.reply_text("✅ Vacation mode *OFF* — normal prompting resumed", parse_mode="Markdown")

        else:
            await update.message.reply_text("Usage: /vacation [on|off|status]")

    except Exception as e:
        await update.message.reply_text(f"Error: {e}")
