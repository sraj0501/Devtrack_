"""
VoiceSync — Tier 1 voice corpus enrichment from PM platform content.

Polls GitHub, GitLab, and Azure DevOps for PR descriptions and issue comments
authored by the developer (filtered by pm_username from workspaces.yaml) and
embeds them into ChromaDB. This enriches the voice corpus with more formal
written communication beyond commit messages.

Non-negotiable patterns:
- Never calls os.getenv directly — all config via backend.config.
- Graceful per-platform failure: one platform failing never blocks others.
- Idempotent: already-embedded items are skipped (tracked via
  backend.db.voice_sync_store's Python-owned voice_synced_items table,
  dual-dialect through backend.db.engine).
- context_type="description" for PR/MR bodies.
- context_type="comment" for issue/work-item comments.
"""
from __future__ import annotations

import asyncio
import logging
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)


# ── ChromaDB embedding helper ─────────────────────────────────────────────────


def _embed_text(doc_id: str, text: str, context_type: str, source: str) -> bool:
    """Embed a single text document into ChromaDB.

    Returns True on success, False when ChromaDB or the embedding model is
    unavailable (graceful degradation).
    """
    try:
        from backend.rag.embedder import embed as rag_embed
        from backend.rag.vector_store import VectorStore

        formatted = f"Context: {text}\nResponse: {text}"
        vec = rag_embed(formatted)
        if vec is None:
            return False  # embedding model unavailable

        store = VectorStore()
        metadata = {
            "source": source,
            "context_type": context_type,
            "trigger": text[:300],
            "response": text[:400],
        }
        return store.upsert(doc_id, formatted, vec, metadata)
    except Exception as exc:
        logger.warning("voice_sync: ChromaDB embed failed for %s: %s", doc_id, exc)
        return False


# ── VoiceSync ─────────────────────────────────────────────────────────────────


