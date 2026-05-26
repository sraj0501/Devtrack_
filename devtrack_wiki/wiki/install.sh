#!/usr/bin/env sh
set -e

GITLAB_PROJECT="devtrack3_cloud%2Fdevtrack_client"
GITLAB_API="https://gitlab.com/api/v4/projects/${GITLAB_PROJECT}"

# Fetch latest version tag
VERSION=$(curl -sf "${GITLAB_API}/repository/tags?order_by=version&sort=desc&per_page=1" \
  | grep -o '"name":"v[^"]*"' | head -1 \
  | sed 's/"name":"//;s/".*//') \
  || VERSION=""

if [ -z "$VERSION" ]; then
  echo "Error: could not fetch latest version from GitLab. Check your network or visit https://devtrack.cloud/download"
  exit 1
fi

BASE_URL="${GITLAB_API}/packages/generic/devtrack/${VERSION}"
INSTALL_DIR="${DEVTRACK_INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS
OS=$(uname -s 2>/dev/null || echo "unknown")
case "$OS" in
  Linux)  os="linux"  ;;
  Darwin) os="darwin" ;;
  *)
    echo "Unsupported OS: $OS"
    echo "Download manually from https://devtrack.cloud/download"
    exit 1
    ;;
esac

# Detect architecture
ARCH=$(uname -m 2>/dev/null || echo "unknown")
case "$ARCH" in
  x86_64|amd64)  arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    echo "Download manually from https://devtrack.cloud/download"
    exit 1
    ;;
esac

ARCHIVE="devtrack_${os}_${arch}.tar.gz"
URL="${BASE_URL}/${ARCHIVE}"

echo "Detected: ${os}/${arch}"
echo "Downloading devtrack..."

mkdir -p "$INSTALL_DIR"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

if command -v curl > /dev/null 2>&1; then
  curl -fsSL "$URL" -o "${TMP_DIR}/${ARCHIVE}"
elif command -v wget > /dev/null 2>&1; then
  wget -q "$URL" -O "${TMP_DIR}/${ARCHIVE}"
else
  echo "Error: curl or wget is required."
  exit 1
fi

tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"
install -m 755 "${TMP_DIR}/devtrack" "${INSTALL_DIR}/devtrack"

echo "Installed to ${INSTALL_DIR}/devtrack"

# PATH reminder
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*)
    echo ""
    echo "Run: devtrack setup"
    ;;
  *)
    echo ""
    echo "Add to your PATH:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
    echo "Then run: devtrack setup"
    ;;
esac
