#!/bin/sh
#
# Dagu Installer Script
#
# This script downloads and installs the latest version of Dagu.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | sh
#
# Options:
#   --version <version>       Install a specific version (e.g., --version v1.2.3)
#   --install-dir <path>      Install to a custom directory (default: ~/.local/bin)
#   --prefix <path>           Alias for --install-dir
#   --working-dir <path>      Use a custom directory for temporary files (default: /tmp)
#
# Examples:
#   # Install latest version to default location (~/.local/bin)
#   curl -sSL https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | sh
#
#   # Install specific version
#   curl -sSL https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | sh -s -- --version v1.2.3
#
#   # Install to /usr/local/bin (requires sudo)
#   curl -sSL https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | sh -s -- --install-dir /usr/local/bin
#
#   # Install to custom directory
#   curl -sSL https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.sh | sh -s -- --install-dir ~/bin
#
# Environment Variables:
#   DAGU_INSTALL_DIR         Override the default installation directory
#

# Set up constants
RELEASES_URL="https://github.com/dagu-org/dagu/releases"
FILE_BASENAME="dagu"

# Default values
DAGU_INSTALL_DIR="${DAGU_INSTALL_DIR:-$HOME/.local/bin}"
WORKING_ROOT_DIR="/tmp"

# Parse CLI arguments
while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      shift
      VERSION="$1"
      ;;
    --install-dir)
      shift
      DAGU_INSTALL_DIR="$1"
      ;;
    --prefix) # For compatibility with some conventions
      shift
      DAGU_INSTALL_DIR="$1"
      ;;
    --working-dir)
      shift
      if [ -z "$1" ]; then
        echo "Error: --working-dir requires a path argument" >&2
        exit 1
      fi
      WORKING_ROOT_DIR="$1"
      ;;
    *)
      ;;
  esac
  shift
done

# Check dependencies
command -v curl >/dev/null 2>&1 || { echo "curl is not installed. Aborting." >&2; exit 1; }
command -v tar >/dev/null 2>&1 || { echo "tar is not installed. Aborting." >&2; exit 1; }

# Determine version
if [ -z "$VERSION" ]; then
  # Use GitHub API for faster version detection
  VERSION="$(curl -sfL "https://api.github.com/repos/dagu-org/dagu/releases/latest" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed 's/.*"\([^"]*\)".*/\1/')"
fi

if [ -z "$VERSION" ]; then
  echo "Failed to determine the Dagu version to install." >&2
  exit 1
fi

echo "Installing Dagu version: $VERSION"

# Determine system and architecture
SYSTEM="$(uname -s | awk '{print tolower($0)}')"
ARCHITECTURE="$(uname -m)"
case "$ARCHITECTURE" in
  x86_64) ARCHITECTURE="amd64" ;;
  aarch64) ARCHITECTURE="arm64" ;;
  armv7l) ARCHITECTURE="armv7" ;;
  armv6l) ARCHITECTURE="armv6" ;;
esac

# Prepare working directory for temporary files
mkdir -p "$WORKING_ROOT_DIR" || {
  echo "Failed to create working directory: $WORKING_ROOT_DIR" >&2
  exit 1
}
NORMALIZED_WORKING_ROOT_DIR="${WORKING_ROOT_DIR%/}"
if [ -z "$NORMALIZED_WORKING_ROOT_DIR" ]; then
  NORMALIZED_WORKING_ROOT_DIR="/"
fi

# Create temporary working directory
TMPDIR="$(mktemp -d "${NORMALIZED_WORKING_ROOT_DIR}/dagu-installer.XXXXXX")" || {
  echo "Failed to create temporary directory under: $WORKING_ROOT_DIR" >&2
  exit 1
}
TAR_FILE="${TMPDIR}/${FILE_BASENAME}_${SYSTEM}_${ARCHITECTURE}.tar.gz"

# Build download URL
DOWNLOAD_URL="${RELEASES_URL}/download/${VERSION}/${FILE_BASENAME}_${VERSION#v}_${SYSTEM}_${ARCHITECTURE}.tar.gz"

# Download tarball
echo "Downloading: $DOWNLOAD_URL"
curl -sfLo "$TAR_FILE" "$DOWNLOAD_URL" || {
  echo "Failed to download the release archive." >&2
  exit 1
}

# Extract archive
tar -xf "$TAR_FILE" -C "$TMPDIR" || {
  echo "Failed to extract the archive." >&2
  exit 1
}

# Ensure installation directory exists
mkdir -p "$DAGU_INSTALL_DIR" || {
  echo "Failed to create installation directory: $DAGU_INSTALL_DIR" >&2
  exit 1
}

# Move binary to destination
INSTALL_PATH="${DAGU_INSTALL_DIR}/dagu"
if [ -w "$DAGU_INSTALL_DIR" ]; then
  mv "${TMPDIR}/dagu" "$INSTALL_PATH"
else
  echo "$DAGU_INSTALL_DIR is not writable. Using sudo to install."
  sudo mv "${TMPDIR}/dagu" "$INSTALL_PATH"
fi

# Make binary executable
chmod +x "$INSTALL_PATH" || {
  echo "Failed to set executable permission on: $INSTALL_PATH" >&2
  exit 1
}

# Clean up
rm -rf "$TMPDIR"

echo "Dagu $VERSION has been installed to: $INSTALL_PATH"

# Check if install directory is in PATH and provide guidance if not
if ! echo "$PATH" | tr ':' '\n' | grep -q "^$DAGU_INSTALL_DIR$"; then
  echo ""
  echo "Warning: $DAGU_INSTALL_DIR is not in your PATH."
  echo "Add the following line to your shell profile (~/.zshrc, ~/.bashrc, or ~/.bash_profile):"
  echo ""
  echo "  export PATH=\"\$PATH:$DAGU_INSTALL_DIR\""
  echo ""
fi
