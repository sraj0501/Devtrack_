# Known issues and current limitations

## Release boundary

Stable v3.2.0 is public, including DevTrack Sage, ten read-only MCP tools, five native MCPB bundles,
deterministic ticket mapping, rolling update channels and onboarding. v3.3.0 is development work on
`dev`; rolling builds use `dev-<commit>`. An existing installation is not updated by pushing source.

## Basic workflow qualification

v3.3.0 development removes time tracking and makes work reports commit-based. The running monitor
walks unseen commits across local branch tips and detached HEAD to retain rapid bursts and
switched-away branches. Server queue history and EOD reports are directly readable from the CLI;
server actions use `server:<id>` for review without colliding with numeric local IDs.

Source-based Linux/PostgreSQL container checks passed for capture, ticket context, intentional
outages, restart, queue review and manual/scheduled report visibility. Packaged development-build
acceptance, native Windows rehearsal, real PM/email delivery, overnight coverage and a representative
workday remain to qualify. Commits made while the daemon is stopped are not a guaranteed catch-up
path. Follow `docs/testing/basic-workflow.md` in the repository. Sage testing, learning repairs and
platform-parity work remain on hold until these basics qualify.

## Local model latency

Cold or large Ollama models can take longer than ordinary HTTP operations. The commit-trigger path
uses the centralized long timeout, and the demo wait is configurable, but users may still need to
increase `HTTP_TIMEOUT_LONG` or `LLM_REQUEST_TIMEOUT_SECS` for their hardware/model combination.

## Optional server readiness

Managed sparse checkout, `uv sync --extra ai`, PostgreSQL migration, and generation/embedding model
downloads happen in the background. They may be temporarily degraded after setup. `devtrack status`,
`devtrack doctor`, and `devtrack doctor --repair` expose progress and recovery; Go-native features
remain usable.

## Deferred UI and publication work

The separate TASK-154 server-admin visual redesign at `54ceb05` is intentionally excluded from
v3.2.0. Its old branch requires reconciliation against current routes, canonical project memory,
and fresh accessibility/responsive acceptance before a later release. The existing integrated
admin behavior remains available. Public screenshots/video still require privacy review, and the
exact approved Glama listing path must be copied before updating its score badge.

## Communication-learning CLI adapter

Automatic local Git-history voice seeding is implemented in Managed onboarding. However, the
Python HTTP handlers for communication-learning commands use an incomplete `LearningIntegration`
adapter. Status, enable, sync, reset, cron, test, and revoke call absent method names; profile calls
an existing method before initialization. Until that adapter is repaired, treat all of these CLI
paths as unavailable. Teams and Outlook must not be advertised as a completed end-to-end learning
workflow.

## Platform rollout

The launch is GitHub-first. GitLab PR approval detection is deferred; Azure DevOps has a native
approval checker. This staged rollout is deliberate and should not be described as universal PR-loop
parity.

## Registry distribution

`io.github.sraj0501/devtrack` 3.2.0 is active/latest in the official MCP Registry. v3.3.0 development
has not replaced that stable record. The DevTrack entry
in `punkpeye/awesome-mcp-servers` is awaiting maintainer review in
[PR #13608](https://github.com/punkpeye/awesome-mcp-servers/pull/13608), whose automated submission
check passed. Glama's admins approved the submitted server on 2026-09-06, but its exact approved
path is not yet recorded, so the required PR score-badge update remains pending. Anthropic, Smithery, and
other directory forms still require owner-authenticated sessions, eligibility evidence, or contact
details; neither registry publication nor an open PR implies acceptance by those directories.

## Reporting a problem

Include OS, version, mode, redacted `devtrack doctor` output, and a minimal log excerpt in a GitHub
issue. Never include tokens, private code, or personal data.
