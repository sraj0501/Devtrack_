"""
Alert poller for DevTrack.

Background async loop that polls GitHub and Azure DevOps for events
relevant to the developer, stores results in MongoDB, and delivers
OS/terminal notifications.

Usage (standalone):
    uv run python -m backend.alert_poller          # run poll loop
    uv run python -m backend.alert_poller --show   # show unread (last 24h)
    uv run python -m backend.alert_poller --all    # show all
    uv run python -m backend.alert_poller --clear  # mark all as read

Configuration (.env):
    ALERT_ENABLED=true
    ALERT_POLL_INTERVAL_SECS=300
    ALERT_GITHUB_ENABLED=true
    ALERT_AZURE_ENABLED=true
    ALERT_NOTIFY_ASSIGNED=true
    ALERT_NOTIFY_COMMENTS=true
    ALERT_NOTIFY_STATUS_CHANGES=true
    ALERT_NOTIFY_REVIEW_REQUESTED=true
"""

from __future__ import annotations

import asyncio
import logging
import os
import sys
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import backend.config as cfg
from backend.db.mongo_alerts import get_store

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Config helpers
# ---------------------------------------------------------------------------

def _is_enabled() -> bool:
    return cfg.get_bool("ALERT_ENABLED", True)


def _poll_interval() -> int:
    return cfg.get_int("ALERT_POLL_INTERVAL_SECS", 300)


def _user_id() -> str:
    """
    Determine the user identifier for alert state tracking.

    Priority:
    1. GITHUB_USER env var (explicit)
    2. user_email from consent.json (if learning is enabled)
    3. EMAIL env var
    4. empty string (state not persisted between runs)
    """
    uid = cfg.get("GITHUB_USER")
    if uid:
        return uid
    # Try consent.json
    try:
        import json
        consent_path = cfg.learning_dir() / "consent.json"
        if consent_path.exists():
            data = json.loads(consent_path.read_text())
            email = data.get("user_email", "")
            if email:
                return email
    except Exception:
        pass
    return cfg.get("EMAIL", "")


# ---------------------------------------------------------------------------
# Poll one cycle
# ---------------------------------------------------------------------------

def _sqlite_load_last_checked(user_id: str, source: str) -> Optional[datetime]:
    """Load last_checked from SQLite alert_state (fallback when MongoDB unavailable)."""
    try:
        import sqlite3
        db_path = cfg.database_path()
        if not db_path.exists():
            return None
        key = f"{user_id}:{source}"
        with sqlite3.connect(str(db_path)) as conn:
            row = conn.execute(
                "SELECT last_checked FROM alert_state WHERE id=?", (key,)
            ).fetchone()
            if row:
                ts_str = row[0]
                try:
                    ts = datetime.fromisoformat(ts_str)
                    if ts.tzinfo is None:
                        ts = ts.replace(tzinfo=timezone.utc)
                    return ts
                except Exception:
                    return None
    except Exception as e:
        logger.debug(f"SQLite alert_state load failed: {e}")
    return None


def _sqlite_save_last_checked(user_id: str, source: str, ts: datetime) -> None:
    """Save last_checked to SQLite alert_state (fallback when MongoDB unavailable)."""
    try:
        import sqlite3
        db_path = cfg.database_path()
        if not db_path.exists():
            return
        if ts.tzinfo is None:
            ts = ts.replace(tzinfo=timezone.utc)
        key = f"{user_id}:{source}"
        with sqlite3.connect(str(db_path)) as conn:
            conn.execute(
                """INSERT INTO alert_state (id, user_id, source, last_checked, updated_at)
                   VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
                   ON CONFLICT(id) DO UPDATE SET
                       last_checked=excluded.last_checked,
                       updated_at=CURRENT_TIMESTAMP""",
                (key, user_id, source, ts.isoformat()),
            )
    except Exception as e:
        logger.debug(f"SQLite alert_state save failed: {e}")


async def _poll_github(
    store,
    user_id: str,
) -> List[Dict[str, Any]]:
    """Run one poll cycle for GitHub and return new notification dicts."""
    from backend.alerters.github_alerter import GitHubAlerter

    last_checked: Optional[datetime] = None
    if user_id:
        if store.is_available():
            last_checked = await store.load_last_checked(user_id, "github")
        else:
            last_checked = _sqlite_load_last_checked(user_id, "github")

    notifications: List[Dict[str, Any]] = []
    async with GitHubAlerter() as alerter:
        if not alerter.is_configured():
            logger.debug("GitHub alerter: not configured, skipping")
            return []
        notifications = await alerter.poll(last_checked=last_checked)

    # Persist last_checked timestamp
    if user_id and notifications is not None:
        now = datetime.now(tz=timezone.utc)
        if store.is_available():
            await store.save_last_checked(user_id, "github", now)
        else:
            _sqlite_save_last_checked(user_id, "github", now)

    return notifications


