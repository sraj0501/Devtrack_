"""
MongoDB storage layer for DevTrack learning data.

Collections:
  communication_samples  — deduplicated by Teams/Outlook message_id (_id)
  user_profiles          — one document per user (_id = user_email)
  learning_state         — delta tracking (_id = user_email)

If MONGODB_URI is not set in .env, all methods are no-ops and
is_available() returns False, falling back to file-based storage.

Install: uv sync --extra mongodb
"""

from __future__ import annotations

import logging
from dataclasses import asdict
from datetime import datetime, timezone
from typing import TYPE_CHECKING, Optional

logger = logging.getLogger(__name__)

try:
    import motor.motor_asyncio
    _motor_available = True
except ImportError:
    _motor_available = False
    logger.debug("motor not installed; MongoDB learning storage unavailable. "
                 "Install with: uv sync --extra mongodb")

if TYPE_CHECKING:
    from backend.personalized_ai import UserProfile


class MongoLearningStore:
    """Async MongoDB store for learning data. Thread-safe singleton via get_store()."""

    def __init__(self, uri: str, db_name: str = "devtrack"):
        if not uri or not _motor_available:
            self._client = None
            self._db = None
            self._samples = None
            self._profiles = None
            self._state = None
            return

        self._client = motor.motor_asyncio.AsyncIOMotorClient(uri)
        self._db = self._client[db_name]
        self._samples = self._db["communication_samples"]
        self._profiles = self._db["user_profiles"]
        self._state = self._db["learning_state"]
        logger.info(f"MongoDB learning store connected: {db_name}")

    def is_available(self) -> bool:
        return self._client is not None

    async def ensure_indexes(self) -> None:
        """Create indexes on first use. Safe to call multiple times."""
        if not self.is_available():
            return
        await self._samples.create_index(
            [("user_email", 1), ("timestamp", -1)], background=True
        )

    async def upsert_sample(
        self,
        message_id: str,
        user_email: str,
        source: str,
        context_type: str,
        trigger: str,
        response: str,
        timestamp: datetime,
        metadata: dict,
    ) -> bool:
        """
        Insert a communication sample using message_id as the dedup key.

        Returns True if newly inserted, False if already existed (skipped).
        Uses $setOnInsert so re-runs are fully idempotent.
        """
        if not self.is_available() or not message_id:
            return False

        if timestamp.tzinfo is None:
            timestamp = timestamp.replace(tzinfo=timezone.utc)

        doc = {
            "user_email": user_email,
            "source": source,
            "context_type": context_type,
            "trigger": trigger,
            "response": response,
            "timestamp": timestamp,
            "metadata": metadata or {},
            "collected_at": datetime.now(tz=timezone.utc),
        }
        result = await self._samples.update_one(
            {"_id": message_id},
            {"$setOnInsert": doc},
            upsert=True,
        )
        return result.upserted_id is not None

    async def upsert_profile(self, user_email: str, profile: "UserProfile") -> None:
        """Persist the computed style profile. Replaces previous version."""
        if not self.is_available():
            return

        profile_dict = asdict(profile)
        profile_dict["last_updated"] = profile.last_updated.isoformat()

        # Serialize nested CommunicationPattern objects
        patterns = {}
        for key, pat in profile.response_patterns.items():
            patterns[key] = asdict(pat)
        profile_dict["response_patterns"] = patterns

        profile_dict["saved_at"] = datetime.now(tz=timezone.utc)

        await self._profiles.replace_one(
            {"_id": user_email},
            {"_id": user_email, **profile_dict},
            upsert=True,
        )

    async def load_last_collected(self, user_email: str) -> Optional[datetime]:
        """Return the timestamp of the last successful collection, or None."""
        if not self.is_available():
            return None
        doc = await self._state.find_one({"_id": user_email})
        if doc:
            ts = doc.get("last_collected")
            if ts and isinstance(ts, datetime) and ts.tzinfo is None:
                ts = ts.replace(tzinfo=timezone.utc)
            return ts
        return None

    async def save_last_collected(self, user_email: str, ts: datetime) -> None:
        """Persist the collection timestamp for the next delta run."""
        if not self.is_available():
            return
        if ts.tzinfo is None:
            ts = ts.replace(tzinfo=timezone.utc)
        await self._state.update_one(
            {"_id": user_email},
            {"$set": {"last_collected": ts, "updated_at": datetime.now(tz=timezone.utc)}},
            upsert=True,
        )

    async def load_samples_for_profile(
        self, user_email: str, limit: int = 5000
    ) -> list[dict]:
        """Load recent samples to seed PersonalizedAI for profile recomputation."""
        if not self.is_available():
            return []
        cursor = self._samples.find(
            {"user_email": user_email},
            sort=[("timestamp", -1)],
            limit=limit,
        )
        return await cursor.to_list(length=limit)

    async def count_samples(self, user_email: str) -> int:
        if not self.is_available():
            return 0
        return await self._samples.count_documents({"user_email": user_email})

    async def reset_user_data(self, user_email: str) -> dict:
        """
        Delete all MongoDB learning data for a user.

        Returns counts of deleted documents per collection.
        """
        if not self.is_available():
            return {}
        results = {}
        r = await self._samples.delete_many({"user_email": user_email})
        results["samples"] = r.deleted_count
        r = await self._profiles.delete_many({"_id": user_email})
        results["profiles"] = r.deleted_count
        r = await self._state.delete_many({"_id": user_email})
        results["state"] = r.deleted_count
        return results


# Module-level singleton
_store: Optional[MongoLearningStore] = None


def get_store() -> MongoLearningStore:
    """
    Return the singleton MongoLearningStore (lazy-init from .env config).

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
        _store = MongoLearningStore(uri, db_name)
    return _store
