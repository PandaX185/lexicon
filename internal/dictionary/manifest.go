package dictionary

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Manifest describes a released dictionary asset. Release builds publish it
// next to the database so installers and packages can verify integrity and
// report provenance without opening the database.
type Manifest struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	File           string   `json:"file"`
	Bytes          int64    `json:"bytes"`
	SHA256         string   `json:"sha256"`
	Entries        int      `json:"entries"`
	Definitions    int      `json:"definitions"`
	Forms          int      `json:"forms"`
	AmbiguousForms int      `json:"ambiguous_forms,omitempty"`
	ExceptionForms int      `json:"exception_forms,omitempty"`
	SourceLanguage string   `json:"source_language"`
	TargetLanguage string   `json:"target_language"`
	Licenses       []string `json:"licenses"`
}

// ReadManifest reads and decodes a release manifest. Missing files are
// reported as fs.ErrNotExist so callers can treat the manifest as optional.
func ReadManifest(path string) (Manifest, error) {
	//nolint:gosec // manifest path is chosen by the caller
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest %s: %w", path, err)
	}
	return manifest, nil
}

// WriteManifest writes a release manifest as indented JSON.
func WriteManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')
	//nolint:gosec // the manifest is a public build artifact
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	//nolint:gosec // the manifest is a public build artifact
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest %s: %w", path, err)
	}
	return nil
}

// HashFile returns the SHA-256 hex digest and size of path.
func HashFile(path string) (string, int64, error) {
	//nolint:gosec // path is a dictionary asset chosen by the caller
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), size, nil
}

// VerifySHA256 checks that path has the expected SHA-256 digest.
func VerifySHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if len(expected) != sha256.Size*2 {
		return fmt.Errorf("expected SHA-256 must be 64 hex characters, got %q", expected)
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return fmt.Errorf("expected SHA-256 is not hexadecimal: %w", err)
	}

	actual, _, err := HashFile(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

// Verify checks the asset against the manifest's size and checksum. A manifest
// without a checksum is rejected: the point of the side file is verification.
func (m Manifest) Verify(path string) error {
	if strings.TrimSpace(m.SHA256) == "" {
		return errors.New("manifest has no sha256")
	}
	if m.File != "" && filepath.Base(path) != m.File {
		return fmt.Errorf("manifest describes %s, not %s", m.File, filepath.Base(path))
	}
	if m.Bytes > 0 {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.Size() != m.Bytes {
			return fmt.Errorf("size mismatch: got %d bytes, want %d", info.Size(), m.Bytes)
		}
	}
	return VerifySHA256(path, m.SHA256)
}
