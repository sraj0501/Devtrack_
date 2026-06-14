---
name: SaaS license and auth system
description: License tiers, T&C flow, session/auth — shipped; cloud server not yet built
type: project
---

_[Pivot 2026-06-10: cloud/SaaS deprioritised — DevTrack is local-first per PRODUCT_BIBLE.md. Local license/auth stays; cloud server not the current focus.]_

**Shipped:** license enforcement (advisory), T&C, auth sessions, telemetry opt-in. **Not built:** cloud API server, Stripe, team provisioning.

**Tiers:** Personal (1, free), Team (2–10, free self-hosted), Enterprise (11+, commercial). `detect_tier()` / `check_seat_limit()` in `backend/license_manager.py`. Bump `TERMS_VERSION` to force re-acceptance. CI bypass: `DEVTRACK_AUTO_ACCEPT_TERMS=1`.

**Auth:** `local` = offline UUID token; `cloud` = magic-link → JWT, 90-day TTL (`Data/license/session.json`, chmod 600). Key files: `backend/auth/session.py`, `local_auth.py`, `cloud_auth.py`; Go: `license_cli.go`.

**Rule:** never gate offline functionality behind a login check.

**Telemetry:** no-op unless `session.telemetry_enabled=True`. Never collects code/diffs/credentials. Async batch POST to `DEVTRACK_API_URL/telemetry/batch`.
