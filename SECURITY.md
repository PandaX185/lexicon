# Security policy

## Reporting a vulnerability

Do not open a public issue for a vulnerability. Use the repository's private
vulnerability reporting channel when available. Include the affected version,
reproduction steps, and impact. This project does not currently have a dedicated
security mailbox.

## Security model

Lexicon is a per-user, offline-first desktop tool. It reads the Wayland primary
selection, looks words up in a local SQLite dictionary, and pushes the result to
a local popup over a Unix domain socket.

- **No network access at runtime.** The scaffold does not download dictionaries,
  models, or updates. Anything that later fetches content must use HTTPS and be
  explicitly user-initiated.
- **Selected text is never logged.** The daemon logs counts and error classes,
  not selection contents.
- **Local IPC only.** The socket is created in `$XDG_RUNTIME_DIR/lexicon` with
  directory mode `0700` and socket mode `0600`. Protocol frames are bounded.
- **No clipboard reads.** Only the Wayland primary selection is read, via
  `wl-paste --primary`, and only when the user requests a selection lookup.
- **No shell interpolation.** Subprocesses receive argument vectors.
- **SQL uses bound parameters.** Dictionary files are opened read-only.
- **Config is trusted user input.** The daemon validates it and refuses to start
  on invalid values rather than silently falling back.

## Scope

- Dictionary content and translation models are user-supplied data and are not
  trusted as executable input.
- The optional Bergamot bridge is native code compiled into the binary; build it
  only from a reviewed source revision (see `docs/TRANSLATION.md`).
- This project is Linux/Wayland-focused; other platforms are out of scope.

## Supported versions

The project is pre-1.0. Security fixes target the current development branch.
