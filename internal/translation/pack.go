package translation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Pack is a validated Bergamot model pack installed on disk.
type Pack struct {
	ID            string
	Directory     string
	Source        string
	Target        string
	Architecture  string
	GEMMPrecision string
	License       string
	Attribution   string
	ModelPath     string
	ShortlistPath string
	VocabPaths    []string
}

type packMetadata struct {
	Format        int             `json:"format"`
	ID            string          `json:"id"`
	Source        string          `json:"source"`
	Target        string          `json:"target"`
	Engine        string          `json:"engine"`
	Architecture  string          `json:"architecture"`
	GEMMPrecision string          `json:"gemm_precision"`
	License       string          `json:"license"`
	Attribution   string          `json:"attribution"`
	Files         []packFileEntry `json:"files"`
}

type packFileEntry struct {
	Role string `json:"role"`
	File string `json:"file"`
}

// FindPack scans dir for a metadata.json describing source->target. Invalid
// packs are skipped; the first valid pack in sorted directory order wins.
func FindPack(dir, source, target string) (*Pack, error) {
	packs, err := scanPacks(dir)
	if err != nil {
		return nil, err
	}
	for _, pack := range packs {
		if pack.Source == source && pack.Target == target {
			return pack, nil
		}
	}
	return nil, ErrNoModel
}

// InstalledPacks returns every valid pack directly under dir in directory-name
// order. Invalid packs are skipped, matching FindPack.
func InstalledPacks(dir string) ([]*Pack, error) {
	return scanPacks(dir)
}

func scanPacks(dir string) ([]*Pack, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read model directory: %w", err)
	}
	packs := make([]*Pack, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pack, err := readPack(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		packs = append(packs, pack)
	}
	return packs, nil
}

func readPack(directory string) (*Pack, error) {
	//nolint:gosec // directory is a user-configured translation model pack
	data, err := os.ReadFile(filepath.Join(directory, "metadata.json"))
	if err != nil {
		return nil, err
	}
	var metadata packMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	if metadata.Engine != "bergamot" || metadata.Source == "" || metadata.Target == "" {
		return nil, errors.New("unsupported model pack")
	}
	if !safeName(metadata.ID) {
		return nil, errors.New("unsafe model pack id")
	}
	pack := &Pack{
		ID:            metadata.ID,
		Directory:     directory,
		Source:        CanonicalLanguageCode(metadata.Source),
		Target:        CanonicalLanguageCode(metadata.Target),
		Architecture:  metadata.Architecture,
		GEMMPrecision: metadata.GEMMPrecision,
		License:       metadata.License,
		Attribution:   metadata.Attribution,
	}
	for _, file := range metadata.Files {
		if !safeName(file.File) {
			return nil, errors.New("unsafe model file name")
		}
		path := filepath.Join(directory, file.File)
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("model file missing: %s", file.File)
		}
		switch file.Role {
		case "model":
			pack.ModelPath = path
		case "shortlist":
			pack.ShortlistPath = path
		case "vocab", "srcVocab", "trgVocab":
			pack.VocabPaths = append(pack.VocabPaths, path)
		default:
			return nil, fmt.Errorf("unknown model file role %q", file.Role)
		}
	}
	if pack.ModelPath == "" || pack.ShortlistPath == "" || len(pack.VocabPaths) == 0 {
		return nil, errors.New("model pack is incomplete")
	}
	return pack, nil
}

// ModelConfig renders the Marian YAML configuration Bergamot expects.
func (p *Pack) ModelConfig() (string, error) {
	if p == nil {
		return "", ErrNoModel
	}
	vocabs := append([]string(nil), p.VocabPaths...)
	// Firefox packs that ship a single SentencePiece vocabulary pass it twice.
	if len(vocabs) == 1 {
		vocabs = append(vocabs, vocabs[0])
	}
	lines := []string{
		"models: " + yamlList([]string{p.ModelPath}),
		"vocabs: " + yamlList(vocabs),
		"shortlist: " + yamlList([]string{p.ShortlistPath}),
		"beam-size: 1",
		"normalize: 1.0",
		"word-penalty: 0",
		"max-length: 0",
		"max-length-break: 100",
		"mini-batch-words: 1024",
		"maxi-batch: 1",
		"maxi-batch-sort: none",
	}
	if p.GEMMPrecision != "" {
		lines = append(lines, "gemm-precision: "+p.GEMMPrecision)
	}
	return strings.Join(lines, "\n"), nil
}

func yamlList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, yamlString(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func yamlString(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func safeName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		filepath.Base(name) == name && !strings.ContainsAny(name, `/\`)
}
