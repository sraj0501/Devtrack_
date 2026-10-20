# DevTrack MCP registry submission package

_Release state checked: 2026-10-21. v3.2.0 is the active release candidate; v3.1.1 and its official
MCP Registry record remain public until TASK-171 promotion, tagging, and publication complete.
Third-party directory submissions and launch posts remain unpublished._

## Release and capability facts

- The public MCP implementation is on `main`: a local `stdio` server invoked with `devtrack mcp`.
- It negotiates the finalized handshake protocol through `2025-11-25` (while retaining older
  handshake compatibility) and exposes ten read-only tools backed by the Go client's local SQLite
  database and deterministic Sage Markdown: `get_active_context`, `get_today_commits`,
  `get_pending_actions`, `get_voice_profile`, `get_ticket_context`, and `get_eod_summary`.
  `sage_search`, `sage_topics`, `sage_get_entry`, and `sage_status` expose current published
  knowledge and bounded local status without raw capture data.
- The MCP path does not require the Python server or a network request. It does require an
  installed DevTrack binary and the user's local DevTrack data/configuration.
- The v3.2.0 candidate adds the four Sage tools to the six public work-context tools. Its release
  gate requires five native MCPB smoke jobs, matching checksums, and validated `server.json` before
  publication.
- The latest public release and official MCP Registry version remain `v3.1.1` until those gates
  pass and the verified `main` commit is tagged.
- DevTrack is distributed under its Community License/EULA in `TERMS.md`, not an OSI license.
  Directory forms and manifests must use that exact description and must not label it MIT,
  Apache-2.0, or generally "open source."

## Registry matrix

