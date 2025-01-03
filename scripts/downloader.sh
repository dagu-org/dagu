#!/bin/sh

# Set up constants and URLs
RELEASES_URL="https://github.com/dagu-org/dagu/releases"
FILE_BASENAME="dagu"

echo "Downloading the latest binary to the current directory..."

# Check for curl and tar availability
command -v curl >/dev/null 2>&1 || { echo "curl is not installed. Aborting." >&2; exit 1; }
command -v tar >/dev/null 2>&1 || { echo "tar is not installed. Aborting." >&2; exit 1; }

# Retrieve the latest version if not specified
if [ -z "$VERSION" ]; then
    VERSION="$(curl -sfL -o /dev/null -w %{url_effective} "$RELEASES_URL/latest" | rev | cut -f1 -d'/' | rev)"
fi

# Exit if VERSION is still empty
if [ -z "$VERSION" ]; then
    echo "Unable to get Dagu version." >&2
    exit 1
fi

# Determine architecture
case "$(uname -m)" in
    x86_64)
        ARCHITECTURE="amd64"
        ;;
    aarch64)
        ARCHITECTURE="arm64"
        ;;
    *)
        ARCHITECTURE="$(uname -m)"
        ;;
esac

# Create a temporary directory for the download
TMPDIR=$(mktemp -d)
export TAR_FILE="${TMPDIR}/${FILE_BASENAME}_$(uname -s)_${ARCHITECTURE}.tar.gz"

# Download the binary
echo "Downloading Dagu $VERSION..."
curl -sfLo "$TAR_FILE" "$RELEASES_URL/download/$VERSION/${FILE_BASENAME}_${VERSION:1}_$(uname -s)_${ARCHITECTURE}.tar.gz" || {
    echo "Failed to download the file. Check your internet connection and the URL." >&2
    exit 1
}

# Unpack and install
tar -xf "$TAR_FILE" -C "$TMPDIR" && sudo mv "${TMPDIR}/dagu" /usr/local/bin/dagu && sudo chmod +x /usr/local/bin/dagu || {
    echo "Failed to install Dagu." >&2
    exit 1
}

# Cleanup
rm -rf "$TMPDIR"
echo "Dagu installed successfully and is available at /usr/local/bin/dagu"

# Execute the binary with any provided arguments
"/usr/local/bin/dagu" "$@"