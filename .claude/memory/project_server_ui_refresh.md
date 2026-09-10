---
name: Server UI refresh
description: Completed server-rendered admin-console redesign
type: project
---

TASK-154 finished the existing admin template and CSS redesign across
the full console without changing workflow or authorization semantics. The implementation remains
server-rendered Jinja/HTML with HTMX as optional progressive enhancement.

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

## Completed implementation

1. Inventoried and aligned the login, shell, dashboard, queue, user, credential, server, audit, and licence surfaces.
2. Extended the shared CSS tokens/components to secondary pages, tables, forms, status notices, detail grids, and empty states.
3. Added skip navigation, visible keyboard focus, active-page semantics, reduced-motion behavior, responsive containment, and mobile detail layouts.
4. Added explicit HTMX loading/disconnected/failure feedback while keeping links and forms usable without HTMX or JavaScript.
5. Added route/template regression coverage. Verification: 91 focused admin tests; full suite 972 passed, 10 skipped.

Fresh privacy-reviewed screenshots and video remain an existing release-media follow-up. They are
not part of the redesign implementation and must be captured from the packaged build before publication.
