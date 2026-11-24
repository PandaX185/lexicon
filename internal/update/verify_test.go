package update

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func makeTarball(t *testing.T, entries map[string][]byte, kinds map[string]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := tar.NewWriter(&out)
	for _, name := range []string{
		"./bin/lexicon", "./bin/lexicon-daemon", "./bin/lexicon-installer", "./bin/lexicon-popup",
	} {
		data := entries[name]
		kind := byte(tar.TypeReg)
		if k, ok := kinds[name]; ok {
			kind = k
		}
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data)), Typeflag: kind}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

type tarEntry struct {
	header tar.Header
	data   []byte
}

func buildTarball(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := tar.NewWriter(&out)
	for _, entry := range entries {
		header := entry.header
		header.Size = int64(len(entry.data))
		if err := writer.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// realReleaseEntries mirrors a published tarball: binaries live under a single
// versioned root, while the internal checksums file names them as ./bin/<name>.
func realReleaseEntries(root string) ([]tarEntry, []byte) {
	entries := make([]tarEntry, 0, len(binaryNames))
	var sums strings.Builder
	for _, name := range binaryNames {
		data := []byte("real " + name)
		entries = append(entries, tarEntry{
			header: tar.Header{Name: root + "/bin/" + name, Mode: 0o755, Typeflag: tar.TypeReg},
			data:   data,
		})
		fmt.Fprintf(&sums, "%s  ./bin/%s\n", hash(data), name)
	}
	return entries, []byte(sums.String())
}

func TestExtractBinariesVerifiesInternalChecksums(t *testing.T) {
	entries := map[string][]byte{}
	var sums strings.Builder
	for _, name := range []string{
		"./bin/lexicon", "./bin/lexicon-daemon", "./bin/lexicon-installer", "./bin/lexicon-popup",
	} {
		entries[name] = []byte(name)
		fmt.Fprintf(&sums, "%s  %s\n", hash(entries[name]), name)
	}
	binaries, err := extractBinaries(makeTarball(t, entries, nil), []byte(sums.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(binaries) != 4 || string(binaries["lexicon"]) != "./bin/lexicon" {
		t.Fatalf("binaries = %v", binaries)
	}
}

func TestExtractBinariesRejectsChecksumMismatch(t *testing.T) {
	entries := map[string][]byte{}
	var sums strings.Builder
	for _, name := range []string{
		"./bin/lexicon", "./bin/lexicon-daemon", "./bin/lexicon-installer", "./bin/lexicon-popup",
	} {
		entries[name] = []byte(name)
		value := hash(entries[name])
		if name == "./bin/lexicon" {
			value = hash([]byte("different"))
		}
		fmt.Fprintf(&sums, "%s  %s\n", value, name)
	}
	if _, err := extractBinaries(makeTarball(t, entries, nil), []byte(sums.String())); err == nil {
		t.Fatal("checksum mismatch was accepted")
	}
}

func TestExtractBinariesAcceptsRealReleaseLayout(t *testing.T) {
	const root = "lexicon-0.2.0-linux-amd64"
	entries, sums := realReleaseEntries(root)
	entries = append([]tarEntry{{header: tar.Header{Name: root + "/", Mode: 0o755, Typeflag: tar.TypeDir}}}, entries...)
	for _, name := range []string{
		root + "/share/doc/lexicon/INSTALL.md",
		root + "/share/licenses/lexicon/LICENSE",
		root + "/share/lexicon/lexicon.service",
		root + "/checksums.sha256",
	} {
		entries = append(entries, tarEntry{
			header: tar.Header{Name: name, Mode: 0o644, Typeflag: tar.TypeReg},
			data:   []byte("documentation"),
		})
	}

	binaries, err := extractBinaries(buildTarball(t, entries), sums)
	if err != nil {
		t.Fatal(err)
	}
	if len(binaries) != len(binaryNames) {
		t.Fatalf("binaries = %v, want %d entries", binaries, len(binaryNames))
	}
	for _, name := range binaryNames {
		if got := string(binaries[name]); got != "real "+name {
			t.Fatalf("binaries[%q] = %q, want %q", name, got, "real "+name)
		}
	}
}

func TestExtractBinariesRejectsBinaryLookalikesOutsideBin(t *testing.T) {
	const root = "lexicon-0.2.0-linux-amd64"
	entries, sums := realReleaseEntries(root)
	entries = append(entries, tarEntry{
		header: tar.Header{Name: "share/bin/lexicon", Mode: 0o755, Typeflag: tar.TypeReg},
		data:   []byte("evil"),
	})
	if _, err := extractBinaries(buildTarball(t, entries), sums); err == nil {
		t.Fatal("binary lookalike outside the release bin directory was accepted")
	}
}

func TestExtractBinariesRejectsUnsafeEntries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header tar.Header
		data   []byte
	}{
		{
			name:   "path traversal",
			header: tar.Header{Name: "../bin/lexicon", Mode: 0o755, Typeflag: tar.TypeReg},
			data:   []byte("evil"),
		},
		{
			name:   "absolute path",
			header: tar.Header{Name: "/bin/lexicon", Mode: 0o755, Typeflag: tar.TypeReg},
			data:   []byte("evil"),
		},
		{
			name:   "backslash path",
			header: tar.Header{Name: `bin\lexicon`, Mode: 0o755, Typeflag: tar.TypeReg},
			data:   []byte("evil"),
		},
		{
			name:   "symlink",
			header: tar.Header{Name: "bin/lexicon", Typeflag: tar.TypeSymlink, Linkname: "../evil"},
		},
		{
			name:   "hardlink",
			header: tar.Header{Name: "bin/lexicon", Typeflag: tar.TypeLink, Linkname: "../evil"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			archive := buildTarball(t, []tarEntry{{header: tc.header, data: tc.data}})
			if _, err := extractBinaries(archive, nil); err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
		})
	}
}

func TestVerifyBytes(t *testing.T) {
	data := []byte("asset")
	if err := verifyBytes(data, hash(data), "asset"); err != nil {
		t.Fatal(err)
	}
	if err := verifyBytes(data, strings.Repeat("0", 64), "asset"); err == nil {
		t.Fatal("wrong checksum was accepted")
	}
	for _, bad := range []string{strings.Repeat("0", 63), strings.Repeat("z", 64)} {
		if err := verifyBytes(data, bad, "asset"); err == nil || !strings.Contains(err.Error(), "invalid checksum") {
			t.Fatalf("verifyBytes with %q: err = %v, want an invalid checksum error", bad, err)
		}
	}
}
