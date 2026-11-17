package main

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PandaX185/lexicon/internal/dictionary"
)

func TestARPABETToIPA(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "single", input: "D AO1 G", want: "/ˈdɔɡ/"},
		{name: "variants", input: "D AO1 G ; D AO2 G", want: "/ˈdɔɡ/, /ˌdɔɡ/"},
		{name: "unstressed final vowel", input: "HH AE1 P IY0", want: "/ˈhæpi/"},
		{name: "clusters", input: "K W IH1 K L IY0", want: "/ˈkwɪkli/"},
		{name: "child", input: "CH AY1 L D", want: "/ˈtʃaɪld/"},
		{name: "empty", input: "   ", want: ""},
		{name: "invalid tokens", input: "lower 123", want: "//"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := arpabetToIPA(test.input); got != test.want {
				t.Fatalf("arpabetToIPA(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestCleanGloss(t *testing.T) {
	tests := []struct {
		gloss string
		want  string
	}{
		{gloss: ` the quality of being; "a rare example"`, want: "The quality of being"},
		{gloss: "  a thing;  ", want: "A thing"},
		{gloss: `"only quoted"`, want: "Only quoted"},
		{gloss: "", want: ""},
	}
	for _, test := range tests {
		if got := cleanGloss(test.gloss); got != test.want {
			t.Fatalf("cleanGloss(%q) = %q, want %q", test.gloss, got, test.want)
		}
	}
}

func TestGeneratedForms(t *testing.T) {
	tests := []struct {
		word  string
		parts []string
		want  []generatedForm
	}{
		{
			word:  "run",
			parts: []string{"noun", "verb"},
			want: []generatedForm{
				{form: "runs", kind: "plural"},
				{form: "runs", kind: "third_person"},
				{form: "runned", kind: "past"},
				{form: "running", kind: "gerund"},
			},
		},
		{
			word:  "study",
			parts: []string{"noun", "verb"},
			want: []generatedForm{
				{form: "studies", kind: "plural"},
				{form: "studies", kind: "third_person"},
				{form: "studied", kind: "past"},
				{form: "studying", kind: "gerund"},
			},
		},
		{
			word:  "happy",
			parts: []string{"adjective"},
			want: []generatedForm{
				{form: "happier", kind: "comparative"},
				{form: "happiest", kind: "superlative"},
			},
		},
		{word: "go", parts: []string{"verb"}, want: nil},
		{word: "a", parts: []string{"noun"}, want: nil},
		{word: "domestic dog", parts: []string{"noun"}, want: nil},
		{word: "café", parts: []string{"noun"}, want: nil},
	}
	for _, test := range tests {
		parts := map[string]struct{}{}
		for _, part := range test.parts {
			parts[part] = struct{}{}
		}
		got := generatedForms(test.word, parts)
		if len(got) != len(test.want) {
			t.Fatalf("generatedForms(%q) = %v, want %v", test.word, got, test.want)
		}
		for index := range got {
			if got[index] != test.want[index] {
				t.Fatalf("generatedForms(%q)[%d] = %v, want %v", test.word, index, got[index], test.want[index])
			}
		}
	}
}

func TestParseSynsets(t *testing.T) {
	var lemmas []string
	var definitions []string
	err := parseSynsets(filepath.Join("testdata", "wordnet", "data.noun"), func(gotLemmas []string, definition string) error {
		lemmas = append(lemmas, strings.Join(gotLemmas, ","))
		definitions = append(definitions, definition)
		return nil
	})
	if err != nil {
		t.Fatalf("parse synsets: %v", err)
	}
	if len(lemmas) != 3 {
		t.Fatalf("synsets = %d, want 3", len(lemmas))
	}
	if lemmas[0] != "dog,domestic dog" {
		t.Fatalf("first lemmas = %q, want dog,domestic dog", lemmas[0])
	}
	if definitions[0] != "A domesticated carnivorous mammal" {
		t.Fatalf("first definition = %q", definitions[0])
	}
}

func TestParseExceptionFile(t *testing.T) {
	var pairs []string
	err := parseExceptionFile(filepath.Join("testdata", "wordnet", "verb.exc"), func(inflected, base string) error {
		pairs = append(pairs, inflected+"->"+base)
		return nil
	})
	if err != nil {
		t.Fatalf("parse exceptions: %v", err)
	}
	if len(pairs) != 1 || pairs[0] != "went->go" {
		t.Fatalf("pairs = %v, want [went->go]", pairs)
	}
}

func TestBuild(t *testing.T) {
	output := filepath.Join(t.TempDir(), "en.sqlite")
	stats, err := Build(context.Background(), Options{
		WordNetDir:  filepath.Join("testdata", "wordnet"),
		CMUDictFile: filepath.Join("testdata", "cmudict.dict"),
		OutputPath:  output,
		Progress:    io.Discard,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := Stats{Entries: 7, Definitions: 8, Forms: 9, AmbiguousForms: 0, ExceptionForms: 2}
	if stats != want {
		t.Fatalf("stats = %+v, want %+v", stats, want)
	}

	db, err := sql.Open("sqlite", output)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer func() { _ = db.Close() }()

	var name, source, target, license, attribution string
	err = db.QueryRow(`SELECT name, source_language, target_language, license, attribution FROM dictionary_info`).
		Scan(&name, &source, &target, &license, &attribution)
	if err != nil {
		t.Fatalf("read dictionary info: %v", err)
	}
	if name != dictionaryName || source != "en" || target != "en" || license != "WordNet 3.0" {
		t.Fatalf("dictionary info = %q %q %q %q", name, source, target, license)
	}
	if !strings.Contains(attribution, "CMU Pronouncing Dictionary") {
		t.Fatalf("attribution = %q", attribution)
	}

	var userVersion int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if userVersion != 3 {
		t.Fatalf("user_version = %d, want 3", userVersion)
	}

	ipa := map[string]string{}
	rows, err := db.Query(`SELECT word, pronunciation_ipa FROM entries`)
	if err != nil {
		t.Fatalf("query entries: %v", err)
	}
	for rows.Next() {
		var word, value string
		if err := rows.Scan(&word, &value); err != nil {
			t.Fatalf("scan entry: %v", err)
		}
		ipa[word] = value
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read entries: %v", err)
	}
	_ = rows.Close()
	for word, wantIPA := range map[string]string{
		"dog":     "/ˈdɔɡ/, /ˌdɔɡ/",
		"run":     "/ˈrʌn/",
		"go":      "/ˈɡoʊ/",
		"child":   "/ˈtʃaɪld/",
		"happy":   "/ˈhæpi/",
		"quickly": "/ˈkwɪkli/",
	} {
		if ipa[word] != wantIPA {
			t.Fatalf("IPA for %s = %q, want %q", word, ipa[word], wantIPA)
		}
	}

	definitions := map[string][]string{}
	rows, err = db.Query(`
		SELECT e.word, d.part_of_speech || ':' || d.definition
		FROM definitions d JOIN entries e ON e.id = d.entry_id
		ORDER BY e.word, d.sense_order`)
	if err != nil {
		t.Fatalf("query definitions: %v", err)
	}
	for rows.Next() {
		var word, definition string
		if err := rows.Scan(&word, &definition); err != nil {
			t.Fatalf("scan definition: %v", err)
		}
		definitions[word] = append(definitions[word], definition)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read definitions: %v", err)
	}
	_ = rows.Close()
	if got := definitions["run"]; len(got) != 2 || got[0] != "noun:An act or period of running" || got[1] != "verb:Move quickly on foot" {
		t.Fatalf("run definitions = %v", got)
	}

	var synonyms string
	if err := db.QueryRow(`
		SELECT m.value FROM entry_metadata m JOIN entries e ON e.id = m.entry_id
		WHERE e.word = 'dog' AND m.key = 'synonyms'`).Scan(&synonyms); err != nil {
		t.Fatalf("read synonyms: %v", err)
	}
	if synonyms != "domestic dog" {
		t.Fatalf("dog synonyms = %q", synonyms)
	}

	forms := map[string]string{}
	rows, err = db.Query(`
		SELECT f.form, f.kind || ':' || e.word
		FROM word_forms f JOIN entries e ON e.id = f.entry_id`)
	if err != nil {
		t.Fatalf("query forms: %v", err)
	}
	for rows.Next() {
		var form, target string
		if err := rows.Scan(&form, &target); err != nil {
			t.Fatalf("scan form: %v", err)
		}
		forms[form] = target
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read forms: %v", err)
	}
	_ = rows.Close()
	wantForms := map[string]string{
		"children": "irregular:child",
		"childs":   "plural:child",
		"dogs":     "plural:dog",
		"happier":  "comparative:happy",
		"happiest": "superlative:happy",
		"runned":   "past:run",
		"running":  "gerund:run",
		"runs":     "third_person:run",
		"went":     "irregular:go",
	}
	if len(forms) != len(wantForms) {
		t.Fatalf("forms = %v, want %d entries", forms, len(wantForms))
	}
	for form, wantTarget := range wantForms {
		if forms[form] != wantTarget {
			t.Fatalf("form %s = %q, want %q", form, forms[form], wantTarget)
		}
	}

	store, err := dictionary.Open(output)
	if err != nil {
		t.Fatalf("open with dictionary store: %v", err)
	}
	defer func() { _ = store.Close() }()
	entry, err := store.Lookup(context.Background(), "went")
	if err != nil {
		t.Fatalf("lookup went: %v", err)
	}
	if entry.Word != "go" || entry.MatchedForm != "went" {
		t.Fatalf("went resolved to %+v", entry)
	}

	second := filepath.Join(t.TempDir(), "en-second.sqlite")
	if _, err := Build(context.Background(), Options{
		WordNetDir:  filepath.Join("testdata", "wordnet"),
		CMUDictFile: filepath.Join("testdata", "cmudict.dict"),
		OutputPath:  second,
		Progress:    io.Discard,
	}); err != nil {
		t.Fatalf("second build: %v", err)
	}
	firstBytes, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read first output: %v", err)
	}
	secondBytes, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second output: %v", err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("two builds of the same inputs are not byte-identical")
	}
}

func TestBuildRequiresInputs(t *testing.T) {
	if _, err := Build(context.Background(), Options{}); err == nil {
		t.Fatal("expected an error for missing --wordnet-dir")
	}
	if _, err := Build(context.Background(), Options{WordNetDir: "testdata/wordnet"}); err == nil {
		t.Fatal("expected an error for missing --output")
	}
}

func TestSplitSQL(t *testing.T) {
	statements := splitSQL(`
-- a comment
CREATE TABLE one (id INTEGER); -- trailing
CREATE TABLE two (value TEXT DEFAULT 'a;b');
`)
	if len(statements) != 2 {
		t.Fatalf("statements = %d, want 2: %v", len(statements), statements)
	}
	if !strings.Contains(statements[0], "a comment") || !strings.Contains(statements[0], "CREATE TABLE one") {
		t.Fatalf("first statement = %q", statements[0])
	}
	if !strings.Contains(statements[1], "a;b") {
		t.Fatalf("second statement = %q", statements[1])
	}
}

// TestBuildFullCorpus is opt-in because it needs the raw WordNet and CMUdict
// datasets, which are build-time inputs and are not tracked.
func TestBuildFullCorpus(t *testing.T) {
	wordnetDir := os.Getenv("LEXICON_TEST_WORDNET_DIR")
	cmudictFile := os.Getenv("LEXICON_TEST_CMUDICT")
	if wordnetDir == "" || cmudictFile == "" {
		t.Skip("set LEXICON_TEST_WORDNET_DIR and LEXICON_TEST_CMUDICT to run")
	}

	output := filepath.Join(t.TempDir(), "en.sqlite")
	stats, err := Build(context.Background(), Options{
		WordNetDir:  wordnetDir,
		CMUDictFile: cmudictFile,
		OutputPath:  output,
		Progress:    io.Discard,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := Stats{Entries: 147806, Definitions: 206918, Forms: 113116, AmbiguousForms: 313, ExceptionForms: 4967}
	if stats != want {
		t.Fatalf("stats = %+v, want %+v", stats, want)
	}

	store, err := dictionary.Open(output)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	entry, err := store.Lookup(context.Background(), "serendipity")
	if err != nil {
		t.Fatalf("lookup serendipity: %v", err)
	}
	if !strings.Contains(entry.Pronunciation.IPA, "ˌsɛrənˈdɪpɪti") {
		t.Fatalf("serendipity IPA = %q", entry.Pronunciation.IPA)
	}
	entry, err = store.Lookup(context.Background(), "went")
	if err != nil {
		t.Fatalf("lookup went: %v", err)
	}
	if entry.Word != "go" {
		t.Fatalf("went resolved to %q", entry.Word)
	}
	entry, err = store.Lookup(context.Background(), "children")
	if err != nil {
		t.Fatalf("lookup children: %v", err)
	}
	if entry.Word != "child" {
		t.Fatalf("children resolved to %q", entry.Word)
	}
}
