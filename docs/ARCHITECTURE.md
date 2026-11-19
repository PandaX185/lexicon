# Architecture

This document describes the Go rewrite's runtime structure and the boundaries
that keep it testable on machines without desktop libraries.

## Components

| Component | Module | Responsibility |
| --- | --- | --- |
| `lexicon` CLI | core | Parse commands, resolve config/socket, render results |
| Daemon (`lexicon-daemon`) | core | Own resources, serve IPC, perform lookups, reload config, launch popup |
| `lexicon-installer` | core | Terminal wizard, validate/copy dictionary, write config, install user unit |
| `lexicon-popup` | `popup/` | Wails window and React UI; subscribes to lookup events |
| Shared packages | core `internal/` | Paths, config, dictionary, selection, translation, IPC, systemd |

`cmd/lexicon` is the CLI entry point and `cmd/lexicon-daemon` is the service
entry point. They are separate binaries: systemd starts `lexicon-daemon`
directly and the CLI has no lifecycle subcommands. The popup is a separate Go
module so the core build never needs WebKitGTK.

## Package boundaries

- `internal/paths` resolves XDG locations and rejects an unset
  `XDG_RUNTIME_DIR` rather than falling back to a world-writable directory.
- `internal/config` defines the typed TOML schema, defaults, validation, atomic
  writes, and dotted-key access used by `lexicon config`.
- `internal/dictionary` normalizes lookup text and reads a SQLite dictionary
  read-only. The schema is probed once at open time; optional columns and tables
  are handled without failing the whole lookup.
- `internal/selection` defines a `Provider` with an injected `Runner`. The only
  current implementation invokes `wl-paste --primary --no-newline` without a
  shell, with a timeout, output cap, UTF-8 validation, and rune limit.
- `internal/translation` resolves model packs, embeds the pinned pack catalog,
  and delegates to an engine. The default engine reports unavailability; the
  `bergamot` build tag selects a C++ bridge kept in
  `internal/translation/bergamotengine`. Pack installs are explicit: HTTPS,
  SHA-256, size, atomic rename.
- `internal/ipc` owns the wire protocol, framing, single-instance socket lock,
  request multiplexing, subscriptions, event replay, and lookup fan-out.
- `internal/daemon` wires the above together and implements the IPC handler.
- `internal/systemd` renders, installs, and enables the user unit through
  `systemctl --user`. Service lifecycle itself stays with systemctl.
- `internal/cli` contains no dictionary logic; it parses commands and calls the
  same packages the daemon uses.
- `internal/installer` composes config, dictionary validation, file copying, and
  systemd installation. `BuildPlan` resolves every action, `Apply` performs it
  with progress and rollback, and `Run` provides the non-interactive path. The
  Bubble Tea UI in `internal/installer/tui` is a thin layer over that API.

## Lookup flow

1. A CLI lookup opens a socket connection, sends a versioned `hello`, then a
   `lookup` request. An empty request text means "read the primary selection".
2. The daemon selects text if needed, normalizes it, and enforces
   `general.max_query_words` and `general.max_query_chars`.
3. The dictionary store tries the headword, transliteration, and inflected form
   in that order. Definitions are read in `sense_order, id` order.
4. If translation is enabled, available, and the active session pair's source
   differs from its target, the daemon attempts a best-effort translation. A
   translation failure is reported separately and never discards the definition.
5. The daemon replies to the requester, remembers found results for replay, and
   broadcasts a lookup event to subscribed popups.
6. If no popup is subscribed and `popup.auto_launch` is enabled, the daemon may
   launch `lexicon-popup --replay` at most once every two seconds. The popup
   connects and displays the replayed result when one is available; a manually
   launched popup has no `--replay` and reads the current selection instead.

The daemon logs whether a lookup succeeded, query length, duration, error class,
and a translation error detail when one occurs. It never logs the query, the
selection, the definition, or the translated text.

## Concurrency and reload

`Daemon` holds a read/write mutex around the mutable resources: configuration,
dictionary store, selection provider, and translation service. A lookup holds
the read lock for its entire duration. `reload` builds new resources first, then
swaps them under the write lock and closes the old ones.

- If rebuilding fails, the old configuration remains active and the IPC caller
  receives an error.
- `daemon.socket` cannot change at runtime; the CLI reports that a restart is
  required.
- The SQLite store uses one open connection to serialize reads and avoid
  surprising locking behavior while another process updates the file.
- The IPC server serializes writes per connection and multiplexes request IDs on
  the client side. A popup's event channel keeps the newest result if the
  frontend is slow.
