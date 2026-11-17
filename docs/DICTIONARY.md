# Dictionary format

Lexicon reads dictionaries directly from SQLite. It does not rebuild or modify
the database, and it does not bundle one. The expected format is the SQLite v3
schema used by the earlier Lexicon application.

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
