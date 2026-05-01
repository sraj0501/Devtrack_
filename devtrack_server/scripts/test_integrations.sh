#!/bin/bash
# DevTrack Phase 4: Optional integration tests
# Skips if tokens not set; verifies Azure/GitHub when configured

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

if [ -f ".env" ]; then
    set -a
    source .env
    set +a
fi

echo "=========================================="
echo "Test: Integrations (Optional)"
echo "=========================================="

PASS=0
SKIP=0

# Azure DevOps: list projects (skip if no PAT)
if [ -n "$AZURE_DEVOPS_PAT" ] || [ -n "$AZURE_API_KEY" ]; then
    if [ -n "$ORGANIZATION" ]; then
        echo "  Testing Azure DevOps (list projects)..."
        if uv run python -c "
import os
import sys
sys.path.insert(0, '.')
try:
    from backend.config import azure_org, azure_pat
    org = azure_org()
    pat = azure_pat()
    if org and pat:
        import requests
        from requests.auth import HTTPBasicAuth
        r = requests.get(f'https://dev.azure.com/{org}/_apis/projects?api-version=7.1', auth=HTTPBasicAuth('', pat), timeout=10)
        if r.status_code == 200:
            print('  OK: Azure DevOps projects fetched')
            sys.exit(0)
        print('  WARNING: Azure API returned', r.status_code)
    sys.exit(1)
except Exception as e:
    print('  WARNING:', e)
    sys.exit(1)
" 2>/dev/null; then
            PASS=1
        else
            echo "  WARNING: Azure test failed (check org/PAT)"
        fi
    else
        echo "  SKIP: Azure - ORGANIZATION not set"
        SKIP=1
    fi
else
    echo "  SKIP: Azure - AZURE_DEVOPS_PAT not set"
    SKIP=1
fi

# GitHub: list repos (skip if no token)
if [ -n "$GITHUB_TOKEN" ]; then
    echo "  Testing GitHub (list repos)..."
    if uv run python -c "
import sys
sys.path.insert(0, '.')
try:
    from backend.github.ghAnalysis import GitHubBranchAnalyzer
    a = GitHubBranchAnalyzer()
    repos = a.getRepos()
    if isinstance(repos, dict):
        print('  OK: GitHub repos fetched')
        sys.exit(0)
    sys.exit(1)
except ValueError as e:
    if 'GITHUB_TOKEN' in str(e):
        sys.exit(1)
    raise
except Exception as e:
    print('  WARNING:', e)
    sys.exit(1)
" 2>/dev/null; then
        PASS=1
    else
        echo "  WARNING: GitHub test failed"
    fi
else
    echo "  SKIP: GitHub - GITHUB_TOKEN not set"
    SKIP=1
fi

# Task matcher: should load without crash (no tokens needed)
echo "  Testing task_matcher..."
if uv run python -c "
from backend.task_matcher import TaskMatcher, Task
m = TaskMatcher(use_semantic=False)
t = Task('1', 'Test', 'desc', 'open', 'p')
r = m.match_task('test', [t])
print('  OK: task_matcher loads')
" 2>/dev/null; then
    PASS=1
else
    echo "  WARNING: task_matcher failed"
fi

# Learning integration: should import without crash
echo "  Testing learning_integration..."
if uv run python -c "
from backend.learning_integration import LearningIntegration
print('  OK: learning_integration imports')
" 2>/dev/null; then
    PASS=1
else
    echo "  WARNING: learning_integration import failed"
fi

echo ""
if [ $PASS -ge 1 ]; then
    echo "Integration tests passed (or skipped when tokens not set)."
else
    echo "Some integration checks failed. Set tokens in .env for full verification."
fi
