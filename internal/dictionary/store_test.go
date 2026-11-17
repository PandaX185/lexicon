package dictionary

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func createFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dictionary.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = db.Close() }()

	statements := []string{
		`CREATE TABLE dictionary_info (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			name TEXT NOT NULL,
			source_language TEXT NOT NULL,
			target_language TEXT NOT NULL,
			license TEXT NOT NULL DEFAULT '',
			attribution TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE entries (
			id INTEGER PRIMARY KEY,
			word TEXT NOT NULL COLLATE NOCASE,
			transliteration TEXT NOT NULL DEFAULT '',
			pronunciation_respelling TEXT NOT NULL DEFAULT '',
			pronunciation_ipa TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			license TEXT NOT NULL DEFAULT '',
			UNIQUE (word)
		)`,
		`CREATE TABLE definitions (
			id INTEGER PRIMARY KEY,
			entry_id INTEGER NOT NULL REFERENCES entries (id) ON DELETE CASCADE,
			part_of_speech TEXT NOT NULL DEFAULT 'other',
			definition TEXT NOT NULL,
			label TEXT NOT NULL DEFAULT '',
			sense_order INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE word_forms (
			form TEXT NOT NULL COLLATE NOCASE,
			entry_id INTEGER NOT NULL REFERENCES entries (id) ON DELETE CASCADE,
			kind TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (form, entry_id)
		)`,
		`CREATE TABLE entry_metadata (
			entry_id INTEGER NOT NULL REFERENCES entries (id) ON DELETE CASCADE,
			key TEXT NOT NULL,
			value TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (entry_id, key)
		)`,
		`INSERT INTO dictionary_info (id, name, source_language, target_language, license, attribution)
			VALUES (1, 'Test Dictionary', 'en', 'en', 'CC0', 'Test fixture')`,
		`INSERT INTO entries (id, word, transliteration, pronunciation_respelling, pronunciation_ipa, source, license)
			VALUES (1, 'go', 'go', 'goh', 'ɡoʊ', 'fixture', 'CC0')`,
		`INSERT INTO entries (id, word, transliteration, pronunciation_respelling, pronunciation_ipa, source, license)
			VALUES (2, 'كتب', 'kataba', '', '', 'fixture', 'CC0')`,
		`INSERT INTO definitions (id, entry_id, part_of_speech, definition, label, sense_order)
			VALUES (10, 1, 'noun', 'a board game', 'rare', 2)`,
		`INSERT INTO definitions (id, entry_id, part_of_speech, definition, label, sense_order)
			VALUES (11, 1, 'verb', 'to move from one place to another', '', 1)`,
		`INSERT INTO word_forms (form, entry_id, kind) VALUES ('went', 1, 'past')`,
		`INSERT INTO entry_metadata (entry_id, key, value) VALUES (1, 'frequency', 'high')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture statement failed: %v\n%s", err, statement)
		}
	}
	return path
}

func openFixture(t *testing.T) *Store {
	t.Helper()
	store, err := Open(createFixture(t))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestLookupExactCaseInsensitive(t *testing.T) {
	store := openFixture(t)
	entry, err := store.Lookup(context.Background(), "GO")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if entry.Word != "go" {
		t.Fatalf("word = %q, want go", entry.Word)
	}
	if entry.MatchedForm != "" {
		t.Fatalf("matched form = %q, want empty", entry.MatchedForm)
	}
	if entry.Pronunciation.IPA != "ɡoʊ" || entry.Pronunciation.Respelling != "goh" {
		t.Fatalf("unexpected pronunciation: %+v", entry.Pronunciation)
	}
	if entry.Metadata["source"] != "fixture" || entry.Metadata["frequency"] != "high" {
		t.Fatalf("unexpected metadata: %+v", entry.Metadata)
	}
}

func TestLookupDefinitionOrder(t *testing.T) {
	store := openFixture(t)
	entry, err := store.Lookup(context.Background(), "go")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(entry.Definitions) != 2 {
		t.Fatalf("definitions = %d, want 2", len(entry.Definitions))
	}
	if entry.Definitions[0].PartOfSpeech != "verb" {
		t.Fatalf("first definition = %+v, want verb", entry.Definitions[0])
	}
	if entry.Definitions[1].Label != "rare" {
		t.Fatalf("label = %q, want rare", entry.Definitions[1].Label)
	}
}

func TestLookupTransliterationAndInflection(t *testing.T) {
	store := openFixture(t)

	entry, err := store.Lookup(context.Background(), "KATABA")
	if err != nil {
		t.Fatalf("transliteration lookup: %v", err)
	}
	if entry.Word != "كتب" {
		t.Fatalf("word = %q, want كتب", entry.Word)
	}

	entry, err = store.Lookup(context.Background(), "went")
	if err != nil {
		t.Fatalf("inflected lookup: %v", err)
	}
	if entry.Word != "go" || entry.MatchedForm != "went" {
		t.Fatalf("inflected result = %+v", entry)
	}
}

func TestLookupNotFound(t *testing.T) {
	store := openFixture(t)
	if _, err := store.Lookup(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestLookupUnknownPartOfSpeech(t *testing.T) {
	store := openFixture(t)
	db, err := sql.Open("sqlite", store.Path())
	if err != nil {
		t.Fatalf("open fixture for update: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`UPDATE definitions SET part_of_speech = 'weird' WHERE id = 10`); err != nil {
		t.Fatalf("update: %v", err)
	}
	entry, err := store.Lookup(context.Background(), "go")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if entry.Definitions[1].PartOfSpeech != "other" {
		t.Fatalf("part of speech = %q, want other", entry.Definitions[1].PartOfSpeech)
	}
}

func TestDescribe(t *testing.T) {
	store := openFixture(t)
	info, err := store.Describe(context.Background())
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if info.Name != "Test Dictionary" || info.SourceLanguage != "en" || info.License != "CC0" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestLookupDegradesWithoutOptionalTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	statements := []string{
		`CREATE TABLE entries (id INTEGER PRIMARY KEY, word TEXT NOT NULL COLLATE NOCASE)`,
		`CREATE TABLE definitions (id INTEGER PRIMARY KEY, entry_id INTEGER, part_of_speech TEXT, definition TEXT, label TEXT, sense_order INTEGER)`,
		`INSERT INTO entries (id, word) VALUES (1, 'old')`,
		`INSERT INTO definitions VALUES (1, 1, 'noun', 'legacy entry', '', 0)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("statement: %v", err)
		}
	}
	_ = db.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	entry, err := store.Lookup(context.Background(), "old")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if len(entry.Definitions) != 1 || entry.Definitions[0].Text != "legacy entry" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if _, err := store.Lookup(context.Background(), "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestOpenRejectsMissingFile(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "nope.sqlite")); err == nil {
		t.Fatal("expected an error")
	}
}

func TestValidateLookupSchema(t *testing.T) {
	store := openFixture(t)
	if err := store.ValidateLookupSchema(); err != nil {
		t.Fatalf("validate fixture: %v", err)
	}
}

func TestValidateLookupSchemaRejectsMissingEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata-only.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE dictionary_info (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		name TEXT NOT NULL,
		source_language TEXT NOT NULL,
		target_language TEXT NOT NULL,
		license TEXT NOT NULL DEFAULT '',
		attribution TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create metadata table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.ValidateLookupSchema(); err == nil {
		t.Fatal("expected a missing entries table error")
	}
}
