// Package tui implements the keyboard-driven installer.
package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/PandaX185/lexicon/internal/installer"
	"github.com/PandaX185/lexicon/internal/paths"
)

type screen int

const (
	screenWelcome screen = iota
	screenPreflight
	screenExisting
	screenPaths
	screenDictionary
	screenLanguages
	screenTranslation
	screenService
	screenSummary
	screenProgress
	screenDone
	screenError
)

type model struct {
	ctx      context.Context
	options  installer.Options
	settings installer.Settings
	screen   screen

	width  int
	height int

	checks          []installer.Check
	preflightDone   bool
	configExists    bool
	packagedPath    string
	packagedVersion string

	configInput     textinput.Model
	dictionaryInput textinput.Model
	sourceInput     textinput.Model
	targetInput     textinput.Model
	focus           int

	existingChoice int
	replaceConfig  bool
	dictionaryErr  string

	plan     installer.Plan
	planErr  error
	applyErr error

	progress   installer.Progress
	progressCh chan installer.Progress
	spinner    spinner.Model
}

// Run starts the installer UI and returns any installation error.
func Run(ctx context.Context, options installer.Options) error {
	final, err := tea.NewProgram(newModel(ctx, options), tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	if result, ok := final.(model); ok {
		return result.applyErr
	}
	return nil
}

func newModel(ctx context.Context, options installer.Options) model {
	configPath := options.ConfigPath
	if configPath == "" {
		if resolved, err := paths.ConfigPath(); err == nil {
			configPath = resolved
		}
	}
	sourceLang := options.SourceLang
	if sourceLang == "" {
		sourceLang = "en"
	}
	targetLang := options.TargetLang
	if targetLang == "" {
		targetLang = "en"
	}

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	m := model{
		ctx:     ctx,
		options: options,
		settings: installer.Settings{
			ConfigPath:     configPath,
			Dictionary:     options.Dictionary,
			ModelDir:       options.ModelDir,
			SourceLang:     sourceLang,
			TargetLang:     targetLang,
			Translation:    options.Translation,
			InstallService: !options.SkipService,
			ReplaceConfig:  options.Force,
		},
		replaceConfig:   options.Force,
		screen:          screenWelcome,
		configInput:     newInput("Configuration file", configPath),
		dictionaryInput: newInput("Dictionary .sqlite path (empty to configure later)", options.Dictionary),
		sourceInput:     newInput("Headword language", sourceLang),
		targetInput:     newInput("Definition language", targetLang),
		spinner:         spin,
	}
	m.syncFocus()
	return m
}

func newInput(placeholder, value string) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.CharLimit = 4096
	input.Width = 64
	input.Prompt = "> "
	input.SetValue(value)
	return input
}

type preflightMsg struct {
	checks          []installer.Check
	configExists    bool
	packagedPath    string
	packagedVersion string
}

type progressMsg installer.Progress

type applyDoneMsg struct {
	err error
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.runPreflight())
}

