# IPC protocol

The daemon and its clients communicate over a per-user Unix domain socket using
versioned newline-delimited JSON (NDJSON). This is a private local protocol, not
a public API, but it is documented because the popup and CLI are separate
processes and third-party clients can be written against it.

## Transport

- Socket: `$XDG_RUNTIME_DIR/lexicon/daemon.sock` unless `daemon.socket` is set.
- Directory mode `0700`; socket mode `0600`.
- One JSON object per line. The newline is the frame delimiter.
- Maximum frame size is 1 MiB (`MaxFrame = 1 << 20`), including the newline.
- The server allows five seconds for the initial `hello`; it bounds writes at
  two seconds. The client uses a three-second dial/handshake timeout and a
  fifteen-second request cap.
- The socket lock is `daemon.sock.lock`. A second daemon fails with
  `another lexicon daemon is already running`.

## Versions and roles

`v` is the protocol version. It is currently `1`. A message with any other
version is rejected. Missing optionals are allowed; unknown fields are ignored.

| Role | Meaning |
| --- | --- |
| `cli` | Sends lookups/status/reload and expects replies |
| `popup` | Subscribes to lookup events; can also send requests |
| `probe` | Short-lived health/configuration checks |

Only `popup` connections are added to the subscriber set. Every accepted client
receives a `welcome` message.

## Envelope

```json
{
  "v": 1,
  "type": "lookup",
  "id": 7,
  "role": "cli",
  "text": "hello",
  "result": { "...": "..." },
  "status": { "...": "..." },
  "error": { "code": "reload", "message": "..." },
  "subscribers": 1
}
```

Unused fields are omitted. `id` correlates requests with replies and is zero
for unsolicited events.

## Message types

| Type | Direction | Purpose |
| --- | --- | --- |
| `hello` | client → server | First message; declares role |
| `welcome` | server → client | Handshake accepted; includes subscriber count and `has_last_result` |
| `lookup` | client → server | Resolve text, or the primary selection when text is empty |
| `lookup_result` | server → client | Reply to a lookup request |
| `lookup_event` | server → popup | Result pushed after a lookup; replayed on subscribe |
| `status` | client → server | Health/configuration query |
| `status_result` | server → client | Reply with `Status` |
| `reload` | client → server | Re-read config and swap resources |
| `ack` | server → client | Reload succeeded |
| `ping` / `pong` | both | Liveness check |
| `language_pairs` | client → server | Query installed packs and the active session pair |
| `language_pairs_result` | server → client | Reply with the `language_pairs` state |
| `set_language_pair` | popup → server | Validate and activate a pair for the daemon session |
| `language_pairs_event` | server → popup | Broadcast the state after a pair change |
| `error` | server → client | Protocol, role, or request failure |

## Handshake

```json
{"v":1,"type":"hello","role":"cli"}
{"v":1,"type":"welcome","subscribers":0,"has_last_result":false}
```

The server rejects a bad version, a non-`hello` first message, or an unknown
role. After `welcome`, a `popup` immediately receives the most recent **found**
lookup result as a `lookup_event`, if one exists. Not-found and failed lookups
are not replayed. `has_last_result` tells a newly connected popup whether that
replay is coming; when it is false, the popup sends an empty-text `lookup` so
the daemon reads the current primary selection. Older daemons omit the field,
which is treated as false.

## Lookup

Request:

```json
{"v":1,"type":"lookup","id":1,"text":"running"}
```

An empty or omitted `text` asks the daemon to read the Wayland primary
selection. Validation, selection, dictionary, and not-found failures are normal
lookup results, not transport errors:

```json
{
  "v": 1,
  "type": "lookup_result",
  "id": 1,
  "result": {
    "query": "running",
    "normalized": "running",
    "found": true,
    "entry": {
      "word": "run",
      "matched_form": "running",
      "pronunciation": {"respelling": "RUHN", "ipa": "ˈrʌn"},
      "definitions": [
        {"part_of_speech": "verb", "text": "move at a speed faster than a walk"}
      ]
    },
    "at": "2026-11-17T12:00:00Z"
  }
}
```

