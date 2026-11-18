set shell := ["bash", "-uc"]

go := env("GO", "go")
npm := env("NPM", "npm")
prefix := env("PREFIX", home_directory() / ".local")
destdir := env("DESTDIR", "")
release_dir := env("RELEASE_DIR", "build/release")

daemon_go_tags := env("DAEMON_GO_TAGS", "")
golangci_lint_version := env("GOLANGCI_LINT_VERSION", "v2.15.0")
golangci_lint := env("GOLANGCI_LINT", go + " run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + golangci_lint_version)

# Only application tags (v*) describe the app version; the dictionary is
# versioned separately and tagged dictionary-<version>.
version := env("VERSION", shell("git describe --tags --match 'v*' --always --dirty 2>/dev/null || printf 0.1.0-dev"))
commit := env("COMMIT", shell("git rev-parse --short HEAD 2>/dev/null || printf unknown"))
build_date := env("BUILD_DATE", shell("date -u +%Y-%m-%dT%H:%M:%SZ"))
ldflags := "-s -w -X github.com/PandaX185/lexicon/internal/version.Version=" + version + " -X github.com/PandaX185/lexicon/internal/version.Commit=" + commit + " -X github.com/PandaX185/lexicon/internal/version.BuildDate=" + build_date

# Wails defaults to GTK4/WebKitGTK 6.0. Systems that only ship WebKitGTK 4.1
# can build the same popup with the gtk3 tag. Override POPUP_GO_TAGS to force
# a different combination (for example "gtk3 purego").
webkitgtk6 := shell("pkg-config --exists webkitgtk-6.0 2>/dev/null && echo yes || true")
webkit2gtk41 := shell("pkg-config --exists webkit2gtk-4.1 gtk+-3.0 2>/dev/null && echo yes || true")
popup_go_tags := env("POPUP_GO_TAGS", if webkitgtk6 == "yes" { "" } else if webkit2gtk41 == "yes" { "gtk3" } else { "" })

daemon_tags_flag := if daemon_go_tags == "" { "" } else { "-tags " + daemon_go_tags }
popup_tags_flag := if popup_go_tags == "" { "" } else { "-tags " + popup_go_tags }
popup_lint_tags := if popup_go_tags == "" { "" } else { "--build-tags '" + popup_go_tags + "'" }

alias run := run-cli

# Build the CLI, daemon, installer, and popup.
default: build

# List available recipes.
help:
    @just --list

# Build every binary, including the popup.
build: core build-popup

# Build the lexicon CLI, daemon, and installer without WebKit.
core: build-cli build-daemon build-installer

# Build the lexicon CLI.
build-cli:
    @mkdir -p bin
    {{ go }} build -trimpath -ldflags '{{ ldflags }}' -o bin/lexicon ./cmd/lexicon

# Build the systemd-owned daemon. Set daemon_go_tags=bergamot for translation.
build-daemon:
    @mkdir -p bin
    {{ go }} build {{ daemon_tags_flag }} -trimpath -ldflags '{{ ldflags }}' -o bin/lexicon-daemon ./cmd/lexicon-daemon

# Build the terminal installer.
build-installer:
    @mkdir -p bin
    {{ go }} build -trimpath -ldflags '{{ ldflags }}' -o bin/lexicon-installer ./cmd/lexicon-installer

# Install npm dependencies for the popup frontend.
popup-deps:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -f popup/frontend/package-lock.json ]]; then
        cd popup/frontend
        {{ npm }} ci
    else
        cd popup/frontend
        {{ npm }} install
    fi

# Build the Vite frontend embedded in the popup.
popup-frontend: popup-deps
    cd popup/frontend && {{ npm }} run build
    # Vite cleans dist/, which also removes this tracked placeholder.
    touch popup/frontend/dist/.gitkeep

