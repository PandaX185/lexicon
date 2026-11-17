package translation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanonicalLanguageCode(t *testing.T) {
	tests := map[string]string{
		"EN":   "en",
		" jp ": "ja", //nolint:gocritic // whitespace is the behavior under test
		"deu":  "de",
		"fre":  "fr",
		"ar":   "ar",
		"":     "",
	}
	for input, want := range tests {
		if got := CanonicalLanguageCode(input); got != want {
			t.Fatalf("CanonicalLanguageCode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFindPack(t *testing.T) {
	root := t.TempDir()
	packDir := filepath.Join(root, "en-de")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model.npz", "vocab.spm", "shortlist.bin"} {
		if err := os.WriteFile(filepath.Join(packDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	metadata := `{
		"format": 1,
		"id": "en-de",
		"source": "en",
		"target": "de",
		"engine": "bergamot",
		"architecture": "tiny",
		"gemm_precision": "int8",
		"files": [
			{"role": "model", "file": "model.npz"},
			{"role": "vocab", "file": "vocab.spm"},
			{"role": "shortlist", "file": "shortlist.bin"}
		]
	}`
	if err := os.WriteFile(filepath.Join(packDir, "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}

	pack, err := FindPack(root, "en", "de")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if pack.ID != "en-de" || len(pack.VocabPaths) != 1 {
		t.Fatalf("unexpected pack: %+v", pack)
	}

	config, err := pack.ModelConfig()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	for _, fragment := range []string{
		"models: [", "vocabs: [", "shortlist: [", "beam-size: 1", "gemm-precision: int8",
	} {
		if !strings.Contains(config, fragment) {
			t.Fatalf("config missing %q:\n%s", fragment, config)
		}
	}
	// A single shared vocabulary is passed twice.
	if strings.Count(config, filepath.Join(packDir, "vocab.spm")) != 2 {
		t.Fatalf("shared vocabulary not duplicated:\n%s", config)
	}

	if _, err := FindPack(root, "en", "fr"); !errors.Is(err, ErrNoModel) {
		t.Fatalf("error = %v, want ErrNoModel", err)
	}
	if _, err := FindPack(filepath.Join(root, "absent"), "en", "de"); !errors.Is(err, ErrNoModel) {
		t.Fatalf("error = %v, want ErrNoModel", err)
	}
}

func TestFindPackRejectsUnsafeNames(t *testing.T) {
	root := t.TempDir()
	packDir := filepath.Join(root, "bad")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := `{"engine":"bergamot","id":"bad","source":"en","target":"de","files":[
		{"role":"model","file":"../model.npz"},
		{"role":"vocab","file":"vocab.spm"},
		{"role":"shortlist","file":"shortlist.bin"}]}`
	if err := os.WriteFile(filepath.Join(packDir, "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FindPack(root, "en", "de"); !errors.Is(err, ErrNoModel) {
		t.Fatalf("error = %v, want ErrNoModel", err)
	}
}

func TestServiceDisabled(t *testing.T) {
	service := New(Options{Enabled: false})
	defer func() { _ = service.Close() }()
	if service.Available() {
		t.Fatal("disabled service is available")
	}
	if service.Name() != "disabled" {
		t.Fatalf("name = %q", service.Name())
	}
	if _, err := service.Translate(context.Background(), "hello", "en", "de"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("error = %v, want ErrDisabled", err)
	}
}

func TestServiceEnabledWithoutEngine(t *testing.T) {
	service := New(Options{Enabled: true, Timeout: time.Second})
	defer func() { _ = service.Close() }()
	if service.Available() {
		t.Skip("engine is compiled in")
	}
	if _, err := service.Translate(context.Background(), "hello", "en", "de"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestTranslateSameLanguageReturnsInput(t *testing.T) {
	service := &Service{enabled: true, engine: &fakeEngine{}}
	translated, err := service.Translate(context.Background(), " hello ", "en", "en")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if translated != "hello" {
		t.Fatalf("translated = %q", translated)
	}
}

type fakeEngine struct{}

func (fakeEngine) Name() string { return "fake" }
func (fakeEngine) Translate(context.Context, string, string) (string, error) {
	return "translated", nil
}
func (fakeEngine) Close() error { return nil }
