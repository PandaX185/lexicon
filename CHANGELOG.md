# Changelog

All notable changes to Lexicon are documented here. The format follows Keep a Changelog
and the project uses semantic versioning.

## [Unreleased]

### Added

- Unit, integration, and QML smoke tests covering core types, configuration, dictionary
  lookup, controller orchestration, SQLite access, and QML loading.
- Header-only test fakes plus SQLite and scoped-environment fixtures under `tests/support/`.
- Coverage-instrumented builds (`coverage` preset and `just coverage`) with gcovr XML and
  HTML reports.
- Qt 6 `qmllint` target, clang-format check, and clang-tidy gate with warnings as errors.
- GCC, Clang, and ASan/UBSan CI presets, plus GitHub Actions jobs for all quality checks.
- Install-tree verification that catches files escaping the intended installation prefix.

### Changed

- Split the application into testable `lexicon_controller` and `lexicon_app` libraries.
- Added an explicit Qt Widgets dependency.
- The autostart template now installs under `${KDE_INSTALL_DATADIR}/lexicon` instead of an
  unintended `/autostart` path.
- Clarified that Escape works while the popup has focus and outside-click dismissal only
  applies inside the popup window.
- Documented `start_on_login` as accepted but deferred to Stage 3.
- `theme = "system"` now follows the platform color scheme and falls back to dark when the
  platform does not report one.

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
- `start_on_login` is accepted but does not install an autostart entry until Stage 3.
- The bundled dictionary data is a test fixture only.
- `cache_enabled` and `offline_first` are reserved for future stages and are not wired to behavior yet.
- Coverage is reported but no minimum threshold is enforced yet.
