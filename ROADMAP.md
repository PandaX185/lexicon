# Roadmap

## Stage 1: scaffold

- [x] CMake, Qt 6, KF6 skeleton
- [x] TOML configuration
- [x] SQLite schema and repository
- [x] Asynchronous lookup controller
- [x] Global shortcut
- [x] Tray integration
- [x] Hidden settings window
- [x] Lazy lookup popup
- [x] Unit and integration tests

## Stage 2: real selected-text lookup

- [ ] Wayland data-control proof of concept behind `SelectionProvider`
- [ ] Primary-selection MIME handling and length limits
- [ ] Explicit unavailable/unsupported/non-text errors
- [ ] X11 behavior decision
- [ ] KDE-labelled integration tests on a live Plasma session

## Stage 3: usable dictionary product

- [ ] Redistributable dictionary packaging and attribution
- [ ] Installer/first-run flow for dictionary content
- [ ] Editable settings backed by `config.toml`
- [ ] Opt-in per-user autostart management for `start_on_login`
- [ ] Lookup cache with bounded size
- [ ] Packaging for Arch, Flatpak, and a release artifact

## Stage 4: polish

- [ ] Keyboard navigation and accessibility review
- [ ] Multi-monitor and high-DPI popup placement
- [ ] Theming integration with Plasma
- [ ] Localization
- [ ] Crash reporting and privacy review

## Stage 5: testing and continuous integration

Implemented ahead of Stage 2 so later work lands on a reliable baseline.

- [x] Unit, integration, and QML smoke tests registered with CTest labels
- [x] Header-only fakes, SQLite fixture, and scoped environment helpers
- [x] GCC and Clang builds with warnings as errors
- [x] clang-format and clang-tidy gates
- [x] Qt 6 `qmllint` integration through a CMake target
- [x] Coverage-instrumented builds with gcovr XML and HTML reports
- [x] AddressSanitizer and UndefinedBehaviorSanitizer test runs
- [x] Install-tree verification, including the autostart template location
- [x] GitHub Actions pipeline for all of the above
- [ ] KDE-labelled tests against a live Plasma session (tracked in Stage 2)
- [ ] Enforced coverage thresholds in CI
