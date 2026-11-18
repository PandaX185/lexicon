package translation

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testCatalogEntry() CatalogEntry {
	return CatalogEntry{
		ID:            "test-en-de",
		Name:          "Test English → German",
		Source:        "en",
		Target:        "de",
		Architecture:  "tiny11",
		GEMMPrecision: "int8shiftAlphaAll",
		License:       "CC-BY-SA-4.0",
		Attribution:   "test",
		URL:           "https://example.invalid/pack.tar.gz",
		SHA256:        strings.Repeat("0", 64),
		Size:          1,
		Archive: ArchiveLayout{
			Root: "pack",
			Files: []ArchiveFile{
				{Role: "model", Path: "model.bin"},
				{Role: "shortlist", Path: "shortlist.bin"},
				{Role: "vocab", Path: "vocab.spm"},
			},
		},
	}
}

func TestCatalogIsValid(t *testing.T) {
	entries, err := Catalog()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("catalog is empty")
	}
	ids := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if ids[entry.ID] {
			t.Fatalf("duplicate catalog id %q", entry.ID)
		}
		ids[entry.ID] = true
		if entry.Source == entry.Target {
			t.Fatalf("%s has the same source and target", entry.ID)
		}
		if !strings.HasPrefix(entry.URL, "https://") {
			t.Fatalf("%s does not use https: %s", entry.ID, entry.URL)
		}
		if len(entry.SHA256) != 64 {
			t.Fatalf("%s has a malformed checksum", entry.ID)
		}
		if entry.Size <= 0 {
			t.Fatalf("%s has no size", entry.ID)
		}
	}
}

func TestCatalogEntryForPair(t *testing.T) {
	entry, found, err := CatalogEntryForPair("EN", "de")
	if err != nil {
		t.Fatalf("find pair: %v", err)
	}
	if !found {
		t.Fatal("en->de is not in the catalog")
	}
	if entry.Source != "en" || entry.Target != "de" {
		t.Fatalf("entry pair = %s->%s", entry.Source, entry.Target)
	}
}

func TestValidLanguageCode(t *testing.T) {
	for _, code := range []string{"en", "de", "zh-hant", "EN", "deu"} {
		if !ValidLanguageCode(code) {
			t.Fatalf("ValidLanguageCode(%q) = false", code)
		}
	}
	for _, code := range []string{"", "e", "english", "../en", "en_US"} {
		if ValidLanguageCode(code) {
			t.Fatalf("ValidLanguageCode(%q) = true", code)
		}
	}
}

