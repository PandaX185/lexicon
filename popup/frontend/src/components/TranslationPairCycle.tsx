import type { LanguagePairsState } from "../types";

interface Props {
  state: LanguagePairsState;
  error: string;
  onChange: (source: string, target: string) => void;
}

function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

export default function TranslationPairCycle({ state, error, onChange }: Props) {
  if (!state.enabled || state.pairs.length === 0) {
    return null;
  }

  const pairs = [...state.pairs].sort(
    (a, b) => compare(a.source, b.source) || compare(a.target, b.target),
  );
  const sources = [...new Set(pairs.map((pair) => pair.source))];
  const activeSource = pairs.some(
    (pair) => pair.source === state.source && pair.target === state.target,
  )
    ? state.source
    : (sources[0] ?? "");
  const targets = pairs
    .filter((pair) => pair.source === activeSource)
    .map((pair) => pair.target);

  const cycle = () => {
    const index = pairs.findIndex(
      (pair) => pair.source === state.source && pair.target === state.target,
    );
    const next = pairs[(index + 1) % pairs.length];
    if (next) {
      onChange(next.source, next.target);
    }
  };

  const list = (label: string, codes: string[], active: string) => (
    <div className="min-w-0">
      <div className="mb-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted">
        {label}
      </div>
      <ul className="flex flex-wrap gap-1">
        {codes.map((code) => (
          <li
            key={code}
            className={`rounded-md border px-1.5 py-0.5 font-mono text-[10px] ${
              code === active
                ? "border-accent/40 bg-accent-soft text-accent"
                : "border-line bg-surface-soft text-muted"
            }`}
          >
            {code}
          </li>
        ))}
      </ul>
    </div>
  );

  return (
    <div className="flex flex-col gap-2 rounded-xl border border-line bg-surface-soft/40 px-3 py-2.5">
      <div className="flex items-center justify-between gap-2">
        <span className="text-[10px] font-semibold uppercase tracking-[0.12em] text-faint">
          Translation pair
        </span>
        <button
          type="button"
          aria-label="Next translation pair"
          onClick={cycle}
          className="min-h-7 rounded-full border border-accent/30 bg-accent-soft px-2.5 py-1 text-[10px] font-semibold text-accent hover:border-accent/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent/60"
        >
          Next
        </button>
      </div>
      <div className="flex flex-wrap gap-x-6 gap-y-2">
        {list("From", sources, activeSource)}
        {list("To", targets, state.target)}
      </div>
      {error ? (
        <p className="max-w-[28ch] break-words text-[10px] text-red-400">{error}</p>
      ) : null}
    </div>
  );
}
