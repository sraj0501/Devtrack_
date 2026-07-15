"""
WorkspaceRouter — routes a work update to the correct PM platform.

When a commit or timer trigger carries pm_platform (from workspaces.yaml),
the router dispatches directly to that platform. If pm_platform is empty or
"" it falls back to the legacy priority chain (Azure → GitLab → GitHub).
"""

import logging
from typing import Optional, Tuple

logger = logging.getLogger(__name__)

try:
    from runtime_narrative import stage as _stage
except ImportError:
    from contextlib import contextmanager as _cm
    @_cm
    def _stage(name):  # type: ignore[misc]
        yield

# Sentinel: (work_item_id, platform) tuple returned by route()
RouteResult = Tuple[Optional[int], Optional[str]]


class WorkspaceRouter:
    """Routes PM sync calls to the correct platform based on workspace config."""

    def __init__(self, azure_client=None, gitlab_client=None, github_client=None):
        self.azure_client = azure_client
        self.gitlab_client = gitlab_client
        self.github_client = github_client

    def route(
        self,
        pm_platform: str,
        description: str,
        ticket_id: str,
        status: str,
        pm_project: str = "",
        pm_assignee: str = "",
        pm_iteration_path: str = "",
        pm_area_path: str = "",
        pm_milestone: str = "",
        commit_info: Optional[dict] = None,
        task_matcher=None,
    ) -> RouteResult:
        """
        Route the work update to the correct platform.

        Args:
            pm_platform: Platform key from workspaces.yaml ("azure", "gitlab",
                         "github", "jira", "none", or "" for priority chain).
            description:  Human-readable work description.
            ticket_id:    Ticket/issue ID extracted by NLP (may be empty).
            status:       Task status string (e.g. "in_progress", "done").
            pm_project:        Optional platform-specific project override.
            pm_assignee:       Assignee to set when creating work items (Azure: email/name; GitHub: login; GitLab: user ID int).
            pm_iteration_path: Azure iteration path override (e.g. "MyProject\\Sprint 5").
            pm_area_path:      Azure area path override (e.g. "MyProject\\Backend").
            pm_milestone:      Milestone number/ID for GitHub (int) or GitLab (int) on create.
            commit_info:       Dict with commit_hash, commit_message, author.
            task_matcher:      TaskMatcher instance for fuzzy matching.

        Returns:
            (work_item_id, platform) — work_item_id is None on no-match/error.
        """
        platform = (pm_platform or "").strip().lower()
        overrides = {
            "pm_assignee": pm_assignee,
            "pm_iteration_path": pm_iteration_path,
            "pm_area_path": pm_area_path,
            "pm_milestone": pm_milestone,
        }

        if platform == "none":
            logger.debug("Workspace pm_platform=none: skipping PM sync")
            return None, None

        if platform == "azure":
            return self._route_azure(description, ticket_id, status, commit_info, task_matcher, overrides)

        if platform == "gitlab":
            return self._route_gitlab(description, ticket_id, status, pm_project, commit_info, task_matcher, overrides)

        if platform == "github":
            return self._route_github(description, ticket_id, status, commit_info, task_matcher, overrides)

        if platform == "jira":
            logger.info("Jira routing not yet implemented in WorkspaceRouter")
            return None, None

        # Empty or unknown platform — fall back to priority chain
        if platform and platform not in ("azure", "gitlab", "github", "jira", "none"):
            logger.warning(f"Unknown pm_platform={platform!r}, falling back to priority chain")

        return self._route_priority_chain(description, ticket_id, status, pm_project, commit_info, task_matcher, overrides)

    # ------------------------------------------------------------------
    # Platform-specific helpers
    # ------------------------------------------------------------------

    def _route_azure(self, description, ticket_id, status, commit_info, task_matcher, overrides=None) -> RouteResult:
        if not self.azure_client:
            logger.debug("Azure client not configured, skipping")
            return None, None
        try:
            import backend.config as config
            if not config.is_azure_sync_enabled():
                return None, None
        except Exception:
            pass

        return self._call_sync(
            "azure",
            self.azure_client,
            description,
            ticket_id,
            status,
            commit_info,
            task_matcher,
            project_id=None,
            overrides=overrides,
        )

    def _route_gitlab(self, description, ticket_id, status, pm_project, commit_info, task_matcher, overrides=None) -> RouteResult:
        if not self.gitlab_client:
            logger.debug("GitLab client not configured, skipping")
            return None, None
        try:
            import backend.config as config
            if not config.is_gitlab_sync_enabled():
                return None, None
        except Exception:
            pass

        project_id = None
        if pm_project:
            try:
                project_id = int(pm_project)
            except ValueError:
                logger.warning(f"pm_project={pm_project!r} is not an integer for GitLab, ignoring")

        return self._call_sync(
            "gitlab",
            self.gitlab_client,
            description,
            ticket_id,
            status,
            commit_info,
            task_matcher,
            project_id=project_id,
            overrides=overrides,
        )

    def _route_github(self, description, ticket_id, status, commit_info, task_matcher, overrides=None) -> RouteResult:
        if not self.github_client:
            logger.debug("GitHub client not configured, skipping")
            return None, None
        try:
            import backend.config as config
            if not config.is_github_sync_enabled():
                return None, None
        except Exception:
            pass

        return self._call_sync(
            "github",
            self.github_client,
            description,
            ticket_id,
            status,
            commit_info,
            task_matcher,
            project_id=None,
            overrides=overrides,
        )

    def _route_priority_chain(self, description, ticket_id, status, pm_project, commit_info, task_matcher, overrides=None) -> RouteResult:
        """Azure → GitLab → GitHub fallback (legacy single-repo behavior)."""
        try:
            import backend.config as config
        except ImportError:
            config = None

        # Try Azure
        if self.azure_client and config and config.is_azure_sync_enabled():
            work_item_id, platform = self._call_sync(
                "azure", self.azure_client, description, ticket_id, status,
                commit_info, task_matcher, project_id=None, overrides=overrides,
            )
            if work_item_id:
                return work_item_id, platform

        # Try GitLab
        if self.gitlab_client and config and config.is_gitlab_sync_enabled():
            project_id = None
            if pm_project:
                try:
                    project_id = int(pm_project)
                except ValueError:
                    pass
            work_item_id, platform = self._call_sync(
                "gitlab", self.gitlab_client, description, ticket_id, status,
                commit_info, task_matcher, project_id=project_id, overrides=overrides,
            )
            if work_item_id:
                return work_item_id, platform

        # Try GitHub
        if self.github_client and config and config.is_github_sync_enabled():
            work_item_id, platform = self._call_sync(
                "github", self.github_client, description, ticket_id, status,
                commit_info, task_matcher, project_id=None, overrides=overrides,
            )
            if work_item_id:
                return work_item_id, platform

        return None, None

    # ------------------------------------------------------------------
    # Direct state transitions (TASK-126)
    # ------------------------------------------------------------------

    #: Logical status values that mean "this ticket's work is finished".
    DONE_WORDS = ("done", "completed", "closed")

    def route_state_transition(
        self,
        pm_platform: str,
        ticket_id: str,
        new_state: str,
        pm_project: str = "",
        clear_label: str = "",
    ) -> bool:
        """Apply a state transition to an exact ticket, targeted by ID.

        Unlike :meth:`route`, which fuzzy-matches open items against commit
        text, this targets the ticket the extractor already identified — the
        state_transition queue action carries a resolved ticket_id, and
        guessing when the answer is in hand is a correctness bug.

        ``new_state`` is one of:
        - the logical ``"done"`` (mapped per platform: Azure → configured done
          state, GitHub/GitLab → close issue),
        - a platform-native state string (e.g. Azure ``"Active"``, Jira
          ``"In Progress"``) applied verbatim where supported,
        - ``"label:<name>"`` (TASK-129) — GitHub/GitLab in-progress convention:
          apply the label to the issue (their APIs have no in-progress state).

        ``clear_label`` names a label to best-effort remove on done transitions
        (GitHub/GitLab), so the in-progress label doesn't linger on closed
        issues.

        Returns True when the transition was applied, False when it was
        skipped (unsupported platform/state) or failed.
        """
        platform = (pm_platform or "").strip().lower()
        coro = self._async_state_transition(platform, ticket_id, new_state, pm_project, clear_label)
        return self._run_coro(coro, f"route_state_transition({platform})")

    async def _async_state_transition(self, platform, ticket_id, new_state, pm_project, clear_label="") -> bool:
        import re

        try:
            import backend.config as config
        except ImportError:
            config = None

        num_match = re.search(r"(\d+)$", ticket_id or "")
        if not num_match:
            logger.warning(f"state_transition: no numeric ID in ticket_id={ticket_id!r}, skipping")
            return False
        item_id = int(num_match.group(1))
        state_str = (new_state or "").strip()
        is_done = state_str.lower() in self.DONE_WORDS
        label = state_str[len("label:"):].strip() if state_str.lower().startswith("label:") else ""

        if platform == "azure":
            if not self.azure_client:
                logger.debug("state_transition: Azure client not configured, skipping")
                return False
            state = new_state
            if is_done:
                state = "Done"
                if config:
                    try:
                        state = config.get_azure_done_state()
                    except Exception:
                        pass
            ok = await self.azure_client.update_work_item_state(item_id, state)
            logger.info(f"state_transition: Azure #{item_id} → {state!r} ({'ok' if ok else 'failed'})")
            return bool(ok)

        if platform == "github":
            if not self.github_client:
                logger.debug("state_transition: GitHub client not configured, skipping")
                return False
            if label:
                # TASK-129: in-progress via label convention.
                ok = await self.github_client.add_label(item_id, label)
                logger.info(f"state_transition: GitHub #{item_id} label {label!r} ({'ok' if ok else 'failed'})")
                return bool(ok)
            if not is_done:
                logger.info(f"state_transition: GitHub has no state {new_state!r}, skipping")
                return False
            ok = await self.github_client.close_issue(item_id)
            if ok and clear_label:
                # Best-effort — a lingering label never fails the transition.
                await self.github_client.remove_label(item_id, clear_label)
            logger.info(f"state_transition: GitHub #{item_id} closed ({'ok' if ok else 'failed'})")
            return bool(ok)

        if platform == "gitlab":
            if not self.gitlab_client:
                logger.debug("state_transition: GitLab client not configured, skipping")
                return False
            project_id = None
            if pm_project:
                try:
                    project_id = int(pm_project)
                except ValueError:
                    pass
            if project_id is None and config:
                try:
                    project_id = config.get_gitlab_default_project_id()
                except Exception:
                    pass
            if project_id is None:
                logger.warning(f"state_transition: no GitLab project_id for issue #{item_id}, skipping")
                return False
            if label:
                # TASK-129: in-progress via label convention (additive API param).
                ok = await self.gitlab_client.update_issue(project_id, item_id, {"add_labels": label})
                logger.info(f"state_transition: GitLab #{item_id} label {label!r} ({'ok' if ok else 'failed'})")
                return bool(ok)
            if not is_done:
                logger.info(f"state_transition: GitLab has no state {new_state!r}, skipping")
                return False
            ok = await self.gitlab_client.close_issue(project_id, item_id)
            if ok and clear_label:
                await self.gitlab_client.update_issue(project_id, item_id, {"remove_labels": clear_label})
            logger.info(f"state_transition: GitLab #{item_id} closed ({'ok' if ok else 'failed'})")
            return bool(ok)

        logger.info(f"state_transition: platform {platform!r} not supported, skipping")
        return False

    def route_comment(self, pm_platform: str, ticket_id: str, comment: str, pm_project: str = "") -> bool:
        """Post a comment on an exact ticket, targeted by ID (TASK-127).

        Used by queue actions that carry a resolved ticket_id (e.g. staged by
        ``devtrack git`` or the offline outbox) — the fuzzy-matching
        :meth:`route` must not run when the target is already known.

        Returns True when the comment was posted.
        """
        platform = (pm_platform or "").strip().lower()
        coro = self._async_comment(platform, ticket_id, comment, pm_project)
        return self._run_coro(coro, f"route_comment({platform})")

    @staticmethod
    def _run_coro(coro, label: str) -> bool:
        """Run a coroutine to completion from sync code, safely whether or not
        an event loop is already running in this thread."""
        import asyncio

        try:
            try:
                asyncio.get_running_loop()
            except RuntimeError:
                return asyncio.run(coro)
            # A loop is running in this thread — execute on a worker thread.
            import concurrent.futures
            with concurrent.futures.ThreadPoolExecutor() as pool:
                return pool.submit(asyncio.run, coro).result(timeout=30)
        except Exception as e:
            logger.error(f"WorkspaceRouter.{label} failed: {e}")
            return False

    async def _async_comment(self, platform, ticket_id, comment, pm_project) -> bool:
        import re

        try:
            import backend.config as config
        except ImportError:
            config = None

        num_match = re.search(r"(\d+)$", ticket_id or "")
        if not num_match:
            logger.warning(f"route_comment: no numeric ID in ticket_id={ticket_id!r}, skipping")
            return False
        item_id = int(num_match.group(1))

        if platform == "azure":
            if not self.azure_client:
                return False
            ok = await self.azure_client.add_comment(item_id, comment)
            return bool(ok)

        if platform == "github":
            if not self.github_client:
                return False
            ok = await self.github_client.add_comment(item_id, comment)
            return bool(ok)

        if platform == "gitlab":
            if not self.gitlab_client:
                return False
            project_id = None
            if pm_project:
                try:
                    project_id = int(pm_project)
                except ValueError:
                    pass
            if project_id is None and config:
                try:
                    project_id = config.get_gitlab_default_project_id()
                except Exception:
                    pass
            if project_id is None:
                logger.warning(f"route_comment: no GitLab project_id for issue #{item_id}, skipping")
                return False
            ok = await self.gitlab_client.add_comment(project_id, item_id, comment)
            return bool(ok)

        logger.info(f"route_comment: platform {platform!r} not supported, skipping")
        return False

    # ------------------------------------------------------------------
    # Sync dispatcher — calls the appropriate async helper via asyncio
    # ------------------------------------------------------------------

    def _call_sync(self, platform, client, description, ticket_id, status, commit_info, task_matcher, project_id, overrides=None) -> RouteResult:
        """Run the async platform sync in the current or new event loop."""
        import asyncio
        try:
            loop = asyncio.get_event_loop()
            if loop.is_running():
                import concurrent.futures
                with concurrent.futures.ThreadPoolExecutor() as pool:
                    future = pool.submit(
                        asyncio.run,
                        self._async_sync(platform, client, description, ticket_id, status, commit_info, task_matcher, project_id, overrides),
                    )
                    return future.result(timeout=30)
            else:
                return loop.run_until_complete(
                    self._async_sync(platform, client, description, ticket_id, status, commit_info, task_matcher, project_id, overrides)
                )
        except Exception as e:
            logger.error(f"WorkspaceRouter._call_sync({platform}) failed: {e}")
            return None, None

    async def _async_sync(self, platform, client, description, ticket_id, status, commit_info, task_matcher, project_id, overrides=None) -> RouteResult:
        """Async core: match → comment/transition → create-on-no-match."""
        try:
            import backend.config as config
        except ImportError:
            config = None

        commit_info = commit_info or {}
        overrides = overrides or {}
        commit_msg = commit_info.get("commit_message", description)
        commit_hash = commit_info.get("commit_hash", "")
        author = commit_info.get("author", "")

        pm_assignee       = overrides.get("pm_assignee", "") or ""
        pm_iteration_path = overrides.get("pm_iteration_path", "") or ""
        pm_area_path      = overrides.get("pm_area_path", "") or ""
        pm_milestone      = overrides.get("pm_milestone", "") or ""

        # ---- Azure ----
        if platform == "azure":
            work_items = []
            with _stage("Azure: fetch work items"):
                try:
                    work_items = await client.get_my_work_items() or []
                except Exception as e:
                    logger.error(f"Azure get_my_work_items failed: {e}")
                    return None, None

            matched_item = None
            if task_matcher and work_items:
                candidates = [{"id": wi.id, "title": wi.title} for wi in work_items]
                threshold = 0.6
                if config:
                    try:
                        threshold = config.get_azure_match_threshold()
                    except Exception:
                        pass
                match = task_matcher.find_best_match(commit_msg, candidates, threshold=threshold)
                if match:
                    matched_item = next((wi for wi in work_items if wi.id == match["id"]), None)

            comment = f"DevTrack: {description}"
            if commit_hash:
                comment += f"\n\nCommit: `{commit_hash[:8]}`"
            if author:
                comment += f"\nAuthor: {author}"

            if matched_item:
                with _stage(f"Azure: comment on AB#{matched_item.id}"):
                    try:
                        await client.add_comment(matched_item.id, comment)
                        logger.info(f"Azure: commented on work item #{matched_item.id}")
                        if config and config.is_azure_auto_transition() and status in ("done", "completed", "closed"):
                            done_state = "Done"
                            try:
                                done_state = config.get_azure_done_state()
                            except Exception:
                                pass
                            await client.update_work_item_state(matched_item.id, done_state)
                        return matched_item.id, "azure"
                    except Exception as e:
                        logger.error(f"Azure comment/transition failed: {e}")
                        return None, None
            elif config and config.is_azure_create_on_no_match():
                with _stage("Azure: create work item"):
                    try:
                        title = description[:120] if description else commit_msg[:120]
                        new_item = await client.create_work_item(
                            title=title,
                            description=description,
                            assigned_to=pm_assignee or None,
                            area_path=pm_area_path or None,
                            iteration_path=pm_iteration_path or None,
                        )
                        logger.info(f"Azure: created work item #{new_item.id}")
                        return new_item.id, "azure"
                    except Exception as e:
                        logger.error(f"Azure create work item failed: {e}")
                        return None, None
            return None, None

        # ---- GitLab ----
        if platform == "gitlab":
            issues = []
            with _stage("GitLab: fetch issues"):
                try:
                    if project_id is None and config:
                        try:
                            project_id = config.get_gitlab_default_project_id()
                        except Exception:
                            pass
                    issues = await client.get_my_issues(project_id=project_id) or []
                except Exception as e:
                    logger.error(f"GitLab get_my_issues failed: {e}")
                    return None, None

            matched_issue = None
            if task_matcher and issues:
                candidates = [{"id": iss.id, "title": iss.title} for iss in issues]
                threshold = 0.6
                if config:
                    try:
                        threshold = config.get_gitlab_match_threshold()
                    except Exception:
                        pass
                match = task_matcher.find_best_match(commit_msg, candidates, threshold=threshold)
                if match:
                    matched_issue = next((iss for iss in issues if iss.id == match["id"]), None)

            comment = f"DevTrack: {description}"
            if commit_hash:
                comment += f"\n\nCommit: `{commit_hash[:8]}`"
            if author:
                comment += f"\nAuthor: {author}"

            issue_project_id = project_id
            if matched_issue:
                if hasattr(matched_issue, "project_id") and matched_issue.project_id:
                    issue_project_id = matched_issue.project_id
                if issue_project_id is None:
                    logger.warning("GitLab: no project_id for matched issue, skipping")
                    return None, None
                with _stage(f"GitLab: comment on #{matched_issue.iid}"):
                    try:
                        await client.add_comment(issue_project_id, matched_issue.iid, comment)
                        logger.info(f"GitLab: commented on issue #{matched_issue.iid}")
                        if config and config.is_gitlab_auto_transition() and status in ("done", "completed", "closed"):
                            await client.close_issue(issue_project_id, matched_issue.iid)
                        return matched_issue.id, "gitlab"
                    except Exception as e:
                        logger.error(f"GitLab comment/close failed: {e}")
                        return None, None
            elif config and config.is_gitlab_create_on_no_match():
                if issue_project_id is None:
                    logger.warning("GitLab: no project_id for create-on-no-match, skipping")
                    return None, None
                with _stage("GitLab: create issue"):
                    try:
                        title = description[:120] if description else commit_msg[:120]
                        gl_assignee_ids = None
                        if pm_assignee:
                            try:
                                gl_assignee_ids = [int(pm_assignee)]
                            except ValueError:
                                logger.warning(f"GitLab pm_assignee={pm_assignee!r} is not an integer user ID, ignoring")
                        gl_milestone_id = None
                        if pm_milestone:
                            try:
                                gl_milestone_id = int(pm_milestone)
                            except ValueError:
                                logger.warning(f"GitLab pm_milestone={pm_milestone!r} is not an integer, ignoring")
                        new_issue = await client.create_issue(
                            issue_project_id,
                            title=title,
                            description=description,
                            assignee_ids=gl_assignee_ids,
                            milestone_id=gl_milestone_id,
                        )
                        logger.info(f"GitLab: created issue #{new_issue.iid}")
                        return new_issue.id, "gitlab"
                    except Exception as e:
                        logger.error(f"GitLab create issue failed: {e}")
                        return None, None
            return None, None

        # ---- GitHub ----
        if platform == "github":
            issues = []
            with _stage("GitHub: fetch issues"):
                try:
                    issues = await client.get_my_issues(state="open") or []
                except Exception as e:
                    logger.error(f"GitHub get_my_issues failed: {e}")
                    return None, None

            matched_issue = None
            if task_matcher and issues:
                candidates = [{"id": iss.number, "title": iss.title} for iss in issues]
                threshold = 0.6
                if config:
                    try:
                        threshold = config.get_github_match_threshold()
                    except Exception:
                        pass
                match = task_matcher.find_best_match(commit_msg, candidates, threshold=threshold)
                if match:
                    matched_issue = next((iss for iss in issues if iss.number == match["id"]), None)

            comment = f"DevTrack: {description}"
            if commit_hash:
                comment += f"\n\nCommit: `{commit_hash[:8]}`"
            if author:
                comment += f"\nAuthor: {author}"

            if matched_issue:
                with _stage(f"GitHub: comment on #{matched_issue.number}"):
                    try:
                        await client.add_comment(matched_issue.number, comment)
                        logger.info(f"GitHub: commented on issue #{matched_issue.number}")
                        if config and config.is_github_auto_transition() and status in ("done", "completed", "closed"):
                            await client.close_issue(matched_issue.number)
                        return matched_issue.number, "github"
                    except Exception as e:
                        logger.error(f"GitHub comment/close failed: {e}")
                        return None, None
            elif config and config.is_github_create_on_no_match():
                with _stage("GitHub: create issue"):
                    try:
                        title = description[:120] if description else commit_msg[:120]
                        labels = []
                        if config:
                            try:
                                label = config.get_github_sync_label()
                                if label:
                                    labels = [label]
                            except Exception:
                                pass
                        gh_assignees = [pm_assignee] if pm_assignee else None
                        gh_milestone = None
                        if pm_milestone:
                            try:
                                gh_milestone = int(pm_milestone)
                            except ValueError:
                                logger.warning(f"GitHub pm_milestone={pm_milestone!r} is not an integer, ignoring")
                        new_issue = await client.create_issue(
                            title=title,
                            body=description,
                            labels=labels,
                            assignees=gh_assignees,
                            milestone=gh_milestone,
                        )
                        logger.info(f"GitHub: created issue #{new_issue.number}")
                        return new_issue.number, "github"
                    except Exception as e:
                        logger.error(f"GitHub create issue failed: {e}")
                        return None, None
            return None, None

        logger.warning(f"WorkspaceRouter: unhandled platform {platform!r}")
        return None, None
