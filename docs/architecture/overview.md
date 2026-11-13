# Architecture overview

Lexicon is split into a small domain core, explicit infrastructure adapters, a KDE
integration layer, and a QML presentation layer. The application object is the only
composition root.

```text
main.cpp / Application
        |
        v
ApplicationController
   |        |         |
   |        |         +--> PopupPresenter --> QmlPresenter --> QML popup/settings
   |        +------------> ShortcutService --> KGlobalAccelShortcutService
   +---------------------> TrayService -----> KStatusNotifierTrayService
   |
   +--> SelectionProvider --> PrimarySelectionProvider --+--> SelectionTextPolicy
   |                                                      +--> PrimarySelectionSource
   |                                                             (platform backend)
   +--> DictionaryService --> DictionaryRepository --> SqliteDictionaryRepository
```

## Responsibilities

### Core

`src/core` contains plain domain types, errors, and interfaces. It has no dependency on
KDE, SQLite, or QML.

- `Result<T>` and `AppError` provide explicit success/failure values.
- `SelectionResult`, `ServiceStatus`, and `DictionaryLookupResult` model expected
  failures without exceptions.
- Interfaces define the seams used by the controller and by tests.

### Configuration

`src/config` loads TOML using toml++ and returns `ConfigLoadResult`, including warnings
and errors. Invalid values keep their defaults. Unknown keys are reported. Paths are
resolved against XDG locations or the configuration file directory.

`ConfigLoader::save` writes the whole document through `QSaveFile`, so an interrupted
save cannot truncate an existing configuration. If the previous file contained comments
or keys the schema does not know, it is copied to `config.toml.bak` and a warning lists
what the rewrite would drop.

### Dictionary

`src/dictionary` contains the `WordEntry` model and `DictionaryService`. The service
normalizes input, checks empty input, delegates to a repository, and maps a missing row
to `WordNotFound`. Successful lookups pass through a bounded `QCache`; `cache_size = 0`
disables it, and `clearCache()` is available after dictionary content changes.

### Infrastructure

`src/infrastructure` contains operational adapters:

- SQLite lookup opens a short-lived read-only connection per lookup. This keeps lookups
  safe when run from a worker thread.
- Logging categories are namespaced under `lexicon.*`.
- `UnixSignalHandler` turns SIGINT/SIGTERM into a Qt signal.

### Platform backends

`src/platform` contains optional adapters that implement a core interface and are
selected by the composition root only. Nothing above `src/app` includes them.

- `wayland/` implements `PrimarySelectionSource` with `ext_data_control_manager_v1`.
  It is built only when `wayland-client` and `wayland-scanner` are available, or can
  be disabled with `-DLEXICON_WITH_WAYLAND=OFF`.
- A future `x11/` backend would implement the same interface. X11 currently reports an
  explicit unsupported error instead of reading the clipboard.
- The controller, presenter, and UI depend only on `SelectionProvider`, so backends can
  be added, removed, or swapped without touching them.

See [selection retrieval](selection-retrieval.md).

### KDE integration

`src/kde` contains adapters that depend on KDE Frameworks:

- KGlobalAccel registration creates a `QAction` per shortcut and removes it on shutdown.
- KStatusNotifierItem provides tray actions.
- KDBusService enforces single-instance behavior and re-activation opens settings.

### Application

`src/app` wires everything together.

- `Application` owns the QML engine and service composition. Before the repository is
  built it asks `DictionarySeeder` to copy the installed dictionary into the configured
  or writable dictionary directory when none is present.
- `ApplicationController` handles asynchronous lookups with `QtConcurrent` and a
  `QFutureWatcher`, so the GUI thread is never blocked by SQLite. It delegates autostart
  reconciliation to `AutostartManager`.
- `AutostartManager` installs or removes the per-user XDG entry, writes an absolute
  `Exec` path (or `flatpak run …` inside a sandbox), and is testable with temporary
  paths.
- `SettingsController` exposes the editable configuration to QML, applies autostart
  changes immediately, and saves through `ConfigLoader`.
- `QmlPresenter` owns the lazy `QQuickView` popup and maps domain models to QVariant
  data for QML.

## Threading

The GUI thread owns QML, actions, and the controller. Dictionary lookups run on a worker
thread. `SqliteDictionaryRepository` never shares a `QSqlDatabase` connection between
threads; each call creates and removes its own connection.

## Lifecycle

1. `main` parses CLI options, initializes logging, loads configuration, and applies the
   optional database override.
2. `Application` creates the settings controller, seeds the user dictionary directory if
   needed, then creates the hidden settings `ApplicationWindow` and the presenter.
3. `ApplicationController` registers the global shortcut, reconciles the autostart entry
   with `start_on_login`, and starts the tray item.
4. The application stays alive with no visible window (`quitOnLastWindowClosed` is off).
5. The lookup popup is created lazily on first use.
6. Shutdown unregisters shortcuts, stops the tray item, waits for any in-flight lookup,
   and then tears down QML.
