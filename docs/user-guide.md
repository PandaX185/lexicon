# Lexicon user guide

Lexicon is an offline English dictionary that runs in the background on KDE Plasma. It
does not open a main window: a global shortcut or the tray icon shows the definition
popup, and the settings window is where you change how it behaves.

This guide is for people installing and using Lexicon. If you want to build or modify
it, see [CONTRIBUTING.md](../CONTRIBUTING.md) and the
[architecture overview](architecture/overview.md).

## Requirements

- KDE Plasma 6 on Wayland.
- A dictionary database. The Arch package installs the bundled WordNet 3.0 dictionary;
  source builds can install it with `just install-dictionary`.
- Optional: `python` and `argostranslate` for offline translation between languages.

Selected-text lookup uses the Wayland `ext_data_control_manager_v1` protocol, which KWin
supports. On X11 Lexicon reports that selection lookup is unsupported; it never falls
back to the clipboard or sends synthetic key presses.

## Install

### Arch Linux (AUR)

The packaging repository contains two PKGBUILDs. If `lexicon` is available from the AUR:

```sh
yay -S lexicon          # latest release
yay -S lexicon-git      # tracks the main branch
```

Until the AUR package is published, build it from a checkout:

```sh
git clone https://github.com/PandaX185/lexicon.git
cd lexicon/packaging/arch
makepkg -si
```

`makepkg` verifies the release tarball's SHA-256 before building. Optional dependencies:

| Package | Provides |
| --- | --- |
| `python` | Runs the Argos Translate worker used for translation. |
| `python-argostranslate` | The offline translation models and library. |

### Release install tarball

Each GitHub release publishes two archives:

- `lexicon-<version>-x86_64-install.tar.gz` — the installed files, for users.
- `lexicon-<version>.tar.gz` — the source tree, for packagers.

