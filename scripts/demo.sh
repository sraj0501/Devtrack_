#!/bin/sh
set -eu

mode="${1:---check}"
automation_arg="${2:-}"
automated="${DEMO_AUTOMATED:-false}"
stage_timeout_secs="${DEMO_STAGE_TIMEOUT_SECS:-120}"

usage() {
    printf '%s\n' "Usage: $0 [--check|--record] [--automated]"
}

case "$mode" in
    --check|--record) ;;
    *) usage >&2; exit 2 ;;
esac

case "$automation_arg" in
    '') ;;
    --automated) automated=true ;;
    *) usage >&2; exit 2 ;;
esac

case "$automated" in
    true|false) ;;
    *) printf '%s\n' 'DEMO_AUTOMATED must be true or false.' >&2; exit 2 ;;
esac

case "$stage_timeout_secs" in
    ''|*[!0-9]*|0) printf 'DEMO_STAGE_TIMEOUT_SECS must be a positive integer.\n' >&2; exit 2 ;;
esac

for command_name in devtrack git comm grep sort; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        printf 'Missing required command: %s\n' "$command_name" >&2
        exit 1
    fi
done

printf '%s\n' 'Checking DevTrack CLI surfaces...'
mcp_status=$(devtrack mcp status 2>&1) || {
    printf '%s\n' 'This demo requires an MCP-capable build from upstream dev (or a future release).' >&2
    printf '%s\n' 'The latest public release, v3.0.10, predates MCP.' >&2
    exit 1
}
if ! printf '%s\n' "$mcp_status" | grep -F 'get_active_context' >/dev/null 2>&1 || \
   ! printf '%s\n' "$mcp_status" | grep -F '6 registered' >/dev/null 2>&1; then
    printf '%s\n' 'This binary does not expose the six-tool MCP surface required by the demo.' >&2
    printf '%s\n' 'Build upstream dev; v3.0.10 is still the latest public release and predates MCP.' >&2
    exit 1
fi
printf '%s\n' "$mcp_status"
devtrack status

if [ "$mode" = "--check" ]; then
    printf '%s\n' 'Preflight complete. Run with --record after `devtrack doctor` reports the AI server ready.'
    exit 0
fi

demo_root=$(mktemp -d "${TMPDIR:-/tmp}/devtrack-demo.XXXXXX")
demo_name="devtrack-demo-$$"
workspace_added=false

cleanup() {
    if [ "$workspace_added" = true ]; then
        devtrack workspace remove "$demo_name" >/dev/null 2>&1 || true
    fi
    case "$demo_root" in
        "${TMPDIR:-/tmp}"/devtrack-demo.*) rm -rf -- "$demo_root" ;;
        *) printf 'Refusing to remove unexpected demo path: %s\n' "$demo_root" >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

pause_scene() {
    printf '\n%s\n' "$1"
    if [ "$automated" != true ]; then
        printf '%s' 'Press Enter to continue... '
        read answer
    fi
}

pause_scene 'Scene 1/5 — Go-native MCP status and self-test'
devtrack mcp status
devtrack mcp test

GIT_NO_DEVTRACK=1 git -C "$demo_root" init -q
GIT_NO_DEVTRACK=1 git -C "$demo_root" config user.name "DevTrack Demo"
GIT_NO_DEVTRACK=1 git -C "$demo_root" config user.email "demo@localhost"
printf '%s\n' 'credential-free demo' > "$demo_root/README.md"
GIT_NO_DEVTRACK=1 git -C "$demo_root" add README.md
GIT_NO_DEVTRACK=1 git -C "$demo_root" commit -q -m "initial demo repository"

devtrack workspace add "$demo_name" "$demo_root" --pm none
workspace_added=true

pause_scene 'Scene 2/5 — a normal commit on feature/DEMO-101-standup'
GIT_NO_DEVTRACK=1 git -C "$demo_root" switch -q -c feature/DEMO-101-standup
printf '%s\n' 'The standup follows the commit.' >> "$demo_root/README.md"
GIT_NO_DEVTRACK=1 git -C "$demo_root" add README.md
staging_pattern='staged .*confidence=|PM sync staged .*confidence='
staging_baseline="$demo_root/.staging-before"
staging_current="$demo_root/.staging-current"
(devtrack logs 2>&1 || true) | grep -E "$staging_pattern" | sort -u > "$staging_baseline" || true
GIT_NO_DEVTRACK=1 git -C "$demo_root" commit -m "DEMO-101: document standup outcome"
demo_hash=$(GIT_NO_DEVTRACK=1 git -C "$demo_root" rev-parse --short=12 HEAD)

pause_scene 'Scene 3/5 — real detection and queue-staging evidence from daemon logs'
attempt=0
matched=false
while [ "$attempt" -lt "$stage_timeout_secs" ]; do
    log_output=$(devtrack logs 2>&1 || true)
    printf '%s\n' "$log_output" | grep -E "$staging_pattern" | sort -u > "$staging_current" || true
    new_staging=$(comm -13 "$staging_baseline" "$staging_current")
    if printf '%s\n' "$log_output" | grep -F "$demo_hash" >/dev/null 2>&1 && \
       [ -n "$new_staging" ]; then
        printf '%s\n' "$log_output" | grep -F "$demo_hash" | tail -n 6
        printf '%s\n' "$new_staging" | tail -n 6
        matched=true
        break
    fi
    attempt=$((attempt + 1))
    sleep 1
done

if [ "$matched" != true ]; then
    printf 'Could not verify commit detection plus confidence-bearing staging within %s seconds.\n' "$stage_timeout_secs" >&2
    printf '%s\n' 'Run `devtrack doctor`, resolve readiness, and rerun the storyboard.' >&2
    exit 1
fi
devtrack queue list

pause_scene 'Scene 4/5 — on-demand EOD narrative; no email and no approval'
eod_output=$(devtrack eod)
printf '%s\n' "$eod_output"
if ! printf '%s\n' "$eod_output" | grep -E 'Queued as action [0-9]+' >/dev/null 2>&1; then
    printf '%s\n' 'EOD generation returned without proof that the report was staged.' >&2
    printf '%s\n' 'Run `devtrack doctor`, resolve queue readiness, and rerun the storyboard.' >&2
    exit 1
fi

pause_scene "Scene 5/5 — MCP context after today's real commit"
devtrack mcp test

printf '\n%s\n' 'Demo complete. The disposable PM-none workspace will now be removed.'
