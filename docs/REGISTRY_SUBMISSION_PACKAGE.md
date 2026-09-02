# DevTrack MCP registry submission package

_Research verified: 2026-09-02. Local artifacts only; nothing in this document has been
submitted or published._

## Release and capability facts

- The MCP implementation is on `main`: a local `stdio` server invoked with `devtrack mcp`.
- It negotiates the finalized handshake protocol through `2025-11-25` (while retaining older
  handshake compatibility) and exposes six read-only tools backed by the Go
  client's local SQLite database: `get_active_context`, `get_today_commits`,
  `get_pending_actions`, `get_voice_profile`, `get_ticket_context`, and `get_eod_summary`.
- The MCP path does not require the Python server or a network request. It does require an
  installed DevTrack binary and the user's local DevTrack data/configuration.
- The latest public release is `v3.0.10`. That tag does **not** contain `mcp_cmd.go`; the MCP work
  landed later (TASK-098–101, completed on 2026-06-24). Registry copy must therefore describe the
  MCP-capable `main` state as unreleased until a newer release is published.
- DevTrack is distributed under its Community License/EULA in `TERMS.md`, not an OSI license.
  Directory forms and manifests must use that exact description and must not label it MIT,
  Apache-2.0, or generally "open source."

## Registry matrix

| Target | Eligibility now | Required fields or artifact | Authoritative submission URL | Blocker / next action |
|---|---|---|---|---|
| Official MCP Registry (preview) | **Blocked** | Publicly downloadable supported package plus `server.json`; GitHub-authenticated names use `io.github.<owner>/<server>`; metadata includes name, description, repository, version, package identifier, and transport. For MCPB, include a GitHub/GitLab release URL and SHA-256. | [Official publishing quickstart](https://modelcontextprotocol.io/registry/quickstart) and [supported package types](https://modelcontextprotocol.io/registry/package-types) | The registry hosts metadata, not binaries. Existing release artifacts are generic executables/tarballs; the next release workflow is now prepared to add MCPBs. Publish and install-test an MCP-capable release first; then generate and validate `server.json`, authenticate as GitHub owner `sraj0501`, and publish only with explicit approval. |
| Anthropic Connectors Directory (desktop extension) | **Blocked on release and install testing** | Installable `.mcpb` with a valid `manifest.json`, privacy policy, clear documentation, working examples, and required `title` plus `readOnlyHint`/`destructiveHint` annotations on every tool, followed by Anthropic's desktop-extension submission form and review. The current MCPB manifest specification is version `0.3`; required fields include name, semantic version, description, author, and server launch configuration. | [Build an MCPB](https://claude.com/docs/connectors/building/mcpb), [directory submission requirements](https://claude.com/docs/connectors/building/submission), and [MCPB manifest specification](https://github.com/modelcontextprotocol/mcpb/blob/main/MANIFEST.md) | The source now declares tool titles/safety annotations and reproducibly builds per-platform MCPBs, with official CLI validation enforced in CI. v3.0.10 still contains neither MCP nor those bundles. Privacy-policy review plus macOS, Linux, and Windows install/smoke testing must precede submission. |
| Smithery | **Blocked** | For local stdio servers, a prebuilt `.mcpb` bundle and listing metadata; hosted URL submissions instead require Streamable HTTP. | [Smithery publishing documentation](https://smithery.ai/docs/build/publish) | DevTrack exposes stdio only and has no published MCPB; source packaging is now ready for the next release. Do not present the Python HTTP API as an MCP endpoint. After an MCP-capable MCPB release exists, upload/publish requires an account and explicit external-publishing approval. |
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

An MCPB `manifest.json` template and reproducible packager now live under `mcpb/` and `scripts/`.
Release automation creates platform-specific bundles containing the matching Go executable; CI
extracts each platform variant and validates its generated manifest with the official MCPB CLI.
No `server.json` is included yet because the official registry manifest must identify a real,
publicly downloadable MCP-capable release and checksum. v3.0.10 cannot honestly fill those fields.

Before release and registry publication, the smallest honest sequence is:

1. Prepare the privacy policy and working examples for directory review. The human-readable tool
   titles and read-only/destructive annotations are now present.
2. Install and smoke-test the generated bundles on their claimed platforms, including local database/config
   resolution and all six tools.
3. Publish the versioned bundles to a GitHub release and record their SHA-256 values.
4. Generate `server.json` with `mcp-publisher init`, use the matching MCPB release URL/hash and
   `stdio` transport, then run the publisher's validation without publishing.
5. Re-check every form/schema and obtain explicit approval before any login, upload, PR, form
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