Download the install archive and its `.sha256` file from the
[releases page](https://github.com/PandaX185/lexicon/releases), verify it, and extract it
at the filesystem root:

```sh
sha256sum -c lexicon-0.1.0-x86_64-install.tar.gz.sha256
sudo tar -C / -xzf lexicon-0.1.0-x86_64-install.tar.gz
```

### From source

```sh
git clone https://github.com/PandaX185/lexicon.git
cd lexicon
cmake --preset dev-release
cmake --build --preset dev-release
sudo cmake --install build/dev-release --prefix /usr
just install-dictionary
```

The CMake requirements are listed in the [README](../README.md#requirements).

## First run

Start Lexicon from the application launcher or run `lexicon` in a terminal. It appears in
the system tray and registers the global shortcut; no window opens.

On first start, if the dictionary directory is empty, Lexicon copies the installed
dictionary to:

```text
~/.local/share/lexicon/dictionaries/en.sqlite
```

If you see **No dictionary installed**, install the package data (Arch) or run
`just install-dictionary` (source), then restart Lexicon.

To start Lexicon automatically with your Plasma session, open the settings window and
enable **Start on login**. This writes a per-user autostart entry; it is not enabled by
default.

## Looking up a word

### Selected text

1. Select a word or short phrase in any application.
2. Press the global shortcut, **`Super+Shift+D`** by default.

The popup appears near the cursor with the definition. Surrounding quotes, brackets, and
sentence punctuation are ignored, and runs of whitespace are collapsed. Selections longer
than 128 characters are rejected instead of looked up.

A selection of more than four words is treated as a sentence, not a headword, and is
rejected with a message. Select a single word or a short phrase instead.

### Inflected forms

Past tense, past participle, gerund, plural, and comparative forms resolve to their
base word automatically. Selecting `went` shows the entry for `go`, and the popup makes
the substitution visible:

```text
went → go
```

Exact headwords still win: `running` has its own noun entry, so it is shown as `running`
rather than redirected to `run`. A form that could belong to more than one base word is
not guessed.

The shortcut is registered with KGlobalAccel and appears under **Lexicon** in
**System Settings → Shortcuts**. If another application already owns the combination,
assign a different one there, or change [`hotkeys.lookup`](configuration.md#hotkeys) in
the configuration file and restart Lexicon.

### Tray icon

- **Left click** — open the settings window.
- **Middle click** — look up the current selection.
- **Right click → Quit** — stop Lexicon.

### Command line

```sh
lexicon --lookup serendipity
lexicon --lookup serendipity --database ~/dictionaries/en.sqlite
lexicon --list-dictionaries
lexicon --debug
```

| Option | Effect |
| --- | --- |
| `-l`, `--lookup <word>` | Start Lexicon and show the entry for `<word>`. |
| `-c`, `--config <path>` | Read configuration from `<path>` instead of the default. |
| `--database <path>` | Use one specific dictionary file. |
| `--list-dictionaries` | Print installed dictionaries and exit. |
| `--debug` | Enable debug logging to stderr. |
| `-h`, `--help` / `-v`, `--version` | Show help or the version. |

`--list-dictionaries` prints one line per dictionary: name, language pair, entry count,
and file path.

### Closing the popup

- **Escape** closes the popup while it has focus.
- **Clicking inside the popup** (including its margin) closes it.
- Clicks elsewhere on the desktop are not detected yet, so the popup can stay open until
  Escape, a click inside it, or another lookup.

## Settings

Open the settings window from the tray icon, the application launcher, or by running
`lexicon` while an instance is already running (the running instance shows its settings).

Changes are saved immediately. Shortcut, dictionary, cache, and theme changes take effect
after restarting Lexicon; the other options apply at once.

| Section | What you can change |
| --- | --- |
| **Status** | Read-only summary of the active dictionary and shortcut. |
| **Interaction** | Dismiss on Escape, dismiss on outside click. |
| **Appearance** | Theme (dark, light, system), popup width, animation, popup position. |
| **Language packs & translation** | Install or remove Argos language pairs; set the pack catalog URL. |
| **Dictionary** | Show pronunciation, and choose respelling or IPA. |
| **Startup and performance** | Start on login, enable the lookup cache, and set its size. |

Saving rewrites `config.toml` from the known schema. If the previous file contained
comments or keys Lexicon does not recognize, they cannot be preserved: the original is
copied to `config.toml.bak` first and the discarded items are listed in the log.

## Configuration file

The default location is:

```text
~/.config/lexicon/config.toml
```

`--config PATH` overrides it, and `LEXICON_CONFIG` is used when no `--config` argument is
given. A missing file is fine; built-in defaults are used. Invalid values log a warning
and keep their defaults, and a malformed TOML file makes the whole configuration fall back
to defaults.

The complete key list is in the [configuration reference](configuration.md). A commented
example is available at
[`resources/config/config.example.toml`](../resources/config/config.example.toml).

A minimal configuration that changes the shortcut and uses IPA:

```toml
[hotkeys]
lookup = "SUPER+ALT+D"

[dictionary]
pronunciation_style = "ipa"
```

`SUPER` is normalized to Meta. See [IPA pronunciation](pronunciation-ipa.md) for what the
two pronunciation styles contain.

## Dictionaries

Dictionaries are SQLite files in:

```text
~/.local/share/lexicon/dictionaries/
```

Lexicon picks one by the configured language pair. With the default
`source_language = "en"` and `target_language = "en"` it uses the monolingual English
dictionary. To use another language, put a compatible dictionary file in that directory
and set the pair in the configuration:

```toml
[dictionary]
source_language = "de"
target_language = "en"
```

Resolution order is: an exact pair, then a monolingual dictionary for the source
language, then one for the target language. If no matching dictionary is installed,
Lexicon reports `database_unavailable` instead of answering in the wrong language.

Run `lexicon --list-dictionaries` to see what is installed. Adding dictionary content is
covered in the [dictionary format](dictionary-format.md) documentation.

## Offline translation

Translation is optional and local. It only runs when the source and target languages
differ, for example:

```toml
[dictionary]
source_language = "en"
target_language = "de"
```

Install the runtime and a language pair:

```sh
python3 -m pip install --user argostranslate
```

Then open **Settings → Language packs & translation**. Installed pairs are listed there.
To download packs from a catalog, set
[`translation.catalog_url`](configuration.md#translation) to an HTTPS URL first; nothing
is fetched until you press **Install**. Downloads are verified against the catalog's
SHA-256 before they are installed.

A missing Python runtime or a missing pair never breaks a lookup: the definition is shown
without the translation. See [offline translation](argos-translation.md) and
[language packs](language-packs.md) for details.

## Files and locations

| Path | Purpose |
| --- | --- |
| `~/.config/lexicon/config.toml` | Configuration. |
| `~/.config/lexicon/config.toml.bak` | Backup written when a save would drop comments or unknown keys. |
| `~/.local/share/lexicon/dictionaries/` | Installed dictionaries. |
| `~/.config/autostart/io.github.PandaX185.Lexicon.desktop` | Per-user autostart entry, when enabled. |
| `/usr/share/lexicon/dictionaries/en.sqlite` | Dictionary installed by the package. |
| `/usr/share/lexicon/scripts/argos_worker.py` | Translation worker used by the optional Argos runtime. |

`XDG_CONFIG_HOME` and `XDG_DATA_HOME` change the two base directories.

## Troubleshooting

### The shortcut does nothing

- Confirm Lexicon is running: the tray icon should be present.
- Check **System Settings → Shortcuts → Lexicon** for a conflict and assign another
  combination.
- On X11, selection lookup is unsupported. Log out and choose a Wayland Plasma session.

### The popup does not appear for a selection

- The selection must be at most 128 characters.
- The source application must expose the Wayland data-control selection. Some sandboxed
  or non-native applications do not.
- Lexicon never falls back to the clipboard, so copying the text and pressing the
  shortcut is not expected to work.

### "No dictionary installed"

Install the package data or run `just install-dictionary` for a source build. Check that
`~/.local/share/lexicon/dictionaries/` contains a `.sqlite` file, and use
`lexicon --list-dictionaries` to see whether Lexicon recognizes it.

### Translations are missing

- Confirm `source_language` and `target_language` differ.
- Confirm `python3` can import `argostranslate`:
  `python3 -c "import argostranslate"`.
- Confirm the pair is installed in **Settings → Language packs & translation**.

### The settings window is not visible

Lexicon has no main window by design. Open the settings from the tray icon or the
application launcher; if the tray icon is missing, start `lexicon --debug` in a terminal
and check the log for `tray_unavailable`.

### Collecting logs

Run Lexicon in a terminal with `--debug` and reproduce the problem; diagnostics are
written to stderr. When Lexicon is started by the session, the same messages go to the
journal, for example:

```sh
journalctl --user -b | grep -i lexicon
```

## Uninstall

Remove the package with your package manager, then optionally delete user data:

```sh
rm -rf ~/.config/lexicon ~/.local/share/lexicon
rm -f ~/.config/autostart/io.github.PandaX185.Lexicon.desktop
```

Removing `~/.config/lexicon` also removes your configuration; removing
`~/.local/share/lexicon` deletes the copied dictionary and any downloaded language packs.
