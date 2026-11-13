# Lexicon

Lexicon is a small, native, offline-first English dictionary for KDE Plasma. It runs
quietly in the background behind a global shortcut and tray icon, and shows definitions
in a focused popup.

New to Lexicon? The [user guide](docs/user-guide.md) walks through installation,
first-run setup, lookups, settings, configuration, translation, and troubleshooting.

## Features

- Offline WordNet 3.0 dictionary with CMUdict-derived IPA or respelling pronunciation,
  including inflected forms (`went` resolves to `go`) with the substitution shown.
- Multi-dictionary catalog with language-pair metadata and romanized lookup.
- Selected-text lookup on Wayland (KWin, `ext_data_control_manager_v1`) through a
  configurable global shortcut. X11 reports an explicit unsupported error; there is no
  clipboard fallback.
- Tray integration, a lazily created popup, and command-line lookup.
- Optional offline translation through local Argos Translate packs that the user
  installs explicitly over HTTPS.
- Editable settings written atomically to `config.toml`, a bounded lookup cache,
  per-user autostart, and first-run dictionary seeding.

## Requirements

- CMake 3.24 or newer
- A C++20 compiler
- Qt 6.10 or newer (Core, Gui, Widgets, Qml, Quick, QuickControls2, Sql, Test, DBus,
  Concurrent, Network)
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

## Installation

The package produces the install layout `bin/lexicon`, `share/lexicon/`,
`share/applications/`, and the autostart template in `share/lexicon/`.

### Arch Linux

Build and install from `packaging/arch/PKGBUILD`:

```sh
cd packaging/arch
makepkg -si
```

The PKGBUILD consumes the released source tarball and verifies its SHA-256 checksum.
Bump `pkgver`, `sha256sums`, and `.SRCINFO` together when preparing a release. `check()`
runs the CTest suite with the `kde` label excluded, so the package builds on a headless
machine; live Plasma tests remain available through `just test-kde`.

Optional dependencies:

- `python` runs `share/lexicon/scripts/argos_worker.py`.
- `python-argostranslate` provides the offline translation models. It is not in the
  official repositories; install it with `pip` or from the AUR.

### From source

```sh
cmake --preset dev-release
cmake --build --preset dev-release
ctest --preset dev-release
sudo cmake --install build/dev-release --prefix /usr
```

### Release tarball

Tagged `v*` pushes run `.github/workflows/release.yml`, which builds and tests in an Arch
container and publishes two archives with SHA-256 checksums:

- `lexicon-<version>-x86_64-install.tar.gz` contains the staged install tree. This is the
  one to use for a manual install; extract it at the filesystem root:

  ```sh
  sha256sum -c lexicon-0.1.0-x86_64-install.tar.gz.sha256
  sudo tar -C / -xzf lexicon-0.1.0-x86_64-install.tar.gz
  ```

- `lexicon-<version>.tar.gz` contains the source tree and is intended for packagers.

## Usage

Lexicon starts without showing a window. Invoke it with the global shortcut
(`SUPER+SHIFT+D` by default), the tray icon, or a command-line lookup:

```sh
lexicon --lookup serendipity
lexicon --config ~/.config/lexicon/config.toml --debug
```

The settings window edits the same configuration the daemon uses. Most changes apply
immediately; shortcut, dictionary, cache, and theme changes take effect after a restart.

### Configuration

The default location is:

```text
$XDG_CONFIG_HOME/lexicon/config.toml
```

If the file is missing, built-in defaults are used. See the
[configuration reference](docs/configuration.md) and
[`resources/config/config.example.toml`](resources/config/config.example.toml).
Saving from the settings window rewrites the file atomically; when the previous file
contained comments or keys Lexicon does not recognize, it is copied to
`config.toml.bak` first and a warning records what would be dropped.

### Dictionary data

The default dictionary location is:

```text
$XDG_DATA_HOME/lexicon/dictionaries/en.sqlite
```

On first run Lexicon copies the installed dictionary there when the configured or
default dictionary directory is empty. The repository ships the generated WordNet 3.0 +
CMUdict database at `data/dictionaries/en.sqlite` (about 43 MB); see
[WordNet dictionary](docs/wordnet-dictionary.md) and
[dictionary format](docs/dictionary-format.md).

A small CC0 fixture is available for fast smoke tests:

```sh
just seed-db
./build/dev-debug/lexicon --lookup serendipity --database build/dev/lexicon.sqlite
```

## Documentation

- [User guide](docs/user-guide.md) — install, first run, lookups, settings, troubleshooting
- [Configuration reference](docs/configuration.md)
- [Architecture overview](docs/architecture/overview.md)
- [Selection retrieval](docs/architecture/selection-retrieval.md)
- [Dictionary format](docs/dictionary-format.md) and
  [WordNet dictionary](docs/wordnet-dictionary.md)
- [IPA pronunciation](docs/pronunciation-ipa.md)
- [Language packs](docs/language-packs.md) and
  [offline translation](docs/argos-translation.md)
- [Support](SUPPORT.md), [security policy](SECURITY.md), and
  [contributing](CONTRIBUTING.md)

## Development

```sh
just build         # configure and build the dev-debug preset
just test          # unit and integration tests (excludes the kde label)
just check         # formatting, QML lint, build, and tests
just lint          # clang-tidy with warnings as errors
just coverage      # instrumented build, tests, and gcovr report
just install-check # stage an install tree and verify expected paths
```

The presets use Ninja and write to `build/<preset>`. The default CTest preset excludes
tests labelled `kde`, because shortcut, tray, and selection behavior depends on a live
Plasma session. Run those with `just test-kde`.

The GitHub Actions pipeline runs formatting and QML lint, GCC and Clang builds with
warnings as errors, ASan/UBSan, clang-tidy, coverage, and an install-tree check. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the full command list and code guidelines, and
[CHANGELOG.md](CHANGELOG.md) for release history.

### Project layout

```text
src/core/            domain types, errors, interfaces
src/config/          TOML loading, validation, defaults
src/dictionary/      domain models and lookup service
src/infrastructure/  logging, SQLite repository, Unix signal handling
src/platform/        optional selection backends (Wayland; X11 later)
src/kde/             KGlobalAccel, tray, and session adapters
src/app/             composition root, controller, QML presenter, selection factory, CLI
ui/                  QML settings window and popup
data/dictionaries/   WordNet 3.0 database, schema, and test fixture
docs/                architecture and reference documentation
packaging/           Arch PKGBUILD and shared desktop files
.github/workflows/   CI and tagged release automation
```

## Known limitations

- Selected-text retrieval works on Wayland (`ext_data_control_manager_v1`) only. X11
  returns an explicit `SelectionUnsupported` error, and nothing ever falls back to the
  clipboard or reuses stale text.
- Selections longer than 128 characters, or longer than four words, are rejected rather
  than looked up; the latter is treated as a sentence, not a headword.
- Settings are saved immediately, but shortcut, dictionary, cache, and theme changes
  take effect after restart unless the UI says otherwise.
- Language pack downloads require HTTPS. Offline translation additionally needs
  `python3` and `argostranslate`.

## License

MIT. See [LICENSE](LICENSE). The vendored toml++ copy is also MIT licensed; see
`third_party/toml++/LICENSE`.
