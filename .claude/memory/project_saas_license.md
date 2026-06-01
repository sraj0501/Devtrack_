---
name: SaaS license and auth system
description: License tiers, T&C flow, session/auth — shipped; cloud server not yet built
type: project
---

**Shipped:** license enforcement (advisory locally), T&C, auth sessions, telemetry opt-in.
**Not built:** cloud API server, Stripe billing, team provisioning, token refresh.

**Tiers:** Personal (1 user, free), Team (2–10, free self-hosted), Enterprise (11+, commercial). `detect_tier()` / `check_seat_limit()` in `backend/license_manager.py`. Bump `TERMS_VERSION` to force re-acceptance. CI bypass: `DEVTRACK_AUTO_ACCEPT_TERMS=1`.

**Auth:** `local` mode = offline UUID token; `cloud` = magic-link → JWT, 90-day TTL in `Data/license/session.json` (chmod 600). Key files: `devtrack_server/backend/auth/session.py`, `local_auth.py`, `cloud_auth.py`; Go side in `devtrack_client/` license CLI (`license_cli.go`).

**Rule:** never gate offline functionality behind a login check.

**Telemetry:** no-op unless logged in + `session.telemetry_enabled=True`. Never collects code/diffs/credentials. Async batch POST to `DEVTRACK_API_URL/telemetry/batch`; silent on error.
