# Selection retrieval

Stage 2 implements automatic selected-text retrieval on Wayland (KWin,
`ext_data_control_manager_v1`) behind platform-neutral interfaces. X11 and any
other desktop report an explicit unsupported error.

## Layering

Three small pieces keep platform knowledge out of the core:

```text
ApplicationController
        |
        v
SelectionProvider                 (core/interfaces)
        ^
        | implemented by
PrimarySelectionProvider          (core/selection)
   |                |
   |                +--> SelectionTextPolicy        (core/selection)
   +--------------------> PrimarySelectionSource   (core/interfaces)
                              ^
                              | implemented by
                    WaylandDataControlSource       (platform/wayland)
                    X11… (future)                  (platform/x11)
```

- `SelectionProvider` is what the controller uses. It knows nothing about
  display servers.
- `PrimarySelectionSource` is the platform seam. It reports transport facts
  only: unsupported protocol, no selection, no acceptable MIME type, or the raw
  bytes. Implementations must never log the selected bytes.
- `PrimarySelectionProvider` combines a source with `SelectionTextPolicy`, so
  the same interpretation rules apply on every platform.
- `SelectionTextPolicy` owns the application-level rules (MIME preference,
  UTF-8 decoding, trimming, length limit) and is unit-tested without a
  compositor.

`src/app/SelectionProviderFactory.cpp` is the **only** platform-aware code. It
detects the session and picks a source. Adding X11 or another desktop means
adding a `PrimarySelectionSource` implementation and one branch there; the
controller, the presenter, and the UI do not change.

## Wayland backend

`WaylandDataControlSource` connects to the session Wayland display, binds
`ext_data_control_manager_v1` and `wl_seat`, creates a data device, waits for
the compositor to announce the primary selection, chooses the best accepted
MIME type, and reads the offer through a pipe with a bounded timeout.

Details that matter:

- Each call opens a short-lived connection and tears it down afterwards. There
  is no shared mutable client state.
- Accepted MIME types, in preference order: `text/plain;charset=utf-8`,
  `text/plain`, `UTF8_STRING` (compared case-insensitively).
- The read buffer is capped before policy interpretation, so a hostile or
  broken source cannot make the process allocate without bound.
- Errors are explicit and distinct: `SelectionUnavailable` (no display, no
  seat, no primary selection, timeout), `SelectionUnsupported` (protocol
  missing), `SelectionNotText` (MIME type or invalid UTF-8),
  `SelectionTooLong` (over the policy limit).

The protocol XML is vendored in
[`third_party/wayland-protocols`](../third_party/wayland-protocols/README.md)
and generated at build time with `wayland-scanner`.

## X11 decision

Automatic selection retrieval is **not** implemented on X11 in Stage 2.

- An X11 session gets an explicit `SelectionUnsupported` result whose detail
  says the clipboard is not used as a fallback.
- Reading the clipboard would return whatever was last copied, which is not the
  current selection.
- A future `X11PrimarySelectionSource` can implement `PrimarySelectionSource`
  with `XGetSelectionOwner`/`XConvertSelection` (including INCR transfers) and
  be selected in the factory without touching anything else.

## Why not a one-line clipboard read

1. Selecting text with the mouse usually updates the primary selection, not the
   clipboard.
2. Reading the clipboard would return unrelated text, and a dictionary lookup
   based on the wrong text is worse than a clear failure.
3. Synthesizing `Ctrl+C` requires input injection, can disturb focus, and is
   unreliable under Wayland security policies.

Lexicon therefore never falls back to the clipboard, never reuses stale text,
and never injects keystrokes.

## Runtime limits

- `SelectionTextPolicy` accepts at most 128 characters (default) and rejects
  longer selections with `SelectionTooLong`. The limit is applied after
  trimming, so surrounding whitespace does not count.
- Surrounding whitespace is trimmed before lookup.
- Selection text is never sent over the network and never shell-interpolated.
- Selection text is never logged: logs carry only error codes and counts.
- The selection is read only after an explicit user action (shortcut or tray).

## Build configuration

The Wayland backend is optional:

```sh
cmake --preset dev-debug -DLEXICON_WITH_WAYLAND=OFF
```

Without `wayland-client` and `wayland-scanner`, or with the option off, the
backend target is not built and configure still succeeds. At runtime the
factory returns a "backend not built" `SelectionUnsupported` error. This keeps
the selection subsystem swappable and lets other desktops be added without
making Wayland a hard dependency of the whole application.

## Testing

- `tests/unit/test_selection_provider.cpp` covers MIME preference, trimming,
  UTF-8 errors, the length limit, and error propagation with a fake source.
- `tests/unit/test_selection_factory.cpp` pins the degradation contract for
  X11, no-display, and Wayland sessions.
- `tests/integration/test_wayland_selection.cpp` is labelled `kde` and excluded
  from the default CTest preset. It requires a live Wayland session and
  `wl-copy`, and verifies a real selection, no selection, a non-text selection,
  and an over-long selection. Run it with `just test-kde`.

## Acceptance criteria for Stage 2

- [ ] A single word selected in Kate, Firefox, and a Qt application is
  retrieved on KWin Wayland. The live test above covers a real selection; the
  three-application matrix still needs confirmation on the target session.
- [x] A clipboard-only copy does not trigger a lookup: only the primary
  selection is read.
- [x] No selection, non-text selection, unsupported protocol, and over-long
  selection each produce a distinct error.
- [x] X11 degrades to the same explicit error, with a documented decision.
- [x] Selected text is absent from debug logs.

## Known gaps

- Only `ext_data_control_manager_v1` is bound. `zwlr_data_control_manager_v1`
  is not used as a fallback because the target KWin version exposes the
  standard protocol; adding it would be another `PrimarySelectionSource`.
- The backend does not handle incremental (`INCR`) transfers; the text limit
  makes them unnecessary for this use case.
- X11 retrieval and clipboard-content opt-in remain out of scope.
