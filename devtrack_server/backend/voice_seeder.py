"""
VoiceSeeder — Tier 0 voice corpus seeding from git commit history.

Seeds ChromaDB with commit messages from local git repositories so that
personalization (inject_style) has training data from the developer's
own writing even before any manual examples are added.

Non-negotiable patterns:
- Never calls os.getenv directly — all config via backend.config.
- Graceful fallback on git or ChromaDB unavailability (return 0, never raise).
- Idempotent: commits already embedded are skipped (tracked by SQLite hash table).
"""
from __future__ import annotations

import logging
import sqlite3
import subprocess
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

logger = logging.getLogger(__name__)

# ── database helpers ──────────────────────────────────────────────────────────


def _db_path() -> Path:
    """Resolve the SQLite database path via backend.config."""
    try:
        from backend.config import database_path
        return database_path()
    except Exception:
        return Path("Data") / "db" / "devtrack.db"


def _ensure_seed_table(conn: sqlite3.Connection) -> None:
    """Create the voice_seeded_commits tracking table if it does not exist."""
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS voice_seeded_commits (
            hash       TEXT NOT NULL,
            repo_path  TEXT NOT NULL,
            seeded_at  DATETIME NOT NULL DEFAULT (datetime('now')),
            PRIMARY KEY (hash, repo_path)
        )
        """
    )
    conn.commit()


def _is_already_seeded(conn: sqlite3.Connection, commit_hash: str, repo_path: str) -> bool:
    """Return True if this (hash, repo_path) pair is already in the tracking table."""
    row = conn.execute(
        "SELECT 1 FROM voice_seeded_commits WHERE hash = ? AND repo_path = ?",
        (commit_hash, repo_path),
    ).fetchone()
    return row is not None


def _mark_seeded(conn: sqlite3.Connection, commit_hash: str, repo_path: str) -> None:
    """Insert a row into the tracking table to mark a commit as seeded."""
    conn.execute(
        "INSERT OR IGNORE INTO voice_seeded_commits (hash, repo_path, seeded_at) VALUES (?, ?, ?)",
        (commit_hash, repo_path, datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")),
    )
    conn.commit()


# ── VoiceSeeder ───────────────────────────────────────────────────────────────


class VoiceSeeder:
    """Seeds ChromaDB with commit messages from a git repository.

    Designed to be called once at daemon startup (or on demand via
    `devtrack voice seed`) to bootstrap the personalization corpus from
    existing git history, requiring zero developer action.
    """

    def seed_from_git(self, repo_path: str, since_months: int = 6) -> int:
        """Embed recent commit messages from *repo_path* into ChromaDB.

        Args:
            repo_path: Absolute path to the git repository root.
            since_months: How many months of history to mine (default 6).

        Returns:
            Number of newly embedded commit messages.
            Returns 0 when git or ChromaDB is unavailable — never raises.
        """
        try:
            return self._seed(repo_path, since_months)
        except Exception as exc:  # noqa: BLE001
            logger.warning("voice_seeder: unexpected error seeding %s: %s", repo_path, exc)
            return 0

    # ── internal ──────────────────────────────────────────────────────────────

    def _seed(self, repo_path: str, since_months: int) -> int:
        """Internal implementation — callers should use seed_from_git()."""
        # Step 1: Run git log to collect commits.
        commits = self._run_git_log(repo_path, since_months)
        if commits is None:
            return 0  # git unavailable

        # Step 2: Open the SQLite tracking DB.
        try:
            conn = sqlite3.connect(str(_db_path()))
            _ensure_seed_table(conn)
        except Exception as exc:
            logger.warning("voice_seeder: cannot open DB for tracking (%s) — skipping idempotency guard", exc)
            conn = None

        # Step 3: Embed each commit message into ChromaDB.
        embedded = 0
        for commit_hash, message in commits:
            # Skip merge commits — they don't reflect the developer's writing voice.
            if message.startswith("Merge branch") or message.startswith("Merge pull request"):
                continue

            # Idempotency check.
            if conn is not None and _is_already_seeded(conn, commit_hash, repo_path):
                continue

            # Embed into ChromaDB via the existing RAG pipeline.
            success = self._embed_commit(commit_hash, message, repo_path)
            if success:
                if conn is not None:
                    _mark_seeded(conn, commit_hash, repo_path)
                embedded += 1

        if conn is not None:
            conn.close()

        if embedded:
            logger.info("voice_seeder: embedded %d new commit messages from %s", embedded, repo_path)
        return embedded

    def _run_git_log(
        self, repo_path: str, since_months: int
    ) -> Optional[list[tuple[str, str]]]:
        """Run `git log` and return a list of (hash, subject) pairs.

        Returns None when git is unavailable or the repo does not exist.
        """
        since_spec = f"{since_months} months ago"
        try:
            result = subprocess.run(
                [
                    "git",
                    "-C", repo_path,
                    "log",
                    f"--since={since_spec}",
                    "--pretty=format:%H|%s",
                    "--",
                ],
                capture_output=True,
                text=True,
                timeout=30,
            )
        except (FileNotFoundError, subprocess.TimeoutExpired) as exc:
            logger.warning("voice_seeder: git unavailable for %s: %s", repo_path, exc)
            return None
        except Exception as exc:
            logger.warning("voice_seeder: git log failed for %s: %s", repo_path, exc)
            return None

        if result.returncode != 0:
            logger.warning(
                "voice_seeder: git log exited %d for %s: %s",
                result.returncode, repo_path, result.stderr.strip()
            )
            return None

        commits: list[tuple[str, str]] = []
        for line in result.stdout.splitlines():
            line = line.strip()
            if "|" not in line:
                continue
            commit_hash, _, subject = line.partition("|")
            commit_hash = commit_hash.strip()
            subject = subject.strip()
            if commit_hash and subject:
                commits.append((commit_hash, subject))

        return commits

    def _embed_commit(self, commit_hash: str, message: str, repo_path: str) -> bool:
        """Embed a single commit message into ChromaDB.

        Returns True on success, False when ChromaDB or Ollama is unavailable.
        """
        try:
            from backend.rag.embedder import embed as rag_embed
            from backend.rag.vector_store import VectorStore

            text = f"Context: {message}\nResponse: {message}"
            vec = rag_embed(text)
            if vec is None:
                return False  # Ollama / embedding model unavailable

            store = VectorStore()
            metadata = {
                "source": "git_history",
                "context_type": "commit",
                "trigger": message[:300],
                "response": message[:400],
                "repo_path": repo_path[:200],
            }
            return store.upsert(commit_hash, text, vec, metadata)
        except Exception as exc:
            logger.warning("voice_seeder: ChromaDB embed failed for %s: %s", commit_hash, exc)
            return False
