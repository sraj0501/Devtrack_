#!/usr/bin/env bash
# devtrack-server — management CLI for the DevTrack Python backend
#
# Usage: devtrack-server <command>
#
# Commands:
#   install    Copy server files to ~/.local/share/devtrack-server, install deps
#   setup      Interactive .env configuration wizard
#   start      Start the webhook server in the background
#   stop       Stop the running webhook server
#   restart    Stop then start
#   status     Show running state and health
#   logs       Tail the server log
#   upgrade    Download and install the latest server release
#   uninstall  Stop the server and remove all installed files
#   version    Print installed version

set -euo pipefail

# ── Colours ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

ok()   { echo -e "  ${GREEN}✓${NC}  $*"; }
err()  { echo -e "  ${RED}✗${NC}  $*" >&2; }
warn() { echo -e "  ${YELLOW}!${NC}  $*"; }
info() { echo -e "  ${BLUE}→${NC}  $*"; }
hdr()  { echo -e "\n${CYAN}${BOLD}── $* ──${NC}"; }
die()  { err "$*"; exit 1; }

# ── Paths ──────────────────────────────────────────────────────────────────────
SERVER_HOME="${DEVTRACK_SERVER_HOME:-$HOME/.local/share/devtrack-server}"
SERVER_BIN="$HOME/.local/bin/devtrack-server"
GITHUB_REPO="sraj0501/automation_tools"

# Re-derive PID/log paths from SERVER_HOME (call whenever SERVER_HOME changes)
_resolve_home() {
    SERVER_PID="$SERVER_HOME/devtrack-server.pid"
    ADMIN_PID="$SERVER_HOME/devtrack-admin.pid"
    SERVER_LOG="$SERVER_HOME/logs/server.log"
    ADMIN_LOG="$SERVER_HOME/logs/admin.log"
}
_resolve_home

# Scan well-known install locations; updates SERVER_HOME if a valid install is
# found somewhere other than the current value.  Skips if already pointing at
# a valid install, and skips during `install` (no backend/ yet).
_detect_home() {
    [[ -d "$SERVER_HOME/backend" ]] && return 0   # already correct

    local candidate
    for candidate in \
        "${DEVTRACK_SERVER_HOME:-}" \
        "$HOME/.local/share/devtrack-server" \
        "/opt/devtrack-server" \
        "/usr/local/share/devtrack-server"
    do
        [[ -n "$candidate" && -d "$candidate/backend" ]] || continue
        SERVER_HOME="$candidate"
        _resolve_home
        return 0
    done
    return 1   # no install found — caller decides whether to die
}

# Source .env if present so PORT etc. are available
_load_env() {
    _detect_home || true   # best-effort; commands that need an install will die later
    local env_file="$SERVER_HOME/.env"
    if [[ -f "$env_file" ]]; then
        set -o allexport
        # shellcheck disable=SC1090
        source "$env_file"
        set +o allexport
    fi
}

_server_version() {
    local ver_file="$SERVER_HOME/VERSION"
    [[ -f "$ver_file" ]] && cat "$ver_file" || echo "unknown"
}

_server_port() {
    echo "${WEBHOOK_PORT:-8089}"
}

# ── Process helpers ────────────────────────────────────────────────────────────
_is_running() {
    [[ -f "$SERVER_PID" ]] || return 1
    local pid
    pid=$(cat "$SERVER_PID")
    kill -0 "$pid" 2>/dev/null
}

_admin_is_running() {
    [[ -f "$ADMIN_PID" ]] || return 1
    local pid
    pid=$(cat "$ADMIN_PID")
    kill -0 "$pid" 2>/dev/null
}

_health_check() {
    local port; port=$(_server_port)
    curl -sf "http://127.0.0.1:${port}/health" &>/dev/null
}

