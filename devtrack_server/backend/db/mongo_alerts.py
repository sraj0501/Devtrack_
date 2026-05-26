"""
MongoDB storage layer for DevTrack alert/notification data.

Collections:
  notifications  — one document per alert event (_id = ObjectId)
  alert_state    — delta tracking per (user, source) pair (_id = "<email>:<source>")

If MONGODB_URI is not set in .env, all methods are no-ops and
is_available() returns False, falling back to in-memory / print-only mode.
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)

try:
    import motor.motor_asyncio
    from bson import ObjectId
    _motor_available = True
except ImportError:
    _motor_available = False
    logger.debug(
        "motor not installed; MongoDB alert storage unavailable. "
        "Install with: uv sync --extra mongodb"
    )


class MongoAlertsStore:
    """Async MongoDB store for alert notifications. Thread-safe singleton via get_store()."""

    def __init__(self, uri: str, db_name: str = "devtrack"):
        if not uri or not _motor_available:
            self._client = None
            self._db = None
            self._notifications = None
            self._state = None
            return

        self._client = motor.motor_asyncio.AsyncIOMotorClient(uri)
        self._db = self._client[db_name]
        self._notifications = self._db["notifications"]
        self._state = self._db["alert_state"]
        logger.info(f"MongoDB alerts store connected: {db_name}")

    def is_available(self) -> bool:
        return self._client is not None

    async def ensure_indexes(self) -> None:
        """Create indexes on first use. Safe to call multiple times."""
        if not self.is_available():
            return
        await self._notifications.create_index(
            [("source", 1), ("timestamp", -1)], background=True
        )
        await self._notifications.create_index(
            [("read", 1), ("timestamp", -1)], background=True
        )
        await self._notifications.create_index(
            [("dismissed", 1)], background=True
        )

    async def insert_notification(self, doc: Dict[str, Any]) -> Optional[str]:
        """
        Insert a new notification document.

        Returns the string representation of the inserted _id, or None on failure.
        """
        if not self.is_available():
            return None
        if "timestamp" not in doc:
            doc["timestamp"] = datetime.now(tz=timezone.utc)
        elif doc["timestamp"].tzinfo is None:
            doc["timestamp"] = doc["timestamp"].replace(tzinfo=timezone.utc)
        doc.setdefault("read", False)
        doc.setdefault("dismissed", False)
        result = await self._notifications.insert_one(doc)
        return str(result.inserted_id)

    async def mark_all_read(self, source: Optional[str] = None) -> int:
        """
        Mark all unread notifications as read.

        If ``source`` is given, only notifications from that source are affected.
        Returns the count of documents updated.
        """
        if not self.is_available():
            return 0
        query: Dict[str, Any] = {"read": False}
        if source:
            query["source"] = source
        result = await self._notifications.update_many(
            query, {"$set": {"read": True}}
        )
        return result.modified_count

    async def get_notifications(
        self,
        source: Optional[str] = None,
        unread_only: bool = True,
        hours: int = 24,
        limit: int = 100,
    ) -> List[Dict[str, Any]]:
        """
        Retrieve notifications, newest first.

        Args:
            source: Filter by source (e.g. "github"). None = all sources.
            unread_only: If True only return unread notifications.
            hours: How many hours back to look (0 = no time filter).
            limit: Max documents to return.
        """
        if not self.is_available():
            return []

        query: Dict[str, Any] = {"dismissed": False}
        if source:
            query["source"] = source
        if unread_only:
            query["read"] = False
        if hours > 0:
            cutoff = datetime.now(tz=timezone.utc)
            from datetime import timedelta
            cutoff -= timedelta(hours=hours)
            query["timestamp"] = {"$gte": cutoff}

        cursor = self._notifications.find(
            query,
            sort=[("timestamp", -1)],
            limit=limit,
        )
        docs = await cursor.to_list(length=limit)
        # Convert ObjectId to string for JSON compatibility
        for doc in docs:
            if "_id" in doc:
                doc["_id"] = str(doc["_id"])
        return docs

    async def count_unread(self, source: Optional[str] = None) -> int:
        """Count unread, non-dismissed notifications."""
        if not self.is_available():
            return 0
        query: Dict[str, Any] = {"read": False, "dismissed": False}
        if source:
            query["source"] = source
        return await self._notifications.count_documents(query)

    # ------------------------------------------------------------------
    # Alert state (delta tracking)
    # ------------------------------------------------------------------

    async def load_last_checked(self, user_id: str, source: str) -> Optional[datetime]:
        """
        Return the last_checked timestamp for the given (user, source) pair.

        ``user_id`` is typically the user's email address or GITHUB_USER env var.
        """
        if not self.is_available():
            return None
        key = f"{user_id}:{source}"
        doc = await self._state.find_one({"_id": key})
        if doc:
            ts = doc.get("last_checked")
            if ts and isinstance(ts, datetime) and ts.tzinfo is None:
                ts = ts.replace(tzinfo=timezone.utc)
            return ts
        return None

    async def save_last_checked(
        self, user_id: str, source: str, ts: datetime
    ) -> None:
        """Persist the last_checked timestamp for delta polling."""
        if not self.is_available():
            return
        if ts.tzinfo is None:
            ts = ts.replace(tzinfo=timezone.utc)
        key = f"{user_id}:{source}"
        await self._state.update_one(
            {"_id": key},
            {
                "$set": {
                    "last_checked": ts,
                    "source": source,
                    "user_id": user_id,
                    "updated_at": datetime.now(tz=timezone.utc),
                }
            },
            upsert=True,
        )


# Module-level singleton
_store: Optional[MongoAlertsStore] = None


def get_store() -> MongoAlertsStore:
    """
    Return the singleton MongoAlertsStore (lazy-init from .env config).

    If MONGODB_URI is not set, returns a no-op store (is_available() == False).
    """
    global _store
    if _store is None:
        try:
            from backend.config import mongodb_uri, mongodb_db_name
            uri = mongodb_uri()
            db_name = mongodb_db_name()
        except Exception:
            uri = ""
            db_name = "devtrack"
        _store = MongoAlertsStore(uri, db_name)
    return _store
