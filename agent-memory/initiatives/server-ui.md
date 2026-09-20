---
name: Server UI refresh
description: Active uncommitted admin-console visual work
type: project
---

Current uncommitted admin template and CSS changes are active work in progress. Test them and either include them in or isolate them from the release-candidate baseline before packaged qualification and privacy-reviewed media capture.

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

## Next steps

1. Inventory routes, templates, partials, states, actions, and viewports; capture a safe fixture baseline.
2. Define information architecture and primary operator action per page.
3. Build shared design tokens and components in the existing CSS.
4. Apply the system to the shell/dashboard, then review flows, tables, forms, and secondary pages.
5. Verify HTMX focus/state behavior, accessibility, zoom, reduced motion, overflow, long values, and responsiveness.
6. Add route/template coverage and lightweight visual regression for key desktop and mobile states.

The refresh is ready only when the design is consistent across admin routes, non-happy states are explicit, accessibility and responsive checks pass, and current captures reflect tested behavior.
