# Language packs

Translation uses Argos Translate models (`.argosmodel` archives). Lexicon does not define
its own model format: a pack is exactly the archive Argos installs. Because models are
large and only some users need a given pair, they are downloaded on request rather than
bundled with the dictionary.

Nothing downloads until the user presses **Install** in *Settings → Language packs &
translation*, and the catalog is only fetched when it is configured:

```toml
[translation]
catalog_url = "https://example.org/lexicon/language-packs.json"
```

An empty `catalog_url` (the default) disables downloads entirely. Installed packs still
work offline and are still listed in the settings window.

## Catalog format

The catalog is a static JSON document; it needs no server logic, so it can be a plain file
on a release host or object storage.

```json
{
  "version": 1,
  "packs": [
    {
      "id": "de-en",
      "source": "de",
      "target": "en",
      "version": "1.9",
      "url": "https://example.org/lexicon/packs/de-en_1.9.argosmodel",
      "sha256": "9f1c…64-hex-characters…",
      "size": 104857600,
      "description": "German to English, Argos Translate 1.9."
    }
  ]
}
```

| Field | Required | Notes |
| --- | --- | --- |
| `version` | yes | Catalog schema version. Documents newer than 1 are ignored. |
| `packs[].id` | yes | Unique, stable identifier; also the settings-window key. |
| `packs[].source`, `packs[].target` | yes | BCP 47 language codes matching the Argos pair. |
| `packs[].url` | yes | HTTPS URL of the `.argosmodel` file. Redirects must not downgrade the transport. |
| `packs[].sha256` | yes | Lowercase hex digest of the archive. Case is normalized on read. |
| `packs[].version`, `size`, `description` | no | Shown in the settings window. |

A pack missing `id`, `source`, `target`, `url`, or a 64-character `sha256` is dropped
silently, so one bad entry cannot break the list. Entries whose `url` is not HTTPS are
dropped as well. The catalog URL itself must be HTTPS; requesting an insecure catalog
fails before any network traffic is sent.

## Install flow

1. `LanguagePackManager` downloads the archive to `$TMPDIR/lexicon-language-packs/`,
   emitting `downloadProgress` while it goes.
2. The file is hashed and compared to the catalog digest. A mismatch deletes the file and
   emits `operationError`; the model is never handed to the runtime.
3. On a match, `LanguagePackController` calls `install_model` on the worker in a
   `QtConcurrent` task, passing the verified local path.
4. The worker runs `argostranslate.package.install_from_path()`. The temporary archive is
   removed afterwards either way, and the installed-pair list is refreshed.

Removal goes the other way: `uninstall_model` asks Argos to uninstall the pair and the
settings list refreshes. Lexicon never deletes files under the Argos data directory
itself.

Downloads run on the GUI thread's `QNetworkAccessManager`; only the model install and the
pair listing use the worker thread. `LanguagePackManager` never touches `QSqlDatabase`.

## Hosting packs

Any static host works. To publish new packs:

```sh
sha256sum de-en_1.9.argosmodel
```

and add the digest to the catalog. Because verification happens on the client, the catalog
is the trust anchor: serve it over HTTPS and keep it in the same release as the packs it
references. Packs must not be re-uploaded in place under the same URL; publish a new
version and new digest instead.

## Privacy

Downloading a pack is a plain HTTPS GET for a fixed URL. No lookup text, selected word, or
usage data is transmitted, and installing a pack never changes what the application logs
about selections.

## Tests

`tests/unit/test_language_pack_manager.cpp` covers catalog parsing (including invalid
entries and a too-new schema), checksum verification, and progress reporting without
network access.
