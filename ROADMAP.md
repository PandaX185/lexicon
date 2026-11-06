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
- [ ] Lookup cache with bounded size
- [ ] Packaging for Arch, Flatpak, and a release artifact

## Stage 4: polish

- [ ] Keyboard navigation and accessibility review
- [ ] Multi-monitor and high-DPI popup placement
- [ ] Theming integration with Plasma
- [ ] Localization
- [ ] Crash reporting and privacy review
