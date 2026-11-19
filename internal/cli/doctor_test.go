package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PandaX185/lexicon/internal/config"
)

func TestCheckSocketPermissions(t *testing.T) {
	dir := t.TempDir()

	secure := filepath.Join(dir, "secure.sock")
	if err := os.WriteFile(secure, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	status, detail := checkSocketPermissions(secure)
	if status != "ok" {
		t.Fatalf("secure socket: status = %s, detail = %s", status, detail)
	}
	if !strings.Contains(detail, "0600") {
		t.Fatalf("secure socket detail = %q, want the mode", detail)
	}

	loose := filepath.Join(dir, "loose.sock")
	if err := os.WriteFile(loose, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(loose, 0o666); err != nil {
		t.Fatal(err)
	}
	status, detail = checkSocketPermissions(loose)
	if status != "warn" {
		t.Fatalf("world-writable socket: status = %s, detail = %s", status, detail)
	}
	if !strings.Contains(detail, "0666") {
		t.Fatalf("loose socket detail = %q, want the mode", detail)
	}

	status, _ = checkSocketPermissions(filepath.Join(dir, "missing.sock"))
	if status != "warn" {
		t.Fatalf("missing socket: status = %s", status)
	}
}

func writeDoctorPack(t *testing.T, dir string) {
	t.Helper()
	packDir := filepath.Join(dir, "en-de")
	if err := os.MkdirAll(packDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model.bin", "shortlist.bin", "vocab.spm"} {
		if err := os.WriteFile(filepath.Join(packDir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	metadata := `{"format":1,"id":"en-de","source":"en","target":"de","engine":"bergamot","files":[` +
		`{"role":"model","file":"model.bin"},` +
		`{"role":"shortlist","file":"shortlist.bin"},` +
		`{"role":"vocab","file":"vocab.spm"}]}`
	if err := os.WriteFile(filepath.Join(packDir, "metadata.json"), []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReportMissingPacks(t *testing.T) {
	cfg := config.Default()
	cfg.Translation.Enabled = true
	cfg.Translation.ModelDirectory = t.TempDir()

	var got []diagnostic
	add := func(status, name, detail string) {
		got = append(got, diagnostic{status: status, name: name, detail: detail})
	}

	reportMissingPacks(cfg, true, add)
	if len(got) != 1 || got[0].status != "warn" || got[0].name != "language packs" {
		t.Fatalf("diagnostics = %+v, want one language-packs warning", got)
	}
	if !strings.Contains(got[0].detail, "popup") {
		t.Fatalf("warning detail = %q, want pair selection guidance", got[0].detail)
	}

	got = nil
	reportMissingPacks(cfg, false, add)
	disabled := cfg
	disabled.Translation.Enabled = false
	reportMissingPacks(disabled, true, add)
	if len(got) != 0 {
		t.Fatalf("diagnostics = %+v, want no warning when translation is unavailable or disabled", got)
	}

	writeDoctorPack(t, cfg.Translation.ModelDirectory)
	reportMissingPacks(cfg, true, add)
	if len(got) != 0 {
		t.Fatalf("diagnostics = %+v, want no warning when a pack is installed", got)
	}

	// An unreadable model directory is reported by the models diagnostic, not here.
	badDir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(badDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Translation.ModelDirectory = badDir
	reportMissingPacks(cfg, true, add)
	if len(got) != 0 {
		t.Fatalf("diagnostics = %+v, want no warning on a pack scan error", got)
	}
}

func TestDescribeModelDirectory(t *testing.T) {
	dir := t.TempDir()
	status, detail := describeModelDirectory(dir)
	if status != "ok" || !strings.Contains(detail, "0 pack(s)") {
		t.Fatalf("empty model directory: status = %s, detail = %s", status, detail)
	}

	status, _ = describeModelDirectory(filepath.Join(dir, "missing"))
	if status != "warn" {
		t.Fatalf("missing model directory: status = %s", status)
	}

	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	status, _ = describeModelDirectory(file)
	if status != "fail" {
		t.Fatalf("model directory that is a file: status = %s", status)
	}

	status, _ = describeModelDirectory("")
	if status != "fail" {
		t.Fatalf("empty model directory path: status = %s", status)
	}
}
