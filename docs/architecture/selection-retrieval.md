# Selection retrieval on Wayland

## Decision

Stage 1 does **not** implement automatic selected-text retrieval.

`KdeSelectionProvider::currentSelection()` returns an explicit
`ErrorCode::SelectionUnavailable` result. It must not silently fall back to the clipboard,
reuse stale text, or inject keystrokes. A dictionary lookup based on the wrong text is
worse than a clear failure.

## Findings

The target session is KDE Plasma 6.8.2 on Wayland with KWin 6.8.2.

- `wl-paste --primary --list-types` works on the target session. A primary selection
  exists and can be read by a Wayland client.
- KWin advertises compositor data-control support, including
  `ext_data_control_manager_v1` and `zwlr_data_control_manager_v1`.
- Klipper's D-Bus interface exposes clipboard history, not the Wayland primary
  selection. It is not a correct source for selected text.
- Qt's public clipboard API does not expose a portable, always-available primary
  selection reader on Wayland.

## Why this is not a one-line clipboard read

"Selected text" and "clipboard" are different concepts:

1. Selecting text with the mouse usually updates the primary selection, not the
   clipboard.
2. Reading the clipboard would return whatever was last copied, which may be unrelated.
3. Synthesizing `Ctrl+C` requires input injection and permission, can disturb focus, and
   is unreliable under Wayland security policies.
4. Toolkits do not expose selection ownership/retrieval uniformly across compositors.

## Proof-of-concept plan

The next stage should implement a small, isolated Wayland client behind the existing
`SelectionProvider` interface.

1. Connect to the session Wayland display.
2. Bind `ext_data_control_manager_v1` when available.
3. Fall back to `zwlr_data_control_manager_v1` only if needed for KWin versions that do
   not expose the standardized protocol.
4. Request the current primary selection, read the offered MIME types, and accept only
   plain text (`text/plain;charset=utf-8`, `text/plain`, or `UTF8_STRING`).
5. Return an explicit error when no protocol, device, or text selection is available.
6. Keep the transport behind `SelectionProvider` so the rest of the application remains
   compositor-agnostic.

Do not make the main application depend on Qt Wayland private headers if a small
dedicated helper can do the job. A helper must still report failures explicitly and must
not log selected text.

## Runtime limits

- Cap accepted selection length before lookup (for example, 128 characters or a single
  line for the popup lookup path).
- Trim surrounding whitespace.
- Never send selection text to a network service.
- Never shell-interpolate selection text.
- Read the selection only after an explicit user action.

## Acceptance criteria for Stage 2

- A single word selected in Kate, Firefox, and a Qt application is retrieved correctly on
  KWin Wayland.
- A clipboard-only copy does not trigger a lookup.
- No selection, non-text selection, and unsupported protocol each produce a clear,
  separate error.
- The implementation degrades to the same explicit error on X11 if selection support is
  not yet wired there.
- Selected text is absent from debug logs.
