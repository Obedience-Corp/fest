#!/bin/bash
# Installation script for fest CLI

set -euo pipefail

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

# Set OS name used in release asset names
case "$OS" in
    darwin|linux)
        ;;
    *)
        echo "Unsupported OS: $OS" >&2
        exit 1
        ;;
esac

fail_no_verified_release() {
    cat >&2 <<EOF
error: $1

This installer refuses to install a fest binary that cannot be
downloaded from and checksum-verified against the official
Obedience-Corp/fest GitHub releases. Use one of these instead:

  go install github.com/Obedience-Corp/fest/cmd/fest@latest

  git clone https://github.com/Obedience-Corp/fest
  cd fest && just install
EOF
    exit 1
}

GITHUB_REPO="Obedience-Corp/fest"
VERSION=${1:-latest}

if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/') || \
        fail_no_verified_release "could not determine the latest fest release from the GitHub API"
fi

if [ -z "$VERSION" ]; then
    fail_no_verified_release "could not determine the latest fest release from the GitHub API"
fi

echo "Installing fest ${VERSION} for ${OS}/${ARCH}..."

ARCHIVE="fest-${VERSION}-${OS}-${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${ARCHIVE}"
CHECKSUMS_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/checksums.txt"

TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

echo "Downloading from ${DOWNLOAD_URL}..."
if ! curl -fsSL -o "${TEMP_DIR}/${ARCHIVE}" "${DOWNLOAD_URL}"; then
    fail_no_verified_release "no release archive found at ${DOWNLOAD_URL}"
fi

echo "Downloading checksums from ${CHECKSUMS_URL}..."
if ! curl -fsSL -o "${TEMP_DIR}/checksums.txt" "${CHECKSUMS_URL}"; then
    fail_no_verified_release "no checksums.txt published at ${CHECKSUMS_URL}; refusing to install an unverified binary"
fi

EXPECTED_SHA=$(grep " ${ARCHIVE}\$" "${TEMP_DIR}/checksums.txt" | awk '{print $1}')
if [ -z "$EXPECTED_SHA" ]; then
    fail_no_verified_release "checksums.txt has no entry for ${ARCHIVE}; refusing to install an unverified binary"
fi

if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL_SHA=$(sha256sum "${TEMP_DIR}/${ARCHIVE}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL_SHA=$(shasum -a 256 "${TEMP_DIR}/${ARCHIVE}" | awk '{print $1}')
else
    fail_no_verified_release "neither sha256sum nor shasum is available to verify the download"
fi

if [ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]; then
    fail_no_verified_release "checksum mismatch for ${ARCHIVE} (expected ${EXPECTED_SHA}, got ${ACTUAL_SHA})"
fi

echo "Checksum verified."

echo "Extracting..."
tar -xzf "${TEMP_DIR}/${ARCHIVE}" -C "${TEMP_DIR}"

# Install to /usr/local/bin (may require sudo)
INSTALL_PATH="/usr/local/bin/fest"

if [ -w "/usr/local/bin" ]; then
    mv "${TEMP_DIR}/fest" "${INSTALL_PATH}"
else
    echo "Installing to /usr/local/bin requires sudo access..."
    sudo mv "${TEMP_DIR}/fest" "${INSTALL_PATH}"
fi

chmod +x "${INSTALL_PATH}"

echo "✅ fest installed successfully to ${INSTALL_PATH}"
echo "Run 'fest --version' to verify the installation"
