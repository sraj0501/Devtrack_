"""
DevTrack Webhook + Trigger Server (CS-1)

FastAPI server that handles both inbound webhook events (Azure DevOps, GitHub,
GitLab, Jira) AND outbound trigger calls from the Go daemon (commit_trigger,
timer_trigger, and the rest of the IPC message vocabulary).

Communication with Go is over HTTPS (mutual cert-pinning on the Go side).
All /trigger/* endpoints require the X-DevTrack-API-Key header.

Usage: python -m backend.webhook_server
"""

import asyncio
import base64
import hashlib
import hmac
import json
import logging
import os
import signal
import sys

# Ensure project root is importable
_script_dir = os.path.dirname(os.path.abspath(__file__))
_project_root = os.path.dirname(_script_dir)
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

from contextlib import asynccontextmanager

from fastapi import Depends, FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse

try:
    from backend import config
except ImportError:
    config = None

from backend.webhook_handlers import WebhookEventHandler
from backend.webhook_notifier import WebhookNotifier

logger = logging.getLogger("devtrack.webhook_server")
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)

# Inject current narrative story_id into every log record so that logger.*
# output and narrative.log can be correlated by story_id.  The filter always
# sets record.story_id so the formatter can safely reference %(story_id)s.
class _NarrativeLogFilter(logging.Filter):
    def filter(self, record: logging.LogRecord) -> bool:
        try:
            from runtime_narrative.context import current_story as _cs
            s = _cs.get(None)
            record.story_id = str(s.story_id) if s else "-"
        except Exception:
            record.story_id = "-"
        return True

logging.getLogger().addFilter(_NarrativeLogFilter())

# Formatter that includes story_id and supplies "-" when the attribute is
# absent (e.g. on records from loggers that bypass the filter).
class _NarrativeFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        if not hasattr(record, "story_id"):
            record.story_id = "-"
        return super().format(record)

_narrative_fmt = _NarrativeFormatter(
    "%(asctime)s - %(name)s - %(levelname)s - [%(story_id)s] %(message)s"
)
for _h in logging.getLogger().handlers:
    _h.setFormatter(_narrative_fmt)

# Passed to uvicorn.run() so it does not call dictConfig() and overwrite the
# _NarrativeFormatter we just installed.  disable_existing_loggers=False keeps
# all handlers intact.  uvicorn.access is set to CRITICAL to silence the
# per-request "GET /path 200" lines — runtime-narrative covers all of that.
_UVICORN_LOG_CONFIG: dict = {
    "version": 1,
    "disable_existing_loggers": False,
    "handlers": {},
    "loggers": {
        "uvicorn":        {"level": "WARNING", "propagate": True},
        "uvicorn.error":  {"level": "WARNING", "propagate": True},
        "uvicorn.access": {"level": "CRITICAL", "propagate": False},
    },
}

# ---------------------------------------------------------------------------
# App setup
# ---------------------------------------------------------------------------

@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("DevTrack Webhook + Trigger Server starting (CS-1 HTTP mode)")
    from backend.db.startup import initialize_server_database
    await asyncio.to_thread(initialize_server_database)
    await asyncio.to_thread(TriggerProcessor.get)
    await _ensure_gitlab_webhooks()
    yield
    logger.info("DevTrack Webhook + Trigger Server stopped")


app = FastAPI(title="DevTrack Webhooks", version="1.0", lifespan=lifespan)

try:
    from runtime_narrative import RuntimeNarrativeMiddleware, JsonRenderer, OllamaFailureAnalyzer

    # Log path: NARRATIVE_LOG_PATH > LOG_DIR/narrative.log
    _narrative_log_dir = os.environ.get("LOG_DIR", "Data/logs")
    os.makedirs(_narrative_log_dir, exist_ok=True)
    _narrative_log_path = os.environ.get(
        "NARRATIVE_LOG_PATH",
        os.path.join(_narrative_log_dir, "narrative.log"),
    )
    _narrative_fh = open(_narrative_log_path, "a", encoding="utf-8")  # noqa: SIM115

    # OllamaFailureAnalyzer: wire only when Ollama is reachable at startup.
    # Uses GIT_SAGE_DEFAULT_MODEL (same model as git-sage) for consistency.
    _failure_analyzer = None
    _ollama_host = os.environ.get("OLLAMA_HOST", "").rstrip("/")
    # OLLAMA_HOST=0.0.0.0 leaks from Ollama's own process into the shell env.
    # Normalise to localhost so the ping and API calls actually work.
    if _ollama_host in ("http://0.0.0.0", "0.0.0.0"):
        _ollama_host = "http://localhost:11434"
    if _ollama_host:
        try:
            import urllib.request as _ur
            _ur.urlopen(f"{_ollama_host}/api/tags", timeout=2)
            # Prefer a small fast model for failure analysis — gemma4/large
            # models take >12s cold-start and will always time out.
            # Override with NARRATIVE_FAILURE_MODEL in .env if needed.
            _sage_model = (
                os.environ.get("NARRATIVE_FAILURE_MODEL")
                or os.environ.get("GIT_SAGE_DEFAULT_MODEL")
                or os.environ.get("OLLAMA_MODEL", "llama3.2")
            )
            _timeout = float(os.environ.get("NARRATIVE_FAILURE_TIMEOUT_SECS", "30"))
            _failure_analyzer = OllamaFailureAnalyzer(
                model=_sage_model,
                endpoint=f"{_ollama_host}/api/generate",
                timeout_seconds=_timeout,
            )
            logger.info(
                "runtime-narrative: OllamaFailureAnalyzer wired (model=%s, timeout=%.0fs)",
                _sage_model, _timeout,
            )
        except Exception as _e:
            logger.debug("runtime-narrative: OllamaFailureAnalyzer skipped — Ollama unreachable: %s", _e)

    # Renderers: JsonRenderer always; ConsoleRenderer added in dev mode.
    # Set NARRATIVE_RENDERER=console (requires PYTHONIOENCODING=utf-8 on Windows).
    _renderers = [JsonRenderer(output=_narrative_fh)]
    if os.environ.get("NARRATIVE_RENDERER") == "console":
        from runtime_narrative.renderer.console import ConsoleRenderer as _CR
        _renderers.append(_CR())

    app.add_middleware(
        RuntimeNarrativeMiddleware,
        renderers=_renderers,
        failure_analyzer=_failure_analyzer,
    )
    logger.info("runtime-narrative middleware enabled → %s", _narrative_log_path)
except (ImportError, TypeError):
    pass

# stage() and story_id helper — both fall back to no-ops if runtime_narrative absent.
try:
    from runtime_narrative import stage as _stage
    from runtime_narrative.context import current_story as _current_story

    def _story_id() -> str | None:
        s = _current_story.get(None)
        return str(s.story_id) if s else None

except ImportError:
    from contextlib import contextmanager as _cm

    @_cm
    def _stage(name):  # type: ignore[misc]
        yield

    def _story_id() -> None:  # type: ignore[misc]
        return None

# Optionally embed the admin console directly on this app (single-process mode).
# Controlled by ADMIN_EMBED=true in .env.  When false (default) the admin app
# runs as a separate process on ADMIN_PORT.
try:
    from backend.config import get_admin_embed
    if get_admin_embed():
        from pathlib import Path
        from fastapi.staticfiles import StaticFiles
        from backend.admin.routes import router as _admin_router, startup as _admin_startup
        _admin_startup()
        app.include_router(_admin_router, prefix="/admin")
        _admin_static = Path(__file__).parent / "admin" / "static"
        app.mount("/admin/static", StaticFiles(directory=str(_admin_static)), name="admin-static")
        logger.info("Admin console embedded on main webhook server at /admin")
except Exception as _exc:
    logger.error(
        "Admin embed FAILED — /admin will not be available. "
        "Check SCRYPT_N/R/P/DKLEN, ADMIN_* vars in .env. Error: %s",
        _exc, exc_info=True,
    )

_handler: WebhookEventHandler | None = None

# ---------------------------------------------------------------------------
# Trigger processor (CS-1: HTTP trigger endpoints for external/remote mode)
# ---------------------------------------------------------------------------

