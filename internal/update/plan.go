package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/paths"
	"github.com/PandaX185/lexicon/internal/systemd"
)

// Restarter activates a swapped daemon binary. Tests use a fake.
type Restarter interface {
	Restart(context.Context) error
}

// ErrRestartUnavailable reports that systemctl is absent. The swapped files
// remain installed; the caller surfaces the notice instead of rolling back.
var ErrRestartUnavailable = errors.New("systemctl is unavailable; the service was not restarted")

type systemdRestarter struct{}

func (systemdRestarter) Restart(ctx context.Context) error {
	if !systemd.Available() {
		return ErrRestartUnavailable
	}
	return systemd.Restart(ctx)
}

// Runner performs verified updates. Its injectable fields keep tests offline.
type Runner struct {
	Discoverer  Discoverer
	Client      HTTPClient
	Restarter   Restarter
	ReleaseBase string                 // default ReleaseBase
	Arch        string                 // default runtime.GOARCH
	Executable  func() (string, error) // default os.Executable
	EnSQLiteSHA string                 // default EnSQLiteSHA256
	ManifestSHA string                 // default ManifestSHA256
	NoRestart   bool                   // CLI --no-restart
}

func NewRunner() *Runner {
	return &Runner{
		Discoverer:  GitHubDiscoverer{},
		Client:      &http.Client{Timeout: 2 * time.Minute},
		Restarter:   systemdRestarter{},
		ReleaseBase: ReleaseBase,
		Arch:        runtime.GOARCH,
		Executable:  os.Executable,
		EnSQLiteSHA: EnSQLiteSHA256,
		ManifestSHA: ManifestSHA256,
	}
}

// Options selects the release and target paths.
type Options struct {
	Version    string // empty means latest stable
	ConfigPath string // empty means the standard config path
	BinDir     string // empty means the directory of the running binary
	Check      bool   // report versions without downloading any assets
}

// Plan is a fully verified update, ready to apply.
type Plan struct {
	Version           string
	Tag               string
	BinDir            string
	DictionaryPath    string
	DictionarySkipped string // non-empty means the dictionary is not replaced
	RestartSkipped    string // non-empty when systemctl was unavailable
	TarballName       string // verified release tarball asset name
	TarballSHA256     string // verified SHA-256 of the release tarball
	Binaries          map[string][]byte
	Dictionary        []byte
}

func (r *Runner) discover(ctx context.Context, version string) (Release, error) {
	discoverer := r.Discoverer
	if discoverer == nil {
		discoverer = GitHubDiscoverer{Client: r.Client}
	}
	if version == "" {
		return discoverer.Latest(ctx)
	}
	return discoverer.Version(ctx, version)
}

func (r *Runner) resolveBinDir(override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	executable := r.Executable
	if executable == nil {
		executable = os.Executable
	}
	path, err := executable()
	if err != nil {
		return "", fmt.Errorf("resolve running binary: %w", err)
	}
	binDir := filepath.Dir(path)
	for _, name := range binaryNames {
		info, err := os.Stat(filepath.Join(binDir, name))
		if err != nil {
			return "", fmt.Errorf("binary %s is missing from %s: %w", name, binDir, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("binary %s is not a regular file", name)
		}
	}
	return binDir, nil
}

func (r *Runner) resolveDictionaryPath(configPath string) (string, string, error) {
	path := ""
	if configPath == "" {
		resolved, err := paths.DefaultDictionaryPath()
		if err != nil {
			return "", "", err
		}
		path = resolved
	} else {
		cfg, _, err := config.Load(configPath)
		if err != nil {
			return "", "", err
		}
		path = config.Effective(cfg).Dictionary.DatabasePath
	}
	if path == "" {
		resolved, err := paths.DefaultDictionaryPath()
		if err != nil {
			return "", "", err
		}
		path = resolved
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return path, "dictionary is not installed; skipping", nil
	}
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return path, "dictionary path is not a regular file; skipping", nil
	}
	if !dictionaryDirWritable(path) {
		return path, "dictionary directory is read-only; skipping", nil
	}
	return path, "", nil
}

// dictionaryDirWritable reports whether a temporary file can be created next
// to the dictionary, which is needed later to stage a replacement.
func dictionaryDirWritable(path string) bool {
	temp, err := os.CreateTemp(filepath.Dir(path), ".lexicon-write-test-*")
	if err != nil {
		return false
	}
	name := temp.Name()
	_ = temp.Close()
	_ = os.Remove(name)
	return true
}

