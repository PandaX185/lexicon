package installer

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/systemd"
	_ "modernc.org/sqlite"
)

func createTestDictionary(t *testing.T, path string) {
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

func writeTestManifest(t *testing.T, dir, version string) dictionary.Manifest {
	t.Helper()
	path := filepath.Join(dir, "en.sqlite")
	hash, size, err := dictionary.HashFile(path)
	if err != nil {
		t.Fatalf("hash fixture: %v", err)
	}
	manifest := dictionary.Manifest{
		Name:           "Test Dictionary",
		Version:        version,
		File:           "en.sqlite",
		Bytes:          size,
		SHA256:         hash,
		SourceLanguage: "en",
		TargetLanguage: "en",
	}
	if err := dictionary.WriteManifest(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return manifest
}

func writeTempFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "payload")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// isolate points every XDG directory at temporary paths so plan resolution
// never touches the real user profile.
func isolate(t *testing.T) (dataHome, configHome string) {
	t.Helper()
	dataHome = t.TempDir()
	configHome = t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	return dataHome, configHome
}

func TestFindPackagedDictionary(t *testing.T) {
	t.Run("database without manifest", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "en.sqlite")
		createTestDictionary(t, path)

		got, err := findPackagedDictionary([]string{dir})
		if err != nil {
			t.Fatalf("find packaged dictionary: %v", err)
		}
		if got == nil || got.path != path || got.manifest != nil {
			t.Fatalf("packaged = %+v, want path %s without manifest", got, path)
		}
	})

	t.Run("verified manifest", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "en.sqlite")
		createTestDictionary(t, path)
		writeTestManifest(t, dir, "1.2.3")

		got, err := findPackagedDictionary([]string{dir})
		if err != nil {
			t.Fatalf("find packaged dictionary: %v", err)
		}
		if got == nil || got.manifest == nil || got.manifest.Version != "1.2.3" {
			t.Fatalf("packaged = %+v, want a verified 1.2.3 manifest", got)
		}
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "en.sqlite")
		createTestDictionary(t, path)
		manifest := writeTestManifest(t, dir, "1.2.3")
		manifest.SHA256 = strings.Repeat("0", 64)
		if err := dictionary.WriteManifest(filepath.Join(dir, "manifest.json"), manifest); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		if _, err := findPackagedDictionary([]string{dir}); err == nil || !strings.Contains(err.Error(), "checksum") {
			t.Fatalf("error = %v, want a checksum mismatch", err)
		}
	})

	t.Run("priority", func(t *testing.T) {
		empty := t.TempDir()
		dir := t.TempDir()
		path := filepath.Join(dir, "en.sqlite")
		createTestDictionary(t, path)

		got, err := findPackagedDictionary([]string{empty, dir})
		if err != nil {
			t.Fatalf("find packaged dictionary: %v", err)
		}
		if got == nil || got.path != path {
			t.Fatalf("packaged = %+v, want %s", got, path)
		}
	})

	t.Run("none", func(t *testing.T) {
		got, err := findPackagedDictionary([]string{t.TempDir()})
		if err != nil {
			t.Fatalf("find packaged dictionary: %v", err)
		}
		if got != nil {
			t.Fatalf("packaged = %+v, want nil", got)
		}
	})
}

func TestBuildPlanUsesPackagedDictionary(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "en.sqlite")
	createTestDictionary(t, path)
	writeTestManifest(t, dir, "1.0")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	plan, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{dir},
	}, Settings{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if !plan.WriteConfig {
		t.Fatal("expected a new configuration to be written")
	}
	if !plan.DictionaryPackaged || plan.CopyDictionary {
		t.Fatalf("packaged=%t copy=%t, want a packaged dictionary used in place", plan.DictionaryPackaged, plan.CopyDictionary)
	}
	if plan.DictionaryTarget != path || plan.DictionarySource != path {
		t.Fatalf("source=%q target=%q, want %s", plan.DictionarySource, plan.DictionaryTarget, path)
	}
	if plan.DictionaryVersion != "1.0" {
		t.Fatalf("version = %q, want 1.0", plan.DictionaryVersion)
	}
	if plan.Config.Dictionary.DatabasePath != path {
		t.Fatalf("config dictionary = %q, want %s", plan.Config.Dictionary.DatabasePath, path)
	}
	if failed := plan.FailedChecks(); len(failed) > 0 {
		t.Fatalf("unexpected failed checks: %+v", failed)
	}
}

