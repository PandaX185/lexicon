import { useEffect, useId, useRef, useState } from "react";
import type { KeyboardEvent as ReactKeyboardEvent } from "react";
import type { LanguagePairsState } from "../types";

interface Props {
  state: LanguagePairsState;
  error: string;
  onChange: (source: string, target: string) => void;
}

function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

interface DropdownProps {
  label: string;
  value: string;
  options: string[];
  onChange: (value: string) => void;
}

// A popover drawn inside the webview: a native <select> would open a separate
// GTK window and fire WindowLostFocus, which quits the whole popup process.
function LanguageDropdown({ label, value, options, onChange }: DropdownProps) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const optionRefs = useRef<Array<HTMLLIElement | null>>([]);
  const listId = useId();
  const optionsKey = options.join("\u0000");

  useEffect(() => {
    if (!open) {
      return;
    }
    setActiveIndex(Math.max(0, options.indexOf(value)));
    listRef.current?.focus();
    // optionsKey tracks option changes without refocusing on every render.
  }, [open, value, optionsKey]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (open) {
      optionRefs.current[activeIndex]?.scrollIntoView({ block: "nearest" });
    }
  }, [activeIndex, open]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const closeOutside = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("pointerdown", closeOutside);
    return () => document.removeEventListener("pointerdown", closeOutside);
  }, [open]);

  const commit = (option: string) => {
    setOpen(false);
    buttonRef.current?.focus();
    if (option !== value) {
      onChange(option);
    }
  };

  const onKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (!open) {
      return;
    }
    switch (event.key) {
      case "ArrowDown":
      case "ArrowUp": {
        event.preventDefault();
        const step = event.key === "ArrowDown" ? 1 : -1;
        setActiveIndex((index) => (index + step + options.length) % options.length);
        break;
      }
      case "Home":
        event.preventDefault();
        setActiveIndex(0);
        break;
      case "End":
        event.preventDefault();
        setActiveIndex(options.length - 1);
        break;
      case "Enter":
      case " ":
        event.preventDefault();
        if (options[activeIndex]) {
          commit(options[activeIndex]);
        }
        break;
      case "Escape":
        event.preventDefault();
        // Escape also dismisses the whole popup through the Wails keybinding.
        buttonRef.current?.focus();
        setOpen(false);
        break;
      default:
        break;
    }
  };

  return (
    <div
      ref={rootRef}
      className="relative min-w-0"
      onKeyDown={onKeyDown}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setOpen(false);
        }
      }}
    >
      <div className="mb-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted">
        {label}
      </div>
      <button
        ref={buttonRef}
        type="button"
        aria-label={`${label} language: ${value}`}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        onClick={() => setOpen((value) => !value)}
        className="flex min-h-7 min-w-16 items-center justify-between gap-2 rounded-md border border-line bg-surface-soft px-2 py-1 font-mono text-[11px] text-ink focus-visible:border-accent/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent/60"
      >
        <span aria-hidden="true">{value}</span>
        <svg
          aria-hidden="true"
          className={`h-2.5 w-2.5 shrink-0 text-muted transition-transform ${open ? "rotate-180" : ""}`}
          fill="none"
          stroke="currentColor"
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="2"
          viewBox="0 0 12 12"
        >
          <path d="m2.5 4.5 3.5 3 3.5-3" />
        </svg>
      </button>
      {open ? (
        <ul
          ref={listRef}
          id={listId}
          role="listbox"
          aria-label={`${label} language`}
          aria-activedescendant={`${listId}-${activeIndex}`}
          tabIndex={-1}
          className="absolute left-0 top-full z-20 mt-1 max-h-40 min-w-full overflow-y-auto rounded-lg border border-line bg-surface p-1 shadow-lg focus:outline-none"
        >
          {options.map((option, index) => (
            <li
              key={option}
              ref={(node) => {
                optionRefs.current[index] = node;
              }}
              id={`${listId}-${index}`}
              role="option"
              aria-selected={option === value}
              tabIndex={-1}
              onMouseDown={(event) => event.preventDefault()}
              onMouseEnter={() => setActiveIndex(index)}
              onClick={() => commit(option)}
              className={`cursor-pointer rounded-md px-2 py-1 font-mono text-[11px] ${
                index === activeIndex ? "bg-accent-soft text-accent" : "text-muted"
              }`}
            >
              {option}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

export default function TranslationPairSelect({
  state,
  error,
  onChange,
}: Props) {
  const pairs = [...state.pairs].sort(
    (a, b) => compare(a.source, b.source) || compare(a.target, b.target),
  );
  const sources = [...new Set(pairs.map((pair) => pair.source))];
  const targetsFor = (source: string) =>
    pairs.filter((pair) => pair.source === source).map((pair) => pair.target);
  const derivedSource = pairs.some(
    (pair) => pair.source === state.source && pair.target === state.target,
  )
    ? state.source
    : (sources[0] ?? "");
  const derivedTargets = targetsFor(derivedSource);
  const derivedTarget = derivedTargets.includes(state.target)
    ? state.target
    : (derivedTargets[0] ?? "");

  // Update immediately for responsiveness, then take daemon state as the
  // source of truth when it catches up. A rejected choice resyncs to the last
  // applied pair. Clearing an error at request time (or on success) is NOT a
  // resync signal: the daemon may not have answered yet, and resetting here
  // would snap an optimistic From back to the old pair and break last-change-
  // wins on a quick follow-up To change.
  const [selected, setSelected] = useState({
    source: derivedSource,
    target: derivedTarget,
  });
  const lastPairRef = useRef({ source: derivedSource, target: derivedTarget });
  const lastErrorRef = useRef(error);

  useEffect(() => {
    const pairChanged =
      derivedSource !== lastPairRef.current.source ||
      derivedTarget !== lastPairRef.current.target;
    const newError = error !== "" && error !== lastErrorRef.current;
    lastPairRef.current = { source: derivedSource, target: derivedTarget };
    lastErrorRef.current = error;
    if (pairChanged || newError) {
      setSelected({ source: derivedSource, target: derivedTarget });
    }
  }, [derivedSource, derivedTarget, error]);

  if (!state.enabled || !state.available || pairs.length === 0) {
    return null;
  }

  const selectedTargets = targetsFor(selected.source);
  const activeSource = selectedTargets.length > 0 ? selected.source : derivedSource;
  const targets = selectedTargets.length > 0 ? selectedTargets : derivedTargets;
  const activeTarget = targets.includes(selected.target)
    ? selected.target
    : (targets[0] ?? derivedTarget);

  const selectSource = (source: string) => {
    const nextTargets = targetsFor(source);
    const first = nextTargets[0];
    if (!first) {
      return;
    }
    setSelected({ source, target: first });
    onChange(source, first);
  };

  const selectTarget = (target: string) => {
    if (target === activeTarget) {
      return;
    }
    setSelected({ source: activeSource, target });
    onChange(activeSource, target);
  };

  return (
    <div className="flex flex-col gap-2 rounded-xl border border-line bg-surface-soft/40 px-3 py-2.5">
      <div className="flex items-center justify-between gap-2">
        <span className="text-[10px] font-semibold uppercase tracking-[0.12em] text-faint">
          Translation pair
        </span>
        <span className="font-mono text-[10px] text-faint" dir="auto">
          {activeSource} → {activeTarget}
        </span>
      </div>
      <div className="flex flex-wrap gap-x-6 gap-y-2">
        <LanguageDropdown
          label="From"
          value={activeSource}
          options={sources}
          onChange={selectSource}
        />
        <LanguageDropdown
          label="To"
          value={activeTarget}
          options={targets}
          onChange={selectTarget}
        />
      </div>
      {error ? (
        <p className="max-w-[28ch] break-words text-[10px] text-red-400">{error}</p>
      ) : null}
    </div>
  );
}
