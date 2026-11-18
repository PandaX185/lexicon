package installer

// Shortcut is compositor-specific guidance. The installer prints it; it never
// registers a keybinding.
type Shortcut struct {
	Name string
	Hint string
}

// Shortcuts returns the ordered shortcut menu shown on the Done screen.
func Shortcuts() []Shortcut {
	return []Shortcut{
		{
			Name: "GNOME",
			Hint: "Settings -> Keyboard -> Keyboard Shortcuts -> Custom Shortcuts -> +. " +
				"Set Command to `lex lookup`, then assign a key.",
		},
		{
			Name: "KDE Plasma",
			Hint: "System Settings -> Shortcuts -> Custom Shortcuts -> Edit -> New -> " +
				"Global Shortcut -> Command/URL. Set Command to `lex lookup`.",
		},
		{
			Name: "Sway",
			Hint: "Add `bindsym $mod+Shift+l exec lex lookup` to ~/.config/sway/config, then reload Sway.",
		},
		{
			Name: "Hyprland",
			Hint: "Add `bind = $mainMod, L, exec, lex lookup` to ~/.config/hypr/hyprland.conf, then reload Hyprland.",
		},
		{
			Name: "Other",
			Hint: "Bind `lex lookup` in your compositor's global shortcut settings.",
		},
	}
}
