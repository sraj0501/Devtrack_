# Known issues and current limitations

## Release boundary

The latest public release is v3.1.1. MCP, five native MCPB bundles, and the Phase 9
onboarding/demo reliability work are included. Older installations must upgrade before following
the MCP and Phase 9 guides.

## Local model latency

Cold or large Ollama models can take longer than ordinary HTTP operations. The commit-trigger path
uses the centralized long timeout, and the demo wait is configurable, but users may still need to
increase `HTTP_TIMEOUT_LONG` or `LLM_REQUEST_TIMEOUT_SECS` for their hardware/model combination.

## Optional server readiness

Managed sparse checkout, `uv sync --extra ai`, PostgreSQL migration, and generation/embedding model
downloads happen in the background. They may be temporarily degraded after setup. `devtrack status`,
`devtrack doctor`, and `devtrack doctor --repair` expose progress and recovery; Go-native features
remain usable.

## Remaining release qualification

Clean Windows installation and the full Managed Linux journey were confirmed complete by the owner
on 2026-09-10. The credential-free core lane also passed locally on native Windows, in a Linux Go
container, and on GitHub-hosted Windows and Ubuntu runners, covering a real `DEMO-201` commit through
the daemon, SQLite, and MCP. Those source and environment checks do not close the remaining gates:
the separate TASK-154 server-admin redesign at `54ceb05` is not integrated into `dev` and must be
reviewed, integrated, or retired before the packaged build is qualified. Public screenshots/video
still need privacy review, and the exact Glama listing path must be recorded before its score badge
is updated.

The merged PR #263 and PR #264 check sets each retain a failed hosted-Windows unit-test job. PR
#264's job timed out after 60 seconds in SQLite-backed `internal/db` and `internal/mcp` tests;
Windows build/vet, native MCPB smoke, no-send E2E, and Sage distillation tests passed. These green
lanes do not replace the failed unit-test gate.

DevTrack Sage development is the active product initiative. Its capture, model-free search, neutral
Go LLM client, validated structured distiller, and non-blocking worker foundation are on `dev`.
Durable SQLite claims and daemon wiring, deterministic Markdown, correction routing,
retry/diagnostic state, safe local commits, and executable parity closure remain incomplete.

TASK-158's completely silent native-Git path is also merged into `dev` but is not in v3.1.1.
Until a newer public build ships, v3.1.1 users can still encounter its shell-wrapped post-commit
prompt regression.

Rolling `dev` update-channel support is TASK-161 work on
`features/TASK-161-rolling-dev-channel`. Treat `devtrack upgrade --dev`, `--main`, and `--stable`
as unavailable in published or merged builds until its PR is reviewed, integrated, and publishes
the first `dev` prerelease.

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

`io.github.sraj0501/devtrack` 3.1.1 is active/latest in the official MCP Registry. The DevTrack entry
in `punkpeye/awesome-mcp-servers` is awaiting maintainer review in
[PR #13608](https://github.com/punkpeye/awesome-mcp-servers/pull/13608), whose automated submission
check passed. Glama's admins approved the submitted server on 2026-09-06, but its exact approved
path is not yet recorded, so the required PR score-badge update remains pending. Anthropic, Smithery, and
other directory forms still require owner-authenticated sessions, eligibility evidence, or contact
details; neither registry publication nor an open PR implies acceptance by those directories.

## Reporting a problem

Include OS, version, mode, redacted `devtrack doctor` output, and a minimal log excerpt in a GitHub
issue. Never include tokens, private code, or personal data.
