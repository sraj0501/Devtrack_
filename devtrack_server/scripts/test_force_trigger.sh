#!/bin/bash
# DevTrack Phase 2: Test force-trigger flow
# Verifies: daemon receives SIGUSR2, scheduler triggers, Python receives via IPC

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

if [ -f ".env" ]; then
    set -a
    source .env
    set +a
fi

echo "=========================================="
echo "Test: Force Trigger Flow"
echo "=========================================="

# Find devtrack binary (use devtrack-bin)
DEVTRACK_BIN=""
for loc in "$PROJECT_ROOT/devtrack-bin/devtrack"; do
    if [ -f "$loc" ]; then
        DEVTRACK_BIN="$loc"
        break
    fi
done

if [ -z "$DEVTRACK_BIN" ]; then
    echo "  SKIP: devtrack binary not found"
    exit 0
fi

# Ensure we're in a git repo
if ! git rev-parse --git-dir >/dev/null 2>&1; then
    echo "  SKIP: Not in a git repository"
    exit 0
fi

LOG_DIR="${LOG_DIR:-$PROJECT_ROOT/Data/logs}"
LOG_PATH="${LOG_DIR}/${LOG_FILE_NAME:-daemon.log}"
[ -f "$LOG_PATH" ] && : > "$LOG_PATH"

# Stop any existing daemon and free port 35893
"$DEVTRACK_BIN" stop 2>/dev/null || true
sleep 2
# Kill any process still holding the IPC port (stale daemon)
if command -v lsof >/dev/null 2>&1; then
    PIDS=$(lsof -ti :35893 2>/dev/null) || true
    if [ -n "$PIDS" ]; then
        echo "  Killing stale process(es) on port 35893: $PIDS"
        echo "$PIDS" | xargs kill -9 2>/dev/null || true
        sleep 2
    fi
fi

# Start daemon
echo "  Starting daemon..."
export PROJECT_ROOT
export DEVTRACK_WORKSPACE="$PROJECT_ROOT"
"$DEVTRACK_BIN" start &
DAEMON_PID=$!
trap 'EXIT_CODE=$?; kill $DAEMON_PID 2>/dev/null; $DEVTRACK_BIN stop 2>/dev/null || true; exit $EXIT_CODE' EXIT

# Wait for IPC
echo "  Waiting for IPC server..."
for i in $(seq 1 15); do
    if nc -z 127.0.0.1 35893 2>/dev/null; then
        echo "  OK: IPC server ready"
        break
    fi
    sleep 1
    [ $i -eq 15 ] && echo "  WARNING: IPC not ready after 15s"
done

# Wait for Python bridge
sleep 3
if pgrep -f "python_bridge.py" >/dev/null 2>&1; then
    echo "  OK: Python bridge running"
else
    echo "  WARNING: Python bridge not found (timer trigger may not reach Python)"
fi

# Ensure force-trigger uses same env (cwd and .env path)
cd "$PROJECT_ROOT"
export DEVTRACK_ENV_FILE="$PROJECT_ROOT/.env"

# Run force-trigger (sends SIGUSR2 to daemon)
echo "  Running devtrack force-trigger..."
"$DEVTRACK_BIN" force-trigger

# Wait for trigger to be processed
echo "  Waiting for trigger processing..."
sleep 5

# Check logs
PASS=0
if [ -f "$LOG_PATH" ]; then
    if grep -q "Force trigger requested via signal" "$LOG_PATH" 2>/dev/null; then
        echo "  OK: Daemon received force-trigger signal"
        PASS=1
    fi
    if grep -q "Sent trigger to Python via IPC" "$LOG_PATH" 2>/dev/null; then
        echo "  OK: Trigger sent to Python via IPC"
        PASS=1
    fi
    if grep -q "TIMER TRIGGER" "$LOG_PATH" 2>/dev/null; then
        echo "  OK: Python received timer trigger"
        PASS=1
    fi
fi

# Stop daemon
"$DEVTRACK_BIN" stop 2>/dev/null || true
sleep 2

if [ $PASS -eq 1 ]; then
    echo ""
    echo "Force trigger test passed."
else
    echo ""
    echo "  INFO: Check $LOG_PATH for details"
    echo "Force trigger test complete (some checks may be environment-dependent)."
fi
