"""Entry point for the DevTrack Slack bot.

Usage:
    python -m backend.slack

Requires:
    SLACK_ENABLED=true
    SLACK_BOT_TOKEN=xoxb-...    (Bot User OAuth Token)
    SLACK_APP_TOKEN=xapp-...    (App-Level Token with connections:write scope)
    SLACK_ALLOWED_CHANNEL_IDS=C123,C456   (optional; empty = accept from all channels)
"""
import logging
import os
import sys

# Add repo root to path
_repo_root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
if _repo_root not in sys.path:
    sys.path.insert(0, _repo_root)

from dotenv import load_dotenv

env_file = os.environ.get("DEVTRACK_ENV_FILE", os.path.join(_repo_root, ".env"))
if os.path.exists(env_file):
    load_dotenv(env_file)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s slack-bot %(levelname)s %(message)s",
)
logger = logging.getLogger(__name__)


def main():
    from backend.config import get_slack_enabled, get_slack_bot_token, get_slack_app_token, get_slack_allowed_channel_ids
    enabled = get_slack_enabled()
    if not enabled:
        logger.error("Slack bot is disabled. Set SLACK_ENABLED=true in .env")
        sys.exit(1)

    bot_token = get_slack_bot_token()
    app_token = get_slack_app_token()
    if not bot_token:
        logger.error("SLACK_BOT_TOKEN is required (xoxb-...).")
        sys.exit(1)
    if not app_token:
        logger.error("SLACK_APP_TOKEN is required (xapp-...). Enable Socket Mode in your Slack app.")
        sys.exit(1)

    allowed_channel_ids: set[str] = set(get_slack_allowed_channel_ids())

    if not allowed_channel_ids:
        logger.warning(
            "SLACK_ALLOWED_CHANNEL_IDS is empty — bot will accept commands from any channel (dev mode)"
        )

    from backend.slack.bot import SlackBot
    bot = SlackBot(
        bot_token=bot_token,
        app_token=app_token,
        allowed_channel_ids=allowed_channel_ids,
    )
    logger.info("Starting DevTrack Slack bot (Socket Mode)…")
    bot.run()


if __name__ == "__main__":
    main()
