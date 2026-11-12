# Dictionary format

Lexicon reads a local SQLite database with this schema (currently `PRAGMA user_version = 2`):

- `dictionary_info`: exactly one row describing the dictionary and the language pair it
  covers.
- `entries`: one row per headword, with optional Latin transliteration, respelling, IPA,
  source, and license.
- `definitions`: one row per sense, linked to an entry, with part of speech, text,
  optional label, and sense order.
- `entry_metadata`: arbitrary key/value metadata for an entry.

The schema is in [`data/dictionaries/schema.sql`](../data/dictionaries/schema.sql).

> The fixture and the tests apply the schema file statement by statement, splitting on
> `;`. Keep comments in that file free of semicolons.

## Dictionary metadata and language pairs

```sql
INSERT INTO dictionary_info (id, name, source_language, target_language, license, attribution)
VALUES (1, 'Example English', 'en', 'en', 'CC0-1.0', 'Example attribution');
```

- `source_language` is the language of the headwords, `target_language` the language of
  the definitions. A monolingual dictionary repeats the same tag.
- Tags are lowercase BCP 47 (`en`, `de`, `pt-br`). The loader normalizes `EN`, `en_GB`
  and `en-GB`, so write the canonical form.
- A database without a `dictionary_info` row is not a Lexicon dictionary and is ignored by
  the catalog rather than shown to the user.

Having the pair in the file is what makes "any language to any language" a matter of
installed content instead of code: a new pair is a new database in the dictionary
directory, and the configured `dictionary.source_language` / `dictionary.target_language`
select it. See [configuration](configuration.md#choosing-a-dictionary) for the resolution
order.

## Transliteration

`entries.transliteration` holds a Latin-script romanization of a headword written in
another script, for example `zhongwen` for `中文`:

```sql
INSERT INTO entries (word, transliteration) VALUES ('中文', 'zhongwen');
```

Lookups match the headword first and the transliteration second, both case-insensitively,
so a word typed in Latin letters finds an entry it would otherwise miss. The popup shows
the transliteration under the headword. Generating transliterations belongs to the import
step that builds a dictionary, not to the daemon: Lexicon only indexes and reads them.

## Lookup behavior

- Lookup is case-insensitive (`COLLATE NOCASE`) on `word`, then on `transliteration`.
- The first matching entry is used.
- Definitions are ordered by `sense_order`, then `id`.
- A missing database, a file that is not a dictionary, and a missing word are distinct
  errors.

## Development fixture

[`dev-seed.sql`](../data/dictionaries/dev-seed.sql) contains a small CC0 fixture for
smoke tests only. It is not a dictionary and must not be presented as one.

```sh
just seed-db
```

## Production content

The catalog, the metadata, and the transliteration index are in place, but no dictionary
content ships with Lexicon yet. A production dictionary needs a compatible license,
an attribution recorded in `dictionary_info`, and a redistribution route. The repository
must not silently download dictionary content or merge incompatibly licensed data; any
future download has to be an explicit user action with a checksum and the attribution
shown before it is installed.