`lookup_result` is sent to the requester first, then a `lookup_event` with the
same `result` is sent to every subscribed popup:

```json
{"v":1,"type":"lookup_event","result":{"query":"running","found":true,"..." : "..."}}
```

Failure shape:

```json
{
  "v": 1,
  "type": "lookup_result",
  "id": 2,
  "result": {
    "query": "asdf",
    "normalized": "asdf",
    "found": false,
    "error": "No entry found for \"asdf\".",
    "error_kind": "not_found",
    "at": "2026-11-17T12:00:00Z"
  }
}
```

Error kinds:

| Kind | Meaning |
| --- | --- |
| `validation` | Empty, too many words, too long, or otherwise invalid query |
| `selection` | No selection, non-UTF-8 data, timeout, or `wl-paste` failure |
| `not_found` | Valid query with no matching headword or form |
| `dictionary` | Missing/unreadable database or query failure |
| `internal` | Reserved for unexpected daemon failures |

Translation is best-effort. A successful result may also contain
`translation`; a failed translation adds `translation_error` without changing
`found`.

## Status

```json
{"v":1,"type":"status","id":3}
```

```json
{
  "v": 1,
  "type": "status_result",
  "id": 3,
  "status": {
    "version": "0.1.0-dev",
    "pid": 4242,
    "uptime_seconds": 120,
    "subscribers": 1,
    "protocol_version": 1,
    "dictionary_path": "/home/user/.local/share/lexicon/dictionary.sqlite",
    "dictionary_name": "WordNet English Dictionary",
    "selection": "wl-paste",
    "translation": "disabled",
    "socket": "/run/user/1000/lexicon/daemon.sock"
  }
}
```

Status never includes selected text or query contents.

## Language pairs

Daemons that support translation answer a `language_pairs` request with the
installed pairs sorted by source and target code:

```json
{
  "v": 1,
  "type": "language_pairs_result",
  "id": 5,
  "language_pairs": {
    "enabled": true,
    "available": true,
    "source": "en",
    "target": "de",
    "pairs": [
      {"source": "en", "target": "de", "pack_id": "bergamot-en-de-tiny11"},
      {"source": "en", "target": "es", "pack_id": "bergamot-en-es-tiny11"}
    ]
  }
}
```

Only `popup` clients may send `set_language_pair`; other roles receive an
`error` with code `role`. A valid request names an installed pair:

```json
{"v":1,"type":"set_language_pair","id":6,"source_language":"en","target_language":"es"}
```

The daemon replies with `language_pairs_result` and broadcasts the same state as
a `language_pairs_event` (id `0`) to subscribed popups. The active pair is held
in daemon memory for the current session: it is never written to configuration,
and a restart falls back to the first installed pair. After a change, the popup
relooks up its last query so the displayed translation uses the new pair.

## Reload

```json
{"v":1,"type":"reload","id":4}
{"v":1,"type":"ack","id":4}
```

The daemon reloads the configuration and rebuilds the dictionary, selection,
and translation resources. On failure it returns:

```json
{
  "v": 1,
  "type": "error",
  "id": 4,
  "error": {"code": "reload", "message": "config file not found: ..."}
}
```

Changing `daemon.socket` is rejected; restart the service with `systemctl --user
restart lexicon.service`. `lexicon config set` and `config reset` send a reload
automatically after writing the file.

## Errors and compatibility

- `error.code` values include `handshake`, `protocol_version`, `unknown_type`,
  and `reload`.
- Malformed JSON, a frame over the limit, or a truncated frame ends the
  connection.
- Unknown message types receive an `error` but the connection stays open.
- A duplicate `hello` receives an `error`.
- Clients must ignore unknown JSON fields. New optional fields may be added
  without a version bump; framing or semantic changes require a new `v`.
