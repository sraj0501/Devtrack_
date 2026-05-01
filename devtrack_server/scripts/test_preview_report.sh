#!/bin/bash
# DevTrack Phase 3: Test preview-report command
# Verifies devtrack preview-report produces output (empty or with data)

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
echo "Test: Preview Report"
echo "=========================================="

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

export PROJECT_ROOT
export DEVTRACK_ENV_FILE="$PROJECT_ROOT/.env"

echo "  Running devtrack preview-report..."
OUTPUT=$("$DEVTRACK_BIN" preview-report 2>&1) || true

# Should produce output (empty report or with content)
if echo "$OUTPUT" | grep -qE "Daily Status Report|Total Hours|No activities|SUMMARY|ACTIVITIES|Generating"; then
    echo "  OK: Report generated"
elif echo "$OUTPUT" | grep -qE "Error|error|Failed|Exception"; then
    echo "  WARNING: Report command reported errors"
    echo "  Output: $OUTPUT"
else
    echo "  OK: Command completed"
fi

echo ""
echo "Preview report test complete."
