# AGENTS.md — Lexicon Development Guide

Lexicon is a native, offline-first English dictionary daemon for KDE Plasma (C++20, Qt 6, KF6). It runs in the background, registers a global shortcut and tray icon, and displays definitions in a focused QML popup. Currently at **Stage 3** (editable settings, dictionary catalog, pronunciation, optional offline translation, lookup cache, autostart, and packaging; Wayland primary-selection lookup implemented, X11 explicitly unsupported).

## Essential Commands

The project uses `just` (wrapping `cmake`, `ninja`, and `ctest` with presets):

```sh
just build         # Configure and build dev-debug preset
just test          # Run unit and integration tests (excludes 'kde' labeled tests)
just test-all      # Run all tests including live session tests
just format        # Reformat C++ files using clang-format
just format-check  # Verify formatting without modifying files
just lint          # Run clang-tidy against build/lint compile database
just seed-db       # Build dev SQLite dictionary fixture at build/dev/lexicon.sqlite
just demo          # Build, seed fixture, and run headless test lookup (QT_QPA_PLATFORM=offscreen)
just run           # Build and launch daemon
```

Direct binary execution:
```sh
./build/dev-debug/lexicon --lookup serendipity --database build/dev/lexicon.sqlite
./build/dev-debug/lexicon --config <path> --debug
```

## Architecture & Module Seams

Strict inward-only dependencies are enforced:

- `src/core/`: Plain domain types (`Result<T>`, `AppError`, `ErrorCode`), interface contracts (`SelectionProvider`, `PrimarySelectionSource`, `PopupPresenter`, `ShortcutService`, `TrayService`), and platform-neutral selection logic (`PrimarySelectionProvider`, `SelectionTextPolicy`). Zero dependencies on QML, Widgets, SQLite, KDE, or a display server.
- `src/dictionary/`: Domain models (`WordEntry`, `Definition`, `PartOfSpeech`, `Pronunciation`, `DictionaryInfo`), `DictionaryService`, `DictionaryRepository` interface, `DictionaryCatalog` (installed dictionaries and language-pair resolution), `CatalogDictionaryRepository`. Zero dependencies on QML, SQLite, or KDE.
- `src/infrastructure/`: Operational adapters:
  - `database/`: `SqliteDictionaryRepository` implementing `DictionaryRepository`.
  - `logging/`: Structured category logging (`lexiconApp`, `lexiconConfig`, `lexiconDictionary`, `lexiconDatabase`, `lexiconKde`, `lexiconUi`).
  - `UnixSignalHandler`: POSIX signal translation to Qt signals.
- `src/config/`: TOML configuration loading via `toml++`, schema validation, fallback defaults, and XDG path resolution (`ConfigLoader`).
- `src/platform/`: Optional, per-platform selection backends implementing `PrimarySelectionSource`. `wayland/` uses `ext_data_control_manager_v1`. A future `x11/` backend implements the same interface. Backends must stay optional (`LEXICON_WITH_WAYLAND`) and must not be included above `src/app`.
- `src/kde/`: KDE Frameworks 6 adapters (`KGlobalAccelShortcutService`, `KStatusNotifierTrayService`, `KdeSession`).
- `src/app/`: Composition root (`Application`, `main.cpp`), async orchestrator (`ApplicationController` via `QtConcurrent`), presenter (`QmlPresenter`), and `SelectionProviderFactory` — the only platform-aware code.
- `ui/`: Purely declarative QML (Qt Quick Controls 2, custom theme). Business logic remains in C++ presenters.

## Critical Design Rules & Gotchas

### Concurrency & Database Access
- GUI thread owns QML engine, windows, QActions, and `ApplicationController`.
- Dictionary lookups run asynchronously on worker threads via `QtConcurrent::run`.
- **Never share `QSqlDatabase` connections across threads.** `SqliteDictionaryRepository` creates a short-lived UUID-named read-only connection per lookup and removes it before returning.

### Error Handling
- Never use C++ exceptions for control flow.
- Return explicit `Result<T>` and `AppError` (`ErrorCode`).
- Model expected domain outcomes (such as `WordNotFound` and `SelectionUnavailable`) as explicit results.

### Privacy & Security Guardrails
- Offline-only: Do not add network calls or remote telemetry.
- Selection privacy: Never log selected text, clipboard contents, or sensitive data.
- Selection integrity: Do not fall back to the clipboard or inject synthetic keystrokes (`Ctrl+C`) when selected text is unavailable. Return an explicit error (`SelectionUnavailable`, `SelectionUnsupported`, `SelectionNotText`, or `SelectionTooLong`).
- Platform seams: keep platform code in `src/platform` behind `PrimarySelectionSource`; `SelectionProviderFactory` is the only place that may branch on the session. Adding X11 or another desktop must not require changes to core, the controller, or the UI.
- SQLite queries: Always use bound parameters (`?` placeholders); never format values directly into SQL strings.
- Dictionary content: Never download dictionary data without an explicit user action, and never present content whose `dictionary_info` attribution is missing.
- Test fixture: `data/dictionaries/dev-seed.sql` is a small CC0 test fixture, not a production dictionary.

### Coding Standards
- Standard: C++20 with strict compiler warnings (`-Wall -Wextra -Wpedantic -Wshadow`).
- Formatting: Follow `.clang-format` (LLVM base, 4-space indent, attach braces, column limit 120, pointer left-aligned) and `.clang-tidy`.
- Naming: PascalCase for types, camelCase for functions/methods, `m_` prefix for private member variables.
- Structured logging: Use category macros with key-value format:
  `qCWarning(lexiconDatabase) << "event=database_open_failed path=" << path << "detail=" << detail;`

### Testing
- Unit tests live in `tests/unit/` (`test_config.cpp`, `test_dictionary_service.cpp`).
- Integration tests live in `tests/integration/` (`test_sqlite_repository.cpp`).
- Label any test requiring a live Plasma/Wayland session with `kde` (the default CTest preset excludes `kde`; run them with `just test-kde`).
- Selection logic is tested without a compositor through `PrimarySelectionSource` fakes; the live Wayland path is covered by `tests/integration/test_wayland_selection.cpp` using `wl-copy`.

## Context Pointers

Consult these references when working on specific areas:

- Architecture, subsystem wiring, and application lifecycle: [docs/architecture/overview.md](docs/architecture/overview.md)
- Wayland primary selection protocol (`ext_data_control_manager_v1`) and Stage 2 design: [docs/architecture/selection-retrieval.md](docs/architecture/selection-retrieval.md)
- Configuration options, TOML schema, and defaults: [docs/configuration.md](docs/configuration.md)
- Database schema, dictionary tables, and fixture rules: [docs/dictionary-format.md](docs/dictionary-format.md)
- Milestone progression and roadmap stages: [ROADMAP.md](ROADMAP.md)
