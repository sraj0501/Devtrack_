# DevTrack MCP registry submission package

_Research re-verified: 2026-09-04. v3.1.1 and the official MCP Registry record are public;
third-party directory submissions and launch posts remain unpublished._

## Release and capability facts

- The MCP implementation is on `main`: a local `stdio` server invoked with `devtrack mcp`.
- It negotiates the finalized handshake protocol through `2025-11-25` (while retaining older
  handshake compatibility) and exposes six read-only tools backed by the Go
  client's local SQLite database: `get_active_context`, `get_today_commits`,
  `get_pending_actions`, `get_voice_profile`, `get_ticket_context`, and `get_eod_summary`.
- The MCP path does not require the Python server or a network request. It does require an
  installed DevTrack binary and the user's local DevTrack data/configuration.
- The latest public release is `v3.1.1`. It contains the MCP command plus five native MCPB bundles
  for Windows amd64, macOS amd64/arm64, and Linux amd64/arm64. All bundles passed native CI smoke
  tests; the release's 10 executable/MCPB payloads independently matched `checksums.txt`.
- `io.github.sraj0501/devtrack` version 3.1.1 is active and latest in the official MCP Registry.
- DevTrack is distributed under its Community License/EULA in `TERMS.md`, not an OSI license.
  Directory forms and manifests must use that exact description and must not label it MIT,
  Apache-2.0, or generally "open source."

## Registry matrix

| Target | Eligibility now | Required fields or artifact | Authoritative submission URL | Blocker / next action |
|---|---|---|---|---|
| Official MCP Registry | **Published** | Publicly downloadable supported packages plus validated `server.json`; MCPB release URLs include SHA-256 values and `stdio` transport. | [Official publishing quickstart](https://modelcontextprotocol.io/registry/quickstart), [supported package types](https://modelcontextprotocol.io/registry/package-types), and [public record](https://registry.modelcontextprotocol.io/v0.1/servers?search=io.github.sraj0501%2Fdevtrack&version=latest) | Published through GitHub OIDC. The active latest record is `io.github.sraj0501/devtrack` 3.1.1 with all five MCPB packages; registry hashes match the release checksums. |
| Anthropic Connectors Directory (desktop extension) | **Packaging-compliant; desktop review and owner form required** | Installable `.mcpb` with a valid `manifest.json`, privacy policy, clear documentation, working examples, and required `title` plus `readOnlyHint`/`destructiveHint` annotations on every tool, followed by Anthropic's desktop-extension submission form and review. | [Build an MCPB](https://claude.com/docs/connectors/building/mcpb), [directory submission requirements](https://claude.com/docs/connectors/building/submission), [pre-submission checklist](https://claude.com/docs/connectors/building/review-criteria), and [MCPB manifest specification](https://github.com/modelcontextprotocol/mcpb/blob/main/MANIFEST.md) | v3.1.1 declares `privacy_policies`, includes a bundled Privacy Policy section, and passes official CLI plus native execution checks. Anthropic also requires every tool to be exercised with MCP Inspector and as a Claude custom connector; that interactive desktop evidence and the authenticated owner form are unavailable in this environment. |
| Smithery | **Artifact-ready; account required** | For local stdio servers, a prebuilt `.mcpb` bundle and listing metadata; hosted URL submissions instead require Streamable HTTP. | [Smithery publishing documentation](https://smithery.ai/docs/build/publish) | v3.1.1 supplies local stdio MCPBs. Do not present the Python HTTP API as MCP. Upload/publication remains blocked by the absence of Smithery account credentials in this environment. |
| Glama | **Blocked pending eligibility and build proof** | GitHub OAuth by a maintainer with write/admin access; for its open-source-server path, Glama clones, builds, runs, and introspects the repository using a provided or inferred Dockerfile in an ephemeral microVM. | [Glama registry](https://glama.ai/mcp/servers) and [indexing methodology](https://glama.ai/mcp/methodology) | DevTrack uses a source-available Community License rather than an OSI license, so confirm eligibility for Glama's "open-source MCP servers" path first. The monorepo also lacks a Glama-specific build/launch definition for the local Go MCP command, and a disposable scan has no user's SQLite context. Repository verification and submission are external account actions; do not offer the server as hosted/remote. |
| mcpservers.org / `wong2/awesome-mcp-servers` | **Form-ready; contact/login required** | Server name, one-sentence description, GitHub/docs link, category, and contact email. The associated awesome list no longer accepts PRs. | [Submission form](https://mcpservers.org/submit) and [list repository](https://github.com/wong2/awesome-mcp-servers) | The release gate is clear. Submission still needs an owner-approved contact email and an authenticated interactive form session. Premium review is optional and must not be purchased implicitly. |
| `punkpeye/awesome-mcp-servers` | **PR-ready; GitHub authentication required** | A repository-linked server name, concise accurate description, appropriate category, alphabetical placement, one server per line, and a contributor PR. | [Contribution guide](https://github.com/punkpeye/awesome-mcp-servers/blob/main/CONTRIBUTING.md) | The release gate and external-publication authorization are clear. The GitHub CLI owner token expired before the fork/PR step, so submission remains pending authentication and a fresh check of the live contribution rules. |

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

An MCPB `manifest.json` template and reproducible packager live under `mcpb/` and `scripts/`.
Release automation creates platform-specific bundles containing the matching Go executable; CI
extracts and runs each native target and validates its manifest with the official MCPB CLI.
`scripts/generate_server_json.py` creates release metadata from the published bundle
hashes; the release workflow validates it and publishes it to the official registry through OIDC.

For future releases, preserve this sequence: privacy/examples review, five native bundle smokes,
published hashes, validated `server.json`, release creation, then official-registry publication.
Re-check third-party forms and contribution rules immediately before each owner-authenticated action.

## Claims not approved for submission

Do not claim any of the following without new evidence:

- compatibility with every MCP client, Claude Desktop version, or operating system;
- sandboxing, formal security review, encryption, or zero filesystem access;
- a hosted/remote or Streamable HTTP MCP endpoint;
- MCP support in releases before `v3.1.0`;
- public adoption, user counts, download counts, time savings, or accuracy rates;
- OSI-approved/open-source licensing.

## TASK-124 evidence outcome

TASK-124 completed on 2026-09-02 after the engineer log captured a current seven-day evidence
window covering TASK-121–123 and TASK-143–147. Review-only dev.to, Show HN, and LinkedIn drafts
are stored under `Data/agent_logs/posts/2026-W36/`.

The drafts remain deliberately unpublished because no authenticated channel sessions are available
in this environment. They now describe public v3.1.1, avoid adoption and time-saved claims that the
repository cannot prove, and link to the released artifacts. TASK-150 cleared the technical release
gate but did not create external-usage evidence; launch copy must keep that boundary.
