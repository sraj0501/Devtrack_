# Getting started with DevTrack

DevTrack watches Git repositories in the background. When you commit, it records work locally,
links the commit to a configured work item when there is reliable evidence, and can prepare ticket
updates and end-of-day (EOD) reports for review. Normal `git commit` never waits for a model or
project-management (PM) service.

## What you can use

| If you want to... | Start with... | What to expect |
|---|---|---|
| Record commits against incidents, stories, bugs, or tasks | [Quick Start](QUICK_START.md), then [Ticket Mapping](../../../docs/TICKET_MAPPING.md) | The Go daemon observes a configured repository and stores mapping evidence in local SQLite. |
| Inspect prepared updates before delivery | [Usage Guide](USAGE_GUIDE.md) | `devtrack queue list` shows staged actions; approval is a delivery step. |
| Generate a narrative of today's work | [Usage Guide](USAGE_GUIDE.md) | Managed or external Python service required; generation can stage a report. |
| Give an MCP-capable assistant local work context | [CLI Quickstart](CLI_QUICKSTART.md) | `devtrack mcp setup` and `devtrack mcp test` use local, read-only tools. |
| Add more repositories or a PM integration | [Configuration](CONFIGURATION.md) | Each workspace has its own path, ticket key, and PM routing. |

The **Go client** provides Git observation, local SQLite history, ticket resolution, the queue,
MCP, and terminal commands. The optional **Python server** uses PostgreSQL for generated reports
and other AI features. You can begin with a local-only PM choice and add a connector later.

## Your route through the wiki

1. [Install](INSTALLATION.md) the binary for your operating system and check its version.
2. Follow the [Quick Start](QUICK_START.md) to configure one repository, start the daemon, make
   a normal commit, and inspect the result.
3. Read [Ticket Mapping](../../../docs/TICKET_MAPPING.md) to see how an incident, story, bug, or
   task is selected, including conflicts and corrections.
4. Use the [Usage Guide](USAGE_GUIDE.md) for queue review, EOD reporting, and daily commands.
   [Troubleshooting](TROUBLESHOOTING.md) explains recovery if setup or a service fails.

The latest public release is **v3.2.0**. The v3.3.0 development line removes time tracking and
changes commit capture and report visibility. A rolling development binary identifies itself as
`dev-<commit>`; use `devtrack version` and follow the behavior of the binary you installed. For
daily unattended use, consult [Verification](VERIFICATION.md) and qualify the exact packaged build
and delivery setup on your machine.
