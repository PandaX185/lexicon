package translation

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxPackDownload = 512 << 20
	maxPackExtract  = 2 << 30
)

// InstallPack downloads one catalog entry, verifies its checksum, and installs
// it atomically under modelDir. It never replaces an existing pack.
func InstallPack(ctx context.Context, entry CatalogEntry, modelDir string, client *http.Client) (string, error) {
	normalizeCatalogEntry(&entry)
	if err := validateCatalogEntry(entry); err != nil {
		return "", err
	}
	if strings.TrimSpace(modelDir) == "" {
		return "", errors.New("a model directory is required")
	}

	target := filepath.Join(modelDir, entry.ID)
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("model pack %s is already installed at %s", entry.ID, target)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect model directory: %w", err)
	}
	if err := os.MkdirAll(modelDir, 0o700); err != nil {
		return "", fmt.Errorf("create model directory: %w", err)
	}

	archive, err := downloadPackArchive(ctx, entry, modelDir, client)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(archive) }()

	return installPackArchive(archive, entry, modelDir)
}

func downloadPackArchive(ctx context.Context, entry CatalogEntry, dir string, client *http.Client) (string, error) {
	httpClient := &http.Client{Timeout: 10 * time.Minute}
	if client != nil {
		copied := *client
		httpClient = &copied
	}
	if httpClient.CheckRedirect == nil {
		httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, entry.URL, nil)
	if err != nil {
		return "", fmt.Errorf("create model download request: %w", err)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("download model pack: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download model pack: %s", response.Status)
	}

	temp, err := os.CreateTemp(dir, ".pack-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("create download file: %w", err)
	}
	tempName := temp.Name()
	fail := func(cause error) (string, error) {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return "", cause
	}

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(response.Body, maxPackDownload+1))
	if err != nil {
		return fail(fmt.Errorf("download model pack: %w", err))
	}
	if written > maxPackDownload {
		return fail(errors.New("downloaded model pack exceeds 512 MiB"))
	}
	if written != entry.Size {
		return fail(fmt.Errorf("downloaded model pack is %d bytes, want %d", written, entry.Size))
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != entry.SHA256 {
		return fail(fmt.Errorf("model pack checksum mismatch: got %s, want %s", actual, entry.SHA256))
	}
	if err := temp.Sync(); err != nil {
		return fail(fmt.Errorf("sync model pack: %w", err))
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return "", fmt.Errorf("close model pack: %w", err)
	}
	return tempName, nil
}

type archiveDestination struct {
	role string
	name string
}

func installPackArchive(archive string, entry CatalogEntry, modelDir string) (string, error) {
	expected := make(map[string]archiveDestination, len(entry.Archive.Files))
	for _, file := range entry.Archive.Files {
		expected[archivePath(entry.Archive.Root, file.Path)] = archiveDestination{
			role: file.Role,
			name: path.Base(file.Path),
		}
	}

	//nolint:gosec // archive is the verified temp file this package downloaded
	source, err := os.Open(archive)
	if err != nil {
		return "", fmt.Errorf("open model pack: %w", err)
	}
	defer func() { _ = source.Close() }()

	gzipReader, err := gzip.NewReader(source)
	if err != nil {
		return "", fmt.Errorf("read model pack: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	staging, err := os.MkdirTemp(modelDir, ".install-"+entry.ID+"-*")
	if err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()

	extracted := make(map[string]bool, len(expected))
	var total int64
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read model pack: %w", err)
		}

		name := strings.TrimPrefix(header.Name, "./")
		if header.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		if !safeArchivePath(name) {
			return "", fmt.Errorf("model pack contains unsafe path %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			return "", fmt.Errorf("model pack contains unsupported entry %q", header.Name)
		}

		destination, wanted := expected[name]
		if !wanted {
			continue
		}
		if extracted[name] {
			return "", fmt.Errorf("model pack lists %q more than once", name)
		}
		written, err := extractArchiveFile(reader, filepath.Join(staging, destination.name), maxPackExtract-total)
		if err != nil {
			return "", fmt.Errorf("extract %q: %w", name, err)
		}
		total += written
		extracted[name] = true
	}

	for name := range expected {
		if !extracted[name] {
			return "", fmt.Errorf("model pack is missing %q", name)
		}
	}

	metadata := packMetadata{
		Format:        1,
		ID:            entry.ID,
		Source:        entry.Source,
		Target:        entry.Target,
		Engine:        "bergamot",
		Architecture:  entry.Architecture,
		GEMMPrecision: entry.GEMMPrecision,
		License:       entry.License,
		Attribution:   entry.Attribution,
	}
	for _, file := range entry.Archive.Files {
		metadata.Files = append(metadata.Files, packFileEntry{
			Role: file.Role,
			File: path.Base(file.Path),
		})
	}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode model metadata: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(filepath.Join(staging, "metadata.json"), encoded, 0o600); err != nil {
		return "", fmt.Errorf("write model metadata: %w", err)
	}

	pack, err := readPack(staging)
	if err != nil {
		return "", fmt.Errorf("validate installed model pack: %w", err)
	}
	if pack.ID != entry.ID || pack.Source != entry.Source || pack.Target != entry.Target {
		return "", errors.New("installed model pack does not match the catalog entry")
	}

	target := filepath.Join(modelDir, entry.ID)
	if err := os.Rename(staging, target); err != nil {
		return "", fmt.Errorf("install model pack: %w", err)
	}
	committed = true
	return target, nil
}

func extractArchiveFile(reader io.Reader, target string, remaining int64) (int64, error) {
	if remaining <= 0 {
		return 0, errors.New("model pack expands beyond 2 GiB")
	}
	//nolint:gosec // target is inside this package's staging directory
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	written, copyErr := io.Copy(file, io.LimitReader(reader, remaining+1))
	closeErr := file.Close()
	if copyErr != nil {
		return 0, copyErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if written > remaining {
		return 0, errors.New("model pack expands beyond 2 GiB")
	}
	return written, nil
}
