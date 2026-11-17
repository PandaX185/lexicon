package dictionary

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realDictionaryPath returns the first readable dictionary from the explicit
// override or the known development locations. The test skips when none is
// present so the core suite still runs on a machine without the release asset.
func realDictionaryPath(t *testing.T) string {
	t.Helper()

	var candidates []string
	if path := strings.TrimSpace(os.Getenv("LEXICON_TEST_DICTIONARY")); path != "" {
		candidates = append(candidates, path)
	}
	candidates = append(candidates,
		filepath.Join("testdata", "en.sqlite"),
		// Local development convenience: the archived C++ tree keeps the
		// generated WordNet asset beside the rewrite.
		filepath.Join("..", "..", "..", "lexicon-qt-archive", "data", "dictionaries", "en.sqlite"),
	)
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		candidates = append(candidates, filepath.Join(dataHome, "lexicon", "dictionary.sqlite"))
	} else if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "share", "lexicon", "dictionary.sqlite"))
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	t.Skip("real dictionary not found; set LEXICON_TEST_DICTIONARY to run this test")
	return ""
}

func TestRealDictionary(t *testing.T) {
	path := realDictionaryPath(t)

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open real dictionary: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := t.Context()
	info, err := store.Describe(ctx)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if info.Name != "WordNet English Dictionary" {
		t.Fatalf("dictionary name = %q, want WordNet English Dictionary", info.Name)
	}
	if info.SourceLanguage != "en" || info.TargetLanguage != "en" {
		t.Fatalf("language pair = %s→%s, want en→en", info.SourceLanguage, info.TargetLanguage)
	}
	if !strings.Contains(info.Attribution, "CMU Pronouncing Dictionary") {
		t.Fatalf("attribution = %q, want CMU credit", info.Attribution)
	}

	t.Run("pronunciation", func(t *testing.T) {
		entry, err := store.Lookup(ctx, "serendipity")
		if err != nil {
			t.Fatalf("lookup serendipity: %v", err)
		}
		if entry.Word != "serendipity" {
			t.Fatalf("word = %q, want serendipity", entry.Word)
		}
		if !strings.Contains(entry.Pronunciation.IPA, "ˌsɛrənˈdɪpɪti") {
			t.Fatalf("IPA = %q, want /ˌsɛrənˈdɪpɪti/", entry.Pronunciation.IPA)
		}
		if len(entry.Definitions) == 0 {
			t.Fatal("serendipity has no definitions")
		}
	})

	t.Run("parts of speech", func(t *testing.T) {
		entry, err := store.Lookup(ctx, "run")
		if err != nil {
			t.Fatalf("lookup run: %v", err)
		}
		if !strings.Contains(entry.Pronunciation.IPA, "ˈrʌn") {
			t.Fatalf("IPA = %q, want /ˈrʌn/", entry.Pronunciation.IPA)
		}
		parts := map[string]bool{}
		for _, definition := range entry.Definitions {
			parts[definition.PartOfSpeech] = true
		}
		if !parts["noun"] || !parts["verb"] {
			t.Fatalf("parts of speech = %v, want noun and verb", parts)
		}
	})

	t.Run("inflected forms", func(t *testing.T) {
		tests := []struct {
			form string
			word string
		}{
			{form: "went", word: "go"},
			{form: "children", word: "child"},
		}
		for _, test := range tests {
			entry, err := store.Lookup(ctx, test.form)
			if err != nil {
				t.Fatalf("lookup %s: %v", test.form, err)
			}
			if entry.Word != test.word {
				t.Fatalf("%s resolved to %q, want %q", test.form, entry.Word, test.word)
			}
			if entry.MatchedForm != test.form {
				t.Fatalf("matched form = %q, want %q", entry.MatchedForm, test.form)
			}
		}
	})

	t.Run("row counts", func(t *testing.T) {
		db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
		if err != nil {
			t.Fatalf("open for counts: %v", err)
		}
		defer func() { _ = db.Close() }()

		var entries, definitions, forms int
		err = db.QueryRowContext(ctx, `
			SELECT (SELECT count(*) FROM entries),
			       (SELECT count(*) FROM definitions),
			       (SELECT count(*) FROM word_forms)`).Scan(&entries, &definitions, &forms)
		if err != nil {
			t.Fatalf("count rows: %v", err)
		}
		if entries != 147806 || definitions != 206918 || forms != 113116 {
			t.Fatalf("row counts = %d entries, %d definitions, %d forms; want 147806, 206918, 113116",
				entries, definitions, forms)
		}
	})
}
