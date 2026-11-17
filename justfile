set shell := ["bash", "-uc"]

go := env_var_or_default("GO", "go")
npm := env_var_or_default("NPM", "npm")
prefix := env_var_or_default("PREFIX", home_directory() / ".local")
destdir := env_var_or_default("DESTDIR", "")

daemon_go_tags := env_var_or_default("DAEMON_GO_TAGS", "")
golangci_lint_version := env_var_or_default("GOLANGCI_LINT_VERSION", "v2.15.0")
golangci_lint := env_var_or_default("GOLANGCI_LINT", go + " run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + golangci_lint_version)

version := env_var_or_default("VERSION", shell("git describe --tags --always --dirty 2>/dev/null || printf 0.1.0-dev"))
commit := env_var_or_default("COMMIT", shell("git rev-parse --short HEAD 2>/dev/null || printf unknown"))
build_date := env_var_or_default("BUILD_DATE", shell("date -u +%Y-%m-%dT%H:%M:%SZ"))
ldflags := "-s -w -X github.com/PandaX185/lexicon/internal/version.Version=" + version + " -X github.com/PandaX185/lexicon/internal/version.Commit=" + commit + " -X github.com/PandaX185/lexicon/internal/version.BuildDate=" + build_date

# Wails defaults to GTK4/WebKitGTK 6.0. Systems that only ship WebKitGTK 4.1
# can build the same popup with the gtk3 tag. Override POPUP_GO_TAGS to force
# a different combination (for example "gtk3 purego").
webkitgtk6 := shell("pkg-config --exists webkitgtk-6.0 2>/dev/null && echo yes || true")
webkit2gtk41 := shell("pkg-config --exists webkit2gtk-4.1 gtk+-3.0 2>/dev/null && echo yes || true")
popup_go_tags := env_var_or_default("POPUP_GO_TAGS", if webkitgtk6 == "yes" { "" } else if webkit2gtk41 == "yes" { "gtk3" } else { "" })

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

# Build the lex CLI, daemon, and installer without WebKit.
core: build-cli build-daemon build-installer

# Build the lex CLI and the lexicon compatibility symlink.
build-cli:
    @mkdir -p bin
    {{ go }} build -trimpath -ldflags '{{ ldflags }}' -o bin/lex ./cmd/lex
    ln -sfn lex bin/lexicon

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

# Run Go tests for the core and popup.
test: test-core test-popup

# Run core Go tests.
test-core:
    {{ go }} test ./...

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
    gofmt -w cmd internal
    cd popup && gofmt -w .

# Fail if Go formatting is needed.
fmt-check:
    #!/usr/bin/env bash
    set -euo pipefail
    files="$(gofmt -l cmd internal popup)"
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

# Install the CLI, alias, daemon, and installer under prefix.
install-core: core
    install -Dm755 bin/lex "{{ destdir }}{{ prefix }}/bin/lex"
    ln -sfn lex "{{ destdir }}{{ prefix }}/bin/lexicon"
    install -Dm755 bin/lexicon-daemon "{{ destdir }}{{ prefix }}/bin/lexicon-daemon"
    install -Dm755 bin/lexicon-installer "{{ destdir }}{{ prefix }}/bin/lexicon-installer"

# Install the popup under prefix.
install-popup: build-popup
    install -Dm755 bin/lexicon-popup "{{ destdir }}{{ prefix }}/bin/lexicon-popup"

# Build and run the lex CLI. Pass lex arguments after the recipe name.
run-cli *args: build-cli
    ./bin/lex {{ args }}

# Build and run the daemon in the foreground.
run-daemon *args: build-daemon
    ./bin/lexicon-daemon {{ args }}

# Build and run the terminal installer.
run-installer *args: build-installer
    ./bin/lexicon-installer {{ args }}

# Build and run the popup.
run-popup *args: build-popup
    ./bin/lexicon-popup {{ args }}

# Build and run a lookup through the CLI. Use `just lookup --json`.
lookup *args: build-cli
    ./bin/lex lookup {{ args }}

# Build and show the doctor report.
doctor: build-cli
    ./bin/lex doctor

# Build and run a config subcommand. Use `just config show`.
config *args: build-cli
    ./bin/lex config {{ args }}
