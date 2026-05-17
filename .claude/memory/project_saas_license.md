---
name: SaaS license and auth system
description: License tiers, T&C flow, session/auth architecture — shipped; cloud server not yet built
type: project
---

**Shipped**: license enforcement (advisory in local mode), T&C acceptance, auth sessions, telemetry opt-in.
**Not built yet**: cloud API server, Stripe billing, team provisioning, token refresh.

## License Tiers
- Personal (1 user, free), Team (2–10, free self-hosted), Enterprise (11+, commercial licence)
- `detect_tier()` / `check_seat_limit()` in `backend/license_manager.py`
- `TERMS_VERSION = "1.0"` — bump triggers re-acceptance on next startup
- CI bypass: `DEVTRACK_AUTO_ACCEPT_TERMS=1`

## Auth Sessions
Two modes: `local` (no `DEVTRACK_API_URL`, offline UUID token) and `cloud` (magic-link → JWT, 90-day TTL stored in `Data/license/session.json` chmod 600).

Key files: `backend/auth/session.py`, `local_auth.py`, `cloud_auth.py`, `devtrack-bin/license.go`.

**Rule**: never gate offline functionality behind a login check. All features work without a session.

## Telemetry
- No-op unless user is logged in AND `session.telemetry_enabled=True`
- Never collects code, commit messages, diffs, credentials
- `record()` queues async batch POST to `DEVTRACK_API_URL/telemetry/batch`; silent on network error
