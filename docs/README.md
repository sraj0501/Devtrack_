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
| Basic capture and EOD acceptance | [`testing/basic-workflow.md`](testing/basic-workflow.md) |
| Incident/story/ticket mapping and correction | [`TICKET_MAPPING.md`](TICKET_MAPPING.md) |
| Time-tracking removal and context commands | [`WORK_TIME_INFERENCE.md`](WORK_TIME_INFERENCE.md) |
| v3.3.0 development changes | [`releases/v3.3.0.md`](releases/v3.3.0.md) |
| Current execution priorities | [`NEXT_STEPS.md`](NEXT_STEPS.md) and [`../agent-memory/execution-plan.md`](../agent-memory/execution-plan.md) |
| DevTrack Sage scope and completed implementation | [`DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md`](DEVTRACK_SAGE_IMPLEMENTATION_PLAN.md) |
| Sage executable parity inventory | [`SAGE_PORT_PARITY_MATRIX.md`](SAGE_PORT_PARITY_MATRIX.md) |
| Sage capture/logging and Codex-history qualification | [`SAGE_CAPTURE_HISTORY_QUALIFICATION.md`](SAGE_CAPTURE_HISTORY_QUALIFICATION.md) |
| Sage optional IDE parity qualification | [`SAGE_IDE_PARITY_QUALIFICATION.md`](SAGE_IDE_PARITY_QUALIFICATION.md) |
| Sage end-to-end acceptance qualification | [`SAGE_END_TO_END_ACCEPTANCE.md`](SAGE_END_TO_END_ACCEPTANCE.md) |
| Sage event and harness contracts | [`SAGE_EVENT_CONTRACT_V1.md`](SAGE_EVENT_CONTRACT_V1.md), [`SAGE_HARNESS_EVALUATION.md`](SAGE_HARNESS_EVALUATION.md), and [`SAGE_HARNESS_PLUGIN_CONTRACT.md`](SAGE_HARNESS_PLUGIN_CONTRACT.md) |
| Release-facing MCP listing copy | [`REGISTRY_SUBMISSION_PACKAGE.md`](REGISTRY_SUBMISSION_PACKAGE.md) |
| Public user documentation | [`../devtrack_wiki/wiki/wiki.html`](../devtrack_wiki/wiki/wiki.html) and [`../devtrack_wiki/wiki/docs/`](../devtrack_wiki/wiki/docs/) |

Stable v3.2.0 includes the earlier Sage roadmap, ticket mapping, former time inference, rolling
update channels, and the TASK-158 silent-Git correction. Its notes describe that historical release.
The v3.3.0 development line removes time tracking and fixes burst/branch capture and server report
visibility. Source fixes are on `dev` at `860b91b`; installed binaries and stable publication are
separate. The packaged `dev-dc39101` build passed isolated container and Windows no-send checks.
Use the basic-workflow plan to qualify the native installation, controlled delivery, and a
representative workday before relying on it for daily use.

## Historical records

- [`CLIENT_SERVER_DECOUPLING_PLAN.md`](CLIENT_SERVER_DECOUPLING_PLAN.md) records a completed
  migration. Its pre-migration prose is not a current ownership guide.
- [`split-manifest.md`](split-manifest.md) records the pre-split tree from TASK-041. Old paths,
  remotes, module names, licence text, and Git Sage prototypes are historical evidence only.
- [`releases/`](releases/) contains immutable release notes for the named versions.
- Dated validation evidence in [`END_TO_END_VALIDATION.md`](END_TO_END_VALIDATION.md) and
  [`WINDOWS_ADMIN_ACCEPTANCE.md`](WINDOWS_ADMIN_ACCEPTANCE.md) should not be interpreted as a claim
  that native installed workflow, controlled delivery, or media gates are complete.
- [`TASK_159_QUALIFICATION.md`](TASK_159_QUALIFICATION.md) records tests of the former time-tracking
  feature; its commands, test names, and integration gates are historical, not current requirements.
- [`testing/basic-workflow-source-acceptance.md`](testing/basic-workflow-source-acceptance.md)
  records the initial v3.2.0 failures, passing source/container regression, packaged Linux
  container acceptance, and isolated Windows no-send checkpoint. It does not qualify a native
  installation with real email/PM delivery.

## Sage naming

**DevTrack Sage** is the current Go-native cross-harness command-knowledge product. The legacy
repository chat/Git-operation experiment previously called **Git Sage** has been removed.
Independently useful Git helpers now live behind `devtrack git`; command capture and knowledge live
behind `devtrack sage`. Do not restore or document `sage ask`, `sage do`, `sage pr`,
`sage interactive`, `sage git`, free-form Sage questions, or `GIT_SAGE_*` configuration as current
Sage behavior. A few Python-server reads of those variable names remain only as legacy model
fallbacks.
