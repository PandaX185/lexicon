# Offline translation with Argos Translate

Translation is optional and entirely local. Lexicon never sends selected text anywhere:
the text goes to a Python subprocess on the same machine, which uses the Argos Translate
library and models installed on disk. There is no translation service, no API key, and no
socket in this path.

The feature is only reachable when `dictionary.source_language` and
`dictionary.target_language` differ:

```toml
[dictionary]
source_language = "en"
target_language = "de"
```

With equal tags the lookup stays monolingual and no worker is started.

## Components

| Layer | Type | Responsibility |
| --- | --- | --- |
| `src/core/interfaces/TranslationService.h` | interface | `translate`, `installedPairs`, `isPairInstalled` |
| `src/infrastructure/translation/ArgosProcessWorker` | QProcess adapter | starts `scripts/argos_worker.py`, speaks the JSON protocol, enforces timeouts |
| `src/infrastructure/translation/ArgosTranslationService` | `TranslationService` | maps protocol errors to `AppError`, offers pack install/remove for the settings UI |
| `src/infrastructure/translation/LanguagePackManager` | downloader | fetches the catalog and packs, verifies SHA-256, emits `packReady` |
| `scripts/argos_worker.py` | worker | the only component that imports `argostranslate` |

The worker is never started at application startup. It is launched lazily on the first
translated lookup, stopped on shutdown, and every request is serialized through one
mutex, because a single `QProcess` carries all requests.

## Protocol

Newline-delimited JSON on stdin/stdout; stderr is diagnostics only. One request per line,
one response per line.

```json
{"id": 3, "version": 1, "action": "translate", "params": {"text": "hello", "from": "en", "to": "de"}}
{"id": 3, "version": 1, "success": true, "result": {"translatedText": "hallo"}}
{"id": 4, "version": 1, "success": false, "error": {"code": "pair_not_installed", "message": "The language pair en -> de is not installed."}}
```

| Action | Params | Result |
| --- | --- | --- |
| `ping` | – | `{"status": "ok", "version": 1}` |
| `list_installed_pairs` | – | `{"pairs": [{"from", "from_name", "to", "to_name"}]}` |
| `translate` | `text`, `from`, `to` | `{"translatedText": "…"}` |
| `install_model` | `path` (local archive) | `{"installed": true}` |
| `uninstall_model` | `from`, `to` | `{"uninstalled": true}` |

Error codes are stable strings mapped to Lexicon error codes:

| Worker code | Meaning | `ErrorCode` |
| --- | --- | --- |
| `invalid_argument` | empty text or missing language tags | `InvalidArgument` |
| `pair_not_installed` | the model for the pair is missing | `TranslationUnavailable` |
| `runtime_missing` | `argostranslate` is not importable | `TranslationUnavailable` |
| `worker_failure`, `protocol_violation` | crash, timeout, or malformed line | `ProtocolViolation` |

Failures never fail the lookup. `ApplicationController` logs
`event=translation_unavailable` and presents the dictionary entry without a translation,
which is why a mixed-language lookup still works on a machine with no Python runtime.

## Runtime requirements

The worker needs `python3` and `argostranslate`:

```sh
python3 -m pip install --user argostranslate
```

Lexicon does not install Python packages. When the import fails, the worker answers
`runtime_missing` and the popup silently omits the translation card. Pack management in
the settings window reports the same message.

The worker script is installed to `/usr/share/lexicon/scripts/argos_worker.py`.
`ArgosProcessWorker` resolves it from the `LEXICON_ARGOS_WORKER` environment variable
first, then from the XDG data directories, so a development build can point at the source
tree.

## Tests

`tests/unit/test_argos_worker_protocol.cpp` drives the real worker script through the
`QProcess` plumbing: it pings, lists pairs (empty when `argostranslate` is absent), and
checks that a missing runtime surfaces as `runtime_missing`. A malformed-output case uses
`/bin/echo` instead of the script. The tests skip when `python3` or the script is missing;
`argostranslate` itself is never required.
