# Work-item and ticket mapping

DevTrack associates an observed Git commit with **one work-item reference** in its configured
workspace. That item can be an incident, bug, story, task, or other type supported by the connected
project-management (PM) system. The item type and hierarchy come from the PM system; DevTrack's
branch resolver identifies the item, not its type. A `fix/` branch may point to a story, and a
`feature/` branch may point to an incident. DevTrack does not infer an incident or story merely from
the branch kind or commit wording.

This describes the deterministic resolver shipped in stable v3.2.0 and the v3.3.0 development
line. The latter removes time tracking and improves commit capture and EOD reporting. Check
`devtrack version` before applying version-specific workflow guidance.

## Configure a workspace

Run `devtrack setup` in the repository, then inspect its `workspaces.yaml`. Each enabled workspace
has a repository path, a PM adapter/project, and an uppercase `ticket_key`. The key is the local,
branch-safe namespace. It is separate from the PM platform's native item ID. For example:

```yaml
version: "1"
workspaces:
  - name: service-api
    path: /path/to/service-api
    pm_platform: azure
    ticket_key: ADO
    enabled: true
    ignore_branches: ["main"]
```

Use the actual path and configured project/credentials for that workspace. Keep credentials in
the registered environment file, not in `workspaces.yaml` or Git. A repository backed by Jira,
GitHub, or GitLab can use its own key and adapter. `pm_platform: none` keeps mapping local without
a PM destination. `devtrack ticket convention` shows the current workspace rule; run it from a
directory inside an enabled workspace.

## Name branches

The default full-branch format is `<kind>/<ticket-key>-<number>-<slug>`. The accepted kinds are
`feature`, `feat`, `fix`, `bugfix`, `hotfix`, `chore`, `docs`, `refactor`, and `test`. The number is
positive; the slug is required and uses lowercase letters/digits separated by hyphens. The key
must match the workspace's `ticket_key` when one is configured.

| Work item in the PM system | Example branch | Canonical reference | Native PM ID |
|---|---|---|---|
| Jira incident in project `OPS` | `hotfix/OPS-123-restore-login` | `OPS-123` | `OPS-123` |
| Azure DevOps user story 456 | `feature/ADO-456-add-export` | `ADO-456` | `456` |
| GitHub issue 42 labelled `bug` | `fix/GH-42-handle-timeout` | `GH-42` | `42` |
| GitLab issue IID 18 labelled `story` | `feature/GL-18-report-filter` | `GL-18` | `18` |

These examples are **separate workspaces**, each with its own PM routing and key. `OPS-123` is a
Jira key only if that workspace is configured for Jira. The branch kind describes the code change;
it does not create, classify, or change the PM item's type. Parent epic/story relationships are
read from the PM system when available; they are not encoded in this branch grammar. An advanced
`ticket_pattern` may replace the default grammar, but it must match the entire branch and contain
exactly one named `ticket` capture. Validate it with `devtrack ticket check <branch>`.

## How a commit gets its reference

DevTrack uses the first available deterministic level below. The winning result and its source
are recorded with the commit. Git itself is never blocked or prompted.

1. A full canonical branch match. For a merge commit, one parseable canonical source branch in
   the merge subject can supply this level.
2. An explicit first-line prefix such as `ADO-456: add export` or a `Refs: ADO-456` trailer on a
   later line. Prefix and trailer have equal priority.
3. A ticket explicitly selected for that workspace and repository with
   `devtrack work start ADO-456`. `devtrack work stop` clears it; this is context, not a timer.
4. Unlinked, when none of those signals resolves one reference.

Examples, assuming an Azure workspace with key `ADO`:

| Branch and commit message | Active context | Result |
|---|---|---|
| `feature/ADO-456-add-export`, `Add export` | `ADO-900` | `ADO-456`, from branch; the contradictory context is recorded as a conflict. |
| `chore/cleanup`, `ADO-456: add export` | none | `ADO-456`, from first-line prefix. |
| `chore/cleanup`, `Add export` followed by `Refs: ADO-456` | none | `ADO-456`, from trailer. |
| `chore/cleanup`, `Add export` | `ADO-456` | `ADO-456`, from explicit active context. |
| `chore/cleanup`, `Discuss ADO-456 in docs` | none | Unlinked; incidental prose is not a reference. |
| `chore/cleanup`, `ADO-456: add export` plus `Refs: ADO-900` | none | Unlinked with conflict metadata; equal-priority references disagree. |

A contradictory lower-priority signal does not replace the winning branch reference. It marks
the mapping as conflicted, and outbound PM comments/transitions from that mapping must remain
reviewable rather than auto-execute. Multiple different references at the winning level are
ambiguous and leave the commit unlinked with conflict metadata. DevTrack never reuses the previous
commit's ticket as a fallback. An LLM may suggest candidates for an unlinked commit, but a
suggestion is never the effective mapping without a user correction. A commit on a branch whose
name exactly matches an `ignore_branches` entry is skipped, so it will not be captured or mapped.
Do not assume wildcard entries match branch families.

## Inspect and correct

```sh
devtrack ticket convention
devtrack ticket check feature/ADO-456-add-export
devtrack work start ADO-456
devtrack work status
devtrack work stop
devtrack status
devtrack doctor
devtrack ticket link <commit-hash> ADO-456
```

`ticket check` reports whether the branch conforms; it does not rename it. `status` and `doctor`
surface recent unlinked/conflicted counts and configuration errors. `ticket link` makes an explicit
correction for an observed commit in an enabled workspace. It preserves the original evidence and
records an append-only correction; it does not rewrite Git history. Older linked records with
unknown provenance remain labelled `legacy` rather than being re-resolved under newer rules.

Commits and their mapping feed ticket-grouped EOD reports. Outbound PM updates and report delivery
are staged for review; inspect the destination and content before approval. For the v3.3.0
development line, commits made while the daemon is stopped are **not guaranteed to be caught up**
after restart. Keep it running during work and reconcile expected commit hashes against the EOD
before relying on unattended reporting. See the [basic workflow test plan](testing/basic-workflow.md).
