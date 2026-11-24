#!/bin/sh
set -eu
umask 077

VERSION="${LEXICON_INSTALL_VERSION:-0.3.0}"
RELEASE_BASE="${LEXICON_INSTALL_BASE_URL:-https://github.com/PandaX185/lexicon/releases/download}"
DICT_BASE="${LEXICON_DICTIONARY_BASE_URL:-$RELEASE_BASE/dictionary-3.0}"
SKIP_EXEC="${LEXICON_INSTALL_SKIP_EXEC:-0}"
KEEP_WORK="${LEXICON_INSTALL_KEEP:-0}"

DICT_SHA="${LEXICON_INSTALL_DICT_SHA:-556973a4033d85d7ba7a778430064638129eed9f9e998ff7894b0eaf22778780}"
MANIFEST_SHA="${LEXICON_INSTALL_MANIFEST_SHA:-7f962840a2aea9d4e03258207441ab4110face5392ef6ac6bfb7ba37fc3f37a0}"
BIN_DIR="${LEXICON_BIN_DIR:-${XDG_BIN_HOME:-$HOME/.local/bin}}"

fail() {
	printf 'lexicon-install: %s\n' "$*" >&2
	exit 1
}

require_https() {
	case "$1" in
		https://*|file://*) return 0 ;;
		*) fail "refusing non-HTTPS download: $1" ;;
	esac
}

case "$(uname -s)" in
	Linux) ;;
	*) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
	x86_64|amd64) GOARCH=amd64 ;;
	aarch64|arm64) GOARCH=arm64 ;;
	*) fail "unsupported architecture: $(uname -m)" ;;
esac

ARCHIVE="lexicon-${VERSION}-linux-${GOARCH}.tar.gz"
RELEASE_URL="${RELEASE_BASE}/v${VERSION}/${ARCHIVE}"
CHECKSUMS_URL="${RELEASE_BASE}/v${VERSION}/checksums.sha256"
DICT_URL="${DICT_BASE}/en.sqlite"
MANIFEST_URL="${DICT_BASE}/manifest.json"

require_https "$RELEASE_URL"
require_https "$CHECKSUMS_URL"
require_https "$DICT_URL"
require_https "$MANIFEST_URL"

command -v curl >/dev/null 2>&1 || fail 'curl is required'
command -v sha256sum >/dev/null 2>&1 || fail 'sha256sum is required'
command -v tar >/dev/null 2>&1 || fail 'tar is required'
command -v install >/dev/null 2>&1 || fail 'install is required'

WORK="$(mktemp -d "${TMPDIR:-/tmp}/lexicon-install.XXXXXX")"
cleanup() {
	if [ "$KEEP_WORK" = "1" ]; then
		printf 'lexicon-install: kept workspace %s\n' "$WORK" >&2
		return
	fi
	rm -rf "$WORK"
}
trap cleanup EXIT HUP INT TERM

printf 'Downloading %s\n' "$RELEASE_URL"
curl -fsSL "$RELEASE_URL" -o "$WORK/$ARCHIVE" || fail "download failed: $RELEASE_URL"
curl -fsSL "$CHECKSUMS_URL" -o "$WORK/checksums.sha256" || fail "download failed: $CHECKSUMS_URL"
printf 'Downloading %s\n' "$DICT_URL"
curl -fsSL "$DICT_URL" -o "$WORK/en.sqlite" || fail "download failed: $DICT_URL"
printf 'Downloading %s\n' "$MANIFEST_URL"
curl -fsSL "$MANIFEST_URL" -o "$WORK/manifest.json" || fail "download failed: $MANIFEST_URL"

EXPECTED="$(grep "  ${ARCHIVE}\$" "$WORK/checksums.sha256" | awk '{print $1}')"
[ -n "$EXPECTED" ] || fail "no checksum for $ARCHIVE in $CHECKSUMS_URL"
ACTUAL="$(sha256sum "$WORK/$ARCHIVE" | awk '{print $1}')"
[ "$ACTUAL" = "$EXPECTED" ] || fail "checksum mismatch for $ARCHIVE: got $ACTUAL, want $EXPECTED"

printf '%s  %s\n' "$DICT_SHA" "$WORK/en.sqlite" > "$WORK/dictionary.sha256"
printf '%s  %s\n' "$MANIFEST_SHA" "$WORK/manifest.json" >> "$WORK/dictionary.sha256"
( cd "$WORK" && sha256sum -c dictionary.sha256 ) || fail 'dictionary checksum verification failed'

mkdir -p "$WORK/stage"
tar -xzf "$WORK/$ARCHIVE" -C "$WORK/stage" || fail "extract failed: $ARCHIVE"
STAGE="$WORK/stage/lexicon-${VERSION}-linux-${GOARCH}"
[ -x "$STAGE/bin/lexicon-installer" ] || fail "missing installer in $ARCHIVE"
( cd "$STAGE" && sha256sum -c checksums.sha256 ) || fail 'archive contents failed checksum verification'

if [ "$SKIP_EXEC" = "1" ]; then
	printf 'Staged installer at %s\n' "$STAGE/bin/lexicon-installer"
	exit 0
fi

printf 'Installing binaries to %s\n' "$BIN_DIR"
mkdir -p "$BIN_DIR" || fail "cannot create $BIN_DIR"
for binary in lexicon lexicon-daemon lexicon-installer lexicon-popup; do
	temp="$BIN_DIR/.$binary.$$"
	install -m 755 "$STAGE/bin/$binary" "$temp" || fail "install $binary"
	mv -f "$temp" "$BIN_DIR/$binary" || fail "install $binary"
done
case ":$PATH:" in
	*":$BIN_DIR:"*) ;;
	*) printf 'lexicon-install: add %s to your PATH to use the CLI\n' "$BIN_DIR" >&2 ;;
esac

exec "$BIN_DIR/lexicon-installer" \
	--dictionary "$WORK/en.sqlite" \
	--daemon "$BIN_DIR/lexicon-daemon"
