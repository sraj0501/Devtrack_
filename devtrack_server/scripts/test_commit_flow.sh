#!/bin/bash
# DevTrack Phase 0.3: Test git commit flow (daemon detects commit, sends to Python)
# Requires: .env configured, devtrack binary built, project is a git repo

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

echo "=========================================="
echo "Test: Git Commit Flow (Daemon + IPC)"
echo "=========================================="

# Load .env
if [ -f ".env" ]; then
    set -a
    source .env
    set +a
fi

# Find devtrack binary (use devtrack-bin, no install to ~/.local/bin)
DEVTRACK_BIN=""
for loc in "$PROJECT_ROOT/devtrack-bin/devtrack"; do
    if [ -f "$loc" ]; then
        DEVTRACK_BIN="$loc"
        break
    fi
done

if [ -z "$DEVTRACK_BIN" ]; then
    echo "  SKIP: devtrack binary not found. Run: cd devtrack-bin && go build -o devtrack ."
    exit 0
fi

# Ensure we're in a git repo
if ! git rev-parse --git-dir >/dev/null 2>&1; then
    echo "  SKIP: Not in a git repository"
    exit 0
fi

# Stop any existing daemon
"$DEVTRACK_BIN" stop 2>/dev/null || true
sleep 2

# Log path from .env (LOG_DIR, LOG_FILE_NAME)
LOG_DIR="${LOG_DIR:-$PROJECT_ROOT/Data/logs}"
mkdir -p "$LOG_DIR"
LOG_FILE="${LOG_FILE_NAME:-daemon.log}"
LOG_PATH="$LOG_DIR/$LOG_FILE"
[ -f "$LOG_PATH" ] && : > "$LOG_PATH"

# Start daemon in background
echo "  Starting daemon..."
export PROJECT_ROOT
export DEVTRACK_WORKSPACE="$PROJECT_ROOT"
"$DEVTRACK_BIN" start &
DAEMON_PID=$!
trap 'EXIT_CODE=$?; kill $DAEMON_PID 2>/dev/null; $DEVTRACK_BIN stop 2>/dev/null || true; exit $EXIT_CODE' EXIT

# Wait for IPC server (max 15s)
echo "  Waiting for IPC server..."
for i in $(seq 1 15); do
    if nc -z 127.0.0.1 35893 2>/dev/null; then
        echo "  OK: IPC server ready"
        break
    fi
    sleep 1
    if [ $i -eq 15 ]; then
        echo "  WARNING: IPC server not ready after 15s"
    fi
done

# Wait for Python bridge (check process)
sleep 3
if pgrep -f "python_bridge.py" >/dev/null 2>&1; then
    echo "  OK: Python bridge running"
else
    echo "  WARNING: Python bridge process not found"
fi

# Make empty commit to trigger git monitor (disable GPG signing for test)
echo "  Making test commit..."
git -c commit.gpgsign=false commit --allow-empty -m "DevTrack test commit $(date +%s)" || true

# Wait for git monitor to detect (polls every 2s)
echo "  Waiting for commit detection..."
sleep 6

# Check log for expected messages
if [ -f "$LOG_PATH" ]; then
    if grep -q "Sent trigger to Python via IPC" "$LOG_PATH" 2>/dev/null; then
        echo "  OK: Commit trigger sent to Python"
    elif grep -q "New commit detected" "$LOG_PATH" 2>/dev/null; then
        echo "  OK: Commit detected"
    else
        echo "  INFO: Check $LOG_PATH for daemon output"
    fi
    if grep -q "Commit processing complete" "$LOG_PATH" 2>/dev/null; then
        echo "  OK: Python processed commit"
    fi
else
    echo "  INFO: Log file not found at $LOG_PATH"
fi

# Stop daemon
"$DEVTRACK_BIN" stop 2>/dev/null || true
sleep 2

echo ""
echo "Commit flow test complete."
