---
name: SaaS License & Auth System
description: License tier model, auth architecture, T&C acceptance flow, and opt-in telemetry for DevTrack SaaS
type: project
---

# DevTrack SaaS License & Auth System

**Status**: Shipped on `feature/saas_model` (March 31, 2026)
**Why**: DevTrack is transitioning from pure open-source to a freemium SaaS model. The foundation layer (license enforcement, auth, telemetry) was implemented first so the rest of the system can build on it.
**How to apply**: All new features that differ by tier should call `detect_tier()` from `backend/license_manager.py`. Telemetry for new commands should call `record()` from `backend/telemetry.py`. Never gate offline functionality behind a login check.

---

## License Tiers

| Tier | Users | Cost | Notes |
|---|---|---|---|
| Personal | 1 | Free | Default; single user personal use |
| Team | 2–10 | Free | Self-hosted; no licence server needed |
| Enterprise | 11+ | Commercial licence required | Contact license@devtrack.dev |

**Key constants** (`backend/license_manager.py`):
- `TERMS_VERSION = "1.0"` — bump when T&C materially change; triggers re-acceptance
- `FREE_TEAM_SEAT_LIMIT = 10`
- Tier detection: `detect_tier(active_user_count: int) -> str`
- Seat check: `check_seat_limit(active_user_count) -> (bool, str)`

**Tier enforcement** is advisory in local mode, strict in SaaS mode. This preserves the offline guarantee.

---

## Auth System Design

**Principle**: Login is entirely optional for personal use. All DevTrack features work offline with no session.

### Two Session Modes

| Mode | When | Network | Token |
|---|---|---|---|
| `local` | No `DEVTRACK_API_URL` set, or network unavailable | None | Local UUID (offline session) |
| `cloud` | `DEVTRACK_API_URL` set + successful magic-link login | Required for login only | JWT from server, cached locally |

### Session Lifecycle

1. `devtrack login` → `cloud_auth.interactive_login()` → magic-link email → 6-digit OTP → JWT stored in `Data/license/session.json` (chmod 600)
2. Subsequent runs: `session.py:get_session()` loads from disk, validates expiry (90-day TTL), caches in memory
3. `devtrack logout` → `clear_session()` wipes memory + disk
4. Offline fallback: `make_offline_session(email)` creates local UUID token without any network call

### AuthSession Fields (`backend/auth/session.py`)

```python
email: str
user_id: str           # Opaque server-assigned ID
display_name: str
token: str             # Bearer JWT or local UUID
token_expires_at: str  # ISO8601 UTC; 90-day TTL
tier: str              # personal | team | enterprise
seat_count: int
telemetry_enabled: bool  # Only meaningful in cloud mode
mode: str              # local | cloud
organisation: str
```

### Key Files

| File | Purpose |
|---|---|
| `backend/auth/session.py` | `AuthSession` dataclass; in-memory cache; `get/set/clear_session`; `make_offline_session` |
| `backend/auth/local_auth.py` | Read/write/delete `Data/license/session.json`; sets chmod 600 |
| `backend/auth/cloud_auth.py` | Magic-link request + verify flows; `interactive_login()`; telemetry opt-in prompt |
| `devtrack-bin/license.go` | Go CLI handlers: `handleLogin`, `handleLogout`, `handleWhoami`, `handleLicense`, `handleTerms`, `handleTelemetry`; `EnsureTermsAccepted()` first-run check |

---

## Terms & Conditions Acceptance Flow

**Design goals**:
- First-run prompt shown once; acceptance persisted to `Data/license/acceptance.json`
- Never blocks execution on read errors (offline safety)
- CI/automation bypass: `DEVTRACK_AUTO_ACCEPT_TERMS=1`
- Non-interactive mode auto-accepts with a printed warning

### Flow

```
devtrack <any command>
    └── Go: EnsureTermsAccepted(projectRoot)
            ├── Skip for: terms, license, help, version, shell-init
            ├── checkTermsAccepted() → uv run python: is_accepted()
            │       └── reads Data/license/acceptance.json
            ├── If not accepted → promptTermsAcceptance()
            │       └── uv run python: ensure_accepted()
            │               └── shows _TERMS_SUMMARY banner
            │               └── prompts yes/no
            │               └── on yes: saves acceptance.json
            └── Returns false only on explicit "no" — never on errors
```

**CLI commands**:
- `devtrack terms` — show T&C summary + pointer to TERMS.md
- `devtrack terms --accept` — accept without interactive prompt
- `devtrack license` — show current tier and acceptance status

**Acceptance record** (`Data/license/acceptance.json`):
```json
{
  "terms_version": "1.0",
  "accepted_at": "2026-03-31T...",
  "user_identifier": "sraj",
  "mode": "local"
}
```

**Re-acceptance trigger**: bump `TERMS_VERSION` in `license_manager.py`. Next startup will show the new terms.

---

## Telemetry Opt-In Design

**Rules** (hardcoded, cannot be bypassed):
1. Completely no-op if user is not logged in
2. Completely no-op if `session.telemetry_enabled` is False
3. Never collects: code, commit messages, file contents, credentials, diffs
4. Async batched — never blocks CLI commands
5. Silently drops on network error — telemetry failures never surface to user

### Event Flow

```
record("command.run", command="devtrack start")
    └── _is_enabled() → checks session.telemetry_enabled
    └── Strips blocked keys: token, password, key, secret, credential, message, content, diff
    └── Appends to _event_queue (in-memory, thread-safe)
    └── If queue ≥ MAX_BATCH_SIZE (50) → _flush_sync()

Background thread (start_background_flush):
    └── Every FLUSH_INTERVAL_SECS (300) → flush()
    └── POST {DEVTRACK_API_URL}/telemetry/batch in daemon thread

stop_background_flush():
    └── Called on daemon shutdown → final flush
```

### Opt-In/Out Commands

```bash
devtrack telemetry status   # show current state
devtrack telemetry on       # enable (requires login)
devtrack telemetry off      # disable
```

### Key File: `backend/telemetry.py`

Public API:
- `record(event_type: str, **props)` — queue a safe event (no-op if disabled)
- `start_background_flush()` — call once at daemon startup
- `stop_background_flush()` — call on daemon shutdown
- `flush()` — force immediate flush

---

## SaaS-Related Environment Variables

| Variable | Required | Purpose |
|---|---|---|
| `DEVTRACK_API_URL` | No | Cloud server base URL (e.g. `https://api.devtrack.dev`). If unset, cloud features are disabled and all auth is local-only. |
| `DEVTRACK_AUTO_ACCEPT_TERMS` | No | Set to `1` to skip interactive T&C prompt (CI use) |
| `DEVTRACK_VERSION` | No | Fallback version string for telemetry if package metadata unavailable |

---

## What's NOT Enforced Yet

Tier detection and `check_seat_limit()` exist but enforcement is **advisory only** in local mode — warns but does not block. Hard enforcement requires the SaaS backend to be live. Offline single-user mode is never blocked regardless.

## Next SaaS Features to Build (immediate next session)

1. **Cloud server** — FastAPI/Go service implementing `/auth/magic-link`, `/auth/verify`, `/telemetry/batch`
2. **Stripe integration** — enterprise licence purchase, seat management, webhook for tier upgrades
3. **Admin dashboard** — org management, seat counts, telemetry overview
4. **Hosted deployment** — Docker Compose + cloud infra; `DEVTRACK_API_URL` will point here
5. **Token refresh** — JWT refresh flow (before 90-day expiry) to keep long-lived sessions alive
6. **Team seat provisioning** — `devtrack team invite <email>`, seat count tracking per org
