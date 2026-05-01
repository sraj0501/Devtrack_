#!/bin/bash

# DevTrack Docker Installer
# Builds Docker images and deploys DevTrack as a containerized application

set -e

# Installation directory
INSTALL_DIR="$HOME/.local/bin"
WORKSPACE_PATH="${WORKSPACE_PATH:-$HOME/workspace}"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Print colored output
print_header() {
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║        DevTrack - Docker Installer                       ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_success() {
    echo -e "   ${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "   ${RED}✗${NC} $1"
}

print_warning() {
    echo -e "   ${YELLOW}⚠️${NC}  $1"
}

print_info() {
    echo -e "${BLUE}$1${NC}"
}

# Check Docker
check_docker() {
    echo -e "${BLUE}🔍 Checking Docker installation...${NC}"
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed."
        echo ""
        echo "   Please install Docker Desktop from:"
        echo "   ${CYAN}https://www.docker.com/products/docker-desktop${NC}"
        exit 1
    fi
    print_success "Docker is installed"
    
    # Check if Docker is running
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker is not running."
        echo ""
        echo "   Please start Docker Desktop and try again."
        exit 1
    fi
    print_success "Docker is running"
}

# Check Docker Compose
check_docker_compose() {
    echo ""
    echo -e "${BLUE}🔍 Checking Docker Compose...${NC}"
    if ! docker compose version >/dev/null 2>&1 && ! command -v docker-compose &> /dev/null; then
        print_error "Docker Compose is not installed."
        echo ""
        echo "   Docker Compose is usually included with Docker Desktop."
        echo "   If not, install it from:"
        echo "   ${CYAN}https://docs.docker.com/compose/install/${NC}"
        exit 1
    fi
    print_success "Docker Compose is available"
}

# Check installation directory
check_install_dir() {
    echo ""
    echo -e "${BLUE}🔍 Checking installation directory...${NC}"
    if [ ! -d "$INSTALL_DIR" ]; then
        echo -e "${YELLOW}   Creating $INSTALL_DIR${NC}"
        mkdir -p "$INSTALL_DIR"
    fi
    print_success "Installation directory ready: $INSTALL_DIR"
    
    # Check if it's in PATH
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        print_warning "$INSTALL_DIR is not in your PATH"
        echo ""
        echo "   Add this to your ~/.zshrc or ~/.bashrc:"
        echo -e "   ${YELLOW}export PATH=\"\$HOME/.local/bin:\$PATH\"${NC}"
        echo ""
    else
        print_success "$INSTALL_DIR is in your PATH"
    fi
}

# Check/create workspace directory
check_workspace() {
    echo ""
    echo -e "${BLUE}🔍 Checking workspace directory...${NC}"
    if [ ! -d "$WORKSPACE_PATH" ]; then
        echo -e "${YELLOW}   Creating workspace at $WORKSPACE_PATH${NC}"
        mkdir -p "$WORKSPACE_PATH"
    fi
    print_success "Workspace ready: $WORKSPACE_PATH"
}

# Ensure host-side Ollama is installed and reachable
check_ollama() {
    echo ""
    echo -e "${BLUE}🔍 Checking host Ollama service...${NC}"

    if ! command -v ollama >/dev/null 2>&1; then
        print_warning "Ollama CLI not found on host"
        echo ""
        echo "   Download it from:"
        echo "   ${CYAN}https://ollama.com/download${NC}"
    else
        print_success "Ollama CLI detected"
    fi

    if command -v curl >/dev/null 2>&1; then
        if curl -fsS "http://127.0.0.1:11434/api/version" >/dev/null 2>&1; then
            print_success "Ollama service responding on localhost:11434"
        else
            print_warning "Ollama service not reachable on localhost:11434"
            echo "   Start it with: ${CYAN}ollama serve${NC}"
            echo "   or set ${CYAN}OLLAMA_HOST${NC} in .env to match your endpoint"
        fi
    else
        print_info "curl not available; skipping Ollama reachability check"
    fi
}

# Build Docker images
build_images() {
    echo ""
    echo -e "${MAGENTA}━━━ Building DevTrack Docker Images ━━━${NC}"
    echo ""
    
    SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
    cd "$SCRIPT_DIR"
    
    echo -e "${BLUE}📦 Building DevTrack image...${NC}"
    echo -e "${YELLOW}   This may take several minutes on first run...${NC}"
    
    if docker build -t devtrack:latest . ; then
        print_success "DevTrack image built successfully"
    else
        print_error "Failed to build DevTrack image"
        exit 1
    fi
}

# Deploy wrapper script
deploy_wrapper() {
    echo ""
    echo -e "${MAGENTA}━━━ Deploying DevTrack Wrapper ━━━${NC}"
    echo ""
    
    SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
    
    # Detect OS
    OS_TYPE="$(uname -s)"
    case "$OS_TYPE" in
        Linux*)     
            WRAPPER_SOURCE="$SCRIPT_DIR/devtrack-wrapper.sh"
            WRAPPER_DEST="$INSTALL_DIR/devtrack"
            ;;
        Darwin*)    
            WRAPPER_SOURCE="$SCRIPT_DIR/devtrack-wrapper.sh"
            WRAPPER_DEST="$INSTALL_DIR/devtrack"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            print_warning "Windows detected - Please use devtrack.bat or devtrack.ps1"
            echo ""
            echo "   Copy these files to a directory in your PATH:"
            echo "   - devtrack-wrapper.bat → devtrack.bat"
            echo "   - devtrack-wrapper.ps1 (required by .bat)"
            return 0
            ;;
        *)
            print_error "Unsupported operating system: $OS_TYPE"
            exit 1
            ;;
    esac
    
    if [ ! -f "$WRAPPER_SOURCE" ]; then
        print_error "Wrapper script not found at $WRAPPER_SOURCE"
        exit 1
    fi
    
    echo -e "${BLUE}🚀 Installing devtrack command...${NC}"
    if cp "$WRAPPER_SOURCE" "$WRAPPER_DEST"; then
        chmod +x "$WRAPPER_DEST"
        print_success "devtrack installed to $INSTALL_DIR"
    else
        print_error "Failed to install devtrack wrapper"
        exit 1
    fi
}