func TestBuildPlanExplicitPackagedPathUsesItInPlace(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "en.sqlite")
	createTestDictionary(t, path)
	writeTestManifest(t, dir, "1.0")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	plan, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{dir},
	}, Settings{ConfigPath: configPath, Dictionary: path})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if !plan.DictionaryPackaged || plan.CopyDictionary {
		t.Fatalf("packaged=%t copy=%t, want the explicit packaged path used in place", plan.DictionaryPackaged, plan.CopyDictionary)
	}
	if plan.DictionaryTarget != path {
		t.Fatalf("target = %q, want %s", plan.DictionaryTarget, path)
	}
}

func TestBuildPlanExplicitDictionarySurvivesBrokenPackage(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	packagedPath := filepath.Join(dir, "en.sqlite")
	createTestDictionary(t, packagedPath)
	manifest := writeTestManifest(t, dir, "1.0")
	manifest.SHA256 = strings.Repeat("0", 64)
	if err := dictionary.WriteManifest(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)
	configPath := filepath.Join(t.TempDir(), "config.toml")

	plan, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{dir},
	}, Settings{ConfigPath: configPath, Dictionary: source})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if failed := plan.FailedChecks(); len(failed) > 0 {
		t.Fatalf("unexpected failed checks: %+v", failed)
	}
	if !plan.CopyDictionary || plan.DictionarySource != source {
		t.Fatalf("source=%q copy=%t, want the explicit local dictionary", plan.DictionarySource, plan.CopyDictionary)
	}
}

func TestBuildPlanCopiesLocalDictionary(t *testing.T) {
	isolate(t)
	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	plan, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{t.TempDir()},
	}, Settings{ConfigPath: configPath, Dictionary: source})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if !plan.CopyDictionary {
		t.Fatal("expected the local dictionary to be copied")
	}
	if plan.DictionarySource != source {
		t.Fatalf("source = %q, want %s", plan.DictionarySource, source)
	}
	target := filepath.Join(os.Getenv("XDG_DATA_HOME"), "lexicon", "dictionary.sqlite")
	if plan.DictionaryTarget != target {
		t.Fatalf("target = %q, want %s", plan.DictionaryTarget, target)
	}
	if plan.Config.Dictionary.DatabasePath != target {
		t.Fatalf("config dictionary = %q, want %s", plan.Config.Dictionary.DatabasePath, target)
	}
}

func TestBuildPlanKeepsExistingDictionary(t *testing.T) {
	dataHome, _ := isolate(t)
	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)

	target := filepath.Join(dataHome, "lexicon", "dictionary.sqlite")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatalf("create data directory: %v", err)
	}
	createTestDictionary(t, target)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	plan, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{t.TempDir()},
	}, Settings{ConfigPath: configPath, Dictionary: source})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.CopyDictionary {
		t.Fatal("existing dictionary must not be overwritten without --force")
	}
	if !containsNote(plan.Notes, "keeping existing dictionary") {
		t.Fatalf("notes = %v, want a keep note", plan.Notes)
	}

	forced, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{t.TempDir()},
		Force:                true,
	}, Settings{ConfigPath: configPath, Dictionary: source})
	if err != nil {
		t.Fatalf("build forced plan: %v", err)
	}
	if !forced.CopyDictionary {
		t.Fatal("--force must replace the existing dictionary")
	}
}

func TestBuildPlanChecksumMismatch(t *testing.T) {
	isolate(t)
	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	_, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySHA256:     strings.Repeat("0", 64),
		DictionarySearchDirs: []string{t.TempDir()},
	}, Settings{ConfigPath: configPath, Dictionary: source})
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("error = %v, want a checksum mismatch", err)
	}
}

