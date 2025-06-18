#!/bin/sh

# Set up constants
RELEASES_URL="https://github.com/dagu-org/dagu/releases"
FILE_BASENAME="dagu"

# Default values
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Parse CLI arguments
while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      shift
      VERSION="$1"
      ;;
    --install-dir)
      shift
      INSTALL_DIR="$1"
      ;;
    --prefix) # For compatibility with some conventions
      shift
      INSTALL_DIR="$1"
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
  VERSION="$(curl -sfL -o /dev/null -w '%{url_effective}' "$RELEASES_URL/latest" | rev | cut -d'/' -f1 | rev)"
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
esac

# Create temporary working directory
TMPDIR="$(mktemp -d)"
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
mkdir -p "$INSTALL_DIR" || {
  echo "Failed to create installation directory: $INSTALL_DIR" >&2
  exit 1
}

# Move binary to destination
INSTALL_PATH="${INSTALL_DIR}/dagu"
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/dagu" "$INSTALL_PATH"
else
  echo "$INSTALL_DIR is not writable. Using sudo to install."
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
echo "Ensure that $INSTALL_DIR is included in your PATH."
