# Lexicon

Lexicon is an offline dictionary for Wayland desktops. Highlight a word, press
your shortcut, and read the definition in a small popup. Everything stays on
your machine: no account, no network requests, and no telemetry.

## What you get

- **Look up the text you selected.** Lexicon reads the Wayland primary
  selection, so the word you highlighted is the word it looks up.
- **Definitions in a popup or the terminal.** Use `lex lookup` for a quick
  answer, or let the popup appear next to your work.
- **A real dictionary, included.** Release packages ship an English dictionary
  built from WordNet 3.0, with pronunciations from the CMU Pronouncing
  Dictionary.
- **No background clutter.** One systemd user service does the work. There is no
  tray icon, no settings window, and no global shortcut registration — you bind
  the shortcut in your desktop, where it belongs.
- **Optional offline translation.** Translation can be built in for language
  pairs you have model packs for. It is off by default.

Lexicon only reads the primary selection. It never reads or changes your
clipboard.

## Install

### From a package

If your distribution provides a Lexicon package, install it with your package
manager. The package includes the English dictionary and the systemd user
service.

### From source

You need Go 1.27 or newer, `just`, Node.js 22 or newer, npm, `wl-clipboard`, and
GTK/WebKit development packages for the popup. See
[docs/INSTALL.md](docs/INSTALL.md) for distribution-specific package names.

```sh
git clone https://github.com/PandaX185/lexicon.git
cd lexicon
just build
just install          # installs to ~/.local/bin
```

The source checkout does not contain the dictionary. Download `en.sqlite` from
the [dictionary release](https://github.com/PandaX185/lexicon/releases/tag/dictionary-3.0)
and pass it to the installer, or use a packaged copy.

## Set up

Run the installer once. It checks your system, writes the configuration,
installs the dictionary, and starts the background service.

```sh
lexicon-installer
```

If the dictionary is not packaged, point the installer at the file you
downloaded:

```sh
lexicon-installer --dictionary ~/Downloads/en.sqlite
```

Then let the systemd user service see your Wayland session, so the popup can
open:

```sh
systemctl --user import-environment WAYLAND_DISPLAY
systemctl --user restart lexicon.service
```

Check that everything is ready:

```sh
lex doctor
```

`lex doctor` reports the configuration, socket path and permissions, dictionary,
selection backend, service state, model directory, translation state, and popup.
Fix anything marked `fail` before continuing.

## Use it

Look up a word you type:

```sh
lex lookup hello
```

Look up whatever you have highlighted:

```sh
lex lookup
```

Ask for machine-readable output:

```sh
lex lookup --json hello
```

To use the popup, bind `lex lookup` to a keyboard shortcut in your desktop's
keyboard settings. Any shortcut works; Lexicon does not register one itself.
You can also launch `lexicon-popup` directly: on startup it asks the daemon to
read the current highlight and shows the definition right away.

### Everyday commands

| Command | What it does |
| --- | --- |
| `lex lookup [word]` | Look up a word, or the current selection |
| `lex lookup --json [word]` | Print the full result as JSON |
| `lex doctor` | Check the installation and show what is wrong |
| `lex config show` | Show the effective configuration |
| `lex config set KEY VALUE` | Change one setting |
| `lex config edit` | Edit the configuration file directly |
| `lex languages list` | Show installed and available translation packs |
| `lex languages install ID` | Download and install a translation pack |
| `lex languages use SRC-TGT` | Set the translation pair |
| `lex languages remove ID` | Remove an installed translation pack |
| `lex version` | Show the version |

### Service commands

The background service is a normal systemd user unit:

```sh
systemctl --user status lexicon.service
systemctl --user restart lexicon.service
journalctl --user -u lexicon.service -f
```

## Configuration

The installer writes `~/.config/lexicon/config.toml`. You can change settings
with `lex config set` or by editing the file. Every key, its default, and its
allowed range are documented in [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

The dictionary lives in `~/.local/share/lexicon/dictionary.sqlite` unless a
package provides it under `/usr/share/lexicon`, in which case Lexicon uses the
packaged copy in place.

## Requirements

- A Wayland session (X11 is not supported)
- `wl-clipboard` for reading the primary selection
- systemd user manager for the background service (optional if you only use
  `lex lookup` with explicit text)
- An English dictionary: included in packages, or downloaded separately

## Troubleshooting

**The popup does not appear.** Give the user service your Wayland display and
restart it:

```sh
systemctl --user import-environment WAYLAND_DISPLAY
systemctl --user restart lexicon.service
```

Then check `journalctl --user -u lexicon.service` for errors.

**`lex` says the daemon is not running.** Start the service with
`systemctl --user start lexicon.service`, or run `lexicon-daemon` in a terminal
to see its log.

**Selection lookups do nothing.** Install `wl-clipboard` and run `lex doctor`.
Explicit lookups such as `lex lookup hello` still work without it.

**The dictionary is missing.** Run `lex doctor`, then rerun the installer with
`--dictionary /path/to/en.sqlite`.

More help is in [docs/INSTALL.md](docs/INSTALL.md).

## Learn more

- [Installation and service setup](docs/INSTALL.md)
- [Configuration reference](docs/CONFIGURATION.md)
- [Dictionary details](docs/DICTIONARY.md)
- [Translation](docs/TRANSLATION.md)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)

## License

Lexicon is MIT licensed. The dictionary and translation models keep their own
licenses and attribution; see the metadata stored with those files.
