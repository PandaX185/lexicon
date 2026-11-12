# Lexicon

Lexicon is a small, native, offline-first English dictionary utility for KDE Plasma.
It starts quietly, stays available through a global shortcut and tray icon, and shows
definitions in a focused popup.

This repository currently implements **Stage 2**: automatic selected-text lookup on
Wayland, on top of the Stage 1 scaffold. It stays small, buildable, and honest about
what is not implemented yet.

## Status

| Area | State |
| --- | --- |
| C++20 / Qt 6 / KF6 build | Implemented |
| TOML configuration with XDG paths | Implemented |
| SQLite dictionary schema and lookup | Implemented |
| KGlobalAccel global shortcut | Implemented |
| KStatusNotifierItem tray integration | Implemented |
| QML settings window and lookup popup | Implemented |
| Command-line lookup | Implemented |
| Tests, formatting, linting, coverage, and CI | Implemented; see [contributing](CONTRIBUTING.md) |
| Automatic selected-text retrieval on Wayland | Implemented behind `SelectionProvider`; see [selection retrieval](docs/architecture/selection-retrieval.md) |
| Automatic selected-text retrieval on X11 | Explicit unsupported error; decision recorded in [selection retrieval](docs/architecture/selection-retrieval.md) |
| Bundled English dictionary | WordNet 3.0 definitions with CMUdict IPA; see [WordNet dictionary](docs/wordnet-dictionary.md) |
| IPA pronunciation | `dictionary.pronunciation_style = "ipa"`; see [IPA pronunciation](docs/pronunciation-ipa.md) |
| Offline translation | Optional Argos Translate packs, downloaded only on request; see [translation](docs/argos-translation.md) |
| Language pack management | Install and remove packs from the settings window; see [language packs](docs/language-packs.md) |
| Dictionary content distribution (other languages) | Not implemented beyond the bundled pair |
| Settings editing | Read-only in Stage 1; edit `config.toml` and restart |

## Requirements

- CMake 3.24 or newer
- A C++20 compiler
- Qt 6.10 or newer (Core, Gui, Widgets, Qml, Quick, QuickControls2, Sql, Test, DBus, Concurrent, Network)
- KDE Frameworks 6: KGlobalAccel, KStatusNotifierItem, KDBusAddons
- Extra CMake Modules (ECM)
- SQLite through the Qt SQL plugin
- toml++ 3.4 or newer, or the vendored copy in `third_party/`
- Optional: `wayland-client` and `wayland-scanner` for the Wayland selection backend
- Optional: `python3` and `argostranslate` for offline translation

On Arch-based systems the development packages are typically:

```sh
sudo pacman -S --needed base-devel cmake ninja extra-cmake-modules \
    qt6-base qt6-declarative qt6-svg \
    kglobalaccel kstatusnotifieritem kdbusaddons \
    wayland wayland-protocols
```

For the development checks, also install `clang`, `just`, and `gcovr`.

The build uses the vendored toml++ header when the system package is unavailable.

## Build and run

```sh
cmake --preset dev-debug
cmake --build --preset dev-debug
ctest --preset dev-debug
./build/dev-debug/lexicon
```

Or use the task runner:

```sh
just build
just test
just run
```

Lexicon starts without showing a window. Use the global shortcut (`SUPER+SHIFT+D` by
default), the tray icon, or a command-line lookup:

```sh
./build/dev-debug/lexicon --lookup serendipity
```

### Development database

The repository does not ship a production dictionary. A small CC0 fixture is provided
for smoke tests:

```sh
just seed-db
./build/dev-debug/lexicon --lookup serendipity --database build/dev/lexicon.sqlite
```

## Configuration

The default location is:

```text
$XDG_CONFIG_HOME/lexicon/config.toml
```

If the file is missing, built-in defaults are used. See
[configuration reference](docs/configuration.md) and
[`resources/config/config.example.toml`](resources/config/config.example.toml).

The default dictionary location is:

```text
$XDG_DATA_HOME/lexicon/dictionaries/en.sqlite
```

## Project layout

```text
src/core/            domain types, errors, interfaces
src/config/          TOML loading, validation, defaults
src/dictionary/      domain models and lookup service
src/infrastructure/  logging, SQLite repository, Unix signal handling
src/platform/        optional selection backends (Wayland; X11 later)
src/kde/             KGlobalAccel, tray, and session adapters
src/app/             composition root, controller, QML presenter, selection factory, CLI
ui/                  QML settings window and popup
data/dictionaries/   schema and development fixture
docs/                architecture and reference documentation
packaging/shared/    desktop, metainfo, and autostart templates
```

## Testing and quality checks

```sh
just check       # formatting, QML lint, build, and tests
just lint        # clang-tidy with warnings as errors
just coverage    # instrumented build, tests, and gcovr report
```

Reports are written to `build/coverage-report/`. The tests cover core types,
configuration loading and validation, dictionary service behavior, controller
orchestration, SQLite lookup, and QML smoke loading. The default CTest preset excludes
tests labelled `kde`, because shortcut, tray, and selection behavior depends on a live
Plasma session. Run those with `just test-kde`.

The GitHub Actions pipeline runs formatting and QML lint, GCC and Clang builds with
warnings as errors, ASan/UBSan, clang-tidy, coverage, and an install-tree check. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the full command list.

## Known limitations

- Selected-text retrieval works on Wayland (`ext_data_control_manager_v1`) only. X11
  returns an explicit `SelectionUnsupported` error, and nothing ever falls back to the
  clipboard or reuses stale text.
- Selections longer than 128 characters are rejected rather than looked up.
- The settings window is read-only in Stage 1.
- `start_on_login` is accepted but does not install an autostart entry until Stage 3.
- The bundled dictionary data is a fixture, not a real dictionary.
- The project has no production package, Flatpak, AppImage, or release pipeline yet.

## License

MIT. See [LICENSE](LICENSE). The vendored toml++ copy is also MIT licensed; see
`third_party/toml++/LICENSE`.