func TestBuildPlanRejectsConflictingFlags(t *testing.T) {
	isolate(t)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	base := Options{
		ConfigPath:           configPath,
		DictionaryURL:        "https://example.invalid/en.sqlite",
		DictionarySHA256:     strings.Repeat("0", 64),
		DictionarySearchDirs: []string{t.TempDir()},
	}

	if _, err := BuildPlan(context.Background(), base,
		Settings{ConfigPath: configPath, Dictionary: "local.sqlite"}); err == nil {
		t.Fatal("expected --dictionary and --dictionary-url to conflict")
	}

	dryRun := base
	dryRun.DryRun = true
	if _, err := BuildPlan(context.Background(), dryRun,
		Settings{ConfigPath: configPath}); err == nil {
		t.Fatal("expected --dry-run with --dictionary-url to be rejected")
	}

	missingHash := base
	missingHash.DictionarySHA256 = ""
	if _, err := BuildPlan(context.Background(), missingHash,
		Settings{ConfigPath: configPath}); err == nil {
		t.Fatal("expected a missing checksum to be rejected")
	}
}

func TestCheckEnvironmentValidatesExplicitDictionary(t *testing.T) {
	isolate(t)
	dictionaryPath := filepath.Join(t.TempDir(), "dictionary.sqlite")
	createTestDictionary(t, dictionaryPath)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	checks := CheckEnvironment(context.Background(), Options{
		ConfigPath:           configPath,
		Dictionary:           dictionaryPath,
		DictionarySearchDirs: []string{t.TempDir()},
	})
	check := findCheck(t, checks, "dictionary")
	if check.Status != StatusOK || check.Detail != dictionaryPath {
		t.Fatalf("dictionary check = %+v, want ok at %s", check, dictionaryPath)
	}

	broken := filepath.Join(t.TempDir(), "broken.sqlite")
	if err := os.WriteFile(broken, []byte("not a database"), 0o600); err != nil {
		t.Fatalf("write broken dictionary: %v", err)
	}
	checks = CheckEnvironment(context.Background(), Options{
		ConfigPath:           configPath,
		Dictionary:           broken,
		DictionarySearchDirs: []string{t.TempDir()},
	})
	if check := findCheck(t, checks, "dictionary"); check.Status != StatusFail {
		t.Fatalf("dictionary check = %+v, want a failure", check)
	}

	// An existing configuration is kept without --force, so an explicit path
	// that will not be used must not block preflight.
	existing := config.Default()
	existing.Dictionary.DatabasePath = dictionaryPath
	if err := config.Write(configPath, existing); err != nil {
		t.Fatalf("write config: %v", err)
	}
	checks = CheckEnvironment(context.Background(), Options{
		ConfigPath:           configPath,
		Dictionary:           broken,
		DictionarySearchDirs: []string{t.TempDir()},
	})
	if check := findCheck(t, checks, "dictionary"); check.Status == StatusFail {
		t.Fatalf("dictionary check = %+v, want the unused path ignored", check)
	}
}

func TestBuildPlanReplacesEnvironmentDictionaryCheck(t *testing.T) {
	isolate(t)
	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)
	configPath := filepath.Join(t.TempDir(), "config.toml")

	plan, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{t.TempDir()},
	}, Settings{ConfigPath: configPath, Dictionary: source})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	var dictionaryChecks int
	for _, check := range plan.Checks {
		if check.Name == "dictionary" {
			dictionaryChecks++
			if check.Status != StatusOK {
				t.Fatalf("dictionary check = %+v, want ok", check)
			}
		}
	}
	if dictionaryChecks != 1 {
		t.Fatalf("dictionary checks = %d, want exactly one", dictionaryChecks)
	}
}

func TestBuildPlanSetsModelDirectory(t *testing.T) {
	isolate(t)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	modelDir := filepath.Join(t.TempDir(), "models")

	plan, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{t.TempDir()},
	}, Settings{ConfigPath: configPath, Translation: true, ModelDir: modelDir})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if !plan.Config.Translation.Enabled {
		t.Fatal("translation = false, want true")
	}
	if plan.Config.Translation.ModelDirectory != modelDir {
		t.Fatalf("model directory = %q, want %s", plan.Config.Translation.ModelDirectory, modelDir)
	}

	relative, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{t.TempDir()},
	}, Settings{ConfigPath: configPath, ModelDir: "~/lexicon-models"})
	if err != nil {
		t.Fatalf("build plan with home path: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home directory: %v", err)
	}
	if want := filepath.Join(home, "lexicon-models"); relative.Config.Translation.ModelDirectory != want {
		t.Fatalf("model directory = %q, want %s", relative.Config.Translation.ModelDirectory, want)
	}
}

