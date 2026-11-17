// Package dictionary reads Lexicon's SQLite dictionary format and normalizes
// lookup text.
package dictionary

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits mirror the C++ implementation: the selection policy reads at most 128
// characters, and a lookup is a single word or a short phrase.
const (
	DefaultMaxWords = 4
	DefaultMaxChars = 128
)

// QueryError describes a rejected lookup before any database access.
type QueryError struct {
	Message string
	Detail  string
}

func (e *QueryError) Error() string {
	if e.Detail == "" {
		return e.Message
	}
	return e.Message + ": " + e.Detail
}

var errEmptyQuery = &QueryError{Message: "Enter a word to look up."}

// Normalize collapses whitespace and strips wrapping punctuation from a raw
// selection or CLI argument. The algorithm mirrors core/text/LookupText.cpp so
// the rewrite does not change which words resolve.
func Normalize(raw string) string {
	text := simplify(raw)

	// Wrapping and punctuation can nest, for example `("word").`; repeat a
	// bounded number of passes instead of enumerating combinations.
	for range 4 {
		changed := false

		for len(text) > 0 {
			last, _ := utf8.DecodeLastRuneInString(text)
			if !strings.ContainsRune(",;:!?", last) {
				break
			}
			text = trimLastRune(text)
			changed = true
		}

		for stripped := true; stripped && utf8.RuneCountInString(text) >= 2; {
			stripped = false
			runes := []rune(text)
			for _, wrapper := range wrappers {
				if runes[0] == wrapper.opening && runes[len(runes)-1] == wrapper.closing {
					text = simplify(string(runes[1 : len(runes)-1]))
					changed = true
					stripped = true
					break
				}
			}
		}

		for len(text) > 0 {
			last, _ := utf8.DecodeLastRuneInString(text)
			if !strings.ContainsRune("\"'`”’»)]}", last) {
				break
			}
			text = trimLastRune(text)
			changed = true
		}

		// A leading apostrophe is kept unless it wraps the whole token because
		// it is part of contractions such as 'tis.
		for len(text) > 0 {
			first, size := utf8.DecodeRuneInString(text)
			if !strings.ContainsRune("\"'`“‘«([{", first) {
				break
			}
			if first == '\'' && utf8.RuneCountInString(text) > 1 {
				break
			}
			text = text[size:]
			changed = true
		}

		before := len(text)
		text = stripTrailingDots(text)
		changed = changed || len(text) != before

		text = simplify(text)
		if !changed {
			break
		}
	}

	return text
}

// Prepare normalizes raw and enforces the configured limits. It returns a
// *QueryError for invalid input.
func Prepare(raw string, maxWords, maxChars int) (string, error) {
	if maxWords <= 0 {
		maxWords = DefaultMaxWords
	}
	if maxChars <= 0 {
		maxChars = DefaultMaxChars
	}
	text := Normalize(raw)
	if text == "" {
		return "", errEmptyQuery
	}
	words := len(strings.Fields(text))
	if words > maxWords {
		return "", &QueryError{
			Message: fmt.Sprintf("Select a single word or a short phrase of at most %d words.", maxWords),
			Detail:  fmt.Sprintf("The selection contains %d words.", words),
		}
	}
	if count := utf8.RuneCountInString(text); count > maxChars {
		return "", &QueryError{
			Message: fmt.Sprintf("The selection is too long to look up (maximum %d characters).", maxChars),
			Detail:  fmt.Sprintf("Selected %d characters.", count),
		}
	}
	return text, nil
}

// IsQueryError reports whether err is a rejected lookup.
func IsQueryError(err error) bool {
	var queryError *QueryError
	return errors.As(err, &queryError)
}

type wrapper struct {
	opening rune
	closing rune
}

var wrappers = []wrapper{
	{'"', '"'},
	{'\'', '\''},
	{'“', '”'},
	{'‘', '’'},
	{'(', ')'},
	{'[', ']'},
	{'{', '}'},
	{'«', '»'},
}

func simplify(text string) string {
	return strings.Join(strings.FieldsFunc(text, unicode.IsSpace), " ")
}

func trimLastRune(text string) string {
	_, size := utf8.DecodeLastRuneInString(text)
	return text[:len(text)-size]
}

// stripTrailingDots removes a trailing run of dots only when it is sentence
// punctuation. One dot in a token with no other dots is a full stop
// ("word."); dots inside the token belong to an abbreviation ("U.S.").
func stripTrailingDots(text string) string {
	if !strings.HasSuffix(text, ".") {
		return text
	}
	lastSpace := strings.LastIndexByte(text, ' ')
	lastToken := text[lastSpace+1:]
	withoutDots := strings.TrimRight(lastToken, ".")
	if withoutDots == "" || !strings.Contains(withoutDots, ".") {
		return text[:len(text)-(len(lastToken)-len(withoutDots))]
	}
	return text
}
