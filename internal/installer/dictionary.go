package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PandaX185/lexicon/internal/dictionary"
)

type packagedDictionary struct {
	path     string
	manifest *dictionary.Manifest
}

// findPackagedDictionary returns the first distribution-installed dictionary.
// When a manifest is present its size and checksum must match; a broken package
// is an error rather than a silent fallback.
func findPackagedDictionary(dirs []string) (*packagedDictionary, error) {
	for _, dir := range dirs {
		path := filepath.Join(dir, "en.sqlite")
		info, err := os.Stat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		case err != nil:
			return nil, fmt.Errorf("inspect packaged dictionary %s: %w", path, err)
		case !info.Mode().IsRegular():
			continue
		}
		manifest, err := dictionary.ReadManifest(filepath.Join(dir, "manifest.json"))
		switch {
		case errors.Is(err, os.ErrNotExist):
			return &packagedDictionary{path: path}, nil
		case err != nil:
			return nil, fmt.Errorf("read packaged dictionary manifest: %w", err)
		}
		if err := manifest.Verify(path); err != nil {
			return nil, fmt.Errorf("verify packaged dictionary %s: %w", path, err)
		}
		return &packagedDictionary{path: path, manifest: &manifest}, nil
	}
	return nil, nil
}

// downloadDictionary fetches an explicit URL into a temporary file and verifies
// its checksum before anything is installed.
func downloadDictionary(ctx context.Context, rawURL, expectedSHA string, client *http.Client) (string, error) {
	if strings.TrimSpace(expectedSHA) == "" {
		return "", errors.New("a dictionary URL requires a SHA-256 checksum")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse dictionary URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return "", errors.New("dictionary URL must use https")
	}

	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download dictionary: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download dictionary: %s", response.Status)
	}

	temp, err := os.CreateTemp("", "lexicon-dictionary-*.sqlite")
	if err != nil {
		return "", err
	}
	tempName := temp.Name()
	fail := func(cause error) (string, error) {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return "", cause
	}

	const maxBytes = 512 << 20
	written, err := io.Copy(temp, io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return fail(fmt.Errorf("download dictionary: %w", err))
	}
	if written > maxBytes {
		return fail(errors.New("downloaded dictionary exceeds 512 MiB"))
	}
	if err := temp.Sync(); err != nil {
		return fail(err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return "", err
	}
	if err := dictionary.VerifySHA256(tempName, expectedSHA); err != nil {
		_ = os.Remove(tempName)
		return "", fmt.Errorf("downloaded dictionary: %w", err)
	}
	return tempName, nil
}

func validateDictionary(ctx context.Context, path string) error {
	store, err := dictionary.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	if err := store.ValidateLookupSchema(); err != nil {
		return err
	}
	info, err := store.Describe(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(info.Name) == "" {
		return errors.New("dictionary metadata has no name")
	}
	return nil
}

// ValidateDictionary opens path read-only and checks the lookup schema and
// metadata. The TUI uses it to validate a path before showing the summary.
func ValidateDictionary(ctx context.Context, path string) error {
	return validateDictionary(ctx, path)
}

func copyFile(source, target string) error {
	//nolint:gosec // source is the dictionary path chosen by the installer user
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()

	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".dictionary-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()

	if _, err := io.Copy(temp, input); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempName, 0o600); err != nil {
		return err
	}
	return os.Rename(tempName, target)
}
