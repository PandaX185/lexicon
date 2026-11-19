package daemon

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/PandaX185/lexicon/internal/config"
)

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func writePairTestConfig(t *testing.T, modelDir string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Selection.Provider = "none"
	cfg.Translation.Enabled = true
	cfg.Translation.ModelDirectory = modelDir
	path := filepath.Join(dir, "config.toml")
	if err := config.Write(path, cfg); err != nil {
		t.Fatal(err)
	}
	return path
}

func writePairPack(t *testing.T, dir, id, source, target string) {
	t.Helper()
	packDir := filepath.Join(dir, id)
	if err := os.MkdirAll(packDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model.bin", "shortlist.bin", "vocab.spm"} {
		if err := os.WriteFile(filepath.Join(packDir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	metadata := `{"format":1,"id":"` + id + `","source":"` + source + `","target":"` + target + `","engine":"bergamot","files":[` +
		`{"role":"model","file":"model.bin"},` +
		`{"role":"shortlist","file":"shortlist.bin"},` +
		`{"role":"vocab","file":"vocab.spm"}]}`
	if err := os.WriteFile(filepath.Join(packDir, "metadata.json"), []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSetLanguagePairValidatesAndReportsPair(t *testing.T) {
	modelDir := t.TempDir()
	writePairPack(t, modelDir, "en-de", "en", "de")
	configPath := writePairTestConfig(t, modelDir)

	d, err := New(configPath, newTestLogger(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.closeResources()

	state, err := d.SetLanguagePair(context.Background(), "EN", "DE")
	if err != nil {
		t.Fatalf("SetLanguagePair: %v", err)
	}
	if state.Source != "en" || state.Target != "de" || len(state.Pairs) != 1 {
		t.Fatalf("state = %+v", state)
	}

	offline, err := d.SetLanguagePair(context.Background(), "de", "fr")
	if err == nil {
		t.Fatalf("uninstalled pair succeeded: %+v", offline)
	}
}
