package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingRestarter struct{ calls int }

func (f *failingRestarter) Restart(context.Context) error {
	f.calls++
	return errors.New("restart failed")
}

type unavailableRestarter struct{}

func (unavailableRestarter) Restart(context.Context) error { return ErrRestartUnavailable }

func testPlan(t *testing.T, binDir, dictPath string) *Plan {
	t.Helper()
	binaries := map[string][]byte{}
	for _, name := range binaryNames {
		path := filepath.Join(binDir, name)
		if err := os.WriteFile(path, []byte("old "+name), 0o755); err != nil {
			t.Fatal(err)
		}
		binaries[name] = []byte("new " + name)
	}
	if err := os.WriteFile(dictPath, []byte("old dictionary"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &Plan{Version: "9.9.9", BinDir: binDir, DictionaryPath: dictPath, Binaries: binaries, Dictionary: []byte("new dictionary")}
}

func TestApplySwapsFilesAndRestarts(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	dictPath := filepath.Join(dir, "dictionary.sqlite")
	plan := testPlan(t, binDir, dictPath)
	restarter := &fakeRestarter{}

	if err := (&Runner{Restarter: restarter}).Apply(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(binDir, "lexicon"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new lexicon" {
		t.Fatalf("binary = %q", got)
	}
	got, err = os.ReadFile(dictPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new dictionary" {
		t.Fatalf("dictionary = %q", got)
	}
	if restarter.calls != 1 {
		t.Fatalf("restart calls = %d, want 1", restarter.calls)
	}
}

func TestApplyRestartFailureRollsBack(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	dictPath := filepath.Join(dir, "dictionary.sqlite")
	plan := testPlan(t, binDir, dictPath)
	restarter := &failingRestarter{}

	err := (&Runner{Restarter: restarter}).Apply(context.Background(), plan)
	if err == nil || !strings.Contains(err.Error(), "restart failed") {
		t.Fatalf("error = %v, want restart failure", err)
	}
	got, err := os.ReadFile(filepath.Join(binDir, "lexicon"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old lexicon" {
		t.Fatalf("binary after rollback = %q", got)
	}
	got, err = os.ReadFile(dictPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old dictionary" {
		t.Fatalf("dictionary after rollback = %q", got)
	}
	if restarter.calls != 2 {
		t.Fatalf("restart calls = %d, want 2 (new then old)", restarter.calls)
	}
}

func TestApplyMissingBinaryFailsBeforeAnySwap(t *testing.T) {
	dir := t.TempDir()
	plan := testPlan(t, dir, filepath.Join(dir, "dictionary.sqlite"))
	if err := os.Remove(filepath.Join(dir, "lexicon-daemon")); err != nil {
		t.Fatal(err)
	}
	if err := (&Runner{Restarter: &fakeRestarter{}}).Apply(context.Background(), plan); err == nil {
		t.Fatal("missing binary was accepted")
	}
	got, err := os.ReadFile(filepath.Join(dir, "lexicon"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old lexicon" {
		t.Fatalf("binary was swapped despite failure: %q", got)
	}
}

func TestApplyKeepsFilesWhenRestartUnavailable(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	dictPath := filepath.Join(dir, "dictionary.sqlite")
	plan := testPlan(t, binDir, dictPath)

	if err := (&Runner{Restarter: unavailableRestarter{}}).Apply(context.Background(), plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(binDir, "lexicon"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new lexicon" {
		t.Fatalf("binary after unavailable restart = %q, want new lexicon", got)
	}
	if plan.RestartSkipped == "" {
		t.Fatal("RestartSkipped was not set when systemctl was unavailable")
	}
}