class VoiceSync:
    """Polls PM platforms for developer-authored PR descriptions and issue comments,
    then embeds them into ChromaDB to enrich the voice corpus (Tier 1, Phase 5).

    Designed to run on a daily cron schedule or on demand via
    `devtrack voice sync`. Each method is self-contained and never raises to its
    caller — all failures are logged at WARNING level.
    """

    def sync_pr_descriptions(self, workspace: Any) -> int:
        """Fetch PRs/MRs authored by the developer from the workspace PM platform.

        Embeds PR body text into ChromaDB with context_type="description".
        Filters to items authored by workspace.pm_username only.
        Idempotent: already-embedded PR IDs are skipped.

        Args:
            workspace: A WorkspaceConfig object (has pm_platform, pm_username,
                       pm_org, pm_api_url, etc.).

        Returns:
            Number of newly embedded PR descriptions.
        """
        try:
            return self._sync_prs(workspace)
        except Exception as exc:
            logger.warning(
                "voice_sync: unexpected error in sync_pr_descriptions for workspace %s: %s",
                getattr(workspace, "name", "?"), exc,
            )
            return 0

    def sync_issue_comments(self, workspace: Any) -> int:
        """Fetch issue/ticket comments authored by the developer.

        Embeds comment text into ChromaDB with context_type="comment".
        Filters to items authored by workspace.pm_username only.
        Idempotent: already-embedded comment IDs are skipped.

        Args:
            workspace: A WorkspaceConfig object.

        Returns:
            Number of newly embedded issue comments.
        """
        try:
            return self._sync_comments(workspace)
        except Exception as exc:
            logger.warning(
                "voice_sync: unexpected error in sync_issue_comments for workspace %s: %s",
                getattr(workspace, "name", "?"), exc,
            )
            return 0

    def sync_workspace(self, workspace: Any) -> Dict[str, int]:
        """Run both sync_pr_descriptions and sync_issue_comments for one workspace.

        Never raises — per-method failures are already caught and logged.

        Returns:
            {"prs": N, "comments": N}
        """
        prs = self.sync_pr_descriptions(workspace)
        comments = self.sync_issue_comments(workspace)
        return {"prs": prs, "comments": comments}

    def sync_all(self, workspaces: List[Any]) -> Dict[str, int]:
        """Run sync_workspace for all workspaces.

        Accumulates per-platform counts. A failure on one platform does not
        block others.

        Returns:
            {"github": N, "azure": N, "gitlab": N, "total": N}
        """
        totals: Dict[str, int] = {"github": 0, "azure": 0, "gitlab": 0}

        for ws in workspaces:
            platform = getattr(ws, "pm_platform", "").lower()
            try:
                result = self.sync_workspace(ws)
                count = result.get("prs", 0) + result.get("comments", 0)
                if platform in totals:
                    totals[platform] += count
                # Workspaces with unknown/unconfigured platforms contribute to
                # nothing — their counts are safely discarded.
            except Exception as exc:
                logger.warning(
                    "voice_sync: sync_workspace failed for %s (%s): %s",
                    getattr(ws, "name", "?"), platform, exc,
                )

        totals["total"] = sum(v for k, v in totals.items() if k != "total")
        return totals

    # ── internal ──────────────────────────────────────────────────────────────

    def _sync_prs(self, workspace: Any) -> int:
        """Internal: fetch and embed PR descriptions for one workspace."""
        from backend.db.voice_sync_store import is_already_synced, mark_synced

        platform = getattr(workspace, "pm_platform", "").lower()
        pm_username = getattr(workspace, "pm_username", "")
        ws_name = getattr(workspace, "name", "?")

        if not pm_username:
            logger.warning(
                "voice_sync: workspace %s has no pm_username — skipping PR sync", ws_name
            )
            return 0

        embedded = 0

        try:
            prs = asyncio.run(self._fetch_prs(workspace))
        except Exception as exc:
            logger.warning(
                "voice_sync: failed to fetch PRs for workspace %s (%s): %s",
                ws_name, platform, exc,
            )
            return 0

        for pr in prs:
            pr_id = str(pr.get("id", ""))
            author = pr.get("author", "")
            body = pr.get("body", "") or ""

            # Author filter: skip PRs not authored by the developer.
            if author.lower() != pm_username.lower():
                continue

            if not body.strip():
                continue

            doc_id = f"{platform}-pr-{pr_id}"

            # Idempotency check.
            if is_already_synced(platform, doc_id, "description"):
                continue

            success = _embed_text(doc_id, body, "description", "pr_sync")
            if success:
                mark_synced(platform, doc_id, "description")
                embedded += 1

        if embedded:
            logger.info(
                "voice_sync: embedded %d new PR descriptions from %s (%s)",
                embedded, ws_name, platform,
            )
        return embedded

    def _sync_comments(self, workspace: Any) -> int:
        """Internal: fetch and embed issue comments for one workspace."""
        from backend.db.voice_sync_store import is_already_synced, mark_synced

        platform = getattr(workspace, "pm_platform", "").lower()
        pm_username = getattr(workspace, "pm_username", "")
        ws_name = getattr(workspace, "name", "?")

        if not pm_username:
            logger.warning(
                "voice_sync: workspace %s has no pm_username — skipping comment sync", ws_name
            )
            return 0

        embedded = 0

        try:
            comments = asyncio.run(self._fetch_comments(workspace))
        except Exception as exc:
            logger.warning(
                "voice_sync: failed to fetch comments for workspace %s (%s): %s",
                ws_name, platform, exc,
            )
            return 0

        for comment in comments:
            comment_id = str(comment.get("id", ""))
            author = comment.get("author", "")
            body = comment.get("body", "") or ""

            # Author filter: skip comments not authored by the developer.
            if author.lower() != pm_username.lower():
                continue

            if not body.strip():
                continue

            doc_id = f"{platform}-comment-{comment_id}"

            # Idempotency check.
            if is_already_synced(platform, doc_id, "comment"):
                continue

            success = _embed_text(doc_id, body, "comment", "pr_sync")
            if success:
                mark_synced(platform, doc_id, "comment")
                embedded += 1

        if embedded:
            logger.info(
                "voice_sync: embedded %d new issue comments from %s (%s)",
                embedded, ws_name, platform,
            )
        return embedded

    # ── async PM platform fetchers ────────────────────────────────────────────

    async def _fetch_prs(self, workspace: Any) -> List[Dict[str, Any]]:
        """Fetch PR/MR list from the appropriate PM platform.

        Returns a list of dicts with keys: id, author, body.
        """
        platform = getattr(workspace, "pm_platform", "").lower()

        if platform == "github":
            return await self._fetch_github_prs(workspace)
        elif platform == "azure":
            return await self._fetch_azure_prs(workspace)
        elif platform == "gitlab":
            return await self._fetch_gitlab_prs(workspace)
        else:
            logger.warning("voice_sync: unsupported platform %r — skipping PR fetch", platform)
            return []

    async def _fetch_comments(self, workspace: Any) -> List[Dict[str, Any]]:
        """Fetch issue/ticket comments from the appropriate PM platform.

        Returns a list of dicts with keys: id, author, body.
        """
        platform = getattr(workspace, "pm_platform", "").lower()

        if platform == "github":
            return await self._fetch_github_comments(workspace)
        elif platform == "azure":
            return await self._fetch_azure_comments(workspace)
        elif platform == "gitlab":
            return await self._fetch_gitlab_comments(workspace)
        else:
            logger.warning(
                "voice_sync: unsupported platform %r — skipping comment fetch", platform
            )
            return []

    # ── GitHub ────────────────────────────────────────────────────────────────

    async def _fetch_github_prs(self, workspace: Any) -> List[Dict[str, Any]]:
        """Fetch closed+open PRs from GitHub, returning {id, author, body} dicts."""
        try:
            from backend.config import get
            token = get("GITHUB_TOKEN", "")
            org = getattr(workspace, "pm_org", "") or get("GITHUB_OWNER", "")
            repo = getattr(workspace, "pm_project", "") or get("GITHUB_REPO", "")

            if not token or not org or not repo:
                logger.warning("voice_sync: GitHub not fully configured — skipping PR fetch")
                return []

            from backend.github.client import GitHubClient
            client = GitHubClient(token=token, owner=org, repo=repo)

            results: List[Dict[str, Any]] = []
            for state in ("open", "closed"):
                url: Optional[str] = client._api(
                    f"/repos/{org}/{repo}/pulls"
                )
                params: Dict[str, Any] = {
                    "state": state,
                    "per_page": 100,
                }
                while url:
                    data, headers = await client._get_with_headers(url, params=params)
                    if not isinstance(data, list):
                        break
                    for pr in data:
                        user = pr.get("user", {}) or {}
                        results.append({
                            "id": str(pr.get("number", "")),
                            "author": user.get("login", ""),
                            "body": pr.get("body", "") or "",
                        })
                    url = client._parse_next_link(headers.get("Link", ""))
                    params = {}

            await client.close()
            return results
        except Exception as exc:
            logger.warning("voice_sync: GitHub PR fetch failed: %s", exc)
            return []

    async def _fetch_github_comments(self, workspace: Any) -> List[Dict[str, Any]]:
        """Fetch issue comments from GitHub, returning {id, author, body} dicts."""
        try:
            from backend.config import get
            token = get("GITHUB_TOKEN", "")
            org = getattr(workspace, "pm_org", "") or get("GITHUB_OWNER", "")
            repo = getattr(workspace, "pm_project", "") or get("GITHUB_REPO", "")

            if not token or not org or not repo:
                logger.warning(
                    "voice_sync: GitHub not fully configured — skipping comment fetch"
                )
                return []

            from backend.github.client import GitHubClient
            client = GitHubClient(token=token, owner=org, repo=repo)

            results: List[Dict[str, Any]] = []
            url: Optional[str] = client._api(f"/repos/{org}/{repo}/issues/comments")
            params: Dict[str, Any] = {"per_page": 100}
            while url:
                data, headers = await client._get_with_headers(url, params=params)
                if not isinstance(data, list):
                    break
                for comment in data:
                    user = comment.get("user", {}) or {}
                    results.append({
                        "id": str(comment.get("id", "")),
                        "author": user.get("login", ""),
                        "body": comment.get("body", "") or "",
                    })
                url = client._parse_next_link(headers.get("Link", ""))
                params = {}

            await client.close()
            return results
        except Exception as exc:
            logger.warning("voice_sync: GitHub comment fetch failed: %s", exc)
            return []

    # ── Azure DevOps ──────────────────────────────────────────────────────────

    async def _fetch_azure_prs(self, workspace: Any) -> List[Dict[str, Any]]:
        """Fetch PR descriptions from Azure DevOps, returning {id, author, body} dicts."""
        try:
            from backend.config import get
            pat = get("AZURE_DEVOPS_PAT", "") or get("AZURE_API_KEY", "")
            org = getattr(workspace, "pm_org", "") or get("AZURE_ORGANIZATION", "")
            project = getattr(workspace, "pm_project", "") or get("AZURE_PROJECT", "")

            if not pat or not org:
                logger.warning(
                    "voice_sync: Azure DevOps not fully configured — skipping PR fetch"
                )
                return []

            from backend.azure.client import AzureDevOpsClient
            client = AzureDevOpsClient(org=org, project=project, pat=pat)

            # Azure DevOps: GET _apis/git/repositories/{project}/pullrequests
            base = f"https://dev.azure.com/{org}/{project}/_apis/git/pullrequests"
            session = await client._get_session()

            results: List[Dict[str, Any]] = []
            import aiohttp
            for status in ("completed", "active"):
                url: Optional[str] = base
                params: Dict[str, Any] = {
                    "api-version": client._api_version,
                    "status": status,
                    "$top": 100,
                    "$skip": 0,
                }
                while url:
                    try:
                        async with session.get(url, params=params) as resp:
                            if resp.status != 200:
                                break
                            data = await resp.json()
                    except Exception:
                        break

                    items = data.get("value", [])
                    if not items:
                        break

                    for pr in items:
                        created_by = pr.get("createdBy", {}) or {}
                        results.append({
                            "id": str(pr.get("pullRequestId", "")),
                            "author": created_by.get("uniqueName", "")
                                      or created_by.get("displayName", ""),
                            "body": pr.get("description", "") or "",
                        })

                    # Azure pagination: advance $skip
                    if len(items) < 100:
                        break
                    params["$skip"] = params.get("$skip", 0) + 100

            await client.close()
            return results
        except Exception as exc:
            logger.warning("voice_sync: Azure PR fetch failed: %s", exc)
            return []

    async def _fetch_azure_comments(self, workspace: Any) -> List[Dict[str, Any]]:
        """Fetch work-item comments from Azure DevOps, returning {id, author, body} dicts."""
        try:
            from backend.config import get
            pat = get("AZURE_DEVOPS_PAT", "") or get("AZURE_API_KEY", "")
            org = getattr(workspace, "pm_org", "") or get("AZURE_ORGANIZATION", "")
            project = getattr(workspace, "pm_project", "") or get("AZURE_PROJECT", "")

            if not pat or not org:
                logger.warning(
                    "voice_sync: Azure DevOps not fully configured — skipping comment fetch"
                )
                return []

            from backend.azure.client import AzureDevOpsClient
            client = AzureDevOpsClient(org=org, project=project, pat=pat)

            # Fetch assigned work items, then fetch comments on each.
            pm_username = getattr(workspace, "pm_username", "")
            work_items = await client.get_work_items_for_user(
                pm_username, max_results=50
            ) if pm_username else await client.get_my_work_items(max_results=50)

            session = await client._get_session()
            results: List[Dict[str, Any]] = []

            for wi in work_items[:20]:  # limit to most recent 20 items for speed
                url = (
                    f"https://dev.azure.com/{org}/{project}/_apis/"
                    f"wit/workItems/{wi.id}/comments"
                )
                try:
                    async with session.get(
                        url, params={"api-version": "7.1-preview.3"}
                    ) as resp:
                        if resp.status != 200:
                            continue
                        data = await resp.json()
                except Exception:
                    continue

                for comment in data.get("comments", []):
                    created_by = comment.get("createdBy", {}) or {}
                    results.append({
                        "id": f"{wi.id}-{comment.get('id', '')}",
                        "author": created_by.get("uniqueName", "")
                                  or created_by.get("displayName", ""),
                        "body": comment.get("text", "") or "",
                    })

            await client.close()
            return results
        except Exception as exc:
            logger.warning("voice_sync: Azure comment fetch failed: %s", exc)
            return []

    # ── GitLab ────────────────────────────────────────────────────────────────

    async def _fetch_gitlab_prs(self, workspace: Any) -> List[Dict[str, Any]]:
        """Fetch MR descriptions from GitLab, returning {id, author, body} dicts."""
        try:
            from backend.config import get
            pat = get("GITLAB_PAT", "") or get("GITLAB_API_KEY", "")
            base_url = get("GITLAB_URL", "https://gitlab.com")
            project_id = getattr(workspace, "pm_project", "") or get("GITLAB_PROJECT_ID", "")

            if not pat:
                logger.warning(
                    "voice_sync: GitLab not configured (no GITLAB_PAT) — skipping MR fetch"
                )
                return []

            from backend.gitlab.client import GitLabClient
            client = GitLabClient(
                base_url=base_url,
                pat=pat,
                project_id=int(project_id) if project_id else None,
            )
            if not client._project_id:
                logger.warning("voice_sync: no GITLAB_PROJECT_ID configured — skipping MR fetch")
                await client.close()
                return []

            session = await client._get_session()
            results: List[Dict[str, Any]] = []

            for state in ("opened", "merged", "closed"):
                url: Optional[str] = (
                    f"{base_url.rstrip('/')}/api/v4/"
                    f"projects/{client._project_id}/merge_requests"
                )
                params: Dict[str, Any] = {
                    "state": state,
                    "per_page": 100,
                    "page": 1,
                }
                while url:
                    try:
                        async with session.get(url, params=params) as resp:
                            if resp.status != 200:
                                break
                            data = await resp.json()
                            link_header = resp.headers.get("Link", "")
                    except Exception:
                        break

                    if not isinstance(data, list) or not data:
                        break

                    for mr in data:
                        author_info = mr.get("author", {}) or {}
                        results.append({
                            "id": str(mr.get("iid", mr.get("id", ""))),
                            "author": author_info.get("username", ""),
                            "body": mr.get("description", "") or "",
                        })

                    # GitLab uses X-Next-Page / Link header for pagination
                    from backend.github.client import GitHubClient as _GH
                    next_url = _GH._parse_next_link(link_header)
                    if next_url:
                        url = next_url
                        params = {}
                    else:
                        break

            await client.close()
            return results
        except Exception as exc:
            logger.warning("voice_sync: GitLab MR fetch failed: %s", exc)
            return []

    async def _fetch_gitlab_comments(self, workspace: Any) -> List[Dict[str, Any]]:
        """Fetch issue comments from GitLab, returning {id, author, body} dicts."""
        try:
            from backend.config import get
            pat = get("GITLAB_PAT", "") or get("GITLAB_API_KEY", "")
            base_url = get("GITLAB_URL", "https://gitlab.com")
            project_id = getattr(workspace, "pm_project", "") or get("GITLAB_PROJECT_ID", "")

            if not pat:
                logger.warning(
                    "voice_sync: GitLab not configured (no GITLAB_PAT) — skipping comment fetch"
                )
                return []

            from backend.gitlab.client import GitLabClient
            client = GitLabClient(
                base_url=base_url,
                pat=pat,
                project_id=int(project_id) if project_id else None,
            )
            if not client._project_id:
                logger.warning(
                    "voice_sync: no GITLAB_PROJECT_ID configured — skipping comment fetch"
                )
                await client.close()
                return []

            session = await client._get_session()
            results: List[Dict[str, Any]] = []

            url: Optional[str] = (
                f"{base_url.rstrip('/')}/api/v4/"
                f"projects/{client._project_id}/issues/notes"
            )
            params: Dict[str, Any] = {"per_page": 100, "page": 1}
            while url:
                try:
                    async with session.get(url, params=params) as resp:
                        if resp.status != 200:
                            break
                        data = await resp.json()
                        link_header = resp.headers.get("Link", "")
                except Exception:
                    break

                if not isinstance(data, list) or not data:
                    break

                for note in data:
                    author_info = note.get("author", {}) or {}
                    results.append({
                        "id": str(note.get("id", "")),
                        "author": author_info.get("username", ""),
                        "body": note.get("body", "") or "",
                    })

                from backend.github.client import GitHubClient as _GH
                next_url = _GH._parse_next_link(link_header)
                if next_url:
                    url = next_url
                    params = {}
                else:
                    break

            await client.close()
            return results
        except Exception as exc:
            logger.warning("voice_sync: GitLab comment fetch failed: %s", exc)
            return []
