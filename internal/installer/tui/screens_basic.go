package tui

import (
	"fmt"
	"strings"

	"github.com/PandaX185/lexicon/internal/installer"
	"github.com/PandaX185/lexicon/internal/paths"
)

func (m model) viewWelcome() (string, string) {
	body := strings.Join([]string{
		"This wizard configures Lexicon and starts its background service.",
		"",
		"It writes only user files under your home directory.",
		"System files are never modified.",
		"",
		"You can change every setting later with `lexicon config`.",
	}, "\n")
	return body, "enter  begin    q  quit"
}

func (m model) viewPreflight() (string, string) {
	var b strings.Builder
	b.WriteString("Checking this system\n\n")
	for _, check := range m.checks {
		b.WriteString(renderCheck(check))
		b.WriteString("\n")
	}
	if m.hasFailure() {
		b.WriteString("\n")
		b.WriteString(failStyle.Render("Fix the failures above before continuing."))
	}
	return b.String(), "enter  continue    r  recheck    esc  back    q  quit"
}

func (m model) viewExisting() (string, string) {
	options := []string{
		"Keep it and repair the service",
		"Replace it with new settings",
		"Cancel",
	}
	var b strings.Builder
	b.WriteString("A configuration already exists\n\n")
	b.WriteString("  ")
	b.WriteString(valueStyle.Render(m.settings.ConfigPath))
	b.WriteString("\n\n")
	for i, option := range options {
		cursor := "  "
		style := valueStyle
		if i == m.existingChoice {
			cursor = "> "
			style = selectedStyle
		}
		b.WriteString(cursor)
		b.WriteString(style.Render(option))
		b.WriteString("\n")
	}
	return b.String(), "up/down  choose    enter  confirm    q  quit"
}

func (m model) viewPaths() (string, string) {
	body := "Install paths\n\n" +
		labelStyle.Render("Configuration file") + "\n" +
		m.configInput.View() + "\n\n" +
		labelStyle.Render("Dictionary location") + "\n" +
		valueStyle.Render(dictionaryLocation(m)) + "\n\n" +
		subtitleStyle.Render("A packaged dictionary is used in place; a local file is copied.")
	return body, "enter/tab  continue    esc  back"
}

func dictionaryLocation(m model) string {
	if m.packagedPath != "" {
		return m.packagedPath + " (packaged)"
	}
	if target, err := paths.DefaultDictionaryPath(); err == nil {
		return target
	}
	return "~/.local/share/lexicon/dictionary.sqlite"
}

func (m model) viewDictionary() (string, string) {
	var b strings.Builder
	b.WriteString("Dictionary\n\n")
	if m.packagedPath != "" {
		version := ""
		if m.packagedVersion != "" {
			version = " version " + m.packagedVersion
		}
		b.WriteString(okStyle.Render("Found packaged dictionary"))
		b.WriteString(": ")
		b.WriteString(m.packagedPath)
		b.WriteString(version)
		b.WriteString("\n\n")
	} else {
		b.WriteString(warnStyle.Render("No packaged dictionary found.") + " Download one from the releases page,\n")
		b.WriteString("or leave this empty and configure it later.\n\n")
	}
	b.WriteString(m.dictionaryInput.View() + "\n")
	if m.dictionaryErr != "" {
		b.WriteString("\n" + failStyle.Render(m.dictionaryErr) + "\n")
	}
	return b.String(), "enter/tab  continue    esc  back"
}

func (m model) viewLanguages() (string, string) {
	body := "Languages\n\n" +
		labelStyle.Render("Headword language") + "\n" +
		m.sourceInput.View() + "\n\n" +
		labelStyle.Render("Definition language") + "\n" +
		m.targetInput.View() + "\n\n" +
		subtitleStyle.Render("BCP 47 tags, for example en, de, or pt-BR.")
	return body, "tab  next field    enter  continue    esc  back"
}

func renderCheck(check installer.Check) string {
	marker := "✓"
	style := okStyle
	switch check.Status {
	case installer.StatusWarn:
		marker = "!"
		style = warnStyle
	case installer.StatusFail:
		marker = "✗"
		style = failStyle
	}
	return style.Render(marker) + " " + labelStyle.Render(fmt.Sprintf("%-18s", check.Name)) + check.Detail
}

func toggle(on bool) string {
	if on {
		return okStyle.Render("[x]")
	}
	return labelStyle.Render("[ ]")
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