async def _poll_azure(
    store,
    user_id: str,
) -> List[Dict[str, Any]]:
    """Run one poll cycle for Azure DevOps and return new notification dicts."""
    from backend.alerters.azure_alerter import AzureAlerter

    last_checked: Optional[datetime] = None
    if user_id:
        if store.is_available():
            last_checked = await store.load_last_checked(user_id, "azure")
        else:
            last_checked = _sqlite_load_last_checked(user_id, "azure")

    notifications: List[Dict[str, Any]] = []
    async with AzureAlerter() as alerter:
        if not alerter.is_configured():
            logger.debug("Azure alerter: not configured, skipping")
            return []
        notifications = await alerter.poll(last_checked=last_checked)

    # Persist last_checked timestamp
    if user_id and notifications is not None:
        now = datetime.now(tz=timezone.utc)
        if store.is_available():
            await store.save_last_checked(user_id, "azure", now)
        else:
            _sqlite_save_last_checked(user_id, "azure", now)

    return notifications


async def _poll_jira(
    store,
    user_id: str,
) -> List[Dict[str, Any]]:
    """Run one poll cycle for Jira and return new notification dicts."""
    from backend.alerters.jira_alerter import JiraAlerter

    last_checked: Optional[datetime] = None
    if user_id:
        if store.is_available():
            last_checked = await store.load_last_checked(user_id, "jira")
        else:
            last_checked = _sqlite_load_last_checked(user_id, "jira")

    notifications: List[Dict[str, Any]] = []
    async with JiraAlerter() as alerter:
        if not alerter.is_configured():
            logger.debug("Jira alerter: not configured, skipping")
            return []
        notifications = await alerter.poll(last_checked=last_checked)

    # Persist last_checked timestamp
    if user_id and notifications is not None:
        now = datetime.now(tz=timezone.utc)
        if store.is_available():
            await store.save_last_checked(user_id, "jira", now)
        else:
            _sqlite_save_last_checked(user_id, "jira", now)

    return notifications


async def _run_one_cycle(store, user_id: str) -> List[Dict[str, Any]]:
    """Run a single poll cycle across all enabled sources."""
    if not _is_enabled():
        logger.info("Alert polling disabled (ALERT_ENABLED=false)")
        return []

    all_notifications: List[Dict[str, Any]] = []

    if cfg.get_bool("ALERT_GITHUB_ENABLED", True):
        try:
            github_notifs = await _poll_github(store, user_id)
            all_notifications.extend(github_notifs)
        except Exception as e:
            logger.warning(f"GitHub poll failed: {e}")

    if cfg.get_bool("ALERT_AZURE_ENABLED", True):
        try:
            azure_notifs = await _poll_azure(store, user_id)
            all_notifications.extend(azure_notifs)
        except Exception as e:
            logger.warning(f"Azure poll failed: {e}")

    if cfg.get_bool("ALERT_JIRA_ENABLED", True):
        try:
            jira_notifs = await _poll_jira(store, user_id)
            all_notifications.extend(jira_notifs)
        except Exception as e:
            logger.warning(f"Jira poll failed: {e}")

    # Persist to MongoDB and deliver notifications
    new_notifications = []
    for notif in all_notifications:
        try:
            inserted_id = await store.insert_notification(notif)
            if inserted_id:
                notif["_id"] = inserted_id
                new_notifications.append(notif)
        except Exception as e:
            logger.warning(f"Failed to store notification: {e}")
            # Still deliver even if storage fails
            new_notifications.append(notif)
        # Dual-write to SQLite so the Bubble Tea TUI can read alerts offline
        _write_notification_to_sqlite(notif)

    # Deliver via notifier
    if new_notifications:
        try:
            from backend.alert_notifier import notify_many
            notify_many(new_notifications)
        except Exception as e:
            logger.warning(f"Alert notifier failed: {e}")

    return new_notifications


# ---------------------------------------------------------------------------
# SQLite dual-write (for Bubble Tea TUI offline access)
# ---------------------------------------------------------------------------

