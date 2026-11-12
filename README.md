# Lexicon

Lexicon is a small, native, offline-first English dictionary utility for KDE Plasma.
It starts quietly, stays available through a global shortcut and tray icon, and shows
definitions in a focused popup.

This repository currently contains the **Stage 1 scaffold**. It is intentionally small,
buildable, and honest about what is not implemented yet.

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
| Automatic selected-text retrieval on Wayland | Not implemented; see [selection retrieval](docs/architecture/selection-retrieval.md) |
| Dictionary content distribution | Not implemented; the included data is a tiny CC0 test fixture |
| Settings editing | Read-only in Stage 1; edit `config.toml` and restart |

## Requirements

- CMake 3.24 or newer
- A C++20 compiler
- Qt 6.10 or newer (Core, Gui, Widgets, Qml, Quick, QuickControls2, Sql, Test, DBus, Concurrent)
- KDE Frameworks 6: KGlobalAccel, KStatusNotifierItem, KDBusAddons
- Extra CMake Modules (ECM)
- SQLite through the Qt SQL plugin
- toml++ 3.4 or newer, or the vendored copy in `third_party/`

On Arch-based systems the development packages are typically:

```sh
sudo pacman -S --needed base-devel cmake ninja extra-cmake-modules \
    qt6-base qt6-declarative qt6-svg \
    kglobalaccel kstatusnotifieritem kdbusaddons
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
src/kde/             KGlobalAccel, tray, session, selection boundary
src/app/             composition root, controller, QML presenter, CLI
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
tests labelled `kde`, because KDE shortcut and tray behavior depends on a live Plasma
session.

The GitHub Actions pipeline runs formatting and QML lint, GCC and Clang builds with
warnings as errors, ASan/UBSan, clang-tidy, coverage, and an install-tree check. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the full command list.

## Known limitations

- Selected-text retrieval on Wayland is not implemented. The selection provider returns
  an explicit `SelectionUnavailable` error rather than silently reading the clipboard or
  reusing stale text.
- The settings window is read-only in Stage 1.
- `start_on_login` is accepted but does not install an autostart entry until Stage 3.
- The bundled dictionary data is a fixture, not a real dictionary.
- The project has no production package, Flatpak, AppImage, or release pipeline yet.

## License

MIT. See [LICENSE](LICENSE). The vendored toml++ copy is also MIT licensed; see
`third_party/toml++/LICENSE`.
