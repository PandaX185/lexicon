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
| Automatic selected-text retrieval on Wayland | Not implemented; see [selection retrieval](docs/architecture/selection-retrieval.md) |
| Dictionary content distribution | Not implemented; the included data is a tiny CC0 test fixture |
| Settings editing | Read-only in Stage 1; edit `config.toml` and restart |

## Requirements

- CMake 3.24 or newer
- A C++20 compiler
- Qt 6.10 or newer (Core, Gui, Qml, Quick, QuickControls2, Sql, Test, DBus, Concurrent)
- KDE Frameworks 6: KGlobalAccel, KStatusNotifierItem, KDBusAddons
- Extra CMake Modules (ECM)
- SQLite through the Qt SQL plugin
- toml++ 3.4 or newer, or the vendored copy in `third_party/`

On Arch-based systems the development packages are typically:

```sh
sudo pacman -S --needed base-devel cmake ninja extra-cmake-modules \
    qt6-base qt6-declarative qt6-quickcontrols2 qt6-sql \
    kglobalaccel kstatusnotifieritem kdbusaddons
```

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

## Testing

```sh
ctest --preset dev-debug
```

The tests cover configuration loading and validation, dictionary service behavior, and
SQLite lookup. The default CTest preset excludes tests labelled `kde`, because KDE
shortcut and tray behavior depends on a live Plasma session.

## Known limitations

- Selected-text retrieval on Wayland is not implemented. The selection provider returns
  an explicit `SelectionUnavailable` error rather than silently reading the clipboard or
  reusing stale text.
- The settings window is read-only in Stage 1.
- The bundled dictionary data is a fixture, not a real dictionary.
- The project has no production package, Flatpak, AppImage, or release pipeline yet.

## License

MIT. See [LICENSE](LICENSE). The vendored toml++ copy is also MIT licensed; see
`third_party/toml++/LICENSE`.
