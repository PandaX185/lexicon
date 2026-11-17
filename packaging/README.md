# Packaging

The source checkout never contains the English dictionary. Packaged releases
consume the dictionary release directory produced by:

```sh
just dict-release WordNet-3.0/dict cmudict.dict
```

That writes four files to `build/dictionaries`:

| File | Purpose |
| --- | --- |
| `en.sqlite` | The dictionary asset |
| `manifest.json` | Name, version, counts, size, SHA-256, license list |
| `WORDNET.txt` | WordNet 3.0 notice |
| `CMUDICT.txt` | CMUdict BSD 2-Clause notice |

The manifest is generated from the built asset, so its checksum always matches.
`dict-build` produces only the database; `dict-release` adds the manifest and
licenses.

## Publishing

The dictionary is versioned independently of the application. Publish the
release directory under a dictionary tag (currently `dictionary-3.0`):

```sh
gh release create dictionary-3.0 build/dictionaries/* \
  --title "English dictionary 3.0" \
  --notes "WordNet 3.0 + CMUdict; see manifest.json for counts and checksums."
```

Packages and the installer pin the asset URL and checksum. Re-publishing an
existing tag is not supported: bump the dictionary version instead.

## Arch package

`arch/PKGBUILD` builds the application from the `v$pkgver` tag and downloads the
dictionary release assets. It installs:

- binaries to `/usr/bin`
- the systemd user unit to `/usr/lib/systemd/user/lexicon.service`
- the dictionary and manifest to `/usr/share/lexicon/dictionaries`
- licenses to `/usr/share/licenses/lexicon`
- documentation to `/usr/share/doc/lexicon`

The `sha256sums` entries for the dictionary assets are pinned to the current
release. Update them together with `dictionary-3.0` when the dictionary changes.

The asset URLs assume the GitHub repository is publicly readable. While it is
private, download the four release assets with an authenticated client (for
example `gh release download dictionary-3.0`) and place them next to the
PKGBUILD before building; `makepkg` will use the local files.

Build with:

```sh
cd packaging/arch
makepkg -si
```

The popup build runs `npm ci` and needs network access on a clean source tree.
Vendoring the frontend dependencies is deferred to the release workflow.

## Installer discovery

When `--dictionary` is omitted, `lexicon-installer` looks for a packaged copy in:

1. `/usr/share/lexicon/dictionaries/en.sqlite`
2. `/usr/share/lexicon/en.sqlite`
3. `/usr/local/share/lexicon/dictionaries/en.sqlite`
4. `/usr/local/share/lexicon/en.sqlite`

If a `manifest.json` sits next to the database, its size and SHA-256 must match
before the installer will point the configuration at it. A packaged dictionary
is referenced in place, never copied into the user's home directory.

An explicit local file is still validated and copied to
`$XDG_DATA_HOME/lexicon/dictionary.sqlite`. An explicit `--dictionary-url`
requires `--dictionary-sha256`, must use HTTPS, and is never used implicitly.
