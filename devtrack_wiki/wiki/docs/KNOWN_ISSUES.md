# Known issues and current limitations

## Release boundary

The v3.2.0 release candidate includes DevTrack Sage, ten read-only MCP tools, five native MCPB
bundles, deterministic ticket mapping, silent time inference, rolling update channels, and the
Phase 9 onboarding path. v3.1.1 remains public until candidate promotion, tagging, and publication
complete.

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

`io.github.sraj0501/devtrack` 3.1.1 remains active/latest in the official MCP Registry while the
3.2.0 candidate is qualified. The DevTrack entry
in `punkpeye/awesome-mcp-servers` is awaiting maintainer review in
[PR #13608](https://github.com/punkpeye/awesome-mcp-servers/pull/13608), whose automated submission
check passed. Glama's admins approved the submitted server on 2026-09-06, but its exact approved
path is not yet recorded, so the required PR score-badge update remains pending. Anthropic, Smithery, and
other directory forms still require owner-authenticated sessions, eligibility evidence, or contact
details; neither registry publication nor an open PR implies acceptance by those directories.

## Reporting a problem

Include OS, version, mode, redacted `devtrack doctor` output, and a minimal log excerpt in a GitHub
issue. Never include tokens, private code, or personal data.
