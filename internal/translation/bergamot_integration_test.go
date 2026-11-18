//go:build bergamot

package translation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBergamotTranslatesInstalledPack exercises the native bridge end to end
// against a real model pack. It is skipped unless LEXICON_BERGAMOT_TEST_PACK
// names the pack directory (the directory that holds metadata.json), because
// model packs are user data and never part of the source tree.
func TestBergamotTranslatesInstalledPack(t *testing.T) {
	packDir := os.Getenv("LEXICON_BERGAMOT_TEST_PACK")
	if packDir == "" {
		t.Skip("LEXICON_BERGAMOT_TEST_PACK is not set")
	}
	pack, err := readPack(packDir)
	if err != nil {
		t.Fatalf("read pack %s: %v", packDir, err)
	}
	if pack.Source == pack.Target {
		t.Fatalf("pack %s has an identical source and target", pack.ID)
	}

	service := New(Options{
		Enabled:        true,
		ModelDirectory: filepath.Dir(packDir),
		Timeout:        2 * time.Minute,
	})
	defer func() {
		if err := service.Close(); err != nil {
			t.Errorf("close service: %v", err)
		}
	}()
	if !service.Available() {
		t.Fatalf("engine unavailable: %s", service.Name())
	}
	if name := service.Name(); name != "bergamot" {
		t.Fatalf("engine name = %q, want bergamot", name)
	}

	text := os.Getenv("LEXICON_BERGAMOT_TEST_TEXT")
	if text == "" {
		text = "Hello world"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	translated, err := service.Translate(ctx, text, pack.Source, pack.Target)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	translated = strings.TrimSpace(translated)
	if translated == "" {
		t.Fatal("translation is empty")
	}
	if strings.EqualFold(translated, text) {
		t.Fatalf("translation was not translated: %q", translated)
	}
	t.Logf("%s->%s: %q", pack.Source, pack.Target, translated)
}

// TestBergamotReportsMissingPack checks that an enabled engine still reports a
// clean error when no pack covers the requested pair, so lookups keep their
// definitions.
func TestBergamotReportsMissingPack(t *testing.T) {
	dir := t.TempDir()
	service := New(Options{Enabled: true, ModelDirectory: dir, Timeout: time.Second})
	defer service.Close()
	if !service.Available() {
		t.Fatalf("engine unavailable: %s", service.Name())
	}
	if _, err := service.Translate(context.Background(), "hello", "en", "de"); err == nil {
		t.Fatal("expected an error for a missing model pack")
	}
}
