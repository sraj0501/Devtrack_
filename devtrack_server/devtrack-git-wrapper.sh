#!/bin/bash
# DevTrack Git Wrapper
# Enhances git commands with AI-powered features while maintaining git compatibility
# All paths from .env - no hardcoded fallbacks

set -e

# Load .env: DEVTRACK_ENV_FILE, PROJECT_ROOT/.env, or relative to script (when in PROJECT_ROOT/bin)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
for env_candidate in "$DEVTRACK_ENV_FILE" "$PROJECT_ROOT/.env" "$SCRIPT_DIR/../.env"; do
    if [ -n "$env_candidate" ] && [ -f "$env_candidate" ]; then
        set -a
        source "$env_candidate"
        set +a
        break
    fi
done

# PROJECT_ROOT must be set from .env
if [ -z "$PROJECT_ROOT" ]; then
    echo "Error: PROJECT_ROOT not set. Ensure .env exists and defines PROJECT_ROOT."
    echo "  Set DEVTRACK_ENV_FILE or run from automation_tools/bin/"
    exit 1
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Support both: devtrack git commit ... and devtrack-git commit ...
if [ "$1" = "git" ]; then
    shift  # Remove 'git' from args
fi

GIT_COMMAND="$1"
if [ -z "$GIT_COMMAND" ]; then
    echo "Usage: devtrack git <command> [args...]  OR  devtrack-git <command> [args...]"
    echo ""
    echo "Commands:"
    echo "  commit [args...]     AI-enhanced commit with interactive refinement"
    echo "  history [n]          Show last n DevTrack-enhanced commit messages (default: 10)"
    echo "  messages [n]         Alias for history"
    echo ""
    echo "Examples:"
    echo "  devtrack git commit -m 'message'        (AI-enhanced, interactive)"
    echo "  devtrack git commit -m 'message' --dry-run  (preview only)"
    echo "  devtrack git history 5                  (show last 5 commits)"
    echo "  devtrack git messages                   (show last 10 commits)"
    exit 1
fi
shift  # Remove the subcommand (commit, add, etc.)