func TestBuildPlanKeepsExistingConfig(t *testing.T) {
	isolate(t)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	dictionaryPath := filepath.Join(t.TempDir(), "existing.sqlite")
	createTestDictionary(t, dictionaryPath)

	existing := config.Default()
	existing.Dictionary.DatabasePath = dictionaryPath
	existing.Dictionary.TargetLanguage = "de"
	existing.Popup.Width = 500
	if err := config.Write(configPath, existing); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	plan, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{t.TempDir()},
	}, Settings{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.WriteConfig {
		t.Fatal("existing configuration must be kept without --force")
	}
	if !plan.ConfigExists {
		t.Fatal("ConfigExists = false, want true")
	}
	if plan.Config.Dictionary.TargetLanguage != "de" || plan.Config.Popup.Width != 500 {
		t.Fatalf("config = %+v, want the existing values", plan.Config)
	}
	if plan.DictionaryTarget != dictionaryPath {
		t.Fatalf("dictionary target = %q, want %s", plan.DictionaryTarget, dictionaryPath)
	}
	if !containsNote(plan.Notes, "existing configuration kept") {
		t.Fatalf("notes = %v, want a keep note", plan.Notes)
	}

	replace, err := BuildPlan(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{t.TempDir()},
		Force:                true,
	}, Settings{ConfigPath: configPath, SourceLang: "en", TargetLang: "en"})
	if err != nil {
		t.Fatalf("build forced plan: %v", err)
	}
	if !replace.WriteConfig {
		t.Fatal("--force must replace the existing configuration")
	}
	if replace.Config.Popup.Width != config.Default().Popup.Width {
		t.Fatalf("popup width = %d, want the default", replace.Config.Popup.Width)
	}
}

func TestBuildPlanRepairsMissingUnit(t *testing.T) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl is not available")
	}
	_, configHome := isolate(t)
	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)
	daemon := filepath.Join(t.TempDir(), "lexicon-daemon")
	if err := os.WriteFile(daemon, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write daemon stub: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "config.toml")
	options := Options{
		ConfigPath:           configPath,
		DaemonBinary:         daemon,
		DictionarySearchDirs: []string{t.TempDir()},
	}
	settings := Settings{ConfigPath: configPath, Dictionary: source, InstallService: true}

	plan, err := BuildPlan(context.Background(), options, settings)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if !plan.InstallUnit || !plan.EnableUnit {
		t.Fatalf("install=%t enable=%t, want both", plan.InstallUnit, plan.EnableUnit)
	}
	unitPath, err := systemd.UnitPath()
	if err != nil {
		t.Fatalf("unit path: %v", err)
	}
	if plan.UnitPath != unitPath || !strings.HasPrefix(plan.UnitPath, configHome) {
		t.Fatalf("unit path = %q, want %q", plan.UnitPath, unitPath)
	}

	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		t.Fatalf("create unit directory: %v", err)
	}
	if err := os.WriteFile(unitPath, []byte("[Unit]\nDescription=old\n"), 0o644); err != nil {
		t.Fatalf("write existing unit: %v", err)
	}

	kept, err := BuildPlan(context.Background(), options, settings)
	if err != nil {
		t.Fatalf("build plan with unit: %v", err)
	}
	if kept.InstallUnit || !kept.EnableUnit {
		t.Fatalf("install=%t enable=%t, want the existing unit started and kept", kept.InstallUnit, kept.EnableUnit)
	}

	options.Force = true
	replaced, err := BuildPlan(context.Background(), options, settings)
	if err != nil {
		t.Fatalf("build forced plan: %v", err)
	}
	if !replaced.InstallUnit || !replaced.EnableUnit {
		t.Fatalf("install=%t enable=%t, want --force to rewrite the unit", replaced.InstallUnit, replaced.EnableUnit)
	}
}