# Build the Wails popup (auto-selects gtk3 when needed).
build-popup: popup-frontend
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -z "{{ webkitgtk6 }}{{ webkit2gtk41 }}" ]]; then
        echo "error: install WebKitGTK 6.0 (GTK4) or WebKitGTK 4.1 + GTK3 development packages" >&2
        exit 1
    fi
    mkdir -p bin
    cd popup
    {{ go }} build {{ popup_tags_flag }} -trimpath -ldflags '{{ ldflags }}' -o ../bin/lexicon-popup .

# Run Go tests and the bootstrap installer test.
test: test-core test-popup test-install

# Run core Go tests.
test-core:
    {{ go }} test ./...

# Run the install.sh integration test with a local fake release.
test-install:
    bash tests/install_sh_test.sh

# Build the pinned native Bergamot translator in Docker. Then source
# build/bergamot/env.sh and build the daemon with daemon_go_tags=bergamot.
bergamot-build:
    ./tools/bergamot/build.sh

# Run the opt-in Bergamot integration test against an installed model pack.
bergamot-test pack:
    #!/usr/bin/env bash
    set -euo pipefail
    source build/bergamot/env.sh
    pack="$(realpath "{{ pack }}")"
    LEXICON_BERGAMOT_TEST_PACK="$pack" {{ go }} test -tags bergamot -run TestBergamot -v ./internal/translation

