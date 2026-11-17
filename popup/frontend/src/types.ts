export interface Pronunciation {
  respelling?: string;
  ipa?: string;
}

export interface Definition {
  part_of_speech: string;
  text: string;
  label?: string;
}

export interface Entry {
  word: string;
  transliteration?: string;
  matched_form?: string;
  pronunciation: Pronunciation;
  definitions: Definition[];
  metadata?: Record<string, string>;
}

export type ErrorKind =
  | "validation"
  | "selection"
  | "not_found"
  | "dictionary"
  | "internal";

export interface LookupResult {
  query: string;
  normalized?: string;
  found: boolean;
  entry?: Entry;
  translation?: string;
  translation_error?: string;
  error?: string;
  error_kind?: ErrorKind;
  at: string;
}

export interface ConnectionState {
  connected: boolean;
  error?: string;
}
