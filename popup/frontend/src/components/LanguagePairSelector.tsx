import { useEffect, useMemo, useState } from "react";
import type { LanguagePairsState } from "../types";

interface Props {
  state: LanguagePairsState;
  error: string;
  onChange: (source: string, target: string) => void;
}

const selectClass =
  "min-w-0 rounded-md border border-line bg-surface px-2 py-1 text-[11px] text-ink outline-none focus:border-accent";

export default function LanguagePairSelector({ state, error, onChange }: Props) {
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);

  const sources = useMemo(
    () => Array.from(new Set(state.pairs.map((pair) => pair.source))).sort(),
    [state.pairs],
  );
  const targetsFor = (source: string) =>
    state.pairs
      .filter((pair) => pair.source === source)
      .map((pair) => pair.target)
      .sort();

  const configuredAvailable = state.pairs.some(
    (pair) => pair.source === state.source && pair.target === state.target,
  );
  const initialSource = configuredAvailable ? state.source : (sources[0] ?? "");
  const initialTargets = initialSource ? targetsFor(initialSource) : [];
  const initialTarget = configuredAvailable
    ? state.target
    : (initialTargets[0] ?? "");
  const [source, setSource] = useState(initialSource);
  const [target, setTarget] = useState(initialTarget);

  useEffect(() => {
    setPending(false);
    if (configuredAvailable) {
      setSource(state.source);
      setTarget(state.target);
    } else if (sources.length > 0) {
      const nextSource = sources[0] ?? "";
      const nextTargets = nextSource ? targetsFor(nextSource) : [];
      setSource(nextSource);
      setTarget(nextTargets[0] ?? "");
    }
  }, [state.source, state.target, configuredAvailable, sources.join(",")]);

  if (!state.enabled || state.pairs.length === 0) {
    return null;
  }

  const label = configuredAvailable
    ? `${state.source} → ${state.target}`
    : "Choose pair";

  const toggle = () => {
    if (pending) return;
    setOpen((value) => !value);
  };

  return (
    <div className="relative">
      <button
        type="button"
        aria-expanded={open}
        onClick={toggle}
        className="min-h-7 rounded-full border border-accent/30 bg-accent-soft px-2.5 py-1 text-[10px] font-semibold text-accent hover:border-accent/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent/60"
      >
        {label}
      </button>

      {open ? (
        <div className="absolute right-0 top-8 z-20 flex w-max max-w-[min(18rem,calc(100vw-2rem))] flex-col gap-2 rounded-xl border border-line bg-surface px-3 py-2.5 shadow-lg">
          <label className="flex items-center gap-2 text-[10px] uppercase tracking-wide text-muted">
            <span className="w-7 shrink-0">From</span>
            <select
              className={`${selectClass} flex-1`}
              value={source}
              onChange={(event) => {
                const nextSource = event.target.value;
                setSource(nextSource);
                const nextTargets = targetsFor(nextSource);
                const nextTarget = nextTargets[0] ?? "";
                setTarget(nextTarget);
                if (nextSource && nextTarget) {
                  setPending(true);
                  onChange(nextSource, nextTarget);
                }
              }}
            >
              {sources.map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
          </label>
          <label className="flex items-center gap-2 text-[10px] uppercase tracking-wide text-muted">
            <span className="w-7 shrink-0">To</span>
            <select
              className={`${selectClass} flex-1`}
              value={target}
              onChange={(event) => {
                const nextTarget = event.target.value;
                setTarget(nextTarget);
                if (source && nextTarget) {
                  setPending(true);
                  onChange(source, nextTarget);
                }
              }}
            >
              {targetsFor(source).map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
          </label>
          {error ? (
            <p className="max-w-[28ch] text-[10px] text-red-400">{error}</p>
          ) : null}
          {!configuredAvailable ? (
            <p className="max-w-[28ch] text-[10px] text-muted">
              Configured pair has no installed model. Choose an installed pair.
            </p>
          ) : null}
          <p className="text-[10px] text-muted">Escape exits Lexicon.</p>
        </div>
      ) : null}
    </div>
  );
}