# ── install ────────────────────────────────────────────────────────────────────
cmd_install() {
    hdr "Installing DevTrack Server"

    local src_dir
    src_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

    # Ensure uv is available
    export PATH="$HOME/.local/bin:$PATH"
    if ! command -v uv &>/dev/null; then
        info "uv not found — installing..."
        curl -LsSf https://astral.sh/uv/install.sh | sh
        ok "uv installed"
    fi

    info "Creating server home: $SERVER_HOME"
    mkdir -p "$SERVER_HOME/logs"

    info "Copying server files..."
    cp -r "$src_dir/backend"               "$SERVER_HOME/"
    cp    "$src_dir/pyproject.toml"        "$SERVER_HOME/"
    [[ -f "$src_dir/uv.lock" ]]      && cp "$src_dir/uv.lock"      "$SERVER_HOME/"
    [[ -f "$src_dir/.env_sample" ]]  && cp "$src_dir/.env_sample"  "$SERVER_HOME/"
    [[ -f "$src_dir/VERSION" ]]      && cp "$src_dir/VERSION"       "$SERVER_HOME/"


    # Workspace templates only — never copy a live workspaces.yaml
    [[ -f "$src_dir/workspaces.yaml.example" ]] && cp "$src_dir/workspaces.yaml.example" "$SERVER_HOME/"
    [[ -f "$src_dir/workspaces.yaml.sample" ]]  && cp "$src_dir/workspaces.yaml.sample"  "$SERVER_HOME/"

    # Remove bytecode and test dirs from the install
    find "$SERVER_HOME" -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
    find "$SERVER_HOME" -name "*.pyc" -o -name "*.pyo" -delete 2>/dev/null || true
    find "$SERVER_HOME" -type d -name "tests" -exec rm -rf {} + 2>/dev/null || true

    info "Installing Python dependencies..."
    uv sync --directory "$SERVER_HOME" --quiet
    ok "Python dependencies ready (core)"

    info "Installing AI extras (spaCy + NLP model)..."
    if ( cd "$SERVER_HOME" && uv sync --extra ai --quiet 2>&1 ); then
        ok "AI extras installed (spaCy, en_core_web_sm, ChromaDB)"
    else
        warn "AI extras failed — NLP features will be unavailable"
        warn "Retry manually: cd $SERVER_HOME && uv sync --extra ai"
    fi

    # Install this script to PATH
    mkdir -p "$(dirname "$SERVER_BIN")"
    cp "${BASH_SOURCE[0]}" "$SERVER_BIN"
    chmod +x "$SERVER_BIN"
    ok "devtrack-server installed to $SERVER_BIN"

    # Ensure ~/.local/bin is in PATH
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
        local rc_file
        [[ "$(uname -s)" == "Darwin" ]] && rc_file="$HOME/.zshrc" || rc_file="$HOME/.bashrc"
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$rc_file"
        export PATH="$HOME/.local/bin:$PATH"
        warn "Added ~/.local/bin to PATH in $rc_file — run: source $rc_file"
    fi

    echo ""
    ok "Installation complete  (version: $(_server_version))"
    echo ""
    echo -e "  Next step: ${CYAN}devtrack-server setup${NC}"
    echo ""
}

