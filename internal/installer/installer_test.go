package installer

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PandaX185/lexicon/internal/dictionary"
	_ "modernc.org/sqlite"
)

func createTestDictionary(t *testing.T, path string) {
	t.Helper()
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
			UNIQUE (word)
		)`,
		`INSERT INTO dictionary_info (id, name, source_language, target_language, license, attribution)
			VALUES (1, 'Test Dictionary', 'en', 'en', 'CC0', 'Test fixture')`,
		`INSERT INTO entries (id, word) VALUES (1, 'go')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture statement failed: %v\n%s", err, statement)
		}
	}
}

func writeTestManifest(t *testing.T, dir, version string) dictionary.Manifest {
	t.Helper()
	path := filepath.Join(dir, "en.sqlite")
	hash, size, err := dictionary.HashFile(path)
	if err != nil {
		t.Fatalf("hash fixture: %v", err)
	}
	manifest := dictionary.Manifest{
		Name:           "Test Dictionary",
		Version:        version,
		File:           "en.sqlite",
		Bytes:          size,
		SHA256:         hash,
		SourceLanguage: "en",
		TargetLanguage: "en",
	}
	if err := dictionary.WriteManifest(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return manifest
}

func TestFindPackagedDictionary(t *testing.T) {
	t.Run("database without manifest", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "en.sqlite")
		createTestDictionary(t, path)

		got, err := findPackagedDictionary([]string{dir})
		if err != nil {
			t.Fatalf("find packaged dictionary: %v", err)
		}
		if got == nil || got.path != path || got.manifest != nil {
			t.Fatalf("packaged = %+v, want path %s without manifest", got, path)
		}
	})

	t.Run("verified manifest", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "en.sqlite")
		createTestDictionary(t, path)
		writeTestManifest(t, dir, "1.2.3")

		got, err := findPackagedDictionary([]string{dir})
		if err != nil {
			t.Fatalf("find packaged dictionary: %v", err)
		}
		if got == nil || got.manifest == nil || got.manifest.Version != "1.2.3" {
			t.Fatalf("packaged = %+v, want a verified 1.2.3 manifest", got)
		}
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "en.sqlite")
		createTestDictionary(t, path)
		manifest := writeTestManifest(t, dir, "1.2.3")
		manifest.SHA256 = strings.Repeat("0", 64)
		if err := dictionary.WriteManifest(filepath.Join(dir, "manifest.json"), manifest); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		if _, err := findPackagedDictionary([]string{dir}); err == nil || !strings.Contains(err.Error(), "checksum") {
			t.Fatalf("error = %v, want a checksum mismatch", err)
		}
	})

	t.Run("priority", func(t *testing.T) {
		empty := t.TempDir()
		dir := t.TempDir()
		path := filepath.Join(dir, "en.sqlite")
		createTestDictionary(t, path)

		got, err := findPackagedDictionary([]string{empty, dir})
		if err != nil {
			t.Fatalf("find packaged dictionary: %v", err)
		}
		if got == nil || got.path != path {
			t.Fatalf("packaged = %+v, want %s", got, path)
		}
	})

	t.Run("none", func(t *testing.T) {
		got, err := findPackagedDictionary([]string{t.TempDir()})
		if err != nil {
			t.Fatalf("find packaged dictionary: %v", err)
		}
		if got != nil {
			t.Fatalf("packaged = %+v, want nil", got)
		}
	})
}

func TestResolveDictionarySourceUsesPackaged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "en.sqlite")
	createTestDictionary(t, path)
	writeTestManifest(t, dir, "1.0")

	source, downloaded, packaged, err := resolveDictionarySource(context.Background(), Options{
		Yes:                  true,
		DictionarySearchDirs: []string{dir},
	}, newPrompter(strings.NewReader(""), io.Discard))
	if err != nil {
		t.Fatalf("resolve source: %v", err)
	}
	if source != path || downloaded != "" || packaged == nil || packaged.path != path {
		t.Fatalf("source=%q downloaded=%q packaged=%+v, want packaged %s", source, downloaded, packaged, path)
	}
}

func TestResolveDictionarySourceLocalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, path)

	source, downloaded, packaged, err := resolveDictionarySource(context.Background(), Options{
		Dictionary:           path,
		Yes:                  true,
		DictionarySearchDirs: []string{t.TempDir()},
	}, newPrompter(strings.NewReader(""), io.Discard))
	if err != nil {
		t.Fatalf("resolve source: %v", err)
	}
	if source != path || downloaded != "" || packaged != nil {
		t.Fatalf("source=%q downloaded=%q packaged=%+v, want local %s", source, downloaded, packaged, path)
	}
}

func TestResolveDictionarySourceChecksumMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, path)

	_, _, _, err := resolveDictionarySource(context.Background(), Options{
		Dictionary:           path,
		DictionarySHA256:     strings.Repeat("0", 64),
		Yes:                  true,
		DictionarySearchDirs: []string{t.TempDir()},
	}, newPrompter(strings.NewReader(""), io.Discard))
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v, want a checksum mismatch", err)
	}
}

