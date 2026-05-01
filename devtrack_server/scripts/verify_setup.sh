#!/bin/bash
# DevTrack Phase 0.1: Environment and Setup Verification
# Verifies that the development environment is correctly configured.
# Exit 0 when all checks pass.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

echo "=========================================="
echo "DevTrack Setup Verification"
echo "=========================================="
echo "Project root: $PROJECT_ROOT"
echo ""

FAILED=0

# Check 1: .env or .env_sample exists
echo "[1/4] Checking .env file..."
if [ -f "$PROJECT_ROOT/.env" ]; then
    echo "  OK: .env found"
elif [ -f "$PROJECT_ROOT/.env_sample" ]; then
    echo "  WARNING: .env not found. Copy from .env_sample: cp .env_sample .env"
    echo "  Proceeding with .env_sample for other checks..."
else
    echo "  FAIL: Neither .env nor .env_sample found"
    FAILED=1
fi
echo ""

# Check 2: uv sync
echo "[2/4] Running uv sync..."
if ! uv sync 2>/dev/null; then
    echo "  FAIL: uv sync failed"
    FAILED=1
else
    echo "  OK"
fi
echo ""

# Check 3: spaCy model
echo "[3/4] Verifying spaCy model..."
if ! uv run python -c "
import spacy
nlp = spacy.load('en_core_web_sm')
print('  OK')
" 2>/dev/null; then
    echo "  FAIL: spaCy or en_core_web_sm not available"
    echo "  Run: uv run python -m spacy download en_core_web_sm"
    FAILED=1
fi
echo ""

# Check 4: Go build
echo "[4/4] Building Go binary..."
cd "$PROJECT_ROOT/devtrack-bin"
BUILD_OUTPUT=$(go build -o devtrack . 2>&1)
BUILD_EXIT=$?
if [ $BUILD_EXIT -ne 0 ]; then
    echo "  FAIL: go build failed"
    echo "$BUILD_OUTPUT" | head -20
    FAILED=1
else
    echo "  OK"
fi
cd "$PROJECT_ROOT"
echo ""

# Summary
echo "=========================================="
if [ $FAILED -eq 0 ]; then
    echo "All checks passed."
    exit 0
else
    echo "Some checks failed."
    exit 1
fi
