PRAGMA foreign_keys = ON;
PRAGMA user_version = 2;

-- One row per database: what this dictionary is, and which language pair it
-- covers. `source_language` is the language of the headwords and
-- `target_language` the language of the definitions, both equal for a
-- monolingual dictionary. Language tags are lowercase BCP 47 ("en", "de").
-- Note: the fixture applies this file statement by statement, so comments here
-- must not contain semicolons.
CREATE TABLE IF NOT EXISTS dictionary_info (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    name TEXT NOT NULL,
    source_language TEXT NOT NULL,
    target_language TEXT NOT NULL,
    license TEXT NOT NULL DEFAULT '',
    attribution TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS entries (
    id INTEGER PRIMARY KEY,
    word TEXT NOT NULL COLLATE NOCASE,
    -- Latin-script romanization of `word`, for dictionaries whose headwords are
    -- written in another script. Empty when the headword is already Latin.
    transliteration TEXT NOT NULL DEFAULT '',
    pronunciation_respelling TEXT NOT NULL DEFAULT '',
    pronunciation_ipa TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    license TEXT NOT NULL DEFAULT '',
    UNIQUE (word)
);

CREATE INDEX IF NOT EXISTS idx_entries_word ON entries (word COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_entries_transliteration ON entries (transliteration COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS definitions (
    id INTEGER PRIMARY KEY,
    entry_id INTEGER NOT NULL REFERENCES entries (id) ON DELETE CASCADE,
    part_of_speech TEXT NOT NULL DEFAULT 'other',
    definition TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    sense_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_definitions_entry ON definitions (entry_id, sense_order);

CREATE TABLE IF NOT EXISTS entry_metadata (
    entry_id INTEGER NOT NULL REFERENCES entries (id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (entry_id, key)
);
