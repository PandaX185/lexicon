# Installation and service setup

## One-line bootstrap (recommended)

~~~sh
curl -fsSL https://github.com/PandaX185/lexicon/releases/download/v0.1.0/install.sh | sh
~~~

The script is fetched from the pinned `v0.1.0` tag, not `main`. It downloads the
release tarball and `checksums.sha256` over HTTPS, verifies the tarball hash,
downloads `en.sqlite` and `manifest.json` from the pinned `dictionary-3.0`
release, verifies both dictionary SHA-256 hashes, extracts the tarball to a
private temporary directory, verifies its internal checksums, and runs
`lexicon-installer` from there. It never uses root or a package manager.

The installer asks for curated appearance settings, offers installable
Bergamot packs, writes configuration, installs the dictionary, enables the
systemd user service when available, and prints CLI usage plus
compositor-specific shortcut guidance. Pack download failures are warnings with
an exact `lex languages install ID` retry command; they do not roll back a
working base install.

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

## Install from a release

Each stable release publishes a checksummed tarball named
`lexicon-<version>-linux-<arch>.tar.gz`, where `<arch>` is `amd64` or `arm64`.
The daemon in the tarball has native Bergamot compiled in.

```sh
version=0.1.0
arch=amd64
base="https://github.com/PandaX185/lexicon/releases/download/v${version}"
curl -fsSL "$base/lexicon-${version}-linux-${arch}.tar.gz" \
  -o "lexicon-${version}-linux-${arch}.tar.gz"
curl -fsSL "$base/checksums.sha256" -o checksums.sha256
grep "lexicon-${version}-linux-${arch}.tar.gz" checksums.sha256 | sha256sum -c -
tar -xzf "lexicon-${version}-linux-${arch}.tar.gz"
cd "lexicon-${version}-linux-${arch}"
sha256sum -c checksums.sha256
```

Then run `./bin/lexicon-installer --yes` with a dictionary (the Arch package
installs one at `/usr/share/lexicon/dictionaries/en.sqlite`), or copy the files
from `bin/` into a directory on your `PATH`. The tarball also carries the
systemd unit, docs, and licenses under `share/`.

The tarball does **not** include the dictionary; it stays a separate pinned
release asset (see [DICTIONARY.md](DICTIONARY.md)).

### Supported distributions and dependency names

The release binaries need only a small set of runtime libraries. Building from
source additionally needs the GTK/WebKit development packages.

| Distribution | Runtime | Popup build: GTK4 / WebKitGTK 6.0 | Popup build: GTK3 / WebKitGTK 4.1 |
| --- | --- | --- | --- |
| Arch / EndeavourOS | `glibc`, `wl-clipboard`, `blas`, `lapack`, `pcre2`, `gcc-libs` | `gtk4`, `webkitgtk-6.0`, `pkgconf` | `gtk3`, `webkit2gtk-4.1`, `pkgconf` |
| Debian 12 / Ubuntu 24.04 | `libc6`, `wl-clipboard`, `libblas3`, `liblapack3`, `libpcre2-8-0`, `libstdc++6` | `libgtk-4-dev`, `libwebkitgtk-6.0-dev`, `pkg-config` | `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `pkg-config` |
| Fedora 40+ | `glibc`, `wl-clipboard`, `blas`, `lapack`, `pcre2`, `libstdc++` | `gtk4-devel`, `webkitgtk6.0-devel`, `pkgconf-pkg-config` | `gtk3-devel`, `webkit2gtk4.1-devel`, `pkgconf-pkg-config` |

Package names can change between distribution releases; the `pkg-config`
checks above are the authoritative test. The release daemon has Bergamot
statically linked in, so the BLAS, LAPACK, PCRE2, and C++ runtime libraries are
its only shared-library dependencies beyond glibc. Model packs are still
installed at runtime with `lex languages install ID`.

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
just bergamot-build
source build/bergamot/env.sh
just daemon_go_tags=bergamot build-daemon
```

The build is pinned to Bergamot v0.4.5 and runs in Docker. Install model packs
afterwards with `lex languages install ID`; see [TRANSLATION.md](TRANSLATION.md).

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
lexicon-installer                 # interactive TUI; uses a packaged dictionary
lexicon-installer --dictionary /path/to/en.sqlite
lexicon-installer --yes           # non-interactive, accept the flag values
```

In a terminal, `lexicon-installer` opens a keyboard-driven wizard:

1. **Welcome** — what the wizard will do.
2. **Preflight** — Wayland session, `wl-paste`, `systemctl`, writable config and
   data directories, the daemon and popup binaries, and a readable dictionary.
   Failures block installation; warnings do not.
3. **Install paths** — the configuration file to write.
4. **Dictionary** — a packaged dictionary is used in place; a local `.sqlite`
   file is validated and copied to `$XDG_DATA_HOME/lexicon/dictionary.sqlite`.
5. **Languages** — headword and definition tags, with optional translation.
6. **Service** — install and start the systemd user service when it is available.
7. **Summary** — every resolved action, including warnings and notes.
8. **Progress and completion** — each step is reported, and failures roll back
   the configuration, dictionary, and unit file before showing the error.

Keys are shown at the bottom of each screen: `enter` advances, `esc` goes back,
and `q` quits. `ctrl+c` is ignored while files are being installed so an
in-progress copy is not interrupted.

If a configuration already exists, the wizard offers to keep it and repair the
service, replace it with new settings, or cancel. Keeping it never rewrites the
file.

### Flags

| Flag | Meaning |
| --- | --- |
| `--config PATH` | Configuration file to write |
| `--dictionary PATH` | Source `.sqlite` file to validate and install |
| `--dictionary-url URL` | Explicit HTTPS dictionary asset (requires `--dictionary-sha256`) |
| `--dictionary-sha256 HEX` | Expected SHA-256 of the local or downloaded dictionary |
| `--daemon PATH` | `lexicon-daemon` binary used in the systemd unit |
| `--model-dir PATH` | Translation model directory (default `$XDG_DATA_HOME/lexicon/models`) |
| `--source-lang TAG` | Headword language (default `en`) |
| `--target-lang TAG` | Definition language (default `en`) |
| `--translation` | Enable translation in the config |
| `--no-service` | Do not install a systemd unit (`--skip-service` is an alias) |
| `--force` | Replace an existing config, dictionary, or unit |
| `--yes` | Run without the TUI and accept the flag values |
| `--dry-run` | Print the resolved plan and change nothing |

`--yes` and `--dry-run` always use the non-interactive text output. Running
without either flag when stdin is not a terminal is an error, so scripts must
choose one explicitly. `--dry-run` never downloads, writes, or starts anything.

`--dictionary-url` is the only network path. It is never used implicitly,
requires HTTPS and an explicit checksum, and is rejected with `--dry-run`
because a dry run must not download anything.

### Re-running

The installer is idempotent. Without `--force` it keeps an existing
configuration and dictionary, and it repairs a missing systemd unit. A unit that
already exists is left in place but started again. Replacing any of these files
requires `--force`.

The installer never writes system-wide files.

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

A healthy `doctor` reports `ok` for config, socket, socket mode, dictionary,
selection, models, translation state, daemon, and popup. Warnings are not fatal;
any failed check exits non-zero.

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
