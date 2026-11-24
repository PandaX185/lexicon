package systemd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderUnitDeclaresRuntimeDirectory(t *testing.T) {
	unit := RenderUnit("/home/user/.local/bin/lexicon-daemon", "/home/user/.config/lexicon/config.toml")

	// ReadWritePaths=%t/lexicon needs the directory to exist before systemd
	// builds the mount namespace; RuntimeDirectory creates it before exec.
	if !strings.Contains(unit, "\nRuntimeDirectory=lexicon\n") {
		t.Fatalf("rendered unit must declare RuntimeDirectory=lexicon, got:\n%s", unit)
	}
}

func TestPackagedUnitDeclaresRuntimeDirectory(t *testing.T) {
	path := filepath.Join("..", "..", "packaging", "systemd", "lexicon.service")
	unit, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read packaged unit: %v", err)
	}

	if !strings.Contains(string(unit), "\nRuntimeDirectory=lexicon\n") {
		t.Fatalf("packaged unit must declare RuntimeDirectory=lexicon, got:\n%s", unit)
	}
}

func TestAvailableReflectsSystemctlPresence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if Available() {
		t.Fatal("Available = true with no systemctl on PATH")
	}
	script := filepath.Join(dir, "systemctl")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !Available() {
		t.Fatal("Available = false with systemctl on PATH")
	}
}

func TestRestartCallsSystemctlUserRestart(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	script := filepath.Join(dir, "systemctl")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$LEXICON_SYSTEMCTL_ARGS\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("LEXICON_SYSTEMCTL_ARGS", argsPath)

	if err := Restart(context.Background()); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	got, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "--user\nrestart\nlexicon.service\n" {
		t.Fatalf("systemctl args = %q", got)
	}
}