class TriggerProcessor:
    """
    Processes commit and timer triggers received via HTTP POST.

    Used when DEVTRACK_SERVER_MODE=external — Go POSTs to /trigger/commit
    and /trigger/timer instead of sending over IPC.  Mirrors the component
    All imports are lazy and guarded so missing deps degrade
    gracefully (Rule 0: everything still works locally too).
    """

    _instance: "TriggerProcessor | None" = None

    @classmethod
    def get(cls) -> "TriggerProcessor":
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance

    def __init__(self) -> None:
        self._init_components()
        # Queue gateway — holds no connection of its own (backed by the
        # shared engine, opened per-call). Construction is lightweight and
        # only fails on an import error; actual DB/queue-availability
        # failures surface later from individual method calls and are handled
        # at the call site in process_commit(), not here.
        self._queue_gateway = None
        try:
            from backend.queue_gateway import QueueGateway
            self._queue_gateway = QueueGateway()
            logger.info("✓ TriggerProcessor: QueueGateway ready")
        except Exception as e:
            logger.debug("QueueGateway unavailable (non-fatal): %s", e)

    def _init_components(self) -> None:
        # Optional LLM enrichment; failures degrade to the raw commit message.
        self.llm_task_parser = None
        try:
            from backend.llm_task_parser import LLMTaskParser
            self.llm_task_parser = LLMTaskParser()
            logger.info("✓ TriggerProcessor: LLM task parser ready")
        except Exception as e:
            logger.debug("LLM task parser unavailable: %s", e)

        # Description enhancer
        self.description_enhancer = None
        try:
            from backend.description_enhancer import DescriptionEnhancer
            self.description_enhancer = DescriptionEnhancer()
            logger.info("✓ TriggerProcessor: description enhancer ready")
        except Exception as e:
            logger.debug(f"Description enhancer unavailable: {e}")

        # Azure DevOps
        self.azure_client = None
        try:
            from backend.azure.client import AzureDevOpsClient
            c = AzureDevOpsClient()
            if c.is_configured():
                self.azure_client = c
                logger.info("✓ TriggerProcessor: Azure DevOps ready")
        except Exception as e:
            logger.debug(f"Azure DevOps unavailable: {e}")

        # GitLab
        self.gitlab_client = None
        try:
            from backend.gitlab.client import GitLabClient
            c = GitLabClient()
            if c.is_configured():
                self.gitlab_client = c
                logger.info("✓ TriggerProcessor: GitLab ready")
        except Exception as e:
            logger.debug(f"GitLab unavailable: {e}")

        # GitHub
        self.github_client = None
        try:
            from backend.github.client import GitHubClient
            c = GitHubClient()
            if c.is_configured():
                self.github_client = c
                logger.info("✓ TriggerProcessor: GitHub ready")
        except Exception as e:
            logger.debug(f"GitHub unavailable: {e}")

        # WorkspaceRouter — routes PM updates to the right platform
        self.workspace_router = None
        try:
            from backend.workspace_router import WorkspaceRouter
            self.workspace_router = WorkspaceRouter(
                azure_client=self.azure_client,
                gitlab_client=self.gitlab_client,
                github_client=self.github_client,
            )
            logger.info("✓ TriggerProcessor: WorkspaceRouter ready")
        except Exception as e:
            logger.debug(f"WorkspaceRouter unavailable: {e}")

        # TaskMatcher
        self.task_matcher = None
        try:
            from backend.task_matcher import TaskMatcher
            self.task_matcher = TaskMatcher(use_semantic=False)
            logger.info("✓ TriggerProcessor: TaskMatcher ready")
        except Exception as e:
            logger.debug(f"TaskMatcher unavailable: {e}")

    # ------------------------------------------------------------------
    # Internal: execute a staged PM action (called by /queue/execute)
    # ------------------------------------------------------------------

    def _execute_pm_action(self, action: dict) -> dict:
        """Execute a staged PM action by routing it through the workspace router.

        This method encapsulates ALL direct PM API calls.  It is the only
        place in ``TriggerProcessor`` that actually posts to GitHub, Azure,
        GitLab, or Jira.  The trigger handlers (``process_commit``,
        ``process_timer``) no longer call the workspace router directly —
        they stage actions via ``QueueGateway.stage()`` instead, and the Go
        daemon calls this endpoint via ``POST /queue/execute`` when the action
        is approved or its confidence timeout has expired.

        :param action: A ``pending_actions`` row as a plain dict (as returned
            by ``QueueGateway.get_action()``).  The ``payload`` field is a
            JSON string; this method parses it internally.
        :returns: ``{"status": "posted"}`` on success, or
            ``{"status": "failed", "error": "<message>"}`` on failure.
        """
        import json as _json

        payload_raw = action.get("payload", "{}")
        try:
            payload = _json.loads(payload_raw) if isinstance(payload_raw, str) else payload_raw
        except Exception:
            payload = {}

        pm_platform = action.get("platform", "")
        workspace   = action.get("workspace", "")
        action_type = action.get("action_type", "")
        target      = action.get("target", "")

        # ---------------------------------------------------------------------------
        # Notification-only actions (no PM API call required, no workspace_router).
        # These are handled before the workspace_router guard so they work even
        # when the router is unavailable. Telegram delivery is done by the Go bot.
        # ---------------------------------------------------------------------------

        if action_type == "pr_approved_notify":
            # Notification-only action — Telegram delivery is handled by the Go bot
            # (SendPRApproved wired in the daemon goroutine). The Python server just
            # acknowledges receipt so the queue executor can mark the row posted.
            pr_id = payload.get("pr_id", target)
            fixes_applied = payload.get("fixes_applied", 0)
            logger.info(
                "queue: pr_approved_notify action %s (pr=%s fixes_applied=%s) — "
                "notification handled by Go Telegram bot",
                action.get("id"), pr_id, fixes_applied,
            )
            return {"status": "posted"}

        if action_type == "pr_escalation":
            # Notification-only action — Telegram delivery is handled by the Go bot
            # (SendPREscalation wired in the daemon goroutine). The Python server just
            # acknowledges receipt so the queue executor can mark the row posted.
            pr_id = payload.get("pr_id", target)
            blocker_reason = payload.get("blocker_reason", "")
            logger.info(
                "queue: pr_escalation action %s (pr=%s blocker=%r) — "
                "notification handled by Go Telegram bot",
                action.get("id"), pr_id, blocker_reason,
            )
            return {"status": "posted"}

        if not self.workspace_router:
            return {"status": "failed", "error": "workspace_router not available"}

        # Branch on action_type to route correctly.
        # post_comment   → pass description/comment text to workspace_router.route()
        # state_transition → pass new_state as status; no comment text required
        # unknown type    → log warning, mark complete (never fail the queue)

        if action_type == "state_transition":
            new_state = payload.get("new_state", "")
            ticket_id_for_route = payload.get("ticket_id", target)
            try:
                # TASK-126: direct-by-ID transition. The old path went through
                # route(), which fuzzy-matches commit text and only transitions
                # on done-words — so in-progress transitions never applied and
                # the exact ticket_id in hand was ignored.
                applied = self.workspace_router.route_state_transition(
                    pm_platform=pm_platform,
                    ticket_id=ticket_id_for_route,
                    new_state=new_state,
                    pm_project=payload.get("pm_project", ""),
                    clear_label=payload.get("in_progress_label", ""),
                )
                logger.info(
                    "queue: executed state_transition action %s "
                    "(target=%s new_state=%r platform=%s applied=%s)",
                    action.get("id"), target, new_state, pm_platform, applied,
                )
                if applied:
                    return {"status": "posted"}
                return {"status": "failed", "error": "transition not applied (unsupported or API error)"}
            except Exception as e:
                logger.warning(
                    "queue: state_transition action %s failed (target=%s): %s",
                    action.get("id"), target, e,
                )
                return {"status": "failed", "error": str(e)}

        if action_type == "eod_report":
            narrative = payload.get("narrative", "")
            email = payload.get("email", "")
            try:
                if email:
                    from backend.email_reporter import EmailReporter as _EODEmailReporter
                    reporter = _EODEmailReporter()
                    reporter.send_text_report(narrative, email)
                logger.info(
                    "queue: executed eod_report action %s (delivered_to=%s)",
                    action.get("id"), email or "none",
                )
                return {"status": "posted", "delivered_to": email or "none"}
            except Exception as e:
                # Never raise — Non-Negotiable #8: never block on failure.
                logger.warning(
                    "queue: eod_report action %s delivery error (non-fatal): %s",
                    action.get("id"), e,
                )
                return {"status": "posted", "delivered_to": email or "none"}

        if action_type not in ("post_comment",):
            # Unknown action type — log and mark complete to avoid blocking the queue
            logger.warning(
                "queue: unknown action_type %r for action %s (target=%s) — "
                "marking complete without executing",
                action_type, action.get("id"), target,
            )
            return {"status": "posted"}

        # post_comment (and any future comment-type actions)
        # TASK-127: actions staged with an exact, developer-confirmed ticket
        # (devtrack git / offline outbox) set direct_ticket=true — post by ID,
        # never fuzzy-match a target the developer already named.
        if payload.get("direct_ticket"):
            try:
                posted = self.workspace_router.route_comment(
                    pm_platform=pm_platform,
                    ticket_id=payload.get("ticket_id", target),
                    comment=payload.get("comment", payload.get("description", "")),
                    pm_project=payload.get("pm_project", ""),
                )
                if posted:
                    return {"status": "posted"}
                return {"status": "failed", "error": "direct comment not posted (unsupported or API error)"}
            except Exception as e:
                logger.warning(
                    "queue: direct post_comment action %s failed (target=%s): %s",
                    action.get("id"), target, e,
                )
                return {"status": "failed", "error": str(e)}

        try:
            self.workspace_router.route(
                pm_platform=pm_platform,
                description=payload.get("description", payload.get("comment", "")),
                ticket_id=payload.get("ticket_id", target),
                status=payload.get("status", ""),
                pm_project=payload.get("pm_project", ""),
                pm_assignee=payload.get("pm_assignee", ""),
                pm_iteration_path=payload.get("pm_iteration_path", ""),
                pm_area_path=payload.get("pm_area_path", ""),
                pm_milestone=payload.get("pm_milestone", ""),
                commit_info=payload.get("commit_info", {}),
            )
            logger.info(
                "queue: executed action %s (type=%s target=%s platform=%s)",
                action.get("id"), action_type, target, pm_platform,
            )
            return {"status": "posted"}
        except Exception as e:
            logger.warning(
                "queue: action %s failed (type=%s target=%s): %s",
                action.get("id"), action_type, target, e,
            )
            return {"status": "failed", "error": str(e)}

    # ------------------------------------------------------------------
    # Commit trigger
    # ------------------------------------------------------------------

    def process_commit(self, data: dict) -> dict:
        """
        Process a commit trigger.  Returns a dict of actions taken.

        Phase 1 change: instead of posting directly to PM APIs via the
        workspace router, this method stages a ``post_comment`` action in
        ``pending_actions`` via ``QueueGateway.stage()``.  The Go daemon's
        queue executor will call ``POST /queue/execute`` after the confidence
        timeout expires (or the developer approves manually).

        If the queue gateway is unavailable (daemon not yet started, DB
        missing, or ``.stage()`` raises for any reason), PM sync is
        skipped for this commit — logged, not raised. There is no
        direct-post fallback: every outbound PM action must stage in the
        pending-actions queue first (non-negotiable), so a queue failure
        here can never result in an unreviewed direct PM API call. This
        method itself never raises — Non-Negotiable #8 (never block, never
        raise back to the trigger caller) still applies; the HTTP response
        is always a clean 200 with whatever ``actions`` were actually taken.
        """
        commit_hash = data.get("commit_hash", "")
        commit_msg  = data.get("commit_message", "")
        repo_path   = data.get("repo_path", "")
        author      = data.get("author", "")
        branch      = data.get("branch", "")
        pm_platform       = data.get("pm_platform", "")
        pm_project        = data.get("pm_project", "")
        pm_assignee       = data.get("pm_assignee", "")
        pm_iteration_path = data.get("pm_iteration_path", "")
        pm_area_path      = data.get("pm_area_path", "")
        pm_milestone      = data.get("pm_milestone", "")
        # Phase 2 — Go-resolved ticket ID (branch/message/active-ticket fallback
        # chain, ~100% hit-rate verified). Absent or empty means Go's extractor
        # found no ticket for this commit (logged [UNLINKED] on the client side).
        # This is the authoritative ticket-resolution signal. Server-side LLM
        # parsing only enriches descriptive fields and cannot infer or redirect
        # the PM target.
        resolved_ticket_id = data.get("ticket_id", "")

        logger.info(f"[HTTP commit] {commit_hash[:12]} — {commit_msg[:60]}")

        actions: list[str] = []

        # Auto-link commit to active work session
        with _stage("Link work session"):
            if commit_hash:
                try:
                    from backend.work_tracker.session_store import WorkSessionStore
                    store   = WorkSessionStore()
                    active  = store.get_active_session()
                    if active:
                        store.append_commit(active["id"], commit_hash)
                        actions.append(f"session_linked:{active['id']}")
                        logger.info(f"Commit {commit_hash[:12]} linked to session #{active['id']}")
                except Exception as e:
                    logger.debug(f"Work session link failed (non-fatal): {e}")

        # Optional LLM enrichment
        task_data = None
        with _stage("LLM task parse"):
            if self.llm_task_parser and commit_msg:
                try:
                    task_data = self.llm_task_parser.parse(commit_msg, repo_path=repo_path)
                except Exception as e:
                    logger.warning("LLM task enrichment failed: %s", e)

        # PM sync — stage via queue (Phase 1) or fall back to direct post
        with _stage("PM sync"):
            if not resolved_ticket_id:
                # Phase 2 found no ticket for this commit (branch/message/
                # active-ticket fallback chain all came up empty — logged
                # [UNLINKED] on the Go side). Non-Negotiable #8: never block,
                # never error. We do not fall back to server inference or
                # truncated-commit-hash target — if Go says unlinked, treat
                # it as unlinked here too.
                logger.info(
                    "PM sync skipped: commit %s has no resolved ticket_id "
                    "(Phase 2 unlinked) — no queue action staged",
                    commit_hash[:12] if commit_hash else "?",
                )
            elif self.workspace_router:
                # task_data may legitimately be None here (configured provider
                # unavailable or parse failed). resolved_ticket_id is the
                # authoritative signal for *targeting*; task_data is only
                # optional descriptive enrichment, so every read of it below
                # must fall back to commit_msg / "" without raising.
                status = task_data.get("status", "") if task_data else ""

                # TASK-128: completion words in a commit message ("done",
                # "finished") are a weak signal — "done with refactor prep"
                # must not close a ticket. Strip the status from the comment
                # payload (the fuzzy route auto-transitions on done-words) and
                # stage an explicit low-confidence state_transition instead
                # (<0.70 → 15-minute review tier), further below.
                language_done = status in ("done", "completed", "closed")
                if language_done:
                    status = ""

                # Phase 3 (TASK-072): generate a voice-aware ticket comment via
                # the voice-aware LLM pipeline. Falls back to a
                # templated string on any LLM failure — never blocks processing.
                try:
                    from backend.commit_message_enhancer import generate_ticket_comment as _gtc
                    ticket_comment = _gtc(
                        commit_message=commit_msg,
                        diff=data.get("diff", ""),
                        files=data.get("files", []),
                        ticket_id=resolved_ticket_id,
                        repo_path=repo_path or None,
                    )
                except Exception as _gtc_exc:
                    logger.warning(
                        "generate_ticket_comment failed (belt-and-suspenders fallback): %s",
                        _gtc_exc,
                    )
                    # Validated enrichment or raw commit text as last resort.
                    ticket_comment = (
                        task_data.get("description", commit_msg) if task_data else commit_msg
                    )

                # Build the payload that _execute_pm_action expects
                pm_payload = {
                    "description": ticket_comment,
                    "ticket_id":   resolved_ticket_id,
                    "status":      status,
                    "pm_project":  pm_project,
                    "pm_assignee": pm_assignee,
                    "pm_iteration_path": pm_iteration_path,
                    "pm_area_path":      pm_area_path,
                    "pm_milestone":      pm_milestone,
                    "comment":     ticket_comment,
                    "commit_info": {
                        "hash":    commit_hash,
                        "message": commit_msg,
                        "author":  author,
                        "branch":  branch,
                    },
                }
                ticket_id = resolved_ticket_id
                # TASK-128: confidence comes from Go's extraction strategy
                # (0.95 branch / 0.85 message / 0.60 active-ticket fallback),
                # carried in the trigger payload. Fallback-derived tickets land
                # below 0.70 → the 15-minute explicit-review tier. 0.85 remains
                # the default for older clients that don't send the field.
                confidence = float(data.get("ticket_confidence") or 0.85)
                confidence = max(0.0, min(confidence, 1.0))

                if getattr(self, "_queue_gateway", None):
                    try:
                        action_id = self._queue_gateway.stage(
                            action_type="post_comment",
                            target=ticket_id,
                            platform=pm_platform or "auto",
                            workspace=data.get("workspace_name", ""),
                            payload=pm_payload,
                            confidence=confidence,
                        )
                        actions.append(f"queued:post_comment:{action_id}")
                        logger.info(
                            "PM sync staged (action_id=%d, confidence=%.2f, platform=%s)",
                            action_id, confidence, pm_platform or "auto",
                        )

                        # Phase 3 (TASK-073): stage a SEPARATE state_transition action
                        # when this is the first linked commit for this ticket.
                        # This is an independent queue row with its own confidence and
                        # expiry — per PRODUCT_BIBLE.md Layer 2 ("each queued action").
                        # confidence=0.90 → 2-minute auto-approve window (just above the
                        # 0.90 threshold in ConfidenceTimeout that maps to 2 minutes).
                        try:
                            from backend.ticket_state_mapper import in_progress_state_for as _ips_for
                            is_first = data.get("is_first_commit_for_ticket", False)
                            is_merge_to_default = data.get("is_merge_to_default", False)
                            new_state = _ips_for(pm_platform)

                            # TASK-129: GitHub/GitLab have no in-progress API state —
                            # use the label convention instead. Configurable per
                            # workspace (in_progress_label in workspaces.yaml, carried
                            # in the trigger payload); "none" opts out.
                            in_progress_label = ""
                            if not new_state and (pm_platform or "").lower() in ("github", "gitlab"):
                                in_progress_label = (
                                    data.get("pm_in_progress_label") or "in-progress"
                                ).strip()
                                if in_progress_label.lower() == "none":
                                    in_progress_label = ""
                                elif is_first and not is_merge_to_default:
                                    new_state = f"label:{in_progress_label}"

                            # TASK-126: merge commit landed on the default branch —
                            # "merged to main → Done". Stage a done transition instead
                            # of (never alongside) an in-progress one. The logical
                            # "done" is mapped per platform at execution time
                            # (route_state_transition: Azure done-state, GH/GL close).
                            if is_merge_to_default or language_done:
                                # Merge to default branch: unambiguous completion
                                # signal, capped at 0.90 but tied to how surely the
                                # ticket itself was identified (TASK-128).
                                # Commit-language "done": weak signal — fixed 0.65,
                                # the 15-minute explicit-review tier.
                                done_confidence = (
                                    min(0.90, confidence + 0.05) if is_merge_to_default else 0.65
                                )
                                done_action_id = self._queue_gateway.stage(
                                    action_type="state_transition",
                                    target=resolved_ticket_id,
                                    platform=pm_platform or "auto",
                                    workspace=data.get("workspace_name", ""),
                                    payload={
                                        "ticket_id": resolved_ticket_id,
                                        "new_state": "done",
                                        "pm_project": pm_project,
                                        "in_progress_label": in_progress_label,
                                        "commit_info": pm_payload["commit_info"],
                                    },
                                    confidence=done_confidence,
                                )
                                actions.append(f"queued:state_transition:{done_action_id}")
                                logger.info(
                                    "Done transition staged (action_id=%d, platform=%s, ticket=%s, "
                                    "confidence=%.2f, signal=%s)",
                                    done_action_id, pm_platform or "auto", resolved_ticket_id,
                                    done_confidence, "merge" if is_merge_to_default else "commit-language",
                                )
                            elif is_first and new_state:
                                state_action_id = self._queue_gateway.stage(
                                    action_type="state_transition",
                                    target=resolved_ticket_id,
                                    platform=pm_platform or "auto",
                                    workspace=data.get("workspace_name", ""),
                                    payload={
                                        "ticket_id": resolved_ticket_id,
                                        "new_state": new_state,
                                        "pm_project": pm_project,
                                        "commit_info": pm_payload["commit_info"],
                                    },
                                    # First-commit-for-ticket is a strong signal, but only
                                    # as strong as the ticket identification itself
                                    # (TASK-128): capped at 0.90, fallback-derived tickets
                                    # land in the explicit-review tier.
                                    confidence=min(0.90, confidence + 0.05),
                                )
                                actions.append(f"queued:state_transition:{state_action_id}")
                                logger.info(
                                    "State transition staged (action_id=%d, new_state=%r, platform=%s)",
                                    state_action_id, new_state, pm_platform or "auto",
                                )
                            elif is_first and not new_state:
                                logger.debug(
                                    "State transition skipped: platform %r has no "
                                    "in-progress state mapping", pm_platform,
                                )
                        except Exception as _st_exc:
                            logger.warning(
                                "State transition staging failed (non-fatal): %s", _st_exc
                            )

                        return {
                            "actions":    actions,
                            "commit_hash": commit_hash,
                            "narrative_id": _story_id(),
                            "status":     "queued",
                            "action_id":  action_id,
                        }
                    except Exception as e:
                        # No direct-post fallback (removed — see TASK-112 module
                        # 5/15 notes): every outbound PM action must stage in the
                        # pending-actions queue first, so a staging failure means
                        # PM sync did not happen for this commit, not that it
                        # happens some other way. Never raise out of
                        # process_commit() — Non-Negotiable #8.
                        logger.error(
                            "PM sync could not be staged (queue unavailable) — "
                            "PM sync skipped for this commit, no direct-post "
                            "fallback: %s", e,
                        )
                else:
                    logger.error(
                        "PM sync skipped: queue gateway unavailable — "
                        "no direct-post fallback (every outbound PM action "
                        "must stage in the pending-actions queue first)"
                    )

        return {"actions": actions, "commit_hash": commit_hash, "narrative_id": _story_id()}

    # ------------------------------------------------------------------
    # Timer trigger
    # ------------------------------------------------------------------

    def process_timer(self, data: dict) -> dict:
        """
        Process a timer trigger.

        In HTTP/external mode there is no local TUI — the developer is not
        sitting at the terminal where Python runs.  Primary interaction channel
        is Telegram (the bot is already running and handles /workstop etc.).
        This method acknowledges the trigger and optionally sends a Telegram
        nudge; the full interactive flow happens via Telegram commands.

        Phase 1 note: the timer trigger does NOT post to PM APIs directly today —
        that is the job of the EOD pipeline (Phase 4).  However, to satisfy the
        Phase 1 requirement that ``process_timer`` also calls
        ``queue_gateway.stage()``, this method stages a ``timer_nudge`` action
        when a queue gateway is available.  The confidence is set to 0.60 so
        the action enters the 15-minute review window; the Go executor will skip
        it if the developer does not approve within that window.
        """
        interval_mins  = data.get("interval_mins", 0)
        trigger_count  = data.get("trigger_count", 0)
        pm_platform    = data.get("pm_platform", "")
        workspace_name = data.get("workspace_name", "")

        logger.info(f"[HTTP timer] trigger #{trigger_count} (every {interval_mins}m, workspace={workspace_name})")

        # Phase 1: stage a timer_nudge action so the queue is populated and
        # the developer can see timer events in the TUI queue panel.
        timer_action_id = None
        _gw = getattr(self, "_queue_gateway", None)
        if _gw:
            try:
                timer_action_id = _gw.stage(
                    action_type="timer_nudge",
                    target=workspace_name or "default",
                    platform=pm_platform or "none",
                    workspace=workspace_name or "",
                    payload={
                        "interval_mins": interval_mins,
                        "trigger_count": trigger_count,
                        "workspace_name": workspace_name,
                    },
                    confidence=0.60,
                )
                logger.info(
                    "timer trigger staged in queue (action_id=%d)", timer_action_id
                )
            except Exception as e:
                logger.debug("timer queue staging failed (non-fatal): %s", e)

        # Check vacation mode — auto-respond instead of nudging
        with _stage("Check vacation mode"):
            try:
                from backend.vacation.auto_responder import is_vacation_active, VacationAutoResponder
                if is_vacation_active():
                    logger.info("[vacation mode] active — auto-generating work update")
                    import asyncio
                    responder = VacationAutoResponder()
                    result = asyncio.run(responder.handle(data))
                    logger.info(
                        "[vacation mode] confidence=%.2f submitted=%s reason=%s",
                        result.get("confidence", 0),
                        result.get("submitted"),
                        result.get("skipped_reason"),
                    )
                    return {
                        "status": "vacation_auto",
                        "trigger_count": trigger_count,
                        "confidence": result.get("confidence", 0),
                        "submitted": result.get("submitted", False),
                        "skipped_reason": result.get("skipped_reason"),
                    }
            except Exception as e:
                logger.debug(f"Vacation auto-responder error (non-fatal): {e}")

        # Check active work session
        active_session = None
        with _stage("Check active session"):
            try:
                from backend.work_tracker.session_store import WorkSessionStore
                active_session = WorkSessionStore().get_active_session()
            except Exception as e:
                logger.debug(f"Work session check failed: {e}")

        # Attempt Telegram nudge (non-fatal)
        telegram_sent = False
        with _stage("Send Telegram reminder"):
            try:
                from backend.telegram.notifier import send_work_reminder as _tg_reminder
                _tg_reminder(
                    interval_mins=interval_mins,
                    trigger_count=trigger_count,
                    active_session=active_session,
                    pm_platform=pm_platform,
                    workspace_name=workspace_name,
                )
                telegram_sent = True
                logger.info("✓ Work reminder sent via Telegram")
            except Exception:
                logger.debug("Telegram reminder unavailable (non-fatal)")

        # Attempt Slack nudge (non-fatal)
        slack_sent = False
        with _stage("Send Slack reminder"):
            try:
                from backend.slack.notifier import send_work_reminder as _slack_reminder
                slack_sent = _slack_reminder(
                    interval_mins=interval_mins,
                    trigger_count=trigger_count,
                    active_session=active_session,
                    pm_platform=pm_platform,
                    workspace_name=workspace_name,
                )
                if slack_sent:
                    logger.info("✓ Work reminder sent via Slack")
            except Exception:
                logger.debug("Slack reminder unavailable (non-fatal)")

        channels = []
        if telegram_sent:
            channels.append("telegram")
        if slack_sent:
            channels.append("slack")

        result: dict = {
            "status": "accepted",
            "trigger_count": trigger_count,
            "prompt_channel": ",".join(channels) if channels else "none",
            "active_session": active_session is not None,
            "narrative_id": _story_id(),
        }
        if timer_action_id is not None:
            result["action_id"] = timer_action_id
        return result