func TestResolveDictionarySourceRejectsFlags(t *testing.T) {
	options := Options{
		Dictionary:           "local.sqlite",
		DictionaryURL:        "https://example.invalid/en.sqlite",
		DictionarySHA256:     strings.Repeat("0", 64),
		Yes:                  true,
		DictionarySearchDirs: []string{t.TempDir()},
	}
	if _, _, _, err := resolveDictionarySource(context.Background(), options,
		newPrompter(strings.NewReader(""), io.Discard)); err == nil {
		t.Fatal("expected --dictionary and --dictionary-url to conflict")
	}

	options.Dictionary = ""
	options.DryRun = true
	if _, _, _, err := resolveDictionarySource(context.Background(), options,
		newPrompter(strings.NewReader(""), io.Discard)); err == nil {
		t.Fatal("expected --dry-run with --dictionary-url to be rejected")
	}
}

func TestDownloadDictionary(t *testing.T) {
	payload := []byte("dictionary payload")
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	hash, _, err := dictionary.HashFile(writeTempFile(t, payload))
	if err != nil {
		t.Fatalf("hash payload: %v", err)
	}

	path, err := downloadDictionary(context.Background(), Options{
		DictionaryURL:    server.URL + "/en.sqlite",
		DictionarySHA256: hash,
		HTTPClient:       server.Client(),
	})
	if err != nil {
		t.Fatalf("download dictionary: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("download = %q, want %q", got, payload)
	}
}

func TestDownloadDictionaryRejects(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("payload"))
	}))
	defer server.Close()

	base := Options{DictionaryURL: server.URL + "/en.sqlite", HTTPClient: server.Client()}

	missingHash := base
	if _, err := downloadDictionary(context.Background(), missingHash); err == nil {
		t.Fatal("expected a missing checksum error")
	}

	wrongHash := base
	wrongHash.DictionarySHA256 = strings.Repeat("0", 64)
	if _, err := downloadDictionary(context.Background(), wrongHash); err == nil {
		t.Fatal("expected a checksum mismatch")
	}

	insecure := base
	insecure.DictionaryURL = "http://example.invalid/en.sqlite"
	insecure.DictionarySHA256 = strings.Repeat("0", 64)
	if _, err := downloadDictionary(context.Background(), insecure); err == nil {
		t.Fatal("expected a non-HTTPS URL to be rejected")
	}

	notFound := httptest.NewTLSServer(http.NotFoundHandler())
	defer notFound.Close()
	failing := Options{
		DictionaryURL:    notFound.URL + "/en.sqlite",
		DictionarySHA256: strings.Repeat("0", 64),
		HTTPClient:       notFound.Client(),
	}
	if _, err := downloadDictionary(context.Background(), failing); err == nil {
		t.Fatal("expected a non-200 response to fail")
	}
}

func TestValidateDictionary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "valid.sqlite")
	createTestDictionary(t, path)
	if err := validateDictionary(context.Background(), path); err != nil {
		t.Fatalf("validate dictionary: %v", err)
	}

	empty := filepath.Join(t.TempDir(), "empty.sqlite")
	db, err := sql.Open("sqlite", empty)
	if err != nil {
		t.Fatalf("open empty database: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE dictionary_info (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		name TEXT NOT NULL,
		source_language TEXT NOT NULL,
		target_language TEXT NOT NULL,
		license TEXT NOT NULL DEFAULT '',
		attribution TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create metadata: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO dictionary_info VALUES (1, 'Empty', 'en', 'en', '', '')`); err != nil {
		t.Fatalf("insert metadata: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close empty database: %v", err)
	}
	if err := validateDictionary(context.Background(), empty); err == nil {
		t.Fatal("expected a missing entries table error")
	}
}

func TestRunUsesPackagedDictionaryInDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "en.sqlite")
	createTestDictionary(t, path)
	writeTestManifest(t, dir, "1.0")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	configPath := filepath.Join(t.TempDir(), "config.toml")
	var out bytes.Buffer
	err := Run(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{dir},
		SkipService:          true,
		Yes:                  true,
		DryRun:               true,
		Stdin:                strings.NewReader(""),
		Stdout:               &out,
		Stderr:               io.Discard,
	})
	if err != nil {
		t.Fatalf("run installer: %v", err)
	}
	if !strings.Contains(out.String(), path+" (packaged 1.0, no copy)") {
		t.Fatalf("output does not report the packaged dictionary:\n%s", out.String())
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote %s", configPath)
	}
}

func TestRunCopiesLocalDictionary(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)

	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	var out bytes.Buffer
	err := Run(context.Background(), Options{
		ConfigPath:           configPath,
		Dictionary:           source,
		DictionarySearchDirs: []string{t.TempDir()},
		SkipService:          true,
		Yes:                  true,
		Stdin:                strings.NewReader(""),
		Stdout:               &out,
		Stderr:               io.Discard,
	})
	if err != nil {
		t.Fatalf("run installer: %v", err)
	}

	target := filepath.Join(dataHome, "lexicon", "dictionary.sqlite")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("installed dictionary: %v", err)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(config), target) {
		t.Fatalf("config does not point at %s:\n%s", target, config)
	}
}

func writeTempFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "payload")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}