func TestBuildPlanStepsAndFailures(t *testing.T) {
	plan := Plan{WriteConfig: true, CopyDictionary: true, InstallUnit: true, EnableUnit: true}
	steps := plan.Steps()
	want := []string{"Writing configuration", "Installing dictionary", "Installing systemd unit", "Starting service"}
	if len(steps) != len(want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
	for i := range want {
		if steps[i] != want[i] {
			t.Fatalf("steps = %v, want %v", steps, want)
		}
	}

	plan.Checks = []Check{
		{Name: "wayland", Status: StatusWarn},
		{Name: "dictionary", Status: StatusFail, Detail: "missing"},
	}
	failed := plan.FailedChecks()
	if len(failed) != 1 || failed[0].Name != "dictionary" {
		t.Fatalf("failed = %+v, want only the dictionary failure", failed)
	}
}

func TestApplyWritesConfigAndDictionary(t *testing.T) {
	isolate(t)
	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)
	target := filepath.Join(t.TempDir(), "dictionary.sqlite")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	cfg := config.Default()
	cfg.Dictionary.DatabasePath = target
	plan := Plan{
		ConfigPath:       configPath,
		Config:           cfg,
		WriteConfig:      true,
		DictionarySource: source,
		DictionaryTarget: target,
		CopyDictionary:   true,
	}

	var progress []Progress
	if err := Apply(context.Background(), plan, func(p Progress) { progress = append(progress, p) }); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(progress) != 2 {
		t.Fatalf("progress = %+v, want two steps", progress)
	}
	if progress[0].Step != "Writing configuration" || progress[1].Step != "Installing dictionary" {
		t.Fatalf("progress = %+v, want configuration then dictionary", progress)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("installed dictionary: %v", err)
	}
	loaded, exists, err := config.Load(configPath)
	if err != nil || !exists {
		t.Fatalf("load config: exists=%t err=%v", exists, err)
	}
	if loaded.Dictionary.DatabasePath != target {
		t.Fatalf("config dictionary = %q, want %s", loaded.Dictionary.DatabasePath, target)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".lexicon-backup-*")); len(leftovers) > 0 {
		t.Fatalf("backups were not cleaned up: %v", leftovers)
	}
}

func TestApplyRollbackRestoresConfig(t *testing.T) {
	isolate(t)
	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)
	configPath := filepath.Join(t.TempDir(), "config.toml")

	original := []byte("# original configuration\n")
	if err := os.WriteFile(configPath, original, 0o600); err != nil {
		t.Fatalf("write original config: %v", err)
	}

	// A directory at the dictionary target makes backupFile fail after the
	// configuration has already been written.
	blockedTarget := filepath.Join(t.TempDir(), "dictionary.sqlite")
	if err := os.Mkdir(blockedTarget, 0o700); err != nil {
		t.Fatalf("create blocking directory: %v", err)
	}

	cfg := config.Default()
	cfg.Dictionary.DatabasePath = blockedTarget
	plan := Plan{
		ConfigPath:       configPath,
		Config:           cfg,
		WriteConfig:      true,
		DictionarySource: source,
		DictionaryTarget: blockedTarget,
		CopyDictionary:   true,
	}

	err := Apply(context.Background(), plan, nil)
	if err == nil {
		t.Fatal("expected the dictionary step to fail")
	}
	if !strings.Contains(err.Error(), "back up dictionary") {
		t.Fatalf("error = %v, want a dictionary backup failure", err)
	}
	restored, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read restored config: %v", readErr)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("config = %q, want the original %q", restored, original)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(configPath), ".lexicon-backup-*")); len(leftovers) > 0 {
		t.Fatalf("backups were not cleaned up: %v", leftovers)
	}
}

