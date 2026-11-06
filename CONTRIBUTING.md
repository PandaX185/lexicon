# Contributing

## Development setup

```sh
cmake --preset dev-debug
cmake --build --preset dev-debug
ctest --preset dev-debug
```

Use `just format` before submitting changes and run `just lint` when clang-tidy is
available.

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
- Use the `kde` label only for tests that need a live Plasma session; the default preset
  excludes them.

## Commits and pull requests

- Keep changes focused and explain the user-visible effect.
- Include verification commands and results.
- Do not commit generated build directories or local databases.
