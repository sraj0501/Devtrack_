#!/bin/bash

# DevTrack Docker Wrapper
# Makes DevTrack container callable like a native CLI application

set -e

# Configuration
DOCKER_IMAGE="devtrack:latest"
CONTAINER_NAME="devtrack-app"
WORKSPACE_PATH="${WORKSPACE_PATH:-$HOME/workspace}"

# Check if Docker is running
if ! docker info >/dev/null 2>&1; then
    echo "Error: Docker is not running. Please start Docker Desktop." >&2
    exit 1
fi

# Check if docker-compose services are running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    # Check if image exists
    if ! docker image inspect "$DOCKER_IMAGE" >/dev/null 2>&1; then
        echo "Error: DevTrack image not found. Please run the installer first." >&2
        exit 1
    fi
    
    # Start services if they're not running
    SCRIPT_DIR="$(dirname "$(readlink -f "$0" 2>/dev/null || realpath "$0" 2>/dev/null || echo "$0")")"
    REPO_DIR="$(dirname "$SCRIPT_DIR")"
    
    if [ -f "$REPO_DIR/docker-compose.yml" ]; then
        cd "$REPO_DIR"
        docker-compose up -d >/dev/null 2>&1
    else
        echo "Error: docker-compose.yml not found. DevTrack may not be properly installed." >&2
        exit 1
    fi
fi

# Get current directory and find git repository root
CURRENT_DIR="$(pwd)"
GIT_ROOT="$CURRENT_DIR"

# Find git repository root by traversing up
while [ "$GIT_ROOT" != "/" ]; do
    if [ -d "$GIT_ROOT/.git" ]; then
        break
    fi
    GIT_ROOT="$(dirname "$GIT_ROOT")"
done

# If we found a git repo, calculate its path in the container (/home/host prefix)
if [ -d "$GIT_ROOT/.git" ]; then
    # Map host path to container path: $HOME/... -> /home/host/...
    CONTAINER_WORKDIR="${GIT_ROOT/#$HOME/\/home\/host}"
    
    # For 'start' command, run detached so daemon persists
    if [ "$1" = "start" ]; then
        docker exec -d \
            -w "$CONTAINER_WORKDIR" \
            -e TERM="$TERM" \
            -e COLORTERM="$COLORTERM" \
            "$CONTAINER_NAME" \
            devtrack "$@"
        
        # Give it a moment to start
        sleep 1
        
        # Show status
        docker exec \
            -w "$CONTAINER_WORKDIR" \
            "$CONTAINER_NAME" \
            devtrack status
    else
        # Run interactively for other commands
        docker exec -it \
            -w "$CONTAINER_WORKDIR" \
            -e TERM="$TERM" \
            -e COLORTERM="$COLORTERM" \
            "$CONTAINER_NAME" \
            devtrack "$@"
    fi
else
    # No git repo found, use workspace as fallback
    REL_PATH="${CURRENT_DIR#$WORKSPACE_PATH}"
    if [ "$REL_PATH" = "$CURRENT_DIR" ]; then
        WORKDIR="/workspace"
    else
        WORKDIR="/workspace${REL_PATH}"
    fi
    
    # For 'start' command, run detached so daemon persists
    if [ "$1" = "start" ]; then
        docker exec -d \
            -w "$WORKDIR" \
            -e TERM="$TERM" \
            -e COLORTERM="$COLORTERM" \
            "$CONTAINER_NAME" \
            devtrack "$@"
        
        # Give it a moment to start
        sleep 1
        
        # Show status
        docker exec \
            -w "$WORKDIR" \
            "$CONTAINER_NAME" \
            devtrack status
    else
        # Run interactively for other commands
        docker exec -it \
            -w "$WORKDIR" \
            -e TERM="$TERM" \
            -e COLORTERM="$COLORTERM" \
            "$CONTAINER_NAME" \
            devtrack "$@"
    fi
fi
