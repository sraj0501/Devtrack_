#!/usr/bin/env bash
# =============================================================================
# DevTrack — Local Setup Script (macOS & Linux)
# =============================================================================
# Installs all dependencies, builds the Go binary, configures the Python
# environment, and sets up .env for a first-time local installation.
#
# Usage:
#   chmod +x setup_local.sh
#   ./setup_local.sh
#
# What this script does:
#   1. Checks / installs system deps (Go, Python 3.12+, uv, Ollama)
#   2. Installs Python packages via uv sync
#   3. Downloads the spaCy NLP model
#   4. Builds the Go binary and installs it to ~/.local/bin
#   5. Copies .env_sample → .env and prompts you to fill it in
#   6. Verifies the installation
# =============================================================================

set -euo pipefail

# ── Colours ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

ok()   { echo -e "  ${GREEN}✓${NC}  $*"; }
err()  { echo -e "  ${RED}✗${NC}  $*"; }
warn() { echo -e "  ${YELLOW}!${NC}  $*"; }
info() { echo -e "  ${BLUE}→${NC}  $*"; }
hdr()  { echo -e "\n${CYAN}${BOLD}── $* ──${NC}"; }

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
INSTALL_BIN="$HOME/.local/bin"
OS="$(uname -s)"

# Ensure ~/.local/bin is on PATH for this script's lifetime.
# uv installs there; without this it's invisible on re-runs in a fresh shell.
export PATH="$HOME/.local/bin:$PATH"

# ── Header ────────────────────────────────────────────────────────────────────
clear
echo -e "${CYAN}${BOLD}"
echo "  ██████╗ ███████╗██╗   ██╗████████╗██████╗  █████╗  ██████╗██╗  ██╗"
echo "  ██╔══██╗██╔════╝██║   ██║╚══██╔══╝██╔══██╗██╔══██╗██╔════╝██║ ██╔╝"
echo "  ██║  ██║█████╗  ██║   ██║   ██║   ██████╔╝███████║██║     █████╔╝ "
echo "  ██║  ██║██╔══╝  ╚██╗ ██╔╝   ██║   ██╔══██╗██╔══██║██║     ██╔═██╗ "
echo "  ██████╔╝███████╗ ╚████╔╝    ██║   ██║  ██║██║  ██║╚██████╗██║  ██╗"
echo "  ╚═════╝ ╚══════╝  ╚═══╝     ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝"
echo -e "${NC}"
echo -e "  ${BOLD}Developer Automation Platform — Local Setup${NC}"
echo -e "  ${YELLOW}No Docker required · Runs 100% on your machine${NC}"
echo ""
echo -e "  This script installs all dependencies and gets DevTrack running."
echo -e "  Takes about ${CYAN}5–10 minutes${NC} on a fresh machine."
echo ""
read -rp "  Press Enter to begin, or Ctrl+C to cancel..."

# ── 1. System dependencies ────────────────────────────────────────────────────
hdr "Step 1: System dependencies"

# Go 1.20+
if command -v go &>/dev/null; then
    GO_VER=$(go version | awk '{print $3}' | sed 's/go//')
    ok "Go $GO_VER"
else
    warn "Go not found — installing..."
    if [[ "$OS" == "Darwin" ]] && command -v brew &>/dev/null; then
        brew install go
        ok "Go installed via Homebrew"
    elif [[ "$OS" == "Linux" ]]; then
        GO_VERSION="1.22.3"
        ARCH=$(uname -m); [[ "$ARCH" == "x86_64" ]] && ARCH="amd64" || ARCH="arm64"
        info "Downloading Go $GO_VERSION ($ARCH)..."
        curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf /tmp/go.tar.gz
        export PATH="$PATH:/usr/local/go/bin"
        ok "Go $GO_VERSION installed to /usr/local/go"
    else
        err "Please install Go 1.20+ from https://go.dev/dl/ and re-run."
        exit 1
    fi
fi

# Python 3.12+
if command -v python3 &>/dev/null; then
    PY_VER=$(python3 --version | awk '{print $2}')
    PY_MINOR=$(echo "$PY_VER" | cut -d. -f2)
    if [[ "$PY_MINOR" -ge 12 ]]; then
        ok "Python $PY_VER"
    else
        warn "Python $PY_VER found — 3.12+ required"
        if [[ "$OS" == "Darwin" ]] && command -v brew &>/dev/null; then
            brew install python@3.12
            ok "Python 3.12 installed"
        else
            err "Please install Python 3.12+ from https://python.org and re-run."
            exit 1
        fi
    fi
else
    err "Python 3 not found. Install Python 3.12+ and re-run."
    exit 1
fi

# uv (Python package manager)
if command -v uv &>/dev/null; then
    ok "uv $(uv --version | awk '{print $2}')"
else
    warn "uv not found — installing..."
    curl -LsSf https://astral.sh/uv/install.sh | sh
    export PATH="$HOME/.local/bin:$PATH"
    ok "uv installed"
fi

