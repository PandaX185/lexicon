SHELL := /bin/bash

GO ?= go
NPM ?= npm
PREFIX ?= $(HOME)/.local

# Build the linter with the active Go toolchain. Prebuilt golangci-lint
# binaries built with an older Go cannot read current export data.
GOLANGCI_LINT_VERSION ?= v2.15.0
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

BIN_DIR := bin
CLI_BIN := $(BIN_DIR)/lex
CLI_ALIAS_BIN := $(BIN_DIR)/lexicon
DAEMON_BIN := $(BIN_DIR)/lexicon-daemon
INSTALLER_BIN := $(BIN_DIR)/lexicon-installer
POPUP_BIN := $(BIN_DIR)/lexicon-popup
CORE_SOURCES := $(shell find cmd internal -name '*.go' -type f) go.mod go.sum

# Optional daemon build tags. Use DAEMON_GO_TAGS=bergamot together with the
# Bergamot CGO_*FLAGS described in docs/TRANSLATION.md.
DAEMON_TAGS_FLAG := $(if $(strip $(DAEMON_GO_TAGS)),-tags $(DAEMON_GO_TAGS))

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/PandaX185/lexicon/internal/version.Version=$(VERSION) \
	-X github.com/PandaX185/lexicon/internal/version.Commit=$(COMMIT) \
	-X github.com/PandaX185/lexicon/internal/version.BuildDate=$(BUILD_DATE)

# Wails defaults to GTK4/WebKitGTK 6.0. Systems that only ship WebKitGTK 4.1
# can build the same popup with the gtk3 tag. Override POPUP_GO_TAGS to force
# a different combination (for example "gtk3 purego").
HAS_WEBKITGTK6 := $(shell pkg-config --exists webkitgtk-6.0 2>/dev/null && echo yes)
HAS_WEBKIT2GTK41 := $(shell pkg-config --exists webkit2gtk-4.1 gtk+-3.0 2>/dev/null && echo yes)
ifeq ($(origin POPUP_GO_TAGS), undefined)
  ifeq ($(HAS_WEBKITGTK6),yes)
    POPUP_GO_TAGS :=
  else ifeq ($(HAS_WEBKIT2GTK41),yes)
    POPUP_GO_TAGS := gtk3
  endif
endif
POPUP_TAGS_FLAG := $(if $(strip $(POPUP_GO_TAGS)),-tags $(POPUP_GO_TAGS))

.PHONY: all help build core popup-deps popup-frontend popup-build popup-test \
	test test-core test-popup lint lint-core lint-popup lint-frontend \
	fmt fmt-check clean distclean install install-core install-popup

all: build

help:
	@echo "Targets:"
	@echo "  build            CLI, daemon, installer, and popup (popup needs WebKitGTK)"
	@echo "  core             CLI, daemon, and installer only"
	@echo "  popup-frontend   install npm dependencies and build the frontend"
	@echo "  popup-build      build the Wails popup (auto-selects gtk3 when needed)"
	@echo "  test             Go tests for core and popup"
	@echo "  lint             golangci-lint and TypeScript checks"
	@echo "  fmt / fmt-check  format or verify Go formatting"
	@echo "  install          install binaries under PREFIX (default: ~/.local)"
	@echo ""
	@echo "Variables:"
	@echo "  DAEMON_GO_TAGS   extra daemon build tags (for example: bergamot)"

build: core popup-build

core: $(CLI_BIN) $(CLI_ALIAS_BIN) $(DAEMON_BIN) $(INSTALLER_BIN)

$(CLI_BIN): $(CORE_SOURCES)
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $@ ./cmd/lex

$(CLI_ALIAS_BIN): $(CLI_BIN)
	@mkdir -p $(BIN_DIR)
	ln -sfn lex $@

$(DAEMON_BIN): $(CORE_SOURCES)
	@mkdir -p $(BIN_DIR)
	$(GO) build $(DAEMON_TAGS_FLAG) -trimpath -ldflags '$(LDFLAGS)' -o $@ ./cmd/lexicon-daemon

$(INSTALLER_BIN): $(CORE_SOURCES)
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $@ ./cmd/lexicon-installer

popup-deps:
ifeq ($(wildcard popup/frontend/package-lock.json),)
	cd popup/frontend && $(NPM) install
else
	cd popup/frontend && $(NPM) ci
endif

popup-frontend: popup-deps
	cd popup/frontend && $(NPM) run build

popup-build: popup-frontend
	@if [ -z "$(HAS_WEBKITGTK6)$(HAS_WEBKIT2GTK41)" ]; then \
		echo "error: install WebKitGTK 6.0 (GTK4) or WebKitGTK 4.1 + GTK3 development packages"; \
		exit 1; \
	fi
	@mkdir -p $(BIN_DIR)
	cd popup && $(GO) build $(POPUP_TAGS_FLAG) -trimpath -ldflags '$(LDFLAGS)' -o ../$(POPUP_BIN) .

test: test-core test-popup

test-core:
	$(GO) test ./...

test-popup:
	cd popup && $(GO) test $(POPUP_TAGS_FLAG) ./...

lint: lint-core lint-popup lint-frontend

lint-core:
	$(GOLANGCI_LINT) run

lint-popup:
	cd popup && $(GOLANGCI_LINT) run $(if $(strip $(POPUP_GO_TAGS)),--build-tags "$(POPUP_GO_TAGS)") ./...

lint-frontend: popup-deps
	cd popup/frontend && $(NPM) run typecheck

fmt:
	gofmt -w ./cmd ./internal
	cd popup && gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l cmd internal popup)" || { \
		echo "gofmt needs to run on:"; gofmt -l cmd internal popup; exit 1; }

clean:
	rm -rf $(BIN_DIR) popup/build/bin

distclean: clean
	rm -rf popup/frontend/node_modules popup/frontend/dist
	mkdir -p popup/frontend/dist
	touch popup/frontend/dist/.gitkeep

install: install-core install-popup

install-core: core
	install -Dm755 $(CLI_BIN) $(DESTDIR)$(PREFIX)/bin/lex
	ln -sfn lex $(DESTDIR)$(PREFIX)/bin/lexicon
	install -Dm755 $(DAEMON_BIN) $(DESTDIR)$(PREFIX)/bin/lexicon-daemon
	install -Dm755 $(INSTALLER_BIN) $(DESTDIR)$(PREFIX)/bin/lexicon-installer

install-popup: popup-build
	install -Dm755 $(POPUP_BIN) $(DESTDIR)$(PREFIX)/bin/lexicon-popup
