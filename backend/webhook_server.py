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
    from runtime_narrative import RuntimeNarrativeMiddleware
    app.add_middleware(RuntimeNarrativeMiddleware)
    logger.info("runtime-narrative middleware enabled for webhook server")
except (ImportError, TypeError):
    pass

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
    logger.warning("Admin embed skipped: %s", _exc)

_handler: WebhookEventHandler | None = None

# ---------------------------------------------------------------------------
# Trigger processor (CS-1: HTTP trigger endpoints for external/remote mode)
# ---------------------------------------------------------------------------

class TriggerProcessor:
    """
    Processes commit and timer triggers received via HTTP POST.

    Used when DEVTRACK_SERVER_MODE=external — Go POSTs to /trigger/commit
    and /trigger/timer instead of sending over IPC.  Mirrors the component
    initialization in DevTrackBridge (python_bridge.py) but without an IPC
    connection.  All imports are lazy and guarded so missing deps degrade
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
    # Commit trigger
    # ------------------------------------------------------------------

    def process_commit(self, data: dict) -> dict:
        """
        Process a commit trigger.  Returns a dict of actions taken.
        Mirrors handle_commit_trigger() in DevTrackBridge minus the IPC ack.
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

        logger.info(f"[HTTP commit] {commit_hash[:12]} — {commit_msg[:60]}")

        actions: list[str] = []

        # Auto-link commit to active work session
        if commit_hash:
            try:
                from backend.work_tracker.session_store import WorkSessionStore
                store   = WorkSessionStore()
                active  = store.get_active_session()
                if active:
                    store.append_commit(active["id"], commit_hash)
                    actions.append(f"session_linked:{active['id']}")
                    logger.info(f"📎 Commit {commit_hash[:12]} linked to session #{active['id']}")
            except Exception as e:
                logger.debug(f"Work session link failed (non-fatal): {e}")

        # NLP parse
        task_data = None
        if self.nlp_parser and commit_msg:
            try:
                task_data = self.nlp_parser.parse(commit_msg, repo_path=repo_path)
            except Exception as e:
                logger.warning(f"NLP parse failed: {e}")

        # PM sync via workspace router
        if task_data and self.workspace_router:
            try:
                self.workspace_router.route(
                    pm_platform=pm_platform,
                    description=task_data.get("description", commit_msg),
                    ticket_id=task_data.get("ticket_id", ""),
                    status=task_data.get("status", ""),
                    pm_project=pm_project,
                    pm_assignee=pm_assignee,
                    pm_iteration_path=pm_iteration_path,
                    pm_area_path=pm_area_path,
                    pm_milestone=pm_milestone,
                    commit_info={
                        "hash": commit_hash,
                        "message": commit_msg,
                        "author": author,
                        "branch": branch,
                    },
                )
                actions.append(f"pm_sync:{pm_platform or 'auto'}")
                logger.info(f"✓ PM sync complete (platform={pm_platform or 'auto'})")
            except Exception as e:
                logger.warning(f"PM sync failed: {e}")

        return {"actions": actions, "commit_hash": commit_hash}

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
        """
        interval_mins  = data.get("interval_mins", 0)
        trigger_count  = data.get("trigger_count", 0)
        pm_platform    = data.get("pm_platform", "")
        workspace_name = data.get("workspace_name", "")

        logger.info(f"[HTTP timer] trigger #{trigger_count} (every {interval_mins}m, workspace={workspace_name})")

        # Check vacation mode — auto-respond instead of nudging
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
        try:
            from backend.work_tracker.session_store import WorkSessionStore
            active_session = WorkSessionStore().get_active_session()
        except Exception as e:
            logger.debug(f"Work session check failed: {e}")

        # Attempt Telegram nudge (non-fatal)
        telegram_sent = False
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

        return {
            "status": "accepted",
            "trigger_count": trigger_count,
            "prompt_channel": ",".join(channels) if channels else "none",
            "active_session": active_session is not None,
        }


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
        log_level="info",
        **ssl_kwargs,
    )


if __name__ == "__main__":
    main()
