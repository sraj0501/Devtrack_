#!/usr/bin/env bash
# =============================================================================
# DevTrack — Uninstaller (macOS & Linux)
# =============================================================================
# Removes the devtrack binary, stops the daemon, and optionally wipes data.
#
# Usage:
#   chmod +x uninstall.sh
#   ./uninstall.sh
# =============================================================================

set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

ok()   { echo -e "  ${GREEN}✓${NC}  $*"; }
err()  { echo -e "  ${RED}✗${NC}  $*"; }
warn() { echo -e "  ${YELLOW}!${NC}  $*"; }
info() { echo -e "  ${BLUE}→${NC}  $*"; }
hdr()  { echo -e "\n${CYAN}${BOLD}── $* ──${NC}"; }

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )}" && pwd )"
INSTALL_BIN="$HOME/.local/bin"
OS="$(uname -s)"

# ── Header ────────────────────────────────────────────────────────────────────
clear
echo -e "${RED}${BOLD}"
echo "  DevTrack — Uninstaller"
echo -e "${NC}"
echo -e "  ${YELLOW}WARNING: This will remove DevTrack from your machine.${NC}"
echo ""
echo "  The following will be removed:"
echo "    • Daemon process (if running)"
echo "    • devtrack binary from $INSTALL_BIN"
echo ""
echo "  You will be asked separately about:"
echo "    • Data directory (logs, database, reports)"
echo "    • Project source directory"
echo ""
echo -ne "  ${CYAN}Continue? [y/N] ${NC}"
read -r confirm
if [[ ! "${confirm:-N}" =~ ^[Yy]$ ]]; then
    echo ""
    echo "  Cancelled."
    exit 0
fi

# ── 1. Stop daemon ─────────────────────────────────────────────────────────────
hdr "Step 1: Stopping daemon"

if command -v devtrack &>/dev/null; then
    if devtrack status &>/dev/null 2>&1; then
        info "Stopping devtrack daemon..."
        devtrack stop || true
        ok "Daemon stopped"
    else
        info "Daemon not running — skipping"
    fi
else
    # Binary not in PATH — try stopping via PID file if project dir is here
    PID_FILE="$SCRIPT_DIR/Data/devtrack.pid"
    if [[ -f "$PID_FILE" ]]; then
        PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            info "Stopping daemon (PID $PID)..."
            kill "$PID" || true
            ok "Daemon stopped"
        fi
    else
        info "No running daemon found"
    fi
fi

# ── 2. Remove binary ──────────────────────────────────────────────────────────
hdr "Step 2: Removing binary"

BIN_PATH="$INSTALL_BIN/devtrack"
if [[ -f "$BIN_PATH" ]]; then
    rm -f "$BIN_PATH"
    ok "Removed $BIN_PATH"
else
    info "Binary not found at $BIN_PATH — already removed?"
fi

# Also remove from project root if present
if [[ -f "$SCRIPT_DIR/devtrack" ]]; then
    rm -f "$SCRIPT_DIR/devtrack"
    ok "Removed $SCRIPT_DIR/devtrack"
fi

# ── 3. Data directory ─────────────────────────────────────────────────────────
hdr "Step 3: Data directory"

# Try to find DATA_DIR from .env
DATA_DIR=""
if [[ -f "$SCRIPT_DIR/.env" ]]; then
    DATA_DIR=$(grep -E '^DATA_DIR=' "$SCRIPT_DIR/.env" | cut -d= -f2- | tr -d '"' | tr -d "'" || true)
fi
DATA_DIR="${DATA_DIR:-$SCRIPT_DIR/Data}"

if [[ -d "$DATA_DIR" ]]; then
    echo ""
    warn "Data directory: $DATA_DIR"
    warn "Contains: logs, database, reports, PID files, learned profile"
    echo ""
    echo -ne "  ${YELLOW}Delete all DevTrack data? [y/N] ${NC}"
    read -r del_data
    if [[ "${del_data:-N}" =~ ^[Yy]$ ]]; then
        rm -rf "$DATA_DIR"
        ok "Data directory removed"
    else
        info "Data preserved at $DATA_DIR"
    fi
else
    info "No data directory found at $DATA_DIR"
fi

# ── 4. Project source directory ───────────────────────────────────────────────
hdr "Step 4: Project source"

echo ""
warn "Project directory: $SCRIPT_DIR"
warn "Removing this will delete all source code and configuration."
echo ""
echo -ne "  ${YELLOW}Delete the project directory? [y/N] ${NC}"
read -r del_project
if [[ "${del_project:-N}" =~ ^[Yy]$ ]]; then
    # Can't delete ourselves — print the command instead
    echo ""
    warn "Run the following to finish removing the project:"
    echo ""
    echo -e "    ${CYAN}rm -rf \"$SCRIPT_DIR\"${NC}"
    echo ""
else
    info "Project directory preserved at $SCRIPT_DIR"
fi

# ── 5. PATH cleanup ───────────────────────────────────────────────────────────
hdr "Step 5: PATH entry"

[[ "$OS" == "Darwin" ]] && SHELL_RC="$HOME/.zshrc" || SHELL_RC="$HOME/.bashrc"

if grep -q 'local/bin.*PATH\|PATH.*local/bin' "$SHELL_RC" 2>/dev/null; then
    echo ""
    echo -ne "  Remove \$HOME/.local/bin from PATH in $SHELL_RC? [y/N] ${NC}"
    read -r del_path
    if [[ "${del_path:-N}" =~ ^[Yy]$ ]]; then
        # Remove the line — compatible with both macOS sed and GNU sed
        if [[ "$OS" == "Darwin" ]]; then
            sed -i '' '/\.local\/bin.*PATH\|PATH.*\.local\/bin/d' "$SHELL_RC"
        else
            sed -i '/\.local\/bin.*PATH\|PATH.*\.local\/bin/d' "$SHELL_RC"
        fi
        ok "PATH entry removed from $SHELL_RC"
        warn "Run: source $SHELL_RC"
    else
        info "PATH entry left in $SHELL_RC (other tools may use ~/.local/bin)"
    fi
else
    info "No ~/.local/bin PATH entry found in $SHELL_RC"
fi

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════${NC}"
echo -e "${GREEN}${BOLD}  DevTrack uninstalled.${NC}"
echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════${NC}"
echo ""
echo "  To reinstall:"
echo "    1. Download the binary:  ${CYAN}https://github.com/sraj0501/Devtrack_/releases/latest${NC}"
echo "    2. Install the server:   ${CYAN}devtrack-server install${NC}"
echo ""
