# Installation and service setup

## Prerequisites

- Linux with a Wayland session (`WAYLAND_DISPLAY` set)
- Go 1.27 or newer
- `just` for the build recipes
- Node.js 22 or newer and npm for the popup
- `wl-clipboard` (`wl-paste`) for primary-selection lookups
- `systemctl` and `journalctl` for service lifecycle commands
- For the popup: GTK 4 + WebKitGTK 6.0 **or** GTK 3 + WebKitGTK 4.1 development
  packages, plus `pkg-config` and a C toolchain
- A dictionary SQLite file (see [DICTIONARY.md](DICTIONARY.md)); release
  packages bundle one, or build it with `just dict-build`

Package names vary by distribution. Verify the development packages with:

```sh
pkg-config --exists webkitgtk-6.0 && echo "GTK4/WebKitGTK 6 available"
pkg-config --exists webkit2gtk-4.1 gtk+-3.0 && echo "GTK3/WebKitGTK 4.1 available"
```

At least one line must print for `just build`.

## Build and install

```sh
just core                  # lex CLI, daemon, and installer; no WebKit dependency
just build                 # core + popup (installs npm deps)
just test
just lint
just install               # installs to ~/.local/bin
```

`just install` respects `prefix` and `destdir`:

```sh
just prefix="$HOME/.local" install
```

Installed binaries: `lex`, `lexicon-daemon`, `lexicon-installer`, and
`lexicon-popup`. `lexicon` is installed as a compatibility symlink to `lex`.

Build the optional Bergamot engine into the daemon with:

```sh
just daemon_go_tags=bergamot core
```

This requires the native library and `CGO_*FLAGS` described in
[TRANSLATION.md](TRANSLATION.md).

If `just build` cannot find WebKitGTK 6.0 but finds WebKitGTK 4.1 + GTK 3, the
justfile adds `-tags gtk3`. To force a tag set:

```sh
just popup_go_tags=gtk3 build-popup
just popup_go_tags=gtk3 test-popup
```

The frontend is built into `popup/frontend/dist` and embedded in the popup
binary. `npm ci` is used when `package-lock.json` exists; otherwise the justfile
falls back to `npm install`.

## Run the installer

```sh
lexicon-installer --dictionary /path/to/en.sqlite
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--config PATH` | Configuration file to write |
| `--dictionary PATH` | Source `.sqlite` file to validate and install |
| `--daemon PATH` | `lexicon-daemon` binary used in the systemd unit |
| `--source-lang TAG` | Headword language (default `en`) |
| `--target-lang TAG` | Definition language (default `en`) |
| `--translation` | Enable translation in the config |
| `--no-service` | Do not install a systemd unit |
| `--force` | Replace an existing config or installed dictionary |
| `--yes` | Accept defaults without prompting |
| `--dry-run` | Print the resolved plan and change nothing |

The installer:

1. validates the dictionary by opening it read-only and reading
   `dictionary_info`;
2. writes `$XDG_CONFIG_HOME/lexicon/config.toml`;
3. copies the dictionary to `$XDG_DATA_HOME/lexicon/dictionary.sqlite` unless it
   is already there or a target exists without `--force`;
4. writes a user unit to `$XDG_CONFIG_HOME/systemd/user/lexicon.service`, runs
   `systemctl --user daemon-reload`, and enables/starts it when `systemctl` and
   the `lexicon-daemon` binary are available.

It never writes system-wide files.

## systemd user unit

The generated unit is equivalent to:

```ini
[Unit]
Description=Lexicon selection lookup daemon
Documentation=https://github.com/PandaX185/lexicon
PartOf=graphical-session.target
After=graphical-session.target

[Service]
Type=simple
ExecStart=/home/user/.local/bin/lexicon-daemon
Restart=on-failure
RestartSec=2
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=%t/lexicon
Environment=LEXICON_CONFIG=/home/user/.config/lexicon/config.toml

[Install]
WantedBy=graphical-session.target
```

`packaging/systemd/lexicon.service` is a smaller static example. The generated
unit is authoritative for installer-based setups.

