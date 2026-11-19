# Configuration

Lexicon uses one TOML file. The CLI is the only supported editing interface; the
popup has no settings screen.

## Location and precedence

1. `--config PATH` (global option)
2. `LEXICON_CONFIG` environment variable
3. `$XDG_CONFIG_HOME/lexicon/config.toml`, falling back to
   `~/.config/lexicon/config.toml`

A missing file is valid and yields the defaults. An existing file must parse and
pass validation, and unknown keys are rejected. The layout is additive: missing
keys take their defaults, so there is no schema version stored in the file.

The path values in the file are not shell-expanded (`~`, `$VAR`). Use absolute
paths. `popup.binary` additionally accepts a leading `~/`.

## Commands

```sh
lexicon config path          # print the resolved path
lexicon config show          # canonical TOML with effective XDG values
lexicon config get KEY       # print one effective value
lexicon config set KEY VALUE # validate, write, and notify a running daemon
lexicon config reset [--yes] # restore defaults
lexicon config edit          # open the raw file in $VISUAL/$EDITOR
```

`set` and `reset` rewrite the file canonically, so comments are not preserved.
Use `config edit` when comments or hand formatting matter. After a successful
write, the CLI asks the daemon to reload. If the daemon is not running, the
change applies on its next start. A changed `daemon.socket` cannot be applied by
reload; restart the service with `systemctl --user restart lexicon.service`.

## Installer-managed versus advanced settings

The terminal installer manages only:

- `popup.theme`
- `dictionary.show_pronunciation`
- `popup.show_header`, `popup.show_status`, `popup.show_footer`
- `general.max_query_words`, `general.max_query_chars`

Everything else — sockets, paths, timeouts, selection tuning, and binary
overrides — remains in `lexicon config` and `lexicon config edit`.

The active translation pair is daemon session state, not a config key. Change it
with the **From** and **To** dropdowns under the popup definition. Changing
**From** limits **To** to the targets installed for that source and selects the
first one; the translation updates in place without reopening the popup. Use
`lexicon languages use SRC-TGT` to validate an installed pair; it points to that
control and writes no config. The choice resets to the first installed pair when
the daemon restarts.
Old files containing the removed `translation.source_language` and
`translation.target_language` keys still load; the keys are accepted and
ignored.

## Keys

| Key | Type | Default | Validation |
| --- | --- | --- | --- |
| `general.max_query_words` | integer | `4` | 1–16 |
| `general.max_query_chars` | integer | `128` | 1–512 |
| `daemon.socket` | string | empty → `$XDG_RUNTIME_DIR/lexicon/daemon.sock` | non-empty effective path; absolute recommended |
| `daemon.log_level` | string | `info` | `debug`, `info`, `warn`/`warning`, or `error` |
| `selection.provider` | string | `auto` | `auto`, `wl-paste`, or `none` |
| `selection.timeout_ms` | integer | `400` | 50–5000 |
| `selection.max_chars` | integer | `128` | 1–512 |
| `dictionary.database_path` | string | empty → `$XDG_DATA_HOME/lexicon/dictionary.sqlite` | readable regular SQLite file at use time |
| `dictionary.source_language` | string | `en` | lowercase BCP 47-shaped tag |
| `dictionary.target_language` | string | `en` | lowercase BCP 47-shaped tag |
| `dictionary.show_pronunciation` | boolean | `true` | — |
| `translation.enabled` | boolean | `false` | — |
| `translation.model_directory` | string | empty → `$XDG_DATA_HOME/lexicon/models` | directory scanned at use time |
| `translation.timeout_seconds` | integer | `30` | 5–300 |
| `popup.auto_launch` | boolean | `true` | — |
| `popup.binary` | string | empty → next to `lexicon-daemon`, then `PATH` | executable path checked at launch |
| `popup.width` | integer | `420` | 240–1200 logical pixels |
| `popup.height` | integer | `320` | 160–1200 logical pixels |
| `popup.theme` | string | `system` | see the theme list below |
| `popup.show_header` | boolean | `false` | show the brand header |
| `popup.show_status` | boolean | `false` | show the daemon connection badge |
| `popup.show_footer` | boolean | `false` | show the timestamp and close hint |

