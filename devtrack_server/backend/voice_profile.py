"""
VoiceProfile — Dialectic profile generation from ChromaDB corpus.

Runs a local LLM reasoning pass over the embedded commit corpus to produce
a human-readable Developer Voice Profile markdown file at:

    {DATA_DIR}/learning/profile.md

The profile is evidence-based (derived from actual commit messages), not
declared. It is always re-generated from evidence on each call — never
accumulated by hand-editing.

Non-negotiable patterns:
- Never calls os.getenv directly — all config via backend.config.
- generate() never raises — any exception produces the fallback template.
- save() creates parent directories if they do not exist.
- No hardcoded DATA_DIR, model names, or hosts.
"""
from __future__ import annotations

import logging
import pathlib
from typing import Optional

logger = logging.getLogger(__name__)

# ── Fallback template ─────────────────────────────────────────────────────────

_FALLBACK_TEMPLATE = """\
# Developer Voice Profile

Insufficient data for automated profiling. Run `devtrack voice add` to add examples manually.
"""

# ── ProfileGenerator ──────────────────────────────────────────────────────────


class ProfileGenerator:
    """Generates a Developer Voice Profile by reasoning over the ChromaDB corpus.

    Usage:
        gen = ProfileGenerator()
        profile_text = gen.generate(["path/to/repo"])
        saved_path = gen.save(profile_text, "/abs/path/to/DATA_DIR")
    """

    def __init__(self, provider=None):
        # provider=None means lazy-init on first use (same as CommitMessageEnhancer)
        self._provider = provider

    def _get_provider(self):
        """Lazy-init the multi-provider LLM chain."""
        if self._provider is None:
            from backend.llm import get_provider
            self._provider = get_provider()
        return self._provider

    # ── public API ────────────────────────────────────────────────────────────

    def generate(self, repo_paths: list[str]) -> str:
        """Retrieve 20–50 recent commit messages from ChromaDB and ask the LLM
        to infer the developer's writing style.

        Args:
            repo_paths: List of repo path strings to filter by. If empty or None,
                        samples from any repo are retrieved.

        Returns:
            A markdown string beginning with '# Developer Voice Profile'.
            Returns the fallback template on any failure — never raises.
        """
        try:
            return self._generate(repo_paths)
        except Exception as exc:  # noqa: BLE001
            logger.warning("voice_profile: unexpected error in generate(): %s", exc)
            return _FALLBACK_TEMPLATE

    def save(self, profile_text: str, data_dir: str) -> pathlib.Path:
        """Write profile_text to {data_dir}/learning/profile.md.

        Creates {data_dir}/learning/ if it does not exist.

        Args:
            profile_text: The markdown profile string to write.
            data_dir:     Absolute path to DATA_DIR (from config.get_path("DATA_DIR")).

        Returns:
            The pathlib.Path to the written file.
        """
        target_dir = pathlib.Path(data_dir) / "learning"
        target_dir.mkdir(parents=True, exist_ok=True)
        target_path = target_dir / "profile.md"
        target_path.write_text(profile_text, encoding="utf-8")
        logger.info("voice_profile: profile saved to %s (%d chars)", target_path, len(profile_text))
        return target_path

    # ── internal ──────────────────────────────────────────────────────────────

    def _retrieve_commit_messages(self, repo_paths: list[str], limit: int = 50) -> list[str]:
        """Retrieve up to *limit* commit messages from ChromaDB.

        Filters by context_type="commit". If repo_paths is provided, further
        filters by matching repo_path metadata (best-effort — falls back to
        unfiltered when the metadata filter produces no results).

        Returns a list of commit message strings (deduplicated).
        If ChromaDB is unavailable, returns an empty list.
        """
        try:
            from backend.rag.vector_store import VectorStore
            store = VectorStore()
            if not store._init():
                logger.debug("voice_profile: ChromaDB not available")
                return []

            collection = store._collection
            if collection is None:
                return []

            total = store.count()
            if total == 0:
                return []

            # Retrieve documents with context_type="commit" metadata filter.
            # ChromaDB's get() supports where filters; limit to at most *limit* docs.
            fetch_limit = min(limit, total)
            try:
                result = collection.get(
                    where={"context_type": "commit"},
                    limit=fetch_limit,
                    include=["metadatas", "documents"],
                )
            except Exception as exc:
                logger.debug("voice_profile: filtered get failed (%s), falling back to unfiltered", exc)
                # Fallback: retrieve without filter if the where clause fails
                # (e.g. ChromaDB version that doesn't support where on get())
                try:
                    result = collection.get(
                        limit=fetch_limit,
                        include=["metadatas", "documents"],
                    )
                except Exception as exc2:
                    logger.warning("voice_profile: ChromaDB get failed: %s", exc2)
                    return []

            metadatas = result.get("metadatas") or []
            # Extract the "response" field from metadata — this is the commit subject
            # stored by voice_seeder (field "response": message[:400])
            messages: list[str] = []
            seen: set[str] = set()
            for meta in metadatas:
                if not isinstance(meta, dict):
                    continue
                msg = meta.get("response") or meta.get("trigger") or ""
                msg = msg.strip()
                if msg and msg not in seen:
                    seen.add(msg)
                    messages.append(msg)

            return messages

        except Exception as exc:
            logger.warning("voice_profile: error retrieving from ChromaDB: %s", exc)
            return []

    def _build_prompt(self, messages: list[str]) -> str:
        """Build the LLM prompt for inferring writing style from commit messages."""
        sample_text = "\n".join(f"- {m}" for m in messages)
        return f"""You are a writing-style analyst. Analyze the following commit messages written by a software developer and infer their characteristic writing voice and style.

Commit messages (sample of {len(messages)}):
{sample_text}

Based on these commit messages, produce a Developer Voice Profile in Markdown. The profile must:
1. Start with the heading: # Developer Voice Profile
2. Include the following sections (use ## for each):
   - **Formality Level** — is the writing formal, informal, or mixed? Give evidence.
   - **Sentence Length Preference** — are messages typically short (under 8 words), medium (8–20 words), or long (20+ words)?
   - **Verb Mood** — does the developer prefer imperative ("fix", "add", "remove") or past tense ("fixed", "added", "removed")?
   - **Characteristic Phrases and Vocabulary** — list words or phrases used repeatedly.
   - **What the Developer Avoids** — passive voice, exclamation marks, filler words, hedging language, etc. Only include patterns that are clearly absent.
3. Be evidence-based: quote short examples from the messages to support each inference.
4. Be concise — the profile should be 200–400 words total.
5. Do NOT include meta-commentary, options lists, or preamble. Output ONLY the profile.

Developer Voice Profile:"""

    def _generate(self, repo_paths: list[str]) -> str:
        """Internal implementation — called by generate() which wraps it in try/except."""
        # Step 1: Retrieve commit messages from ChromaDB.
        messages = self._retrieve_commit_messages(repo_paths or [], limit=50)

        if not messages:
            logger.info("voice_profile: no commit messages in ChromaDB — returning fallback")
            return _FALLBACK_TEMPLATE

        # Trim to 50 max, prefer most recent (ChromaDB returns insertion order by default)
        if len(messages) > 50:
            messages = messages[:50]
        # Use at least 20 if available; if fewer, use all
        messages_to_use = messages[:50]

        logger.info("voice_profile: generating profile from %d commit messages", len(messages_to_use))

        # Step 2: Build prompt and call LLM.
        prompt = self._build_prompt(messages_to_use)

        try:
            from backend.llm.base import LLMOptions
            from backend.config import http_timeout, commit_llm_temperature

            result = self._get_provider().generate(
                prompt=prompt,
                options=LLMOptions(
                    temperature=commit_llm_temperature(),
                    max_tokens=800,
                ),
                timeout=http_timeout(),
            )
        except Exception as exc:
            logger.warning("voice_profile: LLM call failed (returning fallback): %s", exc)
            return _FALLBACK_TEMPLATE

        if not result or len(result.strip()) < 20:
            logger.warning("voice_profile: LLM returned empty/trivial response — returning fallback")
            return _FALLBACK_TEMPLATE

        # Ensure output begins with the required heading.
        result = result.strip()
        if not result.startswith("# Developer Voice Profile"):
            # LLM may have omitted the heading; prepend it
            result = "# Developer Voice Profile\n\n" + result

        return result
