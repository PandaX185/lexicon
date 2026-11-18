#!/bin/sh
set -eu
umask 077

VERSION="${LEXICON_INSTALL_VERSION:-0.1.0}"
RELEASE_BASE="${LEXICON_INSTALL_BASE_URL:-https://github.com/PandaX185/lexicon/releases/download}"
DICT_BASE="${LEXICON_DICTIONARY_BASE_URL:-$RELEASE_BASE/dictionary-3.0}"
SKIP_EXEC="${LEXICON_INSTALL_SKIP_EXEC:-0}"
KEEP_WORK="${LEXICON_INSTALL_KEEP:-0}"

DICT_SHA='4abacd0b88267af00bff88a731644bb3cc6a3c8f579729c2b69a405fdc03b458'
MANIFEST_SHA='7cf561d001f109101305d475f5d5b71bc48310c1668babd935aabeb4f3e2ef83'

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

exec "$STAGE/bin/lexicon-installer" --dictionary "$WORK/en.sqlite"