- The active translation pair is separate from the config-backed resources: the
  daemon guards it with its own mutex, validates `set_language_pair` against
  installed packs, and broadcasts `language_pairs_event`. It is session-only;
  a restart selects the first installed pair. On a change the popup relooks up
  its last query, so the displayed translation follows the selected pair.

## IPC and security

The socket lives in `$XDG_RUNTIME_DIR/lexicon`, created mode `0700`, with the
socket mode `0600`. A sibling lock file (`daemon.sock.lock`) is held with
`flock` for the daemon lifetime, so a second daemon fails cleanly; stale sockets
are removed only after the lock is held. Non-socket paths are never replaced.

Messages are newline-delimited JSON with a protocol version and a 1 MiB frame
cap. The server requires a `hello` within five seconds and accepts only known
client roles. Details are in [IPC.md](IPC.md).

## Process and lifecycle model

- The daemon has no custom supervisor or lifecycle subcommands. systemd owns the
  process: `systemctl --user start|stop|restart|status lexicon.service` and
  `journalctl --user -u lexicon.service`.
- The installer writes a user unit under
  `$XDG_CONFIG_HOME/systemd/user/lexicon.service` and enables it. The unit is
  sandboxed with `NoNewPrivileges`, `PrivateTmp`, `ProtectSystem=strict`,
  `ProtectHome=read-only`, and a writable runtime directory.
- The popup is a normal child process launched with an explicit `--socket` and
  `--config`, so a daemon started with a custom configuration reaches the
  matching instance.
- The popup starts hidden. It shows on a replayed or live lookup result. The
  daemon passes `--replay` on every auto-launch; when the lookup produced a found
  result it is already stored for replay, so that path displays the replay and
  does not look up or translate the same word twice. A manually launched popup
  has no `--replay`: on connect it sends an empty-text lookup even if the welcome
  advertises `has_last_result`, discards the stale replay event, and displays
  the fresh reply. This makes a manually launched popup show the current
  highlighted word without a separate `lexicon lookup`.
  Closing the window, pressing Escape, or losing focus quits the popup process,
  so no popup stays resident after dismissal. Focus loss is ignored only for
  350 ms after the popup is shown; later focus loss quits even when an
  in-webview control has focus. The daemon also keeps a single popup instance:
  each new subscriber is given a sequence number, and older popup subscribers
  receive a `dismiss_popup` IPC message that quits their entire process. The
  legacy `popup.hide_on_blur` key remains
  ignored; Wails `HideOnFocusLost` is not used because it only hides the window
  and is unreliable on Hyprland, Sway, and i3.
- The window is frameless, solid, and resizable by dragging its edges. Appearance
  is read from `[popup]` at startup: one of 20 built-in colour presets (10 dark,
  10 light; `system` follows the desktop light/dark preference) plus optional
  brand/status/footer chrome. No theme files or downloads are involved. Go maps
  the preset to the opaque window base colour to avoid a first-paint flash and
  sends the same values to the frontend over `lexicon:settings`. The result view
  shows the headword, pronunciation, definitions, the translation of the queried
  word (or its error), and then From/To dropdowns for the installed translation
  pairs. Changing a pair relooks up the last query and updates the result in
  place without re-showing the window. The dropdowns are in-webview popovers; a
  native `<select>` would open a separate GTK window and trigger focus loss.
  Window transparency and backdrop blur were deliberately
  omitted: WebKitGTK's `backdrop-filter` cannot reach other X11/Wayland windows,
  so neither has a visible effect.

## Build split

Core (`go test ./...`, `go build ./cmd/...`) has no CGO or desktop dependency.
The popup module adds Wails v3 and WebKitGTK. The justfile picks `-tags gtk3`
when WebKitGTK 6.0 is absent but WebKitGTK 4.1 and GTK 3 are present.

The optional Bergamot engine is the only other native boundary. It is isolated
in a C++ translation unit exposing opaque handles and C strings; C++ exceptions
and types never cross into Go. Only the daemon links it, through the `bergamot`
build tag (`just bergamot-build`, then `just daemon_go_tags=bergamot
build-daemon`); the CLI and installer never need the Bergamot library.

## Deliberate non-goals

The rewrite does not add a generic RPC framework, a second configuration
format, a plugin system, a dictionary engine, an implicit model downloader, a
settings UI, or automatic shortcut registration. These would expand the trusted
and native surface without being required for selection lookup.

Deferred work and open maintainer decisions are tracked in the local, git-ignored
`plan/` directory and summarized in the project handoff report.
