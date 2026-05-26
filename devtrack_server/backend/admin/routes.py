"""
Admin Console FastAPI router — mounted at /admin in the admin app.

All routes require a valid session cookie (see auth.py).
Process control actions delegate to server_tui.process_monitor.
"""
from __future__ import annotations

import os
from pathlib import Path
from typing import Optional

from fastapi import APIRouter, Cookie, Depends, Form, HTTPException, Request, Response, status
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates

from backend.admin.auth import (
    COOKIE_NAME,
    check_credentials,
    create_token,
    decode_token,
    require_auth,
)
from backend.admin.server_status import get_snapshot
from backend.admin.user_manager import (
    create_api_key,
    create_user,
    delete_user,
    disable_user,
    enable_user,
    ensure_default_admin,
    get_audit_log,
    get_user,
    init_db,
    list_api_keys,
    list_users,
    log_action,
    revoke_api_key,
    touch_last_login,
    update_password,
    update_role,
    verify_user_password,
)

router = APIRouter()

_TEMPLATES_DIR = Path(__file__).parent / "templates"
templates = Jinja2Templates(directory=str(_TEMPLATES_DIR))


# ---------------------------------------------------------------------------
# Startup initialisation (called from app.py)
# ---------------------------------------------------------------------------

def startup() -> None:
    init_db()
    from backend.config import get_admin_username, get_admin_password
    admin_user = get_admin_username()
    admin_pass = get_admin_password()
    if admin_pass:
        ensure_default_admin(admin_user, admin_pass)


