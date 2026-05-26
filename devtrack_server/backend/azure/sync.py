"""
Azure DevOps sync utilities.

Provides a simple sync layer that fetches work items from Azure DevOps
and reports status. State is persisted in SQLite (devtrack.db) via
backend.db.platform_store.

No dependency on ProjectManager — designed to work standalone.
"""

import logging
import os
from datetime import datetime, timedelta, timezone
from typing import Dict, List, Optional

from backend.azure.client import AzureDevOpsClient, AzureWorkItem
from backend.db.platform_store import (
    clear_sync_items,
    load_sync_items,
    load_sync_meta,
    save_sync_items,
)

logger = logging.getLogger(__name__)

_PLATFORM = "azure"


# ---------------------------------------------------------------------------
# AzureSync — simple fetch + report, no ProjectManager dependency
# ---------------------------------------------------------------------------

class AzureSync:
    """Fetch Azure DevOps work items and maintain a local sync state."""

    def __init__(self, client: Optional[AzureDevOpsClient] = None) -> None:
        self.client = client or AzureDevOpsClient()

    # -- public API ---------------------------------------------------------

    async def sync(self, full: bool = True, hours: Optional[int] = None) -> Dict:
        """Fetch work items and persist state.

        Args:
            full:  If True (default), clear existing cache and do a complete resync.
            hours: If set, only fetch items changed in the last N hours and merge
                   into the existing cache. Overrides `full`.
        """
        changed_after: Optional[datetime] = None
        existing: Dict = {}

        if hours is not None:
            # Windowed sync — merge into existing cache
            changed_after = datetime.now(timezone.utc) - timedelta(hours=hours)
            existing = load_sync_items(_PLATFORM)
            logger.info(f"Azure sync: windowed mode, last {hours}h (since {changed_after.isoformat()})")
        elif full:
            # Full sync — start fresh
            logger.info("Azure sync: full mode, clearing existing cache")
            clear_sync_items(_PLATFORM)
        else:
            # Incremental default: read window from env
            from backend.config import get_azure_sync_window_hours
            window_hours = get_azure_sync_window_hours()
            if window_hours > 0:
                changed_after = datetime.now(timezone.utc) - timedelta(hours=window_hours)
                existing = load_sync_items(_PLATFORM)
                logger.info(f"Azure sync: env window {window_hours}h")
            else:
                logger.info("Azure sync: full mode (AZURE_SYNC_WINDOW_HOURS=0)")
                clear_sync_items(_PLATFORM)

        items = await self.client.get_my_work_items(
            max_results=200,
            changed_after=changed_after,
        )

        now = datetime.now(timezone.utc).isoformat()
        fetched_records = {
            str(wi.id): {
                "id": wi.id,
                "title": wi.title,
                "description": wi.description,
                "state": wi.state,
                "type": wi.work_item_type,
                "assigned_to": wi.assigned_to,
                "area_path": wi.area_path,
                "iteration_path": wi.iteration_path,
                "due_date": wi.due_date,
                "tags": wi.tags,
                "url": wi.url,
                "synced_at": now,
            }
            for wi in items
        }

        # Merge: fetched records overwrite existing ones
        merged = {**existing, **fetched_records}

        sync_mode = f"windowed:{hours}h" if hours else ("full" if not existing else f"windowed:{window_hours}h" if changed_after else "full")
        save_sync_items(_PLATFORM, merged, now)
        logger.info(f"Azure sync complete: {len(items)} fetched, {len(merged)} total in cache")

        by_state: Dict[str, List[AzureWorkItem]] = {}
        for wi in items:
            by_state.setdefault(wi.state, []).append(wi)

        return {
            "total": len(merged),
            "fetched": len(items),
            "by_state": {s: len(v) for s, v in by_state.items()},
            "items": items,
            "last_sync": now,
            "mode": sync_mode,
        }

    async def get_work_item(self, item_id: int) -> Optional[AzureWorkItem]:
        """Fetch a single work item by ID."""
        return await self.client.get_work_item(item_id)

    async def add_comment(self, item_id: int, comment: str) -> bool:
        return await self.client.add_comment(item_id, comment)

    async def update_state(self, item_id: int, new_state: str) -> bool:
        return await self.client.update_work_item_state(item_id, new_state)

    def get_cached(self) -> Dict:
        """Return locally cached state without hitting the API."""
        return {
            "last_sync": load_sync_meta(_PLATFORM),
            "work_items": load_sync_items(_PLATFORM),
        }