# ── setup ──────────────────────────────────────────────────────────────────────
cmd_setup() {
    hdr "DevTrack Server Setup"

    if _detect_home; then
        info "Using installation at $SERVER_HOME"
    else
        die "No installation found — run: devtrack-server install first"
    fi
    _load_env

    local env_src="$SERVER_HOME/.env_sample"
    local env_dst="$SERVER_HOME/.env"

    if [[ ! -f "$env_dst" ]]; then
        [[ -f "$env_src" ]] || die ".env_sample not found in $SERVER_HOME — re-run: devtrack-server install"
        cp "$env_src" "$env_dst"
    fi


    echo -e "  Configuring ${CYAN}$env_dst${NC}"
    echo ""

    _prompt_env() {
        local key="$1" prompt="$2" default="$3"
        local current
        current=$(grep -E "^${key}=" "$env_dst" 2>/dev/null | cut -d= -f2- | tr -d '"' || true)
        # Expand any shell variable references (e.g. ${PROJECT_ROOT}/Data → absolute path)
        [[ -n "$current" ]] && current=$(eval echo "$current" 2>/dev/null || echo "$current")
        # Ignore unedited sample placeholders (e.g. /path/to/devtrack_server)
        [[ "$current" == *"/path/to/"* ]] && current=""
        [[ -n "$current" ]] && default="$current"
        echo -ne "  ${prompt}"
        [[ -n "$default" ]] && echo -ne " ${YELLOW}[$default]${NC}"
        echo -ne ": "
        read -r val
        val="${val:-$default}"
        # Replace or append
        if grep -qE "^${key}=" "$env_dst" 2>/dev/null; then
            sed -i "s|^${key}=.*|${key}=${val}|" "$env_dst"
        else
            echo "${key}=${val}" >> "$env_dst"
        fi
    }

    _prompt_env "PROJECT_ROOT"        "Project root (path to devtrack-server dir)" "$SERVER_HOME"
    _prompt_env "DATA_DIR"            "Data directory"                             "$SERVER_HOME/Data"
    _prompt_env "DATABASE_DIR"        "Database directory"                         "$SERVER_HOME/Data/db"
    _prompt_env "LOG_DIR"             "Log directory"                              "$SERVER_HOME/Data/logs"
    _prompt_env "LEARNING_DIR_PATH"   "Learning/personalization directory"         "$SERVER_HOME/Data/learning"
    _prompt_env "WEBHOOK_PORT"        "Webhook server port"                        "8089"
    _prompt_env "WEBHOOK_HOST"        "Webhook server bind address"                "0.0.0.0"
    _prompt_env "LLM_PROVIDER"        "LLM provider (ollama/openai/anthropic)"     "ollama"
    _prompt_env "OLLAMA_HOST"         "Ollama host URL"                            "http://localhost:11434"
    _prompt_env "GIT_SAGE_DEFAULT_MODEL" "Default LLM model"                      "llama3"

    # Create all data directories — expand any variable references before mkdir
    local _expand; _expand() { eval echo "$1" 2>/dev/null || echo "$1"; }
    local data_dir db_dir log_dir learning_dir
    data_dir=$(_expand "$(grep -E "^DATA_DIR="         "$env_dst" | cut -d= -f2- || echo "$SERVER_HOME/Data")")
    db_dir=$(_expand    "$(grep -E "^DATABASE_DIR="    "$env_dst" | cut -d= -f2- || echo "$data_dir/db")")
    log_dir=$(_expand   "$(grep -E "^LOG_DIR="         "$env_dst" | cut -d= -f2- || echo "$data_dir/logs")")
    learning_dir=$(_expand "$(grep -E "^LEARNING_DIR_PATH=" "$env_dst" | cut -d= -f2- || echo "$data_dir/learning")")
    mkdir -p "$data_dir" "$db_dir" "$log_dir" "$learning_dir"

    echo ""
    ok "Configuration written to $env_dst"
    echo ""
    echo -e "  Start the server: ${CYAN}devtrack-server start${NC}"
    echo ""
}

# ── start ──────────────────────────────────────────────────────────────────────
cmd_start() {
    _load_env
    if _is_running; then
        ok "Webhook server already running (PID: $(cat "$SERVER_PID"))"
    else
        [[ -f "$SERVER_HOME/.env" ]] || die "Not configured — run: devtrack-server setup"

        mkdir -p "$(dirname "$SERVER_LOG")"
        info "Starting webhook server..."

        export PATH="$HOME/.local/bin:$PATH"
        (
            cd "$SERVER_HOME"
            nohup uv run python -m backend.webhook_server \
                >> "$SERVER_LOG" 2>&1 &
            echo $! > "$SERVER_PID"
        )

        # Wait up to 10s for health to pass
        local i=0
        while (( i < 20 )); do
            sleep 0.5
            if _health_check; then
                ok "Webhook server started (PID: $(cat "$SERVER_PID"), port: $(_server_port))"
                break
            fi
            (( i++ ))
        done
        (( i >= 20 )) && warn "Webhook server started but health check timed out — check: devtrack-server logs"
    fi

    # Start admin console unless embedded on the webhook server
    local admin_embed="${ADMIN_EMBED:-false}"
    if [[ "$admin_embed" == "true" ]]; then
        ok "Admin console embedded on webhook server (ADMIN_EMBED=true)"
    elif _admin_is_running; then
        ok "Admin console already running (PID: $(cat "$ADMIN_PID"))"
    else
        local admin_port="${ADMIN_PORT:-8090}"
        info "Starting admin console on port $admin_port..."
        mkdir -p "$(dirname "$ADMIN_LOG")"
        export PATH="$HOME/.local/bin:$PATH"
        (
            cd "$SERVER_HOME"
            nohup uv run python -m backend.admin \
                >> "$ADMIN_LOG" 2>&1 &
            echo $! > "$ADMIN_PID"
        )
        sleep 1
        if _admin_is_running; then
            ok "Admin console started (PID: $(cat "$ADMIN_PID"), port: $admin_port)"
        else
            warn "Admin console may not have started — check: devtrack-server logs --admin"
        fi
    fi
}

