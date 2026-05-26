"""
High-level API for indexing communication samples and retrieving few-shot
style examples for prompt injection.

Usage:
    from backend.rag import get_indexer

    indexer = get_indexer()

    # Index all samples (called once when profile loads, incremental thereafter)
    indexer.index_samples(ai.samples)

    # Index a single new sample as it arrives
    indexer.index_sample(sample)

    # Retrieve examples to inject into a prompt
    block = indexer.retrieve_examples(
        query="fixed null pointer in session handler",
        context_type="commit",   # caller's context — mapped to allowed types below
        k=3,
    )
    prompt = block + "\\n\\n" + original_prompt if block else original_prompt
"""
from __future__ import annotations

import logging
from typing import TYPE_CHECKING, Optional

if TYPE_CHECKING:
    from backend.personalized_ai import CommunicationSample

logger = logging.getLogger(__name__)

# ── context_type mapping ──────────────────────────────────────────────────────
# Maps the caller's coarse context to the stored context_type values that are
# most relevant for retrieval (fetch from these bucket(s)).

_CTX_ALLOW: dict[str, list[str]] = {
    "commit":      ["comment", "chat"],
    "description": ["comment", "chat", "meeting"],
    "report":      ["email", "meeting", "chat"],
    "task":        ["comment", "email", "meeting"],
    "comment":     ["comment", "chat"],
    "chat":        ["chat"],
    "email":       ["email"],
    "general":     [],   # empty = no filter = all types
}

# ── rag enabled check ─────────────────────────────────────────────────────────

def _rag_enabled() -> bool:
    try:
        from backend.config import get_bool
        return get_bool("PERSONALIZATION_RAG_ENABLED", True)
    except Exception:
        return True


def _default_k() -> int:
    try:
        from backend.config import get_int
        return get_int("PERSONALIZATION_RAG_K", 3) or 3
    except Exception:
        return 3


# ── singleton ─────────────────────────────────────────────────────────────────

_indexer_instance: Optional["SampleIndexer"] = None


def get_indexer() -> "SampleIndexer":
    global _indexer_instance
    if _indexer_instance is None:
        _indexer_instance = SampleIndexer()
    return _indexer_instance


def reset_indexer() -> None:
    """Reset the singleton — useful in tests."""
    global _indexer_instance
    _indexer_instance = None


# ── main class ────────────────────────────────────────────────────────────────

class SampleIndexer:
    """Indexes CommunicationSamples into ChromaDB and retrieves few-shot examples."""

    def __init__(self) -> None:
        from .vector_store import VectorStore
        self._store = VectorStore()

    # ── indexing ──────────────────────────────────────────────────────────────

    def index_sample(self, sample: "CommunicationSample") -> bool:
        """
        Embed and upsert a single sample.  Skips if already indexed.
        Returns True on success, False if embedding or store unavailable.
        """
        if not _rag_enabled():
            return False
        if self._store.is_indexed(sample.id):
            return True  # already up to date

        from .embedder import embed  # noqa: PLC0415

        text = _sample_text(sample)
        vec = embed(text)
        if vec is None:
            return False  # Ollama / model unavailable

        metadata = {
            "source": sample.source,
            "context_type": sample.context_type,
            "trigger": sample.trigger[:300],
            "response": sample.response[:400],
        }
        return self._store.upsert(sample.id, text, vec, metadata)

    def index_samples(self, samples: list["CommunicationSample"]) -> int:
        """
        Bulk-index a list of samples.  Skips already-indexed ones.
        Returns the count of newly indexed samples.
        """
        if not _rag_enabled():
            return 0
        indexed = 0
        for sample in samples:
            if self.index_sample(sample):
                indexed += 1
        if indexed:
            logger.debug("RAG indexer: indexed %d new samples (%d total)", indexed, self._store.count())
        return indexed

    # ── retrieval ─────────────────────────────────────────────────────────────

    def retrieve_examples(
        self,
        query_text: str,
        context_type: str = "general",
        k: Optional[int] = None,
    ) -> str:
        """
        Return a formatted block of few-shot examples for prompt injection, or
        "" if RAG is unavailable / no relevant examples found.

        The block looks like:
            Here are real examples of how this user has written in similar situations
            (use these for voice and style only — do NOT copy the content):
            [1] (chat) "sorted — the cache wasn't being invalidated, added a flush"
            [2] (comment) "looks good, one nit: null check should come before the write"
        """
        if not _rag_enabled():
            return ""
        if self._store.count() == 0:
            return ""

        k = k or _default_k()

        from .embedder import embed  # noqa: PLC0415

        vec = embed(query_text)
        if vec is None:
            return ""

        allowed_types = _CTX_ALLOW.get(context_type, [])
        results = self._store.query(vec, context_types=allowed_types or None, k=k)
        if not results:
            return ""

        lines = [
            "Here are real examples of how this user has written in similar situations"
            " (use these for voice and style only — do NOT copy the content):"
        ]
        for i, r in enumerate(results, 1):
            ctx = r.get("context_type", "")
            resp = r.get("response", "").strip()
            if resp:
                lines.append(f'[{i}] ({ctx}) "{resp}"')

        return "\n".join(lines) if len(lines) > 1 else ""

    # ── stats ─────────────────────────────────────────────────────────────────

    def indexed_count(self) -> int:
        return self._store.count()

    def delete_all(self) -> None:
        """Wipe the vector store — called when user revokes consent."""
        self._store.delete_all()


# ── helpers ───────────────────────────────────────────────────────────────────

def _sample_text(sample: "CommunicationSample") -> str:
    """Produce the text we embed for a sample (trigger + response gives richer semantics)."""
    return f"Context: {sample.trigger}\nResponse: {sample.response}"
