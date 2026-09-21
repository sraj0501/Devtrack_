---
name: Deterministic ticket mapping
description: Durable implementation contract for TASK-160 branch-to-ticket resolution
type: project
---

# Deterministic ticket-mapping contract

TASK-160 replaces permissive substring and last-ticket assignment with a deterministic, explainable
resolver. `PRODUCT_BIBLE.md` owns the product rule; this record owns the implementation plan.

## Fixed product decisions

### Canonical branch grammar

The default grammar is:

```text
<kind>/<ticket-key>-<number>-<slug>
```

- Default kinds: `feature`, `feat`, `fix`, `bugfix`, `hotfix`, `chore`, `docs`, `refactor`, `test`.
- `ticket-key`: uppercase workspace namespace configured during workspace setup.
- `number`: positive decimal integer.
- `slug`: required lowercase kebab-case summary.
- Matching is anchored to the full branch. A ticket-looking substring elsewhere is not a branch
  match.
- Default, release, and dependency branches may be ignored by workspace configuration. Ignoring a
  branch means no work mapping is attempted; it does not weaken the grammar elsewhere.

Examples:

```text
feature/PROJ-123-login
fix/ADO-456-null-check
hotfix/GH-789-regression
```

Workspace setup stores the ticket key and provider adapter. An advanced custom pattern must be a
full anchored branch regex with exactly one named `ticket` capture. Invalid or unanchored patterns
fail explicit setup/config validation; runtime reload keeps the last valid configuration and logs a
diagnostic rather than silently falling back to a broader pattern.

### Canonical and external identity

Never overload one string with both branch identity and provider routing:

| Platform | Canonical reference | External ID |
|---|---|---|
| Jira | `PROJ-123` | `PROJ-123` |
| GitHub | `GH-42` | issue number `42` |
| GitLab | `GL-18` | issue IID `18` |
| Azure DevOps | `ADO-456` | work item ID `456` |

The workspace binds the key to its configured PM platform and project. Resolution is offline after
configuration; network existence checks may enrich diagnostics but never gate local mapping.

### Authoritative precedence

1. Full canonical branch match.
2. One explicit first-line prefix (`PROJ-123: summary`) or one Git trailer (`Refs: PROJ-123`).
3. Explicit active ticket set for that workspace and repository.
4. Unlinked.

Rules:

- Branch evidence wins over every lower-priority signal.
- Prefix and trailer are equal priority. If both exist and disagree, the result is ambiguous.
- Two or more distinct references at the winning priority are ambiguous.
- Incidental IDs in commit prose are ignored.
- The previous mapped ticket is never an authoritative fallback.
- An ambiguous or contradictory result is recorded, not guessed.
- Merge-to-default may parse one canonical source branch from the merge subject. More than one
  candidate is ambiguous.

### Conflicts and outbound behavior

Effective mapping states are `linked`, `unlinked`, `conflict`, `corrected`, and `legacy`.
A canonical branch plus a contradictory lower-priority reference retains the branch as the effective
mapping but records `conflict`. No PM comment or state transition from a conflicted mapping may
auto-execute; it remains reviewable until corrected. An ambiguous winning signal has no effective
ticket and is stored as `unlinked` with conflict metadata.

DevTrack never prompts or blocks Git. Status, doctor, CLI/Telegram correction, and the pending queue
are where conflicts and unlinked commits become visible.

### LLM boundary

LLMs and recent-ticket ranking run only after deterministic resolution returns unlinked. Their
output is a candidate list, never an effective mapping. Store candidate ID, confidence, model/source,
and timestamp separately. Approval creates a correction event; rejection is negative learning
evidence. A later LLM run cannot overwrite a deterministic or corrected mapping.

## Persistence plan

Add mapping provenance without rewriting historical rows:

- Extend the effective commit mapping with canonical reference, provider external ID, source,
  confidence, state, normalized branch, and conflict indicator.
- Add an append-only correction table containing commit/trigger identity, previous and replacement
  reference, channel/actor, timestamp, and optional reason.
- Keep original evidence and the original mapping immutable; derive the effective mapping from the
  latest valid correction.
- Mark pre-migration linked rows as `legacy` with unknown provenance. Do not backfill them by running
  the new resolver against old branch/message data.
- Include mapping source, state, and confidence in Go-to-Python trigger payloads and MCP/read models
  where commit mappings are already exposed. Raw LLM candidate text does not enter MCP.

Exact schema names are chosen during implementation, but these semantics are mandatory. Go SQLite
remains the source of truth for commit mapping; Python does not infer a replacement.

## User-facing contract

Workspace onboarding displays and saves the convention. Add non-mutating inspection commands:

```text
devtrack ticket convention
devtrack ticket check [branch]
```

Add an explicit correction command with channel parity:

```text
devtrack ticket link <commit> <ticket-ref>
```

`status` and `doctor` show convention compliance, unlinked count, conflict count, and invalid
configuration. These commands report; they never rewrite a branch. Documentation uses the same
grammar and provider examples everywhere.

## Implementation slices

1. **Resolver and configuration:** typed result, anchored grammar, explicit prefix/trailer parser,
   workspace ticket key/provider adapter, strict validation, and hot-reload identity.
2. **Persistence and migration:** provenance/state/external identity, append-only corrections,
   legacy marking, query/read-model updates, and Go/Python contract fields.
3. **Runtime integration:** replace free-form scanning and last-ticket assignment; gate conflicted
   outbound work; preserve fail-open silent commit handling.
4. **Inspection and correction:** convention/check/link commands, status/doctor metrics, and CLI plus
   at least one non-TUI correction channel.
5. **Suggestion boundary:** optional LLM/cached-ticket candidates stored separately and routed
   through review; deterministic mapping remains model-free.
6. **Documentation and qualification:** onboarding, README/wiki, migration notes, and cross-platform
   end-to-end evidence.

Each slice is one reviewable PR targeting `dev`; TASK-160 may be decomposed into child board tasks
before dispatch. Do not combine it with TASK-158 silence changes or TASK-159 time inference.

## Required tests

- Table-driven grammar tests for every default kind, invalid case, custom pattern, and path edge.
- Precedence matrix covering branch, prefix, trailer, active state, and contradictions.
- Multiple-ticket ambiguity and merge-subject tests.
- Provider canonical-to-external normalization tests.
- Config reload tests proving ticket-contract changes update the correct monitor.
- Migration tests proving legacy rows are not reinterpreted.
- Correction audit, idempotency, and concurrency tests.
- Outbound tests proving conflicts and LLM-only candidates cannot auto-execute.
- Windows and Linux TTY/non-TTY tests proving invalid branches remain silent and Git succeeds.

## Exit gate

In a clean workspace, canonical branches map identically without a model or network; contradictory,
ambiguous, and nonconforming inputs never produce an unreviewed PM action; every mapping explains its
source and history; and the same convention appears in setup, CLI help, README, and wiki.
