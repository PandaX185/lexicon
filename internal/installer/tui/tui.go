// Package tui implements the keyboard-driven installer.
package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/installer"
	"github.com/PandaX185/lexicon/internal/paths"
	"github.com/PandaX185/lexicon/internal/translation"
)

type screen int

const (
	screenWelcome screen = iota
	screenPreflight
	screenExisting
	screenPaths
	screenLanguages
	screenAppearance
	screenPacks
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
	pathsField      pathField

	existingChoice int
	replaceConfig  bool

	appearance      installer.AppearanceSettings
	appearanceField int
	themeInput      textinput.Model
	maxWordsInput   textinput.Model
	maxCharsInput   textinput.Model
	appearanceErr   string

	packEntries  []translation.CatalogEntry
	packSelected map[string]bool
	packCursor   int
	packErr      string

	shortcutIndex int

	plan     installer.Plan
	planErr  error
	applyErr error
	err      error

	progress   installer.Progress
	progressCh chan installer.Progress
	spinner    spinner.Model

	packResults []installer.PackResult
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
	defaults := config.Default()

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
		appearance: installer.AppearanceSettings{
			Theme:             defaults.Popup.Theme,
			ShowPronunciation: boolPtr(defaults.Dictionary.ShowPronunciation),
			ShowHeader:        boolPtr(defaults.Popup.ShowHeader),
			ShowStatus:        boolPtr(defaults.Popup.ShowStatus),
			ShowFooter:        boolPtr(defaults.Popup.ShowFooter),
			MaxQueryWords:     intPtr(defaults.General.MaxQueryWords),
			MaxQueryChars:     intPtr(defaults.General.MaxQueryChars),
		},
		themeInput:    newInput("Theme preset", defaults.Popup.Theme),
		maxWordsInput: newInput("Max query words", strconv.Itoa(defaults.General.MaxQueryWords)),
		maxCharsInput: newInput("Max query characters", strconv.Itoa(defaults.General.MaxQueryChars)),
		packSelected:  map[string]bool{},
		spinner:       spin,
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

func boolPtr(value bool) *bool { return &value }

func intPtr(value int) *int { return &value }

func (m model) startService() (tea.Model, tea.Cmd) {
	m.screen = screenService
	return m, nil
}

type preflightMsg struct {
	checks          []installer.Check
	configExists    bool
	packagedPath    string
	packagedVersion string
}

type progressMsg installer.Progress

type applyDoneMsg struct {
	err   error
	packs []installer.PackResult
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
	case packCatalogMsg:
		(&m).applyPackCatalog(msg)
		return m, nil
	case applyDoneMsg:
		if msg.err != nil {
			m.applyErr = msg.err
			m.screen = screenError
			return m, nil
		}
		m.applyErr = nil
		m.packResults = msg.packs
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
		case "tab", "shift+tab":
			m.pathsField = (m.pathsField + 1) % 2
			return m, m.syncFocus()
		case "enter":
			m.err = nil
			configPath := strings.TrimSpace(m.configInput.Value())
			if configPath == "" {
				configPath = m.options.ConfigPath
			}
			dictPath, err := validateDictionaryValue(m.ctx, m.dictionaryInput.Value())
			if err != nil {
				m.err = err
				return m, nil
			}
			m.settings.ConfigPath = configPath
			m.settings.Dictionary = dictPath
			return m.startLanguages()
		default:
			input := &m.configInput
			if m.pathsField == pathFieldDictionary {
				input = &m.dictionaryInput
			}
			var cmd tea.Cmd
			*input, cmd = input.Update(msg)
			return m, cmd
		}

	case screenLanguages:
		switch key {
		case "esc":
			return m.startPaths()
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
			return m.startAppearance()
		}
		var cmd tea.Cmd
		if m.focus == 0 {
			m.sourceInput, cmd = m.sourceInput.Update(msg)
		} else {
			m.targetInput, cmd = m.targetInput.Update(msg)
		}
		return m, cmd

	case screenAppearance:
		return m.updateAppearance(msg)

	case screenPacks:
		return m.updatePacks(msg)

	case screenService:
		switch key {
		case "esc":
			return m.startPacks()
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
			total := len(m.plan.Steps()) + len(m.plan.PackIDs)
			m.progressCh = make(chan installer.Progress, 8)
			m.progress = installer.Progress{Step: "Preparing", Total: total}
			m.screen = screenProgress
			return m, tea.Batch(installCmd(m.ctx, m.options, m.plan, m.progressCh), waitForProgress(m.progressCh))
		}

	case screenDone:
		return m.updateDone(msg)

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
	m.pathsField = pathFieldConfig
	m.err = nil
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
		if m.pathsField == pathFieldDictionary {
			return m.dictionaryInput.Focus()
		}
		return m.configInput.Focus()
	case screenLanguages:
		if m.focus == 0 {
			return m.sourceInput.Focus()
		}
		return m.targetInput.Focus()
	case screenAppearance:
		switch m.appearanceField {
		case fieldTheme:
			return m.themeInput.Focus()
		case fieldMaxWords:
			return m.maxWordsInput.Focus()
		case fieldMaxChars:
			return m.maxCharsInput.Focus()
		}
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

func installCmd(ctx context.Context, options installer.Options, plan installer.Plan, ch chan<- installer.Progress) tea.Cmd {
	return func() tea.Msg {
		base := len(plan.Steps())
		total := base + len(plan.PackIDs)
		if err := installer.Apply(ctx, plan, func(progress installer.Progress) { ch <- progress }); err != nil {
			close(ch)
			return applyDoneMsg{err: err}
		}
		results := installer.InstallPacks(ctx, options, plan, func(progress installer.PackProgress) {
			ch <- installer.Progress{
				Step:  progress.Step(),
				Index: base + progress.Index,
				Total: total,
			}
		})
		close(ch)
		return applyDoneMsg{packs: results}
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

// validateDictionaryValue expands a leading ~/ and checks that a supplied
// dictionary path points at a usable SQLite dictionary. An empty value means
// the dictionary will be configured later.
func validateDictionaryValue(ctx context.Context, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	expanded, err := paths.ExpandHome(value)
	if err != nil {
		return "", err
	}
	if err := installer.ValidateDictionary(ctx, expanded); err != nil {
		return "", err
	}
	return expanded, nil
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
	case screenLanguages:
		body, help = m.viewLanguages()
	case screenAppearance:
		body, help = m.viewAppearance()
	case screenPacks:
		body, help = m.viewPacks()
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

func screenName(value screen) string {
	switch value {
	case screenWelcome:
		return "welcome"
	case screenPreflight:
		return "preflight"
	case screenExisting:
		return "existing"
	case screenPaths:
		return "paths"
	case screenLanguages:
		return "languages"
	case screenAppearance:
		return "appearance"
	case screenPacks:
		return "packs"
	case screenService:
		return "service"
	case screenSummary:
		return "summary"
	case screenProgress:
		return "progress"
	case screenDone:
		return "done"
	case screenError:
		return "error"
	default:
		return "unknown"
	}
}
