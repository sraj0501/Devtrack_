# Known issues and current limitations

## Release boundary

The latest public release is v3.0.10. MCP and the Phase 9 onboarding/demo reliability work currently
exist on `dev`; a newer release must be published before public-release instructions can assume them.

## Local model latency

Cold or large Ollama models can take longer than ordinary HTTP operations. The commit-trigger path
uses the centralized long timeout, and the demo wait is configurable, but users may still need to
increase `HTTP_TIMEOUT_LONG` or `LLM_REQUEST_TIMEOUT_SECS` for their hardware/model combination.

## Optional server readiness

Managed sparse checkout, `uv sync`, PostgreSQL migration, and model download happen in the background.
They may be temporarily degraded after setup. `devtrack status`, `devtrack doctor`, and
`devtrack doctor --repair` expose progress and recovery; Go-native features remain usable.

## Platform rollout

The launch is GitHub-first. GitLab PR approval detection is deferred; Azure DevOps has a native
approval checker. This staged rollout is deliberate and should not be described as universal PR-loop
parity.

## Registry distribution

Registry copy is prepared, but official MCP publication remains blocked until a supported,
versioned MCP-capable package (such as MCPB where required) is released and submission is explicitly
authorized. No registry listing has been published by the repository work.

## Reporting a problem

Include OS, version, mode, redacted `devtrack doctor` output, and a minimal log excerpt in a GitHub
issue. Never include tokens, private code, or personal data.
