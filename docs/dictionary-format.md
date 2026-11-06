# Dictionary format

Lexicon reads a local SQLite database with this schema:

- `entries`: one row per headword, with optional respelling, IPA, source, and license.
- `definitions`: one row per sense, linked to an entry, with part of speech, text,
  optional label, and sense order.
- `entry_metadata`: arbitrary key/value metadata for an entry.

The schema is in [`data/dictionaries/schema.sql`](../data/dictionaries/schema.sql).

## Lookup behavior

- Lookup is case-insensitive (`COLLATE NOCASE`).
- The first matching entry is used.
- Definitions are ordered by `sense_order`, then `id`.
- A missing database, failed query, and missing word are distinct errors.

## Development fixture

[`dev-seed.sql`](../data/dictionaries/dev-seed.sql) contains a small CC0 fixture for
smoke tests only. It is not a dictionary and must not be presented as one.

```sh
just seed-db
```

## Production content

A future release needs a separately distributed dictionary with a compatible license and
attribution. The repository must not silently download dictionary content or merge
incompatibly licensed data.
