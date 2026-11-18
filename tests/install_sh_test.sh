#!/usr/bin/env bash
# Integration test for the bootstrap installer. Builds a fake release with
# file:// URLs so no network access is needed.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

version="9.9.9"
case "$(uname -m)" in
	x86_64|amd64) arch=amd64 ;;
	aarch64|arm64) arch=arm64 ;;
	*) printf 'unsupported architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac
archive="lexicon-${version}-linux-${arch}.tar.gz"
stage_name="lexicon-${version}-linux-${arch}"

releases="$tmp/releases/download"
mkdir -p "$releases/v$version" "$tmp/stage/$stage_name/bin" "$tmp/dictionary" "$tmp/bin"

for binary in lexicon lexicon-daemon lexicon-popup; do
	printf '#!/bin/sh\nprintf "%%s\\n" "%s"\n' "$binary" > "$tmp/stage/$stage_name/bin/$binary"
done
cat > "$tmp/stage/$stage_name/bin/lexicon-installer" <<'EOF'
#!/bin/sh
printf '%s\n' "$0" >> "$FAKE_INSTALLER_LOG"
printf '%s\n' "$@" >> "$FAKE_INSTALLER_LOG"
EOF
chmod 755 "$tmp/stage/$stage_name/bin/"*

(
	cd "$tmp/stage/$stage_name"
	find . -type f ! -name checksums.sha256 -print0 | sort -z | xargs -0 sha256sum > checksums.sha256
)
tar -czf "$releases/v$version/$archive" -C "$tmp/stage" "$stage_name"
( cd "$releases/v$version" && sha256sum "$archive" > checksums.sha256 )

printf 'dictionary fixture\n' > "$tmp/dictionary/en.sqlite"
printf 'manifest fixture\n' > "$tmp/dictionary/manifest.json"
dict_sha="$(sha256sum "$tmp/dictionary/en.sqlite" | awk '{print $1}')"
manifest_sha="$(sha256sum "$tmp/dictionary/manifest.json" | awk '{print $1}')"

LEXICON_INSTALL_VERSION="$version" \
LEXICON_INSTALL_BASE_URL="file://$releases" \
LEXICON_DICTIONARY_BASE_URL="file://$tmp/dictionary" \
LEXICON_INSTALL_DICT_SHA="$dict_sha" \
LEXICON_INSTALL_MANIFEST_SHA="$manifest_sha" \
LEXICON_BIN_DIR="$tmp/bin" \
FAKE_INSTALLER_LOG="$tmp/installer.log" \
	sh "$root/install.sh"

for binary in lexicon lexicon-daemon lexicon-installer lexicon-popup; do
	if [[ ! -x "$tmp/bin/$binary" ]]; then
		printf 'FAIL: %s was not installed to the bin directory\n' "$binary" >&2
		exit 1
	fi
done

if [[ ! -f "$tmp/installer.log" ]]; then
	printf 'FAIL: installer was not launched\n' >&2
	exit 1
fi
if ! grep -qx -- "$tmp/bin/lexicon-installer" "$tmp/installer.log"; then
	printf 'FAIL: installer was not launched from the bin directory\n' >&2
	cat "$tmp/installer.log" >&2
	exit 1
fi
if ! grep -qx -- '--dictionary' "$tmp/installer.log"; then
	printf 'FAIL: installer did not receive the dictionary flag\n' >&2
	cat "$tmp/installer.log" >&2
	exit 1
fi
if ! grep -qx -- '--daemon' "$tmp/installer.log" ||
	! grep -qx -- "$tmp/bin/lexicon-daemon" "$tmp/installer.log"; then
	printf 'FAIL: installer did not receive the installed daemon path\n' >&2
	cat "$tmp/installer.log" >&2
	exit 1
fi

printf 'install.sh integration test passed\n'