from backend.config import (
    get_admin_session_hours as _get_admin_session_hours,
    get_webhook_port as _get_webhook_port,
    get_admin_port as _get_admin_port,
    get_stats_refresh_interval_seconds as _get_stats_refresh_secs,
    get_process_refresh_interval_seconds as _get_process_refresh_secs,
    get_audit_log_limit as _get_audit_log_limit,
    get_license_contact_email as _get_license_contact_email,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _ctx(request: Request, current_user: str, active: str, **extra) -> dict:
    return {"request": request, "current_user": current_user, "active": active, **extra}


def _snapshot_ctx():
    """Return a fresh server snapshot (non-blocking; errors are silenced)."""
    try:
        return get_snapshot()
    except Exception:
        from backend.admin.server_status import ServerSnapshot
        try:
            _wport = _get_webhook_port()
        except ValueError:
            _wport = 0
        try:
            _aport = _get_admin_port()
        except ValueError:
            _aport = 0
        return ServerSnapshot(processes=[], services=[], llm_provider="—",
                              llm_model="—", webhook_port=_wport, admin_port=_aport)


# ---------------------------------------------------------------------------
# Auth routes
# ---------------------------------------------------------------------------

@router.get("/login", response_class=HTMLResponse)
async def login_page(request: Request):
    return templates.TemplateResponse("login.html", {"request": request, "error": None})


@router.post("/login")
async def login(
    request: Request,
    response: Response,
    username: str = Form(...),
    password: str = Form(...),
):
    if not check_credentials(username, password):
        return templates.TemplateResponse(
            "login.html",
            {"request": request, "error": "Invalid username or password"},
            status_code=401,
        )
    touch_last_login(username)
    log_action(username, "login", ip=request.client.host if request.client else "")
    token = create_token(username)
    resp = RedirectResponse("/admin/", status_code=303)
    resp.set_cookie(COOKIE_NAME, token, httponly=True, samesite="lax",
                    max_age=_get_admin_session_hours() * 3600)
    return resp


@router.get("/logout")
async def logout(current_user: str = Depends(require_auth)):
    resp = RedirectResponse("/admin/login", status_code=303)
    resp.delete_cookie(COOKIE_NAME)
    return resp


# ---------------------------------------------------------------------------
# Dashboard
# ---------------------------------------------------------------------------

def _trigger_stats_ctx():
    """Return TriggerStats; degrades to zero-valued instance on any error."""
    try:
        from backend.server_tui.stats_client import get_trigger_stats, TriggerStats
        return get_trigger_stats()
    except Exception:
        try:
            from backend.server_tui.stats_client import TriggerStats
            return TriggerStats()
        except Exception:
            return None


@router.get("/", response_class=HTMLResponse)
async def dashboard(request: Request, current_user: str = Depends(require_auth)):
    snapshot = _snapshot_ctx()
    user_count = len(list_users())
    try:
        from backend.license_manager import detect_tier, get_tier_label
        tier = detect_tier(user_count)
        tier_label = get_tier_label(tier)
    except Exception:
        tier = "personal"
        tier_label = "Personal (Free)"
    stats = _trigger_stats_ctx()
    try:
        stats_refresh_secs = _get_stats_refresh_secs()
    except ValueError:
        stats_refresh_secs = 30
    try:
        process_refresh_secs = _get_process_refresh_secs()
    except ValueError:
        process_refresh_secs = 15
    return templates.TemplateResponse(
        "dashboard.html",
        _ctx(
            request, current_user, "dashboard",
            snapshot=snapshot,
            user_count=user_count,
            tier=tier,
            tier_label=tier_label,
            stats=stats,
            stats_refresh_secs=stats_refresh_secs,
            process_refresh_secs=process_refresh_secs,
        ),
    )


# ---------------------------------------------------------------------------
# HTMX partials
# ---------------------------------------------------------------------------

@router.get("/_partials/stats", response_class=HTMLResponse)
async def partial_stats(request: Request, current_user: str = Depends(require_auth)):
    stats = _trigger_stats_ctx()
    return templates.TemplateResponse(
        "_stats_panel.html",
        {"request": request, "stats": stats},
    )


@router.get("/_partials/processes", response_class=HTMLResponse)
async def partial_processes(request: Request, current_user: str = Depends(require_auth)):
    snapshot = _snapshot_ctx()
    return templates.TemplateResponse(
        "_proc_rows.html",
        {"request": request, "snapshot": snapshot},
    )


# ---------------------------------------------------------------------------
# Process control (HTMX POST — returns updated proc rows)
# ---------------------------------------------------------------------------

@router.post("/process/{name}/restart", response_class=HTMLResponse)
async def process_restart(
    name: str, request: Request, current_user: str = Depends(require_auth)
):
    try:
        from backend.server_tui.process_monitor import ProcessMonitor
        mon = ProcessMonitor()
        mon.restart(name)
        log_action(current_user, "restart_process", name,
                   ip=request.client.host if request.client else "")
    except Exception:
        pass
    snapshot = _snapshot_ctx()
    return templates.TemplateResponse("_proc_rows.html", {"request": request, "snapshot": snapshot})


@router.post("/process/{name}/stop", response_class=HTMLResponse)
async def process_stop(
    name: str, request: Request, current_user: str = Depends(require_auth)
):
    try:
        from backend.server_tui.process_monitor import ProcessMonitor
        mon = ProcessMonitor()
        mon.stop(name)
        log_action(current_user, "stop_process", name,
                   ip=request.client.host if request.client else "")
    except Exception:
        pass
    snapshot = _snapshot_ctx()
    return templates.TemplateResponse("_proc_rows.html", {"request": request, "snapshot": snapshot})


@router.post("/process/{name}/start", response_class=HTMLResponse)
async def process_start(
    name: str, request: Request, current_user: str = Depends(require_auth)
):
    try:
        from backend.server_tui.process_monitor import ProcessMonitor
        mon = ProcessMonitor()
        mon.restart(name)  # restart handles start of stopped process
        log_action(current_user, "start_process", name,
                   ip=request.client.host if request.client else "")
    except Exception:
        pass
    snapshot = _snapshot_ctx()
    return templates.TemplateResponse("_proc_rows.html", {"request": request, "snapshot": snapshot})


# ---------------------------------------------------------------------------
# Users
# ---------------------------------------------------------------------------

@router.get("/users", response_class=HTMLResponse)
async def users_page(
    request: Request,
    current_user: str = Depends(require_auth),
    flash: Optional[str] = None,
    flash_type: str = "info",
):
    users = list_users()
    return templates.TemplateResponse(
        "users.html",
        _ctx(request, current_user, "users", users=users, flash=flash, flash_type=flash_type),
    )


@router.post("/users/create")
async def users_create(
    request: Request,
    current_user: str = Depends(require_auth),
    username: str = Form(...),
    password: str = Form(...),
    role: str = Form("viewer"),
):
    if get_user(username):
        return RedirectResponse(
            f"/admin/users?flash=User+'{username}'+already+exists&flash_type=error",
            status_code=303,
        )
    create_user(username, password, role)
    log_action(current_user, "create_user", f"username={username} role={role}",
               ip=request.client.host if request.client else "")
    return RedirectResponse(f"/admin/users?flash=User+'{username}'+created", status_code=303)


@router.post("/users/{username}/delete")
async def users_delete(
    username: str, request: Request, current_user: str = Depends(require_auth)
):
    if username == current_user:
        return RedirectResponse(
            "/admin/users?flash=Cannot+delete+yourself&flash_type=error", status_code=303
        )
    delete_user(username)
    log_action(current_user, "delete_user", f"username={username}",
               ip=request.client.host if request.client else "")
    return RedirectResponse(f"/admin/users?flash=User+'{username}'+deleted", status_code=303)


@router.post("/users/{username}/role")
async def users_update_role(
    username: str,
    request: Request,
    current_user: str = Depends(require_auth),
    role: str = Form(...),
):
    if role not in ("viewer", "admin"):
        return RedirectResponse(
            f"/admin/users?flash=Invalid+role+'{role}'&flash_type=error", status_code=303
        )
    update_role(username, role)
    log_action(current_user, "update_role", f"username={username} role={role}",
               ip=request.client.host if request.client else "")
    return RedirectResponse(
        f"/admin/users?flash=Role+updated+for+'{username}'", status_code=303
    )


@router.post("/users/{username}/disable")
async def users_disable(
    username: str, request: Request, current_user: str = Depends(require_auth)
):
    if username == current_user:
        return RedirectResponse(
            "/admin/users?flash=Cannot+disable+yourself&flash_type=error", status_code=303
        )
    disable_user(username)
    log_action(current_user, "disable_user", f"username={username}",
               ip=request.client.host if request.client else "")
    return RedirectResponse(f"/admin/users?flash=User+'{username}'+disabled", status_code=303)


@router.post("/users/{username}/enable")
async def users_enable(
    username: str, request: Request, current_user: str = Depends(require_auth)
):
    enable_user(username)
    log_action(current_user, "enable_user", f"username={username}",
               ip=request.client.host if request.client else "")
    return RedirectResponse(f"/admin/users?flash=User+'{username}'+enabled", status_code=303)


@router.post("/users/{username}/reset-password")
async def users_reset_password(
    username: str,
    request: Request,
    current_user: str = Depends(require_auth),
    new_password: str = Form(...),
    current_password: str = Form(""),
):
    """Reset a user's password.

    - For other users: admin can reset directly (no current password required).
    - For own account: current_password must be verified first.
    """
    if username == current_user:
        if not current_password:
            return RedirectResponse(
                f"/admin/users/{username}/keys?flash=Current+password+required+to+reset+own+account&flash_type=error",
                status_code=303,
            )
        if not verify_user_password(current_user, current_password):
            return RedirectResponse(
                f"/admin/users/{username}/keys?flash=Current+password+incorrect&flash_type=error",
                status_code=303,
            )
    if not new_password:
        return RedirectResponse(
            f"/admin/users?flash=New+password+cannot+be+empty&flash_type=error",
            status_code=303,
        )
    update_password(username, new_password)
    log_action(current_user, "reset_password", f"username={username}",
               ip=request.client.host if request.client else "")
    return RedirectResponse(
        f"/admin/users?flash=Password+reset+for+'{username}'", status_code=303
    )


# ---------------------------------------------------------------------------
# API Keys
# ---------------------------------------------------------------------------

@router.get("/users/{username}/keys", response_class=HTMLResponse)
async def api_keys_page(
    username: str, request: Request, current_user: str = Depends(require_auth),
    new_key: Optional[str] = None,
):
    api_keys = list_api_keys(username)
    return templates.TemplateResponse(
        "api_keys.html",
        _ctx(request, current_user, "users", target_user=username,
             api_keys=api_keys, new_key=new_key),
    )


@router.post("/users/{username}/keys/create")
async def api_keys_create(
    username: str,
    request: Request,
    current_user: str = Depends(require_auth),
    label: str = Form(""),
):
    raw, _ = create_api_key(username, label)
    log_action(current_user, "create_api_key", f"user={username} label={label}",
               ip=request.client.host if request.client else "")
    # Pass the raw key as a query param so it can be shown once
    from urllib.parse import quote
    return RedirectResponse(
        f"/admin/users/{username}/keys?new_key={quote(raw)}", status_code=303
    )


@router.post("/keys/{key_id}/revoke")
async def api_key_revoke(
    key_id: int, request: Request, current_user: str = Depends(require_auth)
):
    revoke_api_key(key_id)
    log_action(current_user, "revoke_api_key", f"key_id={key_id}",
               ip=request.client.host if request.client else "")
    # Redirect back to the referring user's key page (best-effort)
    referer = request.headers.get("referer", "/admin/users")
    return RedirectResponse(referer, status_code=303)


# ---------------------------------------------------------------------------
# License page
# ---------------------------------------------------------------------------

@router.get("/license", response_class=HTMLResponse)
async def license_page(request: Request, current_user: str = Depends(require_auth)):
    from backend.license_manager import (
        detect_tier,
        get_acceptance_record,
        get_tier_label,
        check_seat_limit,
        is_accepted,
        LICENSE_TIERS,
        FREE_TEAM_SEAT_LIMIT,
    )
    users = list_users()
    user_count = len(users)
    tier = detect_tier(user_count)
    tier_label = get_tier_label(tier)
    seat_ok, seat_msg = check_seat_limit(user_count)
    acceptance = get_acceptance_record()
    accepted = is_accepted()
    return templates.TemplateResponse(
        "license.html",
        _ctx(
            request, current_user, "license",
            acceptance=acceptance,
            accepted=accepted,
            user_count=user_count,
            tier=tier,
            tier_label=tier_label,
            seat_ok=seat_ok,
            seat_msg=seat_msg,
            license_tiers=LICENSE_TIERS,
            free_team_seat_limit=FREE_TEAM_SEAT_LIMIT,
            license_email=_safe_license_email(),
        ),
    )


def _safe_license_email() -> str:
    """Return the configured license contact email, falling back to the default."""
    try:
        return _get_license_contact_email()
    except ValueError:
        return "license@devtrack.dev"


# ---------------------------------------------------------------------------
# Server config page
# ---------------------------------------------------------------------------

@router.get("/server", response_class=HTMLResponse)
async def server_page(request: Request, current_user: str = Depends(require_auth)):
    snapshot = _snapshot_ctx()
    from backend.config import (
        llm_provider, ollama_host, ollama_model, openai_model, anthropic_model, groq_model,
        azure_pat, get_github_token, get_gitlab_pat, jira_api_token, get_telegram_bot_token,
    )
    config = {
        "LLM_PROVIDER":  llm_provider() or "—",
        "OLLAMA_HOST":   ollama_host() or "—",
        "OLLAMA_MODEL":  ollama_model() or "—",
        "OPENAI_MODEL":  openai_model() or "—",
        "ANTHROPIC_MODEL": anthropic_model() or "—",
        "GROQ_MODEL":    groq_model() or "—",
    }
    integrations = {
        "Azure DevOps": "configured" if azure_pat() else "not set",
        "GitHub":       "configured" if get_github_token() else "not set",
        "GitLab":       "configured" if get_gitlab_pat() else "not set",
        "Jira":         "configured" if jira_api_token() else "not set",
        "Telegram":     "configured" if get_telegram_bot_token() else "not set",
        "MS Graph":     "configured" if azure_pat() else "not set",
    }
    return templates.TemplateResponse(
        "server.html",
        _ctx(request, current_user, "server",
             snapshot=snapshot, config=config, integrations=integrations),
    )


# ---------------------------------------------------------------------------
# Audit log
# ---------------------------------------------------------------------------

@router.get("/audit", response_class=HTMLResponse)
async def audit_page(request: Request, current_user: str = Depends(require_auth)):
    entries = get_audit_log(limit=_get_audit_log_limit())
    return templates.TemplateResponse(
        "audit.html",
        _ctx(request, current_user, "audit", entries=entries),
    )