func TestApplyRemovesNewConfigOnFailure(t *testing.T) {
	isolate(t)
	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)
	configPath := filepath.Join(t.TempDir(), "config.toml")

	blockedTarget := filepath.Join(t.TempDir(), "dictionary.sqlite")
	if err := os.Mkdir(blockedTarget, 0o700); err != nil {
		t.Fatalf("create blocking directory: %v", err)
	}

	cfg := config.Default()
	cfg.Dictionary.DatabasePath = blockedTarget
	plan := Plan{
		ConfigPath:       configPath,
		Config:           cfg,
		WriteConfig:      true,
		DictionarySource: source,
		DictionaryTarget: blockedTarget,
		CopyDictionary:   true,
	}

	if err := Apply(context.Background(), plan, nil); err == nil {
		t.Fatal("expected the dictionary step to fail")
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("rollback left a new configuration at %s", configPath)
	}
}

func TestDownloadDictionary(t *testing.T) {
	payload := []byte("dictionary payload")
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	hash, _, err := dictionary.HashFile(writeTempFile(t, payload))
	if err != nil {
		t.Fatalf("hash payload: %v", err)
	}

	path, err := downloadDictionary(context.Background(), server.URL+"/en.sqlite", hash, server.Client())
	if err != nil {
		t.Fatalf("download dictionary: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("download = %q, want %q", got, payload)
	}
}

func TestDownloadDictionaryRejects(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("payload"))
	}))
	defer server.Close()

	if _, err := downloadDictionary(context.Background(), server.URL+"/en.sqlite", "", server.Client()); err == nil {
		t.Fatal("expected a missing checksum error")
	}

	if _, err := downloadDictionary(context.Background(), server.URL+"/en.sqlite",
		strings.Repeat("0", 64), server.Client()); err == nil {
		t.Fatal("expected a checksum mismatch")
	}

	if _, err := downloadDictionary(context.Background(), "http://example.invalid/en.sqlite",
		strings.Repeat("0", 64), server.Client()); err == nil {
		t.Fatal("expected a non-HTTPS URL to be rejected")
	}

	notFound := httptest.NewTLSServer(http.NotFoundHandler())
	defer notFound.Close()
	if _, err := downloadDictionary(context.Background(), notFound.URL+"/en.sqlite",
		strings.Repeat("0", 64), notFound.Client()); err == nil {
		t.Fatal("expected a non-200 response to fail")
	}
}

