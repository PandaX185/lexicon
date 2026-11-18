package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
