# Privacy

Lexicon is an offline dictionary. It does not include telemetry, analytics, or
automatic crash reporting. Nothing about your usage is sent anywhere unless you
explicitly install a language pack or translation model over HTTPS.

## What stays on your machine

| Data | Location | Notes |
| --- | --- | --- |
| Configuration | `~/.config/lexicon/config.toml` | Edited in the settings window or by hand. |
| Lookup cache | In memory | Bounded; never persisted. |
| Installed dictionaries | `~/.local/share/lexicon/dictionaries/` | Read locally by SQLite. |
| Crash reports | `~/.local/state/lexicon/` | Written only after a fatal crash. |

`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, and `XDG_STATE_HOME` relocate these
directories.

## Crash reports

On a fatal signal (`SIGSEGV`, `SIGABRT`, `SIGBUS`, `SIGILL`, `SIGFPE`), Lexicon
writes one report to `~/.local/state/lexicon/lexicon-crash-<pid>.log`. The report
is created with owner-only permissions (`0600`) and contains:

- the Lexicon version,
- the signal name and number,
- the process ID,
- a stack trace with function names, addresses, and the executable or library
  paths the unwinder reports.

The report is written directly by the signal handler and is never uploaded. It
is only shared if you choose to attach it to a bug report.

### What crash reports never contain

Lexicon does not read the clipboard, so it cannot record it. Crash handling
deliberately excludes:

- the selected text or any lookup query,
- dictionary contents or definitions,
- the configuration file,
- environment variables,
- the working directory or any file Lexicon has opened,
- usernames, hostnames, or other identifiers.

The unwinder may name the executable and shared libraries involved in the stack
trace, but Lexicon does not add any other paths.

## Deleting local data

Crash reports are ordinary files. Delete them at any time:

```sh
rm -rf ~/.local/state/lexicon
```

Normal operation never creates a report, so the directory may not exist. See the
[user guide](user-guide.md) for a full uninstall checklist.
