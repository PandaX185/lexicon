# Changelog

All notable changes to Lexicon are documented here. The format follows Keep a Changelog
and the project uses semantic versioning.

## [Unreleased]

### Added

- Bundled offline English dictionary built from Princeton WordNet 3.0 (147,806
  headwords, 206,918 senses) with pronunciations converted from the CMU Pronouncing
  Dictionary; the importer and licenses ship with the source, and the database is
  installed to the dictionaries directory and treated as read-only.
- IPA pronunciations: `dictionary.pronunciation_style = "ipa"` shows IPA with a fallback
  to the respelling field, and the popup renders whichever the presenter selected.
- Optional offline translation through a local Argos Translate worker subprocess. Lookups
  with different `source_language` and `target_language` gain a translation card, and a
  missing runtime or pack only omits the card instead of failing the lookup.
- Language pack management in the settings window: a static catalog
  (`translation.catalog_url`), explicit install and remove actions, download progress,
  and SHA-256 verification before the runtime ever sees an archive.
- Dictionary catalog: every `*.sqlite` file in the dictionary directory is inspected, and
  the configured language pair selects which one answers a lookup. Language pairs live in
  the new `dictionary_info` table, so a new language is installed content rather than code.
- `entries.transliteration` indexes a Latin-script romanization of non-Latin headwords.
  Lookups fall back to it after the headword, and the popup shows it under the word.
- `--list-dictionaries` prints the installed dictionaries with their language pair, entry
  count, and path.
- New `dictionary.directory`, `dictionary.source_language`, and `dictionary.target_language`
  configuration keys.
- Automatic primary-selection retrieval on Wayland through
  `ext_data_control_manager_v1`, behind the platform-neutral `SelectionProvider` and
  `PrimarySelectionSource` interfaces.
- `SelectionTextPolicy` for MIME preference, UTF-8 validation, trimming, and a
  128-character limit, with distinct `SelectionUnsupported`, `SelectionNotText`, and
  `SelectionTooLong` errors.
- Optional `src/platform` backend layer: the Wayland backend is built when its
  dependencies exist and can be disabled with `-DLEXICON_WITH_WAYLAND=OFF`, and the
  application still runs with an explicit unsupported error.
- Vendor the `ext-data-control-v1` protocol XML and generate bindings at build time.
- Live `kde`-labelled selection tests using `wl-copy`, plus unit tests for the provider,
  policy, and backend factory.
- Unit, integration, and QML smoke tests covering core types, configuration, dictionary
  lookup, controller orchestration, SQLite access, and QML loading.
- Header-only test fakes plus SQLite and scoped-environment fixtures under `tests/support/`.
- Coverage-instrumented builds (`coverage` preset and `just coverage`) with gcovr XML and
  HTML reports.
- Qt 6 `qmllint` target, clang-format check, and clang-tidy gate with warnings as errors.
- GCC, Clang, and ASan/UBSan CI presets, plus GitHub Actions jobs for all quality checks.
- Install-tree verification that catches files escaping the intended installation prefix.
- Editable settings window backed by `SettingsController`: values are written to
  `config.toml` atomically, and `config.toml.bak` preserves comments or unknown keys the
  rewrite would otherwise drop.
- Opt-in per-user autostart management for `start_on_login`, with an absolute `Exec`
  path and a Flatpak-aware `flatpak run …` entry.
- First-run dictionary seeding that copies the installed database into the user data
  directory without overwriting existing content.
- Bounded lookup cache wired to `performance.cache_enabled` and
  `performance.cache_size`.
- HTTPS is required for the language pack catalog and downloads; insecure catalog
  entries are ignored and insecure downloads are rejected.
- Arch `PKGBUILD`, Flatpak manifest, packaging guide, and tagged release workflow.

### Changed

- The popup shows `pronunciation.text`, so the pronunciation style is chosen by the
  presenter rather than toggled inside the view.
- The dictionary schema is now version 2 with `dictionary_info` and
  `entries.transliteration`. Databases without metadata are ignored by the catalog.
- A missing dictionary now reports the requested language pair instead of only the path.

- Replaced the `KdeSelectionProvider` stub with the layered selection design; the KDE
  directory now holds only KDE Frameworks adapters.
- Fixed the CTest presets so `kde`-labelled tests are actually excluded from the default
  presets (the filter used an unsupported `labels` array instead of `label`).
- Split the application into testable `lexicon_controller` and `lexicon_app` libraries.
- Added an explicit Qt Widgets dependency.
- The autostart template now installs under `${KDE_INSTALL_DATADIR}/lexicon` instead of an
  unintended `/autostart` path.
- Clarified that Escape works while the popup has focus and outside-click dismissal only
  applies inside the popup window.
- `theme = "system"` now follows the platform color scheme and falls back to dark when the
  platform does not report one.
- Unit tests now cover settings persistence, autostart, first-run seeding, the lookup
  cache, and the HTTPS policy; QML tests receive the real `SettingsController`.
- The Flatpak manifest renames the icon to the application ID, so the desktop entry and
  exported icon resolve inside the sandbox.

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