func (m model) runPreflight() tea.Cmd {
	return func() tea.Msg {
		checks := installer.CheckEnvironment(m.ctx, m.options)
		path, version, _ := installer.DiscoverPackaged(m.options)
		_, statErr := os.Stat(m.settings.ConfigPath)
		return preflightMsg{
			checks:          checks,
			configExists:    statErr == nil,
			packagedPath:    path,
			packagedVersion: version,
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case preflightMsg:
		m.checks = msg.checks
		m.preflightDone = true
		m.configExists = msg.configExists
		m.packagedPath = msg.packagedPath
		m.packagedVersion = msg.packagedVersion
		if m.dictionaryInput.Value() == "" && m.packagedPath != "" {
			m.dictionaryInput.SetValue(m.packagedPath)
		}
		if !m.serviceAvailable() {
			m.settings.InstallService = false
		}
		if m.screen == screenWelcome || m.screen == screenPreflight {
			m.screen = screenPreflight
		}
		return m, nil
	case progressMsg:
		m.progress = installer.Progress(msg)
		return m, waitForProgress(m.progressCh)
	case applyDoneMsg:
		if msg.err != nil {
			m.applyErr = msg.err
			m.screen = screenError
			return m, nil
		}
		m.applyErr = nil
		m.screen = screenDone
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		if m.screen == screenProgress {
			return m, nil
		}
		return m, tea.Quit
	}

	switch m.screen {
	case screenWelcome:
		switch key {
		case "q", "esc":
			return m, tea.Quit
		case "enter":
			if !m.preflightDone {
				m.screen = screenPreflight
				return m, nil
			}
			if m.configExists && !m.replaceConfig {
				m.screen = screenExisting
				return m, nil
			}
			return m.startPaths()
		}

	case screenPreflight:
		switch key {
		case "q":
			return m, tea.Quit
		case "esc":
			m.screen = screenWelcome
			return m, nil
		case "r":
			m.preflightDone = false
			return m, m.runPreflight()
		case "enter":
			if !m.preflightDone {
				return m, nil
			}
			if m.hasFailure() {
				return m, nil
			}
			if m.configExists && !m.replaceConfig {
				m.screen = screenExisting
				return m, nil
			}
			return m.startPaths()
		}

	case screenExisting:
		switch key {
		case "q":
			return m, tea.Quit
		case "esc":
			m.screen = screenPreflight
			return m, nil
		case "up", "k":
			if m.existingChoice > 0 {
				m.existingChoice--
			}
			return m, nil
		case "down", "j":
			if m.existingChoice < 2 {
				m.existingChoice++
			}
			return m, nil
		case "enter":
			switch m.existingChoice {
			case 0:
				m.settings.ReplaceConfig = false
				m.showSummary()
				return m, nil
			case 1:
				m.settings.ReplaceConfig = true
				return m.startPaths()
			default:
				return m, tea.Quit
			}
		}

	case screenPaths:
		switch key {
		case "esc":
			m.screen = screenPreflight
			return m, nil
		case "enter":
			m.settings.ConfigPath = strings.TrimSpace(m.configInput.Value())
			if m.settings.ConfigPath == "" {
				m.settings.ConfigPath = m.options.ConfigPath
			}
			return m.startDictionary()
		}
		var cmd tea.Cmd
		m.configInput, cmd = m.configInput.Update(msg)
		return m, cmd

	case screenDictionary:
		switch key {
		case "esc":
			return m.startPaths()
		case "enter":
			value := strings.TrimSpace(m.dictionaryInput.Value())
			if value == "" {
				m.settings.Dictionary = ""
				m.dictionaryErr = ""
				return m.startLanguages()
			}
			expanded, err := paths.ExpandHome(value)
			if err != nil {
				m.dictionaryErr = err.Error()
				return m, nil
			}
			if err := installer.ValidateDictionary(m.ctx, expanded); err != nil {
				m.dictionaryErr = err.Error()
				return m, nil
			}
			m.settings.Dictionary = expanded
			m.dictionaryInput.SetValue(expanded)
			m.dictionaryErr = ""
			return m.startLanguages()
		}
		var cmd tea.Cmd
		m.dictionaryInput, cmd = m.dictionaryInput.Update(msg)
		return m, cmd

	case screenLanguages:
		switch key {
		case "esc":
			if m.options.DictionaryURL != "" {
				return m.startPaths()
			}
			return m.startDictionary()
		case "tab", "shift+tab":
			m.focus = 1 - m.focus
			return m, m.syncFocus()
		case "enter":
			if m.focus == 0 {
				m.focus = 1
				return m, m.syncFocus()
			}
			m.settings.SourceLang = languageOrEnglish(m.sourceInput.Value())
			m.settings.TargetLang = languageOrEnglish(m.targetInput.Value())
			m.screen = screenTranslation
			return m, nil
		}
		var cmd tea.Cmd
		if m.focus == 0 {
			m.sourceInput, cmd = m.sourceInput.Update(msg)
		} else {
			m.targetInput, cmd = m.targetInput.Update(msg)
		}
		return m, cmd

	case screenTranslation:
		switch key {
		case "esc":
			m.screen = screenLanguages
			m.focus = 1
			return m, m.syncFocus()
		case " ", "t":
			m.settings.Translation = !m.settings.Translation
			return m, nil
		case "enter":
			m.screen = screenService
			return m, nil
		}

	case screenService:
		switch key {
		case "esc":
			m.screen = screenTranslation
			return m, nil
		case " ", "s":
			if m.serviceAvailable() {
				m.settings.InstallService = !m.settings.InstallService
			}
			return m, nil
		case "enter":
			m.showSummary()
			return m, nil
		}

	case screenSummary:
		switch key {
		case "q":
			return m, tea.Quit
		case "esc":
			m.screen = screenService
			return m, nil
		case "enter":
			if failed := m.plan.FailedChecks(); len(failed) > 0 {
				m.applyErr = fmt.Errorf("%s: %s", failed[0].Name, failed[0].Detail)
				m.screen = screenError
				return m, nil
			}
			if m.options.DryRun || len(m.plan.Steps()) == 0 {
				m.screen = screenDone
				return m, nil
			}
			m.progressCh = make(chan installer.Progress, 8)
			m.progress = installer.Progress{Step: "Preparing", Total: len(m.plan.Steps())}
			m.screen = screenProgress
			return m, tea.Batch(applyCmd(m.ctx, m.plan, m.progressCh), waitForProgress(m.progressCh))
		}

	case screenDone:
		switch key {
		case "enter", "q", "esc":
			return m, tea.Quit
		}

	case screenError:
		switch key {
		case "q":
			return m, tea.Quit
		case "esc":
			if m.planErr != nil {
				m.screen = screenService
			} else {
				m.screen = screenSummary
			}
			return m, nil
		}
	}
	return m, nil
}

func (m model) startPaths() (tea.Model, tea.Cmd) {
	m.screen = screenPaths
	return m, m.syncFocus()
}

func (m model) startDictionary() (tea.Model, tea.Cmd) {
	if m.options.DictionaryURL != "" {
		return m.startLanguages()
	}
	m.screen = screenDictionary
	return m, m.syncFocus()
}

func (m model) startLanguages() (tea.Model, tea.Cmd) {
	m.screen = screenLanguages
	m.focus = 0
	return m, m.syncFocus()
}

func (m *model) syncFocus() tea.Cmd {
	m.configInput.Blur()
	m.dictionaryInput.Blur()
	m.sourceInput.Blur()
	m.targetInput.Blur()
	switch m.screen {
	case screenPaths:
		return m.configInput.Focus()
	case screenDictionary:
		return m.dictionaryInput.Focus()
	case screenLanguages:
		if m.focus == 0 {
			return m.sourceInput.Focus()
		}
		return m.targetInput.Focus()
	}
	return nil
}

func (m *model) showSummary() {
	m.settings.ConfigPath = strings.TrimSpace(m.configInput.Value())
	if m.settings.ConfigPath == "" {
		m.settings.ConfigPath = m.options.ConfigPath
	}
	m.settings.SourceLang = languageOrEnglish(m.sourceInput.Value())
	m.settings.TargetLang = languageOrEnglish(m.targetInput.Value())

	plan, err := installer.BuildPlan(m.ctx, m.options, m.settings)
	if err != nil {
		m.planErr = err
		m.applyErr = nil
		m.screen = screenError
		return
	}
	m.plan = plan
	m.planErr = nil
	m.applyErr = nil
	m.screen = screenSummary
}

func applyCmd(ctx context.Context, plan installer.Plan, ch chan<- installer.Progress) tea.Cmd {
	return func() tea.Msg {
		err := installer.Apply(ctx, plan, func(progress installer.Progress) { ch <- progress })
		close(ch)
		return applyDoneMsg{err: err}
	}
}

func waitForProgress(ch <-chan installer.Progress) tea.Cmd {
	return func() tea.Msg {
		progress, ok := <-ch
		if !ok {
			return nil
		}
		return progressMsg(progress)
	}
}

func (m model) hasFailure() bool {
	for _, check := range m.checks {
		if check.Status == installer.StatusFail {
			return true
		}
	}
	return false
}

func (m model) serviceAvailable() bool {
	for _, check := range m.checks {
		if (check.Name == "daemon" || check.Name == "systemd") && check.Status != installer.StatusOK {
			return false
		}
	}
	return true
}

func languageOrEnglish(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "en"
	}
	return value
}

