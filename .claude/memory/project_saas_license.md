---
name: SaaS license and auth system
description: Local license/auth shipped; cloud SaaS remains deprioritized
type: project
---

Cloud/SaaS is deprioritized by the local-first pivot; Stripe/team provisioning and a hosted product were not built.
**Tiers:** Personal 1/free; Team 2–10/free self-hosted; Enterprise 11+/commercial. Enforcement is advisory and must never gate offline use.
**Auth:** local offline token or optional magic-link/JWT; session is `<configured data dir>/license/session.json` with owner-only permissions. Server files: `devtrack_server/backend/auth/`; client entry: `devtrack_client/license_cli.go`.
**Terms:** bump `TERMS_VERSION` to require re-acceptance; CI may set `DEVTRACK_AUTO_ACCEPT_TERMS=1`.
**Telemetry:** opt-in and never includes code, diffs, credentials, or work text.
