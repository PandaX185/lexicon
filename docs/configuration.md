# Configuration reference

Lexicon reads TOML from:

```text
$XDG_CONFIG_HOME/lexicon/config.toml
```

`--config PATH` overrides the location. `LEXICON_CONFIG` is also honored when no
`--config` argument is supplied. A missing file is not an error; built-in defaults are
used.

Invalid values produce a warning and keep the default. An invalid TOML document produces
an error and the entire configuration falls back to defaults. Unknown keys are reported
but do not fail startup.

## `[general]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `start_on_login` | boolean | `false` | When `true`, the installed autostart entry is used. The application installs the autostart entry to an XDG autostart directory; see packaging. |
| `offline_first` | boolean | `true` | Reserved for future online providers. |

## `[interaction]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `activation` | string | `"global_shortcut"` | Only `global_shortcut` is supported. |
| `clipboard_fallback` | boolean | `false` | Reserved; must remain off for correct selection behavior. |
| `dismiss_on_escape` | boolean | `true` | Controls the Escape shortcut in the popup. |
| `dismiss_on_outside_click` | boolean | `true` | Outside-click dismissal is wired in the popup, but window policy is still Stage 2. |

## `[hotkeys]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `lookup` | string | `"SUPER+SHIFT+D"` | Parsed with `QKeySequence::PortableText`; `SUPER` becomes `Meta`. |

## `[appearance]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `theme` | string | `"dark"` | `dark`, `light`, or `system`. `system` is a supported value but currently behaves like `dark`; desktop theme probing is not implemented yet. |
| `popup_width` | integer | `360` | Clamped to 240–1200. |
| `animation` | string | `"subtle"` | `none` or `subtle`. |
| `position` | string | `"near_selection"` | `near_selection`, `center`, or `cursor`. Selection coordinates are not available yet, so near-selection uses cursor placement. |

## `[dictionary]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `database_path` | string | `""` | Empty means `$XDG_DATA_HOME/lexicon/dictionaries/en.sqlite`. Relative paths resolve against the configuration file directory. `--database PATH` overrides this. |
| `show_pronunciation` | boolean | `true` | Hides the pronunciation line in the popup when `false`. |
| `pronunciation_style` | string | `"respelling"` | Only `respelling` is supported. |

## `[performance]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `cache_enabled` | boolean | `true` | Reserved; lookup caching is not implemented yet and is not wired to any behavior in Stage 1. |
| `lazy_initialize` | boolean | `true` | The popup is created on first use; the flag is preserved for future eager-initialization behavior. |
