package tui

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/installer"
	"github.com/PandaX185/lexicon/internal/translation"
	_ "modernc.org/sqlite"
)

func testOptions(t *testing.T) installer.Options {
	t.Helper()
	configHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	return installer.Options{
		ConfigPath:           filepath.Join(configHome, "lexicon", "config.toml"),
		DictionarySearchDirs: []string{t.TempDir()},
		SkipService:          true,
		Stdin:                strings.NewReader(""),
		Stdout:               io.Discard,
		Stderr:               io.Discard,
	}
}

func createDictionary(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = db.Close() }()

	statements := []string{
		`CREATE TABLE dictionary_info (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			name TEXT NOT NULL,
			source_language TEXT NOT NULL,
			target_language TEXT NOT NULL,
			license TEXT NOT NULL DEFAULT '',
			attribution TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE entries (
			id INTEGER PRIMARY KEY,
			word TEXT NOT NULL COLLATE NOCASE,
			UNIQUE (word)
		)`,
		`INSERT INTO dictionary_info (id, name, source_language, target_language, license, attribution)
			VALUES (1, 'Test Dictionary', 'en', 'en', 'CC0', 'Test fixture')`,
		`INSERT INTO entries (id, word) VALUES (1, 'go')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("fixture statement failed: %v\n%s", err, statement)
		}
	}
}

func keyEnter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }
func keyEsc() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyEsc} }
func keyUp() tea.KeyMsg    { return tea.KeyMsg{Type: tea.KeyUp} }
func keyDown() tea.KeyMsg  { return tea.KeyMsg{Type: tea.KeyDown} }
func keySpace() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")}
}
func keyCtrlC() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyCtrlC}
}

func press(t *testing.T, m model, keys ...tea.KeyMsg) model {
	t.Helper()
	for _, key := range keys {
		updated, _ := m.Update(key)
		next, ok := updated.(model)
		if !ok {
			t.Fatalf("Update returned %T, want model", updated)
		}
		m = next
	}
	return m
}

func withChecks(m model, checks ...installer.Check) model {
	updated, _ := m.Update(preflightMsg{checks: checks})
	return updated.(model)
}

func TestNewModelStartsAtWelcome(t *testing.T) {
	options := testOptions(t)
	m := newModel(context.Background(), options)
	if m.screen != screenWelcome {
		t.Fatalf("screen = %d, want welcome", m.screen)
	}
	if m.settings.ConfigPath != options.ConfigPath {
		t.Fatalf("config = %q, want %s", m.settings.ConfigPath, options.ConfigPath)
	}
	if m.settings.SourceLang != "en" || m.settings.TargetLang != "en" {
		t.Fatalf("languages = %q/%q, want en/en", m.settings.SourceLang, m.settings.TargetLang)
	}
}

func TestPreflightMessageUpdatesState(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.settings.InstallService = true
	m = withChecks(m,
		installer.Check{Name: "daemon", Status: installer.StatusWarn, Detail: "missing"},
		installer.Check{Name: "systemd", Status: installer.StatusOK},
	)
	if m.screen != screenPreflight {
		t.Fatalf("screen = %d, want preflight", m.screen)
	}
	if m.settings.InstallService {
		t.Fatal("service must be disabled when the daemon is unavailable")
	}
}

func TestPreflightFillsPackagedDictionary(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	updated, _ := m.Update(preflightMsg{
		checks:          []installer.Check{{Name: "daemon", Status: installer.StatusOK}, {Name: "systemd", Status: installer.StatusOK}},
		packagedPath:    "/usr/share/lexicon/en.sqlite",
		packagedVersion: "3.0",
	})
	m = updated.(model)
	if m.packagedPath != "/usr/share/lexicon/en.sqlite" || m.packagedVersion != "3.0" {
		t.Fatalf("packaged = %q %q", m.packagedPath, m.packagedVersion)
	}
	if m.dictionaryInput.Value() != "/usr/share/lexicon/en.sqlite" {
		t.Fatalf("dictionary input = %q, want the packaged path", m.dictionaryInput.Value())
	}
}

func TestPreflightFailureBlocksContinue(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m = withChecks(m, installer.Check{Name: "data directory", Status: installer.StatusFail, Detail: "denied"})
	m = press(t, m, keyEnter())
	if m.screen != screenPreflight {
		t.Fatalf("screen = %d, want to stay on preflight", m.screen)
	}
}

func TestWelcomeWaitsForPreflight(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m = press(t, m, keyEnter())
	if m.screen != screenPreflight {
		t.Fatalf("screen = %d, want preflight", m.screen)
	}
	m = press(t, m, keyEnter())
	if m.screen != screenPreflight {
		t.Fatalf("screen = %d, want to wait for the preflight result", m.screen)
	}
	m = withChecks(m, installer.Check{Name: "daemon", Status: installer.StatusOK})
	if !m.preflightDone {
		t.Fatal("preflightDone = false after a result")
	}
	m = press(t, m, keyEnter())
	if m.screen != screenPaths {
		t.Fatalf("screen = %d, want paths after preflight", m.screen)
	}
}

func TestWelcomeEnterRoutesToExistingConfig(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.configExists = true
	m.preflightDone = true
	m = press(t, m, keyEnter())
	if m.screen != screenExisting {
		t.Fatalf("screen = %d, want existing configuration choice", m.screen)
	}
}

func TestExistingConfigKeepRepairsService(t *testing.T) {
	options := testOptions(t)
	dictionaryPath := filepath.Join(t.TempDir(), "dictionary.sqlite")
	createDictionary(t, dictionaryPath)

	existing := config.Default()
	existing.Dictionary.DatabasePath = dictionaryPath
	if err := config.Write(options.ConfigPath, existing); err != nil {
		t.Fatalf("write config: %v", err)
	}

	m := newModel(context.Background(), options)
	m.configExists = true
	m.screen = screenExisting
	m.existingChoice = 0
	m = press(t, m, keyEnter())
	if m.screen != screenSummary {
		t.Fatalf("screen = %d, want summary (err %v)", m.screen, m.planErr)
	}
	if m.plan.WriteConfig {
		t.Fatal("keeping the existing configuration must not rewrite it")
	}
}

func TestExistingConfigReplaceContinuesToPaths(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.configExists = true
	m.screen = screenExisting
	m.existingChoice = 1
	m = press(t, m, keyEnter())
	if m.screen != screenPaths {
		t.Fatalf("screen = %d, want paths", m.screen)
	}
	if !m.settings.ReplaceConfig {
		t.Fatal("replacing the configuration must set ReplaceConfig")
	}
}

func TestExistingConfigCancelQuits(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.screen = screenExisting
	m.existingChoice = 2
	_, cmd := m.Update(keyEnter())
	if cmd == nil {
		t.Fatal("cancel must quit the program")
	}
}

func TestDictionaryScreenRejectsInvalidPath(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.screen = screenDictionary
	m.dictionaryInput.SetValue(filepath.Join(t.TempDir(), "missing.sqlite"))
	m = press(t, m, keyEnter())
	if m.screen != screenDictionary {
		t.Fatalf("screen = %d, want to stay on the dictionary screen", m.screen)
	}
	if m.dictionaryErr == "" {
		t.Fatal("expected a validation error")
	}
}

func TestDictionaryScreenAcceptsValidPath(t *testing.T) {
	options := testOptions(t)
	path := filepath.Join(t.TempDir(), "dictionary.sqlite")
	createDictionary(t, path)

	m := newModel(context.Background(), options)
	m.screen = screenDictionary
	m.dictionaryInput.SetValue(path)
	m = press(t, m, keyEnter())
	if m.screen != screenLanguages {
		t.Fatalf("screen = %d, want languages (err %q)", m.screen, m.dictionaryErr)
	}
	if m.settings.Dictionary != path {
		t.Fatalf("dictionary = %q, want %s", m.settings.Dictionary, path)
	}
}

func TestDictionaryScreenAllowsEmpty(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.screen = screenDictionary
	m.dictionaryInput.SetValue("")
	m = press(t, m, keyEnter())
	if m.screen != screenLanguages {
		t.Fatalf("screen = %d, want languages", m.screen)
	}
	if m.settings.Dictionary != "" {
		t.Fatalf("dictionary = %q, want empty", m.settings.Dictionary)
	}
}

func TestLanguageAppearancePacksAndServiceFlow(t *testing.T) {
	options := testOptions(t)
	m := newModel(context.Background(), options)
	m = withChecks(m,
		installer.Check{Name: "daemon", Status: installer.StatusOK},
		installer.Check{Name: "systemd", Status: installer.StatusOK},
	)
	m.screen = screenLanguages
	m.focus = 0
	m.sourceInput.SetValue("en")
	m.targetInput.SetValue("de")

	m = press(t, m, keyEnter()) // move to the target language field
	if m.focus != 1 {
		t.Fatalf("focus = %d, want the target field", m.focus)
	}
	m = press(t, m, keyEnter()) // continue to appearance
	if m.screen != screenAppearance {
		t.Fatalf("screen = %d, want appearance", m.screen)
	}
	if m.settings.SourceLang != "en" || m.settings.TargetLang != "de" {
		t.Fatalf("languages = %q/%q, want en/de", m.settings.SourceLang, m.settings.TargetLang)
	}

	m = press(t, m, keyEnter()) // appearance defaults are valid; continue to packs
	if m.screen != screenPacks {
		t.Fatalf("screen = %d, want packs", m.screen)
	}
	updated, _ := m.Update(packCatalogMsg{entries: []translation.CatalogEntry{
		{ID: "bergamot-en-de-tiny11", Source: "en", Target: "de"},
	}})
	m = updated.(model)
	m = press(t, m, keySpace(), keyEnter())
	if m.screen != screenService {
		t.Fatalf("screen = %d, want service", m.screen)
	}
	if !m.settings.Translation {
		t.Fatal("selecting a pack must enable translation")
	}
	if len(m.settings.PackIDs) != 1 || m.settings.PackIDs[0] != "bergamot-en-de-tiny11" {
		t.Fatalf("pack ids = %q", m.settings.PackIDs)
	}

	m = press(t, m, keySpace())
	if !m.settings.InstallService {
		t.Fatal("space must enable the service when it is available")
	}
	m = press(t, m, keyEnter())
	if m.screen != screenSummary {
		t.Fatalf("screen = %d, want summary (err %v)", m.screen, m.planErr)
	}
}

func TestServiceUnavailableCannotBeEnabled(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m = withChecks(m,
		installer.Check{Name: "daemon", Status: installer.StatusOK},
		installer.Check{Name: "systemd", Status: installer.StatusWarn, Detail: "missing"},
	)
	m.screen = screenService
	m = press(t, m, keySpace())
	if m.settings.InstallService {
		t.Fatal("the service must stay disabled when systemd is unavailable")
	}
}

func TestAppearanceViewShowsTheme(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.screen = screenAppearance
	view := m.View()
	if !strings.Contains(view, "Appearance") || !strings.Contains(view, config.PopupThemes[0]) {
		t.Fatalf("appearance view does not show the theme:\n%s", view)
	}
}

func TestSummaryDryRunFinishes(t *testing.T) {
	options := testOptions(t)
	options.DryRun = true
	m := newModel(context.Background(), options)
	m.screen = screenSummary
	m.plan = installer.Plan{ConfigPath: options.ConfigPath, Config: config.Default()}
	m = press(t, m, keyEnter())
	if m.screen != screenDone {
		t.Fatalf("screen = %d, want done", m.screen)
	}
}

func TestSummaryApplyStartsProgress(t *testing.T) {
	options := testOptions(t)
	m := newModel(context.Background(), options)
	m.screen = screenSummary
	cfg := config.Default()
	cfg.Dictionary.DatabasePath = filepath.Join(t.TempDir(), "dictionary.sqlite")
	m.plan = installer.Plan{ConfigPath: options.ConfigPath, Config: cfg, WriteConfig: true}

	updated, cmd := m.Update(keyEnter())
	m = updated.(model)
	if m.screen != screenProgress {
		t.Fatalf("screen = %d, want progress", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected an apply command")
	}
	if m.progress.Total != 1 {
		t.Fatalf("progress total = %d, want 1", m.progress.Total)
	}
}

func TestSummaryBlocksFailedChecks(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.screen = screenSummary
	m.plan = installer.Plan{
		ConfigPath: m.settings.ConfigPath,
		Config:     config.Default(),
		Checks:     []installer.Check{{Name: "dictionary", Status: installer.StatusFail, Detail: "broken"}},
	}
	m = press(t, m, keyEnter())
	if m.screen != screenError {
		t.Fatalf("screen = %d, want error", m.screen)
	}
	if m.applyErr == nil || !strings.Contains(m.applyErr.Error(), "broken") {
		t.Fatalf("apply error = %v, want the failed check", m.applyErr)
	}
}

func TestApplyFailureShowsErrorScreen(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.screen = screenProgress
	updated, _ := m.Update(applyDoneMsg{err: errors.New("copy failed")})
	m = updated.(model)
	if m.screen != screenError {
		t.Fatalf("screen = %d, want error", m.screen)
	}
	if m.applyErr == nil || !strings.Contains(m.applyErr.Error(), "copy failed") {
		t.Fatalf("apply error = %v", m.applyErr)
	}
}

func TestCtrlCIgnoredDuringProgress(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.screen = screenProgress
	updated, cmd := m.Update(keyCtrlC())
	if updated.(model).screen != screenProgress {
		t.Fatal("ctrl+c must not leave the progress screen")
	}
	if cmd != nil {
		t.Fatal("ctrl+c must not quit during progress")
	}
}

func TestProgressMessageUpdatesStep(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.screen = screenProgress
	m.progressCh = make(chan installer.Progress, 1)
	updated, cmd := m.Update(progressMsg{Step: "Installing dictionary", Index: 2, Total: 3})
	m = updated.(model)
	if m.progress.Step != "Installing dictionary" || m.progress.Index != 2 || m.progress.Total != 3 {
		t.Fatalf("progress = %+v", m.progress)
	}
	if cmd == nil {
		t.Fatal("expected to keep waiting for progress")
	}
}

func TestViewsRenderEveryScreen(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.plan = installer.Plan{
		ConfigPath: m.settings.ConfigPath,
		Config:     config.Default(),
		Checks:     []installer.Check{{Name: "dictionary", Status: installer.StatusFail, Detail: "broken"}},
	}
	for _, screen := range []screen{
		screenWelcome, screenPreflight, screenExisting, screenPaths, screenDictionary,
		screenLanguages, screenAppearance, screenPacks, screenService, screenSummary,
		screenProgress, screenDone, screenError,
	} {
		m.screen = screen
		if view := m.View(); strings.TrimSpace(view) == "" {
			t.Fatalf("screen %d rendered an empty view", screen)
		}
	}
}

func TestEscapeNavigation(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m = withChecks(m, installer.Check{Name: "daemon", Status: installer.StatusOK})

	m = press(t, m, keyEsc())
	if m.screen != screenWelcome {
		t.Fatalf("screen = %d, want welcome", m.screen)
	}

	m.screen = screenDictionary
	m = press(t, m, keyEsc())
	if m.screen != screenPaths {
		t.Fatalf("screen = %d, want paths", m.screen)
	}

	m.screen = screenSummary
	m = press(t, m, keyEsc())
	if m.screen != screenService {
		t.Fatalf("screen = %d, want service", m.screen)
	}

	m.screen = screenError
	m.planErr = nil
	m = press(t, m, keyEsc())
	if m.screen != screenSummary {
		t.Fatalf("screen = %d, want summary", m.screen)
	}
}

func TestExistingChoiceNavigationClamps(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.screen = screenExisting
	m = press(t, m, keyUp(), keyUp())
	if m.existingChoice != 0 {
		t.Fatalf("choice = %d, want 0", m.existingChoice)
	}
	m = press(t, m, keyDown(), keyDown(), keyDown(), keyDown())
	if m.existingChoice != 2 {
		t.Fatalf("choice = %d, want 2", m.existingChoice)
	}
}

func TestDictionaryPathExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "dictionary.sqlite")
	createDictionary(t, path)

	m := newModel(context.Background(), testOptions(t))
	m.screen = screenDictionary
	m.dictionaryInput.SetValue("~/dictionary.sqlite")
	m = press(t, m, keyEnter())
	if m.screen != screenLanguages {
		t.Fatalf("screen = %d, want languages (err %q)", m.screen, m.dictionaryErr)
	}
	if m.settings.Dictionary != path {
		t.Fatalf("dictionary = %q, want %s", m.settings.Dictionary, path)
	}
}

func TestDoneViewMentionsNextSteps(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	m.screen = screenDone
	m.plan = installer.Plan{ConfigPath: "/tmp/config.toml", InstallUnit: true}
	view := m.View()
	for _, want := range []string{"Setup complete", "lex doctor", "systemctl --user status lexicon.service"} {
		if !strings.Contains(view, want) {
			t.Fatalf("done view does not mention %q:\n%s", want, view)
		}
	}
}

func TestDryRunDoneView(t *testing.T) {
	options := testOptions(t)
	options.DryRun = true
	m := newModel(context.Background(), options)
	m.screen = screenDone
	if !strings.Contains(m.View(), "Dry run complete") {
		t.Fatalf("dry-run view = %q", m.View())
	}
}

func TestQuitKeys(t *testing.T) {
	m := newModel(context.Background(), testOptions(t))
	for _, key := range []string{"q", "esc"} {
		var msg tea.KeyMsg
		if key == "q" {
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
		} else {
			msg = keyEsc()
		}
		_, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatalf("%q must quit from the welcome screen", key)
		}
	}
}

func TestConfigPathIsKeptWhenInputIsEmpty(t *testing.T) {
	options := testOptions(t)
	m := newModel(context.Background(), options)
	m.screen = screenPaths
	m.configInput.SetValue("")
	m = press(t, m, keyEnter())
	if m.settings.ConfigPath != options.ConfigPath {
		t.Fatalf("config = %q, want %s", m.settings.ConfigPath, options.ConfigPath)
	}
}

func TestDownloadURLSkipsDictionaryScreen(t *testing.T) {
	options := testOptions(t)
	options.DictionaryURL = "https://example.invalid/en.sqlite"
	options.DictionarySHA256 = strings.Repeat("0", 64)
	m := newModel(context.Background(), options)
	m.screen = screenPaths
	m = press(t, m, keyEnter())
	if m.screen != screenLanguages {
		t.Fatalf("screen = %d, want languages", m.screen)
	}
	m = press(t, m, keyEsc())
	if m.screen != screenPaths {
		t.Fatalf("screen = %d, want paths", m.screen)
	}
}

func TestScreenName(t *testing.T) {
	tests := map[screen]string{
		screenWelcome:   "welcome",
		screenPreflight: "preflight",
		screenDone:      "done",
	}
	for value, want := range tests {
		if got := screenName(value); got != want {
			t.Fatalf("screenName(%d) = %q, want %q", value, got, want)
		}
	}
}
