package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PandaX185/lexicon/internal/installer"
)

func (m model) viewDone() string {
	if m.options.DryRun {
		return "Dry run complete\n\nNo changes were made.\n"
	}
	var b strings.Builder
	b.WriteString(okStyle.Render("Setup complete") + "\n\n")

	b.WriteString("CLI overview\n")
	b.WriteString("  lexicon lookup          look up the selected word\n")
	b.WriteString("  lexicon lookup --json   machine-readable result\n")
	b.WriteString("  lexicon doctor          check the installation\n")
	b.WriteString("  lexicon languages       manage translation packs\n\n")

	if len(m.packResults) > 0 {
		b.WriteString("Model packs\n")
		for _, result := range m.packResults {
			if result.Error == nil {
				fmt.Fprintf(&b, "  installed  %s\n", result.ID)
				continue
			}
			fmt.Fprintf(&b, "  failed     %s: %v\n", result.ID, result.Error)
			fmt.Fprintf(&b, "             %s\n", result.RetryCommand())
		}
		b.WriteString("\n")
	}

	shortcuts := installer.Shortcuts()
	shortcut := shortcuts[m.shortcutIndex]
	b.WriteString("Global shortcut\n")
	fmt.Fprintf(&b, "  %s: %s\n\n", shortcut.Name, shortcut.Hint)
	b.WriteString("Open the popup and use the language chip to choose an installed pair.\n")

	if m.plan.InstallUnit || m.plan.EnableUnit {
		b.WriteString("\nService: systemctl --user status lexicon.service\n")
	}
	return b.String()
}

func (m model) updateDone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left":
		count := len(installer.Shortcuts())
		m.shortcutIndex = (m.shortcutIndex + count - 1) % count
		return m, nil
	case "right":
		m.shortcutIndex = (m.shortcutIndex + 1) % len(installer.Shortcuts())
		return m, nil
	case "enter", "q", "esc":
		return m, tea.Quit
	}
	return m, nil
}
