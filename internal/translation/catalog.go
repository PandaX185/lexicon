package translation

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

//go:embed catalog.json
var embeddedCatalog []byte

// CatalogEntry describes one model pack that Lexicon can install. Every entry
// is pinned to an HTTPS tarball and a SHA-256 checksum; nothing is downloaded
// implicitly.
type CatalogEntry struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Source        string        `json:"source"`
	Target        string        `json:"target"`
	Architecture  string        `json:"architecture"`
	GEMMPrecision string        `json:"gemm_precision"`
	License       string        `json:"license"`
	Attribution   string        `json:"attribution"`
	URL           string        `json:"url"`
	SHA256        string        `json:"sha256"`
	Size          int64         `json:"size"`
	Archive       ArchiveLayout `json:"archive"`
}

// ArchiveLayout maps files inside a release tarball to pack roles.
type ArchiveLayout struct {
	// Root is the directory inside the tarball that holds the model files.
	// Empty means the files are at the archive root.
	Root  string        `json:"root"`
	Files []ArchiveFile `json:"files"`
}

// ArchiveFile names one file relative to ArchiveLayout.Root. Its base name
// becomes the installed file name.
type ArchiveFile struct {
	Role string `json:"role"`
	Path string `json:"path"`
}

type catalogDocument struct {
	Version int            `json:"version"`
	Packs   []CatalogEntry `json:"packs"`
}

var (
	languageCodePattern = regexp.MustCompile(`^[a-z]{2,3}(-[a-z0-9]{2,8})*$`)
	sha256Pattern       = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Catalog returns the embedded, validated model catalog.
func Catalog() ([]CatalogEntry, error) {
	var document catalogDocument
	if err := json.Unmarshal(embeddedCatalog, &document); err != nil {
		return nil, fmt.Errorf("parse model catalog: %w", err)
	}
	if document.Version != 1 {
		return nil, fmt.Errorf("unsupported model catalog version %d", document.Version)
	}
	seen := make(map[string]bool, len(document.Packs))
	for i := range document.Packs {
		entry := &document.Packs[i]
		normalizeCatalogEntry(entry)
		if err := validateCatalogEntry(*entry); err != nil {
			return nil, fmt.Errorf("model catalog entry %q: %w", entry.ID, err)
		}
		if seen[entry.ID] {
			return nil, fmt.Errorf("model catalog has duplicate id %q", entry.ID)
		}
		seen[entry.ID] = true
	}
	return document.Packs, nil
}

// FindCatalogEntry returns the catalog entry with id.
func FindCatalogEntry(id string) (CatalogEntry, bool, error) {
	entries, err := Catalog()
	if err != nil {
		return CatalogEntry{}, false, err
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true, nil
		}
	}
	return CatalogEntry{}, false, nil
}

// CatalogEntryForPair returns the first catalog entry for source->target.
func CatalogEntryForPair(source, target string) (CatalogEntry, bool, error) {
	entries, err := Catalog()
	if err != nil {
		return CatalogEntry{}, false, err
	}
	source = CanonicalLanguageCode(source)
	target = CanonicalLanguageCode(target)
	for _, entry := range entries {
		if entry.Source == source && entry.Target == target {
			return entry, true, nil
		}
	}
	return CatalogEntry{}, false, nil
}

// ValidPackID reports whether id is safe to use as a pack directory name.
func ValidPackID(id string) bool { return safeName(id) }

// ValidLanguageCode reports whether code is a lowercase BCP 47-shaped tag.
func ValidLanguageCode(code string) bool {
	return languageCodePattern.MatchString(CanonicalLanguageCode(code))
}

func normalizeCatalogEntry(entry *CatalogEntry) {
	entry.Source = CanonicalLanguageCode(entry.Source)
	entry.Target = CanonicalLanguageCode(entry.Target)
}

func validateCatalogEntry(entry CatalogEntry) error {
	if !ValidPackID(entry.ID) {
		return errors.New("id must be a plain directory name")
	}
	if strings.TrimSpace(entry.Name) == "" {
		return errors.New("name is required")
	}
	if !ValidLanguageCode(entry.Source) || !ValidLanguageCode(entry.Target) {
		return errors.New("source and target must be lowercase BCP 47 tags")
	}
	if entry.Source == entry.Target {
		return errors.New("source and target must differ")
	}
	parsed, err := url.Parse(entry.URL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return errors.New("url must be an https URL without userinfo")
	}
	if !sha256Pattern.MatchString(entry.SHA256) {
		return errors.New("sha256 must be 64 lowercase hexadecimal characters")
	}
	if entry.Size <= 0 {
		return errors.New("size must be positive")
	}
	if entry.Archive.Root != "" && !safeArchivePath(entry.Archive.Root) {
		return errors.New("archive root must be a relative path without '..'")
	}

	roles := make(map[string]bool)
	destinations := make(map[string]bool)
	archived := make(map[string]bool)
	for _, file := range entry.Archive.Files {
		switch file.Role {
		case "model", "shortlist", "vocab", "srcVocab", "trgVocab":
		default:
			return fmt.Errorf("unknown file role %q", file.Role)
		}
		if !safeArchivePath(file.Path) {
			return fmt.Errorf("archive path %q must be relative without '..'", file.Path)
		}
		full := archivePath(entry.Archive.Root, file.Path)
		if archived[full] {
			return fmt.Errorf("archive path %q is listed twice", full)
		}
		archived[full] = true
		destination := path.Base(file.Path)
		if !safeName(destination) {
			return fmt.Errorf("archive path %q has an unsafe file name", file.Path)
		}
		if destinations[destination] {
			return fmt.Errorf("archive files share the installed name %q", destination)
		}
		destinations[destination] = true
		roles[file.Role] = true
	}
	if !roles["model"] {
		return errors.New("archive must provide a model file")
	}
	if !roles["shortlist"] {
		return errors.New("archive must provide a shortlist file")
	}
	if !roles["vocab"] && !roles["srcVocab"] && !roles["trgVocab"] {
		return errors.New("archive must provide a vocabulary file")
	}
	return nil
}

// safeArchivePath accepts clean, relative slash-separated paths. The archive
// reader never writes to a caller-supplied path, but rejecting traversal early
// keeps malformed catalogs and tarballs from ever reaching the filesystem.
func safeArchivePath(value string) bool {
	if value == "" || strings.Contains(value, `\`) || strings.HasPrefix(value, "/") {
		return false
	}
	cleaned := path.Clean(value)
	if cleaned != value {
		return false
	}
	return cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func archivePath(root, file string) string {
	if root == "" {
		return file
	}
	return path.Join(root, file)
}
