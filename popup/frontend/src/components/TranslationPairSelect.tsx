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
  const listId = useId();

  useEffect(() => {
    if (!open) {
      return;
    }
    setActiveIndex(Math.max(0, options.indexOf(value)));
  }, [open, options, value]);

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
        event.stopPropagation();
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
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        aria-activedescendant={open ? `${listId}-${activeIndex}` : undefined}
        onClick={() => setOpen((value) => !value)}
        className="flex min-h-7 min-w-16 items-center justify-between gap-2 rounded-md border border-line bg-surface-soft px-2 py-1 font-mono text-[11px] text-ink focus-visible:border-accent/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent/60"
      >
        <span>{value}</span>
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
          id={listId}
          role="listbox"
          aria-label={`${label} language`}
          className="absolute left-0 top-full z-20 mt-1 max-h-40 min-w-full overflow-y-auto rounded-lg border border-line bg-surface p-1 shadow-lg"
        >
          {options.map((option, index) => (
            <li
              key={option}
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
  if (!state.enabled || !state.available || state.pairs.length === 0) {
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
  const activeTarget = targets.includes(state.target)
    ? state.target
    : (targets[0] ?? "");

  const selectSource = (source: string) => {
    if (source === activeSource) {
      return;
    }
    const nextTargets = pairs
      .filter((pair) => pair.source === source)
      .map((pair) => pair.target);
    if (nextTargets[0]) {
      onChange(source, nextTargets[0]);
    }
  };

  const selectTarget = (target: string) => {
    if (target !== activeTarget) {
      onChange(activeSource, target);
    }
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
