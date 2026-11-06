# Dictionary development fixture

`dev-seed.sql` contains a tiny, original English test fixture. It is **not** a dictionary dataset and
must not be shipped as one.

- Every definition was written for this project and is dedicated to the public domain under CC0-1.0.
- The fixture is intentionally small and covers multiple parts of speech, multiple senses, and metadata.
- Tests create a temporary SQLite database from `schema.sql` and `dev-seed.sql`.
- `just seed-db` creates a local development database at `build/dev/lexicon.sqlite`.

The real offline dictionary resource is a later milestone. Any future dataset must have its license
reviewed and recorded before it is added to the repository or a package.
