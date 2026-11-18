package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestLoadMissingUsesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg, exists, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if exists {
		t.Fatal("exists = true for a missing file")
	}
	if cfg.Dictionary.SourceLanguage != "en" || cfg.General.MaxQueryWords != 4 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	cfg.General.MaxQueryWords = 2
	cfg.Daemon.LogLevel = "debug"
	cfg.Selection.Provider = "wl-paste"
	cfg.Popup.AutoLaunch = false
	cfg.Dictionary.DatabasePath = "/tmp/dictionary.sqlite"

	if err := Write(path, cfg); err != nil {
		t.Fatalf("write: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 0600", info.Mode().Perm())
	}

	loaded, exists, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !exists {
		t.Fatal("exists = false")
	}
	if loaded.General.MaxQueryWords != 2 || loaded.Daemon.LogLevel != "debug" ||
		loaded.Selection.Provider != "wl-paste" || loaded.Popup.AutoLaunch {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[general]\nmax_query_words = 3\nunknown = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path); err == nil {
		t.Fatal("expected an unknown-key error")
	}
}

func TestLoadIgnoresRemovedHideOnBlurKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := "[popup]\nhide_on_blur = true\nwidth = 500\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, exists, err := Load(path)
	if err != nil {
		t.Fatalf("load legacy config: %v", err)
	}
	if !exists {
		t.Fatal("exists = false")
	}
	if cfg.Popup.Width != 500 {
		t.Fatalf("width = %d, want 500", cfg.Popup.Width)
	}
	if cfg.Popup.AutoLaunch != Default().Popup.AutoLaunch {
		t.Fatalf("auto_launch = %v, want the default", cfg.Popup.AutoLaunch)
	}
	rendered, err := Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(rendered), "hide_on_blur") {
		t.Fatalf("rendered config still exposes hide_on_blur:\n%s", rendered)
	}
}

func TestLoadRejectsUnknownPopupKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[popup]\nunknown = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path); err == nil {
		t.Fatal("expected an unknown-key error")
	}
}

func TestLoadRejectsInvalidValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[general]\nmax_query_words = 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path); err == nil || !strings.Contains(err.Error(), "max_query_words") {
		t.Fatalf("error = %v, want max_query_words validation", err)
	}
}

func TestParseLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for value, want := range cases {
		got, err := ParseLogLevel(value)
		if err != nil {
			t.Fatalf("ParseLogLevel(%q): %v", value, err)
		}
		if got != want {
			t.Fatalf("ParseLogLevel(%q) = %v, want %v", value, got, want)
		}
	}
	if _, err := ParseLogLevel("verbose"); err == nil {
		t.Fatal("expected an unknown-level error")
	}
}

func TestSetAndGet(t *testing.T) {
	cfg := Default()
	if err := Set(&cfg, "popup.auto_launch", "false"); err != nil {
		t.Fatalf("set bool: %v", err)
	}
	value, err := Get(cfg, "popup.auto_launch")
	if err != nil || value != "false" {
		t.Fatalf("get bool = %q, %v", value, err)
	}

	if err := Set(&cfg, "general.max_query_words", "6"); err != nil {
		t.Fatalf("set int: %v", err)
	}
	if value, _ := Get(cfg, "general.max_query_words"); value != "6" {
		t.Fatalf("get int = %q", value)
	}

	if err := Set(&cfg, "dictionary.database_path", "/tmp/x.sqlite"); err != nil {
		t.Fatalf("set string: %v", err)
	}
	if value, _ := Get(cfg, "dictionary.database_path"); value != "/tmp/x.sqlite" {
		t.Fatalf("get string = %q", value)
	}

	if err := Set(&cfg, "general.max_query_words", "99"); err == nil {
		t.Fatal("expected range validation")
	}
	if err := Set(&cfg, "popup.auto_launch", "maybe"); err == nil {
		t.Fatal("expected bool validation")
	}
	if _, err := Get(cfg, "general.nope"); err == nil {
		t.Fatal("expected unknown-key error")
	}
	if err := Set(&cfg, "general", "x"); err == nil {
		t.Fatal("expected non-leaf error")
	}
}

func TestKeys(t *testing.T) {
	keys := Keys()
	if len(keys) < 10 {
		t.Fatalf("keys = %v, want the full schema", keys)
	}
	found := false
	for _, key := range keys {
		if key == "dictionary.database_path" {
			found = true
		}
	}
	if !found {
		t.Fatalf("dictionary.database_path missing from %v", keys)
	}
}

func TestEffectiveResolvesXDGPaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))

	effective := Effective(Default())
	if effective.Daemon.Socket != filepath.Join(root, "run", "lexicon", "daemon.sock") {
		t.Fatalf("socket = %q", effective.Daemon.Socket)
	}
	if effective.Dictionary.DatabasePath != filepath.Join(root, "data", "lexicon", "dictionary.sqlite") {
		t.Fatalf("dictionary = %q", effective.Dictionary.DatabasePath)
	}
	if effective.Translation.ModelDirectory != filepath.Join(root, "data", "lexicon", "models") {
		t.Fatalf("models = %q", effective.Translation.ModelDirectory)
	}
}

func TestEnsure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	created, err := Ensure(path)
	if err != nil || !created {
		t.Fatalf("ensure = %v, %v", created, err)
	}
	created, err = Ensure(path)
	if err != nil || created {
		t.Fatalf("second ensure = %v, %v", created, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	}
}
