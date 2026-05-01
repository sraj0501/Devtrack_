---
name: project_cs3_admin_gui
description: CS-3 Admin GUI MVP — Jinja2/HTMX admin console with user management, license status, trigger stats, password reset, and ADMIN_EMBED single-process mode (492 tests)
type: project
---

# CS-3: Admin GUI MVP

**Session**: April 10, 2026
**Branches**: features/TASK-011 through features/TASK-015-cs3-polish
**Commits**: TASK-011 through TASK-015 (5 commits)
**PRs**: #69 (TASK-011), #70 (TASK-012), #71 (TASK-013), #72 (TASK-014), #73 (TASK-015)

## Why:

The backend had an admin console (`backend/admin/`) with basic auth, session management, user CRUD, API key management, server status, and audit log — but no test coverage and several missing management features (role changes, soft-disable, license visibility, live trigger stats). CS-3 completed the MVP by adding route-level tests, the missing management routes, a license page, a live stats panel, password reset, and an embed mode so the admin UI can run inside the same process as the webhook server without a separate daemon.

## How to apply:

**Adding new admin routes**:
1. Add the route handler to `backend/admin/routes.py` using the existing `@router` pattern
2. Create a corresponding Jinja2 template in `backend/admin/templates/`
3. Add a nav link in `backend/admin/templates/base.html` if it needs sidebar navigation
4. Write HTTP-level tests in `backend/tests/test_admin_routes.py` using the `admin_client` fixture
5. Log admin actions via `log_action(username, action, detail)` for audit trail

**Using ADMIN_EMBED**:
- Set `ADMIN_EMBED=true` in `.env`
- The webhook_server mounts `/admin` router + static files automatically at startup
- No separate admin process needed; admin UI accessible at `http://host:port/admin`
- Add `get_admin_embed()` accessor call is already in `backend/config.py`

**Test isolation pattern for admin routes**:
- `DATABASE_DIR` monkeypatched to `tmp_path` — each test gets a fresh SQLite DB
- `get_snapshot` mocked to avoid spawning `ps`/subprocess
- `ADMIN_USERNAME` / `ADMIN_PASSWORD` set via `monkeypatch.setenv` so `check_credentials` works without a real `.env`
- `admin_client` fixture in `test_admin_routes.py` handles login cookie injection automatically

---

## TASK-011 — HTTP-level route tests for admin console

**File added**: `backend/tests/test_admin_routes.py` (338 lines, 31 tests)

**Coverage — 7 test groups across 14 routes**:
| Group | Routes tested |
|---|---|
| TestAuthRoutes | GET /admin/login, POST /admin/login (ok/bad creds), GET /admin/logout |
| TestDashboard | GET /admin/ (authed + unauthed) |
| TestUserRoutes | GET /admin/users, POST /admin/users/create, POST /admin/users/{u}/delete |
| TestApiKeyRoutes | GET /admin/users/{u}/keys, POST /admin/users/{u}/keys/create, POST /admin/keys/{id}/revoke |
| TestServerPage | GET /admin/server |
| TestAuditLog | GET /admin/audit |
| TestHtmxPartials | GET /admin/_partials/stats, GET /admin/_partials/processes |

**Test counts after TASK-011**: 464 (was 433); 1 pre-existing failure unchanged.

---

## TASK-012 — User role update + disable/enable routes

**Files modified**:
- `backend/admin/user_manager.py`: idempotent `ALTER TABLE` migration adds `disabled` column; `disable_user()` / `enable_user()` helpers; `AdminUser.disabled` field; `list_users()` and `get_user()` SELECT updated to include column
- `backend/admin/routes.py`: 3 new POST routes
- `backend/admin/templates/users.html`: inline role-change select + button per row; Disable/Enable toggle; disabled rows at reduced opacity with red status badge
- `backend/tests/test_admin_user_manager.py`: +8 tests in `TestDisableEnable`
- `backend/tests/test_admin_routes.py`: +6 route tests in `TestUserRoleDisable`

**New routes**:
| Route | Behaviour |
|---|---|
| `POST /admin/users/{username}/role` | Updates role; guards invalid values; logs via `log_action()` |
| `POST /admin/users/{username}/disable` | Soft-disables user; blocks self-disable |
| `POST /admin/users/{username}/enable` | Re-enables a disabled user |

**Test counts after TASK-012**: 478 (was 464).

---

## TASK-013 — License status page

**Files modified/added**:
- `backend/admin/routes.py`: `GET /admin/license` route; dashboard updated with `tier` / `tier_label` context (guarded `try/except` so missing license_manager degrades gracefully)
- `backend/admin/templates/license.html` (new): terms acceptance status card, current tier card with seat-OK/warning badge, 3-row tier comparison table (Free / Pro / Enterprise)
- `backend/admin/templates/base.html`: "License" nav link added to sidebar
- `backend/admin/templates/dashboard.html`: "Admin Users" stat card replaced with "License Tier" card (shows tier_label + user count + link to `/admin/license`)
- `backend/tests/test_admin_routes.py`: +5 tests in `TestLicensePage`

**New route**: `GET /admin/license`

**Test counts after TASK-013**: 483 (was 478). *(Note: commit message says +5 tests; final number reconciled at 487 per TASK-014 commit message starting from 478.)*