| Target | Eligibility now | Required fields or artifact | Authoritative submission URL | Blocker / next action |
|---|---|---|---|---|
| Official MCP Registry | **v3.1.1 published; v3.2.0 pending release** | Publicly downloadable supported packages plus validated `server.json`; MCPB release URLs include SHA-256 values and `stdio` transport. | [Official publishing quickstart](https://modelcontextprotocol.io/registry/quickstart), [supported package types](https://modelcontextprotocol.io/registry/package-types), and [public record](https://registry.modelcontextprotocol.io/v0.1/servers?search=io.github.sraj0501%2Fdevtrack&version=latest) | The active latest record is `io.github.sraj0501/devtrack` 3.1.1. The tag-driven release workflow will publish 3.2.0 through GitHub OIDC only after candidate qualification and promotion. |
| Anthropic Connectors Directory (desktop extension) | **Packaging-compliant; desktop review and owner form required** | Installable `.mcpb` with a valid `manifest.json`, privacy policy, clear documentation, working examples, and required `title` plus `readOnlyHint`/`destructiveHint` annotations on every tool, followed by Anthropic's desktop-extension submission form and review. | [Build an MCPB](https://claude.com/docs/connectors/building/mcpb), [directory submission requirements](https://claude.com/docs/connectors/building/submission), [pre-submission checklist](https://claude.com/docs/connectors/building/review-criteria), and [MCPB manifest specification](https://github.com/modelcontextprotocol/mcpb/blob/main/MANIFEST.md) | Public v3.1.1 declares `privacy_policies` and passed official CLI plus native execution checks. Reconfirm the v3.2.0 candidate evidence after its hosted native jobs pass. Interactive desktop evidence and the authenticated owner form remain unavailable in this environment. |
| Smithery | **Artifact-ready; account required** | For local stdio servers, a prebuilt `.mcpb` bundle and listing metadata; hosted URL submissions instead require Streamable HTTP. | [Smithery publishing documentation](https://smithery.ai/docs/build/publish) | Public v3.1.1 supplies local stdio MCPBs; v3.2.0 publication is pending. Do not present the Python HTTP API as MCP. Upload/publication remains blocked by the absence of Smithery account credentials in this environment. |
| Glama | **Approved; exact path/PR badge pending** | GitHub OAuth by a maintainer with write/admin access; Glama clones, builds, runs, and introspects the repository using a Dockerfile supplied in its submission UI. | [Glama registry](https://glama.ai/mcp/servers) and [indexing methodology](https://glama.ai/mcp/methodology) | Glama's admins approved the owner's submission using `Dockerfile.mcp`. Record the exact approved listing path before adding its score badge to PR #13608; do not guess slug normalization or present the Python backend as hosted MCP. |
| mcpservers.org / `wong2/awesome-mcp-servers` | **Form-ready; contact/login required** | Server name, one-sentence description, GitHub/docs link, category, and contact email. The associated awesome list no longer accepts PRs. | [Submission form](https://mcpservers.org/submit) and [list repository](https://github.com/wong2/awesome-mcp-servers) | The release gate is clear. Submission still needs an owner-approved contact email and an authenticated interactive form session. Premium review is optional and must not be purchased implicitly. |
| `punkpeye/awesome-mcp-servers` | **Submitted; checks pass** | A repository-linked server name, concise accurate description, appropriate category, alphabetical placement, one server per line, and a contributor PR. | [PR #13608](https://github.com/punkpeye/awesome-mcp-servers/pull/13608) and [contribution guide](https://github.com/punkpeye/awesome-mcp-servers/blob/main/CONTRIBUTING.md) | Submitted under Developer Tools on 2026-09-04 after rechecking the live rules. The automated `check-submission` job passed; maintainer review/merge remains external. |

## Approved factual copy

Use this copy only after an MCP-capable public release is available. Replace bracketed owner-only
fields before submission.

### Canonical short description

> Read local work context and published Sage command knowledge over MCP.

This is under the official registry's current 100-character description limit.

### Directory description

> DevTrack exposes ten read-only tools over local stdio so MCP clients can retrieve the developer's
> active work context and current published Sage command knowledge from local SQLite and Markdown.

### Longer listing copy

> DevTrack is a local developer-context layer with an MCP `stdio` server built into the `devtrack`
> binary. Its ten read-only tools retrieve work context, pending actions, voice profile, ticket
> history, an end-of-day summary, and current published Sage command knowledge from local SQLite and
> Markdown. The MCP path does not require the Python server or a hosted MCP endpoint. Install
> DevTrack, run `devtrack mcp setup`, reload the MCP client, and verify the protocol surface with
> `devtrack mcp test`.

### mcpservers.org form draft

- **Server name:** DevTrack
- **Short description:** Read local tickets, commits, pending actions, voice profile, and EOD
  context and published Sage knowledge through ten read-only MCP tools.
- **Link:** `https://github.com/sraj0501/Devtrack_`
- **Category:** Developer Tools (use Knowledge & Memory only if the form lacks Developer Tools)
- **Contact email:** `[OWNER TO SUPPLY]`

### `punkpeye/awesome-mcp-servers` entry draft

Suggested category: Developer Tools.

```markdown
- [sraj0501/Devtrack_](https://github.com/sraj0501/Devtrack_) 🏠 🏎️ - Local stdio server exposing ten read-only tools for work context and published Sage command knowledge from local SQLite and Markdown. [![sraj0501/Devtrack_ MCP server](https://glama.ai/mcp/servers/sraj0501/Devtrack_/badges/score.svg)](https://glama.ai/mcp/servers/sraj0501/Devtrack_)

Glama admin approval is complete. Before adding the badge to the live PR, copy the exact path shown
on the approved listing. If Glama normalizes the repository slug differently, use that path instead
of assuming `sraj0501/Devtrack_`.
```

Before using the entry, confirm the list's current emoji legend and exact alphabetical location.

### Anthropic Connectors Directory / Smithery summary

> Local developer context for Claude: active ticket, today's commits, pending actions, voice
> profile, ticket history, EOD summary, and published Sage command knowledge from local SQLite and
> Markdown through ten read-only MCP tools.

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
