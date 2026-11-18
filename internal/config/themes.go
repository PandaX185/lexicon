package config

import "strings"

// PopupThemes is the canonical list of built-in popup colour presets.
// "system" follows the desktop light/dark preference; the remaining entries
// are compiled-in palettes, split evenly between dark and light variants.
// Keep this list in sync with popup/frontend/src/style.css and
// popup/themeBaseColour; the popup tests enforce both.
var PopupThemes = []string{
	"system",

	// Dark presets.
	"dark",
	"nord",
	"catppuccin",
	"gruvbox",
	"rose-pine",
	"dracula",
	"tokyo-night",
	"one-dark",
	"solarized-dark",
	"github-dark",

	// Light presets.
	"light",
	"nord-light",
	"catppuccin-latte",
	"gruvbox-light",
	"rose-pine-dawn",
	"solarized-light",
	"github-light",
	"one-light",
	"tokyo-night-light",
	"everforest-light",
}

// ValidPopupTheme reports whether value names a built-in preset, ignoring
// case and surrounding whitespace.
func ValidPopupTheme(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, theme := range PopupThemes {
		if normalized == theme {
			return true
		}
	}
	return false
}
