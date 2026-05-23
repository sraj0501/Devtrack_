"""
Global personalization helper.

Loads the user's communication profile once (lazily) and provides style
injection for all LLM prompts throughout the system so every piece of
AI-generated output — commit messages, task descriptions, daily reports,
git-sage summaries, etc. — sounds like the user wrote it.

Two complementary signals are combined:

  1. Profile-based style instruction (from PersonalizedAI)
     Fast, always available once a profile exists.  Captures constraints:
     formality, length, emoji preference, common phrases.

  2. RAG few-shot examples (from backend.rag.SampleIndexer)
     Retrieves the most semantically similar past responses the user wrote.
     Shows the LLM *actual* examples of how the user writes in situations
     like the current one — far more effective than abstract descriptions.

Usage anywhere in the codebase:
    from backend.personalization import inject_style
    prompt = inject_style(original_prompt, context_type="commit")

    # Or with an explicit query text for better RAG retrieval:
    prompt = inject_style(original_prompt, context_type="commit",
                          query_text="fixed null pointer in session handler")

If no profile exists (consent not given / not enough samples) the prompt is
returned unchanged and the system behaves exactly as before.
"""
from __future__ import annotations

import json
import logging
import os
from typing import Optional

logger = logging.getLogger(__name__)

_instance = None   # PersonalizedAI | None
_loaded = False    # True once load has been attempted (avoids repeated retries)


# ── singleton loader ──────────────────────────────────────────────────────────

def _load_personalized_ai():
    """Return singleton PersonalizedAI, or None if unavailable."""
    global _instance, _loaded
    if _loaded:
        return _instance
    _loaded = True

    try:
        from backend.personalized_ai import PersonalizedAI  # noqa: PLC0415

        # Resolve the learning dir (PersonalizedAI expects this, not DATA_DIR)
        learning_data_dir: Optional[str] = None
        try:
            from backend.config import learning_dir  # noqa: PLC0415
            learning_data_dir = str(learning_dir())
        except Exception:
            pass

        email = _read_user_email(learning_data_dir)
        if not email:
            logger.debug("personalization: no user email in consent.json — profile not loaded")
            return None

        # Pass None so PersonalizedAI uses its own config resolver (same result,
        # but avoids any path mismatch if learning_dir() is overridden elsewhere).
        ai = PersonalizedAI(user_email=email, data_dir=learning_data_dir)
        if ai.consent_given and ai.profile:
            _instance = ai
            logger.debug(
                "personalization: profile loaded for %s (%d samples)",
                email, ai.profile.total_samples,
            )
            # Kick off incremental RAG indexing in the background (non-blocking)
            _trigger_rag_index(ai)
        else:
            logger.debug(
                "personalization: consent=%s profile=%s — skipping style injection",
                ai.consent_given, bool(ai.profile),
            )
    except Exception as exc:
        logger.debug("personalization: could not load profile: %s", exc)

    return _instance


def _read_user_email(learning_data_dir: Optional[str]) -> Optional[str]:
    """Read user_email from consent.json inside the learning directory."""
    if not learning_data_dir:
        return None
    consent_file = os.path.join(learning_data_dir, "consent.json")
    if not os.path.exists(consent_file):
        return None
    try:
        with open(consent_file) as fh:
            return json.load(fh).get("user_email")
    except Exception:
        return None


def _trigger_rag_index(ai) -> None:
    """
    Index any samples not yet in ChromaDB.

    Runs synchronously but is cheap if all samples are already indexed
    (is_indexed() check avoids re-embedding).  On first load this may take
    a few seconds for large sample sets — acceptable at daemon startup.
    """
    try:
        from backend.rag import get_indexer  # noqa: PLC0415
        indexer = get_indexer()
        n = indexer.index_samples(ai.samples)
        if n:
            logger.debug("personalization: RAG indexed %d new samples", n)
    except Exception as exc:
        logger.debug("personalization: RAG indexing error: %s", exc)


# ── RAG retrieval ─────────────────────────────────────────────────────────────

def _rag_examples(query_text: str, context_type: str) -> str:
    """Return RAG few-shot block, or "" if unavailable."""
    try:
        from backend.rag import get_indexer  # noqa: PLC0415
        return get_indexer().retrieve_examples(query_text, context_type)
    except Exception as exc:
        logger.debug("personalization: RAG retrieve error: %s", exc)
        return ""


# ── public API ────────────────────────────────────────────────────────────────

def get_style_instruction(context_type: str = "general") -> str:
    """Return a style directive derived from the user's learned profile.

    Returns "" when no profile is available so callers need no special
    handling — an empty string prepended to a prompt changes nothing.

    Args:
        context_type: "commit" | "description" | "report" | "task" |
                      "comment" | "chat" | "email" | "general"
    """
    ai = _load_personalized_ai()
    if not ai:
        return ""
    try:
        return ai.get_style_instruction(context_type)
    except Exception as exc:
        logger.debug("personalization: get_style_instruction error: %s", exc)
        return ""


def inject_style(
    prompt: str,
    context_type: str = "general",
    query_text: Optional[str] = None,
) -> str:
    """Prepend personalization context to *prompt*.

    Combines two signals:
      • Profile-based style instruction  (constraints: formality, length, etc.)
      • RAG few-shot examples            (actual examples of the user's voice)

    When no profile / RAG data exists the original prompt is returned unchanged.

    Args:
        prompt:       The LLM prompt to augment.
        context_type: One of "commit" | "description" | "report" | "task" |
                      "comment" | "chat" | "email" | "general".
        query_text:   Text used as the RAG search query.  If None, the first
                      200 chars of *prompt* are used (works well in practice).
    """
    prefix_parts: list[str] = []

    # 1. Profile-based style instruction
    style = get_style_instruction(context_type)
    if style:
        prefix_parts.append(style)

    # 2. RAG few-shot examples
    q = query_text or prompt[:200]
    examples = _rag_examples(q, context_type)
    if examples:
        prefix_parts.append(examples)

    if not prefix_parts:
        return prompt

    return "\n\n".join(prefix_parts) + "\n\n" + prompt


def reset_cache() -> None:
    """Reset the singleton — useful in tests when the data dir changes."""
    global _instance, _loaded
    _instance = None
    _loaded = False
