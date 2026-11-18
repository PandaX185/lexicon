import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import type {
  ConnectionState,
  DisplaySettings,
  Entry,
  LookupResult,
} from "./types";

const LOOKUP_EVENT = "lexicon:lookup";
const CONNECTION_EVENT = "lexicon:connection";
const SETTINGS_EVENT = "lexicon:settings";
const READY_EVENT = "lexicon:ready";
const HIDE_EVENT = "lexicon:hide";

export default function App() {
  const [result, setResult] = useState<LookupResult | null>(null);
  const [connection, setConnection] = useState<ConnectionState | null>(null);
  const [settings, setSettings] = useState<DisplaySettings | null>(null);

  useEffect(() => {
    const stopLookup = Events.On(LOOKUP_EVENT, (event) => {
      setResult(event.data as LookupResult);
    });
    const stopConnection = Events.On(CONNECTION_EVENT, (event) => {
      setConnection(event.data as ConnectionState);
    });
    const stopSettings = Events.On(SETTINGS_EVENT, (event) => {
      const value = event.data as DisplaySettings;
      setSettings(value);
      document.documentElement.dataset.theme = value.theme;
    });

    void Events.Emit(READY_EVENT, null);

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        void Events.Emit(HIDE_EVENT, null);
      }
    };
    window.addEventListener("keydown", onKeyDown);

    return () => {
      stopLookup();
      stopConnection();
      stopSettings();
      window.removeEventListener("keydown", onKeyDown);
    };
  }, []);

  const showHeader = settings?.show_header === true;
  const showStatus = settings?.show_status === true;
  const showFooter = settings?.show_footer === true;

  return (
    <main className="popup-card flex h-full flex-col text-ink">
      {showHeader ? (
        <header className="flex shrink-0 items-center justify-between border-b border-line px-5 py-2.5">
          <div className="flex items-center gap-2">
            <BookIcon />
            <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
              Lexicon
            </span>
          </div>
          {showStatus ? <ConnectionBadge connection={connection} /> : null}
        </header>
      ) : null}

      <section
        aria-live="polite"
        className="min-h-0 flex-1 overflow-y-auto px-6 py-5 text-[13px] select-text"
      >
        {result ? (
          result.found && result.entry ? (
            <ResultView result={result} entry={result.entry} />
          ) : (
            <EmptyResult result={result} />
          )
        ) : (
          <IdleState connection={connection} />
        )}
      </section>

      {showFooter ? (
        <footer className="flex shrink-0 items-center justify-between gap-4 border-t border-line px-5 py-2 text-[10px] text-faint">
          <span className="truncate">
            {lastLookupLabel(result) ?? "Offline dictionary"}
          </span>
          <kbd className="shrink-0 rounded border border-line px-1 py-0.5 font-sans text-[9px] text-muted">
            Esc
          </kbd>
        </footer>
      ) : null}
    </main>
  );
}

function ResultView({ result, entry }: { result: LookupResult; entry: Entry }) {
  const { respelling, ipa } = entry.pronunciation;
  const metadata = entry.metadata ?? {};
  const hasMetadata = Boolean(metadata.source || metadata.license);

  return (
    <div className="flex flex-col gap-5">
      {result.translation ? (
        <div className="rounded-xl border border-accent/20 bg-accent-soft px-4 py-3">
          <div className="mb-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-accent">
            Translation
          </div>
          <p className="text-[17px] leading-snug font-medium text-ink" dir="auto">
            {result.translation}
          </p>
        </div>
      ) : null}

      {result.translation_error ? (
        <p className="text-[11px] leading-relaxed text-faint">
          Translation unavailable: {result.translation_error}
        </p>
      ) : null}

      <div>
        <div className="flex flex-wrap items-baseline gap-x-2.5 gap-y-1">
          <h1
            className="font-serif text-[28px] leading-tight font-medium tracking-tight text-ink"
            dir="auto"
          >
            {entry.word}
          </h1>
          {entry.transliteration && entry.transliteration !== entry.word ? (
            <span className="text-sm text-muted" dir="auto">
              {entry.transliteration}
            </span>
          ) : null}
        </div>

        {respelling || ipa ? (
          <p className="mt-1 flex flex-wrap items-center gap-x-2 text-xs text-muted">
            {respelling ? <span>/{respelling}/</span> : null}
            {respelling && ipa ? <span className="text-faint">·</span> : null}
            {ipa ? <span className="font-mono">{ipa}</span> : null}
          </p>
        ) : null}

        {entry.matched_form ? (
          <p className="mt-1 text-[11px] text-faint">
            matched form{" "}
            <span className="text-muted" dir="auto">
              {entry.matched_form}
            </span>
          </p>
        ) : null}
      </div>

      {entry.definitions.length > 0 ? (
        <ol className="flex flex-col gap-3">
          {entry.definitions.map((definition, index) => (
            <li key={index} className="flex gap-3">
              <span
                aria-hidden="true"
                className="mt-[7px] h-1.5 w-1.5 shrink-0 rounded-full bg-accent/70"
              />
              <div className="min-w-0">
                <div className="mb-0.5 flex flex-wrap items-center gap-2">
                  <span className="text-[10px] font-semibold uppercase tracking-[0.12em] text-accent">
                    {labelPartOfSpeech(definition.part_of_speech)}
                  </span>
                  {definition.label ? (
                    <span className="rounded bg-surface-soft px-1.5 py-0.5 text-[10px] text-muted">
                      {definition.label}
                    </span>
                  ) : null}
                </div>
                <p className="text-[13px] leading-relaxed text-ink">
                  {definition.text}
                </p>
              </div>
            </li>
          ))}
        </ol>
      ) : (
        <p className="text-xs text-faint">No definitions are available.</p>
      )}

      {hasMetadata ? (
        <p className="text-[10px] text-faint">
          {[metadata.source, metadata.license].filter(Boolean).join(" · ")}
        </p>
      ) : null}
    </div>
  );
}

