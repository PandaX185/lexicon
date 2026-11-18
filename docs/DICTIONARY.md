# Dictionary format

Lexicon reads dictionaries directly from SQLite. It does not rebuild or modify
the database at runtime. Release packages bundle the English dictionary as a
release asset; the source checkout does not contain it and builds without it.
The expected format is the SQLite v3 schema used by the earlier Lexicon
application.

## Location

The default path is `$XDG_DATA_HOME/lexicon/dictionary.sqlite`
(`~/.local/share/lexicon/dictionary.sqlite`). Override it with
`dictionary.database_path`:

```sh
lex config set dictionary.database_path /absolute/path/to/en.sqlite
```

The installer validates a supplied file, copies it to the default location
unless it is already there, and writes the configuration. It refuses to replace
an existing dictionary unless `--force` is passed.

## Schema

The database is opened read-only with a busy timeout. The core tables are:

```sql
CREATE TABLE dictionary_info (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL,
    source_language TEXT NOT NULL,
    target_language TEXT NOT NULL,
    license TEXT NOT NULL DEFAULT '',
    attribution TEXT NOT NULL DEFAULT ''
);

CREATE TABLE entries (
    id INTEGER PRIMARY KEY,
    word TEXT NOT NULL COLLATE NOCASE,
    transliteration TEXT NOT NULL DEFAULT '',
    pronunciation_respelling TEXT NOT NULL DEFAULT '',
    pronunciation_ipa TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    license TEXT NOT NULL DEFAULT '',
    UNIQUE (word)
);

CREATE TABLE word_forms (
    form TEXT NOT NULL COLLATE NOCASE,
    entry_id INTEGER NOT NULL REFERENCES entries (id) ON DELETE CASCADE,
    kind TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (form, entry_id)
);

CREATE TABLE definitions (
    id INTEGER PRIMARY KEY,
    entry_id INTEGER NOT NULL REFERENCES entries (id) ON DELETE CASCADE,
    part_of_speech TEXT NOT NULL DEFAULT 'other',
    definition TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    sense_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE entry_metadata (
    entry_id INTEGER NOT NULL REFERENCES entries (id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (entry_id, key)
);

CREATE INDEX idx_entries_word ON entries (word COLLATE NOCASE);
CREATE INDEX idx_entries_transliteration ON entries (transliteration COLLATE NOCASE);
CREATE INDEX idx_definitions_entry ON definitions (entry_id, sense_order);
```

### Required and optional parts

- `entries.id` and `entries.word` are required.
- `entries.transliteration`, `pronunciation_respelling`,
  `pronunciation_ipa`, `source`, and `license` are optional columns. They are
  selected only when present.
- `word_forms`, `definitions`, `entry_metadata`, and `dictionary_info` are
  optional tables for lookup purposes. Missing tables are skipped rather than
  failing a lookup.
- `dictionary_info` is needed for human-readable dictionary metadata. Without it
  `lex doctor` warns; the installer rejects the file because it cannot
  validate a name.
- `definitions` is read with `part_of_speech`, `definition`, `label`, and
  `sense_order`. If you provide that table, use this shape.

This is graceful degradation for the known v3 schema, not a generic schema
mapper.

## Lookup order

Input is first normalized, then resolved case-insensitively in this order:

1. `entries.word = ?`
2. `entries.transliteration = ?` (only when the column exists)
3. `word_forms.form = ?` joined to `entries`, lowest `entries.id` first

Definitions are returned in `sense_order, id` order. Recognized parts of speech
(`noun`, `verb`, `adjective`, `adverb`, `pronoun`, `preposition`,
`conjunction`, `determiner`, `interjection`) are lowercased; anything else is
reported as `other`.

## Normalization

`internal/dictionary.Normalize` mirrors the old `LookupText.cpp` behavior:

- collapse all Unicode whitespace runs to single spaces and trim;
- drop trailing `, ; : ! ?`;
- strip balanced wrapping quotes, brackets, braces, and guillemets;
- strip trailing sentence punctuation and wrapping quote characters;
- preserve a leading apostrophe in contractions (for example `'tis`);
- remove a trailing full stop unless the last token contains another dot, so
  `word.` becomes `word` while `U.S.` stays intact.

It repeats a bounded number of passes to handle nested wrappers such as
`("word").`

`general.max_query_words` (default 4) and `general.max_query_chars` (default
128) are enforced after normalization. Primary-selection reads have their own
limit, `selection.max_chars` (default 128 runes).

## Content and licensing

The reference dictionary is WordNet 3.0, with pronunciation data from the CMU
Pronouncing Dictionary. Its `dictionary_info` row carries the name, language
pair, license, and attribution:

```text
WordNet English Dictionary | en | en | WordNet 3.0 |
WordNet 3.0 Copyright 2006 by Princeton University. All rights reserved.
Pronunciation data from the CMU Pronouncing Dictionary.
```