def _get_handler() -> WebhookEventHandler:
    global _handler
    if _handler is None:
        notifier = WebhookNotifier()
        _handler = WebhookEventHandler(ipc_client=None, notifier=notifier)
    return _handler


# ---------------------------------------------------------------------------
# Auth helpers
# ---------------------------------------------------------------------------

def _cfg(key: str, default: str = "") -> str:
    if config:
        return config.get(key, default)
    return default


def _cfg_bool(key: str, default: bool = False) -> bool:
    if config:
        return config.get_bool(key, default)
    return default


async def _verify_trigger_key(request: Request) -> None:
    """Validate X-DevTrack-API-Key on all /trigger/* endpoints.

    Skipped when DEVTRACK_API_KEY is not set (dev/testing mode).
    """
    expected = config.get_devtrack_api_key() if config else ""
    if not expected:
        return  # auth not configured — allow all (dev mode)
    key = request.headers.get("X-DevTrack-API-Key", "")
    if not key or key != expected:
        raise HTTPException(status_code=403, detail="Invalid or missing X-DevTrack-API-Key")


async def _verify_azure_basic_auth(request: Request) -> None:
    """Validate HTTP Basic Auth for Azure DevOps service hooks."""
    expected_user = _cfg("WEBHOOK_AZURE_USERNAME", "")
    expected_pass = _cfg("WEBHOOK_AZURE_PASSWORD", "")
    if not expected_user and not expected_pass:
        # Auth not configured — allow all (dev mode)
        return

    auth_header = request.headers.get("Authorization", "")
    if not auth_header.startswith("Basic "):
        raise HTTPException(status_code=401, detail="Missing Basic auth")

    try:
        decoded = base64.b64decode(auth_header[6:]).decode("utf-8")
        username, password = decoded.split(":", 1)
    except Exception:
        raise HTTPException(status_code=401, detail="Malformed auth header")

    if username != expected_user or password != expected_pass:
        raise HTTPException(status_code=403, detail="Invalid credentials")


async def _verify_github_signature(request: Request) -> None:
    """Validate HMAC-SHA256 signature for GitHub webhooks."""
    secret = _cfg("WEBHOOK_GITHUB_SECRET", "")
    if not secret:
        return  # Signature validation not configured

    signature_header = request.headers.get("X-Hub-Signature-256", "")
    if not signature_header.startswith("sha256="):
        raise HTTPException(status_code=401, detail="Missing GitHub signature")

    body = await request.body()
    expected = "sha256=" + hmac.new(
        secret.encode(), body, hashlib.sha256
    ).hexdigest()

    if not hmac.compare_digest(signature_header, expected):
        raise HTTPException(status_code=403, detail="Invalid signature")


async def _verify_gitlab_token(request: Request) -> None:
    """Validate GitLab webhook secret token."""
    secret = _cfg("WEBHOOK_GITLAB_SECRET")
    if not secret:
        return  # No secret configured — allow all (dev mode)
    token = request.headers.get("X-Gitlab-Token", "")
    if token != secret:
        raise HTTPException(status_code=401, detail="Invalid GitLab token")


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------

@app.post("/webhooks/azure-devops")
async def handle_azure_devops_webhook(
    request: Request,
    _auth: None = Depends(_verify_azure_basic_auth),
) -> JSONResponse:
    """Handle Azure DevOps service hook events."""
    handler = _get_handler()
    try:
        body = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON body")

    event_type = body.get("eventType", "")
    resource = body.get("resource", {})

    if not event_type:
        raise HTTPException(status_code=400, detail="Missing eventType")

    result = await handler.handle_azure_event(event_type, resource, body)
    return JSONResponse(content=result)