function EmptyResult({ result }: { result: LookupResult }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 px-4 text-center">
      <div className="mb-1 rounded-full bg-surface-soft p-3 text-muted">
        <SearchIcon />
      </div>
      <h2 className="text-sm font-semibold text-ink">
        {errorTitle(result.error_kind)}
      </h2>
      <p className="max-w-[36ch] text-xs leading-relaxed text-muted">
        {result.error ?? "The dictionary returned no result."}
      </p>
    </div>
  );
}

function IdleState({ connection }: { connection: ConnectionState | null }) {
  const disconnected = connection?.connected === false;
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 px-4 text-center">
      <div className="relative mb-2 flex h-3 w-3 items-center justify-center">
        <span
          className={`absolute inline-flex h-full w-full rounded-full ${
            disconnected ? "bg-red-400/20" : "animate-ping bg-accent/25"
          }`}
        />
        <span
          className={`relative inline-flex h-1.5 w-1.5 rounded-full ${
            disconnected ? "bg-red-400" : "bg-accent"
          }`}
        />
      </div>
      <h2 className="text-sm font-semibold text-ink">
        {connection === null
          ? "Connecting to Lexicon…"
          : disconnected
            ? "Daemon unavailable"
            : "Listening for a selection"}
      </h2>
      {disconnected ? (
        <>
          <p className="max-w-[38ch] break-words text-xs leading-relaxed text-muted">
            {connection.error ?? "The daemon is not reachable."}
          </p>
          <p className="text-[11px] text-faint">
            Retrying, or start it with{" "}
            <code className="text-muted">systemctl --user start lexicon</code>
          </p>
        </>
      ) : (
        <p className="max-w-[36ch] text-xs leading-relaxed text-muted">
          Select text, then run <code className="text-ink">lex lookup</code>.
        </p>
      )}
    </div>
  );
}

function ConnectionBadge({ connection }: { connection: ConnectionState | null }) {
  const state =
    connection === null ? "connecting" : connection.connected ? "online" : "offline";
  const copy = {
    connecting: "connecting",
    online: "online",
    offline: "offline",
  }[state];
  const dot = {
    connecting: "bg-accent",
    online: "bg-emerald-400",
    offline: "bg-red-400",
  }[state];

  return (
    <span className="flex items-center gap-1.5 text-[10px] text-faint">
      <span className={`h-1.5 w-1.5 rounded-full ${dot}`} />
      {copy}
    </span>
  );
}

function errorTitle(kind: LookupResult["error_kind"]): string {
  switch (kind) {
    case "selection":
      return "Nothing selected";
    case "validation":
      return "Selection can't be looked up";
    case "not_found":
      return "No entry found";
    case "dictionary":
      return "Dictionary unavailable";
    default:
      return "Lookup failed";
  }
}

function labelPartOfSpeech(value: string): string {
  if (!value) {
    return "other";
  }
  return value.replaceAll("_", " ");
}

function lastLookupLabel(result: LookupResult | null): string | null {
  if (!result) {
    return null;
  }
  const time = new Date(result.at);
  if (Number.isNaN(time.getTime())) {
    return null;
  }
  return `Looked up at ${time.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  })}`;
}

function BookIcon() {
  return (
    <svg
      aria-hidden="true"
      className="h-3.5 w-3.5 text-accent"
      fill="none"
      stroke="currentColor"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="1.6"
      viewBox="0 0 24 24"
    >
      <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
      <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2Z" />
    </svg>
  );
}

function SearchIcon() {
  return (
    <svg
      aria-hidden="true"
      className="h-5 w-5"
      fill="none"
      stroke="currentColor"
      strokeLinecap="round"
      strokeWidth="1.6"
      viewBox="0 0 24 24"
    >
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.5-3.5" />
    </svg>
  );
}
