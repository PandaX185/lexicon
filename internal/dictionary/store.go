package dictionary

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite" // registers the pure-Go SQLite driver
)

// ErrNotFound is returned when no headword, transliteration, or inflected form
// matches.
var ErrNotFound = errors.New("word not found")

// optionalEntryColumns are selected when the database provides them. Older
// dictionaries may only have id and word.
var optionalEntryColumns = []string{
	"transliteration",
	"pronunciation_respelling",
	"pronunciation_ipa",
	"source",
	"license",
}

// Store is a read-only view of a dictionary database.
type Store struct {
	db     *sql.DB
	path   string
	schema dictionarySchema
}

type dictionarySchema struct {
	entryColumns      map[string]bool
	hasWordForms      bool
	hasDefinitions    bool
	hasEntryMetadata  bool
	hasDictionaryInfo bool
}

// Open validates that path is a readable file and opens it read-only. The
// schema is probed once so databases written by older versions can degrade
// gracefully instead of failing on every query.
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("no dictionary database path is configured")
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("dictionary database not found: %s", path)
		}
		return nil, fmt.Errorf("inspect dictionary database: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("dictionary path is not a regular file: %s", path)
	}

	dsn := "file:" + path + "?mode=ro&_pragma=busy_timeout(2000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open dictionary database: %w", err)
	}
	// One connection keeps reads serialized and avoids surprising locking
	// behavior against a file another process may be updating.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("read dictionary database: %w", err)
	}

	schema, err := probeSchema(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("inspect dictionary schema: %w", err)
	}
	return &Store{db: db, path: path, schema: schema}, nil
}

func probeSchema(ctx context.Context, db *sql.DB) (dictionarySchema, error) {
	schema := dictionarySchema{entryColumns: map[string]bool{}}

	columns, err := db.QueryContext(ctx, `PRAGMA table_info(entries)`)
	if err != nil {
		return schema, err
	}
	defer func() { _ = columns.Close() }()
	for columns.Next() {
		var (
			cid       int
			name      string
			columnTyp string
			notNull   int
			defaultV  sql.NullString
			primary   int
		)
		if err := columns.Scan(&cid, &name, &columnTyp, &notNull, &defaultV, &primary); err != nil {
			return schema, err
		}
		schema.entryColumns[name] = true
	}
	if err := columns.Err(); err != nil {
		return schema, err
	}

	tables, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		return schema, err
	}
	defer func() { _ = tables.Close() }()
	for tables.Next() {
		var name string
		if err := tables.Scan(&name); err != nil {
			return schema, err
		}
		switch name {
		case "word_forms":
			schema.hasWordForms = true
		case "definitions":
			schema.hasDefinitions = true
		case "entry_metadata":
			schema.hasEntryMetadata = true
		case "dictionary_info":
			schema.hasDictionaryInfo = true
		}
	}
	if err := tables.Err(); err != nil {
		return schema, err
	}
	return schema, nil
}

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Close releases the database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// entryColumns returns the selectable entry columns in a stable order,
// optionally qualified with a table alias.
func (s *Store) entryColumns(alias string) []string {
	columns := make([]string, 0, len(optionalEntryColumns)+2)
	columns = append(columns, "id", "word")
	for _, column := range optionalEntryColumns {
		if s.schema.entryColumns[column] {
			columns = append(columns, column)
		}
	}
	if alias != "" {
		for i, column := range columns {
			columns[i] = alias + "." + column
		}
	}
	return columns
}