# ── stop ───────────────────────────────────────────────────────────────────────
_kill_pid_file() {
    local pid_file="$1" label="$2"
    [[ -f "$pid_file" ]] || { warn "$label is not running"; return 0; }
    local pid; pid=$(cat "$pid_file")
    kill -0 "$pid" 2>/dev/null || { rm -f "$pid_file"; warn "$label was not running"; return 0; }
    info "Stopping $label (PID: $pid)..."
    kill "$pid" 2>/dev/null || true
    local i=0
    while (( i < 20 )) && kill -0 "$pid" 2>/dev/null; do
        sleep 0.5
        (( i++ ))
    done
    kill -0 "$pid" 2>/dev/null && kill -9 "$pid" 2>/dev/null || true
    rm -f "$pid_file"
    ok "$label stopped"
}

cmd_stop() {
    _load_env
    _kill_pid_file "$SERVER_PID" "Webhook server"
    local admin_embed="${ADMIN_EMBED:-false}"
    if [[ "$admin_embed" != "true" ]]; then
        _kill_pid_file "$ADMIN_PID" "Admin console"
    fi
}

# ── status ─────────────────────────────────────────────────────────────────────
cmd_status() {
    _load_env
    echo ""
    echo -e "  ${BOLD}DevTrack Server${NC}  (version: $(_server_version))"
    echo -e "  Home: $SERVER_HOME"
    echo ""

    # Webhook server
    echo -e "  ${BOLD}Webhook server${NC}  port $(_server_port)"
    if _is_running; then
        local pid; pid=$(cat "$SERVER_PID")
        echo -e "  Process:  ${GREEN}● Running${NC}  (PID: $pid)"
        if _health_check; then
            echo -e "  Health:   ${GREEN}● OK${NC}"
        else
            echo -e "  Health:   ${YELLOW}● Unresponsive${NC}  (process up, HTTP not ready)"
        fi
    else
        echo -e "  Process:  ${RED}● Stopped${NC}"
        echo -e "  Health:   ${RED}● Offline${NC}"
    fi
    echo -e "  Log:      $SERVER_LOG"
    echo ""

    # Admin console
    local admin_embed="${ADMIN_EMBED:-false}"
    local admin_port="${ADMIN_PORT:-8090}"
    if [[ "$admin_embed" == "true" ]]; then
        echo -e "  ${BOLD}Admin console${NC}  embedded on webhook server at /admin"
    else
        echo -e "  ${BOLD}Admin console${NC}  port $admin_port"
        if _admin_is_running; then
            local apid; apid=$(cat "$ADMIN_PID")
            echo -e "  Process:  ${GREEN}● Running${NC}  (PID: $apid)"
        else
            echo -e "  Process:  ${RED}● Stopped${NC}"
        fi
        echo -e "  Log:      $ADMIN_LOG"
    fi
    echo ""
}

# ── logs ───────────────────────────────────────────────────────────────────────
cmd_logs() {
    case "${1:-}" in
        --admin) [[ -f "$ADMIN_LOG" ]] || die "No admin log at $ADMIN_LOG"; tail -f "$ADMIN_LOG" ;;
        *)       [[ -f "$SERVER_LOG" ]] || die "No log file at $SERVER_LOG"; tail -f "$SERVER_LOG" ;;
    esac
}

# ── tui ────────────────────────────────────────────────────────────────────────
cmd_tui() {
    _load_env
    [[ -d "$SERVER_HOME/.venv" ]] || die "Server not installed — run: devtrack-server install"
    export PATH="$HOME/.local/bin:$PATH"
    info "Launching server TUI  (q to quit)"
    cd "$SERVER_HOME"
    uv run python -m backend.server_tui
}