// Plan discovers a release, verifies every asset, and returns staged bytes.
func (r *Runner) Plan(ctx context.Context, opts Options) (*Plan, error) {
	release, err := r.discover(ctx, opts.Version)
	if err != nil {
		return nil, err
	}
	binDir, err := r.resolveBinDir(opts.BinDir)
	if err != nil {
		return nil, err
	}
	dictionaryPath, skip, err := r.resolveDictionaryPath(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	if opts.Check {
		return &Plan{
			Version: release.Version, Tag: release.Tag, BinDir: binDir,
			DictionaryPath: dictionaryPath, DictionarySkipped: skip,
		}, nil
	}
	arch := r.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}
	tarballName, err := TarballName(release.Version, arch)
	if err != nil {
		return nil, err
	}
	tarballURL, ok := release.Assets[tarballName]
	if !ok || tarballURL == "" {
		return nil, fmt.Errorf("release %s has no %s asset", release.Tag, tarballName)
	}
	sumsURL, ok := release.Assets["checksums.sha256"]
	if !ok || sumsURL == "" {
		return nil, fmt.Errorf("release %s has no checksums.sha256 asset", release.Tag)
	}
	tarball, err := fetch(ctx, r.Client, tarballURL, maxTarballBytes)
	if err != nil {
		return nil, err
	}
	releaseSums, err := fetch(ctx, r.Client, sumsURL, maxChecksumsBytes)
	if err != nil {
		return nil, err
	}
	want, err := checksumFor(releaseSums, tarballName)
	if err != nil {
		return nil, err
	}
	if err := verifyBytes(tarball, want, tarballName); err != nil {
		return nil, err
	}
	uncompressed, err := gunzip(tarball, maxTarballBytes)
	if err != nil {
		return nil, err
	}
	internalSumsName := "./" + strings.TrimSuffix(tarballName, ".tar.gz") + "/checksums.sha256"
	internalSums, err := fetchInternalChecksums(uncompressed, internalSumsName, maxChecksumsBytes)
	if err != nil {
		return nil, err
	}
	binaries, err := extractBinaries(uncompressed, internalSums)
	if err != nil {
		return nil, err
	}

	plan := &Plan{
		Version: release.Version, Tag: release.Tag, BinDir: binDir,
		DictionaryPath: dictionaryPath, DictionarySkipped: skip, Binaries: binaries,
		TarballName: tarballName, TarballSHA256: want,
	}
	if skip != "" {
		return plan, nil
	}
	base := r.ReleaseBase
	if base == "" {
		base = ReleaseBase
	}
	dictURL := strings.TrimRight(base, "/") + "/" + DictionaryTag + "/en.sqlite"
	manifestURL := strings.TrimRight(base, "/") + "/" + DictionaryTag + "/manifest.json"
	dictSHA := r.EnSQLiteSHA
	if dictSHA == "" {
		dictSHA = EnSQLiteSHA256
	}
	manifestSHA := r.ManifestSHA
	if manifestSHA == "" {
		manifestSHA = ManifestSHA256
	}
	dict, err := fetch(ctx, r.Client, dictURL, maxDictionaryBytes)
	if err != nil {
		return nil, err
	}
	manifest, err := fetch(ctx, r.Client, manifestURL, maxManifestBytes)
	if err != nil {
		return nil, err
	}
	if err := verifyBytes(dict, dictSHA, DictionaryTag+"/en.sqlite"); err != nil {
		return nil, err
	}
	if err := verifyBytes(manifest, manifestSHA, DictionaryTag+"/manifest.json"); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "lexicon-verify-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	dictFile := filepath.Join(dir, "en.sqlite")
	if err := os.WriteFile(dictFile, dict, 0o600); err != nil {
		return nil, err
	}
	manifestFile := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestFile, manifest, 0o600); err != nil {
		return nil, err
	}
	parsed, err := dictionary.ReadManifest(manifestFile)
	if err != nil {
		return nil, err
	}
	if err := parsed.Verify(dictFile); err != nil {
		return nil, fmt.Errorf("verify dictionary manifest: %w", err)
	}
	store, err := dictionary.Open(dictFile)
	if err != nil {
		return nil, err
	}
	if err := store.ValidateLookupSchema(); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("validate dictionary: %w", err)
	}
	if err := store.Close(); err != nil {
		return nil, err
	}
	plan.Dictionary = dict
	return plan, nil
}

// fetchInternalChecksums reads checksums.sha256 from a verified tarball.
func fetchInternalChecksums(tarball []byte, name string, maxBytes int64) ([]byte, error) {
	reader := tar.NewReader(bytes.NewReader(tarball))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("tarball has no %s", name)
		}
		if err != nil {
			return nil, err
		}
		if path.Clean(header.Name) != path.Clean(name) || header.Typeflag != tar.TypeReg {
			continue
		}
		if header.Size > maxBytes {
			return nil, fmt.Errorf("%s exceeds %d bytes", name, maxBytes)
		}
		data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxBytes {
			return nil, fmt.Errorf("%s exceeds %d bytes", name, maxBytes)
		}
		return data, nil
	}
}

// gunzip decompresses a verified gzip archive with a bounded output size.
func gunzip(data []byte, maxBytes int64) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open gzip archive: %w", err)
	}
	defer func() { _ = reader.Close() }()
	out, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("decompress gzip archive: %w", err)
	}
	if int64(len(out)) > maxBytes {
		return nil, fmt.Errorf("decompressed archive exceeds the %d byte limit", maxBytes)
	}
	return out, nil
}
