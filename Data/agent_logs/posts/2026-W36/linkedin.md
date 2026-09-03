# The useful part of AI automation happens after the session

Coding agents got much better while I was building DevTrack.

That made one part of the product less interesting: asking a model to generate a commit message or a
status update on demand is no longer unusual.

But it made another part more useful.

Agents remember a session. Work does not fit neatly inside one session. At the end of the day, the
real context is spread across repositories, commits, tickets, corrections, and decisions.

DevTrack now focuses on that gap: a local daemon that keeps work context underneath the agents, then
stages ticket updates and reports through one reviewable queue.

The trust boundary is intentional. Outbound actions show their confidence and wait for review. Local
Ollama remains the default model path. The Go-native monitoring and MCP path can work before the
optional Python server finishes bootstrapping.

DevTrack v3.1.0 is the first public release with the new onboarding, MCP, demo, and background
bootstrap work. Its five MCPB variants are executed on native Windows, macOS, and Linux runners
before release because “the code exists” and “a stranger can install the release successfully” are
different claims.

The next metric is not stars. It is the first useful issue or pull request from someone outside the
project.

Repository: https://github.com/sraj0501/Devtrack_
