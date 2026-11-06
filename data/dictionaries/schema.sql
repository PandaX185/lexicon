PRAGMA foreign_keys = ON;
PRAGMA user_version = 1;

CREATE TABLE IF NOT EXISTS entries (
    id INTEGER PRIMARY KEY,
    word TEXT NOT NULL COLLATE NOCASE,
    pronunciation_respelling TEXT NOT NULL DEFAULT '',
    pronunciation_ipa TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    license TEXT NOT NULL DEFAULT '',
    UNIQUE (word)
);

CREATE INDEX IF NOT EXISTS idx_entries_word ON entries (word COLLATE NOCASE);

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
