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
   +--> SelectionProvider --> KdeSelectionProvider (stub)
   |
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

### Dictionary

`src/dictionary` contains the `WordEntry` model and `DictionaryService`. The service
normalizes input, checks empty input, delegates to a repository, and maps a missing row
to `WordNotFound`.

### Infrastructure

`src/infrastructure` contains operational adapters:

- SQLite lookup opens a short-lived read-only connection per lookup. This keeps lookups
  safe when run from a worker thread.
- Logging categories are namespaced under `lexicon.*`.
- `UnixSignalHandler` turns SIGINT/SIGTERM into a Qt signal.

### KDE integration

`src/kde` contains adapters that depend on KDE Frameworks:

- KGlobalAccel registration creates a `QAction` per shortcut and removes it on shutdown.
- KStatusNotifierItem provides tray actions. Tray availability is checked with
  `QSystemTrayIcon::isSystemTrayAvailable()`.
- KDBusService enforces single-instance behavior and re-activation opens settings.
- The selection provider is deliberately a stub. See
  [selection retrieval](selection-retrieval.md).

### Application

`src/app` wires everything together.

- `Application` owns the QML engine and service composition.
- `ApplicationController` handles asynchronous lookups with `QtConcurrent` and a
  `QFutureWatcher`, so the GUI thread is never blocked by SQLite.
- `QmlPresenter` owns the lazy `QQuickView` popup and maps domain models to QVariant
  data for QML.

## Threading

The GUI thread owns QML, actions, and the controller. Dictionary lookups run on a worker
thread. `SqliteDictionaryRepository` never shares a `QSqlDatabase` connection between
threads; each call creates and removes its own connection.

## Lifecycle

1. `main` parses CLI options, initializes logging, loads configuration, and applies the
   optional database override.
2. `Application` creates the hidden settings `ApplicationWindow` and the presenter.
3. `ApplicationController` registers the global shortcut and starts the tray item.
4. The application stays alive with no visible window (`quitOnLastWindowClosed` is off).
5. The lookup popup is created lazily on first use.
6. Shutdown unregisters shortcuts, stops the tray item, waits for any in-flight lookup,
   and then tears down QML.