The unit makes only `%t/lexicon` writable. If you set `daemon.socket` somewhere
else, add that socket directory to `ReadWritePaths=` and restart the service.

### Graphical environment

A systemd user manager may not inherit the Wayland environment. The daemon
launches the popup, so the manager needs `WAYLAND_DISPLAY`:

```sh
systemctl --user import-environment WAYLAND_DISPLAY
systemctl --user restart lexicon.service
```

Some desktops export the full session instead:

```sh
dbus-update-activation-environment --systemd WAYLAND_DISPLAY XDG_CURRENT_DESKTOP
```

Without this, lookups and `systemctl --user status lexicon.service` can work
while popup auto-launch fails to create a window.

### Manual service setup

If you did not use the installer:

```sh
mkdir -p ~/.config/systemd/user
cp packaging/systemd/lexicon.service ~/.config/systemd/user/
# Edit ExecStart to the absolute lexicon-daemon path. The static unit uses the
# default config path; add Environment=LEXICON_CONFIG=... for a custom one.
systemctl --user daemon-reload
systemctl --user enable --now lexicon.service
systemctl --user import-environment WAYLAND_DISPLAY
systemctl --user restart lexicon.service
```

## Verify

```sh
lex version
lex doctor
lex lookup hello
lex lookup                    # primary selection
systemctl --user status lexicon.service
journalctl --user -u lexicon.service -f
```

A healthy `doctor` reports `ok` for config, socket, dictionary, selection,
translation state, daemon, and popup. Warnings are not fatal; a failed config,
socket, or dictionary exits non-zero.

## Manual foreground run

For debugging without systemd:

```sh
lexicon-daemon --config ~/.config/lexicon/config.toml --log-level debug
```

Run the CLI against the same configuration and socket in another terminal. If
you use a custom `daemon.socket`, pass the same `--config` to clients or set it
in the shared config file.

## Uninstall

```sh
systemctl --user disable --now lexicon.service
rm ~/.config/systemd/user/lexicon.service
systemctl --user daemon-reload
rm ~/.local/bin/lex ~/.local/bin/lexicon ~/.local/bin/lexicon-daemon \
   ~/.local/bin/lexicon-installer ~/.local/bin/lexicon-popup
```

Configuration, dictionaries, and models are kept under `~/.config/lexicon` and
`~/.local/share/lexicon`; remove them separately if desired. The runtime socket
and lock disappear with the daemon.

## Troubleshooting

**`XDG_RUNTIME_DIR is not set`** — Lexicon refuses to create a socket in a
world-writable fallback. Start it from a real graphical login session. Do not
invent a shared `/tmp` runtime directory.

**`daemon is not running`** — Check `systemctl --user status lexicon.service`
and `journalctl --user -u lexicon.service`. Confirm the client uses the same
configuration/socket as the service. Start it with `systemctl --user start
lexicon.service`, or run `lexicon-daemon --config <path>` in the foreground for
debugging.

**`wl-paste was not found`** — Install `wl-clipboard`. Check
`lex doctor`; `selection.provider = "none"` is valid if you only want
explicit-text lookups.

**Popup does not appear** — Verify `lexicon-popup` is installed next to
`lexicon-daemon`, on `PATH`, or set `popup.binary`. Check the service journal for
`popup launch failed`. Ensure the user manager has `WAYLAND_DISPLAY` as above.
The popup exits after about 18 seconds of failed daemon dials; the next lookup
can relaunch it.

**Popup window fails to create / WebKit errors** — Install the matching GTK and
WebKitGTK development packages and rebuild. Systems with only WebKitGTK 4.1
need `-tags gtk3`.

**Dictionary unavailable** — Run `lex doctor`. The configured path must be a
readable regular file with an `entries` table; the installer additionally needs
`dictionary_info`. Fix the path with `lex config set
dictionary.database_path /absolute/path.sqlite`.

**A configuration change did not apply** — Check the daemon is running and the
CLI reported `daemon reloaded`. Socket changes require
`systemctl --user restart lexicon.service`. Popup window settings apply to the
next popup process.
