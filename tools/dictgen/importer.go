package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // registers the pure-Go SQLite driver
)

//go:embed schema.sql
var schemaSQL string

const (
	dictionaryName    = "WordNet English Dictionary"
	sourceLanguage    = "en"
	targetLanguage    = "en"
	dictionaryLicense = "WordNet 3.0"
	attribution       = "WordNet 3.0 Copyright 2006 by Princeton University. All rights reserved. " +
		"Pronunciation data from the CMU Pronouncing Dictionary."
	maxSynonyms = 8
)

// Options configures a dictionary build.
type Options struct {
	WordNetDir  string
	CMUDictFile string
	OutputPath  string
	Progress    io.Writer
}

// Stats reports what a build produced.
type Stats struct {
	Entries        int
	Definitions    int
	Forms          int
	AmbiguousForms int
	ExceptionForms int
}

type sense struct {
	partOfSpeech string
	definition   string
}

type entry struct {
	word     string
	ipa      string
	senses   []sense
	parts    map[string]struct{}
	synonyms []string
}

type entryIndex struct {
	byKey map[string]*entry
	order []string
}

func newEntryIndex() *entryIndex {
	return &entryIndex{byKey: map[string]*entry{}}
}

func (index *entryIndex) getOrCreate(word string, cmudict map[string]string) *entry {
	key := strings.ToLower(word)
	if existing, ok := index.byKey[key]; ok {
		return existing
	}
	created := &entry{
		word:  word,
		ipa:   cmudict[key],
		parts: map[string]struct{}{},
	}
	index.byKey[key] = created
	index.order = append(index.order, key)
	return created
}

type formIndex struct {
	candidates map[string]map[string]string
	order      []string
}

func newFormIndex() *formIndex {
	return &formIndex{candidates: map[string]map[string]string{}}
}

func (index *formIndex) add(form, key, kind string) {
	canonical := strings.ToLower(form)
	if canonical == "" || canonical == key {
		return
	}
	bases, ok := index.candidates[canonical]
	if !ok {
		bases = map[string]string{}
		index.candidates[canonical] = bases
		index.order = append(index.order, canonical)
	}
	bases[key] = kind
}

type entryRow struct {
	id   int64
	word string
	ipa  string
}

type definitionRow struct {
	id           int64
	entryID      int64
	partOfSpeech string
	definition   string
	senseOrder   int
}

type metadataRow struct {
	entryID int64
	key     string
	value   string
}

type formRow struct {
	form    string
	entryID int64
	kind    string
}