@app.post("/webhooks/github")
async def handle_github_webhook(
    request: Request,
    _auth: None = Depends(_verify_github_signature),
) -> JSONResponse:
    """Handle GitHub webhook events (placeholder)."""
    handler = _get_handler()
    try:
        body = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON body")

    event_type = request.headers.get("X-GitHub-Event", "unknown")
    result = await handler.handle_github_event(event_type, body)
    return JSONResponse(content=result)


@app.post("/webhooks/gitlab")
async def handle_gitlab_webhook(
    request: Request,
    _auth: None = Depends(_verify_gitlab_token),
) -> JSONResponse:
    """Handle GitLab webhook events (issue events, MR events, comments)."""
    handler = _get_handler()
    try:
        body = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON body")

    event_type = request.headers.get("X-Gitlab-Event", "unknown")
    result = await handler.handle_gitlab_event(event_type, body)
    return JSONResponse(content=result)


@app.post("/webhooks/jira")
async def handle_jira_webhook(request: Request) -> JSONResponse:
    """Handle Jira webhook events (placeholder)."""
    handler = _get_handler()
    try:
        body = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON body")

    event_type = body.get("webhookEvent", "unknown")
    result = await handler.handle_jira_event(event_type, body)
    return JSONResponse(content=result)


@app.get("/health")
async def health() -> dict:
    return {"status": "ok", "service": "devtrack-webhooks"}


@app.get("/narrative/recent")
async def narrative_recent(
    _auth: None = Depends(_verify_trigger_key),
    n: int = 20,
) -> dict:
    """Return the last n completed request stories from narrative.log.

    Used by 'devtrack logs --narrative' and the admin UI panel.
    """
    try:
        from backend.narrative_reader import get_recent_stories
        stories = get_recent_stories(min(n, 100))
        return {"stories": [s.to_dict() for s in stories]}
    except Exception as exc:
        logger.warning("narrative_recent: %s", exc)
        return {"stories": []}


