# Contributing

## Development setup

```sh
cmake --preset dev-debug
cmake --build --preset dev-debug
ctest --preset dev-debug
```

The presets use Ninja and write to `build/<preset>`. Prerequisites are listed in the
README. Install `clang`, `just`, and `gcovr` for the full local checks.

## Quality checks

Run the fast local gate before opening a pull request:

```sh
just check
```

This checks formatting with `clang-format --dry-run --Werror`, lints QML with the Qt 6
`qmllint` target, builds the project, and runs CTest.

Additional checks:

```sh
just format        # apply clang-format
just lint          # clang-tidy with --warnings-as-errors=*
just test-all      # run every label, including kde
just coverage      # gcovr XML and HTML in build/coverage-report
just install-check # stage an install tree and verify expected paths
```

`just coverage` requires `gcovr`. The `coverage` preset instruments the build with
`--coverage` and still runs the normal test suite.

## CI presets

- `ci-gcc` and `ci-clang`: Debug builds with `LEXICON_WARNINGS_AS_ERRORS=ON`.
- `ci-asan`: AddressSanitizer and UndefinedBehaviorSanitizer with leak detection.
- `coverage`: coverage-instrumented Debug build used by `just coverage`.
- `lint`: Clang compile database consumed by clang-tidy.

The GitHub Actions pipeline mirrors these presets inside an Arch Linux container with
Qt 6 and KDE Frameworks 6. It also checks the install tree so packaging regressions,
such as a file escaping the expected prefix, fail the build.

## Code guidelines

- C++20, Qt 6, KDE Frameworks 6.
- Keep domain code in `src/core` and `src/dictionary` free of QML, SQLite, and KDE
  dependencies.
- Use explicit result types for expected failures; do not use exceptions for control
  flow.
- Keep QML declarative and free of business logic.
- Do not log user-selected text, clipboard contents, or secrets.
- Do not add network access without a documented privacy decision.

## Tests

- Add focused unit tests for domain and configuration changes.
- Add integration tests for SQLite behavior.
- Prefer the header-only helpers in `tests/support/`:
  `TestDoubles.h`, `SqliteFixture.h`, and `ScopedEnvironment.h`.
- QML smoke tests use `QQmlApplicationEngine` or `QQuickView` in C++ and load sources
  from the filesystem. Run them with `QT_QPA_PLATFORM=offscreen` and
  `QT_QUICK_BACKEND=software`; the CTest registration sets both.
- Use the `kde` label only for tests that need a live Plasma session; the default preset
  excludes them.

## Commits and pull requests

- Keep changes focused and explain the user-visible effect.
- Include verification commands and results.
- Do not commit generated build directories or local databases.
- `main` is protected: changes must arrive through a pull request with all CI checks
  green. Only squash and rebase merges are allowed, force pushes are rejected, and
  review conversations must be resolved before merging.
- Head branches are deleted automatically after a pull request is closed.
