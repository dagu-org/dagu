#!/bin/sh

RELEASES_URL="https://github.com/dagu-dev/dagu/releases"
FILE_BASENAME="dagu"

echo "Downloading the latest binary to the current directory..."

test -z "$VERSION" && VERSION="$(curl -sfL -o /dev/null -w %{url_effective} "$RELEASES_URL/latest" |
		rev |
		cut -f1 -d'/'|
		rev)"

test -z "$VERSION" && {
	echo "Unable to get Dagu version." >&2
	exit 1
}

if [ "$( uname -m )" = "x86_64" ]
then
	ARCHITECTURE="amd64"
else
	ARCHITECTURE="$( uname -m )"
fi

test -z "$TMPDIR" && TMPDIR="$(mktemp -d)"
export TAR_FILE="${TMPDIR}${FILE_BASENAME}_$(uname -s)_$ARCHITECTURE.tar.gz"

(
	cd "$TMPDIR"
	echo "Downloading dagu $VERSION..."
	curl -sfLo "$TAR_FILE" "$RELEASES_URL/download/$VERSION/${FILE_BASENAME}_${VERSION:1}_$(uname -s)_$ARCHITECTURE.tar.gz"
)

tar -xf "$TAR_FILE" -C "$TMPDIR"
cp "${TMPDIR}/dagu" ./
"./dagu" "$@"
