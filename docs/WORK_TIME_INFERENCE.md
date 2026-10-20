# Local work-time estimates

TASK-159 development behavior: the daemon records time evidence when it observes
a commit in a monitored repository. No duration prompt, model, or server is needed.
These are estimates from sparse observations, not a measurement of all work.

The default policy groups observations for the same repository, workspace, and
ticket when successive observations are at most 45 minutes apart. Each edge gets
up to 15 minutes of padding. A single observation therefore estimates 30 minutes.
Windows stop at local midnight and are capped at eight hours; larger gaps and
ticket changes start another window. New windows and extensions are clipped against recorded intervals in that
repository/workspace so their padding is not counted twice. An extension cannot
bridge another session. Late observations remain separate evidence rather than
rewriting history; fully covered observations add zero minutes. A ticket switch
can receive only the unused trailing padding when its timestamp falls inside an
existing window.
Observation order and the recorded timestamps determine the result.

Confidence is 0.50 for one observation, 0.75 for two, and 0.90 for three or more.
It describes evidence density, not the probability that the estimate is exact.
Unlinked commits retain unlinked time; an estimate cannot invent a ticket mapping.

`devtrack work status` shows measured or adjusted minutes, measurement source, and
confidence. Explicit commands remain optional:

```sh
devtrack work start PROJ-123
devtrack work stop
devtrack work adjust 45
```

Start records an explicit session for the current monitored Git repository and
selects a workspace-valid ticket. Branch and explicit commit references retain
their higher mapping precedence. Matching commits attach to that session. Starting a session removes overlapping
trailing inferred padding atomically and rejects a second active session. A newer
commit for a different ticket closes the explicit session at its last evidence;
a late contradictory commit cannot close or rewind it. Active explicit intervals
reserve their time so late inferred windows cannot double-count it.
Explicit stop measures elapsed time through the user's stop action. Automatic
idle closure and EOD closure end at the last observed activity; a session with
only a start observation measures zero minutes when automatically closed.
`WORK_SESSION_AUTO_STOP_MINUTES` controls the idle threshold (zero disables it).
Automatic closure preserves the selected ticket until an explicit stop or new
selection, so inference can continue after a long gap without another prompt.

Adjust changes the active session, or today's latest session. The override stays
separate from `duration_minutes`; each correction appends its previous and new
values and the measured duration at correction time to the local audit table.
The measured estimate may evolve with later evidence or removal of trailing
padding, while its audit snapshot and the adjusted value remain intact (including
an explicit zero). Older audit rows have no measured snapshot. Adjusted totals are
user overrides and may exceed the non-overlapping measured intervals.
SQLite stores evidence and windows across daemon restarts, and replaying a commit
does not add time again.

Time evidence contains only the observation kind/ID, timestamp, repository,
workspace, ticket, and associated session. It captures no keystrokes, window
activity, command output, or file contents. Raw time evidence is local SQLite
data and is not added to server synchronization. Existing optional session sync
retains its opt-in boundary.

## Validation

Run `scripts/e2e.ps1` on Windows or `sh scripts/e2e.sh` on Linux. These isolated,
no-send tests start the real daemon, create a Git commit, wait for an inferred
session in `work status`, apply a 45-minute correction, restart the daemon and
compare the persisted status, then exercise explicit start/stop while checking
that the earlier correction remains. Both lanes also check commit visibility
through MCP and clean up their temporary daemon and workspace.

For WSL without a Go toolchain, cross-compile the client with `GOOS=linux`,
`GOARCH=amd64`, and `CGO_ENABLED=0`, then set `DEVTRACK_E2E_BINARY` to that binary's
Linux path when running `scripts/e2e.sh`. The default Linux lane builds from
source. Clock-controlled database tests separately cover overlap, late evidence,
ticket changes, idle/EOD closure, replay, and measured correction snapshots.
