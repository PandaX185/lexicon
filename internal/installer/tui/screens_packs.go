package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PandaX185/lexicon/internal/translation"
)

type packCatalogMsg struct {
	entries []translation.CatalogEntry
	err     error
}

func (m *model) loadPacks() tea.Cmd {
	return func() tea.Msg {
		entries, err := translation.Catalog()
		return packCatalogMsg{entries: entries, err: err}
	}
}

func (m model) startPacks() (tea.Model, tea.Cmd) {
	m.screen = screenPacks
	m.packCursor = 0
	m.packErr = ""
	m.packEntries = nil
	return m, m.loadPacks()
}

func (m model) updatePacks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.startAppearance()
	case "up", "k":
		if m.packCursor > 0 {
			m.packCursor--
		}
		return m, nil
	case "down", "j":
		if m.packCursor+1 < len(m.packEntries) {
			m.packCursor++
		}
		return m, nil
	case " ", "t":
		if len(m.packEntries) == 0 {
			return m, nil
		}
		id := m.packEntries[m.packCursor].ID
		m.packSelected[id] = !m.packSelected[id]
		return m, nil
	case "enter":
		ids := make([]string, 0, len(m.packSelected))
		for id, selected := range m.packSelected {
			if selected {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		m.settings.PackIDs = ids
		m.settings.Translation = len(ids) > 0
		return m.startService()
	}
	return m, nil
}

func (m *model) applyPackCatalog(msg packCatalogMsg) {
	m.packEntries = msg.entries
	if msg.err != nil {
		m.packErr = msg.err.Error()
		return
	}
	m.packErr = ""
	if len(m.packEntries) > 0 {
		m.packCursor = 0
	}
}

func (m model) viewPacks() (string, string) {
	var b strings.Builder
	b.WriteString("Offline translation packs\n\n")
	b.WriteString(subtitleStyle.Render("Select zero or more packs. Each is downloaded over HTTPS and verified.") + "\n\n")
	if m.packErr != "" {
		b.WriteString(failStyle.Render(m.packErr) + "\n\n")
	}
	if len(m.packEntries) == 0 && m.packErr == "" {
		b.WriteString("Loading catalog...\n")
	}
	for index, entry := range m.packEntries {
		cursor := "  "
		mark := "[ ]"
		if index == m.packCursor {
			cursor = "> "
		}
		if m.packSelected[entry.ID] {
			mark = "[x]"
		}
		fmt.Fprintf(&b, "%s%s %-28s %s -> %s", cursor, mark, entry.ID, entry.Source, entry.Target)
		if entry.Architecture != "" {
			fmt.Fprintf(&b, "  (%s)", entry.Architecture)
		}
		b.WriteString("\n")
	}
	if len(m.packEntries) == 0 {
		b.WriteString("\n" + subtitleStyle.Render("No packs are available; definitions still work.") + "\n")
	}
	return b.String(), "up/down  choose    space  toggle    enter  continue    esc  back"
}
