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
    from backend.config import is_ai_available
    logger.info("feature:ai %s", "enabled" if is_ai_available() else "disabled (run: devtrack-server enable ai)")
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
        # Queue gateway — instantiated after components so database_path() is
        # available.  Wrapped in try/except so the server degrades gracefully
        # when the DB is absent (e.g. first-run before the Go daemon creates it).
        self._queue_gateway = None
        try:
            from backend.config import database_path
            from backend.queue_gateway import QueueGateway
            db_path = str(database_path())
            import os
            if os.path.exists(db_path):
                self._queue_gateway = QueueGateway(db_path)
                logger.info("✓ TriggerProcessor: QueueGateway ready (db=%s)", db_path)
            else:
                logger.debug(
                    "QueueGateway: DB not found at %s — queue staging disabled "
                    "(daemon not yet started?)", db_path
                )
        except Exception as e:
            logger.debug("QueueGateway unavailable (non-fatal): %s", e)

    def _init_components(self) -> None:
        # NLP parser
        self.nlp_parser = None
        try:
            from backend.nlp_parser import NLPTaskParser
            self.nlp_parser = NLPTaskParser(use_ollama=True)
            logger.info("✓ TriggerProcessor: NLP parser ready")
        except Exception as e:
            logger.debug(f"NLP parser unavailable: {e}")

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

        if not self.workspace_router:
            return {"status": "failed", "error": "workspace_router not available"}

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
        missing), the method falls back to the legacy direct-post behaviour
        so existing functionality is not broken during the transition.
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
        # This is the authoritative ticket-resolution signal — NLP's own guess
        # (task_data.get("ticket_id")) is no longer used to locate the ticket,
        # only to enrich the description/comment text.
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

        # NLP parse
        task_data = None
        with _stage("NLP parse"):
            if self.nlp_parser and commit_msg:
                try:
                    task_data = self.nlp_parser.parse(commit_msg, repo_path=repo_path)
                except Exception as e:
                    logger.warning(f"NLP parse failed: {e}")

        # PM sync — stage via queue (Phase 1) or fall back to direct post
        with _stage("PM sync"):
            if not resolved_ticket_id:
                # Phase 2 found no ticket for this commit (branch/message/
                # active-ticket fallback chain all came up empty — logged
                # [UNLINKED] on the Go side). Non-Negotiable #8: never block,
                # never error. We do not fall back to the old NLP-guess or
                # truncated-commit-hash target — if Go says unlinked, treat
                # it as unlinked here too.
                logger.info(
                    "PM sync skipped: commit %s has no resolved ticket_id "
                    "(Phase 2 unlinked) — no queue action staged",
                    commit_hash[:12] if commit_hash else "?",
                )
            elif self.workspace_router:
                # task_data may legitimately be None here (NLP parser
                # unavailable, e.g. spaCy not installed, or parse() raised —
                # see "NLP parse" stage above). resolved_ticket_id is the
                # authoritative signal for *targeting*; task_data is only
                # optional descriptive enrichment, so every read of it below
                # must fall back to commit_msg / "" without raising.
                status = task_data.get("status", "") if task_data else ""

                # Phase 3 (TASK-072): generate a voice-aware ticket comment via
                # the LLM pipeline, not a raw NLP restatement. Falls back to a
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
                    # task_data.get("description") NLP restatement as last resort
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
                # Phase 3: ticket_id resolution confidence now comes from Go's
                # extraction strategy, not NLP's own (weaker) guess. NLP match
                # only affects descriptive quality, not target confidence.
                # Baseline reflects Phase 2's verified ~100% hit rate for
                # resolved IDs.
                confidence = 0.85

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
                        return {
                            "actions":    actions,
                            "commit_hash": commit_hash,
                            "narrative_id": _story_id(),
                            "status":     "queued",
                            "action_id":  action_id,
                        }
                    except Exception as e:
                        logger.warning(
                            "PM sync staging failed — falling back to direct post: %s", e
                        )
                        # Fall through to legacy direct-post below

                # Legacy direct-post fallback (queue gateway unavailable)
                try:
                    self.workspace_router.route(
                        pm_platform=pm_platform,
                        description=pm_payload["description"],
                        ticket_id=pm_payload["ticket_id"],
                        status=pm_payload["status"],
                        pm_project=pm_project,
                        pm_assignee=pm_assignee,
                        pm_iteration_path=pm_iteration_path,
                        pm_area_path=pm_area_path,
                        pm_milestone=pm_milestone,
                        commit_info=pm_payload["commit_info"],
                    )
                    actions.append(f"pm_sync:{pm_platform or 'auto'}")
                    logger.info(f"✓ PM sync complete (platform={pm_platform or 'auto'})")
                except Exception as e:
                    logger.warning(f"PM sync failed: {e}")

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

    Instantiated per-request (lightweight: just opens a SQLite connection).
    Falls back to None when the DB is absent so the server degrades gracefully.
    """
    try:
        from backend.config import database_path
        from backend.queue_gateway import QueueGateway
        db_path = str(database_path())
        import os
        if not os.path.exists(db_path):
            return None
        return QueueGateway(db_path)
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
    from backend.work_tracker.eod_report_generator import EODReportGenerator as _EODGenerator
    _eod_generator_available = True
except Exception as _eod_err:
    _eod_generator_available = False
    logger.warning(f"EODReportGenerator not available: {_eod_err}")


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
    """Run the EOD report generator (used by the daemon scheduler)."""
    if not _eod_generator_available:
        raise HTTPException(status_code=503, detail="EODReportGenerator not available")
    data = await request.json()
    email = data.get("email") or None
    date_str = data.get("date") or None
    try:
        gen = _EODGenerator()
        report = await gen.generate(target_date=date_str)
        total_h, total_m = divmod(report.total_minutes, 60)
        lines = [
            f"EOD Report — {report.date}",
            f"Total: {total_h}h {total_m}m across {len(report.sessions)} session(s)",
        ]
        if report.narrative:
            lines.append("")
            lines.append(report.narrative)
        output = "\n".join(lines)
        return {"output": output, "success": True}
    except Exception as exc:
        logger.error(f"/reports/eod failed: {exc}")
        return {"output": str(exc), "success": False}


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
