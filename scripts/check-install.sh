#!/usr/bin/env bash
# Verifies that `cmake --install` produced the expected tree.
set -euo pipefail

root="${1:-build/install-root}"
status=0

require_file() {
    if [[ -f "$root$1" ]]; then
        printf 'ok         %s\n' "$1"
    else
        printf 'missing    %s\n' "$1" >&2
        status=1
    fi
}

forbid_path() {
    if [[ -e "$root$1" ]]; then
        printf 'unexpected %s\n' "$1" >&2
        status=1
    fi
}

require_file /usr/bin/lexicon
require_file /usr/share/applications/io.github.PandaX185.Lexicon.desktop
require_file /usr/share/icons/hicolor/scalable/apps/lexicon.svg
require_file /usr/share/metainfo/io.github.PandaX185.Lexicon.metainfo.xml
require_file /usr/share/lexicon/config.example.toml
require_file /usr/share/lexicon/dictionaries/schema.sql
require_file /usr/share/lexicon/dictionaries/en.sqlite
require_file /usr/share/lexicon/scripts/argos_worker.py
require_file /usr/share/lexicon/lexicon-autostart.desktop
require_file /usr/share/doc/lexicon/README.md
require_file /usr/share/doc/lexicon/docs/configuration.md

# The autostart template must live in the data directory. A previous install rule
# resolved to /autostart and wrote files outside the intended tree.
forbid_path /autostart
forbid_path /usr/autostart

if [[ "$status" -ne 0 ]]; then
    printf '\nInstalled tree:\n' >&2
    find "$root" -type f | sort >&2 || true
    exit 1
fi

printf '\ninstall tree looks correct\n'
