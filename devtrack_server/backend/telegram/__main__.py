"""Entry point for running the Telegram bot standalone."""
import logging
import os
import sys

# Add repo root to path
repo_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
if repo_root not in sys.path:
    sys.path.insert(0, repo_root)

from dotenv import load_dotenv

# Load .env
env_file = os.environ.get("DEVTRACK_ENV_FILE", os.path.join(repo_root, ".env"))
if os.path.exists(env_file):
    load_dotenv(env_file)

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def main():
    from backend.config import get, get_bool

    if not get_bool("TELEGRAM_ENABLED", False):
        logger.error("Telegram bot is disabled. Set TELEGRAM_ENABLED=true in .env")
        sys.exit(1)

    token = get("TELEGRAM_BOT_TOKEN", "")
    if not token:
        logger.error("TELEGRAM_BOT_TOKEN is required. Get one from @BotFather on Telegram.")
        sys.exit(1)

    allowed_ids_str = get("TELEGRAM_ALLOWED_CHAT_IDS", "")
    allowed_ids = set()
    if allowed_ids_str:
        for id_str in allowed_ids_str.split(","):
            id_str = id_str.strip()
            if id_str:
                try:
                    allowed_ids.add(int(id_str))
                except ValueError:
                    logger.warning(f"Invalid chat ID in TELEGRAM_ALLOWED_CHAT_IDS: {id_str}")

    if not allowed_ids:
        logger.warning("TELEGRAM_ALLOWED_CHAT_IDS is empty -- bot will accept commands from anyone (dev mode)")

    from backend.telegram.bot import DevTrackBot
    bot = DevTrackBot(token=token, allowed_chat_ids=allowed_ids)

    logger.info("Starting DevTrack Telegram bot...")
    bot.run()


if __name__ == "__main__":
    main()