Dictionary files are user data, never source. They are git-ignored and are not
downloaded by Lexicon at runtime.

## Building the reference dictionary

`tools/dictgen` is the Go importer that produces the bundled English dictionary
from Princeton WordNet 3.0 and the CMU Pronouncing Dictionary. It replaces the
old Python importer, which remains the behavioral reference. The importer is a
build-time tool: it is not installed and the application never invokes it.

Both inputs are downloaded manually; the build does not fetch anything.

| Input | Source | SHA-256 |
| --- | --- | --- |
| WordNet 3.0 (`dict/` directory) | <https://wordnetcode.princeton.edu/3.0/WordNet-3.0.tar.gz> | `640db279c949a88f61f851dd54ebbb22d003f8b90b85267042ef85a3781d3a52` |
| CMUdict (`cmudict.dict`) | <https://raw.githubusercontent.com/cmusphinx/cmudict/master/cmudict.dict> at commit `0f8072f8` (2024-12-17) | `81917843c7f44ce2b094ac63873c2c7a4cf802040792c455ba3ca406891c3d22` |

CMUdict publishes no tags; the file above is the current `master` revision of
the dictionary, which the old documentation referred to as version 0.7b.

```sh
tar xzf WordNet-3.0.tar.gz
just dict-build WordNet-3.0/dict cmudict.dict
```

`dict-build` writes `build/dictionaries/en.sqlite` by default; pass a third
argument to choose another output path. The generated file is git-ignored.

Expected output for the inputs above:

```text
entries         147806
definitions     206918
word_forms      113116   (313 ambiguous forms skipped)
irregular forms   4967
```

The build is deterministic: the same inputs, importer, and SQLite version
produce a byte-identical database. Against the reference Python importer the
logical contents are identical — `sqldiff` reports no changes in any table,
including `sqlite_stat1` and `sqlite_stat4` — but the files are not
byte-identical because the Python and Go toolchains embed different SQLite
point releases (3.54.0 vs 3.54.1) and lay out pages differently. Logical
identity, not byte identity, is the acceptance criterion for the importer.

To exercise the importer against the full corpus:

```sh
LEXICON_TEST_WORDNET_DIR=/path/to/WordNet-3.0/dict \
LEXICON_TEST_CMUDICT=/path/to/cmudict.dict \
go test ./tools/dictgen -run TestBuildFullCorpus -v
```

WordNet is distributed under the WordNet 3.0 license and CMUdict under a
BSD 2-Clause license. Both notices ship in `licenses/` (`WORDNET.txt`,
`CMUDICT.txt`) and must be included wherever the generated dictionary is
redistributed.

### Release assets

`just dict-release WordNet-3.0/dict cmudict.dict` builds the database plus
`manifest.json` and copies both license notices into
`build/dictionaries`. The manifest records the asset name, dictionary version,
byte size, SHA-256, row counts, language pair, and license files. It is
generated from the built file, so the checksum cannot drift from the asset.

The release directory is published as a GitHub release under the
`dictionary-3.0` tag, independently of application tags. Distribution packages
install it to `/usr/share/lexicon/dictionaries/en.sqlite` together with
`manifest.json` and the licenses; see `packaging/README.md`.

The application tarball does not include the dictionary. Downloading a stable
app release alone is therefore not enough for lookups: install a package that
bundles the dictionary, or pass the dictionary asset to the installer
explicitly.

When the installer discovers a packaged copy, it verifies the manifest's size
and checksum before writing the configuration. It points
`dictionary.database_path` at the packaged file instead of copying 47 MB into
each user's home directory. A user-supplied file is still copied to the per-user
data directory, and an explicit `--dictionary-url` requires an explicit
checksum.

## Output

The CLI prints definitions, pronunciation (when enabled), an inflected form when
matched, translation when available, and `source`/`license` metadata. `--json`
prints the daemon's result object, including:

```json
{
  "word": "run",
  "matched_form": "running",
  "transliteration": "",
  "pronunciation": {"respelling": "RUHN", "ipa": "ˈrʌn"},
  "definitions": [
    {"part_of_speech": "verb", "text": "move quickly on foot", "label": ""}
  ],
  "metadata": {"source": "WordNet 3.0", "license": "WordNet 3.0"}
}
```

`show_pronunciation = false` hides pronunciation in the CLI and strips it from
results sent to the popup. JSON lookup output is not filtered.

## Verifying a dictionary

```sh
lex doctor
```

The doctor checks that the path exists, opens read-only, and that
`dictionary_info` is readable. It reports a warning rather than a hard failure
when the metadata is missing.

The core test suite includes an integration test against the real WordNet asset.
It skips when no asset is found. Point it at a specific file with:

```sh
LEXICON_TEST_DICTIONARY=/absolute/path/to/en.sqlite just test-core
```

Without the variable it looks for `internal/dictionary/testdata/en.sqlite`, the
archived development copy, and the installed default path.
