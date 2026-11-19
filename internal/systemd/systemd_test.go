package systemd

import (
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