# ── upgrade ────────────────────────────────────────────────────────────────────
cmd_upgrade() {
    export PATH="$HOME/.local/bin:$PATH"
    command -v git  &>/dev/null || die "git is required for upgrade"
    command -v curl &>/dev/null || die "curl is required for upgrade"

    hdr "Checking for updates"

    local repo_url="https://github.com/${GITHUB_REPO}.git"
    local tmp_dir; tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT

    info "Fetching latest from ${GITHUB_REPO}..."
    git clone --depth 1 --quiet "$repo_url" "$tmp_dir/repo" \
        || die "Could not clone $repo_url — check network and repo access"

    local src_dir="$tmp_dir/repo/devtrack_server"
    [[ -d "$src_dir/backend" ]] \
        || die "Unexpected repo structure — devtrack_server/backend not found"

    local latest_ver current_ver
    latest_ver=$(cat "$src_dir/VERSION" 2>/dev/null || echo "unknown")
    current_ver=$(_server_version)

    echo -e "  Current: ${current_ver}"
    echo -e "  Latest:  ${latest_ver}"

    if [[ "${1:-}" == "--check" ]]; then
        echo ""
        if [[ "$current_ver" == "$latest_ver" ]]; then
            ok "Already up to date."
        else
            echo -e "  ${YELLOW}Update available: ${current_ver} → ${latest_ver}${NC}"
            echo -e "  Run ${CYAN}devtrack-server upgrade${NC} to install."
        fi
        return 0
    fi

    if [[ "$current_ver" == "$latest_ver" && "$current_ver" != "unknown" ]]; then
        ok "Already up to date."
        return 0
    fi

    echo -e "\n  ${YELLOW}Updating: ${current_ver} → ${latest_ver}${NC}"

    local was_running=false
    if _is_running; then
        was_running=true
        info "Stopping server for upgrade..."
        cmd_stop
    fi

    info "Installing new files..."
    # Preserve .env and workspaces.yaml — never overwrite user config
    cp -r "$src_dir/backend"        "$SERVER_HOME/"
    cp    "$src_dir/pyproject.toml" "$SERVER_HOME/"
    [[ -f "$src_dir/uv.lock" ]]   && cp "$src_dir/uv.lock"  "$SERVER_HOME/"
    [[ -f "$src_dir/VERSION" ]]   && cp "$src_dir/VERSION"   "$SERVER_HOME/"

    find "$SERVER_HOME" -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
    find "$SERVER_HOME" -name "*.pyc" -delete 2>/dev/null || true
    find "$SERVER_HOME" -type d -name "tests" -exec rm -rf {} + 2>/dev/null || true

    info "Syncing Python dependencies..."
    uv sync --directory "$SERVER_HOME" --quiet

    # Update the installed management script itself
    cp "$src_dir/devtrack-server.sh" "$SERVER_BIN"
    chmod +x "$SERVER_BIN"

    ok "Updated to ${latest_ver}"

    if [[ "$was_running" == true ]]; then
        info "Restarting server..."
        cmd_start
    else
        echo -e "\n  Run ${CYAN}devtrack-server start${NC} to use the new version."
    fi
}

# ── uninstall ──────────────────────────────────────────────────────────────────
cmd_uninstall() {
    hdr "Uninstall DevTrack Server"
    echo -e "  This will remove:"
    echo -e "    ${CYAN}$SERVER_HOME${NC}  (server files)"
    echo -e "    ${CYAN}$SERVER_BIN${NC}  (this script)"
    echo ""
    echo -ne "  Are you sure? [y/N] "
    read -r confirm
    [[ "${confirm,,}" == "y" ]] || { echo "  Cancelled."; return 0; }

    if _is_running; then
        info "Stopping server..."
        cmd_stop
    fi

    echo -ne "  Also remove data directory (logs, database)? [y/N] "
    read -r rm_data
    if [[ "${rm_data,,}" == "y" ]]; then
        _load_env
        local data_dir="${DATA_DIR:-$SERVER_HOME/data}"
        [[ "$data_dir" != "/" ]] && rm -rf "$data_dir" && ok "Data directory removed"
    fi

    rm -rf "$SERVER_HOME"
    rm -f  "$SERVER_BIN"

    ok "DevTrack server uninstalled."
}