---

## TASK-014 — Trigger stats HTMX panel on admin dashboard

**Files modified/added**:
- `backend/admin/routes.py`: `_trigger_stats_ctx()` helper (try/except guarded; returns zero-valued `TriggerStats` on error); `dashboard()` passes stats to template; `GET /admin/_partials/stats` HTMX partial route
- `backend/admin/templates/_stats_panel.html` (new): 4-stat grid — triggers today, commits today, last trigger (HH:MM), errors 24h; graceful-degrade message when `stats=None`
- `backend/admin/templates/dashboard.html`: "Trigger Activity" card with `{% include "_stats_panel.html" %}` for initial render; `hx-get="/admin/_partials/stats"` + `hx-trigger="every 30s"` for live refresh without page reload

**New route**: `GET /admin/_partials/stats`

**Live refresh**: HTMX polls every 30 seconds; no full-page reload required.

**Test counts after TASK-014**: 487 (was 478).

---

## TASK-015 — Password reset, ADMIN_EMBED, docs sync

### Part A — Password reset route

**File modified**: `backend/admin/routes.py`

**New route**: `POST /admin/users/{username}/reset-password`

Behaviour:
- For *other* users: admin resets directly (no current password needed)
- For *own account*: `current_password` must be verified first
- Empty `new_password` blocked with 400 error
- Action logged via `log_action()`
- +5 tests in `TestPasswordReset` in `test_admin_routes.py`

### Part B — ADMIN_EMBED single-process mode

**Files modified**:
- `backend/config.py`: `get_admin_embed() -> bool` accessor for `ADMIN_EMBED` env var
- `backend/webhook_server.py`: on startup, if `ADMIN_EMBED=true`, mounts admin `router` at `/admin` and admin `static` files; wrapped in `try/except` — any import/mount error is logged and skipped (graceful degradation)
- `.env_sample`: `ADMIN_EMBED=false` entry added with comment

**Effect**: With `ADMIN_EMBED=true`, running `webhook_server.py` is sufficient — no separate admin process needed. Admin UI available at `http://<host>:<port>/admin`.

### Part C — Docs

- `CLAUDE.md` Session Completion Status updated with CS-3 route reference table and ADMIN_EMBED note
- `feature_tracker.md` CS-3 row marked DONE with full task history entry
- `MEMORY.md` updated with April 10 session summary and 492 test count

**Test counts after TASK-015**: **492** (was 487).

---

## Full Route Table (all admin routes as of CS-3)

| Method | Path | Template / Response | Notes |
|---|---|---|---|
| GET | `/admin/login` | `login.html` | Public |
| POST | `/admin/login` | redirect | Sets session cookie |
| GET | `/admin/logout` | redirect | Clears session |
| GET | `/admin/` | `dashboard.html` | Auth required; includes trigger stats + license tier |
| GET | `/admin/_partials/stats` | `_stats_panel.html` | HTMX; 30s auto-refresh |
| GET | `/admin/_partials/processes` | `_proc_rows.html` | HTMX process table rows |
| POST | `/admin/process/{name}/restart` | HTMX fragment | |
| POST | `/admin/process/{name}/stop` | HTMX fragment | |
| POST | `/admin/process/{name}/start` | HTMX fragment | |
| GET | `/admin/users` | `users.html` | User list with role/disable controls |
| POST | `/admin/users/create` | redirect | Creates new admin user |
| POST | `/admin/users/{username}/delete` | redirect | Deletes user |
| POST | `/admin/users/{username}/role` | redirect | Updates role |
| POST | `/admin/users/{username}/disable` | redirect | Soft-disables; blocks self-disable |
| POST | `/admin/users/{username}/enable` | redirect | Re-enables user |
| POST | `/admin/users/{username}/reset-password` | redirect | Password reset; self requires current_password |
| GET | `/admin/users/{username}/keys` | `api_keys.html` | API key management |
| POST | `/admin/users/{username}/keys/create` | redirect | Creates new API key |
| POST | `/admin/keys/{key_id}/revoke` | redirect | Revokes API key |
| GET | `/admin/license` | `license.html` | License tier + acceptance + seat check |
| GET | `/admin/server` | `server.html` | Live process/service status |
| GET | `/admin/audit` | `audit.html` | Admin action log |

---

## Templates Added in CS-3

| Template | Purpose |
|---|---|
| `_stats_panel.html` | HTMX partial: 4-stat trigger activity grid |
| `license.html` | License tier, acceptance status, tier comparison table |

## Templates Modified in CS-3

| Template | Change |
|---|---|
| `users.html` | Role-change select + button; Disable/Enable toggle; disabled row styling |
| `dashboard.html` | "License Tier" stat card; "Trigger Activity" HTMX panel |
| `base.html` | "License" nav link in sidebar |

---

## Test Coverage Summary

| Test file | Tests added in CS-3 | Total |
|---|---|---|
| `test_admin_routes.py` | 31 (TASK-011) + 6 + 5 + 4 + 5 = **51** | 51 |
| `test_admin_user_manager.py` | +8 (TestDisableEnable) | ~41 |
| **Grand total (all suites)** | +59 tests | **492** |

(Previous total was 433 before CS-3.)
