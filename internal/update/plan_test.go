package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"
)

type fakeDiscoverer struct{ release Release }

func (f fakeDiscoverer) Latest(context.Context) (Release, error) { return f.release, nil }
func (f fakeDiscoverer) Version(context.Context, string) (Release, error) {
	return f.release, nil
}

type fakeRestarter struct{ calls int }

func (f *fakeRestarter) Restart(context.Context) error { f.calls++; return nil }

func sum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func testDictionaryBytes(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "en.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE entries (id INTEGER PRIMARY KEY, word TEXT NOT NULL)`,
		`CREATE TABLE definitions (id INTEGER PRIMARY KEY, entry_id INTEGER NOT NULL, part_of_speech TEXT NOT NULL, definition TEXT NOT NULL, label TEXT NOT NULL DEFAULT '', sense_order INTEGER NOT NULL DEFAULT 1)`,
		`CREATE TABLE forms (form TEXT PRIMARY KEY, entry_id INTEGER NOT NULL, form_type TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO metadata (key, value) VALUES ('name','Test'),('version','1')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture: %v\n%s", err, statement)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The fake runner overrides EnSQLiteSHA/ManifestSHA with this fixture's
	// hash; production constants stay pinned to the published dictionary.
	return data
}

// testTarball mirrors a published release: binaries and the internal
// checksums file live under one versioned root, unrelated regular files are
// present, and the whole archive is gzip-compressed like a .tar.gz asset.
func testTarball(t *testing.T, version, arch string) []byte {
	t.Helper()
	root := "lexicon-" + version + "-linux-" + arch
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	var internalSums bytes.Buffer
	add := func(name string, data []byte, mode int64) {
		t.Helper()
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range binaryNames {
		data := []byte("new " + name)
		add("./"+root+"/bin/"+name, data, 0o755)
		fmt.Fprintf(&internalSums, "%s  ./bin/%s\n", sum(data), name)
	}
	for _, name := range []string{
		"share/doc/lexicon/INSTALL.md",
		"share/licenses/lexicon/LICENSE",
	} {
		add(root+"/"+name, []byte("documentation"), 0o644)
	}
	add("./"+root+"/checksums.sha256", internalSums.Bytes(), 0o644)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	// Release assets are .tar.gz; the plan must decompress before tar parsing.
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err := gzipWriter.Write(archive.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}

func TestRunnerPlanVerifiesAndStages(t *testing.T) {
	binDir := t.TempDir()
	for _, name := range binaryNames {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("old "+name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)
	dictionaryPath := filepath.Join(dataDir, "lexicon", "dictionary.sqlite")
	if err := os.MkdirAll(filepath.Dir(dictionaryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dictionaryPath, []byte("installed"), 0o600); err != nil {
		t.Fatal(err)
	}
	dict := testDictionaryBytes(t)
	manifest := []byte(fmt.Sprintf(`{"name":"Test","file":"en.sqlite","bytes":%d,"sha256":"%s"}`, len(dict), sum(dict)))

	tarball := testTarball(t, "9.9.9", "amd64")
	releaseSums := []byte(fmt.Sprintf("%s  lexicon-9.9.9-linux-amd64.tar.gz\n", sum(tarball)))
	var corruptDictionary atomic.Bool

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/assets/lexicon-9.9.9-linux-amd64.tar.gz":
			_, _ = w.Write(tarball)
		case "/assets/checksums.sha256":
			_, _ = w.Write(releaseSums)
		case "/download/dictionary-3.0/en.sqlite":
			if corruptDictionary.Load() {
				_, _ = w.Write([]byte("bad"))
			} else {
				_, _ = w.Write(dict)
			}
		case "/download/dictionary-3.0/manifest.json":
			_, _ = w.Write(manifest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	runner := &Runner{
		Discoverer: fakeDiscoverer{release: Release{
			Tag: "v9.9.9", Version: "9.9.9",
			Assets: map[string]string{
				"lexicon-9.9.9-linux-amd64.tar.gz": srv.URL + "/assets/lexicon-9.9.9-linux-amd64.tar.gz",
				"checksums.sha256":                 srv.URL + "/assets/checksums.sha256",
			},
		}},
		Client:      srv.Client(),
		Restarter:   &fakeRestarter{},
		ReleaseBase: srv.URL + "/download",
		Arch:        "amd64",
		Executable:  func() (string, error) { return filepath.Join(binDir, "lexicon"), nil },
		EnSQLiteSHA: sum(dict),
		ManifestSHA: sum(manifest),
	}
	plan, err := runner.Plan(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Version != "9.9.9" || plan.BinDir != binDir || plan.DictionaryPath != dictionaryPath {
		t.Fatalf("plan = %+v", plan)
	}
	if string(plan.Binaries["lexicon"]) != "new lexicon" {
		t.Fatalf("lexicon = %q", plan.Binaries["lexicon"])
	}
	if !bytes.Equal(plan.Dictionary, dict) {
		t.Fatal("dictionary bytes were not staged")
	}

	corruptDictionary.Store(true)
	if _, err := runner.Plan(context.Background(), Options{}); err == nil {
		t.Fatal("checksum mismatch was accepted")
	}
}

func TestRunnerPlanCheckDownloadsNothing(t *testing.T) {
	binDir := t.TempDir()
	for _, name := range binaryNames {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("old "+name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)
	dictionaryPath := filepath.Join(dataDir, "lexicon", "dictionary.sqlite")
	if err := os.MkdirAll(filepath.Dir(dictionaryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dictionaryPath, []byte("installed"), 0o600); err != nil {
		t.Fatal(err)
	}

	var requests atomic.Int64
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		t.Errorf("unexpected HTTP request: %s", r.URL.Path)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	runner := &Runner{
		Discoverer:  fakeDiscoverer{release: Release{Tag: "v9.9.9", Version: "9.9.9"}},
		Client:      srv.Client(),
		Restarter:   &fakeRestarter{},
		ReleaseBase: srv.URL + "/download",
		Arch:        "amd64",
		Executable:  func() (string, error) { return filepath.Join(binDir, "lexicon"), nil },
	}
	plan, err := runner.Plan(context.Background(), Options{Check: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Version != "9.9.9" || plan.Tag != "v9.9.9" || plan.BinDir != binDir || plan.DictionaryPath != dictionaryPath {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.DictionarySkipped != "" {
		t.Fatalf("dictionary unexpectedly skipped: %q", plan.DictionarySkipped)
	}
	if plan.Binaries != nil || plan.Dictionary != nil {
		t.Fatalf("--check staged assets: %+v", plan)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("HTTP requests = %d, want 0", got)
	}
}