# Handle git commit with AI enhancement
if [ "$GIT_COMMAND" = "commit" ]; then
    # Check if there are staged changes
    if git diff --cached --quiet 2>/dev/null; then
        echo "No changes staged for commit."
        echo "Use 'devtrack git add <files>' to stage changes first."
        exit 1
    fi

    REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

    echo -e "${BLUE}🤖 DevTrack AI-Enhanced Commit${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    # Parse commit args to extract user message and --dry-run
    USER_MESSAGE=""
    COMMIT_ARGS=()
    SKIP_NEXT=false
    EXPLICIT_DRY_RUN=false  # Explicit --dry-run flag (preview only, no interaction)

    for arg in "$@"; do
        if [ "$SKIP_NEXT" = true ]; then
            USER_MESSAGE="$arg"
            SKIP_NEXT=false
        elif [ "$arg" = "-m" ] || [ "$arg" = "--message" ]; then
            SKIP_NEXT=true
        elif [ "$arg" = "--dry-run" ] || [ "$arg" = "-n" ]; then
            EXPLICIT_DRY_RUN=true
        else
            COMMIT_ARGS+=("$arg")
        fi
    done

    # Show what's being committed
    echo -e "${YELLOW}Staged changes:${NC}"
    git diff --cached --stat | head -10
    echo ""

    # If explicit --dry-run, show AI enhancement but don't commit
    if [ "$EXPLICIT_DRY_RUN" = true ]; then
        echo -e "${YELLOW}(preview mode: AI enhancement only, no commit, no interaction)${NC}"
        echo ""

        # Create temp file for message
        TEMP_MSG=$(mktemp)
        if [ -n "$USER_MESSAGE" ]; then
            echo "$USER_MESSAGE" > "$TEMP_MSG"
        else
            echo "auto-generated" > "$TEMP_MSG"
        fi

        # Run enhancer to show AI capability
        cd "$REPO_ROOT"
        export GIT_DIR="$REPO_ROOT/.git"

        echo -e "${BLUE}🔍 Analyzing with AI...${NC}"
        ENHANCEMENT_OUTPUT=$(uv run --directory "$PROJECT_ROOT" python "$PROJECT_ROOT/backend/commit_message_enhancer.py" "$TEMP_MSG" auto 2>&1)

        if echo "$ENHANCEMENT_OUTPUT" | grep -q "enhanced"; then
            ENHANCED_MESSAGE=$(grep -v "^#" "$TEMP_MSG" | sed '/^$/d')
            echo -e "${GREEN}✓ AI-enhanced message:${NC}"
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo "$ENHANCED_MESSAGE" | sed 's/^/  /'
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        else
            GENERATED_MESSAGE=$(grep -v "^#" "$TEMP_MSG" | sed '/^$/d')
            if [ -z "$GENERATED_MESSAGE" ]; then
                GENERATED_MESSAGE="$USER_MESSAGE"
            fi
            echo -e "${YELLOW}Generated message:${NC}"
            echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo "$GENERATED_MESSAGE" | sed 's/^/  /'
            echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        fi

        echo ""
        echo -e "${GREEN}✓ Preview complete. No commit made.${NC}"
        echo -e "${BLUE}Run without --dry-run to commit with interactive refinement.${NC}"

        rm -f "$TEMP_MSG"
        exit 0
    fi

    # Normal flow: AI enhancement with up to 5 attempts
    echo -e "${BLUE}✨ AI-Enhanced Commit Flow (up to 5 attempts)${NC}"
    echo ""

    # Create temp file for message
    TEMP_MSG=$(mktemp)
    if [ -n "$USER_MESSAGE" ]; then
        echo "$USER_MESSAGE" > "$TEMP_MSG"
    else
        echo "auto-generated" > "$TEMP_MSG"
    fi

    # Run enhancer
    cd "$REPO_ROOT"
    export GIT_DIR="$REPO_ROOT/.git"

    # Interactive loop for message refinement (max 5 attempts)
    COMMIT_CONFIRMED=false
    ATTEMPT=0
    MAX_ATTEMPTS=5
    
    while [ "$COMMIT_CONFIRMED" = false ] && [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
        ATTEMPT=$((ATTEMPT + 1))
        echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${BLUE}Attempt $ATTEMPT/$MAX_ATTEMPTS${NC}"
        echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo ""

        # Generate/enhance message
        echo -e "${BLUE}🔍 Analyzing with AI...${NC}"
        ENHANCEMENT_OUTPUT=$(uv run --directory "$PROJECT_ROOT" python "$PROJECT_ROOT/backend/commit_message_enhancer.py" "$TEMP_MSG" auto 2>&1)

        if echo "$ENHANCEMENT_OUTPUT" | grep -q "enhanced"; then
            # Read enhanced message (remove comment lines)
            ENHANCED_MESSAGE=$(grep -v "^#" "$TEMP_MSG" | sed '/^$/d')

            echo -e "${GREEN}✓ AI-generated message:${NC}"
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo "$ENHANCED_MESSAGE" | sed 's/^/  /'
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo ""
        else
            # AI enhancement failed - use original or generated message
            ENHANCED_MESSAGE=$(grep -v "^#" "$TEMP_MSG" | sed '/^$/d')
            if [ -z "$ENHANCED_MESSAGE" ]; then
                ENHANCED_MESSAGE="$USER_MESSAGE"
            fi

            echo -e "${YELLOW}⚠️  AI enhancement unavailable${NC}"
            echo -e "${YELLOW}Generated message:${NC}"
            echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo "$ENHANCED_MESSAGE" | sed 's/^/  /'
            echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo ""
        fi

        # Interactive options
        echo -e "${BLUE}What would you like to do?${NC}"
        echo -e "  ${GREEN}[A]${NC}ccept and commit"
        echo -e "  ${YELLOW}[E]${NC}nhance/improve message"
        echo -e "  ${YELLOW}[R]${NC}egenerate from scratch"
        if [ $ATTEMPT -lt $MAX_ATTEMPTS ]; then
            echo -e "  ${YELLOW}[C]${NC}ancel"
        else
            echo -e "  ${YELLOW}[C]${NC}ancel (last attempt)"
        fi
        echo ""
        echo -ne "${BLUE}Choice (A/E/R/C): ${NC}"
        read -r -n 1 CHOICE
        echo ""
        echo ""

        case "$CHOICE" in
            [Aa])
                # Accept the message
                COMMIT_CONFIRMED=true
                ;;
            [Ee])
                # Enhance current message (use it as input for next iteration)
                if [ $ATTEMPT -lt $MAX_ATTEMPTS ]; then
                    echo "$ENHANCED_MESSAGE" > "$TEMP_MSG"
                else
                    echo -e "${YELLOW}✗ Maximum attempts reached. Cannot enhance further.${NC}"
                    echo ""
                fi
                ;;
            [Rr])
                # Regenerate from scratch (use original message as input)
                if [ $ATTEMPT -lt $MAX_ATTEMPTS ]; then
                    echo -e "${YELLOW}🔄 Regenerating from scratch...${NC}"
                    echo ""
                    if [ -n "$USER_MESSAGE" ]; then
                        echo "$USER_MESSAGE" > "$TEMP_MSG"
                    else
                        echo "auto-generated" > "$TEMP_MSG"
                    fi
                else
                    echo -e "${YELLOW}✗ Maximum attempts reached. Cannot regenerate.${NC}"
                    echo ""
                fi
                ;;
            [Cc])
                echo -e "${YELLOW}✗ Commit cancelled.${NC}"
                rm -f "$TEMP_MSG"
                exit 0
                ;;
            *)
                echo -e "${YELLOW}Invalid choice. Please enter A, E, R, or C.${NC}"
                echo ""
                ATTEMPT=$((ATTEMPT - 1))  # Don't count invalid input as attempt
                ;;
        esac
    done

    # Check if confirmed or max attempts reached
    if [ "$COMMIT_CONFIRMED" = false ]; then
        echo -e "${RED}✗ Maximum attempts ($MAX_ATTEMPTS) reached without acceptance.${NC}"
        echo -e "${RED}Commit cancelled.${NC}"
        rm -f "$TEMP_MSG"
        exit 1
    fi
    
    # Commit if confirmed
    if [ "$COMMIT_CONFIRMED" = true ]; then
        # Read final message
        FINAL_MESSAGE=$(grep -v "^#" "$TEMP_MSG" | sed '/^$/d')
        
        # Commit with enhanced message (commit only, no auto-push)
        # When DEVTRACK_COMMIT_ONLY=true, temporarily disable post-commit hook (often does push)
        POST_COMMIT_HOOK=""
        if [ "${DEVTRACK_COMMIT_ONLY:-false}" = "true" ]; then
            GIT_DIR_REAL="$(cd "$REPO_ROOT" && git rev-parse --git-dir)"
            POST_COMMIT_HOOK="$GIT_DIR_REAL/hooks/post-commit"
            if [ -f "$POST_COMMIT_HOOK" ] && [ -x "$POST_COMMIT_HOOK" ]; then
                chmod -x "$POST_COMMIT_HOOK"
            else
                POST_COMMIT_HOOK=""
            fi
        fi
        
        git commit -F "$TEMP_MSG" "${COMMIT_ARGS[@]}"
        COMMIT_RESULT=$?
        
        if [ -n "$POST_COMMIT_HOOK" ] && [ -f "$POST_COMMIT_HOOK" ]; then
            chmod +x "$POST_COMMIT_HOOK"
        fi
        
        if [ $COMMIT_RESULT -eq 0 ]; then
            echo ""
            echo -e "${GREEN}✓ Committed successfully with AI-enhanced message!${NC}"
            echo ""
            
            # Interactive feedback prompt
            echo -e "${YELLOW}🔔 DevTrack: Log this work? (y/n)${NC}"
            read -r -n 1 RESPONSE
            echo ""
            
            if [[ "$RESPONSE" =~ ^[Yy]$ ]]; then
                # Get the commit hash
                COMMIT_HASH=$(git rev-parse HEAD)
                COMMIT_SHORT=$(git rev-parse --short HEAD)
                
                # Extract ticket ID if present (common patterns: AB-234, PROJ-123, #234)
                TICKET_ID=$(echo "$FINAL_MESSAGE" | grep -oE '([A-Z]+-[0-9]+|#[0-9]+)' | head -1 || true)
                
                # Show logged message (first line only)
                FIRST_LINE=$(echo "$FINAL_MESSAGE" | head -1)
                echo -e "${GREEN}✓ Logged work: ${FIRST_LINE}${NC}"
                
                # Detect and show git provider
                REPO_URL=$(git config --get remote.origin.url 2>/dev/null || true)
                if [ -n "$REPO_URL" ]; then
                    if echo "$REPO_URL" | grep -q "github.com"; then
                        echo -e "${GREEN}✓ Logged to GitHub commit ${COMMIT_SHORT}${NC}"
                    elif echo "$REPO_URL" | grep -q "gitlab"; then
                        echo -e "${GREEN}✓ Logged to GitLab commit ${COMMIT_SHORT}${NC}"
                    elif echo "$REPO_URL" | grep -q "dev.azure.com"; then
                        echo -e "${GREEN}✓ Logged to Azure Repos commit ${COMMIT_SHORT}${NC}"
                        if [ -n "$TICKET_ID" ]; then
                            echo -e "${GREEN}✓ Work item ${TICKET_ID} referenced${NC}"
                        fi
                    else
                        echo -e "${GREEN}✓ Logged to Git commit ${COMMIT_SHORT}${NC}"
                    fi
                else
                    echo -e "${GREEN}✓ Logged to local Git commit ${COMMIT_SHORT}${NC}"
                fi
                
                # Daemon will handle project management sync if configured
                echo -e "${BLUE}  → DevTrack daemon will sync with project management...${NC}"
            else
                echo -e "${BLUE}  Skipped logging. Commit saved to Git.${NC}"
            fi
        fi
    else
        echo -e "${YELLOW}⚠️  Maximum regeneration attempts reached. Commit cancelled.${NC}"
        rm -f "$TEMP_MSG"
        exit 1
    fi
    
    rm -f "$TEMP_MSG"