func (m model) View() string {
	body := ""
	help := ""
	switch m.screen {
	case screenWelcome:
		body, help = m.viewWelcome()
	case screenPreflight:
		body, help = m.viewPreflight()
	case screenExisting:
		body, help = m.viewExisting()
	case screenPaths:
		body, help = m.viewPaths()
	case screenDictionary:
		body, help = m.viewDictionary()
	case screenLanguages:
		body, help = m.viewLanguages()
	case screenTranslation:
		body, help = m.viewTranslation()
	case screenService:
		body, help = m.viewService()
	case screenSummary:
		body, help = m.viewSummary()
	case screenProgress:
		body, help = m.viewProgress()
	case screenDone:
		body = m.viewDone()
		help = "enter  finish"
	case screenError:
		body, help = m.viewError()
	}

	header := titleStyle.Render("Lexicon setup") + "\n" +
		subtitleStyle.Render("Offline dictionary for Wayland") + "\n"
	return header + "\n" + body + "\n" + helpStyle.Render(help)
}

func (m model) viewWelcome() (string, string) {
	body := strings.Join([]string{
		"This wizard configures Lexicon and starts its background service.",
		"",
		"It writes only user files under your home directory.",
		"System files are never modified.",
		"",
		"You can change every setting later with `lex config`.",
	}, "\n")
	return body, "enter  begin    q  quit"
}

