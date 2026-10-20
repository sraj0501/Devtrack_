---
name: Server UI refresh
description: Active admin-console redesign awaiting integration decision
type: project
---

The full-console redesign is committed on `feat/TASK-154-server-admin-ui` at `54ceb05`, but that
commit is not an ancestor of current `dev`. The current `dev` working tree has no uncommitted admin
changes. Review and integrate the branch or explicitly retire it before packaged qualification and
privacy-reviewed media capture.

The admin console is a server-rendered Jinja/HTML and HTMX application under `devtrack_server/backend/admin`, with shared styling in `static/admin.css`. Keep this lightweight architecture for the first redesign. Do not introduce a SPA framework or frontend build pipeline unless audited interaction requirements justify the maintenance cost.

## Goals

- Establish coherent typography, spacing, color, elevation, iconography, and data density.
- Make operational hierarchy obvious: items requiring attention first, supporting metrics second, configuration and audit detail third.
- Use accessible, consistent navigation and explicit active, hover, focus, disabled, loading, destructive, empty, success, warning, error, stale-data, and disconnected states.
- Keep tables, filters, forms, badges, action queues, clients, statistics, narratives, and audit history consistent.
- Meet WCAG AA contrast and keyboard/focus expectations, respect reduced motion, and support common desktop, tablet, and narrow mobile widths.
- Preserve fast server rendering and progressive enhancement. Core workflows must remain usable when HTMX or its CDN dependency is unavailable.

## Scope and safety

The pass covers login, navigation shell, dashboard, pending-action queue/detail, users, server status/configuration, audit log, API keys, license, connected clients, statistics, narrative panels, and HTMX partials.

This is a visual and interaction redesign. It does not authorize changes to approval rules, delivery safety, authentication semantics, retention behavior, or server business logic. Document and review any workflow change separately.

## Integration gate

1. Review the feature-branch diff against current `dev` and reconcile any template, route, or CSS drift.
2. Re-run focused admin route/template tests plus accessibility, zoom, reduced-motion, overflow,
   long-value, and responsive checks on the reconciled branch.
3. Integrate through the normal feature-to-`dev` flow or explicitly retire the branch; do not leave
   release documentation implying uncommitted local work.
4. Qualify the packaged build, then capture fresh privacy-reviewed screenshots and video from the
   integrated behavior.

The refresh is ready for release only when it is integrated, consistent across admin routes,
non-happy states are explicit, accessibility and responsive checks pass, and current captures
reflect tested packaged behavior.