@app.get("/narrative/last-failure")
async def narrative_last_failure(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Return the most recent FailureOccurred event from narrative.log.

    Used by 'devtrack status' to surface the last known server failure.
    Returns {} when no failure has been recorded.
    """
    try:
        from backend.narrative_reader import get_last_failure
        ev = get_last_failure()
        return ev or {}
    except Exception as exc:
        logger.warning("narrative_last_failure: %s", exc)
        return {}


@app.get("/version")
async def get_version() -> dict:
    """Return server version — used by 'devtrack cloud status'."""
    v = getattr(app, "version", "1.0")
    return {"version": v, "service": "devtrack-webhooks"}


# ---------------------------------------------------------------------------
# Spec Review Endpoints  (AI Project Planning)
# ---------------------------------------------------------------------------

_SPEC_REVIEW_HTML = """\
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>DevTrack — Spec Review</title>
  <style>
    body {{ font-family: sans-serif; max-width: 900px; margin: 2em auto; color: #222; }}
    h1 {{ color: #0066cc; }}
    textarea {{ width: 100%; font-family: monospace; font-size: 0.85em; border: 1px solid #ccc;
                border-radius: 4px; padding: 8px; }}
    .btn {{ display: inline-block; padding: 10px 22px; margin: 6px 4px; border: none;
            border-radius: 4px; font-size: 1em; cursor: pointer; font-weight: bold; }}
    .btn-approve {{ background: #27ae60; color: white; }}
    .btn-revise {{ background: #2980b9; color: white; }}
    .meta {{ color: #666; font-size: 0.9em; margin-bottom: 1em; }}
    .feedback-area {{ display: none; margin-top: 1em; }}
    .error {{ color: #c0392b; font-weight: bold; }}
    .success {{ color: #27ae60; font-weight: bold; }}
  </style>
</head>
<body>
  <h1>Project Spec Review</h1>
  <p class="meta">
    Spec ID: <code>{spec_id}</code> &nbsp;|&nbsp;
    Status: <strong>{status}</strong> &nbsp;|&nbsp;
    Platform: <strong>{platform}</strong>
  </p>
  {status_msg}
  <form method="POST">
    <h2>Spec YAML</h2>
    <p>You may edit the YAML directly below, then click Approve or Request Changes.</p>
    <textarea name="spec_yaml" rows="40">{spec_yaml_escaped}</textarea>

    <div class="feedback-area" id="feedback-area">
      <h3>Describe your changes</h3>
      <textarea name="feedback" rows="4" placeholder="Describe what you changed or want changed..."></textarea>
    </div>

    <p>
      <button type="submit" name="action" value="approve" class="btn btn-approve">
        ✅ Approve &amp; Create
      </button>
      <button type="submit" name="action" value="request_changes" class="btn btn-revise"
              onclick="document.getElementById('feedback-area').style.display='block'">
        ✏️ Request Changes
      </button>
    </p>
  </form>

  <script>
    // Show feedback area immediately if action was already selected
    document.querySelectorAll('button[value=request_changes]').forEach(function(btn) {{
      btn.addEventListener('click', function() {{
        document.getElementById('feedback-area').style.display = 'block';
      }});
    }});
  </script>
</body>
</html>
"""

_SPEC_SUBMITTED_HTML = """\
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>DevTrack — Spec {action}</title>
<style>body{{font-family:sans-serif;max-width:600px;margin:3em auto;color:#222;}}
.ok{{color:#27ae60;}} .info{{color:#2980b9;}}</style>
</head>
<body>
  <h1 class="{css_class}">{heading}</h1>
  <p>{message}</p>
  <p><a href="/spec/{spec_id}/review">← Back to spec</a></p>
</body>
</html>
"""


@app.get("/spec/{spec_id}/review")
async def spec_review_form(spec_id: str):
    """Render the spec review/edit form for the PM."""
    from fastapi.responses import HTMLResponse
    import html as html_mod

    try:
        from backend.project_spec.spec_store import SpecStore
        store = SpecStore()
        spec = await store.load(spec_id)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Could not load spec: {e}")

    if not spec:
        raise HTTPException(status_code=404, detail=f"Spec '{spec_id}' not found")

    status_msg = ""
    if spec.status == "approved":
        status_msg = '<p class="success">✅ This spec has been approved and items have been created.</p>'
    elif spec.status == "pending_review":
        status_msg = '<p class="info">⏳ Awaiting your review and approval.</p>'

    spec_yaml_escaped = html_mod.escape(spec.to_yaml())
    body = _SPEC_REVIEW_HTML.format(
        spec_id=spec_id,
        status=spec.status,
        platform=spec.pm_platform,
        status_msg=status_msg,
        spec_yaml_escaped=spec_yaml_escaped,
    )
    return HTMLResponse(content=body)


@app.post("/spec/{spec_id}/review")
async def spec_review_submit(spec_id: str, request: Request):
    """Handle spec approval or change request from the web form."""
    from fastapi.responses import HTMLResponse
    import yaml as _yaml

    form = await request.form()
    action = form.get("action", "")
    feedback = str(form.get("feedback", "")).strip()
    spec_yaml_str = str(form.get("spec_yaml", "")).strip()

    try:
        from backend.project_spec.spec_store import SpecStore
        from backend.project_spec.spec_generator import ProjectSpec
        store = SpecStore()
        spec = await store.load(spec_id)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

    if not spec:
        raise HTTPException(status_code=404, detail=f"Spec '{spec_id}' not found")

    if action == "approve":
        # If PM edited the YAML, save updated version first
        if spec_yaml_str:
            try:
                updated_dict = _yaml.safe_load(spec_yaml_str)
                updated = ProjectSpec.from_dict(updated_dict)
                updated.spec_id = spec_id   # preserve ID
                spec = updated
            except Exception as e:
                logger.warning(f"spec_review_submit: invalid YAML edit: {e}")

        await store.update_status(spec_id, "approved", feedback="Approved via web form")

        # Trigger creation in background
        import asyncio
        from backend.project_spec.project_creator import ProjectCreator
        creator = ProjectCreator()
        asyncio.create_task(creator.create(spec))

        body = _SPEC_SUBMITTED_HTML.format(
            action="Approved",
            spec_id=spec_id,
            css_class="ok",
            heading="✅ Spec Approved",
            message="DevTrack is now creating sprints, epics, and stories in your PM tool.",
        )

    elif action == "request_changes":
        if spec_yaml_str:
            try:
                updated_dict = _yaml.safe_load(spec_yaml_str)
                updated = ProjectSpec.from_dict(updated_dict)
                updated.spec_id = spec_id
                spec = updated
                await store.save(spec)
            except Exception as e:
                logger.warning(f"spec_review_submit: invalid YAML on change request: {e}")

        await store.update_status(
            spec_id, "pending_review",
            feedback=feedback or "Changes requested via web form",
            changed_by=spec.pm_email,
        )
        body = _SPEC_SUBMITTED_HTML.format(
            action="Changes Requested",
            spec_id=spec_id,
            css_class="info",
            heading="✏️ Changes Noted",
            message="Your feedback has been saved. The spec has been updated.",
        )

    else:
        raise HTTPException(status_code=400, detail=f"Unknown action: {action}")

    return HTMLResponse(content=body)


@app.get("/status")
async def status() -> dict:
    return {
        "service": "devtrack-webhooks",
        "azure_devops": _cfg_bool("AZURE_SYNC_ENABLED"),
        "webhook_enabled": _cfg_bool("WEBHOOK_ENABLED"),
        "notify_os": _cfg_bool("WEBHOOK_NOTIFY_OS", True),
        "notify_terminal": _cfg_bool("WEBHOOK_NOTIFY_TERMINAL", True),
    }


# ---------------------------------------------------------------------------
# Phase 1: Pending-actions queue endpoints
# Auth: same X-DevTrack-API-Key as all /trigger/* endpoints.
# The Go daemon's queue executor calls these on every poll cycle.
# ---------------------------------------------------------------------------

def _get_queue_gateway():
    """Return a QueueGateway for the shared DevTrack DB.

    Instantiated per-request (lightweight: holds no connection of its own —
    each QueueGateway method opens the shared engine per call). Falls back
    to None on import error so the server degrades gracefully; individual
    database operations are handled by each endpoint's own try/except.
    """
    try:
        from backend.queue_gateway import QueueGateway
        return QueueGateway()
    except Exception:
        return None


@app.get("/queue/pending")
async def http_queue_pending(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Return all pending actions ordered by expires_at ASC.

    Used by the Go daemon's queue executor to decide which actions to
    auto-approve.  Returns an empty list when the queue is unavailable.

    Response: ``{"actions": [<pending_actions rows as JSON objects>]}``
    """
    gw = _get_queue_gateway()
    if gw is None:
        logger.debug("/queue/pending: queue gateway unavailable")
        return {"actions": []}
    try:
        actions = gw.list_pending()
        return {"actions": actions}
    except Exception as exc:
        logger.warning("/queue/pending error: %s", exc)
        return {"actions": []}
    finally:
        try:
            gw.close()
        except Exception:
            pass


@app.post("/queue/execute")
async def http_queue_execute(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Execute a staged pending action and mark it posted or failed.

    Called by the Go daemon's queue executor after a confidence timeout
    expires (auto-approve) or when the developer manually approves via
    TUI / CLI / Telegram.

    Request body: ``{"action_id": <int>}``

    Response: ``{"status": "posted"|"failed", "error": "<msg if failed>"}``
    """
    try:
        body = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON body")

    action_id = body.get("action_id")
    if action_id is None:
        raise HTTPException(status_code=400, detail="'action_id' field required")

    gw = _get_queue_gateway()
    if gw is None:
        raise HTTPException(status_code=503, detail="Queue gateway unavailable (DB not found)")

    try:
        action = gw.get_action(int(action_id))
        if action is None:
            raise HTTPException(status_code=404, detail=f"Action {action_id} not found")

        processor = TriggerProcessor.get()
        result = await asyncio.to_thread(processor._execute_pm_action, action)

        if result.get("status") == "posted":
            gw.mark_posted(int(action_id))
        else:
            gw.mark_failed(int(action_id), result.get("error", "unknown error"))

        return result
    except HTTPException:
        raise
    except Exception as exc:
        logger.warning("/queue/execute error (action_id=%s): %s", action_id, exc)
        try:
            if gw:
                gw.mark_failed(int(action_id), str(exc))
        except Exception:
            pass
        return {"status": "failed", "error": str(exc)}
    finally:
        try:
            if gw:
                gw.close()
        except Exception:
            pass


@app.post("/queue/execute_staged")
async def http_queue_execute_staged(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Execute an action that already passed the Go client's local queue.

    Client and server databases have independent numeric IDs in PostgreSQL
    mode. The full immutable routing payload therefore crosses the HTTP
    boundary only after local approval/expiry instead of looking up an
    unrelated server row by ID.
    """
    try:
        action = await request.json()
    except Exception as exc:
        raise HTTPException(status_code=400, detail="Invalid JSON body") from exc
    if not isinstance(action, dict):
        raise HTTPException(status_code=400, detail="JSON body must be an object")
    required = (
        "id", "action_type", "target", "platform", "workspace", "payload", "confidence"
    )
    missing = [field for field in required if field not in action]
    if missing:
        raise HTTPException(
            status_code=400, detail=f"missing required field(s): {', '.join(missing)}"
        )
    if (
        not isinstance(action["id"], int)
        or isinstance(action["id"], bool)
        or action["id"] <= 0
    ):
        raise HTTPException(status_code=400, detail="id must be a positive integer")
    string_fields = ("action_type", "target", "platform", "workspace", "payload")
    if not all(isinstance(action[field], str) for field in string_fields):
        raise HTTPException(status_code=400, detail="action routing fields must be strings")
    if not all(action[field].strip() for field in ("action_type", "target", "platform")):
        raise HTTPException(status_code=400, detail="action routing fields cannot be empty")
    confidence = action["confidence"]
    if (
        not isinstance(confidence, (int, float))
        or isinstance(confidence, bool)
        or not 0 <= confidence <= 1
    ):
        raise HTTPException(status_code=400, detail="confidence must be between 0 and 1")
    try:
        decoded_payload = json.loads(action["payload"])
    except (TypeError, ValueError) as exc:
        raise HTTPException(status_code=400, detail="payload must contain valid JSON") from exc
    if not isinstance(decoded_payload, dict):
        raise HTTPException(status_code=400, detail="payload JSON must be an object")
    return await asyncio.to_thread(TriggerProcessor.get()._execute_pm_action, action)


# ---------------------------------------------------------------------------
# Phase 7: PR review comment classification endpoint
# Auth: same X-DevTrack-API-Key as all /trigger/* endpoints.
# The Go client calls this after detecting a new review comment.
# ---------------------------------------------------------------------------

@app.post("/review/classify")
async def http_review_classify(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Classify a PR review comment as auto_fixable or needs_human.

    Called by the Go daemon after detecting a new review comment on a
    developer-authored PR.  Uses the configured LLM (same pipeline as
    commit message enhancement). Falls back to needs_human on any LLM
    failure — safe default, never auto-fixes without confidence.

    Request body:
    {
      "comment_body": "...",
      "pr_title":     "...",
      "platform":     "github"|"azure"|"gitlab",
      "comment_url":  "..."  // optional
    }

    Response:
    {
      "classification": "auto_fixable"|"needs_human",
      "reason":         "...",
      "fix_hint":       "..."
    }
    """
    try:
        body = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON body")

    comment_body = body.get("comment_body", "")
    pr_title = body.get("pr_title", "")
    platform = body.get("platform", "")

    try:
        from backend.review_classifier import ReviewClassifier
        classifier = ReviewClassifier()
        result = classifier.classify(comment_body, pr_title, platform)
        return result
    except Exception as exc:
        logger.warning("/review/classify unexpected error: %s", exc)
        # Final safety net — never propagate errors to the Go client.
        return {
            "classification": "needs_human",
            "reason": "Server error during classification.",
            "fix_hint": "",
        }


# ---------------------------------------------------------------------------
# Phase 6: Dialectic self-improvement endpoint
# Auth: same X-DevTrack-API-Key as all /trigger/* endpoints.
# The Go client calls this after successful queue execution or approve/reject.
# ---------------------------------------------------------------------------

@app.post("/dialectic/infer")
async def http_dialectic_infer(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Run a dialectic reasoning pass and return inferences about the developer.

    Called by the Go client after a successful queue execution, TUI approval,
    or TUI rejection. The Go client stores the returned inferences in SQLite
    via InsertInference() — Python does not write to SQLite directly.

    Request body:
    {
      "interaction_type": "commit" | "approval" | "rejection" | "edit",
      "context_type": "commit" | "comment" | "report" | "task" | "ticket_mapping",
      "before_text": "...",
      "after_text": "...",
      "metadata": {"ticket_id": "...", "workspace": "...", "action_id": 42, ...}
    }

    Response: {"inferences": [{"subject": "...", "inference": "...", "confidence": 0.75}]}
    Returns {"inferences": []} (not an error) when both Hermes 3 and fallback fail.
    """
    try:
        body = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON body")

    interaction_type = body.get("interaction_type", "")
    context_type = body.get("context_type", "")
    before_text = body.get("before_text", "")
    after_text = body.get("after_text", "")
    metadata = body.get("metadata", {})
    if not isinstance(metadata, dict):
        metadata = {}

    try:
        from backend.dialectic_reasoner import DialecticReasoner
        reasoner = DialecticReasoner()
        inferences = await asyncio.to_thread(
            reasoner.reason,
            interaction_type,
            context_type,
            before_text,
            after_text,
            metadata,
        )
    except Exception as exc:
        logger.warning("/dialectic/infer: DialecticReasoner raised unexpectedly: %s", exc)
        inferences = []

    return {"inferences": inferences}


# ---------------------------------------------------------------------------
# HTTP trigger endpoints (CS-1: Go → HTTPS → Python, all modes)
# All endpoints require X-DevTrack-API-Key (when DEVTRACK_API_KEY is set).
# ---------------------------------------------------------------------------

@app.post("/trigger/commit")
async def http_commit_trigger(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Commit trigger from Go daemon. Payload matches CommitTriggerData."""
    data = await request.json()
    result = await asyncio.to_thread(TriggerProcessor.get().process_commit, data)
    return {"status": "ok", **result}


@app.post("/trigger/timer")
async def http_timer_trigger(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Timer trigger from Go daemon. Payload matches TimerTriggerData."""
    data = await request.json()
    result = await asyncio.to_thread(TriggerProcessor.get().process_timer, data)
    return result


@app.post("/trigger/workspace_reload")
async def http_workspace_reload(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Reload workspace router after workspaces.yaml is modified."""
    try:
        TriggerProcessor.get()._init_components()
        logger.info("Workspace router reloaded")
    except Exception as e:
        logger.warning(f"Workspace reload failed: {e}")
    return {"status": "ok", "message": "workspace router reloaded"}


@app.post("/trigger/shutdown")
async def http_shutdown(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Graceful shutdown — daemon is stopping."""
    logger.info("Shutdown signal received from Go daemon")
    # Schedule actual process exit after the response is sent
    import threading
    from backend.config import get_shutdown_grace_period_seconds as _grace
    threading.Timer(_grace(), lambda: os.kill(os.getpid(), signal.SIGTERM)).start()
    return {"status": "ok"}


@app.post("/trigger/ping")
async def http_ping(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Liveness check used by Go health monitor."""
    return {"status": "ok", "pong": True}


@app.post("/trigger/client/heartbeat")
async def http_client_heartbeat(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Client registration heartbeat — upserts connected client info in admin DB."""
    data = await request.json()
    client_id = data.get("client_id", "unknown")
    version = data.get("version", "")
    tls_enabled = bool(data.get("tls_enabled", False))
    workspaces = data.get("workspaces", [])
    ip = request.client.host if request.client else ""
    try:
        from backend.admin.user_manager import upsert_client, prune_stale_clients
        upsert_client(client_id, version, tls_enabled, workspaces, ip)
        prune_stale_clients(stale_minutes=10)
    except Exception as exc:
        logger.warning(f"client heartbeat failed: {exc}")
    return {"status": "ok"}


@app.post("/trigger/client_events")
async def http_client_events(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Persist an opt-in batch of replay-safe Go-client event snapshots."""
    try:
        data = await request.json()
    except Exception as exc:
        raise HTTPException(status_code=400, detail="Invalid JSON body") from exc
    if not isinstance(data, dict):
        raise HTTPException(status_code=400, detail="JSON body must be an object")
    events = data.get("events")
    if not isinstance(events, list) or not events:
        raise HTTPException(status_code=400, detail="'events' must be a non-empty list")
    if len(events) > 1000:
        raise HTTPException(status_code=413, detail="event batch exceeds 1000 rows")
    try:
        from backend.db.client_event_store import persist_client_events

        accepted = await asyncio.to_thread(
            persist_client_events, data.get("client_id", ""), events
        )
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return {"status": "ok", "accepted": accepted}


@app.post("/trigger/work_session_start")
async def http_work_session_start(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Notifies Python that a work session has started in Go."""
    data = await request.json()
    session_id = data.get("session_id")
    ticket_ref = data.get("ticket_ref", "")
    logger.info(f"Work session #{session_id} started (ticket={ticket_ref})")
    return {"status": "ok", "session_id": session_id}


@app.post("/trigger/work_session_stop")
async def http_work_session_stop(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Notifies Python that a work session has stopped in Go."""
    data = await request.json()
    session_id = data.get("session_id")
    logger.info(f"Work session #{session_id} stopped")
    return {"status": "ok", "session_id": session_id}


@app.post("/trigger/ticket_sync")
async def http_ticket_sync(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Receive a slim ticket list from the Go client and upsert into ticket_cache.

    Go calls this after each github-sync / azure-sync / gitlab-sync (and on the
    periodic background sync).  Python uses ticket_cache for fuzzy/semantic
    matching during commit and timer triggers.

    force=true  → drop all existing entries for the source (and repo if given)
                  before inserting the fresh list.  Use for force-refresh.
    force=false → upsert only: add new + update existing, keep stale entries.
    """
    data = await request.json()
    source    = data.get("source", "")
    workspace = data.get("workspace", "")
    force     = bool(data.get("force", False))
    synced_at = data.get("synced_at", "")
    tickets   = data.get("tickets", [])

    if not source:
        raise HTTPException(status_code=400, detail="source is required")

    # Critical-path: exceptions propagate through _stage() so FailureOccurred
    # fires in narrative.log with OllamaFailureAnalyzer diagnosis.
    # Go handles non-2xx gracefully (logs + skips push, local cache intact).
    from backend.db.ticket_db import TicketDB
    with TicketDB.from_config() as db:
        if force:
            with _stage("Force clear cache"):
                repos = {t.get("repo", "") for t in tickets if t.get("repo")}
                if repos:
                    for repo in repos:
                        deleted = db.clear_by_source(source, repo)
                        logger.info(f"ticket_sync: force-cleared {deleted} rows for {source}:{repo}")
                else:
                    deleted = db.clear_by_source(source)
                    logger.info(f"ticket_sync: force-cleared {deleted} rows for source={source}")

        with _stage(f"Upsert {len(tickets)} {source} tickets"):
            for ticket in tickets:
                db.upsert_ticket({
                    "id":          ticket.get("id", ""),
                    "source":      source,
                    "external_id": ticket.get("external_id", ""),
                    "repo":        ticket.get("repo", ""),
                    "title":       ticket.get("title", ""),
                    "description": ticket.get("description", ""),
                    "status":      ticket.get("status", ""),
                    "assignee":    ticket.get("assignee", ""),
                    "url":         ticket.get("url", ""),
                    "synced_at":   synced_at,
                })

    logger.info(
        f"ticket_sync: upserted {len(tickets)} tickets "
        f"(source={source}, workspace={workspace}, force={force})"
    )
    return {"status": "ok", "upserted": len(tickets), "source": source, "narrative_id": _story_id()}


# ---------------------------------------------------------------------------
# PM Agent — plan preview & create
# ---------------------------------------------------------------------------

try:
    from backend.pm_agent import PMAgent, DecompositionPlan
    from backend.plan_parser import parse_plan_file, parse_plan_folder, PlanParseError
    import dataclasses, json as _json
    _pm_agent_available = True
except ImportError as _pm_err:
    _pm_agent_available = False
    logger.warning(f"PM agent not available: {_pm_err}")


@app.post("/trigger/plan/preview")
async def http_plan_preview(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """
    Decompose a problem into an Epic→Story→Task plan and return a preview.

    Accepts either:
      { "problem": "...", "platform": "azure", "project_context": "...", "notes": "..." }
    or:
      { "markdown": "<full plan file contents>", "platform": "azure" }

    Returns:
      { "preview": "...", "plan_token": "<json>", "total_count": N,
        "epic_count": N, "story_count": N, "task_count": N }
    """
    if not _pm_agent_available:
        raise HTTPException(status_code=503, detail="PM agent dependencies not installed")

    data = await request.json()

    markdown = data.get("markdown", "").strip()
    if markdown:
        # Parse structured markdown sent by the CLI
        try:
            from backend.plan_parser import _parse_content
            parsed = _parse_content(markdown)
            problem = parsed.to_problem_statement()
            platform = data.get("platform") or parsed.platform or "azure"
            project_context = parsed.project
        except PlanParseError as exc:
            raise HTTPException(status_code=400, detail=str(exc))
    else:
        problem = data.get("problem", "").strip()
        if not problem:
            raise HTTPException(status_code=400, detail="'problem' or 'markdown' field required")
        platform = data.get("platform", "azure")
        project_context = data.get("project_context") or None

    notes = data.get("notes", "")
    if notes:
        problem = f"{problem}\n\nAdditional constraints:\n{notes}"

    logger.info(f"Plan preview requested — platform={platform}, problem_len={len(problem)}")

    try:
        agent = PMAgent(platform=platform, project_context=project_context)
        plan = agent.decompose(problem)
    except Exception as exc:
        logger.exception("Plan decomposition failed")
        raise HTTPException(status_code=500, detail=f"Decomposition failed: {exc}")

    preview = agent.format_preview(plan)

    # Serialise plan to a token so the CLI can pass it back for creation
    plan_dict = dataclasses.asdict(plan)
    plan_token = base64.b64encode(_json.dumps(plan_dict).encode()).decode()

    return {
        "preview": preview,
        "plan_token": plan_token,
        "total_count": plan.total_count,
        "epic_count": plan.epic_count,
        "story_count": plan.story_count,
        "task_count": plan.task_count,
        "platform": platform,
    }


@app.post("/trigger/plan/create")
async def http_plan_create(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """
    Execute plan creation from a plan_token returned by /trigger/plan/preview.

    Accepts: { "plan_token": "<base64 plan json>" }

    Returns:
      { "created": [ { "title": ..., "platform_id": ..., "platform_url": ... } ],
        "failed":  [ { "title": ..., "error": ... } ] }
    """
    if not _pm_agent_available:
        raise HTTPException(status_code=503, detail="PM agent dependencies not installed")

    data = await request.json()
    token = data.get("plan_token", "")
    if not token:
        raise HTTPException(status_code=400, detail="'plan_token' field required")

    try:
        plan_dict = _json.loads(base64.b64decode(token.encode()).decode())
        # Reconstruct DecompositionPlan + WorkItemNode objects
        from backend.pm_agent import WorkItemNode
        items = [WorkItemNode(**it) for it in plan_dict.pop("items", [])]
        plan = DecompositionPlan(**plan_dict, items=items)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Invalid plan_token: {exc}")

    logger.info(f"Plan create requested — platform={plan.platform}, items={plan.total_count}")

    progress_log: list[str] = []

    async def on_progress(node, status: str) -> None:
        progress_log.append(f"{node.title}: {status}")
        logger.debug(f"Plan progress: {node.title} → {status}")

    try:
        agent = PMAgent(platform=plan.platform)
        created, failed = await agent.create_all(plan, on_progress=on_progress)
    except Exception as exc:
        logger.exception("Plan creation failed")
        raise HTTPException(status_code=500, detail=f"Creation failed: {exc}")

    return {
        "created": [
            {
                "title": n.title,
                "item_type": n.item_type,
                "level": n.level,
                "platform_id": n.platform_id,
                "platform_url": n.platform_url,
            }
            for n in created
        ],
        "failed": [
            {"title": n.title, "error": err}
            for n, err in failed
        ],
        "progress": progress_log,
    }


# ---------------------------------------------------------------------------
# Boardroom — multi-persona plan review
# ---------------------------------------------------------------------------

try:
    from backend.boardroom.session import BoardroomSession
    from backend.boardroom.report import format_terminal, format_markdown
    from backend.boardroom.interactive import (
        select_responders,
        generate_persona_response,
        generate_final_summary,
    )
    _boardroom_available = True
except ImportError as _br_err:
    _boardroom_available = False
    logger.warning(f"Boardroom not available: {_br_err}")


@app.post("/trigger/boardroom")
async def http_boardroom(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """
    Run a full boardroom review on a plan.

    Accepts:
      { "plan_text": "...", "output_format": "terminal"|"markdown" }
    or:
      { "markdown": "<plan file contents>", "output_format": "terminal"|"markdown" }

    Returns:
      { "report": "<formatted report string>",
        "verdict": "PROCEED"|"REVISE"|"RECONSIDER",
        "approve": N, "revise": N, "reject": N }
    """
    if not _boardroom_available:
        raise HTTPException(status_code=503, detail="Boardroom dependencies not installed")

    data = await request.json()
    output_format = data.get("output_format", "terminal")

    markdown_src = data.get("markdown", "").strip()
    if markdown_src:
        try:
            from backend.plan_parser import _parse_content
            parsed = _parse_content(markdown_src)
            plan_text = parsed.to_problem_statement()
        except Exception as exc:
            raise HTTPException(status_code=400, detail=f"Plan parse error: {exc}")
    else:
        plan_text = data.get("plan_text", "").strip()
        if not plan_text:
            raise HTTPException(status_code=400, detail="'plan_text' or 'markdown' field required")

    logger.info(f"Boardroom session starting — plan_len={len(plan_text)}")

    session = BoardroomSession()
    report = await session.run(plan_text)

    if output_format == "markdown":
        report_str = format_markdown(report)
    else:
        report_str = format_terminal(report)

    return {
        "report": report_str,
        "verdict": report.verdict,
        "verdict_summary": report.verdict_summary,
        "approve": report.approve_count,
        "revise": report.revise_count,
        "reject": report.reject_count,
        "pros": report.pros,
        "cons": report.cons,
    }


@app.post("/trigger/boardroom/chat")
async def http_boardroom_chat(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """
    One turn of an interactive boardroom conversation.

    Accepts:
      {
        "plan_text":     "<problem/plan description>",
        "history":       [ {role, content, persona_id?, persona_name?}, ... ],
        "user_message":  "<what the user just typed>",
        "addressed_to":  "<persona_id or null>",   // e.g. "security" if user typed @sam
        "final_say":     "<text or null>"           // set to end the session
      }

    Returns:
      {
        "responses": [ {persona_id, persona_name, role, content}, ... ],
        "updated_history": [ ... ],        // full history including this turn
        "session_closed": false,
        "closing_summary": null
      }
    """
    if not _boardroom_available:
        raise HTTPException(status_code=503, detail="Boardroom dependencies not installed")

    data = await request.json()
    plan_text    = data.get("plan_text", "")
    history      = data.get("history", [])
    user_message = data.get("user_message", "").strip()
    addressed_to = data.get("addressed_to") or None
    final_say    = data.get("final_say") or None

    if not plan_text:
        raise HTTPException(status_code=400, detail="'plan_text' is required")

    from backend.llm import get_provider
    provider = get_provider()

    # ── Final-say path: close the session ───────────────────────────────────
    if final_say:
        closing = await asyncio.to_thread(
            generate_final_summary, provider, plan_text, history, final_say
        )
        # Append final say + closing to history
        updated = list(history)
        updated.append({"role": "user", "content": f"[Final say] {final_say}"})
        updated.append({"role": "system", "content": f"[Closing summary] {closing}"})
        return {
            "responses": [],
            "updated_history": updated,
            "session_closed": True,
            "closing_summary": closing,
        }

    if not user_message:
        raise HTTPException(status_code=400, detail="'user_message' or 'final_say' required")

    # ── Normal turn: select responders ──────────────────────────────────────
    responder_ids = await asyncio.to_thread(
        select_responders, provider, history, user_message, addressed_to
    )

    from backend.boardroom.personas import PERSONAS as _PERSONAS
    id_to_persona = {p.id: p for p in _PERSONAS}

    # ── Generate responses in parallel ──────────────────────────────────────
    # Append user message to history first so personas see it in context
    turn_history = list(history) + [{"role": "user", "content": user_message}]

    async def _get_response(persona_id: str):
        persona = id_to_persona.get(persona_id)
        if not persona:
            return None
        content = await asyncio.to_thread(
            generate_persona_response,
            provider, persona, turn_history, user_message, plan_text,
        )
        return {
            "persona_id": persona.id,
            "persona_name": persona.name,
            "role": persona.role,
            "content": content,
        }

    raw_responses = await asyncio.gather(*[_get_response(rid) for rid in responder_ids])
    responses = [r for r in raw_responses if r and r["content"]]

    # Build updated history
    updated = list(turn_history)
    for r in responses:
        updated.append({
            "role": "persona",
            "persona_id": r["persona_id"],
            "persona_name": r["persona_name"],
            "content": r["content"],
        })

    return {
        "responses": responses,
        "updated_history": updated,
        "session_closed": False,
        "closing_summary": None,
    }


# ---------------------------------------------------------------------------
# Phase 1c: Reports, Learning, Auth, License — previously uv run in client
# All routes use the same _verify_trigger_key dependency.
# ---------------------------------------------------------------------------

import io
from contextlib import redirect_stdout

# ── Reports ─────────────────────────────────────────────────────────────────

try:
    from backend.email_reporter import EmailReporter as _EmailReporter
    _email_reporter_available = True
except Exception as _er_err:
    _email_reporter_available = False
    logger.warning(f"EmailReporter not available: {_er_err}")

try:
    from backend.daily_report_generator import DailyReportGenerator as _DailyReportGenerator
    _daily_report_generator_available = True
except Exception as _drg_err:
    _daily_report_generator_available = False
    logger.warning(f"DailyReportGenerator not available: {_drg_err}")


@app.post("/reports/preview")
async def http_report_preview(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Generate a daily report preview and return formatted text."""
    if not _email_reporter_available:
        raise HTTPException(status_code=503, detail="EmailReporter not available")
    data = await request.json()
    date_str = data.get("date") or None
    try:
        from datetime import datetime as _dt
        date_obj = _dt.strptime(date_str, "%Y-%m-%d") if date_str else None
        reporter = _EmailReporter()
        report = await asyncio.to_thread(reporter.generate_daily_report, date_obj)
        output = reporter.format_report_text(report, style="professional")
        return {"output": output, "success": True}
    except Exception as exc:
        logger.error(f"/reports/preview failed: {exc}")
        return {"output": str(exc), "success": False}


@app.post("/reports/send")
async def http_report_send(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Send a daily report to the given email address."""
    if not _email_reporter_available:
        raise HTTPException(status_code=503, detail="EmailReporter not available")
    data = await request.json()
    email = data.get("email", "")
    date_str = data.get("date") or None
    if not email:
        raise HTTPException(status_code=400, detail="'email' field required")
    try:
        from datetime import datetime as _dt
        date_obj = _dt.strptime(date_str, "%Y-%m-%d") if date_str else None
        reporter = _EmailReporter()
        report = await asyncio.to_thread(reporter.generate_daily_report, date_obj)
        buf = io.StringIO()
        with redirect_stdout(buf):
            await reporter.send_email_report(email, report)
        return {"output": buf.getvalue() or f"Report sent to {email}", "success": True}
    except Exception as exc:
        logger.error(f"/reports/send failed: {exc}")
        return {"output": str(exc), "success": False}


@app.post("/reports/save")
async def http_report_save(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Save a daily report to disk and return the path."""
    if not _email_reporter_available:
        raise HTTPException(status_code=503, detail="EmailReporter not available")
    data = await request.json()
    date_str = data.get("date") or None
    try:
        from datetime import datetime as _dt
        date_obj = _dt.strptime(date_str, "%Y-%m-%d") if date_str else None
        reporter = _EmailReporter()
        report = await asyncio.to_thread(reporter.generate_daily_report, date_obj)
        buf = io.StringIO()
        with redirect_stdout(buf):
            await asyncio.to_thread(reporter.save_report, report)
        return {"output": buf.getvalue(), "success": True}
    except Exception as exc:
        logger.error(f"/reports/save failed: {exc}")
        return {"output": str(exc), "success": False}


@app.post("/reports/eod")
async def http_report_eod(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Run the EOD report generator (Phase 4 — commit-grouped narrative).

    Uses DailyReportGenerator.generate_eod_narrative() which queries the
    triggers table, groups commits by ticket_id, generates per-ticket LLM
    narratives in the developer's voice, and applies inject_style.
    Falls back gracefully if the generator is unavailable.
    """
    from datetime import date as _date
    data = await request.json()
    date_str = data.get("date") or None
    email = data.get("email", "")
    workspace = data.get("workspace", "all")
    supplied_commits = data.get("commits")
    if not isinstance(supplied_commits, list) or not all(
        isinstance(row, dict) for row in supplied_commits
    ):
        supplied_commits = None
    elif len(supplied_commits) > 1000:
        supplied_commits = supplied_commits[:1000]
    try:
        if not _daily_report_generator_available:
            narrative = "No commits recorded today."
        else:
            gen = _DailyReportGenerator()
            narrative = await asyncio.to_thread(
                gen.generate_eod_narrative, date_str, supplied_commits
            )

        # Phase 4 — Non-Negotiable #2: every outbound action stages through
        # pending_actions before touching any external system.
        action_id = None
        gw = _get_queue_gateway()
        if gw is not None:
            try:
                from backend.config import get_eod_report_confidence
                action_id = gw.stage(
                    action_type="eod_report",
                    target="developer",
                    platform="email",
                    workspace=workspace,
                    payload={
                        "narrative": narrative,
                        "email": email or "",
                        "date": date_str or str(_date.today()),
                    },
                    confidence=get_eod_report_confidence(),
                    is_new_action_type=False,
                )
                logger.info(
                    "/reports/eod: staged eod_report action %s (email=%s)",
                    action_id, email or "none",
                )
            except Exception as stage_exc:
                logger.warning("/reports/eod: queue staging failed (non-fatal): %s", stage_exc)
        else:
            logger.debug("/reports/eod: queue gateway unavailable — action not staged")

        return {"output": narrative, "success": True, "action_id": action_id}
    except Exception as exc:
        logger.error(f"/reports/eod failed: {exc}")
        return {"output": str(exc), "success": False, "narrative": ""}


# ── Voice seeding (Phase 5 — Tier 0) ─────────────────────────────────────────


@app.post("/voice/seed")
async def http_voice_seed(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Seed ChromaDB with commit messages from a git repository.

    Accepts: {"repo_path": "...", "since_months": N, "force": false}
    Returns: {"embedded": N, "skipped": N, "repo_path": "..."}

    When force=false (default) the seeder skips repos that already have
    >= 10 corpus entries for that repo_path — idempotent auto-start behaviour.
    """
    data = await request.json()
    repo_path: str = data.get("repo_path", "")
    force: bool = bool(data.get("force", False))

    try:
        from backend.config import get_voice_seed_months
        since_months: int = int(data.get("since_months") or get_voice_seed_months())
    except Exception:
        since_months = 6

    if not repo_path:
        return {"embedded": 0, "skipped": 0, "repo_path": repo_path, "error": "repo_path is required"}

    # Threshold check: skip if corpus already has >= 10 entries for this repo,
    # unless force=true is requested.
    if not force:
        try:
            from backend.rag.vector_store import VectorStore as _VS
            store = _VS()
            # Count docs tagged to this repo_path via metadata filter.
            # VectorStore.count() returns total collection size, not per-repo.
            # Use a heuristic: if total >= 10 assume this repo has been seeded.
            total = store.count()
            if total >= 10:
                logger.info(
                    "/voice/seed: corpus already has %d entries — skipping (use force=true to override)",
                    total,
                )
                return {"embedded": 0, "skipped": -1, "repo_path": repo_path}
        except Exception as chk_exc:
            logger.debug("/voice/seed: threshold check failed (non-fatal): %s", chk_exc)

    try:
        from backend.voice_seeder import VoiceSeeder
        seeder = VoiceSeeder()
        embedded = await asyncio.to_thread(seeder.seed_from_git, repo_path, since_months)
        logger.info("/voice/seed: embedded %d messages from %s", embedded, repo_path)
        return {"embedded": embedded, "skipped": 0, "repo_path": repo_path}
    except Exception as exc:
        logger.error("/voice/seed failed: %s", exc)
        return {"embedded": 0, "skipped": 0, "repo_path": repo_path, "error": str(exc)}



# ── Voice profile generation (Phase 5 — Tier 0 dialectic) ─────────────────────


@app.post("/voice/profile/generate")
async def http_voice_profile_generate(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Generate a Developer Voice Profile from the ChromaDB commit corpus.

    Accepts: {"repo_paths": ["...", "..."]}  (optional)
    Returns: {"path": "<absolute path to profile.md>", "word_count": N}

    If repo_paths is omitted, the endpoint generates a profile from all
    available commit messages in the ChromaDB collection (no repo filtering).
    """
    data: dict = {}
    try:
        data = await request.json()
    except Exception:
        pass

    repo_paths: list[str] = data.get("repo_paths") or []

    try:
        from backend.voice_profile import ProfileGenerator
        from backend.config import get_path

        gen = ProfileGenerator()
        profile_text = await asyncio.to_thread(gen.generate, repo_paths)

        data_dir_path = get_path("DATA_DIR")
        saved_path = await asyncio.to_thread(gen.save, profile_text, str(data_dir_path))

        word_count = len(profile_text.split())
        logger.info("/voice/profile/generate: saved %d-word profile to %s", word_count, saved_path)
        return {"path": str(saved_path), "word_count": word_count}
    except Exception as exc:
        logger.error("/voice/profile/generate failed: %s", exc)
        return {"path": "", "word_count": 0, "error": str(exc)}


# ── Voice sync (Phase 5 — Tier 1) ────────────────────────────────────────────


@app.post("/voice/sync")
async def http_voice_sync(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """POST /voice/sync — Background sync of PR descriptions and issue comments.

    Polls all configured PM workspaces (or a subset when workspace_names is given)
    for PR bodies and issue comments authored by the developer and embeds them into
    ChromaDB. Returns per-platform counts.

    Accepts: {"workspace_names": ["..."]}  — optional; syncs all when omitted.
    Returns: {"synced": {"github": N, "azure": N, "gitlab": N, "total": N}}
    """
    try:
        data = await request.json()
    except Exception:
        data = {}

    workspace_names: list[str] = data.get("workspace_names", [])

    try:
        from backend.config import get_workspaces_file, get_path
        from backend.voice_sync import VoiceSync
    except ImportError as exc:
        logger.error("/voice/sync: import failed: %s", exc)
        return {"synced": {"github": 0, "azure": 0, "gitlab": 0, "total": 0}, "error": str(exc)}

    # Load workspace configs from workspaces.yaml (or fall back to empty list).
    workspaces: list = []
    try:
        import yaml
        ws_file = get_workspaces_file()
        if ws_file:
            ws_path = get_path("WORKSPACES_FILE") if ws_file else None
        else:
            # Try default location relative to PROJECT_ROOT.
            project_root = get_path("PROJECT_ROOT") if True else None
            ws_path = project_root / "workspaces.yaml" if project_root else None

        if ws_path and ws_path.exists():
            with open(ws_path) as fh:
                raw = yaml.safe_load(fh) or {}
            from backend.config import get as cfg_get
            raw_list = raw.get("workspaces", [])
            # Build simple namespace objects compatible with VoiceSync expectations.
            import types
            for ws_dict in raw_list:
                if not isinstance(ws_dict, dict):
                    continue
                ws_obj = types.SimpleNamespace(**ws_dict)
                # Normalise attribute names (workspaces.yaml uses snake_case).
                ws_obj.pm_platform = getattr(ws_obj, "pm_platform", "")
                ws_obj.pm_username = getattr(ws_obj, "pm_username", "")
                ws_obj.pm_org = getattr(ws_obj, "pm_org", "")
                ws_obj.pm_project = getattr(ws_obj, "pm_project", "")
                ws_obj.name = getattr(ws_obj, "name", "")
                workspaces.append(ws_obj)
    except Exception as load_exc:
        logger.warning("/voice/sync: could not load workspaces.yaml: %s", load_exc)

    # Filter to requested workspace_names when provided.
    if workspace_names:
        workspaces = [
            ws for ws in workspaces
            if getattr(ws, "name", "") in workspace_names
        ]

    syncer = VoiceSync()
    totals = await asyncio.to_thread(syncer.sync_all, workspaces)
    logger.info("/voice/sync: synced %s", totals)
    return {"synced": totals}


# ── Voice add / status (Phase 5 — Tier 2) ────────────────────────────────────

# Allowed context types for voice corpus entries.
_VOICE_CONTEXT_TYPES = {"commit", "description", "comment", "report", "task"}


@app.post("/voice/add")
async def http_voice_add(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Inject a manual high-weight writing example into ChromaDB.

    Accepts: {"text": "...", "context_type": "commit|description|comment|report|task"}
    Returns: {"id": "<chroma_document_id>"}
             or {"id": "", "error": "chromadb unavailable"} with HTTP 503
             or HTTP 400/422 on validation failure.

    Entries are tagged with source=manual and weight=high in ChromaDB metadata.
    """
    data: dict = {}
    try:
        data = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Request body must be valid JSON")

    text: str = data.get("text", "").strip()
    context_type: str = data.get("context_type", "").strip()

    if not text:
        raise HTTPException(status_code=400, detail="text is required and must not be empty")

    if not context_type:
        raise HTTPException(
            status_code=422,
            detail=f"context_type is required; allowed values: {sorted(_VOICE_CONTEXT_TYPES)}",
        )

    if context_type not in _VOICE_CONTEXT_TYPES:
        raise HTTPException(
            status_code=422,
            detail=f"invalid context_type {context_type!r}; allowed: {sorted(_VOICE_CONTEXT_TYPES)}",
        )

    try:
        from backend.rag.embedder import embed as rag_embed
        from backend.rag.vector_store import VectorStore

        # Build a unique document ID for this manual entry.
        import hashlib
        import time as _time
        doc_id = "manual-" + hashlib.sha1(
            f"{context_type}:{text}:{_time.time()}".encode()
        ).hexdigest()[:16]

        embedded_text = f"Context: {text}\nResponse: {text}"
        vec = await asyncio.to_thread(rag_embed, embedded_text)
        if vec is None:
            logger.warning("/voice/add: embedding model unavailable — ChromaDB not updated")
            return JSONResponse(
                status_code=503,
                content={"id": "", "error": "chromadb unavailable"},
            )

        store = VectorStore()
        metadata = {
            "source": "manual",
            "weight": "high",
            "context_type": context_type,
            "trigger": text[:300],
            "response": text[:400],
        }
        success = store.upsert(doc_id, embedded_text, vec, metadata)
        if not success:
            logger.warning("/voice/add: VectorStore.upsert returned False — ChromaDB may be unavailable")
            return JSONResponse(
                status_code=503,
                content={"id": "", "error": "chromadb unavailable"},
            )

        logger.info("/voice/add: embedded manual entry id=%s context_type=%s", doc_id, context_type)
        return {"id": doc_id}

    except Exception as exc:
        logger.warning("/voice/add: unexpected error: %s", exc)
        return JSONResponse(
            status_code=503,
            content={"id": "", "error": "chromadb unavailable"},
        )


@app.get("/voice/status")
async def http_voice_status(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Return voice corpus statistics.

    Response shape:
    {
      "total_entries": 127,
      "by_context": {"commit": 95, "description": 18, "comment": 14, "report": 0, "task": 0},
      "by_source":  {"git_history": 95, "pr_sync": 32, "manual": 0},
      "last_seed":  "2026-06-18T10:00:00Z" | null,
      "last_sync":  "2026-06-18T08:00:00Z" | null,
      "profile_exists":    true,
      "profile_word_count": 312
    }
    Never raises; returns zeros/nulls for missing data.
    """
    # ── ChromaDB counts ───────────────────────────────────────────────────────
    by_context: dict = {ct: 0 for ct in sorted(_VOICE_CONTEXT_TYPES)}
    by_source: dict = {"git_history": 0, "pr_sync": 0, "manual": 0}
    total_entries: int = 0

    try:
        from backend.rag.vector_store import VectorStore
        store = VectorStore()
        if store._init():  # noqa: SLF001 — internal init returns bool, safe here
            try:
                # Retrieve all documents with their metadata.
                # ChromaDB .get() with no filters returns everything.
                result = store._collection.get(include=["metadatas"])  # noqa: SLF001
                metadatas = result.get("metadatas") or []
                total_entries = len(metadatas)
                for meta in metadatas:
                    ct = meta.get("context_type", "")
                    if ct in by_context:
                        by_context[ct] += 1
                    src = meta.get("source", "")
                    if src in by_source:
                        by_source[src] += 1
            except Exception as qe:
                logger.debug("/voice/status: ChromaDB count query failed (non-fatal): %s", qe)
    except Exception as ce:
        logger.debug("/voice/status: ChromaDB init failed (non-fatal): %s", ce)

    # ── last_seed from voice_seeded_commits ───────────────────────────────────
    last_seed: str | None = None
    try:
        from backend.db.voice_seed_store import latest_seeded_at
        last_seed = latest_seeded_at()
    except Exception as se:
        logger.debug("/voice/status: last_seed query failed (non-fatal): %s", se)

    # ── last_sync from voice_synced_items (TASK-082) ─────────────────────────
    last_sync: str | None = None
    try:
        from backend.db.voice_sync_store import latest_synced_at
        last_sync = latest_synced_at()
    except Exception as sye:
        logger.debug("/voice/status: last_sync query failed (non-fatal): %s", sye)

    # ── profile file ─────────────────────────────────────────────────────────
    profile_exists: bool = False
    profile_word_count: int = 0
    try:
        from backend.config import get_path
        profile_path = get_path("DATA_DIR") / "learning" / "profile.md"
        if profile_path.exists():
            profile_exists = True
            profile_word_count = len(profile_path.read_text(encoding="utf-8").split())
    except Exception as pe:
        logger.debug("/voice/status: profile read failed (non-fatal): %s", pe)

    # ── dialectic Phase 6 data ────────────────────────────────────────────────
    inferences_data: dict = {"total": 0, "top_by_confidence": [], "correction_count": 0}
    skills_data: dict = {"total": 0, "names": []}
    thresholds_data: dict = {}
    try:
        from backend.dialectic_status import DialecticStatus
        _ds = DialecticStatus()
        inferences_data = _ds.get_inference_summary()
        skills_data = _ds.get_skill_summary()
        thresholds_data = _ds.get_threshold_summary()
    except Exception as de:
        logger.debug("/voice/status: dialectic data failed (non-fatal): %s", de)

    return {
        "total_entries": total_entries,
        "by_context": by_context,
        "by_source": by_source,
        "last_seed": last_seed,
        "last_sync": last_sync,
        "profile_exists": profile_exists,
        "profile_word_count": profile_word_count,
        "inferences": inferences_data,
        "skills": skills_data,
        "thresholds": thresholds_data,
    }


# ── Learning ─────────────────────────────────────────────────────────────────

try:
    from backend.learning_integration import LearningIntegration as _LearningIntegration
    _learning_available = True
except Exception as _li_err:
    _learning_available = False
    logger.warning(f"LearningIntegration not available: {_li_err}")


def _run_learning(fn, *args, **kwargs) -> str:
    """Run a LearningIntegration method, capturing stdout, return output string."""
    buf = io.StringIO()
    with redirect_stdout(buf):
        fn(*args, **kwargs)
    return buf.getvalue()


@app.get("/learning/status")
async def http_learning_status(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Return learning consent/sample status as JSON."""
    if not _learning_available:
        return {"enabled": False, "consent_given": False, "sample_count": 0,
                "last_updated": "", "success": True}
    try:
        li = _LearningIntegration()
        status = await asyncio.to_thread(li.get_status)
        return {"enabled": status.get("enabled", False),
                "consent_given": status.get("consent_given", False),
                "sample_count": status.get("sample_count", 0),
                "last_updated": status.get("last_updated", ""),
                "success": True}
    except Exception as exc:
        logger.error(f"/learning/status failed: {exc}")
        return {"enabled": False, "consent_given": False, "sample_count": 0,
                "last_updated": "", "success": False, "error": str(exc)}


@app.post("/learning/enable")
async def http_learning_enable(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Enable personalized AI learning."""
    if not _learning_available:
        raise HTTPException(status_code=503, detail="Learning not available")
    data = await request.json()
    days = int(data.get("days", 30))
    try:
        li = _LearningIntegration()
        output = await asyncio.to_thread(_run_learning, li.enable, days)
        return {"output": output, "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


@app.post("/learning/sync")
async def http_learning_sync(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Run a delta (or full) learning sync."""
    if not _learning_available:
        raise HTTPException(status_code=503, detail="Learning not available")
    data = await request.json()
    full = bool(data.get("full", False))
    try:
        li = _LearningIntegration()
        output = await asyncio.to_thread(_run_learning, li.sync, full)
        return {"output": output, "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


@app.post("/learning/reset")
async def http_learning_reset(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Wipe all learning data."""
    if not _learning_available:
        raise HTTPException(status_code=503, detail="Learning not available")
    try:
        li = _LearningIntegration()
        output = await asyncio.to_thread(_run_learning, li.reset)
        return {"output": output, "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


@app.post("/learning/cron/setup")
async def http_learning_cron_setup(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Install the learning cron entry."""
    if not _learning_available:
        raise HTTPException(status_code=503, detail="Learning not available")
    try:
        li = _LearningIntegration()
        output = await asyncio.to_thread(_run_learning, li.setup_cron)
        return {"output": output, "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


@app.delete("/learning/cron")
async def http_learning_cron_remove(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Remove the learning cron entry."""
    if not _learning_available:
        raise HTTPException(status_code=503, detail="Learning not available")
    try:
        li = _LearningIntegration()
        output = await asyncio.to_thread(_run_learning, li.remove_cron)
        return {"output": output, "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


@app.get("/learning/cron/status")
async def http_learning_cron_status(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Return cron entry status."""
    if not _learning_available:
        raise HTTPException(status_code=503, detail="Learning not available")
    try:
        li = _LearningIntegration()
        output = await asyncio.to_thread(_run_learning, li.cron_status)
        return {"output": output, "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


@app.get("/learning/profile")
async def http_learning_profile(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Return the current learning profile as text."""
    if not _learning_available:
        raise HTTPException(status_code=503, detail="Learning not available")
    try:
        li = _LearningIntegration()
        output = await asyncio.to_thread(_run_learning, li.show_profile)
        return {"output": output, "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


@app.post("/learning/test-response")
async def http_learning_test_response(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Generate a personalized test response."""
    if not _learning_available:
        raise HTTPException(status_code=503, detail="Learning not available")
    data = await request.json()
    text = data.get("text", "")
    if not text:
        raise HTTPException(status_code=400, detail="'text' field required")
    try:
        li = _LearningIntegration()
        output = await asyncio.to_thread(_run_learning, li.test_response, text)
        return {"output": output, "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


@app.post("/learning/revoke")
async def http_learning_revoke(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Revoke learning consent."""
    if not _learning_available:
        raise HTTPException(status_code=503, detail="Learning not available")
    try:
        li = _LearningIntegration()
        output = await asyncio.to_thread(_run_learning, li.revoke_consent)
        return {"output": output, "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


# ── Auth ─────────────────────────────────────────────────────────────────────

try:
    from backend.auth.cloud_auth import request_magic_link as _req_magic, verify_magic_link as _verify_magic
    from backend.auth.session import (
        get_session as _get_session, is_logged_in as _is_logged_in,
        clear_session as _clear_session, set_session as _set_session,
    )
    _auth_available = True
except Exception as _auth_err:
    _auth_available = False
    logger.warning(f"Auth not available: {_auth_err}")


@app.post("/auth/request-magic-link")
async def http_auth_request_magic_link(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Send a magic-link login code to the given email."""
    if not _auth_available:
        raise HTTPException(status_code=503, detail="Auth not available")
    data = await request.json()
    email = data.get("email", "")
    if not email:
        raise HTTPException(status_code=400, detail="'email' field required")
    success, msg = await asyncio.to_thread(_req_magic, email)
    return {"success": success, "message": msg}


@app.post("/auth/verify-magic-link")
async def http_auth_verify_magic_link(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Verify a magic-link code and return session data."""
    if not _auth_available:
        raise HTTPException(status_code=503, detail="Auth not available")
    data = await request.json()
    email = data.get("email", "")
    code = data.get("code", "")
    if not email or not code:
        raise HTTPException(status_code=400, detail="'email' and 'code' fields required")
    success, msg, session = await asyncio.to_thread(_verify_magic, email, code)
    if not success or session is None:
        return {"success": False, "message": msg}
    _set_session(session)
    return {
        "success": True,
        "message": msg,
        "email": session.email,
        "display_name": session.display_name,
        "tier": session.tier,
        "mode": session.mode,
        "telemetry_enabled": session.telemetry_enabled,
        "token_expires_at": str(session.token_expires_at) if session.token_expires_at else "",
    }


@app.post("/auth/logout")
async def http_auth_logout(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Clear the local session."""
    if not _auth_available:
        raise HTTPException(status_code=503, detail="Auth not available")
    try:
        if _is_logged_in():
            _clear_session()
            return {"success": True, "message": "Logged out successfully."}
        return {"success": True, "message": "Not currently logged in."}
    except Exception as exc:
        return {"success": False, "message": str(exc)}


@app.get("/auth/whoami")
async def http_auth_whoami(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Return current session info."""
    if not _auth_available:
        return {"logged_in": False, "success": True}
    try:
        if not _is_logged_in():
            return {"logged_in": False, "success": True}
        s = _get_session()
        return {
            "logged_in": True,
            "success": True,
            "email": s.email,
            "display_name": s.display_name,
            "tier": s.tier,
            "mode": s.mode,
            "telemetry_enabled": s.telemetry_enabled,
            "token_expires_at": str(s.token_expires_at) if s.token_expires_at else "",
        }
    except Exception as exc:
        return {"logged_in": False, "success": False, "error": str(exc)}


@app.post("/auth/telemetry")
async def http_auth_telemetry(
    request: Request,
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Enable, disable, or query telemetry status."""
    if not _auth_available:
        raise HTTPException(status_code=503, detail="Auth not available")
    data = await request.json()
    action = data.get("action", "status")  # "on" | "off" | "status"
    try:
        if not _is_logged_in():
            if action == "status":
                return {"success": True, "message": "Telemetry: disabled (not logged in)",
                        "telemetry_enabled": False}
            return {"success": False, "message": "Telemetry requires login. Run: devtrack login"}
        s = _get_session()
        if action == "on":
            s.telemetry_enabled = True
            _set_session(s)
            return {"success": True, "message": "Telemetry enabled.", "telemetry_enabled": True}
        elif action == "off":
            s.telemetry_enabled = False
            _set_session(s)
            return {"success": True, "message": "Telemetry disabled.", "telemetry_enabled": False}
        else:
            status = "enabled" if s.telemetry_enabled else "disabled"
            return {"success": True, "message": f"Telemetry: {status}",
                    "telemetry_enabled": s.telemetry_enabled}
    except Exception as exc:
        return {"success": False, "message": str(exc)}


# ── License ───────────────────────────────────────────────────────────────────

try:
    from backend.license_manager import (
        is_accepted as _lic_is_accepted,
        ensure_accepted as _lic_ensure_accepted,
        show_license_status as _lic_show_status,
        show_terms as _lic_show_terms,
        TERMS_VERSION as _TERMS_VERSION,
    )
    _license_available = True
except Exception as _lic_err:
    _license_available = False
    logger.warning(f"LicenseManager not available: {_lic_err}")


@app.get("/license/check")
async def http_license_check(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Return whether the current terms version has been accepted."""
    if not _license_available:
        return {"accepted": True, "success": True}  # fail open
    try:
        accepted = await asyncio.to_thread(_lic_is_accepted)
        return {"accepted": accepted, "success": True}
    except Exception as exc:
        return {"accepted": True, "success": False, "error": str(exc)}  # fail open


@app.get("/license/status")
async def http_license_status(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Return formatted license status text."""
    if not _license_available:
        raise HTTPException(status_code=503, detail="License manager not available")
    try:
        buf = io.StringIO()
        with redirect_stdout(buf):
            await asyncio.to_thread(_lic_show_status)
        return {"output": buf.getvalue(), "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


@app.get("/license/terms")
async def http_license_terms(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Return the Terms of Service text."""
    if not _license_available:
        raise HTTPException(status_code=503, detail="License manager not available")
    try:
        buf = io.StringIO()
        with redirect_stdout(buf):
            await asyncio.to_thread(_lic_show_terms)
        return {"output": buf.getvalue(), "success": True}
    except Exception as exc:
        return {"output": str(exc), "success": False}


@app.post("/license/accept")
async def http_license_accept(
    _auth: None = Depends(_verify_trigger_key),
) -> dict:
    """Accept the current terms of service."""
    if not _license_available:
        raise HTTPException(status_code=503, detail="License manager not available")
    try:
        accepted = await asyncio.to_thread(_lic_ensure_accepted, True)
        return {"accepted": accepted, "success": True,
                "message": "Terms accepted." if accepted else "Terms not accepted."}
    except Exception as exc:
        return {"accepted": False, "success": False, "error": str(exc)}


# ---------------------------------------------------------------------------
# Startup
# ---------------------------------------------------------------------------

async def _ensure_gitlab_webhooks() -> None:
    """Register GitLab webhooks for all project IDs listed in GITLAB_PROJECT_IDS."""
    try:
        from backend import config as _cfg
        project_ids_raw = _cfg.get("GITLAB_PROJECT_IDS", "")
        gitlab_pat = _cfg.get("GITLAB_PAT", "")
        webhook_url = _cfg.get("DEVTRACK_WEBHOOK_PUBLIC_URL", "")
        webhook_secret = _cfg.get("WEBHOOK_GITLAB_SECRET", "")

        if not gitlab_pat or not project_ids_raw or not webhook_url:
            logger.debug("GitLab webhook auto-registration skipped (GITLAB_PAT, GITLAB_PROJECT_IDS, or DEVTRACK_WEBHOOK_PUBLIC_URL not set)")
            return

        from backend.gitlab.client import GitLabClient
        client = GitLabClient()
        if not client.is_configured():
            return

        full_url = webhook_url.rstrip("/") + "/webhooks/gitlab"
        project_ids = [p.strip() for p in project_ids_raw.split(",") if p.strip()]
        for pid_str in project_ids:
            try:
                pid = int(pid_str)
                await client.ensure_webhook_current(pid, full_url, webhook_secret)
            except ValueError:
                logger.warning(f"GitLab webhook auto-reg: invalid project ID '{pid_str}'")
            except Exception as e:
                logger.warning(f"GitLab webhook auto-reg: project {pid_str} failed: {e}")
        await client.close()
    except Exception as e:
        logger.debug(f"GitLab webhook auto-registration error: {e}")


def main() -> None:
    """Entry point when run as a module."""
    import uvicorn

    host = _cfg("WEBHOOK_HOST", "0.0.0.0")
    port = int(_cfg("WEBHOOK_PORT", "8089"))

    # TLS: Go passes DEVTRACK_TLS_CERT / DEVTRACK_TLS_KEY via the subprocess env.
    # If TLS is enabled and both paths are present, start uvicorn with SSL.
    tls_enabled = config.get_devtrack_tls_enabled() if config else True
    cert_path = config.get_devtrack_tls_cert() if config else ""
    key_path = config.get_devtrack_tls_key() if config else ""

    ssl_kwargs: dict = {}
    if tls_enabled and cert_path and key_path:
        if os.path.exists(cert_path) and os.path.exists(key_path):
            ssl_kwargs = {"ssl_certfile": cert_path, "ssl_keyfile": key_path}
            logger.info(f"TLS enabled — cert: {cert_path}")
        else:
            logger.warning("TLS cert/key paths set but files not found — starting without TLS")

    scheme = "https" if ssl_kwargs else "http"
    logger.info(f"Starting {scheme} server on {host}:{port}")

    uvicorn.run(
        "backend.webhook_server:app",
        host=host,
        port=port,
        log_level="warning",
        access_log=False,
        log_config=_UVICORN_LOG_CONFIG,
        **ssl_kwargs,
    )


if __name__ == "__main__":
    main()
