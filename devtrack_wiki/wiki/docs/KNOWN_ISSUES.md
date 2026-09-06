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

## End-to-end validation hold

Roadmap feature work is paused while clean-machine installation, full Linux Managed-demo parity,
admin-queue review, and privacy-reviewed screenshot/video capture are completed. The Windows
local-user Managed workflow has passed twice with real commit detection, `DEMO-101` mapping,
confidence-bearing staging, EOD generation, six-tool MCP introspection, and disposable-workspace
cleanup. A separate credential-free core lane has passed locally on native Windows, in a Linux Go
container, and on GitHub-hosted Windows and Ubuntu runners, covering a real `DEMO-201` commit through
the daemon, SQLite, and MCP. The workflow is committed on `dev`; End-to-end run `34045590767` passed
for commit `ed0f571`. This evidence does not mean the remaining full Managed release gates are
complete.

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
