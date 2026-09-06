#!/bin/sh
set -eu

timeout_secs="${DEVTRACK_E2E_TIMEOUT_SECS:-60}"
case "$timeout_secs" in
    ''|*[!0-9]*|0) printf '%s\n' 'DEVTRACK_E2E_TIMEOUT_SECS must be a positive integer.' >&2; exit 2 ;;
esac

for command_name in go git grep mktemp; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        printf 'Missing required command: %s\n' "$command_name" >&2
        exit 1
    fi
done

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/devtrack-e2e.XXXXXX")
state_root="$test_root/state"
workspace="$test_root/workspace"
binary="$test_root/bin/devtrack"
env_file="$test_root/devtrack.env"
daemon_started=false

cleanup() {
    if [ "$daemon_started" = true ] && [ -x "$binary" ]; then
        DEVTRACK_ENV_FILE="$env_file" XDG_DATA_HOME="$state_root/xdg" "$binary" stop >/dev/null 2>&1 || true
    fi
    case "$test_root" in
        "${TMPDIR:-/tmp}"/devtrack-e2e.*) rm -rf -- "$test_root" ;;
        *) printf 'Refusing to remove unexpected E2E path: %s\n' "$test_root" >&2 ;;
    esac
}
trap cleanup EXIT HUP INT TERM

mkdir -p "$state_root/db" "$state_root/logs" "$state_root/pids" \
    "$state_root/configs" "$state_root/learning" "$workspace" "$(dirname "$binary")"

printf '%s\n' 'Building the Linux DevTrack binary...'
(cd "$repo_root/devtrack_client" && go build -o "$binary" .)

GIT_NO_DEVTRACK=1 git -C "$workspace" init -q
GIT_NO_DEVTRACK=1 git -C "$workspace" config user.name 'DevTrack E2E'
GIT_NO_DEVTRACK=1 git -C "$workspace" config user.email 'e2e@localhost'
printf '%s\n' 'isolated e2e workspace' > "$workspace/README.md"
GIT_NO_DEVTRACK=1 git -C "$workspace" add README.md
GIT_NO_DEVTRACK=1 git -C "$workspace" commit -q -m 'initial e2e repository'

ipc_port=$((39000 + ($$ % 4000)))
http_port=$((ipc_port + 1))
server_port=$((ipc_port + 2))
cat > "$env_file" <<EOF
PROJECT_ROOT=$workspace
DEVTRACK_HOME=$state_root
DEVTRACK_WORKSPACE=$workspace
WORKSPACES_FILE=$state_root/workspaces.yaml
DATABASE_DIR=$state_root/db
LOG_DIR=$state_root/logs
PID_DIR=$state_root/pids
CONFIG_DIR_PATH=$state_root/configs
LEARNING_DIR_PATH=$state_root/learning
CLI_BINARY_NAME=devtrack
CONFIG_FILE_NAME=config.yaml
DATABASE_FILE_NAME=devtrack.db
PID_FILE_NAME=daemon.pid
LOG_FILE_NAME=daemon.log
LEARNING_DIR_NAME=learning
CONFIG_DIR_NAME=.devtrack
CLI_APP_NAME=DevTrack-E2E
CLI_DAEMON_NAME=devtrack-e2e
DEVTRACK_SERVER_MODE=lightweight
DEVTRACK_SERVER_URL=http://127.0.0.1:$server_port
DEVTRACK_TLS=false
DEVTRACK_API_KEY=
IPC_HOST=127.0.0.1
IPC_PORT=$ipc_port
IPC_CONNECT_TIMEOUT_SECS=2
DEVTRACK_SERVER_HTTP_PORT=$http_port
PROMPT_INTERVAL=30
WORK_HOURS_ONLY=false
WORK_START_HOUR=0
WORK_END_HOUR=23
TIMEZONE=UTC
LOG_LEVEL=info
AUTO_SYNC=false
OUTPUT_TYPE=console
DAILY_REPORT_TIME=18:00
WEEKLY_REPORT_DAY=Friday
SEND_ON_TRIGGER=false
SEND_DAILY_SUMMARY=false
TEAMS_MENTION_USER=false
LEARNING_DEFAULT_DAYS=30
SERVER_EVENT_SYNC_ENABLED=false
TICKET_SYNC_ON_START=false
QUEUE_POLL_INTERVAL_SECS=60
HEALTH_CHECK_INTERVAL_SECS=60
VOICE_SYNC_INTERVAL_HOURS=24
DEVTRACK_AUTO_ACCEPT_TERMS=1
PYTHONIOENCODING=utf-8
EOF

printf '%s\n' 'Starting an isolated no-send daemon...'
DEVTRACK_ENV_FILE="$env_file" GIT_NO_DEVTRACK=1 XDG_DATA_HOME="$state_root/xdg" "$binary" workspace add e2e "$workspace" --pm none
DEVTRACK_ENV_FILE="$env_file" GIT_NO_DEVTRACK=1 XDG_DATA_HOME="$state_root/xdg" "$binary" start
daemon_started=true

deadline=$(( $(date +%s) + timeout_secs ))
while [ ! -f "$state_root/pids/daemon.pid" ]; do
    if [ "$(date +%s)" -ge "$deadline" ]; then
        printf '%s\n' 'Daemon PID file was not created before the timeout.' >&2
        exit 1
    fi
    sleep 1
done

GIT_NO_DEVTRACK=1 git -C "$workspace" switch -q -c feature/DEMO-201-automated-e2e
printf '%s\n' 'observed by the real daemon' >> "$workspace/README.md"
GIT_NO_DEVTRACK=1 git -C "$workspace" add README.md
GIT_NO_DEVTRACK=1 git -C "$workspace" commit -q -m 'DEMO-201: verify automated end-to-end flow'
commit_hash=$(GIT_NO_DEVTRACK=1 git -C "$workspace" rev-parse --short=12 HEAD)

while ! grep -F "$commit_hash" "$state_root/logs/daemon.log" >/dev/null 2>&1; do
    if [ "$(date +%s)" -ge "$deadline" ]; then
        printf 'Daemon did not observe commit %s before the timeout.\n' "$commit_hash" >&2
        exit 1
    fi
    sleep 1
done

mcp_output=$(DEVTRACK_ENV_FILE="$env_file" XDG_DATA_HOME="$state_root/xdg" "$binary" mcp test 2>&1)
printf '%s\n' "$mcp_output"
printf '%s\n' "$mcp_output" | grep -F '=== PASS ===' >/dev/null
printf '%s\n' "$mcp_output" | grep -F 'DEMO-201' >/dev/null
printf '%s\n' "$mcp_output" | grep -E 'today_commits[^0-9]*[1-9]' >/dev/null

DEVTRACK_ENV_FILE="$env_file" XDG_DATA_HOME="$state_root/xdg" "$binary" queue list
printf 'PASS: Linux no-send E2E observed %s and exposed it through MCP.\n' "$commit_hash"