// Build reads the source datasets and writes a complete dictionary database.
func Build(ctx context.Context, options Options) (Stats, error) {
	progress := options.Progress
	if progress == nil {
		progress = io.Discard
	}

	if strings.TrimSpace(options.WordNetDir) == "" {
		return Stats{}, errors.New("--wordnet-dir is required; extract WordNet-3.0 and point at its dict/ directory")
	}
	if strings.TrimSpace(options.OutputPath) == "" {
		return Stats{}, errors.New("--output is required")
	}

	files, err := findPOSFiles(options.WordNetDir)
	if err != nil {
		return Stats{}, err
	}

	cmudict := map[string]string{}
	if options.CMUDictFile != "" {
		fmt.Fprintf(progress, "Loading CMUdict from %s...\n", options.CMUDictFile)
		cmudict, err = parseCMUDict(options.CMUDictFile)
		if err != nil {
			return Stats{}, fmt.Errorf("read CMUdict: %w", err)
		}
		fmt.Fprintf(progress, "  %d pronunciations converted to IPA\n", len(cmudict))
	} else {
		fmt.Fprintln(progress, "warning: no CMUdict file given, writing entries without pronunciation")
	}

	entries := newEntryIndex()
	for _, file := range files {
		synsets := 0
		err := parseSynsets(file.path, func(lemmas []string, definition string) error {
			if definition == "" {
				return nil
			}
			for _, lemma := range lemmas {
				current := entries.getOrCreate(lemma, cmudict)
				current.senses = append(current.senses, sense{
					partOfSpeech: file.partOfSpeech,
					definition:   definition,
				})
				current.parts[file.partOfSpeech] = struct{}{}
				for _, sibling := range lemmas {
					if strings.EqualFold(sibling, lemma) || len(current.synonyms) >= maxSynonyms {
						continue
					}
					if !contains(current.synonyms, sibling) {
						current.synonyms = append(current.synonyms, sibling)
					}
				}
			}
			synsets++
			return nil
		})
		if err != nil {
			return Stats{}, fmt.Errorf("read %s: %w", file.path, err)
		}
		fmt.Fprintf(progress, "  %s: %d synsets\n", filepath.Base(file.path), synsets)
	}

	forms := newFormIndex()
	stats := Stats{}
	for _, exception := range exceptionFiles {
		path := filepath.Join(options.WordNetDir, exception.name)
		info, statErr := os.Stat(path)
		switch {
		case statErr == nil && !info.Mode().IsRegular():
			return Stats{}, fmt.Errorf("%s is not a regular file", path)
		case errors.Is(statErr, os.ErrNotExist):
			continue
		case statErr != nil:
			return Stats{}, fmt.Errorf("inspect %s: %w", path, statErr)
		}
		err := parseExceptionFile(path, func(inflected, base string) error {
			key := strings.ToLower(base)
			if _, exists := entries.byKey[key]; !exists {
				return nil
			}
			forms.add(inflected, key, "irregular")
			stats.ExceptionForms++
			return nil
		})
		if err != nil {
			return Stats{}, fmt.Errorf("read %s: %w", path, err)
		}
	}

	for _, key := range entries.order {
		current := entries.byKey[key]
		for _, generated := range generatedForms(current.word, current.parts) {
			forms.add(generated.form, key, generated.kind)
		}
	}

	fmt.Fprintf(progress, "  %d irregular forms from WordNet exception files\n", stats.ExceptionForms)
	fmt.Fprintf(progress, "  %d distinct inflected forms before ambiguity filtering\n", len(forms.order))

	entryRows, definitionRows, metadataRows, entryIDs := collectEntryRows(entries)
	formRows, ambiguous := collectFormRows(forms, entryIDs)

	stats.Entries = len(entryRows)
	stats.Definitions = len(definitionRows)
	stats.Forms = len(formRows)
	stats.AmbiguousForms = ambiguous

	fmt.Fprintf(progress, "Writing %d headwords to %s...\n", stats.Entries, options.OutputPath)
	if err := writeDatabase(ctx, options.OutputPath, entryRows, definitionRows, metadataRows, formRows); err != nil {
		return Stats{}, err
	}

	fmt.Fprintf(progress, "Wrote %d entries and %d definitions to %s\n", stats.Entries, stats.Definitions, options.OutputPath)
	fmt.Fprintf(progress, "Wrote %d inflected forms (%d ambiguous forms skipped)\n", stats.Forms, stats.AmbiguousForms)
	return stats, nil
}

func collectEntryRows(entries *entryIndex) ([]entryRow, []definitionRow, []metadataRow, map[string]int64) {
	entryRows := make([]entryRow, 0, len(entries.order))
	definitionRows := make([]definitionRow, 0, len(entries.order)*2)
	metadataRows := make([]metadataRow, 0, len(entries.order))
	entryIDs := make(map[string]int64, len(entries.order))

	var entryID, definitionID int64
	for _, key := range entries.order {
		current := entries.byKey[key]

		seen := map[sense]struct{}{}
		senses := make([]sense, 0, len(current.senses))
		for _, candidate := range current.senses {
			if _, duplicate := seen[candidate]; duplicate {
				continue
			}
			seen[candidate] = struct{}{}
			senses = append(senses, candidate)
		}
		if len(senses) == 0 {
			continue
		}

		entryID++
		entryIDs[key] = entryID
		entryRows = append(entryRows, entryRow{id: entryID, word: current.word, ipa: current.ipa})
		for senseOrder, candidate := range senses {
			definitionID++
			definitionRows = append(definitionRows, definitionRow{
				id:           definitionID,
				entryID:      entryID,
				partOfSpeech: candidate.partOfSpeech,
				definition:   candidate.definition,
				senseOrder:   senseOrder,
			})
		}
		if len(current.synonyms) > 0 {
			metadataRows = append(metadataRows, metadataRow{
				entryID: entryID,
				key:     "synonyms",
				value:   strings.Join(current.synonyms, ", "),
			})
		}
	}
	return entryRows, definitionRows, metadataRows, entryIDs
}

