# Security policy

## Reporting a vulnerability

Do not open a public issue for a vulnerability. Contact the maintainers privately and
include the affected version, reproduction steps, and impact. This project is under
active development and does not currently have a dedicated security mailbox; use the
repository's private vulnerability reporting channel when available.

## Security principles

- Lexicon is offline-first and does not send selected text or lookups to a network
  service. Network access is limited to an explicit, user-initiated language pack
  download over HTTPS.
- Language pack catalogs and archives must use HTTPS; insecure entries are rejected.
- Selected text must never be logged.
- Dictionary paths and configuration paths are treated as local user input.
- Selection retrieval must be explicit and must not silently read the clipboard.
- SQL uses bound parameters.
- Subprocesses must receive arguments as argument vectors, never through a shell.

## Scope

The bundled `data/dictionaries/dev-seed.sql` is a test fixture, not trusted dictionary
content. A future dictionary distribution channel must document provenance and licensing.
