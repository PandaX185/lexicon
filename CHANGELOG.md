# Changelog

All notable changes to Lexicon are documented here. The format follows Keep a Changelog
and the project uses semantic versioning.

## [Unreleased]

## [0.1.0] - 2026-11-06

### Added

- Initial C++20/Qt 6/KF6 scaffold.
- TOML configuration with XDG defaults, validation, warnings, and clear fallback.
- SQLite dictionary schema, development fixture, and read-only repository.
- Asynchronous dictionary lookup service.
- KGlobalAccel shortcut registration and KStatusNotifierItem tray integration.
- Hidden settings window and lazy QML lookup popup.
- Command-line lookup with configuration and database overrides.
- Unit and integration tests.
- Architecture, configuration, and selection-retrieval documentation.

### Known limitations

- Selected-text retrieval on Wayland is not implemented.
- The settings window is read-only.
- The bundled dictionary data is a test fixture only.
- `theme = "system"` is accepted but currently behaves like `dark`; desktop theme probing is not implemented.
- `cache_enabled` and `offline_first` are reserved for future stages and are not wired to behavior yet.