func collectFormRows(forms *formIndex, entryIDs map[string]int64) ([]formRow, int) {
	formRows := make([]formRow, 0, len(forms.order))
	ambiguous := 0
	for _, form := range forms.order {
		bases := forms.candidates[form]
		if len(bases) != 1 {
			ambiguous++
			continue
		}
		var baseKey, kind string
		for key, candidateKind := range bases {
			baseKey, kind = key, candidateKind
		}
		entryID, exists := entryIDs[baseKey]
		if !exists {
			continue
		}
		formRows = append(formRows, formRow{form: form, entryID: entryID, kind: kind})
	}
	return formRows, ambiguous
}

func writeDatabase(ctx context.Context, path string, entryRows []entryRow, definitionRows []definitionRow, metadataRows []metadataRow, formRows []formRow) error {
	//nolint:gosec // the output is a build artifact meant to be readable
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove existing output: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open output database: %w", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode = OFF",
		"PRAGMA synchronous = OFF",
		"PRAGMA foreign_keys = ON",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("%s: %w", pragma, err)
		}
	}
	for _, statement := range splitSQL(schemaSQL) {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO dictionary_info (id, name, source_language, target_language, license, attribution)
		 VALUES (1, ?, ?, ?, ?, ?)`,
		dictionaryName, sourceLanguage, targetLanguage, dictionaryLicense, attribution); err != nil {
		return fmt.Errorf("insert dictionary info: %w", err)
	}

	entryStatement, err := tx.PrepareContext(ctx,
		`INSERT INTO entries (id, word, transliteration, pronunciation_respelling, pronunciation_ipa, source, license)
		 VALUES (?, ?, '', '', ?, 'WordNet 3.0', 'WordNet 3.0')`)
	if err != nil {
		return fmt.Errorf("prepare entries: %w", err)
	}
	defer func() { _ = entryStatement.Close() }()
	for _, row := range entryRows {
		if _, err := entryStatement.ExecContext(ctx, row.id, row.word, row.ipa); err != nil {
			return fmt.Errorf("insert entry %d: %w", row.id, err)
		}
	}

	definitionStatement, err := tx.PrepareContext(ctx,
		`INSERT INTO definitions (id, entry_id, part_of_speech, definition, label, sense_order)
		 VALUES (?, ?, ?, ?, '', ?)`)
	if err != nil {
		return fmt.Errorf("prepare definitions: %w", err)
	}
	defer func() { _ = definitionStatement.Close() }()
	for _, row := range definitionRows {
		if _, err := definitionStatement.ExecContext(ctx, row.id, row.entryID, row.partOfSpeech, row.definition, row.senseOrder); err != nil {
			return fmt.Errorf("insert definition %d: %w", row.id, err)
		}
	}

	metadataStatement, err := tx.PrepareContext(ctx,
		`INSERT INTO entry_metadata (entry_id, key, value) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare metadata: %w", err)
	}
	defer func() { _ = metadataStatement.Close() }()
	for _, row := range metadataRows {
		if _, err := metadataStatement.ExecContext(ctx, row.entryID, row.key, row.value); err != nil {
			return fmt.Errorf("insert metadata for entry %d: %w", row.entryID, err)
		}
	}

	formStatement, err := tx.PrepareContext(ctx,
		`INSERT INTO word_forms (form, entry_id, kind) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare forms: %w", err)
	}
	defer func() { _ = formStatement.Close() }()
	for _, row := range formRows {
		if _, err := formStatement.ExecContext(ctx, row.form, row.entryID, row.kind); err != nil {
			return fmt.Errorf("insert form %q: %w", row.form, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	for _, statement := range []string{"ANALYZE", "VACUUM"} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("%s: %w", statement, err)
		}
	}
	return nil
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// splitSQL applies the simple statement-per-semicolon schema script. It handles
// line comments and single-quoted strings; the embedded schema uses no other
// quoting.
func splitSQL(script string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	for index := 0; index < len(script); index++ {
		character := script[index]
		if !inString && character == '-' && index+1 < len(script) && script[index+1] == '-' {
			for index < len(script) && script[index] != '\n' {
				current.WriteByte(script[index])
				index++
			}
			if index < len(script) {
				current.WriteByte('\n')
			}
			continue
		}
		if character == '\'' {
			inString = !inString
		}
		if character == ';' && !inString {
			if statement := strings.TrimSpace(current.String()); statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
			continue
		}
		current.WriteByte(character)
	}
	if statement := strings.TrimSpace(current.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}
