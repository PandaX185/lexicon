package update

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// checksumFor finds a SHA-256 line for name, accepting both "hash  name" and
// "./name" forms used by the release checksums files.
func checksumFor(data []byte, name string) (string, error) {
	wanted := strings.TrimPrefix(name, "./")
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "./") != wanted {
			continue
		}
		return strings.ToLower(strings.TrimSpace(fields[0])), nil
	}
	return "", fmt.Errorf("no checksum for %s", name)
}

func verifyBytes(data []byte, want, name string) error {
	want = strings.ToLower(strings.TrimSpace(want))
	if len(want) != sha256.Size*2 {
		return fmt.Errorf("invalid checksum for %s: want 64 hex characters", name)
	}
	if _, err := hex.DecodeString(want); err != nil {
		return fmt.Errorf("invalid checksum for %s: %w", name, err)
	}
	if got := sha256Hex(data); got != want {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, want)
	}
	return nil
}

// isBinaryDir reports whether dir is an accepted location for a release
// binary: the bare bin directory, or bin directly under a single safe release
// root such as lexicon-0.2.0-linux-amd64. Roots follow the tarball naming used
// by TarballName; share/bin/lexicon and deeper nesting are not accepted.
func isBinaryDir(dir string) bool {
	if dir == "bin" {
		return true
	}
	if path.Base(dir) != "bin" {
		return false
	}
	root := path.Dir(dir)
	return root != "" && root != "." && root != ".." && root != "/" &&
		!strings.Contains(root, "/") && strings.HasPrefix(root, "lexicon-")
}

// extractBinaries extracts the four release binaries from a tarball, accepts
// either bin/<name> or <release-root>/bin/<name>, ignores harmless non-binary
// regular files, rejects links and unsafe paths, and verifies each binary
// against the tarball's internal checksums file.
func extractBinaries(tarball, internalChecksums []byte) (map[string][]byte, error) {
	allowed := make(map[string]bool, len(binaryNames))
	for _, name := range binaryNames {
		allowed[name] = true
	}
	binaries := make(map[string][]byte, len(binaryNames))
	reader := tar.NewReader(bytes.NewReader(tarball))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tarball: %w", err)
		}
		name := path.Clean(header.Name)
		if path.IsAbs(header.Name) || strings.Contains(header.Name, "..") || strings.Contains(header.Name, `\`) {
			return nil, fmt.Errorf("unsafe tarball entry %q", header.Name)
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		// tar.Reader normalizes the deprecated TypeRegA ("\x00") to TypeReg.
		if header.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("unsupported tarball entry %q", header.Name)
		}
		base := path.Base(name)
		if !allowed[base] {
			continue // harmless non-binary file; it is never extracted
		}
		if !isBinaryDir(path.Dir(name)) {
			return nil, fmt.Errorf("unexpected tarball entry %q", header.Name)
		}
		if header.Size > maxBinaryBytes {
			return nil, fmt.Errorf("binary %s exceeds %d bytes", base, maxBinaryBytes)
		}
		data, err := io.ReadAll(io.LimitReader(reader, maxBinaryBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read tarball entry %s: %w", header.Name, err)
		}
		if int64(len(data)) > maxBinaryBytes {
			return nil, fmt.Errorf("binary %s exceeds %d bytes", base, maxBinaryBytes)
		}
		want, err := checksumFor(internalChecksums, "./bin/"+base)
		if err != nil {
			return nil, err
		}
		if err := verifyBytes(data, want, "./bin/"+base); err != nil {
			return nil, err
		}
		binaries[base] = data
	}
	for _, name := range binaryNames {
		if _, ok := binaries[name]; !ok {
			return nil, fmt.Errorf("tarball is missing %s", name)
		}
	}
	return binaries, nil
}