func (m model) viewPreflight() (string, string) {
	var b strings.Builder
	b.WriteString("Checking this system\n\n")
	for _, check := range m.checks {
		b.WriteString(renderCheck(check) + "\n")
	}
	if m.hasFailure() {
		b.WriteString("\n" + failStyle.Render("Fix the failures above before continuing."))
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
	b.WriteString("  " + valueStyle.Render(m.settings.ConfigPath) + "\n\n")
	for i, option := range options {
		cursor := "  "
		style := valueStyle
		if i == m.existingChoice {
			cursor = "> "
			style = selectedStyle
		}
		b.WriteString(cursor + style.Render(option) + "\n")
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
	return body, "enter  continue    esc  back"
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
		b.WriteString(okStyle.Render("Found packaged dictionary") + ": " + m.packagedPath + version + "\n\n")
	} else {
		b.WriteString(warnStyle.Render("No packaged dictionary found.") + " Download one from the releases page,\n")
		b.WriteString("or leave this empty and configure it later.\n\n")
	}
	b.WriteString(m.dictionaryInput.View() + "\n")
	if m.dictionaryErr != "" {
		b.WriteString("\n" + failStyle.Render(m.dictionaryErr) + "\n")
	}
	return b.String(), "enter  continue    esc  back"
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

func (m model) viewTranslation() (string, string) {
	body := "Offline translation\n\n" +
		toggle(m.settings.Translation) + " Enable translation when languages differ\n\n" +
		labelStyle.Render("Model directory") + "  " + valueStyle.Render(m.modelDirectory()) + "\n\n" +
		subtitleStyle.Render("Translation needs a Bergamot-enabled build and an installed model pack.\n"+
			"Lexicon works without it. See docs/TRANSLATION.md.")
	return body, "space  toggle    enter  continue    esc  back"
}

func (m model) modelDirectory() string {
	if strings.TrimSpace(m.settings.ModelDir) != "" {
		return m.settings.ModelDir
	}
	if dir, err := paths.DefaultModelDir(); err == nil {
		return dir
	}
	return "~/.local/share/lexicon/models"
}

func (m model) viewService() (string, string) {
	var b strings.Builder
	b.WriteString("Background service\n\n")
	available := m.serviceAvailable()
	if available {
		b.WriteString(toggle(m.settings.InstallService) + " Install and start the systemd user service\n\n")
		if path := m.daemonPath(); path != "" {
			b.WriteString(labelStyle.Render("Daemon") + "  " + path + "\n")
		}
		b.WriteString(subtitleStyle.Render("The service starts on login and launches the popup."))
	} else {
		b.WriteString(warnStyle.Render("The service is unavailable on this system.") + "\n")
		for _, check := range m.checks {
			if (check.Name == "daemon" || check.Name == "systemd") && check.Status != installer.StatusOK {
				b.WriteString("  " + check.Detail + "\n")
			}
		}
	}
	return b.String(), "space  toggle    enter  continue    esc  back"
}

func (m model) daemonPath() string {
	for _, check := range m.checks {
		if check.Name == "daemon" && check.Status == installer.StatusOK {
			return check.Detail
		}
	}
	return ""
}

func (m model) viewSummary() (string, string) {
	plan := m.plan
	var b strings.Builder
	b.WriteString("Summary\n\n")
	config := plan.ConfigPath + " (new)"
	if !plan.WriteConfig {
		config = plan.ConfigPath + " (kept)"
	}
	fmt.Fprintf(&b, "  %-14s %s\n", labelStyle.Render("configuration"), config)
	fmt.Fprintf(&b, "  %-14s %s\n", labelStyle.Render("dictionary"), summaryDictionary(plan))
	fmt.Fprintf(&b, "  %-14s %s -> %s\n", labelStyle.Render("languages"),
		plan.Config.Dictionary.SourceLanguage, plan.Config.Dictionary.TargetLanguage)
	fmt.Fprintf(&b, "  %-14s %s\n", labelStyle.Render("translation"), onOff(plan.Config.Translation.Enabled))
	if plan.Config.Translation.Enabled {
		modelDir := plan.Config.Translation.ModelDirectory
		if modelDir == "" {
			modelDir = "(default model directory)"
		}
		fmt.Fprintf(&b, "  %-14s %s\n", labelStyle.Render("models"), modelDir)
	}
	fmt.Fprintf(&b, "  %-14s %s\n", labelStyle.Render("service"), summaryService(plan))

	for _, note := range plan.Notes {
		b.WriteString("\n" + subtitleStyle.Render(note))
	}
	if failed := plan.FailedChecks(); len(failed) > 0 {
		b.WriteString("\n")
		for _, check := range failed {
			b.WriteString(failStyle.Render("✗ "+check.Name+": "+check.Detail) + "\n")
		}
	}
	if m.options.DryRun {
		b.WriteString("\n" + warnStyle.Render("Dry run: no changes will be made."))
	}
	return b.String(), summaryHelp(m.options.DryRun)
}

func summaryHelp(dryRun bool) string {
	if dryRun {
		return "enter  finish (dry run)    esc  back    q  quit"
	}
	return "enter  install    esc  back    q  quit"
}

func summaryDictionary(plan installer.Plan) string {
	switch {
	case plan.DictionaryPackaged:
		version := ""
		if plan.DictionaryVersion != "" {
			version = " " + plan.DictionaryVersion
		}
		return plan.DictionaryTarget + " (packaged" + version + ", no copy)"
	case plan.DownloadURL != "":
		return plan.DownloadURL + " -> " + plan.DictionaryTarget
	case plan.CopyDictionary:
		return plan.DictionarySource + " -> " + plan.DictionaryTarget
	case plan.DictionaryTarget != "":
		return plan.DictionaryTarget + " (unchanged)"
	default:
		return "not configured"
	}
}

func summaryService(plan installer.Plan) string {
	switch {
	case plan.InstallUnit:
		return plan.UnitPath
	case plan.EnableUnit:
		return "existing unit at " + plan.UnitPath + " will be started"
	default:
		return "not installed"
	}
}

func (m model) viewProgress() (string, string) {
	var b strings.Builder
	b.WriteString("Installing\n\n")
	if m.progress.Total > 0 {
		fmt.Fprintf(&b, "  [%d/%d] %s\n", m.progress.Index, m.progress.Total, m.progress.Step)
	} else {
		b.WriteString("  " + m.progress.Step + "\n")
	}
	b.WriteString("\n" + m.spinner.View() + " working...\n")
	return b.String(), "installing — please wait"
}

func (m model) viewDone() string {
	if m.options.DryRun {
		return "Dry run complete\n\nNo changes were made.\n"
	}
	var b strings.Builder
	b.WriteString(okStyle.Render("Setup complete") + "\n\n")
	b.WriteString("Next steps:\n\n")
	b.WriteString("  lex doctor\n")
	if m.plan.InstallUnit || m.plan.EnableUnit {
		b.WriteString("  systemctl --user status lexicon.service\n")
	} else {
		b.WriteString("  lexicon-daemon --config " + m.plan.ConfigPath + "\n")
	}
	b.WriteString("  systemctl --user import-environment WAYLAND_DISPLAY\n\n")
	b.WriteString(subtitleStyle.Render("Then bind `lex lookup` to a keyboard shortcut in your desktop settings."))
	return b.String()
}

func (m model) viewError() (string, string) {
	err := m.applyErr
	if err == nil {
		err = m.planErr
	}
	var b strings.Builder
	b.WriteString(failStyle.Render("Setup failed") + "\n\n")
	if err != nil {
		b.WriteString(err.Error() + "\n")
	}
	b.WriteString("\n")
	if m.applyErr != nil {
		b.WriteString("Previous files were restored where possible.\n")
	}
	b.WriteString("Run `lex doctor` for more detail.\n")
	return b.String(), "esc  try again    q  quit"
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
