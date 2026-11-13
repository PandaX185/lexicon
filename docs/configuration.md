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

Saving from the settings window rewrites the file with `QSaveFile`, so a failed write
leaves the previous file intact. Because the rewrite is generated from the schema,
comments and unknown keys cannot be preserved: when any are detected, the original file
is copied to `config.toml.bak` and the discarded items are listed in the log.

## `[general]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `start_on_login` | boolean | `false` | Installs or removes a per-user XDG autostart entry under `$XDG_CONFIG_HOME/autostart/`. The system-wide template is never activated automatically. |
| `offline_first` | boolean | `true` | Reserved for future online providers. |

## `[interaction]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `activation` | string | `"global_shortcut"` | Only `global_shortcut` is supported. |
| `clipboard_fallback` | boolean | `false` | Reserved; must remain off for correct selection behavior. |
| `dismiss_on_escape` | boolean | `true` | Controls Escape while the lookup popup has focus. |
| `dismiss_on_outside_click` | boolean | `true` | Dismisses on clicks inside the popup window, including its margin. Clicks outside the popup window are not detected yet. |

## `[hotkeys]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `lookup` | string | `"SUPER+SHIFT+D"` | Parsed with `QKeySequence::PortableText`; `SUPER` becomes `Meta`. |

## `[appearance]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `theme` | string | `"dark"` | `dark`, `light`, or `system`. `system` follows the platform color scheme and falls back to dark when the platform does not report one. |
| `popup_width` | integer | `360` | Clamped to 240–1200. |
| `animation` | string | `"subtle"` | `none` or `subtle`. |
| `position` | string | `"near_selection"` | `near_selection`, `center`, or `cursor`. Selection coordinates are not available yet, so near-selection uses cursor placement. |

## `[dictionary]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `database_path` | string | `""` | An explicit single dictionary file. Empty means the installed dictionaries in `directory` are searched for the configured language pair. Relative paths resolve against the configuration file directory. `--database PATH` overrides this. |
| `directory` | string | `""` | Directory holding installed dictionaries. Empty means `$XDG_DATA_HOME/lexicon/dictionaries`. Relative paths resolve against the configuration file directory. |
| `source_language` | string | `"en"` | Language of the words being looked up, as a BCP 47 tag. `EN`, `en_GB` and `en-GB` are normalized to lowercase. |
| `target_language` | string | `"en"` | Language the definitions should be in. Equal to `source_language` for monolingual definitions. |
| `show_pronunciation` | boolean | `true` | Hides the pronunciation line in the popup when `false`. |
| `pronunciation_style` | string | `"respelling"` | `respelling` or `ipa`. The preferred field is shown, falling back to the other one when empty. See [pronunciation-ipa.md](pronunciation-ipa.md). |

### Choosing a dictionary

`dictionary_path` and the language pair are two ways to pick the same thing. With an
explicit `database_path` (or `--database`) that file is used as-is, which keeps
single-file setups and the development fixture working. Otherwise every `*.sqlite` file in
`directory` is inspected, and the first dictionary admitting to the pair is used:

1. an exact `source_language` to `target_language` dictionary,
2. otherwise a monolingual dictionary for `source_language`,
3. otherwise a monolingual dictionary for `target_language`.

A pair that is not installed produces `database_unavailable` with the pair and the
directory in the message instead of silently answering in the wrong language. Files that
do not carry Lexicon metadata are ignored. `lexicon --list-dictionaries` prints what is
installed.

### Translation

Lookups only produce a translation when `source_language` and `target_language` differ and
an Argos Translate pack for the pair is installed. The translation is added to the entry
as metadata; a failure (missing runtime, missing pack) leaves the definition untouched.
See [argos-translation.md](argos-translation.md) and [language-packs.md](language-packs.md).

## `[translation]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `catalog_url` | string | `""` | Static JSON catalog of downloadable language packs. Empty disables downloads, and only `https://` URLs are accepted; entries with an insecure pack URL are ignored. Packs are only fetched after an explicit install from the settings window. |

## `[performance]`

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `cache_enabled` | boolean | `true` | Enables an in-memory LRU cache for dictionary lookups. Applies after restart. |
| `cache_size` | integer | `256` | Maximum number of cached lookups, clamped to 0–4096; 0 disables caching. |
| `lazy_initialize` | boolean | `true` | The popup is created on first use; the flag is preserved for future eager-initialization behavior. |
