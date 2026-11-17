# Lexicon

Lexicon is an offline dictionary for Wayland desktops. It reads the current
primary selection (the text you highlighted), looks it up in a local SQLite
dictionary, and displays the result in a small popup or on the terminal.

This repository is a from-scratch Go rewrite of the earlier C++/Qt application.
It is intentionally smaller in scope:

- a per-user daemon that owns the dictionary, selection provider, optional
  translation engine, and Unix socket;
- a CLI (`lex`) for lookups, configuration, and diagnostics;
- a Wails v3 + React popup that only renders daemon results;
- a terminal installer that writes user configuration and a systemd user unit.

The C++/Qt project is not part of this repository and is not required to build
or run this one.

## Status

The core CLI, daemon, installer, and popup build and pass their test suites.
The default build has no translation engine; Bergamot is an optional,
unverified native build. No dictionary or translation model is bundled.

## How it works

```
primary selection ──wl-paste──┐
                              ▼
  lex lookup ──socket──► daemon ──► read-only SQLite dictionary
                              │
                              └──lookup event──► lexicon-popup (Wails/React)
```

The daemon owns the SQLite handle and exposes a small versioned NDJSON protocol
over `$XDG_RUNTIME_DIR/lexicon/daemon.sock`. The popup subscribes; the CLI sends
requests. Selected text is never logged.

## Requirements

- Linux with a Wayland session
- Go 1.27 or newer
- Node.js 22 or newer and npm (popup only)
- `wl-clipboard` (`wl-paste`) for selection lookups
- systemd user manager (optional, for the background service)
- GTK 4 + WebKitGTK 6 **or** GTK 3 + WebKitGTK 4.1 development packages (popup
  only; the Makefile selects the matching Wails build tag)

## Build and test

```sh
make core          # lex CLI, daemon, and installer; no WebKit dependency
make build         # also builds the popup (frontend dependencies included)
make test
make lint
make fmt-check
```

`make build` runs `npm ci` (or `npm install` when no lockfile exists), builds the
Vite frontend, and then builds the Wails popup. Systems without
`webkitgtk-6.0` but with `webkit2gtk-4.1` + GTK 3 automatically use
`-tags gtk3`. Override the detection with:

```sh
make POPUP_GO_TAGS=gtk3 popup-build
```

Install binaries into `~/.local/bin` (override with `PREFIX`):

```sh
make install
```

## Quick start

1. Build and install the binaries.
2. Run the installer with a dictionary:

   ```sh
   lexicon-installer --dictionary /path/to/en.sqlite
   ```

   The installer validates the database, copies it to
   `$XDG_DATA_HOME/lexicon/dictionary.sqlite`, writes the TOML configuration, and
   optionally installs and starts `lexicon.service` as a systemd user unit.

3. Import the graphical environment into the user manager so the daemon can
   launch the popup:

   ```sh
   systemctl --user import-environment WAYLAND_DISPLAY
   ```

4. Check the installation and start using it:

   ```sh
   lex doctor
   systemctl --user status lexicon.service
   lex lookup hello
   lex lookup            # looks up the current primary selection
   ```

The daemon does not register a global shortcut. Bind `lex lookup` to a key in
your compositor or desktop environment if you want one.

## CLI

```
lex lookup [--json] [--timeout 60s] [text...]
lex config show|get|set|reset|edit|path
lex doctor
lex version

Global option:
  --config PATH    configuration file
```

- `lookup` with no text reads the primary selection. `--json` prints the full
  result envelope payload.
- `config set KEY VALUE` validates and writes the canonical TOML file, then asks
  a running daemon to reload. Changes to `daemon.socket` require a restart.
- `doctor` checks the config, runtime directory, dictionary, `wl-paste`,
  translation availability, daemon reachability, and popup binary.

The daemon has no lifecycle subcommands. Use `systemctl --user
start|stop|restart|status lexicon.service` and `journalctl --user -u
lexicon.service`; the installer writes the unit and starts it.

`bin/lexicon` is installed as a compatibility symlink to `lex`.

See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for every key, its default,
and its range.

## Configuration and data locations

| Purpose | Default |
| --- | --- |
| Configuration | `$XDG_CONFIG_HOME/lexicon/config.toml` (`~/.config/lexicon/config.toml`) |
| Dictionary | `$XDG_DATA_HOME/lexicon/dictionary.sqlite` (`~/.local/share/lexicon/dictionary.sqlite`) |
| Translation models | `$XDG_DATA_HOME/lexicon/models` |
| Daemon socket | `$XDG_RUNTIME_DIR/lexicon/daemon.sock` |
| systemd unit | `$XDG_CONFIG_HOME/systemd/user/lexicon.service` |

`--config PATH` overrides the configuration path; `LEXICON_CONFIG` is also
honored. A missing configuration file is not an error: built-in defaults apply
until the installer or `lex config set` writes one.

## Dictionary and translation

The daemon reads the existing Lexicon SQLite v3 schema directly and opens it
read-only. Lookup order is exact headword, transliteration, then inflected
surface form, all case-insensitive. The reference dictionary is derived from
WordNet 3.0 with pronunciation data from the CMU Pronouncing Dictionary; those
files are user data and are not committed here. See
[docs/DICTIONARY.md](docs/DICTIONARY.md).

Offline translation through Bergamot is compiled out by default. Enabling it
requires rebuilding the daemon with `-tags bergamot` (for example `make
DAEMON_GO_TAGS=bergamot core`) and a Bergamot library, plus an installed model
pack. This path is implemented but has not been built against a real Bergamot
tree. See [docs/TRANSLATION.md](docs/TRANSLATION.md).

## Limitations

- Wayland only; primary selection only. The clipboard is never read.
- No AT-SPI, OCR, clipboard simulation, application integrations, or global
  shortcut registration.
- No dictionary builder, model downloader, updater, tray icon, or GUI settings.
- `wl-paste` is the selection backend. A native `ext-data-control-v1` client is
  deferred.
- Dictionary and model files are supplied by the user.

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [IPC protocol](docs/IPC.md)
- [Configuration](docs/CONFIGURATION.md)
- [Dictionary format](docs/DICTIONARY.md)
- [Translation](docs/TRANSLATION.md)
- [Installation and systemd](docs/INSTALL.md)
- [Contributing](CONTRIBUTING.md)
- [Code of conduct](CODE_OF_CONDUCT.md)
- [Security policy](SECURITY.md)

## License

MIT. The dictionary content and any translation models keep their own licenses
and attribution; see the metadata stored with those files.
