# DevTrack MCP registry submission package

_Research verified: 2026-08-28. Local artifacts only; nothing in this document has been
submitted or published._

## Release and capability facts

- The MCP implementation is on `dev`: a local `stdio` server invoked with `devtrack mcp`.
- It reports MCP protocol version `2024-11-05` and exposes six read-only tools backed by the Go
  client's local SQLite database: `get_active_context`, `get_today_commits`,
  `get_pending_actions`, `get_voice_profile`, `get_ticket_context`, and `get_eod_summary`.
- The MCP path does not require the Python server or a network request. It does require an
  installed DevTrack binary and the user's local DevTrack data/configuration.
- The latest public release is `v3.0.10`. That tag does **not** contain `mcp_cmd.go`; the MCP work
  landed later (TASK-098–101, completed on 2026-06-24). Registry copy must therefore describe the
  MCP-capable `dev` state as unreleased until a newer release is published.
- DevTrack is distributed under its Community License/EULA in `TERMS.md`, not an OSI license.
  Directory forms and manifests must use that exact description and must not label it MIT,
  Apache-2.0, or generally "open source."

## Registry matrix

| Target | Eligibility now | Required fields or artifact | Authoritative submission URL | Blocker / next action |
|---|---|---|---|---|
| Official MCP Registry (preview) | **Blocked** | Publicly downloadable supported package plus `server.json`; GitHub-authenticated names use `io.github.<owner>/<server>`; metadata includes name, description, repository, version, package identifier, and transport. For MCPB, include a GitHub/GitLab release URL and SHA-256. | [Official publishing quickstart](https://modelcontextprotocol.io/registry/quickstart) and [supported package types](https://modelcontextprotocol.io/registry/package-types) | The registry hosts metadata, not binaries. Existing release artifacts are generic executables/tarballs, while the supported direct-download route for this local stdio server is MCPB. Build and publish versioned MCPB artifact(s) containing an MCP-capable release first; then generate and validate `server.json`, authenticate as GitHub owner `sraj0501`, and publish only with explicit approval. |
| Anthropic Connectors Directory (desktop extension) | **Blocked** | Installable `.mcpb` with a valid `manifest.json`, privacy policy, clear documentation, working examples, and required `title` plus `readOnlyHint`/`destructiveHint` annotations on every tool, followed by Anthropic's desktop-extension submission form and review. The current MCPB manifest specification is version `0.3`; required fields include name, semantic version, description, author, and server launch configuration. | [Build an MCPB](https://claude.com/docs/connectors/building/mcpb), [directory submission requirements](https://claude.com/docs/connectors/building/submission), and [MCPB manifest specification](https://github.com/modelcontextprotocol/mcpb/blob/main/MANIFEST.md) | No DevTrack MCPB exists, and the current release does not contain MCP. The six tools are read-only in implementation but their declarations currently omit the directory-required `title` and `readOnlyHint` annotations. Package/security review, annotation work, examples, privacy-policy packaging, and macOS/Windows bundle testing must precede submission. |
| Smithery | **Blocked** | For local stdio servers, a prebuilt `.mcpb` bundle and listing metadata; hosted URL submissions instead require Streamable HTTP. | [Smithery publishing documentation](https://smithery.ai/docs/build/publish) | DevTrack exposes stdio only and has no MCPB. Do not present the Python HTTP API as an MCP endpoint. After an MCP-capable MCPB release exists, upload/publish requires an account and explicit external-publishing approval. |
| Glama | **Blocked pending eligibility and build proof** | GitHub OAuth by a maintainer with write/admin access; for its open-source-server path, Glama clones, builds, runs, and introspects the repository using a provided or inferred Dockerfile in an ephemeral microVM. | [Glama registry](https://glama.ai/mcp/servers) and [indexing methodology](https://glama.ai/mcp/methodology) | DevTrack uses a source-available Community License rather than an OSI license, so confirm eligibility for Glama's "open-source MCP servers" path first. The monorepo also lacks a Glama-specific build/launch definition for the local Go MCP command, and a disposable scan has no user's SQLite context. Repository verification and submission are external account actions; do not offer the server as hosted/remote. |
| mcpservers.org / `wong2/awesome-mcp-servers` | **Form-ready copy, submission held** | Server name, one-sentence description, GitHub/docs link, category, and contact email. The associated awesome list no longer accepts PRs. | [Submission form](https://mcpservers.org/submit) and [list repository](https://github.com/wong2/awesome-mcp-servers) | Wait for an MCP-capable public release so a visitor can install what the listing describes. The owner must supply/approve the contact email and authorize the form submission. Premium review is optional and would incur a fee. |
| `punkpeye/awesome-mcp-servers` | **PR copy ready, PR held** | A repository-linked server name, concise accurate description, appropriate category, alphabetical placement, one server per line, and a contributor PR. | [Contribution guide](https://github.com/punkpeye/awesome-mcp-servers/blob/main/CONTRIBUTING.md) | Wait for an MCP-capable public release. Forking, pushing, and opening the external PR require explicit authorization. Current automated checks may impose additional requirements at PR time, so re-check the live template/check output immediately before submission. |

## Approved factual copy

Use this copy only after an MCP-capable public release is available. Replace bracketed owner-only
fields before submission.

### Canonical short description

> Read local tickets, commits, pending actions, voice profile, and EOD context over MCP.

This is under the official registry's current 100-character description limit.

### Directory description

> DevTrack exposes six read-only, SQLite-backed tools over local stdio so MCP clients can retrieve
> the developer's active ticket, today's commits, pending actions, voice profile, ticket context,
> and EOD summary.

### Longer listing copy

> DevTrack is a local developer-context layer with an MCP `stdio` server built into the `devtrack`
> binary. Its six read-only tools retrieve the active ticket, today's commits, pending actions,
> inferred voice profile, ticket history, and an end-of-day summary from DevTrack's local SQLite
> database. The MCP path does not require the Python server or a hosted MCP endpoint. Install
> DevTrack, run `devtrack mcp setup`, reload the MCP client, and verify the protocol surface with
> `devtrack mcp test`.

### mcpservers.org form draft

- **Server name:** DevTrack
- **Short description:** Read local tickets, commits, pending actions, voice profile, and EOD
  context through six read-only MCP tools.
- **Link:** `https://github.com/sraj0501/Devtrack_`
- **Category:** Developer Tools (use Knowledge & Memory only if the form lacks Developer Tools)
- **Contact email:** `[OWNER TO SUPPLY]`

### `punkpeye/awesome-mcp-servers` entry draft

Suggested category: Developer Tools.

```markdown
- [sraj0501/Devtrack_](https://github.com/sraj0501/Devtrack_) 🏠 🏎️ - Local stdio server exposing six read-only tools for active-ticket, commit, pending-action, voice-profile, ticket, and EOD context from DevTrack's SQLite database.
```

Before using the entry, confirm the list's current emoji legend and exact alphabetical location.

### Anthropic Connectors Directory / Smithery summary

> Local developer context for Claude: active ticket, today's commits, pending actions, voice
> profile, ticket history, and EOD summary from DevTrack's SQLite database through six read-only
> MCP tools.

## Manifest decision

No `server.json` or `manifest.json` is included in this task. A valid official-registry manifest
must identify a real supported package version, and an MCPB manifest must launch files actually
present inside the bundle. The repository currently has neither an MCP-capable public release nor
an MCPB build artifact. Placeholder manifests would pass only superficial JSON checks while being
factually unpublishable, violating this task's requirement to avoid unsupported compatibility and
distribution claims.

Once release packaging is approved, the smallest honest sequence is:

1. For Anthropic directory eligibility, add the required human-readable tool titles and accurate
   read-only/destructive annotations, then prepare the privacy policy and working examples for
   review. Keep this directory-specific work separate from the official registry manifest.
2. Add reproducible per-platform MCPB packaging for the MCP-capable Go binary and validate each
   bundle with the then-current MCPB CLI/schema (currently manifest version `0.3`).
3. Install and smoke-test the bundles on their claimed platforms, including local database/config
   resolution and all six tools.
4. Publish the versioned bundles to a GitHub release and record their SHA-256 values.
5. Generate `server.json` with `mcp-publisher init`, use the matching MCPB release URL/hash and
   `stdio` transport, then run the publisher's validation without publishing.
6. Re-check every form/schema and obtain explicit approval before any login, upload, PR, form
   submission, or registry publish.

## Claims not approved for submission

Do not claim any of the following without new evidence:

- compatibility with every MCP client, Claude Desktop version, or operating system;
- sandboxing, formal security review, encryption, or zero filesystem access;
- a hosted/remote or Streamable HTTP MCP endpoint;
- MCP support in `v3.0.10`;
- public adoption, user counts, download counts, time savings, or accuracy rates;
- OSI-approved/open-source licensing.

## TASK-124 evidence audit

The post-generator prerequisite is **not met** as of 2026-08-28:

- TASK-143 has prepared one same-day `Evidence Reconciliation` entry for TASK-118–120. It records
  three upstream merges, green CI, unreleased status, and the SSH fetch friction, and it explicitly
  rules out unsupported install/adoption/time-saved/onboarding claims.
- That entry describes a documentation audit, not a lived implementation or user session. It has
  no measured time saved, ticket-update result, standup-generation result, onboarding run, user
  reaction, or task-level timing from the current seven-day window.
- The prior dated engineering sessions in the log are from June. One reconciliation snapshot is
  not the seven-day commit/friction/daily-summary source record required by the post-generator, and
  TASK-124 also depends on completion of TASK-121–123.
- The TASK-124 board criterion requires a current seven-day evidence window before generation.

Therefore no dev.to, Show HN, or LinkedIn drafts were generated. Before TASK-124 starts, append
task-level evidence for the recent Phase 9 work (commands/results, CI links, measured timing and
friction, and explicit release-vs-`dev` state) across the current seven-day window, complete its
TASK-121–123 dependencies, then run the post-generator workflow over only that window. The TASK-143
entry remains a useful claim boundary and upstream-status source, but is insufficient by itself.
