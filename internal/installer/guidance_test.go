package installer

import (
	"strings"
	"testing"
)

func TestShortcutsIncludeLexLookup(t *testing.T) {
	shortcuts := Shortcuts()
	wantNames := []string{"GNOME", "KDE Plasma", "Sway", "Hyprland", "Other"}
	if len(shortcuts) != len(wantNames) {
		t.Fatalf("shortcuts = %d, want %d", len(shortcuts), len(wantNames))
	}
	for index, want := range wantNames {
		if shortcuts[index].Name != want {
			t.Fatalf("shortcut %d = %q, want %q", index, shortcuts[index].Name, want)
		}
		if !strings.Contains(shortcuts[index].Hint, "lex lookup") {
			t.Fatalf("%s hint does not contain lex lookup: %q", want, shortcuts[index].Hint)
		}
	}
}