# Start services
start_services() {
    echo ""
    echo -e "${MAGENTA}━━━ Starting DevTrack Services ━━━${NC}"
    echo ""
    
    SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
    cd "$SCRIPT_DIR"
    
    echo -e "${BLUE}🚀 Starting containers...${NC}"
    
    export DEVTRACK_WORKSPACE="$WORKSPACE_PATH"
    
    if docker compose up -d devtrack; then
        print_success "DevTrack services started"
    else
        print_error "Failed to start services"
        exit 1
    fi
    
    # Wait for services to be healthy
    echo ""
    echo -e "${BLUE}⏳ Waiting for services to be ready...${NC}"
    sleep 3
    
    # Check DevTrack
    if docker ps --format '{{.Names}}' | grep -q "devtrack-app"; then
        print_success "DevTrack service is running"
    else
        print_warning "DevTrack service may not be running"
    fi
}

# Main installation
clear
print_header

echo -e "${GREEN}This installer will:${NC}"
echo "  • Check Docker and Docker Compose"
echo "  • Build DevTrack container image"
echo "  • Deploy devtrack CLI wrapper"
echo "  • Verify host-side Ollama availability"
echo "  • Start the DevTrack container"
echo ""
echo -ne "${CYAN}Press Enter to continue...${NC}"
read -r

clear
print_header

check_docker
check_docker_compose
check_install_dir
check_workspace
check_ollama

build_images
deploy_wrapper
start_services

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║              INSTALLATION COMPLETE!                      ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}🎉 DevTrack installation finished successfully!${NC}"
echo ""

echo -e "${MAGENTA}Installation Summary:${NC}"
echo -e "  ${GREEN}✓${NC} DevTrack CLI → ${CYAN}$INSTALL_DIR/devtrack${NC}"
echo -e "  ${GREEN}✓${NC} Workspace → ${CYAN}$WORKSPACE_PATH${NC}"
echo -e "  ${GREEN}✓${NC} Running in Docker containers"
echo ""

echo -e "${MAGENTA}Quick Start:${NC}"
echo -e "  ${YELLOW}devtrack${NC}                 - Launch interactive TUI"
echo -e "  ${YELLOW}devtrack --help${NC}          - Show all commands"
echo -e "  ${YELLOW}devtrack learning-status${NC} - Check AI learning status"
echo ""

echo -e "${MAGENTA}Docker Management:${NC}"
echo -e "  ${YELLOW}docker compose ps${NC}        - View running services"
echo -e "  ${YELLOW}docker compose logs -f${NC}   - View service logs"
echo -e "  ${YELLOW}docker compose stop${NC}      - Stop services"
echo -e "  ${YELLOW}docker compose start${NC}     - Start services"
echo ""

# Check PATH warning
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "${YELLOW}⚠️  IMPORTANT:${NC} Add ${CYAN}$INSTALL_DIR${NC} to your PATH"
    echo ""
    echo "   Add this line to your ${CYAN}~/.zshrc${NC}:"
    echo -e "   ${GREEN}export PATH=\"\$HOME/.local/bin:\$PATH\"${NC}"
    echo ""
    echo "   Then reload your shell:"
    echo -e "   ${YELLOW}source ~/.zshrc${NC}"
    echo ""
fi

echo -e "${GREEN}Ready to use DevTrack!${NC}"
echo ""
