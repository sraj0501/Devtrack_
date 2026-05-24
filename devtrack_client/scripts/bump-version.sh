#!/usr/bin/env bash
# bump-version.sh — increment the git version tag and push.
# Pushing the tag triggers the release.yml GitHub Actions workflow.
#
# Usage: scripts/bump-version.sh [patch|minor|major]

set -euo pipefail

BUMP="${1:-patch}"

# Get latest semver tag (default v0.0.0)
LATEST=$(git tag --sort=-version:refname | grep '^v' | head -1 2>/dev/null || echo "v0.0.0")
echo "Current version: $LATEST"

VER="${LATEST#v}"
MAJOR=$(echo "$VER" | cut -d. -f1)
MINOR=$(echo "$VER" | cut -d. -f2)
PATCH=$(echo "$VER" | cut -d. -f3)

case "$BUMP" in
  major) MAJOR=$((MAJOR+1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR+1)); PATCH=0 ;;
  patch) PATCH=$((PATCH+1)) ;;
  *)
    echo "Usage: $0 [patch|minor|major]"
    exit 1
    ;;
esac

NEXT="v${MAJOR}.${MINOR}.${PATCH}"
echo "New version:     $NEXT"

git tag -a "$NEXT" -m "Release $NEXT"
git push origin "$NEXT"
echo "✅ Pushed tag $NEXT — GitHub Actions will build and publish the release."
