# Lexicon

**Look it up. Stay in your flow.**

Lexicon is a lightweight, offline dictionary for Linux Wayland desktops. Select a word, press your keyboard shortcut, and get its definition in an instant.

No browser tabs. No accounts. No cloud services. Just your dictionary, right where you need it.

<!-- Add a real popup screenshot or demo GIF here -->

## Features

- **Instant lookups** — Look up highlighted text using the Wayland primary selection.
- **Native popup** — Read definitions in a small desktop popup without leaving your current application.
- **Terminal support** — Use `lexicon lookup` directly from your terminal, including JSON output.
- **Built-in English dictionary** — WordNet 3.0 definitions with pronunciations from the CMU Pronouncing Dictionary.
- **Offline by design** — Dictionary lookups happen locally. No telemetry or cloud dependency.
- **Optional translation** — Add offline translation language packs when you need them.
- **Simple desktop integration** — Use your desktop's keyboard shortcut settings. Lexicon doesn't install its own global shortcut manager.

Lexicon only reads the Wayland primary selection. Your clipboard is never read or modified.

## Get started

On a Wayland desktop, install Lexicon using the bootstrap installer:

```sh
bash <(curl -fsSL https://github.com/PandaX185/lexicon/releases/download/v0.1.5/install.sh)
```

The bootstrap verifies the release tarball, checksums, and pinned dictionary, installs the binaries into `~/.local/bin` (or `$XDG_BIN_HOME`), then launches the terminal installer. No root access is required. Run it in an interactive terminal so the installer can prompt you (piping through `sh` will exit early); if `~/.local/bin` is not on your `PATH`, the script tells you how to add it.

Once installed, run:

```sh
lexicon-installer
```

The installer checks your system, configures Lexicon, installs the dictionary, and sets up the background service.

Check your installation:

```sh
lexicon doctor
```

See the [installation guide](docs/INSTALL.md) for package installation, manual installation, and distribution-specific instructions.

## Using Lexicon

### Look up a word

```sh
lexicon lookup hello
```

### Look up highlighted text

Highlight a word in any supported Wayland application, then run:

```sh
lexicon lookup
```

### Use the popup

Bind the following command to a keyboard shortcut in your desktop's keyboard settings:

```sh
lexicon lookup
```

The popup appears with the definition of your current selection.

You can also launch the popup directly:

```sh
lexicon-popup
```

### Machine-readable output

```sh
lexicon lookup --json hello
```

### Available commands

| Command | Description |
| --- | --- |
| `lexicon lookup [word]` | Look up a word or current selection |
| `lexicon lookup --json [word]` | Return the result as JSON |
| `lexicon doctor` | Diagnose your installation |
| `lexicon config show` | Show effective configuration |
| `lexicon config set KEY VALUE` | Change a setting |
| `lexicon config edit` | Edit configuration directly |
| `lexicon languages list` | List translation packs |
| `lexicon languages install ID` | Install a translation pack |
| `lexicon languages use SRC-TGT` | Validate an installed translation pair |
| `lexicon languages remove ID` | Remove a translation pack |
| `lexicon version` | Show the installed version |

## Offline translation

Lexicon can optionally provide offline machine translation using Bergamot.

The translation engine is included in the release daemon, while language models are installed separately. Translation is disabled by default.

See [Offline translation](docs/TRANSLATION.md) for supported languages, model installation, and configuration.

## Requirements

- Linux running Wayland (X11 is not currently supported).
- `wl-clipboard` for reading the primary selection.
- systemd user manager for the background service.
- An English dictionary, included in release packages or installed separately.

Explicit lookups such as `lexicon lookup hello` can work without the background service or selection backend.

## Installation options

### Release installer

The recommended option for most users:

```sh
bash <(curl -fsSL https://github.com/PandaX185/lexicon/releases/download/v0.1.5/install.sh)
```

### Release tarball

Release assets are published as:

```text
lexicon-<version>-linux-<arch>.tar.gz
checksums.sha256
```

Verify the archive before extracting it:

```sh
grep "lexicon-0.1.5-linux-amd64.tar.gz" checksums.sha256 | sha256sum -c -
tar -xzf lexicon-0.1.5-linux-amd64.tar.gz
```

The English dictionary is distributed as a separate pinned asset. Follow the [installation guide](docs/INSTALL.md#install-from-a-release) for the complete procedure.

### From source

Requirements:

- Go 1.27 or newer
- `just`
- Node.js 22 or newer
- npm
- `wl-clipboard`
- GTK and WebKit development packages for the popup

```sh
git clone https://github.com/PandaX185/lexicon.git
cd lexicon

just build
just install
```

The source checkout does not include the dictionary. Download `en.sqlite` from the [dictionary release](https://github.com/PandaX185/lexicon/releases/tag/dictionary-3.0) and provide it during installation.

See [docs/INSTALL.md](docs/INSTALL.md) for distribution-specific dependencies and manual installation instructions.

## Background service

Lexicon uses a systemd user service to handle lookups and communicate with the popup.

Check its status:

```sh
systemctl --user status lexicon.service
```

Restart it:

```sh
systemctl --user restart lexicon.service
```

Follow its logs:

```sh
journalctl --user -u lexicon.service -f
```

If the service cannot access your Wayland session, import the display environment and restart it:

```sh
systemctl --user import-environment WAYLAND_DISPLAY
systemctl --user restart lexicon.service
```

## Configuration

Lexicon stores its configuration in:

```text
~/.config/lexicon/config.toml
```

You can modify settings through the CLI:

```sh
lexicon config show
lexicon config set KEY VALUE
lexicon config edit
```

The dictionary is stored at:

```text
~/.local/share/lexicon/dictionary.sqlite
```

Packaged installations may use a system-wide dictionary under `/usr/share/lexicon` instead.

See [Configuration](docs/CONFIGURATION.md) for all available settings, defaults, and allowed values.

## Troubleshooting

Start with:

```sh
lexicon doctor
```

It checks your configuration, service, socket, dictionary, selection backend, translation models, and popup.

Common issues:

**The daemon is not running**

```sh
systemctl --user start lexicon.service
```

Alternatively, run `lexicon-daemon` in a terminal to inspect its output.

**Highlighted text isn't detected**

Make sure `wl-clipboard` is installed and run `lexicon doctor`. Explicit lookups still work without the selection backend.

**The dictionary is missing**

Rerun the installer and provide the dictionary file:

```sh
lexicon-installer --dictionary ~/Downloads/en.sqlite
```

For more troubleshooting information, see [docs/INSTALL.md](docs/INSTALL.md).

## Documentation

- [Installation](docs/INSTALL.md)
- [Configuration](docs/CONFIGURATION.md)
- [Offline translation](docs/TRANSLATION.md)
- [Dictionary](docs/DICTIONARY.md)
- [Contributing](CONTRIBUTING.md)
- [Security policy](SECURITY.md)

## License

Lexicon is released under the MIT License.

The dictionary and translation models have their own licenses and attribution requirements. Refer to the metadata distributed with each asset for details.