# Handle git history/messages command
elif [ "$GIT_COMMAND" = "history" ] || [ "$GIT_COMMAND" = "messages" ]; then
    REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
    if [ -z "$REPO_ROOT" ]; then
        echo "Error: Not in a git repository"
        exit 1
    fi
    
    # Parse number of commits to show (default: 10)
    NUM_COMMITS=10
    if [ -n "$1" ] && [[ "$1" =~ ^[0-9]+$ ]]; then
        NUM_COMMITS="$1"
    fi
    
    echo -e "${BLUE}📜 DevTrack Commit History${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Showing last ${NUM_COMMITS} commits${NC}"
    echo ""
    
    cd "$REPO_ROOT"
    
    # Get commit hashes
    COMMIT_HASHES=($(git log -n "$NUM_COMMITS" --format="%H"))
    
    for hash in "${COMMIT_HASHES[@]}"; do
        # Get commit details
        AUTHOR=$(git log -1 --format="%an" "$hash")
        EMAIL=$(git log -1 --format="%ae" "$hash")
        DATE=$(git log -1 --format="%ad" --date=format:"%Y-%m-%d %H:%M" "$hash")
        FULL_MSG=$(git log -1 --format="%B" "$hash")
        
        # Check if message looks like DevTrack-enhanced (has body paragraph)
        # DevTrack messages typically have: subject line, blank line, then body paragraph
        HAS_BODY=false
        # Count lines - if 3+ lines and contains blank line, likely DevTrack-enhanced
        LINE_COUNT=$(echo "$FULL_MSG" | wc -l | tr -d ' ')
        if [ "$LINE_COUNT" -ge 3 ]; then
            # Check for pattern: non-empty line, blank line, then more content
            FIRST_LINE=$(echo "$FULL_MSG" | head -1)
            SECOND_LINE=$(echo "$FULL_MSG" | sed -n '2p')
            THIRD_LINE=$(echo "$FULL_MSG" | sed -n '3p')
            if [ -n "$FIRST_LINE" ] && [ -z "$SECOND_LINE" ] && [ -n "$THIRD_LINE" ]; then
                HAS_BODY=true
            fi
        fi
        
        # Display commit
        if [ "$HAS_BODY" = true ]; then
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo -e "${GREEN}✓ DevTrack Enhanced${NC}"
        else
            echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        fi
        
        echo -e "${BLUE}Commit:${NC} ${hash:0:8}"
        echo -e "${BLUE}Author:${NC} $AUTHOR <$EMAIL>"
        echo -e "${BLUE}Date:${NC}   $DATE"
        echo ""
        echo -e "${YELLOW}Message:${NC}"
        echo "$FULL_MSG" | sed 's/^/  /'
        echo ""
    done
    
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${BLUE}Tip:${NC} Commits marked with '✓ DevTrack Enhanced' have AI-enhanced messages"
    echo -e "     Use: ${YELLOW}devtrack git history <n>${NC} to show more/fewer commits"

# Handle all other git commands - pass through to git
else
    exec git "$GIT_COMMAND" "$@"
fi
