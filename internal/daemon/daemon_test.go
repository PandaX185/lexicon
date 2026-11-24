package daemon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/ipc"
	"github.com/PandaX185/lexicon/internal/selection"
	_ "modernc.org/sqlite"
)

func TestPopupBinaryErrorSuggestsCurrentBuildCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := popupBinary(config.Default())
	if err == nil {
		t.Fatal("expected an error when the popup binary is missing")
	}
	if !strings.Contains(err.Error(), "just build-popup") {
		t.Fatalf("error = %q, want the current build command", err)
	}
	if strings.Contains(err.Error(), "make popup-build") {
		t.Fatalf("error = %q, want no stale make guidance", err)
	}
}

func TestPopupArgumentsIncludeReplayForAutoLaunch(t *testing.T) {
	cfg := config.Default()
	cfg.Daemon.Socket = "/tmp/lexicon-test.sock"

	got := popupArguments(cfg, "/tmp/lexicon-test.toml", true)
	want := []string{"--socket", "/tmp/lexicon-test.sock", "--replay", "--config", "/tmp/lexicon-test.toml"}
	if !slices.Equal(got, want) {
		t.Fatalf("auto-launch arguments = %q, want %q", got, want)
	}
}

func TestPopupArgumentsOmitReplayForManualLaunch(t *testing.T) {
	cfg := config.Default()
	cfg.Daemon.Socket = "/tmp/lexicon-test.sock"

	got := popupArguments(cfg, "", false)
	want := []string{"--socket", "/tmp/lexicon-test.sock"}
	if !slices.Equal(got, want) {
		t.Fatalf("manual arguments without a config = %q, want %q", got, want)
	}

	got = popupArguments(cfg, "/tmp/lexicon-manual.toml", false)
	want = []string{"--socket", "/tmp/lexicon-test.sock", "--config", "/tmp/lexicon-manual.toml"}
	if !slices.Equal(got, want) {
		t.Fatalf("manual arguments with a config = %q, want %q", got, want)
	}
}

func TestWaitForPopupReapsChild(t *testing.T) {
	command := exec.Command("/bin/true")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	pid := command.Process.Pid
	daemon := &Daemon{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	daemon.waitForPopup(command)

	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("process %d still exists after Wait: %v", pid, err)
	}
}

func lookupTestDictionary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dictionary.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
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
		`INSERT INTO dictionary_info (id, name, source_language, target_language, license, attribution)
			VALUES (1, 'Test Dictionary', 'en', 'en', 'CC0', 'Test fixture')`,
		`INSERT INTO entries (id, word) VALUES (1, 'go')`,
		`INSERT INTO definitions (id, entry_id, part_of_speech, definition, label, sense_order)
			VALUES (1, 1, 'verb', 'to move from one place to another', '', 1)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture statement failed: %v\n%s", err, statement)
		}
	}
	return path
}

func TestLookupWithNilTranslatorDoesNotPanic(t *testing.T) {
	store, err := dictionary.Open(lookupTestDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	// The daemon can be closed while an in-flight lookup still references it;
	// closeResources nils the translator. Lookup must tolerate that state.
	d := &Daemon{
		logger: newTestLogger(t),
		cfg:    config.Effective(config.Default()),
		store:  store,
	}

	result := d.Lookup(context.Background(), "go")
	if !result.Found || result.Entry == nil {
		t.Fatalf("result = %+v; want a found entry", result)
	}
	if result.Translation != "" || result.TranslationError != "" {
		t.Fatalf("translation = %q / %q; want neither", result.Translation, result.TranslationError)
	}
}

func TestLookupAndReloadAreRaceFree(t *testing.T) {
	dictionaryPath := lookupTestDictionary(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	cfg := config.Default()
	cfg.Selection.Provider = "none"
	cfg.Dictionary.DatabasePath = dictionaryPath
	if err := config.Write(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	d, err := New(configPath, newTestLogger(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.closeResources()

	const iterations = 20
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range iterations {
			if err := d.Reload(context.Background()); err != nil {
				t.Errorf("Reload: %v", err)
				return
			}
		}
	}()
	for range iterations {
		result := d.Lookup(context.Background(), "go")
		if !result.Found {
			t.Errorf("lookup not found: %+v", result)
			return
		}
	}
	<-done
}

type fixedSelection struct{ text string }

var _ selection.Provider = fixedSelection{}

func (s fixedSelection) Name() string                            { return "test" }
func (s fixedSelection) Primary(context.Context) (string, error) { return s.text, nil }

func TestLookupRecordsPrimarySelectionAsQuery(t *testing.T) {
	store, err := dictionary.Open(lookupTestDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	d := &Daemon{
		logger:    newTestLogger(t),
		cfg:       config.Effective(config.Default()),
		store:     store,
		selection: fixedSelection{text: "go"},
	}

	result := d.Lookup(context.Background(), "")
	if result.Query != "go" {
		t.Fatalf("query = %q, want the PRIMARY selection %q", result.Query, "go")
	}
	if !result.Found {
		t.Fatalf("result = %+v, want a found entry", result)
	}
}

func TestLookupAttributesNeverIncludeText(t *testing.T) {
	const (
		secretQuery = "SECRET-SELECTED-TEXT"
		secretEntry = "SECRET-DICTIONARY-WORD"
		secretTrans = "SECRET-TRANSLATION"
	)
	result := &ipc.LookupResult{
		Found:       true,
		Query:       secretQuery,
		Normalized:  secretQuery,
		Translation: secretTrans,
		Entry: &dictionary.Entry{
			Word: secretEntry,
		},
	}

	attributes := lookupAttributes(result, time.Now())
	rendered := fmt.Sprint(attributes...)
	for _, secret := range []string{secretQuery, secretEntry, secretTrans} {
		if strings.Contains(rendered, secret) {
			t.Fatalf("log attributes contain %q: %s", secret, rendered)
		}
	}

	allowed := map[string]bool{
		"found":             true,
		"query_chars":       true,
		"duration_ms":       true,
		"kind":              true,
		"translation_error": true,
	}
	for i := 0; i < len(attributes); i += 2 {
		key, ok := attributes[i].(string)
		if !ok {
			t.Fatalf("attribute key %v is not a string", attributes[i])
		}
		if !allowed[key] {
			t.Fatalf("unexpected log attribute %q", key)
		}
	}
}