def _write_notification_to_sqlite(notif: Dict[str, Any]) -> None:
    """
    Write a notification record to the local SQLite database so that the
    Bubble Tea TUI (Go side) can display alerts without needing MongoDB.

    Non-fatal: any error is logged at DEBUG level and silently ignored.
    """
    try:
        import sqlite3
        db_path = cfg.database_path()
        if not db_path.exists():
            return
        ts = notif.get("timestamp")
        if isinstance(ts, datetime):
            ts_str = ts.isoformat()
        elif isinstance(ts, str):
            ts_str = ts
        else:
            ts_str = datetime.now(tz=timezone.utc).isoformat()

        with sqlite3.connect(str(db_path)) as conn:
            conn.execute(
                """
                INSERT OR IGNORE INTO notifications
                    (source, event_type, ticket_id, title, body, url, read, created_at)
                VALUES (?, ?, ?, ?, ?, ?, 0, ?)
                """,
                (
                    notif.get("source", ""),
                    notif.get("event_type", ""),
                    notif.get("ticket_id", ""),
                    notif.get("title", ""),
                    notif.get("summary", ""),
                    notif.get("url", ""),
                    ts_str,
                ),
            )
    except Exception as e:
        logger.debug(f"SQLite dual-write skipped: {e}")


# ---------------------------------------------------------------------------
# Display helpers (for --show / --all / --clear)
# ---------------------------------------------------------------------------

async def show_alerts(all_alerts: bool = False, clear: bool = False) -> None:
    """
    Display stored alerts or mark them as read.

    Args:
        all_alerts: If True show all alerts (no time filter).
        clear: If True mark all as read instead of displaying.
    """
    store = get_store()
    await store.ensure_indexes()

    if clear:
        count = await store.mark_all_read()
        print(f"Marked {count} notification(s) as read.")
        return

    hours = 0 if all_alerts else 24
    docs = await store.get_notifications(unread_only=False, hours=hours, limit=200)

    if not docs:
        if all_alerts:
            print("No notifications found.")
        else:
            print("No notifications in the last 24 hours.")
        return

    _print_notifications_table(docs)


def _print_notifications_table(docs: List[Dict[str, Any]]) -> None:
    """Print a formatted table of notifications."""
    unread = [d for d in docs if not d.get("read")]
    read = [d for d in docs if d.get("read")]

    if unread:
        print(f"\n{'=' * 70}")
        print(f"  UNREAD NOTIFICATIONS ({len(unread)})")
        print(f"{'=' * 70}")
        for doc in unread:
            _print_one(doc, highlight=True)

    if read:
        print(f"\n{'─' * 70}")
        print(f"  READ ({len(read)})")
        print(f"{'─' * 70}")
        for doc in read:
            _print_one(doc, highlight=False)

    total = len(docs)
    print(f"\n{total} notification(s) total. Use --clear to mark all as read.")


def _print_one(doc: Dict[str, Any], highlight: bool) -> None:
    source = doc.get("source", "").upper()
    event_type = doc.get("event_type", "")
    ticket_id = doc.get("ticket_id", "")
    title = doc.get("title", "")
    summary = doc.get("summary", "")
    url = doc.get("url", "")
    ts = doc.get("timestamp")

    ts_str = ""
    if isinstance(ts, datetime):
        ts_str = ts.strftime("%Y-%m-%d %H:%M")
    elif isinstance(ts, str):
        ts_str = ts[:16]

    prefix = ">> " if highlight else "   "
    print(f"{prefix}[{source}] {event_type.upper()} — {ticket_id}")
    print(f"   {title}")
    if summary:
        print(f"   {summary}")
    print(f"   {url}")
    if ts_str:
        print(f"   {ts_str}")
    print()


# ---------------------------------------------------------------------------
# Continuous poll loop
# ---------------------------------------------------------------------------

async def run_poll_loop() -> None:
    """Run the alert poller indefinitely, sleeping between cycles."""
    store = get_store()
    await store.ensure_indexes()
    user_id = _user_id()
    interval = _poll_interval()

    logger.info(
        f"Alert poller starting (interval={interval}s, user={user_id or 'unknown'})"
    )

    while True:
        try:
            notifs = await _run_one_cycle(store, user_id)
            if notifs:
                logger.info(f"Alert poller: {len(notifs)} new notification(s)")
        except asyncio.CancelledError:
            logger.info("Alert poller: cancelled, shutting down")
            break
        except Exception as e:
            logger.warning(f"Alert poller: cycle error: {e}")

        await asyncio.sleep(interval)


# ---------------------------------------------------------------------------
# Module entry point
# ---------------------------------------------------------------------------

def _setup_logging() -> None:
    level = cfg.get("LOG_LEVEL", "INFO").upper()
    logging.basicConfig(
        level=getattr(logging, level, logging.INFO),
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )


if __name__ == "__main__":
    _setup_logging()

    args = sys.argv[1:]
    if "--show" in args or "--all" in args or "--clear" in args:
        all_flag = "--all" in args
        clear_flag = "--clear" in args
        asyncio.run(show_alerts(all_alerts=all_flag, clear=clear_flag))
    else:
        asyncio.run(run_poll_loop())
