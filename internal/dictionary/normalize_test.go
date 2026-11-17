package dictionary

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"whitespace", "  hello \n\t world  ", "hello world"},
		{"sentence punctuation", "word,;:!?", "word"},
		{"parentheses and period", `("word").`, "word"},
		{"nested wrappers", `(“hello”)`, "hello"},
		{"quotes", `"quoted"`, "quoted"},
		{"guillemets", "«bonjour»", "bonjour"},
		{"trailing dots", "word...", "word"},
		{"abbreviation keeps dots", "U.S.", "U.S."},
		{"leading apostrophe kept", "'tis", "'tis"},
		{"only punctuation", "...", ""},
		{"empty", "   ", ""},
		{"unicode whitespace", "word\u00a0\u2003next", "word next"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Normalize(test.in); got != test.want {
				t.Fatalf("Normalize(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

func TestPrepareLimits(t *testing.T) {
	if _, err := Prepare("one two three four", 4, 128); err != nil {
		t.Fatalf("four words should be allowed: %v", err)
	}
	if _, err := Prepare("one two three four five", 4, 128); err == nil {
		t.Fatal("five words should be rejected")
	}
	if _, err := Prepare(strings.Repeat("é", 128), 4, 128); err != nil {
		t.Fatalf("128 characters should be allowed: %v", err)
	}
	if _, err := Prepare(strings.Repeat("é", 129), 4, 128); err == nil {
		t.Fatal("129 characters should be rejected")
	}
	if _, err := Prepare("   ", 4, 128); err == nil {
		t.Fatal("empty text should be rejected")
	}
}

func TestPrepareReportsQueryError(t *testing.T) {
	_, err := Prepare("a b c d e", 4, 128)
	if !IsQueryError(err) {
		t.Fatalf("error %v is not a QueryError", err)
	}
	if !strings.Contains(err.Error(), "4 words") {
		t.Fatalf("error = %q, want a word-count explanation", err)
	}
}
