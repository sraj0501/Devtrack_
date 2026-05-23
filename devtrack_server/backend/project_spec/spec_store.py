"""
Persist and retrieve project specs.

Storage backends (in priority order):
  1. MongoDB — when MONGODB_URI + motor are available (spec_id is the _id)
  2. File-system — JSON files under DATA_DIR/project_specs/ (always available)

All methods are async.
"""

import json
import logging
import uuid
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)


def _specs_dir() -> Path:
    """Data directory for spec JSON files."""
    from backend.config import get_data_dir, get_project_root
    data_dir = get_data_dir()
    project_root = get_project_root()
    if data_dir:
        return Path(data_dir) / "project_specs"
    if project_root:
        return Path(project_root) / "Data" / "project_specs"
    return Path(".") / "Data" / "project_specs"


class SpecStore:
    """Async store for ProjectSpec documents."""

    def __init__(self):
        self._mongo_mode = False
        self._collection = None
        self._init_done = False

    async def _init(self) -> None:
        if self._init_done:
            return
        self._init_done = True
        try:
            from backend.config import get_mongodb_uri, get_mongodb_db
            mongo_uri = get_mongodb_uri()
            if mongo_uri:
                import motor.motor_asyncio as motor  # type: ignore
                client = motor.AsyncIOMotorClient(mongo_uri)
                db_name = get_mongodb_db()
                self._collection = client[db_name]["project_specs"]
                self._mongo_mode = True
                logger.debug("SpecStore: using MongoDB")
                return
        except Exception as e:
            logger.debug(f"SpecStore: MongoDB unavailable ({e}), using file store")
        # Fall back to file store
        _specs_dir().mkdir(parents=True, exist_ok=True)

    # -- public API ---------------------------------------------------------

    async def save(self, spec: Any) -> str:
        """Persist a spec and return its spec_id."""
        await self._init()
        spec_id = spec.spec_id or str(uuid.uuid4())
        spec.spec_id = spec_id
        d = spec.to_dict()
        if self._mongo_mode:
            await self._mongo_save(spec_id, d)
        else:
            await self._file_save(spec_id, d)
        return spec_id

    async def load(self, spec_id: str) -> Optional[Any]:
        """Load a spec by spec_id. Returns ProjectSpec or None."""
        await self._init()
        if self._mongo_mode:
            d = await self._mongo_load(spec_id)
        else:
            d = await self._file_load(spec_id)
        if not d:
            return None
        from backend.project_spec.spec_generator import ProjectSpec
        return ProjectSpec.from_dict(d)

    async def update_status(
        self,
        spec_id: str,
        status: str,
        feedback: str = "",
        changed_by: str = "",
    ) -> bool:
        """Update approval status and append an iteration record."""
        spec = await self.load(spec_id)
        if not spec:
            return False
        spec.status = status
        spec.approval["status"] = status
        spec.approval.setdefault("iterations", []).append({
            "changed_at": datetime.utcnow().isoformat() + "Z",
            "changed_by": changed_by or spec.pm_email,
            "summary": feedback[:200] if feedback else status,
        })
        await self.save(spec)
        return True

    async def list_all(self) -> List[Dict[str, Any]]:
        """Return summary list of all stored specs (spec_id, name, status)."""
        await self._init()
        if self._mongo_mode:
            cursor = self._collection.find({}, {"spec_meta": 1, "_id": 0})
            return [doc.get("spec_meta", {}) async for doc in cursor]
        results = []
        for f in _specs_dir().glob("*.json"):
            try:
                d = json.loads(f.read_text())
                results.append(d.get("spec_meta", {}))
            except Exception:
                pass
        return results

    # -- MongoDB backend ----------------------------------------------------

    async def _mongo_save(self, spec_id: str, d: Dict[str, Any]) -> None:
        d["_id"] = spec_id
        await self._collection.replace_one({"_id": spec_id}, d, upsert=True)

    async def _mongo_load(self, spec_id: str) -> Optional[Dict[str, Any]]:
        doc = await self._collection.find_one({"_id": spec_id})
        if doc:
            doc.pop("_id", None)
        return doc

    # -- file-system backend ------------------------------------------------

    async def _file_save(self, spec_id: str, d: Dict[str, Any]) -> None:
        import asyncio
        path = _specs_dir() / f"{spec_id}.json"
        text = json.dumps(d, indent=2, ensure_ascii=False)
        await asyncio.to_thread(path.write_text, text, "utf-8")

    async def _file_load(self, spec_id: str) -> Optional[Dict[str, Any]]:
        import asyncio
        path = _specs_dir() / f"{spec_id}.json"
        if not path.exists():
            return None
        text = await asyncio.to_thread(path.read_text, "utf-8")
        return json.loads(text)
