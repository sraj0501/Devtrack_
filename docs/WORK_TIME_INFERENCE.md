# Time tracking removed

The owner removed this feature on 2026-10-21: measuring time spent on individual tasks enables
micromanagement and is outside DevTrack's purpose. This change is in the working tree; older
released binaries, including v3.2.0, still contain the former feature until upgraded.

DevTrack captures commits, ticket context, and activity timestamps for work reports. It does not
collect or infer durations, create work sessions, log hours, or export time metrics.

`devtrack work start <ticket-ref>` selects context only; `work stop` clears it; `work status`
shows the selected ticket. `work adjust` is removed. Use `devtrack eod` or `devtrack work report`
for commit-based reports. Existing duration tables remain inert for upgrade compatibility and
are not exposed or synchronized. No historical user database is deleted by this change.
