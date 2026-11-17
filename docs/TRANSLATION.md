# Offline translation

Translation is optional and disabled by default. Definition lookup never depends
on it: if translation fails or is missing, the dictionary result is still
returned with a separate `translation_error`.

## Default build

The default build contains a stub engine:

```sh
lex config set translation.enabled true
lex doctor
# translation  unavailable: rebuild with -tags bergamot ...
```

`translation.enabled = true` is a valid configuration. The daemon reports the
translator as unavailable and keeps serving lookups.

## Bergamot build (unverified)

The `bergamot` build tag selects a small C++ bridge in
`internal/translation/bergamotengine`. It targets the Bergamot v0.4.5-style API
(`parseOptionsFromString`, `AsyncService`, `createCompatibleModel`, and the
`translate` callback).

The bridge is intentionally isolated:

- C++ owns the engine and cached models;
- Go holds only an opaque handle;
- C++ exceptions are converted to allocated error strings;
- returned strings are freed through the bridge;
- no C++ type or exception crosses the CGO boundary.

Build requirements:

- CGO enabled and a C++17 compiler;
- Bergamot headers and the `bergamot-translator` library (plus zlib);
- include/library paths supplied through CGO flags.

Example (paths are illustrative):

```sh
CGO_ENABLED=1 \
CGO_CXXFLAGS="-I/opt/bergamot/include" \
CGO_LDFLAGS="-L/opt/bergamot/lib" \
just daemon_go_tags=bergamot core
```

Only the daemon links Bergamot; the `lex` CLI and `lexicon-installer` stay
untagged builds.

The source bridge declares:

```text
-lbergamot-translator -lz -lpthread -lstdc++
```

This path has **not** been compiled or tested in this repository's development
environment because no Bergamot tree or library was available. Treat it as a
reviewed starting point and verify the API against your Bergamot revision. The
default build, tests, and lint do not include this tag.

## Model packs

Packs live under `translation.model_directory`, which defaults to
`$XDG_DATA_HOME/lexicon/models`. Each immediate subdirectory is a candidate and
must contain `metadata.json`:

```json
{
  "format": 1,
  "id": "en-de",
  "source": "en",
  "target": "de",
  "engine": "bergamot",
  "architecture": "tiny11",
  "gemm_precision": "",
  "files": [
    {"role": "model", "file": "model.bin"},
    {"role": "shortlist", "file": "lex.s2t.bin"},
    {"role": "vocab", "file": "vocab.spm"}
  ]
}
```

Rules enforced by `FindPack`:

- `engine` must be `bergamot`; `source` and `target` must be non-empty.
- `id` and every `file` must be a plain file name: no path separators, `.` or
  `..`.
- Every listed file must exist and be a regular file inside the pack directory.
- Roles are `model`, `shortlist`, and `vocab` / `srcVocab` / `trgVocab`.
- A pack must provide at least one model, one shortlist, and one vocabulary.
- Invalid packs are skipped. Directories are scanned in name order and the first
  valid pack matching the requested pair wins.
- Language tags are canonicalized (`jp` → `ja`, `ger` → `de`, …) before matching.

A pack with a single vocabulary file is passed to Bergamot as both the source
and target vocabulary, matching common Firefox model packs.

## Generated Marian configuration

For each pack Lexicon builds a Marian YAML configuration and caches the resulting
model in the native engine by that string:

```yaml
models: ["/path/to/model.bin"]
vocabs: ["/path/to/vocab.spm", "/path/to/vocab.spm"]
shortlist: ["/path/to/lex.s2t.bin"]
beam-size: 1
normalize: 1.0
word-penalty: 0
max-length: 0
max-length-break: 100
mini-batch-words: 1024
maxi-batch: 1
maxi-batch-sort: none
```

`gemm-precision` is added when the pack sets it. The native service uses one
worker and no service-level cache.

## Runtime behavior

- Translation runs only when a dictionary entry was found, translation is
  enabled and available, and `dictionary.source_language` differs from
  `dictionary.target_language` after canonicalization.
- The normalized query is translated, not the original selection.
- `translation.timeout_seconds` bounds a call when no earlier deadline exists.
  The native call is bounded by that deadline, but C++ cancellation is not
  supported: a late callback is ignored after the timeout.
- Lookups can run concurrently under the daemon's resource read lock. The native
  service has one worker, and the bridge mutex protects model creation.

## Deferred work

- Building/distributing model packs.
- Downloading models or updates.
- Validating model signatures or checksums.
- Additional translation engines.
- Testing the CGO bridge against a pinned Bergamot release.