# ── features ───────────────────────────────────────────────────────────────────
cmd_features() {
    _load_env
    hdr "DevTrack Server Features"
    local python_ok=false
    if [[ -d "$SERVER_HOME/.venv" ]]; then
        if ( cd "$SERVER_HOME" && uv run python -c "import spacy" 2>/dev/null ); then
            python_ok=true
        fi
    fi
    echo ""
    ok "core    web server, LLM, integrations, reporting"
    if [[ "$python_ok" == true ]]; then
        ok "ai      NLP parser, RAG personalization"
    else
        err "ai      NLP parser, RAG personalization (run: devtrack-server enable ai)"
    fi
    echo ""
}

# ── enable ─────────────────────────────────────────────────────────────────────
cmd_enable() {
    local feature="${1:-}"
    case "$feature" in
        ai)
            hdr "Enabling AI features"
            export PATH="$HOME/.local/bin:$PATH"
            info "Installing ai extra into server venv..."
            ( cd "$SERVER_HOME" && uv pip install "devtrack[ai]" ) \
                || die "Failed to install ai extra"
            ok "AI features installed"
            warn "Restart the server to apply: devtrack-server restart"
            ;;
        *)
            die "Unknown feature: ${feature}. Available: ai"
            ;;
    esac
}

# ── help ───────────────────────────────────────────────────────────────────────
cmd_help() {
    echo ""
    echo -e "  ${BOLD}devtrack-server${NC} — DevTrack Python backend management CLI"
    echo ""
    echo -e "  ${CYAN}Usage:${NC} devtrack-server <command>"
    echo ""
    echo -e "  ${BOLD}SETUP${NC}"
    echo -e "    install          Install server files, Python deps, and this CLI"
    echo -e "    setup            Interactive .env configuration wizard"
    echo ""
    echo -e "  ${BOLD}RUNTIME${NC}"
    echo -e "    start            Start webhook server + admin console in the background"
    echo -e "    stop             Stop all running server processes"
    echo -e "    restart          Stop then start"
    echo -e "    status           Show process state and HTTP health"
    echo -e "    logs             Tail the webhook server log (Ctrl+C to exit)"
    echo -e "    logs --admin     Tail the admin console log"
    echo -e "    tui              Launch the server TUI in this terminal (q to quit)"
    echo ""
    echo -e "  ${BOLD}MAINTENANCE${NC}"
    echo -e "    upgrade          Download and install the latest release"
    echo -e "    upgrade --check  Check for updates without installing"
    echo -e "    uninstall        Remove all installed server files"
    echo -e "    version          Print installed version"
    echo ""
    echo -e "  ${BOLD}FEATURES${NC}"
    echo -e "    features         Show which optional feature sets are installed"
    echo -e "    enable ai        Install the AI extra (NLP parser, RAG personalization)"
    echo ""
    echo -e "  ${BOLD}Environment:${NC}"
    echo -e "    DEVTRACK_SERVER_HOME  Override install directory (default: ~/.local/share/devtrack-server)"
    echo ""
}

# ── dispatch ───────────────────────────────────────────────────────────────────
CMD="${1:-help}"
shift 2>/dev/null || true

case "$CMD" in
    install)   cmd_install "$@" ;;
    setup)     cmd_setup   "$@" ;;
    start)     cmd_start   "$@" ;;
    stop)      cmd_stop    "$@" ;;
    restart)   cmd_stop; cmd_start ;;
    status)    cmd_status  "$@" ;;
    logs)      cmd_logs    "$@" ;;
    tui)       cmd_tui     "$@" ;;
    upgrade)   cmd_upgrade "$@" ;;
    uninstall) cmd_uninstall "$@" ;;
    version)   echo "devtrack-server $(_server_version)" ;;
    features)  cmd_features "$@" ;;
    enable)    cmd_enable   "$@" ;;
    help|--help|-h) cmd_help ;;
    *) err "Unknown command: $CMD"; cmd_help; exit 1 ;;
esac
