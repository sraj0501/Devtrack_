---
name: SaaS license and auth system
description: License tiers, T&C flow, session/auth — local shipped; cloud server deprioritised (local-first pivot)
type: project
---

_Cloud/SaaS is deprioritised — DevTrack is local-first (PRODUCT_BIBLE.md). Local license/auth stays; the cloud API server, Stripe, and team provisioning were never built._

**Tiers:** Personal (1, free), Team (2–10, free self-hosted), Enterprise (11+, commercial). `detect_tier()` / `check_seat_limit()` in `backend/license_manager.py`. Bump `TERMS_VERSION` to force re-acceptance. CI bypass: `DEVTRACK_AUTO_ACCEPT_TERMS=1`.

**Auth:** `local` = offline UUID token; `cloud` = magic-link → JWT, 90-day TTL (`Data/license/session.json`, chmod 600). Key files: `backend/auth/{session,local_auth,cloud_auth}.py`; Go: `license_cli.go`.

**Rule:** never gate offline functionality behind a login check. License enforcement is advisory only.

**Telemetry: OPT-IN** (TASK-109). No-op unless the user explicitly enables it. Never collects code, diffs, or credentials.
