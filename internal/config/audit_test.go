package config

import (
	"reflect"
	"strconv"
	"testing"
)

// TestKeysAreTheDocumentedSurface pins the complete configuration surface.
// Adding or renaming a key must be a deliberate update here and in docs.
func TestKeysAreTheDocumentedSurface(t *testing.T) {
	want := []string{
		"general.max_query_words",
		"general.max_query_chars",
		"daemon.socket",
		"daemon.log_level",
		"selection.provider",
		"selection.timeout_ms",
		"selection.max_chars",
		"dictionary.database_path",
		"dictionary.source_language",
		"dictionary.target_language",
		"dictionary.show_pronunciation",
		"translation.enabled",
		"translation.model_directory",
		"translation.source_language",
		"translation.target_language",
		"translation.timeout_seconds",
		"popup.auto_launch",
		"popup.binary",
		"popup.width",
		"popup.height",
		"popup.theme",
		"popup.show_header",
		"popup.show_status",
		"popup.show_footer",
	}
	if got := Keys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
}

func TestEveryKeyRoundTrips(t *testing.T) {
	cfg := Default()
	for _, key := range Keys() {
		value, err := Get(cfg, key)
		if err != nil {
			t.Fatalf("Get(%q): %v", key, err)
		}
		updated := Default()
		if err := Set(&updated, key, value); err != nil {
			t.Fatalf("Set(%q, %q): %v", key, value, err)
		}
		got, err := Get(updated, key)
		if err != nil || got != value {
			t.Fatalf("round trip %q = %q, %v; want %q", key, got, err, value)
		}
	}
}

func TestNumericBoundaries(t *testing.T) {
	tests := []struct {
		key     string
		minimum string
		maximum string
	}{
		{"general.max_query_words", "1", "16"},
		{"general.max_query_chars", "1", "512"},
		{"selection.timeout_ms", "50", "5000"},
		{"selection.max_chars", "1", "512"},
		{"translation.timeout_seconds", "5", "300"},
		{"popup.width", "240", "1200"},
		{"popup.height", "160", "1200"},
	}
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			low, err := strconv.Atoi(test.minimum)
			if err != nil {
				t.Fatal(err)
			}
			high, err := strconv.Atoi(test.maximum)
			if err != nil {
				t.Fatal(err)
			}
			for _, value := range []string{test.minimum, test.maximum, strconv.Itoa(low - 1), strconv.Itoa(high + 1)} {
				cfg := Default()
				err := Set(&cfg, test.key, value)
				wantValid := value == test.minimum || value == test.maximum
				if wantValid && err != nil {
					t.Errorf("Set(%q, %q) rejected a boundary: %v", test.key, value, err)
				}
				if !wantValid && err == nil {
					t.Errorf("Set(%q, %q) accepted an out-of-range value", test.key, value)
				}
			}
		})
	}
}

func TestEnumAndLanguageValidation(t *testing.T) {
	enumTests := map[string][]string{
		"daemon.log_level":   {"debug", "info", "warn", "error"},
		"selection.provider": {"auto", "wl-paste", "none"},
		"popup.theme":        PopupThemes,
	}
	for key, valid := range enumTests {
		for _, value := range valid {
			cfg := Default()
			if err := Set(&cfg, key, value); err != nil {
				t.Errorf("Set(%q, %q) rejected a valid value: %v", key, value, err)
			}
		}
		cfg := Default()
		if err := Set(&cfg, key, "not-a-value"); err == nil {
			t.Errorf("Set(%q) accepted an invalid value", key)
		}
	}

	languageKeys := []string{
		"dictionary.source_language",
		"dictionary.target_language",
		"translation.source_language",
		"translation.target_language",
	}
	for _, key := range languageKeys {
		for _, value := range []string{"en", "de", "zh-hant"} {
			cfg := Default()
			if err := Set(&cfg, key, value); err != nil {
				t.Errorf("Set(%q, %q) rejected a valid tag: %v", key, value, err)
			}
		}
		for _, value := range []string{"", "EN", "english", "en_US", "../en"} {
			cfg := Default()
			if err := Set(&cfg, key, value); err == nil {
				t.Errorf("Set(%q, %q) accepted an invalid tag", key, value)
			}
		}
	}
}
