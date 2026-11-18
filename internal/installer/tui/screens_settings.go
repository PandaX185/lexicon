package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/installer"
)

const (
	fieldTheme = iota
	fieldPronunciation
	fieldHideOnBlur
	fieldHeader
	fieldStatus
	fieldFooter
	fieldMaxWords
	fieldMaxChars
	fieldCount
)

func (m model) startAppearance() (tea.Model, tea.Cmd) {
	m.screen = screenAppearance
	m.appearanceField = fieldTheme
	return m, m.syncFocus()
}

func (m model) updateAppearance(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		return m.startDictionary()
	case "up", "k":
		m.appearanceField = (m.appearanceField + fieldCount - 1) % fieldCount
		return m, m.syncFocus()
	case "down", "j", "tab":
		m.appearanceField = (m.appearanceField + 1) % fieldCount
		return m, m.syncFocus()
	case "shift+tab":
		m.appearanceField = (m.appearanceField + fieldCount - 1) % fieldCount
		return m, m.syncFocus()
	case "left", "right":
		if m.appearanceField == fieldTheme {
			m.appearance.Theme = cycleTheme(m.appearance.Theme, key == "right")
			m.themeInput.SetValue(m.appearance.Theme)
		}
		return m, nil
	case " ":
		m.toggleAppearanceBool()
		return m, nil
	case "enter":
		m.appearance.Theme = strings.TrimSpace(m.themeInput.Value())
		words, wordsErr := strconv.Atoi(strings.TrimSpace(m.maxWordsInput.Value()))
		chars, charsErr := strconv.Atoi(strings.TrimSpace(m.maxCharsInput.Value()))
		if wordsErr != nil || charsErr != nil {
			m.appearanceErr = "query limits must be whole numbers"
			return m, nil
		}
		m.appearance.MaxQueryWords = intPtr(words)
		m.appearance.MaxQueryChars = intPtr(chars)
		if err := installer.ValidateAppearance(m.appearance); err != nil {
			m.appearanceErr = err.Error()
			return m, nil
		}
		m.settings.Appearance = m.appearance
		m.appearanceErr = ""
		return m.startPacks()
	}

	var cmd tea.Cmd
	switch m.appearanceField {
	case fieldTheme:
		m.themeInput, cmd = m.themeInput.Update(msg)
	case fieldMaxWords:
		m.maxWordsInput, cmd = m.maxWordsInput.Update(msg)
	case fieldMaxChars:
		m.maxCharsInput, cmd = m.maxCharsInput.Update(msg)
	}
	return m, cmd
}

func (m *model) toggleAppearanceBool() {
	switch m.appearanceField {
	case fieldPronunciation:
		m.appearance.ShowPronunciation = boolPtr(!*m.appearance.ShowPronunciation)
	case fieldHideOnBlur:
		m.appearance.HideOnBlur = boolPtr(!*m.appearance.HideOnBlur)
	case fieldHeader:
		m.appearance.ShowHeader = boolPtr(!*m.appearance.ShowHeader)
	case fieldStatus:
		m.appearance.ShowStatus = boolPtr(!*m.appearance.ShowStatus)
	case fieldFooter:
		m.appearance.ShowFooter = boolPtr(!*m.appearance.ShowFooter)
	}
}

func cycleTheme(current string, forward bool) string {
	for index, theme := range config.PopupThemes {
		if theme != current {
			continue
		}
		step := 1
		if !forward {
			step = -1
		}
		return config.PopupThemes[(index+step+len(config.PopupThemes))%len(config.PopupThemes)]
	}
	return config.PopupThemes[0]
}

func (m model) viewAppearance() (string, string) {
	var b strings.Builder
	b.WriteString("Appearance\n\n")
	rows := []struct {
		label string
		value string
	}{
		{"Theme", m.themeInput.View() + "  (left/right cycles presets)"},
		{"Pronunciation", toggle(*m.appearance.ShowPronunciation)},
		{"Hide on blur", toggle(*m.appearance.HideOnBlur)},
		{"Header", toggle(*m.appearance.ShowHeader)},
		{"Connection badge", toggle(*m.appearance.ShowStatus)},
		{"Footer", toggle(*m.appearance.ShowFooter)},
		{"Max query words", m.maxWordsInput.View()},
		{"Max query characters", m.maxCharsInput.View()},
	}
	for index, row := range rows {
		cursor := "  "
		if index == m.appearanceField {
			cursor = "> "
		}
		fmt.Fprintf(&b, "%s%-20s %s\n", cursor, row.label, row.value)
	}
	if m.appearanceErr != "" {
		b.WriteString("\n" + failStyle.Render(m.appearanceErr) + "\n")
	}
	return b.String(), "up/down  field    left/right  theme    space  toggle    enter  continue    esc  back"
}
