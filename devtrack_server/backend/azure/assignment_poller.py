"""
Azure DevOps assignment poller.

Runs as a background process started by the Go daemon. Polls Azure DevOps
every AZURE_POLL_INTERVAL_MINS minutes and sends a Telegram notification
when a new work item is assigned to the authenticated user.

The notification is formatted like /issues with an inline "View Details"
button. On first run it seeds the seen-set silently to avoid flooding.
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
    format="%(asctime)s assignment-poller %(levelname)s %(message)s",
)
logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_PLATFORM = "azure"


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

async def _notify(bot, chat_ids: list, wi) -> None:
    """Send a Telegram notification for a newly assigned work item."""
    from telegram import InlineKeyboardButton, InlineKeyboardMarkup

    sprint = wi.iteration_path.split("\\")[-1] if wi.iteration_path else ""
    desc = _strip_html(wi.description or "")
    if len(desc) > 150:
        desc = desc[:150] + "..."

    lines = [
        f"🔔 <b>New {_h(wi.work_item_type)} assigned to you</b>",
        "",
        f"<code>#{wi.id}</code>  <b>{_h(wi.title)}</b>",
        f"State: {_h(wi.state)}",
    ]
    if sprint:
        lines.append(f"Sprint: {_h(sprint)}")
    if wi.due_date:
        lines.append(f"Due: {_h(wi.due_date)}")
    if desc:
        lines.append(f"<i>{_h(desc)}</i>")

    text = "\n".join(lines)
    keyboard = InlineKeyboardMarkup([
        [InlineKeyboardButton("View Full Details", callback_data=f"view_issue:{wi.id}")]
    ])

    for chat_id in chat_ids:
        try:
            await bot.send_message(
                chat_id=chat_id,
                text=text,
                parse_mode="HTML",
                reply_markup=keyboard,
            )
            logger.info(f"Notified chat {chat_id} about work item #{wi.id}")
        except Exception as e:
            logger.error(f"Failed to notify chat {chat_id}: {e}")


# ---------------------------------------------------------------------------
# Poll loop
# ---------------------------------------------------------------------------

async def run() -> None:
    from backend.config import get_telegram_bot_token, get_telegram_allowed_chat_ids, get_azure_poll_interval_mins
    token = get_telegram_bot_token()
    chat_ids = get_telegram_allowed_chat_ids()
    interval_mins = get_azure_poll_interval_mins()

    if not token:
        logger.error("TELEGRAM_BOT_TOKEN not set — assignment poller disabled")
        return

    if not chat_ids:
        logger.warning("No TELEGRAM_ALLOWED_CHAT_IDS set — assignment poller disabled")
        return

    from telegram import Bot
    from backend.azure.client import AzureDevOpsClient

    bot = Bot(token=token)
    client = AzureDevOpsClient()

    if not client.is_configured():
        logger.error("Azure DevOps not configured — assignment poller disabled")
        await client.close()
        return

    logger.info(f"Assignment poller started (interval={interval_mins}m, chats={chat_ids})")

    # First run: seed seen-set silently to avoid flooding on startup
    seen = _load_seen()
    if not seen:
        logger.info("First run: seeding seen assignments without notifying")
        try:
            items = await client.get_my_work_items(max_results=200)
            seen = {wi.id for wi in items}
            _save_seen(seen)
            logger.info(f"Seeded {len(seen)} existing assignment(s)")
        except Exception as e:
            logger.error(f"Seed error: {e}")

    # Poll loop
    while True:
        await asyncio.sleep(interval_mins * 60)
        try:
            items = await client.get_my_work_items(max_results=200)
            new_items = [wi for wi in items if wi.id not in seen]
            if new_items:
                logger.info(f"{len(new_items)} new assignment(s) found")
                for wi in new_items:
                    await _notify(bot, chat_ids, wi)
                    seen.add(wi.id)
                _save_seen(seen)
            else:
                logger.debug("No new assignments")
        except Exception as e:
            logger.error(f"Poll error: {e}")


if __name__ == "__main__":
    asyncio.run(run())
