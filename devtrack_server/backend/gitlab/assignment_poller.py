"""
GitLab assignment poller.

Runs as a background process started by the Go daemon. Polls GitLab
every GITLAB_POLL_INTERVAL_MINS minutes and sends a Telegram notification
when a new issue is assigned to the authenticated user.

The notification is formatted with an inline "View Details" button.
On first run it seeds the seen-set silently to avoid flooding.
"""

import asyncio
import logging
import os
import re
import sys
from pathlib import Path

# Add repo root to path
repo_root = Path(__file__).resolve().parents[2]
if str(repo_root) not in sys.path:
    sys.path.insert(0, str(repo_root))


def _load_env() -> None:
    here = Path(__file__).resolve()
    for parent in [here, *here.parents]:
        env_file = parent / ".env"
        if env_file.exists():
            try:
                from dotenv import load_dotenv
                load_dotenv(env_file, override=True)
            except ImportError:
                pass
            return


_load_env()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s gitlab-poller %(levelname)s %(message)s",
)
logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_PLATFORM = "gitlab"


def _load_seen() -> set:
    from backend.db.platform_store import load_seen_events
    return load_seen_events(_PLATFORM)


def _save_seen(seen: set) -> None:
    from backend.db.platform_store import mark_events_seen
    mark_events_seen(_PLATFORM, {str(k) for k in seen})


def _h(text: str) -> str:
    """Escape HTML special chars for Telegram HTML parse mode."""
    return str(text).replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def _strip_html(text: str) -> str:
    """Strip HTML tags and decode common entities."""
    text = re.sub(r"<br\s*/?>", "\n", text, flags=re.IGNORECASE)
    text = re.sub(r"<[^>]+>", "", text)
    text = text.replace("&nbsp;", " ").replace("&amp;", "&").replace("&lt;", "<").replace("&gt;", ">")
    return " ".join(text.split()).strip()


# ---------------------------------------------------------------------------
# Notification
# ---------------------------------------------------------------------------

async def _notify(bot, chat_ids: list, issue) -> None:
    """Send a Telegram notification for a newly assigned GitLab issue."""
    from telegram import InlineKeyboardButton, InlineKeyboardMarkup

    desc = _strip_html(issue.description or "")
    if len(desc) > 150:
        desc = desc[:150] + "..."

    lines = [
        "🔔 <b>New GitLab issue assigned to you</b>",
        "",
        f"<code>!{issue.iid}</code>  <b>{_h(issue.title)}</b>",
        f"State: {_h(issue.state)}",
        f"Project: {issue.project_id}",
    ]
    if issue.milestone_title:
        lines.append(f"Milestone: {_h(issue.milestone_title)}")
    if issue.due_date:
        lines.append(f"Due: {_h(issue.due_date)}")
    if desc:
        lines.append(f"<i>{_h(desc)}</i>")

    text = "\n".join(lines)
    # Use global issue.id (not iid) so the callback can fetch by global ID
    keyboard = InlineKeyboardMarkup([
        [InlineKeyboardButton("View Details", callback_data=f"view_gitlab_issue:{issue.id}")]
    ])

    for chat_id in chat_ids:
        try:
            await bot.send_message(
                chat_id=chat_id,
                text=text,
                parse_mode="HTML",
                reply_markup=keyboard,
            )
            logger.info(f"Notified chat {chat_id} about GitLab issue !{issue.iid} (id={issue.id})")
        except Exception as e:
            logger.error(f"Failed to notify chat {chat_id}: {e}")


# ---------------------------------------------------------------------------
# Poll loop
# ---------------------------------------------------------------------------

async def run() -> None:
    from backend.config import get_telegram_bot_token, get_telegram_allowed_chat_ids, get_gitlab_poll_interval_mins
    token = get_telegram_bot_token()
    chat_ids = get_telegram_allowed_chat_ids()
    interval_mins = get_gitlab_poll_interval_mins()

    if not token:
        logger.error("TELEGRAM_BOT_TOKEN not set — GitLab assignment poller disabled")
        return

    if not chat_ids:
        logger.warning("No TELEGRAM_ALLOWED_CHAT_IDS set — GitLab assignment poller disabled")
        return

    from telegram import Bot
    from backend.gitlab.client import GitLabClient

    bot = Bot(token=token)
    client = GitLabClient()

    if not client.is_configured():
        logger.error("GitLab not configured — assignment poller disabled")
        await client.close()
        return

    logger.info(f"GitLab assignment poller started (interval={interval_mins}m, chats={chat_ids})")

    # First run: seed seen-set silently to avoid flooding on startup
    seen = _load_seen()
    if not seen:
        logger.info("First run: seeding seen assignments without notifying")
        try:
            items = await client.get_my_issues(max_results=200)
            seen = {issue.id for issue in items}
            _save_seen(seen)
            logger.info(f"Seeded {len(seen)} existing assignment(s)")
        except Exception as e:
            logger.error(f"Seed error: {e}")

    # Poll loop
    while True:
        await asyncio.sleep(interval_mins * 60)
        try:
            items = await client.get_my_issues(max_results=200)
            new_items = [issue for issue in items if issue.id not in seen]
            if new_items:
                logger.info(f"{len(new_items)} new GitLab assignment(s) found")
                for issue in new_items:
                    await _notify(bot, chat_ids, issue)
                    seen.add(issue.id)
                _save_seen(seen)
            else:
                logger.debug("No new GitLab assignments")
        except Exception as e:
            logger.error(f"Poll error: {e}")


if __name__ == "__main__":
    asyncio.run(run())
