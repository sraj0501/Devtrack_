"""
ChromaDB persistent vector store for communication samples.

One collection — "communication_samples" — holds every sample the user
has consented to collect.  Each document is:

  text     : "Context: {trigger}\\nResponse: {response}"
  embedding: nomic-embed-text vector via Ollama
  metadata : source, context_type, trigger (first 200 chars), response (first 400 chars)
  id       : sample.id  (used for deduplication on upsert)

Querying is done by cosine similarity.  An optional context_type filter
narrows results to semantically related communication modes.
"""
from __future__ import annotations

import logging
import os
from pathlib import Path
from typing import Optional

logger = logging.getLogger(__name__)

COLLECTION_NAME = "communication_samples"


def _chroma_dir() -> Path:
    """Resolve the ChromaDB persistence directory."""
    try:
        from backend.config import get, get_path, learning_dir
        custom = get("PERSONALIZATION_CHROMA_DIR")
        if custom:
            return get_path("PERSONALIZATION_CHROMA_DIR")
        return learning_dir() / "chroma"
    except Exception:
        return Path(os.getcwd()) / "Data" / "learning" / "chroma"


class VectorStore:
    """
    Thin wrapper around a single ChromaDB persistent collection.

    Lazy-initialises on first use so importing this module never fails even
    if chromadb is not installed.
    """

    def __init__(self, persist_dir: Optional[Path] = None):
        self._persist_dir = persist_dir or _chroma_dir()
        self._client = None
        self._collection = None

    def _init(self) -> bool:
        """Initialise ChromaDB client + collection.  Returns False on failure."""
        if self._collection is not None:
            return True
        try:
            import chromadb  # noqa: PLC0415

            self._persist_dir.mkdir(parents=True, exist_ok=True)
            self._client = chromadb.PersistentClient(path=str(self._persist_dir))
            self._collection = self._client.get_or_create_collection(
                name=COLLECTION_NAME,
                metadata={"hnsw:space": "cosine"},
            )
            logger.debug("RAG vector_store: initialised at %s", self._persist_dir)
            return True
        except ImportError:
            logger.debug("RAG vector_store: chromadb not installed — RAG disabled")
            return False
        except Exception as exc:
            logger.debug("RAG vector_store: init error: %s", exc)
            return False

    # ── write ─────────────────────────────────────────────────────────────────

    def upsert(
        self,
        sample_id: str,
        text: str,
        embedding: list[float],
        metadata: dict,
    ) -> bool:
        """Add or update a sample in the collection."""
        if not self._init():
            return False
        try:
            # Truncate large metadata strings (ChromaDB limits metadata values)
            safe_meta = {
                k: str(v)[:500] if isinstance(v, str) else v
                for k, v in metadata.items()
            }
            self._collection.upsert(
                ids=[sample_id],
                embeddings=[embedding],
                documents=[text],
                metadatas=[safe_meta],
            )
            return True
        except Exception as exc:
            logger.debug("RAG vector_store: upsert error: %s", exc)
            return False

    # ── read ──────────────────────────────────────────────────────────────────

    def query(
        self,
        query_embedding: list[float],
        context_types: Optional[list[str]] = None,
        k: int = 3,
    ) -> list[dict]:
        """
        Return up to *k* most similar samples.

        Each result dict has keys: trigger, response, context_type, source, distance.

        *context_types* is an optional allow-list for the metadata filter
        (e.g. ["chat", "comment"] for commit-message context).
        """
        if not self._init():
            return []
        try:
            where = None
            if context_types and len(context_types) == 1:
                where = {"context_type": context_types[0]}
            elif context_types and len(context_types) > 1:
                where = {"context_type": {"$in": context_types}}

            kwargs: dict = {
                "query_embeddings": [query_embedding],
                "n_results": min(k, max(self.count(), 1)),
            }
            if where:
                kwargs["where"] = where

            results = self._collection.query(**kwargs)

            out = []
            metadatas = results.get("metadatas", [[]])[0]
            distances = results.get("distances", [[]])[0]
            for meta, dist in zip(metadatas, distances):
                out.append({
                    "trigger": meta.get("trigger", ""),
                    "response": meta.get("response", ""),
                    "context_type": meta.get("context_type", ""),
                    "source": meta.get("source", ""),
                    "distance": dist,
                })
            return out
        except Exception as exc:
            logger.debug("RAG vector_store: query error: %s", exc)
            return []

    def is_indexed(self, sample_id: str) -> bool:
        """Return True if *sample_id* is already in the collection."""
        if not self._init():
            return False
        try:
            result = self._collection.get(ids=[sample_id], include=[])
            return bool(result.get("ids"))
        except Exception:
            return False

    def count(self) -> int:
        """Number of indexed samples."""
        if not self._init():
            return 0
        try:
            return self._collection.count()
        except Exception:
            return 0

    def delete_all(self) -> None:
        """Wipe the entire collection (used when user revokes consent)."""
        if not self._init():
            return
        try:
            self._client.delete_collection(COLLECTION_NAME)
            self._collection = None
            logger.info("RAG vector_store: collection deleted")
        except Exception as exc:
            logger.debug("RAG vector_store: delete_all error: %s", exc)
