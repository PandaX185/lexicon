# Offline translation

Translation is optional and disabled by default. Definition lookup never depends
on it: if translation fails or is missing, the dictionary result is still
returned with a separate `translation_error`.

Lexicon translates with the native [Bergamot](https://github.com/browsermt/bergamot-translator)
engine. There is no network provider, no WASM runtime, and no implicit download:
model packs are installed only when you ask for them.

## Default build

The default build contains a stub engine:

```sh
lexicon config set translation.enabled true
lexicon doctor
# translation  unavailable: rebuild with -tags bergamot ...
```

`translation.enabled = true` is a valid configuration. The daemon reports the
translator as unavailable and keeps serving lookups.

## Release packages

Stable tarballs and the Arch package ship a daemon with Bergamot compiled in,
so `lexicon doctor` reports the translator as available without a local C++ build.
Bundling the engine does not install any models: install a pack explicitly with
`lexicon languages install ID` before enabling a language pair. Definitions always
work, including when no pack is installed or a translation fails.

## Building the engine

The `bergamot` build tag selects the C++ bridge in
`internal/translation/bergamotengine`. The bridge is pinned to Bergamot
`v0.4.5` and is built with the same Ubuntu 24.04 / GCC 13 image that was used to
verify it.

```sh
just bergamot-build
source build/bergamot/env.sh
just daemon_go_tags=bergamot build-daemon
```

`just bergamot-build` runs `tools/bergamot/build.sh`, which:

1. clones `browsermt/bergamot-translator` at `v0.4.5` with submodules into
   `build/bergamot/src` (override with `BERGAMOT_ROOT`);
2. builds the pinned `tools/bergamot/Dockerfile` image (Ubuntu 24.04, GCC 13,
   OpenBLAS);
3. configures and builds in Docker into `build/bergamot/build-native`
   (override with `BERGAMOT_BUILD`);
4. writes `build/bergamot/env.sh` with the `CGO_*FLAGS` that match the build
   tree.

Two build details are load-bearing:

- GCC 13 needs `<cstdint>` in the vendored faiss sources. Marian overwrites
  `CMAKE_CXX_FLAGS`, so the include is injected through
  `CMAKE_PROJECT_TOP_LEVEL_INCLUDES=tools/bergamot/cstdint.cmake` and guarded by
  `COMPILE_LANGUAGE:CXX` so the C sources are untouched.
- The default build arch is `x86-64-v2` (SSE4.2/POPCNT), the lowest baseline
  Bergamot's intgemm supports. Set `BERGAMOT_ARCH=native` for a local-only build
  that uses your CPU's full instruction set; release builds stay on v2 so they
  do not require AVX2 or AVX-512.

Only the daemon links Bergamot; `lexicon` and `lexicon-installer` stay untagged
builds. The generated flags are equivalent to:

```sh
CGO_ENABLED=1
CGO_CXXFLAGS="-std=c++17 -O2 -include cstdint -I$BERGAMOT_ROOT -I$BERGAMOT_ROOT/src ..."
CGO_LDFLAGS="-L$BERGAMOT_BUILD/src/translator -L$BERGAMOT_BUILD ... \
  -lbergamot-translator -lmarian -lsentencepiece_train -lsentencepiece \
  -lintgemm -lssplit -lblas -llapack -lcblas -lpcre2-8 -ldl -lpthread -lstdc++ -lm"
```

On distributions without OpenBLAS the link uses netlib `-lblas -llapack
-lcblas`. Override the BLAS flags with `BERGAMOT_BLAS_LIBS` when needed. Release
tarballs are built with `BERGAMOT_BLAS_LIBS=-lopenblas -lpthread`, so the
installed daemon requires `libopenblas.so.0` at runtime (`openblas` on Arch and
Fedora, `libopenblas0` on Debian/Ubuntu).

The bridge is isolated: C++ owns the engine and cached models, Go holds only an
opaque handle, C++ exceptions become allocated error strings, and returned
strings are freed through the bridge. No C++ type or exception crosses the CGO
boundary.

## Installing model packs

`lexicon languages` manages packs from the catalog embedded in the binary. Every
catalog entry pins an HTTPS tarball, its SHA-256, and its exact size.

```sh
lexicon languages list                 # installed packs and the catalog
lexicon languages install bergamot-en-de-tiny11
lexicon languages use en-de            # validates an installed pair; select it in the popup
lexicon config set translation.enabled true
lexicon languages remove bergamot-en-de-tiny11
```

`lexicon languages install` streams the tarball into the model directory, verifies
the size and checksum, extracts only the files named by the catalog entry,
writes a fresh `metadata.json`, validates the result, and renames the staging
directory into place. It never overwrites an installed pack. Redirects are not
followed, and a failed install leaves nothing behind.

The shipped catalog covers these verified statmt.org tiny11 models:

| Pair | Catalog id |
| --- | --- |
| en → de | `bergamot-en-de-tiny11` |
| de → en | `bergamot-de-en-tiny11` |
| en → es | `bergamot-en-es-tiny11` |
| es → en | `bergamot-es-en-tiny11` |
| en → et | `bergamot-en-et-tiny11` |
| et → en | `bergamot-et-en-tiny11` |

They are CC-BY-SA-4.0, curated by the Bergamot project. The catalog is
`internal/translation/catalog.json`; each entry names the files inside the
tarball, so adding a verified pack is a data-only change.

## Model packs on disk

Packs live under `translation.model_directory`, which defaults to
`$XDG_DATA_HOME/lexicon/models`. Each immediate subdirectory is a candidate and
must contain `metadata.json`:

```json
{
  "format": 1,
  "id": "bergamot-en-de-tiny11",
  "source": "en",
  "target": "de",
  "engine": "bergamot",
  "architecture": "tiny11",
  "gemm_precision": "int8shiftAlphaAll",
  "license": "CC-BY-SA-4.0",
  "attribution": "Bergamot project (data.statmt.org)",
  "files": [
    {"role": "model", "file": "model.intgemm.alphas.bin"},
    {"role": "shortlist", "file": "lex.s2t.bin"},
    {"role": "vocab", "file": "vocab.spm"}
  ]
}
```

Rules enforced when a pack is read:

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
and target vocabulary, matching common Firefox and statmt model packs.

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

`gemm-precision` is added when the pack sets it. `mini-batch-words` is required:
Bergamot aborts silently when it is missing, which is why the shipped
`config.intgemm8bitalpha.yml` files cannot be passed through unchanged.
`max-length-break` must not exceed `mini-batch-words`.

## Installing packs

Packs are never downloaded implicitly. During first-run setup the installer
shows the embedded catalog and lets you select zero or more packs. Each select
is a deliberate HTTPS download verified against the catalog SHA-256. Failures
are reported on the final screen with an exact `lexicon languages install ID`
retry command, and definitions continue to work.

You can also install a specific pack later:

~~~sh
lexicon languages list
lexicon languages install bergamot-en-de-tiny11
lexicon languages use en-de             # validates; pair selection stays in the popup
~~~

## Choosing the active pair in the popup

When translation is enabled and at least one pack is installed, the result view
shows two custom dropdowns directly below the definitions: **From** and **To**.
Both lists are sorted BCP 47 codes drawn from installed packs; pack metadata is
available from `lexicon languages list`. **From** lists every installed source
code. **To** lists only the targets installed for the selected source; changing
**From** filters **To** and selects its first target. Changing **To** applies
that target for the current source. The dropdowns are in-webview popovers so
choosing a language cannot open a separate GTK window and dismiss the popup.

The popup emits `lexicon:set-language-pair`. The daemon validates the pair
against installed packs, updates the active pair for the current session,
broadcasts the new state, and relooks up the last query so the translation
matches the selected pair. The popup replaces the visible result as soon as the
new lookup arrives, without re-showing or reopening the window. No config file
is written: closing the popup keeps the choice while the daemon lives, but
restarting the daemon resets it to the first installed pair.
`lexicon languages use SRC-TGT` validates an installed pair and points to this
control; it does not change the active pair or write config.

## Runtime behavior

- Translation runs only when a dictionary entry was found, translation is
  enabled and available, and the active session pair's source and target differ
  after canonicalization. The selected pair is independent of the dictionary
  pair, so a monolingual English dictionary can translate into another
  language.
- The normalized query is translated, not the original selection.
- `translation.timeout_seconds` bounds a call when no earlier deadline exists.
  The native call is bounded by that deadline, but C++ cancellation is not
  supported: a late callback is ignored after the timeout.
- Lookups can run concurrently under the daemon's resource read lock. The native
  service has one worker, and the bridge mutex protects model creation.
- Translation text and results are never logged; the daemon records only the
  query length.

## Verification

The engine was verified end to end against the pinned v0.4.5 build with the six
catalog packs and a real Firefox pack:

- `Hello world` → `Hallo Welt` / `Hola mundo` / `Tere maailm`;
- `Hallo Welt` / `Hola mundo` / `Tere maailm` → `Hello world`;
- a full daemon lookup printed the WordNet definition plus
  `translation: hallo`.

Re-run the opt-in integration test against any installed pack:

```sh
source build/bergamot/env.sh
just bergamot-test ~/.local/share/lexicon/models/bergamot-en-de-tiny11
```

## Deferred work

- Additional translation engines.
- Pack updates and signatures.
- Automatic pack selection when the configured pair has no pack.
