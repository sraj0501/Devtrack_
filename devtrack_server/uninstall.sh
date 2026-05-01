#!/bin/bash

# DevTrack Uninstaller
# Removes all DevTrack components, Docker containers, images, and data

set -e

# Configuration
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
    echo -e "${RED}╔══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║        DevTrack - Uninstaller                            ║${NC}"
    echo -e "${RED}╚══════════════════════════════════════════════════════════╝${NC}"
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

# Confirmation
confirm_uninstall() {
    echo -e "${YELLOW}WARNING: This will remove:${NC}"
    echo "  • DevTrack Docker containers and images"
    echo "  • All DevTrack data and configuration"
    echo "  • DevTrack CLI wrapper from $INSTALL_DIR"
    echo "  • Docker volumes (database and config)"
    echo ""
    echo -e "${RED}This action CANNOT be undone!${NC}"
    echo ""
    echo -ne "${CYAN}Are you sure you want to uninstall DevTrack? (yes/no): ${NC}"
    read -r response
    
    if [[ ! "$response" =~ ^[Yy][Ee][Ss]$ ]]; then
        echo ""
        echo "Uninstallation cancelled."
        exit 0
    fi
}

# Stop and remove containers
stop_containers() {
    echo ""
    echo -e "${MAGENTA}━━━ Stopping DevTrack Containers ━━━${NC}"
    echo ""
    
    SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
    cd "$SCRIPT_DIR"
    
    if [ -f "docker-compose.yml" ]; then
        echo -e "${BLUE}🛑 Stopping containers...${NC}"
        if docker compose down 2>/dev/null; then
            print_success "Containers stopped and removed"
        else
            print_warning "No running containers found or already removed"
        fi
    else
        print_warning "docker-compose.yml not found, trying manual cleanup"
        
        # Try to stop containers manually
        for container in devtrack-app; do
            if docker ps -a --format '{{.Names}}' | grep -q "^${container}$"; then
                echo -e "${BLUE}Stopping $container...${NC}"
                docker stop "$container" 2>/dev/null || true
                docker rm "$container" 2>/dev/null || true
                print_success "$container removed"
            fi
        done
    fi
}

# Remove Docker images
remove_images() {
    echo ""
    echo -e "${MAGENTA}━━━ Removing DevTrack Images ━━━${NC}"
    echo ""
    
    echo -e "${BLUE}🗑️  Removing DevTrack image...${NC}"
    if docker image inspect devtrack:latest >/dev/null 2>&1; then
        docker rmi devtrack:latest 2>/dev/null || true
        print_success "DevTrack image removed"
    else
        print_info "DevTrack image not found (already removed)"
    fi
    
}

# Remove Docker volumes
remove_volumes() {
    echo ""
    echo -e "${MAGENTA}━━━ Removing DevTrack Data ━━━${NC}"
    echo ""
    
    echo -e "${RED}WARNING: This will delete all DevTrack data, including:${NC}"
    echo "  • Database and task history"
    echo "  • Configuration files"
    echo ""
    echo -ne "${CYAN}Delete all data volumes? (yes/no): ${NC}"
    read -r remove_data
    
    if [[ "$remove_data" =~ ^[Yy]([Ee][Ss])?$ ]]; then
        for volume in devtrack-data devtrack-config; do
            if docker volume inspect "$volume" >/dev/null 2>&1; then
                echo -e "${BLUE}Removing volume $volume...${NC}"
                docker volume rm "$volume" 2>/dev/null || true
                print_success "$volume removed"
            fi
        done
    else
        print_info "Skipping data volume removal (data preserved)"
    fi
}

# Remove wrapper script
remove_wrapper() {
    echo ""
    echo -e "${MAGENTA}━━━ Removing CLI Wrapper ━━━${NC}"
    echo ""
    
    WRAPPER_PATH="$INSTALL_DIR/devtrack"
    
    if [ -f "$WRAPPER_PATH" ]; then
        echo -e "${BLUE}🗑️  Removing devtrack command...${NC}"
        rm -f "$WRAPPER_PATH"
        print_success "CLI wrapper removed from $INSTALL_DIR"
    else
        print_info "CLI wrapper not found (already removed)"
    fi
}

# Remove network
remove_network() {
    echo ""
    echo -e "${MAGENTA}━━━ Cleaning Up Network ━━━${NC}"
    echo ""
    
    if docker network inspect devtrack-network >/dev/null 2>&1; then
        echo -e "${BLUE}Removing network...${NC}"
        docker network rm devtrack-network 2>/dev/null || true
        print_success "Network removed"
    else
        print_info "Network not found (already removed)"
    fi
}

# Optional: Remove workspace
remove_workspace_prompt() {
    echo ""
    echo -e "${MAGENTA}━━━ Workspace Directory ━━━${NC}"
    echo ""
    
    if [ -d "$WORKSPACE_PATH" ]; then
        echo -e "${YELLOW}Workspace directory: $WORKSPACE_PATH${NC}"
        echo ""
        echo -ne "${CYAN}Remove workspace directory? (yes/no): ${NC}"
        read -r remove_workspace
        
        if [[ "$remove_workspace" =~ ^[Yy]([Ee][Ss])?$ ]]; then
            echo -e "${BLUE}🗑️  Removing workspace...${NC}"
            rm -rf "$WORKSPACE_PATH"
            print_success "Workspace removed"
        else
            print_info "Workspace preserved at $WORKSPACE_PATH"
        fi
    fi
}

# Main uninstallation
clear
print_header
confirm_uninstall

clear
print_header

stop_containers
remove_images
remove_volumes
remove_wrapper
remove_network
remove_workspace_prompt

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║            UNINSTALLATION COMPLETE!                      ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}DevTrack has been removed from your system.${NC}"
echo ""
echo -e "${BLUE}Cleanup summary:${NC}"
echo "  • Docker containers stopped and removed"
echo "  • Docker images removed"
echo "  • CLI wrapper removed"
echo "  • Network cleaned up"
echo ""
echo -e "${YELLOW}Note: Docker itself is still installed and running.${NC}"
echo ""
echo "To reinstall DevTrack, run:"
echo -e "  ${CYAN}./install_dependencies.sh${NC}"
echo ""
