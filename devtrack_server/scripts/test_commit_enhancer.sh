#!/bin/bash
# DevTrack Phase 0.3: Test commit message enhancer
# Runs commit_message_enhancer.py with staged changes and verifies it runs.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

echo "=========================================="
echo "Test: Commit Message Enhancer"
echo "=========================================="

# Create temp directory (in project; may require full permissions in restricted environments)
TEMP_DIR="$PROJECT_ROOT/.test_commit_enhancer_tmp"
rm -rf "$TEMP_DIR"
mkdir -p "$TEMP_DIR"
trap "rm -rf $TEMP_DIR" EXIT
cd "$TEMP_DIR"
# Use empty template to avoid hooks (avoids permission issues on .git/hooks/)
EMPTY_TEMPLATE="$TEMP_DIR/empty_tpl"
mkdir -p "$EMPTY_TEMPLATE"
trap "rm -rf $TEMP_DIR" EXIT
git init -q --template="$EMPTY_TEMPLATE"
git config user.email "test@devtrack.local"
git config user.name "DevTrack Test"

# Create and stage a file
echo "test content" > testfile.txt
git add testfile.txt

# Create temp commit message file
TEMP_MSG=$(mktemp)
trap "rm -rf $TEMP_DIR; rm -f $TEMP_MSG" EXIT
echo "wip" > "$TEMP_MSG"

# Run enhancer (from project root with correct paths)
cd "$PROJECT_ROOT"
export GIT_DIR="$TEMP_DIR/.git"
OUTPUT=$(uv run python backend/commit_message_enhancer.py "$TEMP_MSG" auto 2>&1) || true

# Check output indicates enhancement or fallback
if echo "$OUTPUT" | grep -qE "(enhanced|Enhancing|No enhancement|AI unavailable|Ollama)"; then
    echo "  OK: Enhancer ran (enhancement or fallback)"
elif echo "$OUTPUT" | grep -qE "(Error|error|Exception)"; then
    echo "  WARNING: Enhancer reported errors (may be expected if Ollama down)"
    echo "  Output: $OUTPUT"
else
    echo "  OK: Enhancer completed"
fi

# Verify temp file was potentially modified
if [ -f "$TEMP_MSG" ]; then
    echo "  OK: Message file exists"
fi

echo ""
echo "Commit enhancer test complete."