# Ollama (optional — AI features, can use cloud LLMs instead)
if command -v ollama &>/dev/null; then
    ok "Ollama (local LLM)"
else
    warn "Ollama not found — skipping (optional)"
    warn "AI features will use cloud LLM if configured (OpenAI, Anthropic, Groq)"
    warn "Install later: https://ollama.com/download"
fi

# ── 2. Python environment ─────────────────────────────────────────────────────
hdr "Step 2: Python environment"

cd "$SCRIPT_DIR"
info "Installing Python packages (uv sync)..."
uv sync
ok "Python packages ready"

info "Downloading spaCy NLP model..."
# uv venvs don't include pip by default; spacy download calls pip internally.
uv pip install pip --quiet
uv run python -m spacy download en_core_web_sm
ok "spaCy en_core_web_sm ready"

# ── 3. Build the Go binary ────────────────────────────────────────────────────
hdr "Step 3: Building devtrack binary"

cd "$SCRIPT_DIR/devtrack-bin"
info "Building..."
go build -o devtrack .
ok "Binary built"

# Install to project root (used when running from project dir directly)
cp devtrack "$SCRIPT_DIR/devtrack"

# Install to ~/.local/bin so it's available system-wide
mkdir -p "$INSTALL_BIN"
cp devtrack "$INSTALL_BIN/devtrack"
chmod +x "$INSTALL_BIN/devtrack"
ok "Installed to $INSTALL_BIN/devtrack"
cd "$SCRIPT_DIR"

# PATH check
if [[ ":$PATH:" != *":$INSTALL_BIN:"* ]]; then
    warn "$INSTALL_BIN is not in your PATH"
    [[ "$OS" == "Darwin" ]] && SHELL_RC="$HOME/.zshrc" || SHELL_RC="$HOME/.bashrc"
    echo ""
    echo -ne "  Add it to ${CYAN}$SHELL_RC${NC} automatically? [Y/n] "
    read -r ADD_PATH
    if [[ "${ADD_PATH:-Y}" =~ ^[Yy]$ ]]; then
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_RC"
        export PATH="$INSTALL_BIN:$PATH"
        ok "PATH updated in $SHELL_RC"
    else
        warn "Remember to add ${CYAN}export PATH=\"\$HOME/.local/bin:\$PATH\"${NC} to your shell config"
    fi
fi

# ── 4. Configure .env ─────────────────────────────────────────────────────────
hdr "Step 4: Environment configuration"

if [[ -f "$SCRIPT_DIR/.env" ]]; then
    ok ".env already exists — leaving it untouched"
else
    cp "$SCRIPT_DIR/.env_sample" "$SCRIPT_DIR/.env"
    ok ".env created from .env_sample"
    echo ""
    echo -e "  ${YELLOW}Required variables to configure in .env:${NC}"
    echo ""
    echo -e "    ${CYAN}PROJECT_ROOT${NC}        → $(pwd)"
    echo -e "    ${CYAN}DEVTRACK_WORKSPACE${NC}  → path to the git repo DevTrack should monitor"
    echo -e "    ${CYAN}DATA_DIR${NC}            → where DevTrack stores its data  (e.g. $(pwd)/Data)"
    echo ""
    echo -e "  See ${CYAN}docs/CONFIGURATION.md${NC} for the full variable reference."
    echo ""
    echo -ne "  Open .env in your editor now? [Y/n] "
    read -r OPEN_ENV
    if [[ "${OPEN_ENV:-Y}" =~ ^[Yy]$ ]]; then
        "${EDITOR:-nano}" "$SCRIPT_DIR/.env"
    fi
fi

# ── 5. Verify ─────────────────────────────────────────────────────────────────
hdr "Step 5: Verifying"

BIN_PATH="$INSTALL_BIN/devtrack"
if [[ -x "$BIN_PATH" ]]; then
    ok "devtrack binary: $BIN_PATH"
else
    err "Binary not found at $BIN_PATH"
fi

if uv run python -c "import spacy; spacy.load('en_core_web_sm')" &>/dev/null; then
    ok "spaCy model loadable"
else
    warn "spaCy model check failed — run: uv run python -m spacy download en_core_web_sm"
fi

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════${NC}"
echo -e "${GREEN}${BOLD}  Setup complete!${NC}"
echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════${NC}"
echo ""
echo -e "  ${BOLD}Start DevTrack:${NC}"
echo ""
echo -e "    ${YELLOW}devtrack start${NC}                 start the daemon"
echo -e "    ${YELLOW}devtrack status${NC}                verify it's running"
echo -e "    ${YELLOW}devtrack git commit -m \"msg\"${NC}   AI-enhanced commit"
echo -e "    ${YELLOW}devtrack help${NC}                  all commands"
echo ""
echo -e "  ${BOLD}Docs:${NC}  ${CYAN}docs/QUICK_START.md${NC}"
echo ""
echo -e "  ${YELLOW}Note:${NC} If 'devtrack' is not found, run: ${CYAN}source ~/.zshrc${NC}"
echo ""
