package dictionary

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashFileAndVerifySHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asset")
	if err := os.WriteFile(path, []byte("lexicon\n"), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	hash, size, err := HashFile(path)
	if err != nil {
		t.Fatalf("hash file: %v", err)
	}
	if size != int64(len("lexicon\n")) {
		t.Fatalf("size = %d, want %d", size, len("lexicon\n"))
	}
	if len(hash) != 64 {
		t.Fatalf("hash = %q, want 64 hex characters", hash)
	}
	if err := VerifySHA256(path, hash); err != nil {
		t.Fatalf("verify own hash: %v", err)
	}
	if err := VerifySHA256(path, strings.ToUpper(hash)); err != nil {
		t.Fatalf("verify uppercase hash: %v", err)
	}
	if err := VerifySHA256(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected a checksum mismatch")
	}
	if err := VerifySHA256(path, "not-a-hash"); err == nil {
		t.Fatal("expected a malformed checksum error")
	}
}

func TestManifestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	want := Manifest{
		Name:           "Test Dictionary",
		Version:        "1.0",
		File:           "en.sqlite",
		Bytes:          123,
		SHA256:         strings.Repeat("a", 64),
		Entries:        10,
		Definitions:    20,
		Forms:          30,
		AmbiguousForms: 1,
		ExceptionForms: 2,
		SourceLanguage: "en",
		TargetLanguage: "en",
		Licenses:       []string{"LICENSE.txt"},
	}
	if err := WriteManifest(path, want); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	got, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if got.Name != want.Name || got.Version != want.Version || got.SHA256 != want.SHA256 ||
		got.Entries != want.Entries || got.AmbiguousForms != want.AmbiguousForms ||
		len(got.Licenses) != 1 || got.Licenses[0] != "LICENSE.txt" {
		t.Fatalf("manifest = %+v, want %+v", got, want)
	}
}

func TestManifestVerify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "en.sqlite")
	if err := os.WriteFile(path, []byte("dictionary"), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	hash, size, err := HashFile(path)
	if err != nil {
		t.Fatalf("hash asset: %v", err)
	}
	manifest := Manifest{File: "en.sqlite", Bytes: size, SHA256: hash}
	if err := manifest.Verify(path); err != nil {
		t.Fatalf("verify manifest: %v", err)
	}

	manifest.Bytes++
	if err := manifest.Verify(path); err == nil {
		t.Fatal("expected a size mismatch")
	}
	manifest.Bytes = size
	manifest.File = "other.sqlite"
	if err := manifest.Verify(path); err == nil {
		t.Fatal("expected a filename mismatch")
	}
	manifest.File = "en.sqlite"
	manifest.SHA256 = ""
	if err := manifest.Verify(path); err == nil {
		t.Fatal("expected a missing-checksum error")
	}
}

func TestReadManifestMissing(t *testing.T) {
	_, err := ReadManifest(filepath.Join(t.TempDir(), "manifest.json"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want fs.ErrNotExist", err)
	}
}
