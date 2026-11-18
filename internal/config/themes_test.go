package config

import (
	"strings"
	"testing"
)

func TestPopupThemesCoverage(t *testing.T) {
	if got := PopupThemes[0]; got != "system" {
		t.Fatalf("first theme = %q, want system", got)
	}

	seen := make(map[string]bool, len(PopupThemes))
	var dark, light int
	for _, theme := range PopupThemes {
		if seen[theme] {
			t.Errorf("duplicate theme %q", theme)
		}
		seen[theme] = true
		if strings.Contains(theme, "-light") || strings.HasSuffix(theme, "-latte") || strings.HasSuffix(theme, "-dawn") || theme == "light" {
			light++
		} else if theme != "system" {
			dark++
		}
	}
	if dark != 10 || light != 10 {
		t.Errorf("theme counts = %d dark, %d light; want 10 each", dark, light)
	}
}