func TestValidateDictionary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "valid.sqlite")
	createTestDictionary(t, path)
	if err := validateDictionary(context.Background(), path); err != nil {
		t.Fatalf("validate dictionary: %v", err)
	}

	empty := filepath.Join(t.TempDir(), "empty.sqlite")
	db, err := sql.Open("sqlite", empty)
	if err != nil {
		t.Fatalf("open empty database: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE dictionary_info (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		name TEXT NOT NULL,
		source_language TEXT NOT NULL,
		target_language TEXT NOT NULL,
		license TEXT NOT NULL DEFAULT '',
		attribution TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create metadata: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO dictionary_info VALUES (1, 'Empty', 'en', 'en', '', '')`); err != nil {
		t.Fatalf("insert metadata: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close empty database: %v", err)
	}
	if err := validateDictionary(context.Background(), empty); err == nil {
		t.Fatal("expected a missing entries table error")
	}
}

func TestRunUsesPackagedDictionaryInDryRun(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "en.sqlite")
	createTestDictionary(t, path)
	writeTestManifest(t, dir, "1.0")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	var out bytes.Buffer
	err := Run(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{dir},
		SkipService:          true,
		Yes:                  true,
		DryRun:               true,
		Stdin:                strings.NewReader(""),
		Stdout:               &out,
		Stderr:               io.Discard,
	})
	if err != nil {
		t.Fatalf("run installer: %v", err)
	}
	if !strings.Contains(out.String(), path+" (packaged 1.0, no copy)") {
		t.Fatalf("output does not report the packaged dictionary:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Dry run") {
		t.Fatalf("output does not mention the dry run:\n%s", out.String())
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote %s", configPath)
	}
}

func TestRunCopiesLocalDictionary(t *testing.T) {
	dataHome, _ := isolate(t)
	source := filepath.Join(t.TempDir(), "local.sqlite")
	createTestDictionary(t, source)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	var out bytes.Buffer
	err := Run(context.Background(), Options{
		ConfigPath:           configPath,
		Dictionary:           source,
		DictionarySearchDirs: []string{t.TempDir()},
		SkipService:          true,
		Yes:                  true,
		Stdin:                strings.NewReader(""),
		Stdout:               &out,
		Stderr:               io.Discard,
	})
	if err != nil {
		t.Fatalf("run installer: %v", err)
	}

	target := filepath.Join(dataHome, "lexicon", "dictionary.sqlite")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("installed dictionary: %v", err)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(config), target) {
		t.Fatalf("config does not point at %s:\n%s", target, config)
	}
}

func TestRunKeepsExistingConfig(t *testing.T) {
	isolate(t)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	dictionaryPath := filepath.Join(t.TempDir(), "existing.sqlite")
	createTestDictionary(t, dictionaryPath)

	existing := config.Default()
	existing.Dictionary.DatabasePath = dictionaryPath
	existing.Popup.Width = 500
	if err := config.Write(configPath, existing); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	var out bytes.Buffer
	err := Run(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{t.TempDir()},
		SkipService:          true,
		Yes:                  true,
		Stdin:                strings.NewReader(""),
		Stdout:               &out,
		Stderr:               io.Discard,
	})
	if err != nil {
		t.Fatalf("run installer: %v", err)
	}
	if !strings.Contains(out.String(), "(kept)") {
		t.Fatalf("output does not report the kept configuration:\n%s", out.String())
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(config), "width = 500") {
		t.Fatalf("existing configuration was overwritten:\n%s", config)
	}
}

func TestRunReportsFailedPreflight(t *testing.T) {
	isolate(t)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	dictionaryPath := filepath.Join(t.TempDir(), "broken.sqlite")
	if err := os.WriteFile(dictionaryPath, []byte("not a database"), 0o600); err != nil {
		t.Fatalf("write broken dictionary: %v", err)
	}

	existing := config.Default()
	existing.Dictionary.DatabasePath = dictionaryPath
	if err := config.Write(configPath, existing); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	var out bytes.Buffer
	err := Run(context.Background(), Options{
		ConfigPath:           configPath,
		DictionarySearchDirs: []string{t.TempDir()},
		SkipService:          true,
		Yes:                  true,
		Stdin:                strings.NewReader(""),
		Stdout:               &out,
		Stderr:               io.Discard,
	})
	if err == nil || !strings.Contains(err.Error(), "preflight failed") {
		t.Fatalf("error = %v, want a preflight failure", err)
	}
}

func findCheck(t *testing.T, checks []Check, name string) Check {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("no %q check in %+v", name, checks)
	return Check{}
}

func containsNote(notes []string, fragment string) bool {
	for _, note := range notes {
		if strings.Contains(note, fragment) {
			return true
		}
	}
	return false
}

func TestBuildPlanAppliesAppearanceAndPacks(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	no := false
	words := 6
	chars := 200
	options := Options{
		ConfigPath:  configPath,
		Force:       true,
		SkipService: true,
	}
	settings := Settings{
		ConfigPath:     configPath,
		ReplaceConfig:  true,
		InstallService: false,
		Appearance: AppearanceSettings{
			Theme:             "nord",
			ShowPronunciation: &no,
			ShowHeader:        &no,
			ShowStatus:        &no,
			ShowFooter:        &no,
			MaxQueryWords:     &words,
			MaxQueryChars:     &chars,
		},
		PackIDs: []string{"bergamot-en-de-tiny11"},
	}

	plan, err := BuildPlan(context.Background(), options, settings)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if plan.Config.Popup.Theme != "nord" {
		t.Fatalf("theme = %q", plan.Config.Popup.Theme)
	}
	if plan.Config.Dictionary.ShowPronunciation {
		t.Fatalf("boolean appearance settings were not applied: %+v", plan.Config)
	}
	if plan.Config.General.MaxQueryWords != 6 || plan.Config.General.MaxQueryChars != 200 {
		t.Fatalf("limits = %d/%d", plan.Config.General.MaxQueryWords, plan.Config.General.MaxQueryChars)
	}
	if !plan.Config.Translation.Enabled {
		t.Fatal("selected packs must enable translation")
	}
	if len(plan.PackIDs) != 1 || plan.PackIDs[0] != "bergamot-en-de-tiny11" {
		t.Fatalf("pack ids = %q", plan.PackIDs)
	}
}