Notes:

- `selection.provider = "auto"` uses `wl-paste` when it is installed and
  degrades to a reported runtime error otherwise. `"none"` disables selection
  lookups; explicit text lookups still work.
- `selection.max_chars` is the rune limit for primary-selection text.
  `general.max_query_chars` is the separate limit after normalization.
- Language tags are stored lowercase. Common aliases (`jp`, `ger`, `fre`, …) are
  canonicalized when translation runs, but the config validation expects a
  lowercase tag.
- `translation.enabled = true` on a default build is accepted; the daemon
  reports the translator as unavailable and lookups still return definitions.
- Install translation packs with `lexicon languages`, then select a pair in the
  popup. `lexicon languages use SRC-TGT` only validates the pair and points to
  the popup control. The active pair is session-only and independent of the
  dictionary pair; no language keys are written to the config file.
- The popup is frameless but resizable. Its result card shows the headword,
  pronunciation, definitions, the translation of the queried word (or its
  error), **From**/**To** dropdowns for the installed translation pairs, and
  source metadata. Selecting a pair relooks up the last query and updates the
  translation in place. Closing the window, pressing Escape, or losing focus
  quits the popup process. Focus loss is ignored only for the first 350 ms after
  the popup appears; later focus loss dismisses it even when a dropdown is open.
  Only one popup is kept: when a newer popup connects, the daemon asks every
  older popup to quit its whole process, so an earlier window never lingers.
- The old `popup.hide_on_blur` key is accepted in existing files and ignored;
  it is no longer written to new configurations. Focus loss always quits the
  popup, rather than just hiding its window.
- `popup.*` display values are read when the popup process starts. A running
  popup does not hot-reload them; the next popup launch uses the new values.
- Theme presets are compiled into the popup; there are no theme files or
  downloads. `system` follows `prefers-color-scheme` and updates live when the
  desktop switches between light and dark. Values are case-insensitive.
- The popup window is always solid. There are no `opacity` or `blur` keys:
  WebKitGTK's `backdrop-filter` cannot blur content from other X11/Wayland
  windows, so a translucency/blur option would have no visible effect.
- Built-in themes (10 dark, 10 light, plus `system`):
  - Dark: `dark`, `nord`, `catppuccin`, `gruvbox`, `rose-pine`, `dracula`,
    `tokyo-night`, `one-dark`, `solarized-dark`, `github-dark`
  - Light: `light`, `nord-light`, `catppuccin-latte`, `gruvbox-light`,
    `rose-pine-dawn`, `solarized-light`, `github-light`, `one-light`,
    `tokyo-night-light`, `everforest-light`
- With the generated systemd unit, the only writable path is `%t/lexicon`. A
  custom `daemon.socket` outside that directory needs a matching
  `ReadWritePaths=` entry, or the service cannot create the socket.

## Example

A commented copy of this example is tracked at `config.example.toml`.
`just run-daemon-dev` runs the daemon against it with debug logging and an
isolated runtime directory, so it does not collide with an installed service.

```toml
# Lexicon configuration. Edit with `lexicon config set` or `lexicon config edit`.

[general]
max_query_words = 4
max_query_chars = 128

[daemon]
socket = ""
log_level = "info"

[selection]
provider = "auto"
timeout_ms = 400
max_chars = 128

[dictionary]
database_path = ""
source_language = "en"
target_language = "en"
show_pronunciation = true

[translation]
enabled = false
model_directory = ""
timeout_seconds = 30

[popup]
auto_launch = true
binary = ""
width = 420
height = 320
theme = "system"
show_header = false
show_status = false
show_footer = false
```

An empty string means "use the XDG default". `lexicon config show` prints the
effective values instead of the empty placeholders.
