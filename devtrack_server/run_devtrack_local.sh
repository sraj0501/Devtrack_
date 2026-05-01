#!/bin/bash
# Run DevTrack locally (outside Docker)

set -e  # Exit on error

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🔍 DevTrack Local Setup Verification"
echo "======================================"

# Check 1: Verify required commands are installed
echo "Checking prerequisites..."
MISSING_COMMANDS=()

for CMD in go python3 uv git; do
    if ! command -v $CMD &> /dev/null; then
        MISSING_COMMANDS+=("$CMD")
    fi
done

if [ ${#MISSING_COMMANDS[@]} -gt 0 ]; then
    echo "❌ Error: Missing required commands:"
    printf '  - %s\n' "${MISSING_COMMANDS[@]}"
    echo ""
    echo "Please install missing prerequisites. See LOCAL_SETUP.md for details."
    exit 1
fi

echo "✓ Prerequisites installed (go, python3, uv, git)"

# Check 2: Verify Python version (3.12 or 3.13)
PYTHON_VERSION=$(python3 -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')" 2>/dev/null || echo "0.0")
PYTHON_MAJOR=$(echo $PYTHON_VERSION | cut -d. -f1)
PYTHON_MINOR=$(echo $PYTHON_VERSION | cut -d. -f2)

if [ "$PYTHON_MAJOR" -eq 3 ] && [ "$PYTHON_MINOR" -ge 12 ] && [ "$PYTHON_MINOR" -le 13 ]; then
    echo "✓ Python version $PYTHON_VERSION (compatible)"
else
    echo "❌ Error: Python $PYTHON_VERSION incompatible. Requires Python 3.12 or 3.13"
    echo "   Note: spaCy and other dependencies currently require Python 3.12 or 3.13"
    exit 1
fi

# Check 3: Load .env file - REQUIRED
if [ -f "$SCRIPT_DIR/.env" ]; then
    echo "✓ Loading configuration from .env..."
    set -a  # Automatically export all variables
    source "$SCRIPT_DIR/.env"
    set +a
else
    echo "❌ Error: .env file not found at $SCRIPT_DIR/.env"
    echo ""
    echo "Setup steps:"
    echo "  1. cp .env_sample .env"
    echo "  2. Edit .env with your paths"
    echo "  3. Run this script again"
    exit 1
fi

# Check 4: Verify required environment variables
# NOTE: These are the MINIMUM required variables. Additional variables are needed:
# - IPC_CONNECT_TIMEOUT_SECS, HTTP_TIMEOUT_SHORT, HTTP_TIMEOUT, HTTP_TIMEOUT_LONG
# - IPC_RETRY_DELAY_MS, OLLAMA_HOST, LMSTUDIO_HOST, GIT_SAGE_DEFAULT_MODEL
# - PROMPT_TIMEOUT_SIMPLE_SECS, PROMPT_TIMEOUT_WORK_SECS, PROMPT_TIMEOUT_TASK_SECS
# - LLM_REQUEST_TIMEOUT_SECS, SENTIMENT_ANALYSIS_WINDOW_MINUTES
# See docs/CONFIGURATION.md for complete list
REQUIRED_VARS=(
    "DEVTRACK_WORKSPACE"
    "DEVTRACK_HOME"
    "PROJECT_ROOT"
    "CLI_BINARY_NAME"
    "LOG_FILE_NAME"
    "PID_FILE_NAME"
    "IPC_CONNECT_TIMEOUT_SECS"
    "HTTP_TIMEOUT_SHORT"
    "HTTP_TIMEOUT"
    "HTTP_TIMEOUT_LONG"
    "IPC_RETRY_DELAY_MS"
    "OLLAMA_HOST"
    "LMSTUDIO_HOST"
    "GIT_SAGE_DEFAULT_MODEL"
)

MISSING_VARS=()
for VAR in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!VAR}" ]; then
        MISSING_VARS+=("$VAR")
    fi
done

if [ ${#MISSING_VARS[@]} -gt 0 ]; then
    echo "❌ Error: Missing required environment variables in .env:"
    printf '  - %s\n' "${MISSING_VARS[@]}"
    echo ""
    echo "Please update your .env file. See .env_sample for reference."
    echo "For complete configuration guide, see: docs/CONFIGURATION.md"
    exit 1
fi

echo "✓ Environment variables validated"

# Use .env variables (no fallbacks)
REPO_PATH="$DEVTRACK_WORKSPACE"
DEVTRACK_DIR="$DEVTRACK_HOME"
WORK_DIR="$PROJECT_ROOT"
DEVTRACK_CLI="$CLI_BINARY_NAME"
LOG_FILE="$LOG_FILE_NAME"
PID_FILE="$PID_FILE_NAME"

# Check 5: Verify uv.lock and Python dependencies
if [ ! -f "$WORK_DIR/uv.lock" ]; then
    echo "⚠️  Warning: uv.lock not found. Installing dependencies..."
    cd "$WORK_DIR"
    uv sync
    echo "✓ Dependencies installed"
else
    echo "✓ Python dependencies present (uv.lock found)"
fi

# Check 6: Verify spaCy model is installed
echo "Checking spaCy model..."
if uv run python -c "import spacy; spacy.load('en_core_web_sm')" 2>/dev/null; then
    echo "✓ spaCy model (en_core_web_sm) installed"
else
    echo "⚠️  Warning: spaCy model not found. Downloading..."
    uv run python -m spacy download en_core_web_sm
    echo "✓ spaCy model installed"
fi

# Check 7: Build Go binary if needed, verify it exists
if [ ! -f "$WORK_DIR/devtrack-bin/$DEVTRACK_CLI" ]; then
    echo "Building DevTrack binary..."
    cd "$WORK_DIR/devtrack-bin"
    if ! go build -o "$DEVTRACK_CLI" .; then
        echo "❌ Error: go build failed"
        exit 1
    fi
    cd "$WORK_DIR"
    echo "✓ Binary built"
fi

# Use devtrack from devtrack-bin (no install to ~/.local/bin)
BINARY_FOUND="$WORK_DIR/devtrack-bin/$DEVTRACK_CLI"
echo "✓ Binary at: $BINARY_FOUND"

# Check 8: Create .devtrack directory if it doesn't exist
if [ ! -d "$DEVTRACK_DIR" ]; then
    echo "Creating $DEVTRACK_DIR..."
    mkdir -p "$DEVTRACK_DIR"
fi
echo "✓ DevTrack home directory exists"

# Check 9: Verify repository path exists
if [ ! -d "$REPO_PATH" ]; then
    echo "⚠️  Warning: Repository path does not exist: $REPO_PATH"
    echo "   The daemon will monitor this path when it's created."
fi

echo ""
echo "🚀 Starting DevTrack Daemon"
echo "======================================"
echo "Repository: $REPO_PATH"
echo "Config Dir: $DEVTRACK_DIR"
echo "Project Root: $WORK_DIR"
echo "Binary: $DEVTRACK_CLI"
echo "======================================"

# Kill any existing processes
echo ""
echo "Stopping any existing DevTrack instances..."
pkill -f "$DEVTRACK_CLI" 2>/dev/null || true
pkill -f "python_bridge.py" 2>/dev/null || true
sleep 1

# Log path: daemon writes to LOG_DIR from .env (Data/logs), not DEVTRACK_DIR
LOG_PATH="${LOG_DIR:-$DEVTRACK_DIR}/$LOG_FILE"
if [ -f "$LOG_PATH" ]; then
    > "$LOG_PATH"
    echo "✓ Cleared old logs"
fi

# Start the Go daemon in the background
echo ""
echo "Starting daemon..."
cd "$REPO_PATH"  # Start in the repository to monitor

# Daemon must find .env - set path since cwd is the repo, not project root
export DEVTRACK_ENV_FILE="$WORK_DIR/.env"

# Use the found binary location
"$BINARY_FOUND" start >> "$LOG_PATH" 2>&1 &
DAEMON_PID=$!
echo "Daemon started (PID: $DAEMON_PID)"
echo "Monitoring repository: $REPO_PATH"

# Wait for daemon to initialize
echo "Waiting for daemon to initialize..."
sleep 5

# Check if daemon is running
if ps -p $DAEMON_PID > /dev/null; then
    echo "✅ Daemon is running"
else
    echo "❌ Daemon failed to start"
    echo ""
    echo "Last 30 lines of log:"
    tail -30 "$LOG_PATH"
    exit 1
fi

# Verify components are running
echo ""
echo "Verifying components..."
sleep 2

# Check for key log messages
if grep -q "NLP parser initialized" "$LOG_PATH" 2>/dev/null; then
    echo "✓ NLP parser loaded"
else
    echo "⚠️  NLP parser log not found (may still be initializing)"
fi

if grep -q "IPC server" "$LOG_PATH" 2>/dev/null; then
    echo "✓ IPC server started"
fi

if grep -q "Git monitor" "$LOG_PATH" 2>/dev/null; then
    echo "✓ Git monitor active"
fi

# Show recent logs
echo ""
echo "📋 Recent daemon logs:"
echo "--------------------------------------"
tail -20 "$LOG_PATH" | grep -E "✓|Started|Loaded|Connected" || tail -20 "$LOG_PATH"

echo ""
echo "======================================"
echo "✅ DevTrack running locally!"
echo "======================================"
echo ""
echo "Quick Commands (run from project root or use full path):"
echo "  $BINARY_FOUND status    - Check status"
echo "  $BINARY_FOUND stop      - Stop daemon"
echo "  $BINARY_FOUND git commit -m 'msg'  - AI-enhanced commit"
echo "  tail -f $LOG_PATH       - Watch logs"
echo ""
echo "Test commit detection:"
echo "  cd $REPO_PATH"
echo "  git commit -m 'Working on #PROJ-123 - Feature (2h)'"
echo "  tail -30 $LOG_PATH  # Check NLP parsing"
echo ""
echo "Configuration (from .env):"
echo "  IPC: $IPC_HOST:$IPC_PORT"
echo "  Log: $LOG_PATH"
echo "  PID: ${PID_DIR:-$DEVTRACK_DIR}/$PID_FILE"
echo "  DB:  ${DATABASE_DIR:-$DEVTRACK_DIR}/${DATABASE_FILE_NAME:-devtrack.db}"
echo ""