// Lookup resolves word in this order: a curated preferred-form override, an
// exact headword, Latin transliteration, an inflected surface form, then
// separator variants such as "ice-cream" and "black board".
func (s *Store) Lookup(ctx context.Context, word string) (*Entry, error) {
	if strings.TrimSpace(word) == "" {
		return nil, ErrNotFound
	}

	if entry, err := s.lookupPreferred(ctx, word); err == nil {
		return entry, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	if entry, err := s.lookupDirect(ctx, word); err == nil {
		return entry, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	for _, variant := range separatorVariants(word) {
		entry, err := s.lookupDirect(ctx, variant)
		if err == nil {
			return entry, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return nil, ErrNotFound
}

func (s *Store) lookupPreferred(ctx context.Context, word string) (*Entry, error) {
	if !s.schema.hasEntryMetadata {
		return nil, ErrNotFound
	}
	columns := s.entryColumns("e")
	query := `SELECT ` + strings.Join(columns, ", ") +
		` FROM entry_metadata m JOIN entries e ON e.id = m.entry_id` +
		` WHERE m.key = ? COLLATE NOCASE LIMIT 1`
	return s.readEntry(ctx, columns, query, []any{preferredFormKey(word)}, "")
}

func (s *Store) lookupDirect(ctx context.Context, word string) (*Entry, error) {
	columns := s.entryColumns("")
	query := `SELECT ` + strings.Join(columns, ", ") + ` FROM entries WHERE word = ? COLLATE NOCASE LIMIT 1`
	entry, err := s.readEntry(ctx, columns, query, []any{word}, "")
	if err == nil {
		return entry, nil
	}
	if !errors.Is(err, ErrNotFound) {
		if missingSchema(err) {
			return nil, fmt.Errorf("dictionary database has no usable entries table: %w", err)
		}
		return nil, err
	}

	if s.schema.entryColumns["transliteration"] {
		query = `SELECT ` + strings.Join(columns, ", ") +
			` FROM entries WHERE transliteration = ? COLLATE NOCASE LIMIT 1`
		entry, err = s.readEntry(ctx, columns, query, []any{word}, "")
		if err == nil {
			return entry, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	if s.schema.hasWordForms {
		columns = s.entryColumns("e")
		query = `SELECT ` + strings.Join(columns, ", ") +
			` FROM word_forms f JOIN entries e ON e.id = f.entry_id` +
			` WHERE f.form = ? COLLATE NOCASE ORDER BY e.id LIMIT 1`
		entry, err = s.readEntry(ctx, columns, query, []any{word}, word)
		if err == nil {
			return entry, nil
		}
		if errors.Is(err, ErrNotFound) || missingSchema(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return nil, ErrNotFound
}

func preferredFormKey(word string) string {
	return "preferred_form:" + strings.ToLower(word)
}

func separatorVariants(word string) []string {
	if !strings.ContainsAny(word, "- ") {
		return nil
	}
	candidates := []string{
		strings.ReplaceAll(word, "-", " "),
		strings.ReplaceAll(word, " ", "-"),
		strings.NewReplacer("-", "", " ", "").Replace(word),
	}
	variants := make([]string, 0, len(candidates))
	seen := map[string]struct{}{strings.ToLower(word): {}}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		variants = append(variants, candidate)
	}
	return variants
}

func (s *Store) readEntry(ctx context.Context, columns []string, query string, args []any, matchedForm string) (*Entry, error) {
	var id int64
	values := make([]any, len(columns))
	texts := make([]sql.NullString, len(columns))
	for i, column := range columns {
		if baseColumn(column) == "id" {
			values[i] = &id
			continue
		}
		values[i] = &texts[i]
	}

	row := s.db.QueryRowContext(ctx, query, args...)
	if err := row.Scan(values...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query dictionary entry: %w", err)
	}

	entry := &Entry{
		MatchedForm: matchedForm,
		Definitions: []Definition{},
		Metadata:    map[string]string{},
	}
	for i, column := range columns {
		name := baseColumn(column)
		value := texts[i].String
		switch name {
		case "word":
			entry.Word = value
		case "transliteration":
			entry.Transliteration = value
		case "pronunciation_respelling":
			entry.Pronunciation.Respelling = value
		case "pronunciation_ipa":
			entry.Pronunciation.IPA = value
		case "source":
			if value != "" {
				entry.Metadata["source"] = value
			}
		case "license":
			if value != "" {
				entry.Metadata["license"] = value
			}
		}
	}

	if !s.schema.hasDefinitions {
		return entry, nil
	}
	definitions, err := s.db.QueryContext(ctx,
		`SELECT part_of_speech, definition, label FROM definitions WHERE entry_id = ? ORDER BY sense_order, id`, id)
	if err != nil {
		if missingSchema(err) {
			return entry, nil
		}
		return nil, fmt.Errorf("query dictionary definitions: %w", err)
	}
	defer func() { _ = definitions.Close() }()
	for definitions.Next() {
		var definition Definition
		if err := definitions.Scan(&definition.PartOfSpeech, &definition.Text, &definition.Label); err != nil {
			return nil, fmt.Errorf("read dictionary definition: %w", err)
		}
		definition.PartOfSpeech = normalizePartOfSpeech(definition.PartOfSpeech)
		entry.Definitions = append(entry.Definitions, definition)
	}
	if err := definitions.Err(); err != nil {
		return nil, fmt.Errorf("read dictionary definitions: %w", err)
	}

	if !s.schema.hasEntryMetadata {
		return entry, nil
	}
	// Metadata is optional: a failure here must not discard the entry.
	metadata, err := s.db.QueryContext(ctx, `SELECT key, value FROM entry_metadata WHERE entry_id = ?`, id)
	if err == nil {
		defer func() { _ = metadata.Close() }()
		for metadata.Next() {
			var key, value string
			if err := metadata.Scan(&key, &value); err != nil {
				break
			}
			entry.Metadata[key] = value
		}
	}
	return entry, nil
}

// Describe reads the single dictionary_info row.
func (s *Store) Describe(ctx context.Context) (Info, error) {
	if !s.schema.hasDictionaryInfo {
		return Info{}, errors.New("dictionary is missing dictionary_info metadata")
	}
	var info Info
	err := s.db.QueryRowContext(ctx,
		`SELECT name, source_language, target_language, license, attribution FROM dictionary_info WHERE id = 1`).
		Scan(&info.Name, &info.SourceLanguage, &info.TargetLanguage, &info.License, &info.Attribution)
	if err != nil {
		if missingSchema(err) {
			return Info{}, fmt.Errorf("dictionary is missing dictionary_info metadata: %w", err)
		}
		return Info{}, fmt.Errorf("read dictionary metadata: %w", err)
	}
	return info, nil
}

// ValidateLookupSchema checks the minimum shape needed for a lookup: an entries
// table with id and word. Optional tables and metadata are not required here.
func (s *Store) ValidateLookupSchema() error {
	if !s.schema.entryColumns["id"] || !s.schema.entryColumns["word"] {
		return errors.New("dictionary is missing an entries table with id and word columns")
	}
	return nil
}

// baseColumn strips a table alias from a qualified column name.
func baseColumn(column string) string {
	if dot := strings.LastIndexByte(column, '.'); dot >= 0 {
		return column[dot+1:]
	}
	return column
}

func missingSchema(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table") || strings.Contains(message, "no such column")
}

var knownPartsOfSpeech = map[string]struct{}{
	"noun": {}, "verb": {}, "adjective": {}, "adverb": {}, "pronoun": {},
	"preposition": {}, "conjunction": {}, "determiner": {}, "interjection": {},
}

func normalizePartOfSpeech(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if _, ok := knownPartsOfSpeech[normalized]; ok {
		return normalized
	}
	return "other"
}
