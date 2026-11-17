# Contributing

## Ground rules

- The C++/Qt implementation in the sibling `lexicon/` directory is reference
  material. Do not modify it as part of this project.
- Keep the daemon and CLI free of WebKit/Wails dependencies. The popup is a
  separate Go module under `popup/`.
- The CLI is the only configuration interface. Do not add settings screens or a
  second configuration format.
- Do not add global shortcut registration, AT-SPI, clipboard simulation, OCR, or
  application-specific integrations.
- `plan/` is local tracking and is git-ignored. Never commit it.
- Do not commit dictionaries, models, build output, `node_modules/`, or popup
  `dist/` contents other than `.gitkeep`.

## Development setup

Requirements:

- Go 1.27 or newer
- Node.js 22 or newer and npm
- `wl-clipboard` (`wl-paste`) for real selection tests
- A C toolchain and `pkg-config`
- For the popup: GTK 4 + WebKitGTK 6.0 **or** GTK 3 + WebKitGTK 4.1 development
  packages

The Wails v3 CLI is not required to build this repository; the Go module and
Makefile drive the build. Install it only if you want Wails' own dev tooling:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
```

## Checks

Core checks (no WebKit required):

```sh
make fmt          # gofmt for core and popup
make fmt-check    # fail if formatting is needed
make test-core    # go test ./...
make lint-core    # golangci-lint
make core         # lex CLI + daemon + installer
```

Full checks (popup and frontend):

```sh
make popup-deps     # npm ci, or npm install when no lockfile exists
make test-popup
make lint-popup
make lint-frontend  # tsc --noEmit
make build          # frontend + popup + core
```

`make test` and `make lint` run both sides. `make lint` builds golangci-lint
with the active Go toolchain via:

```sh
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
```

A prebuilt linter built with an older Go cannot read current export data.
Override `GOLANGCI_LINT` or `GOLANGCI_LINT_VERSION` if needed.

### Popup on GTK 3 systems

The Makefile prefers WebKitGTK 6.0/GTK 4 and automatically falls back to
`-tags gtk3` when only WebKitGTK 4.1 + GTK 3 are available. To force it:

```sh
make POPUP_GO_TAGS=gtk3 popup-build
make POPUP_GO_TAGS=gtk3 test-popup
make POPUP_GO_TAGS=gtk3 lint-popup
```

The frontend is embedded from `popup/frontend/dist`. Build it before testing or
building the popup; the Makefile targets do this. `popup/frontend/dist` is
ignored except for `.gitkeep`.

### Native Bergamot build

The optional `bergamot` tag is not covered by the default checks. It needs CGO,
a C++17 compiler, and a Bergamot installation. See
[docs/TRANSLATION.md](docs/TRANSLATION.md). Do not make the default suite depend
on it.

## Code style

- Go: `gofmt`, small packages, explicit errors, `slog` for structured logging.
  Run `make fmt`; do not hand-format.
- TypeScript: strict mode, function components, no `any`.
- SQL: bound parameters only; dictionary access is read-only.
- Keep the CLI separate from dictionary logic; call shared packages rather than
  duplicating behavior.
- Comments explain *why*, not *what*.
- New configuration keys need defaults, validation, a docs table entry, and a
  test for the validation boundary.

## Tests

- Unit-test behavior that contains a decision: normalization, validation,
  protocol framing, selection limits, schema degradation, path resolution.
- Use temporary directories and small SQLite fixtures. Never depend on a full
  dictionary, a translation model, or a live Wayland session in the default
  suite.
- Use injected runners/streams instead of invoking `wl-paste` or `systemctl`
  from tests.
- Label or skip tests that need native or session dependencies instead of
  making the default suite fail.
- The popup's IPC behavior is tested through the core `internal/ipc` package;
  keep the Wails layer thin.

## Documentation

Behavior changes belong with documentation updates:

- `README.md` for user-facing scope and quick start
- `docs/CONFIGURATION.md` for every configuration key
- `docs/IPC.md` when message types or semantics change
- `docs/DICTIONARY.md` for schema or lookup behavior
- `docs/TRANSLATION.md` for model packs or the native boundary
- `docs/INSTALL.md` for build, systemd, or troubleshooting changes

## Commits and pull requests

- One logical change per commit; include tests and docs with behavior changes.
- Describe user-visible behavior, compatibility impact, and verification
  commands in the pull request.
- Do not force-push shared branches or rewrite published history.
- Never commit secrets, `.env` files, keys, tokens, or captured selection text.

## License

By contributing you agree that your contribution is licensed under the MIT
License in `LICENSE`.