func TestValidateCatalogEntry(t *testing.T) {
	tests := map[string]func(*CatalogEntry){
		"unsafe id": func(entry *CatalogEntry) {
			entry.ID = "../escape"
		},
		"http url": func(entry *CatalogEntry) {
			entry.URL = "http://example.invalid/pack.tar.gz"
		},
		"url userinfo": func(entry *CatalogEntry) {
			entry.URL = "https://user:pass@example.invalid/pack.tar.gz"
		},
		"bad checksum": func(entry *CatalogEntry) {
			entry.SHA256 = "abc"
		},
		"zero size": func(entry *CatalogEntry) {
			entry.Size = 0
		},
		"unsafe root": func(entry *CatalogEntry) {
			entry.Archive.Root = "../pack"
		},
		"unsafe path": func(entry *CatalogEntry) {
			entry.Archive.Files[0].Path = "/model.bin"
		},
		"duplicate destination": func(entry *CatalogEntry) {
			entry.Archive.Files[1].Path = "nested/model.bin"
		},
		"same languages": func(entry *CatalogEntry) {
			entry.Target = "en"
		},
		"no model": func(entry *CatalogEntry) {
			entry.Archive.Files = entry.Archive.Files[1:]
		},
		"no shortlist": func(entry *CatalogEntry) {
			entry.Archive.Files = []ArchiveFile{
				{Role: "model", Path: "model.bin"},
				{Role: "vocab", Path: "vocab.spm"},
			}
		},
		"no vocab": func(entry *CatalogEntry) {
			entry.Archive.Files = []ArchiveFile{
				{Role: "model", Path: "model.bin"},
				{Role: "shortlist", Path: "shortlist.bin"},
			}
		},
		"unknown role": func(entry *CatalogEntry) {
			entry.Archive.Files[0].Role = "weights"
		},
	}
	if err := validateCatalogEntry(testCatalogEntry()); err != nil {
		t.Fatalf("valid entry rejected: %v", err)
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			entry := testCatalogEntry()
			mutate(&entry)
			if err := validateCatalogEntry(entry); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

type archiveEntry struct {
	name     string
	body     string
	typeflag byte
	linkname string
}

func buildArchive(t *testing.T, entries ...archiveEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.name,
			Mode:     0o600,
			Size:     int64(len(entry.body)),
			Typeflag: entry.typeflag,
			Linkname: entry.linkname,
		}
		if header.Typeflag == 0 {
			header.Typeflag = tar.TypeReg
		}
		if header.Typeflag != tar.TypeReg {
			header.Size = 0
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := tarWriter.Write([]byte(entry.body)); err != nil {
				t.Fatalf("write tar body: %v", err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}

func testArchive(t *testing.T) []byte {
	t.Helper()
	return buildArchive(t,
		archiveEntry{name: "pack/", typeflag: tar.TypeDir},
		archiveEntry{name: "pack/model.bin", body: "model"},
		archiveEntry{name: "pack/shortlist.bin", body: "shortlist"},
		archiveEntry{name: "pack/vocab.spm", body: "vocab"},
		archiveEntry{name: "pack/unrelated.txt", body: "ignored"},
	)
}

func writeTestArchive(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pack.tar.gz")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	return path
}

func TestInstallPackFromArchive(t *testing.T) {
	modelDir := t.TempDir()
	archive := writeTestArchive(t, testArchive(t))

	target, err := installPackArchive(archive, testCatalogEntry(), modelDir)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if want := filepath.Join(modelDir, "test-en-de"); target != want {
		t.Fatalf("target = %q, want %q", target, want)
	}
	for name, want := range map[string]string{
		"model.bin":     "model",
		"shortlist.bin": "shortlist",
		"vocab.spm":     "vocab",
	} {
		got, err := os.ReadFile(filepath.Join(target, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "unrelated.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unrelated archive file was extracted: %v", err)
	}

	pack, err := FindPack(modelDir, "en", "de")
	if err != nil {
		t.Fatalf("find installed pack: %v", err)
	}
	if pack.ID != "test-en-de" || pack.License != "CC-BY-SA-4.0" {
		t.Fatalf("pack = %+v", pack)
	}
	config, err := pack.ModelConfig()
	if err != nil {
		t.Fatalf("model config: %v", err)
	}
	if !strings.Contains(config, "mini-batch-words: 1024") {
		t.Fatalf("model config is missing mini-batch-words:\n%s", config)
	}

	entries, err := os.ReadDir(modelDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".install-") {
			t.Fatalf("staging directory was left behind: %s", entry.Name())
		}
	}
}

func TestInstallPackRejectsTraversal(t *testing.T) {
	modelDir := t.TempDir()
	archive := writeTestArchive(t, buildArchive(t,
		archiveEntry{name: "pack/model.bin", body: "model"},
		archiveEntry{name: "pack/shortlist.bin", body: "shortlist"},
		archiveEntry{name: "pack/vocab.spm", body: "vocab"},
		archiveEntry{name: "../escaped.txt", body: "nope"},
	))
	if _, err := installPackArchive(archive, testCatalogEntry(), modelDir); err == nil {
		t.Fatal("expected an error for a traversal entry")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(modelDir), "escaped.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("traversal file was written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(modelDir, "escaped.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("traversal file was written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(modelDir, "test-en-de")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pack was installed after a failed extraction: %v", err)
	}
}

func TestInstallPackRejectsSymlink(t *testing.T) {
	modelDir := t.TempDir()
	archive := writeTestArchive(t, buildArchive(t,
		archiveEntry{name: "pack/model.bin", body: "model"},
		archiveEntry{name: "pack/shortlist.bin", body: "shortlist"},
		archiveEntry{name: "pack/vocab.spm", body: "vocab"},
		archiveEntry{name: "pack/link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"},
	))
	if _, err := installPackArchive(archive, testCatalogEntry(), modelDir); err == nil {
		t.Fatal("expected an error for a symlink entry")
	}
}

func TestInstallPackRejectsMissingFile(t *testing.T) {
	modelDir := t.TempDir()
	archive := writeTestArchive(t, buildArchive(t,
		archiveEntry{name: "pack/model.bin", body: "model"},
		archiveEntry{name: "pack/shortlist.bin", body: "shortlist"},
	))
	if _, err := installPackArchive(archive, testCatalogEntry(), modelDir); err == nil {
		t.Fatal("expected an error for a missing vocabulary")
	}
}

func TestInstallPackOverHTTP(t *testing.T) {
	data := testArchive(t)
	sum := sha256.Sum256(data)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer server.Close()

	entry := testCatalogEntry()
	entry.URL = server.URL + "/pack.tar.gz"
	entry.SHA256 = hex.EncodeToString(sum[:])
	entry.Size = int64(len(data))

	modelDir := t.TempDir()
	target, err := InstallPack(context.Background(), entry, modelDir, server.Client())
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := FindPack(modelDir, "en", "de"); err != nil {
		t.Fatalf("find installed pack: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "metadata.json")); err != nil {
		t.Fatalf("metadata: %v", err)
	}
}

func TestInstallPackRejectsChecksumMismatch(t *testing.T) {
	data := testArchive(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer server.Close()

	entry := testCatalogEntry()
	entry.URL = server.URL + "/pack.tar.gz"
	entry.Size = int64(len(data))

	modelDir := t.TempDir()
	_, err := InstallPack(context.Background(), entry, modelDir, server.Client())
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v, want a checksum mismatch", err)
	}
	entries, readErr := os.ReadDir(modelDir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("failed install left files behind: %v", entries)
	}
}

func TestInstallPackRefusesExisting(t *testing.T) {
	modelDir := t.TempDir()
	target := filepath.Join(modelDir, "test-en-de")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPack(context.Background(), testCatalogEntry(), modelDir, nil); err == nil {
		t.Fatal("expected an error for an existing pack")
	}
}

func TestInstallPackRejectsHTTP(t *testing.T) {
	entry := testCatalogEntry()
	entry.URL = "http://example.invalid/pack.tar.gz"
	if _, err := InstallPack(context.Background(), entry, t.TempDir(), nil); err == nil {
		t.Fatal("expected an error for an http URL")
	}
}

func TestInstalledPacks(t *testing.T) {
	modelDir := t.TempDir()
	for _, pair := range []struct{ source, target string }{{"en", "de"}, {"de", "en"}} {
		entry := testCatalogEntry()
		entry.ID = "test-" + pair.source + "-" + pair.target
		entry.Source = pair.source
		entry.Target = pair.target
		archive := writeTestArchive(t, testArchive(t))
		if _, err := installPackArchive(archive, entry, modelDir); err != nil {
			t.Fatalf("install %s: %v", entry.ID, err)
		}
	}
	packs, err := InstalledPacks(modelDir)
	if err != nil {
		t.Fatalf("installed packs: %v", err)
	}
	if len(packs) != 2 {
		t.Fatalf("got %d packs, want 2", len(packs))
	}
}
