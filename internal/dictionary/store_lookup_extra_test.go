package dictionary

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func createLookupVariantFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "variants.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = db.Close() }()

	statements := []string{
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
		`INSERT INTO entries (id, word) VALUES
			(1, 'ice cream'), (2, 'blackboard'), (3, 'high school'),
			(4, 'eat'), (5, 'Ate'), (6, 'fly'), (7, 'flies'),
			(8, 'doe'), (9, 'do'), (10, 'be'), (11, 'are')`,
		`INSERT INTO word_forms (form, entry_id, kind) VALUES
			('ice creams', 1, 'plural'),
			('ate', 4, 'irregular'),
			('flies', 6, 'third_person'),
			('does', 8, 'plural'),
			('does', 9, 'third_person')`,
		`INSERT INTO entry_metadata (entry_id, key, value) VALUES
			(4, 'preferred_form:ate', ''),
			(6, 'preferred_form:flies', ''),
			(9, 'preferred_form:does', ''),
			(10, 'preferred_form:are', '')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture statement failed: %v\n%s", err, statement)
		}
	}
	return path
}

func openLookupVariantFixture(t *testing.T) *Store {
	t.Helper()
	store, err := Open(createLookupVariantFixture(t))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestLookupSeparatorVariants(t *testing.T) {
	store := openLookupVariantFixture(t)
	tests := []struct {
		query       string
		want        string
		matchedForm string
	}{
		{query: "ice-cream", want: "ice cream"},
		{query: "black board", want: "blackboard"},
		{query: "high-school", want: "high school"},
		{query: "ice-creams", want: "ice cream", matchedForm: "ice creams"},
	}
	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			entry, err := store.Lookup(context.Background(), test.query)
			if err != nil {
				t.Fatalf("lookup: %v", err)
			}
			if entry.Word != test.want {
				t.Fatalf("word = %q, want %q", entry.Word, test.want)
			}
			if entry.MatchedForm != test.matchedForm {
				t.Fatalf("matched form = %q, want %q", entry.MatchedForm, test.matchedForm)
			}
		})
	}
}

func TestLookupPreferredFormOverridesExactHeadword(t *testing.T) {
	store := openLookupVariantFixture(t)
	tests := map[string]string{
		"ate":   "eat",
		"are":   "be",
		"flies": "fly",
		"does":  "do",
	}
	for query, want := range tests {
		t.Run(query, func(t *testing.T) {
			entry, err := store.Lookup(context.Background(), query)
			if err != nil {
				t.Fatalf("lookup: %v", err)
			}
			if entry.Word != want {
				t.Fatalf("word = %q, want %q", entry.Word, want)
			}
		})
	}
}
