# DevTrack documentation map

This directory mixes current product contracts with a small number of explicitly historical
records. Use this map before treating a document as current implementation guidance.

## Current sources of truth

| Topic | Document |
|---|---|
| Product direction | [`../PRODUCT_BIBLE.md`](../PRODUCT_BIBLE.md) |
| Runtime architecture and ownership | [`ARCHITECTURE.md`](ARCHITECTURE.md) |
| Go↔Python HTTP contract | [`HTTP_API.md`](HTTP_API.md) |
| Installation and modes | [`INSTALLATION.md`](INSTALLATION.md) |
| Current execution priorities | [`NEXT_STEPS.md`](NEXT_STEPS.md) and [`../agent-memory/execution-plan.md`](../agent-memory/execution-plan.md) |
| DevTrack Sage scope and remaining work | [`DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md`](DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md) |
| Sage executable parity inventory | [`SAGE_PORT_PARITY_MATRIX.md`](SAGE_PORT_PARITY_MATRIX.md) |
| Sage event and harness contracts | [`SAGE_EVENT_CONTRACT_V1.md`](SAGE_EVENT_CONTRACT_V1.md), [`SAGE_HARNESS_EVALUATION.md`](SAGE_HARNESS_EVALUATION.md), and [`SAGE_HARNESS_PLUGIN_CONTRACT.md`](SAGE_HARNESS_PLUGIN_CONTRACT.md) |
| Release-facing MCP listing copy | [`REGISTRY_SUBMISSION_PACKAGE.md`](REGISTRY_SUBMISSION_PACKAGE.md) |
| Public user documentation | [`../devtrack_wiki/wiki/wiki.html`](../devtrack_wiki/wiki/wiki.html) and [`../devtrack_wiki/wiki/docs/`](../devtrack_wiki/wiki/docs/) |

The latest public release is **v3.1.1**. Newer `dev` work—most notably the DevTrack Sage capture,
model-free search, and initial structured-distillation boundary plus the TASK-158 silent-Git
correction—is unreleased and must be labelled as such.

## Historical records

- [`CLIENT_SERVER_DECOUPLING_PLAN.md`](CLIENT_SERVER_DECOUPLING_PLAN.md) records a completed
  migration. Its pre-migration prose is not a current ownership guide.
- [`split-manifest.md`](split-manifest.md) records the pre-split tree from TASK-041. Old paths,
  remotes, module names, licence text, and Git Sage prototypes are historical evidence only.
- [`releases/`](releases/) contains immutable release notes for the named versions.
- Dated validation evidence in [`END_TO_END_VALIDATION.md`](END_TO_END_VALIDATION.md) and
  [`WINDOWS_ADMIN_ACCEPTANCE.md`](WINDOWS_ADMIN_ACCEPTANCE.md) should not be interpreted as a claim
  that later packaged-build or media gates are complete.

## Sage naming

**DevTrack Sage** is the current Go-native cross-harness command-knowledge product. The legacy
repository chat/Git-operation experiment previously called **Git Sage** has been removed.
Independently useful Git helpers now live behind `devtrack git`; command capture and knowledge live
behind `devtrack sage`. Do not restore or document `sage ask`, `sage do`, `sage pr`,
`sage interactive`, `sage git`, free-form Sage questions, or `GIT_SAGE_*` configuration as current
Sage behavior. A few Python-server reads of those variable names remain only as legacy model
fallbacks.