# Assemble a checksummed release tarball from a prepared Bergamot build.
# Run `just bergamot-build` first; then `just release 0.1.2`.
release app_version:
    #!/usr/bin/env bash
    set -euo pipefail
    # Untracked files do not affect build outputs, so only tracked changes block a release.
    # RELEASE_ALLOW_DIRTY is for local artifact testing only; CI never sets it.
    if [[ "${RELEASE_ALLOW_DIRTY:-0}" != "1" ]] && \
       [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
        echo "error: release requires a clean working tree" >&2
        echo "       (set RELEASE_ALLOW_DIRTY=1 only for local artifact testing)" >&2
        exit 1
    fi
    if [[ ! -f build/bergamot/env.sh ]]; then
        echo "error: run 'just bergamot-build' first" >&2
        exit 1
    fi
    goarch="$({{ go }} env GOARCH)"
    case "$goarch" in
        amd64|arm64) ;;
        *) echo "error: unsupported GOARCH: $goarch" >&2; exit 1 ;;
    esac
    target="linux-$goarch"
    stage="{{ release_dir }}/lexicon-{{ app_version }}-$target"
    rm -rf "$stage"
    mkdir -p "$stage/bin" "$stage/share/lexicon" \
             "$stage/share/doc/lexicon" "$stage/share/licenses/lexicon"
    export VERSION="{{ app_version }}"
    export DAEMON_GO_TAGS=bergamot
    # shellcheck disable=SC1091
    source build/bergamot/env.sh
    just core
    just build-popup
    cp bin/lexicon bin/lexicon-daemon bin/lexicon-installer \
       bin/lexicon-popup "$stage/bin/"
    cp packaging/systemd/lexicon.service "$stage/share/lexicon/"
    cp docs/*.md "$stage/share/doc/lexicon/"
    cp LICENSE licenses/WORDNET.txt licenses/CMUDICT.txt \
       "$stage/share/licenses/lexicon/"
    (
        cd "$stage"
        find . -type f ! -name checksums.sha256 -print0 \
          | sort -z | xargs -0 sha256sum > checksums.sha256
    )
    tar -czf "{{ release_dir }}/lexicon-{{ app_version }}-$target.tar.gz" \
        -C "{{ release_dir }}" "$(basename "$stage")"
    echo "release: {{ release_dir }}/lexicon-{{ app_version }}-$target.tar.gz"

# Build the English dictionary from WordNet 3.0 and CMUdict. Both paths are
# build-time inputs; the generated asset is not tracked.
dict-build wordnet_dir cmudict_file output="build/dictionaries/en.sqlite":
    {{ go }} run ./tools/dictgen --wordnet-dir "{{ wordnet_dir }}" --cmudict-file "{{ cmudict_file }}" --output "{{ output }}"

# Build a complete dictionary release directory: asset, manifest, and licenses.
dict-release wordnet_dir cmudict_file output_dir="build/dictionaries":
    {{ go }} run ./tools/dictgen --wordnet-dir "{{ wordnet_dir }}" --cmudict-file "{{ cmudict_file }}" --output "{{ output_dir }}/en.sqlite" --manifest "{{ output_dir }}/manifest.json"
    cp licenses/WORDNET.txt "{{ output_dir }}/WORDNET.txt"
    cp licenses/CMUDICT.txt "{{ output_dir }}/CMUDICT.txt"

# Run popup Go tests.
test-popup:
    cd popup && {{ go }} test {{ popup_tags_flag }} ./...

# Run Go, popup, and frontend linters.
lint: lint-core lint-popup lint-frontend

# Run golangci-lint for the core.
lint-core:
    {{ golangci_lint }} run

# Run golangci-lint for the popup.
lint-popup:
    cd popup && {{ golangci_lint }} run {{ popup_lint_tags }} ./...

# Type-check the React frontend.
lint-frontend: popup-deps
    cd popup/frontend && {{ npm }} run typecheck

# Format Go sources.
fmt:
    gofmt -w cmd internal tools
    cd popup && gofmt -w .

# Fail if Go formatting is needed.
fmt-check:
    #!/usr/bin/env bash
    set -euo pipefail
    files="$(gofmt -l cmd internal tools popup)"
    if [[ -n "$files" ]]; then
        echo "gofmt needs to run on:" >&2
        echo "$files" >&2
        exit 1
    fi

# Remove build outputs.
clean:
    rm -rf bin popup/build/bin

# Remove build outputs, dependencies, and generated frontend assets.
distclean: clean
    rm -rf popup/frontend/node_modules popup/frontend/dist
    mkdir -p popup/frontend/dist
    touch popup/frontend/dist/.gitkeep

# Install the CLI, daemon, installer, and popup under prefix.
install: install-core install-popup

# Install the CLI, daemon, and installer under prefix.
install-core: core
    install -Dm755 bin/lexicon "{{ destdir }}{{ prefix }}/bin/lexicon"
    install -Dm755 bin/lexicon-daemon "{{ destdir }}{{ prefix }}/bin/lexicon-daemon"
    install -Dm755 bin/lexicon-installer "{{ destdir }}{{ prefix }}/bin/lexicon-installer"

# Install the popup under prefix.
install-popup: build-popup
    install -Dm755 bin/lexicon-popup "{{ destdir }}{{ prefix }}/bin/lexicon-popup"

# Build and run the lexicon CLI. Pass lexicon arguments after the recipe name.
run-cli *args: build-cli
    ./bin/lexicon {{ args }}

# Build and run the daemon in the foreground.
run-daemon *args: build-daemon
    ./bin/lexicon-daemon {{ args }}

# Run the daemon in the foreground with the sample config and an isolated
# runtime directory, so it can run beside an installed service.
run-daemon-dev *args: build-daemon
    #!/usr/bin/env bash
    set -euo pipefail
    base="${XDG_RUNTIME_DIR:-${TMPDIR:-/tmp}}"
    runtime="$base/lexicon-dev-$(id -u)"
    mkdir -p "$runtime"
    chmod 700 "$runtime"
    echo "dev runtime: $runtime" >&2
    exec env XDG_RUNTIME_DIR="$runtime" ./bin/lexicon-daemon --config config.example.toml --log-level debug {{ args }}

# Build and run the terminal installer.
run-installer *args: build-installer
    ./bin/lexicon-installer {{ args }}

# Build and run the popup.
run-popup *args: build-popup
    ./bin/lexicon-popup {{ args }}

# Build and run a lookup through the CLI. Use `just lookup --json`.
lookup *args: build-cli
    ./bin/lexicon lookup {{ args }}

# Build and show the doctor report.
doctor: build-cli
    ./bin/lexicon doctor

# Build and run a config subcommand. Use `just config show`.
config *args: build-cli
    ./bin/lexicon config {{ args }}
