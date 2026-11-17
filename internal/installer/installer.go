// Package installer implements Lexicon's setup flow. Plan resolution and file
// operations are independent of the terminal UI in the tui subpackage so they
// can be tested without a terminal.
package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/paths"
	"github.com/PandaX185/lexicon/internal/systemd"
)

// Options carries command-line flags and process streams.
type Options struct {
	ConfigPath       string
	Dictionary       string
	DictionaryURL    string
	DictionarySHA256 string
	DaemonBinary     string
	ModelDir         string
	SourceLang       string
	TargetLang       string
	Translation      bool
	SkipService      bool
	Force            bool
	Yes              bool
	DryRun           bool

	// DictionarySearchDirs overrides the packaged dictionary locations. Tests
	// use it; the command line leaves it empty for paths.PackagedDictionaryDirs.
	DictionarySearchDirs []string
	// HTTPClient overrides the client used for --dictionary-url.
	HTTPClient *http.Client

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Settings are the choices collected by the TUI. Non-interactive runs derive
// them from Options.
type Settings struct {
	ConfigPath     string
	Dictionary     string
	ModelDir       string
	SourceLang     string
	TargetLang     string
	Translation    bool
	InstallService bool
	ReplaceConfig  bool
}

// Plan is the resolved set of actions, also useful in dry-run mode.
type Plan struct {
	ConfigPath   string
	Config       config.Config
	WriteConfig  bool
	ConfigExists bool

	DictionarySource   string
	DictionaryTarget   string
	DictionaryPackaged bool
	DictionaryVersion  string
	CopyDictionary     bool
	DownloadURL        string
	DownloadSHA256     string

	UnitPath    string
	Unit        string
	InstallUnit bool
	EnableUnit  bool

	Checks []Check
	Notes  []string
}

// FailedChecks returns the preflight failures that must block installation.
func (p Plan) FailedChecks() []Check {
	var failed []Check
	for _, check := range p.Checks {
		if check.Status == StatusFail {
			failed = append(failed, check)
		}
	}
	return failed
}

// Steps returns the apply steps in order, used for progress reporting.
func (p Plan) Steps() []string {
	var steps []string
	if p.WriteConfig {
		steps = append(steps, "Writing configuration")
	}
	if p.DownloadURL != "" {
		steps = append(steps, "Downloading dictionary")
	}
	if p.CopyDictionary {
		steps = append(steps, "Installing dictionary")
	}
	if p.InstallUnit {
		steps = append(steps, "Installing systemd unit")
	}
	if p.EnableUnit {
		steps = append(steps, "Starting service")
	}
	return steps
}

// BuildPlan validates options and settings and resolves every filesystem
// action without changing anything.
func BuildPlan(ctx context.Context, options Options, settings Settings) (Plan, error) {
	plan := Plan{Checks: CheckEnvironment(ctx, options)}

	configPath := firstNonEmpty(settings.ConfigPath, options.ConfigPath)
	if configPath == "" {
		resolved, err := paths.ConfigPath()
		if err != nil {
			return plan, err
		}
		configPath = resolved
	}
	configPath, err := paths.ExpandHome(configPath)
	if err != nil {
		return plan, err
	}
	plan.ConfigPath = configPath

	sourceLang := firstNonEmpty(settings.SourceLang, options.SourceLang, "en")
	targetLang := firstNonEmpty(settings.TargetLang, options.TargetLang, "en")

	existing, exists, err := config.Load(configPath)
	if err != nil {
		return plan, fmt.Errorf("read existing configuration: %w", err)
	}
	plan.ConfigExists = exists
	replaceConfig := options.Force || settings.ReplaceConfig

	if exists && !replaceConfig {
		plan.Config = config.Effective(existing)
		plan.WriteConfig = false
		plan.Notes = append(plan.Notes, "existing configuration kept; use --force to replace it")
		plan.DictionaryTarget = plan.Config.Dictionary.DatabasePath
		plan.Checks = replaceCheck(plan.Checks, dictionaryCheck(ctx, plan.DictionaryTarget, false))
	} else {
		plan.WriteConfig = true
		resolution, err := resolveDictionary(ctx, options, settings)
		if err != nil {
			return plan, err
		}
		plan.DictionarySource = resolution.source
		plan.DictionaryTarget = resolution.target
		plan.DictionaryPackaged = resolution.packaged
		plan.DictionaryVersion = resolution.version
		plan.CopyDictionary = resolution.copy
		plan.DownloadURL = resolution.downloadURL
		plan.DownloadSHA256 = resolution.downloadSHA256
		plan.Checks = replaceCheck(plan.Checks, resolution.check)
		if resolution.note != "" {
			plan.Notes = append(plan.Notes, resolution.note)
		}

		cfg := config.Default()
		cfg.Dictionary.DatabasePath = plan.DictionaryTarget
		cfg.Dictionary.SourceLanguage = strings.TrimSpace(sourceLang)
		cfg.Dictionary.TargetLanguage = strings.TrimSpace(targetLang)
		cfg.Translation.Enabled = settings.Translation
		if modelDir := firstNonEmpty(settings.ModelDir, options.ModelDir); modelDir != "" {
			expanded, err := paths.ExpandHome(modelDir)
			if err != nil {
				return plan, fmt.Errorf("resolve model directory: %w", err)
			}
			cfg.Translation.ModelDirectory = expanded
		}
		if err := cfg.Validate(); err != nil {
			return plan, err
		}
		plan.Config = cfg
	}

	plan.InstallUnit, plan.EnableUnit, plan.UnitPath, plan.Unit, err = resolveService(options, settings, configPath)
	if err != nil {
		return plan, err
	}
	if plan.InstallUnit {
		plan.Notes = append(plan.Notes, "systemd unit will be written to "+plan.UnitPath)
	}
	return plan, nil
}

type dictionaryResolution struct {
	source         string
	target         string
	packaged       bool
	version        string
	copy           bool
	downloadURL    string
	downloadSHA256 string
	check          Check
	note           string
}

func resolveDictionary(ctx context.Context, options Options, settings Settings) (dictionaryResolution, error) {
	var result dictionaryResolution
	target, err := paths.DefaultDictionaryPath()
	if err != nil {
		return result, err
	}
	result.target = target

	var packaged *packagedDictionary
	if options.DictionaryURL == "" {
		discovered, findErr := findPackagedDictionary(searchDirs(options))
		switch {
		case findErr != nil && settings.Dictionary == "":
			return result, findErr
		case findErr == nil:
			packaged = discovered
		}
		// An explicit dictionary wins over a broken package; preflight still
		// reports the package problem.
	}

	switch {
	case options.DictionaryURL != "":
		if settings.Dictionary != "" {
			return result, errors.New("--dictionary and --dictionary-url are mutually exclusive")
		}
		if options.DryRun {
			return result, errors.New("--dry-run cannot verify --dictionary-url; download the file first")
		}
		if strings.TrimSpace(options.DictionarySHA256) == "" {
			return result, errors.New("--dictionary-url requires --dictionary-sha256")
		}
		result.source = options.DictionaryURL
		result.downloadURL = options.DictionaryURL
		result.downloadSHA256 = options.DictionarySHA256
		result.copy = true
		result.check = Check{Name: "dictionary", Status: StatusWarn, Detail: "will be downloaded from " + options.DictionaryURL}
		return result, nil

	case settings.Dictionary != "":
		source, err := paths.ExpandHome(settings.Dictionary)
		if err != nil {
			return result, err
		}
		if err := validateDictionary(ctx, source); err != nil {
			return result, fmt.Errorf("dictionary %s: %w", source, err)
		}
		if options.DictionarySHA256 != "" {
			if err := dictionary.VerifySHA256(source, options.DictionarySHA256); err != nil {
				return result, fmt.Errorf("dictionary %s: %w", source, err)
			}
		}
		result.source = source
		if packaged != nil && samePath(source, packaged.path) {
			result.packaged = true
			result.target = packaged.path
			if packaged.manifest != nil {
				result.version = packaged.manifest.Version
			}
			result.check = Check{Name: "dictionary", Status: StatusOK, Detail: packagedDetail(result.target, result.version, true)}
			return result, nil
		}
		result.copy = true
		if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() && !options.Force {
			result.copy = false
			result.note = "keeping existing dictionary at " + target
		}
		result.check = Check{Name: "dictionary", Status: StatusOK, Detail: source}
		return result, nil

	case packaged != nil:
		result.source = packaged.path
		result.target = packaged.path
		result.packaged = true
		if packaged.manifest != nil {
			result.version = packaged.manifest.Version
		}
		result.check = Check{Name: "dictionary", Status: StatusOK, Detail: packagedDetail(result.target, result.version, true)}
		return result, nil

	default:
		result.check = Check{Name: "dictionary", Status: StatusWarn, Detail: "no dictionary found; configure one later with lex config set"}
		result.note = "no dictionary configured; lookups will fail until one is installed"
		return result, nil
	}
}

func resolveService(options Options, settings Settings, configPath string) (install, enable bool, unitPath, unit string, err error) {
	if !settings.InstallService {
		return false, false, "", "", nil
	}
	daemonBinary := findDaemonBinary(options.DaemonBinary)
	if daemonBinary == "" {
		return false, false, "", "", nil
	}
	if _, lookErr := exec.LookPath("systemctl"); lookErr != nil {
		// The service is optional, so a missing systemctl is not an error.
		return false, false, "", "", nil //nolint:nilerr // service is optional
	}
	unitPath, err = systemd.UnitPath()
	if err != nil {
		return false, false, "", "", err
	}
	unit = systemd.RenderUnit(daemonBinary, configPath)
	if info, statErr := os.Stat(unitPath); statErr == nil && info.Mode().IsRegular() && !options.Force {
		return false, true, unitPath, unit, nil
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return false, false, "", "", fmt.Errorf("inspect systemd unit %s: %w", unitPath, statErr)
	}
	return true, true, unitPath, unit, nil
}

func dictionaryCheck(ctx context.Context, path string, packaged bool) Check {
	if strings.TrimSpace(path) == "" {
		return Check{Name: "dictionary", Status: StatusWarn, Detail: "no dictionary configured"}
	}
	if err := validateDictionary(ctx, path); err != nil {
		return Check{Name: "dictionary", Status: StatusFail, Detail: path + ": " + err.Error()}
	}
	return Check{Name: "dictionary", Status: StatusOK, Detail: packagedDetail(path, "", packaged)}
}

func packagedDetail(path, version string, packaged bool) string {
	detail := path
	if packaged {
		detail += " (packaged"
		if version != "" {
			detail += " " + version
		}
		detail += ")"
	}
	return detail
}

// Run performs a non-interactive installation using the flag values.
func Run(ctx context.Context, options Options) error {
	options = withDefaults(options)
	settings := Settings{
		ConfigPath:     options.ConfigPath,
		Dictionary:     options.Dictionary,
		ModelDir:       options.ModelDir,
		SourceLang:     options.SourceLang,
		TargetLang:     options.TargetLang,
		Translation:    options.Translation,
		InstallService: !options.SkipService,
		ReplaceConfig:  options.Force,
	}

	plan, err := BuildPlan(ctx, options, settings)
	if err != nil {
		return err
	}
	WriteChecks(options.Stdout, plan.Checks)
	WriteSummary(options.Stdout, plan)
	if options.DryRun {
		fmt.Fprintln(options.Stdout, "\nDry run: no changes were made.")
		return nil
	}
	if failed := plan.FailedChecks(); len(failed) > 0 {
		return fmt.Errorf("preflight failed: %s", failed[0].Name+": "+failed[0].Detail)
	}
	if err := Apply(ctx, plan, textReporter(options.Stdout)); err != nil {
		return err
	}
	WriteNextSteps(options.Stdout, plan)
	return nil
}

func withDefaults(options Options) Options {
	if options.Stdin == nil {
		options.Stdin = os.Stdin
	}
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}
	if options.ConfigPath == "" {
		if resolved, err := paths.ConfigPath(); err == nil {
			options.ConfigPath = resolved
		}
	}
	if options.SourceLang == "" {
		options.SourceLang = "en"
	}
	if options.TargetLang == "" {
		options.TargetLang = "en"
	}
	return options
}

func searchDirs(options Options) []string {
	if options.DictionarySearchDirs != nil {
		return options.DictionarySearchDirs
	}
	return paths.PackagedDictionaryDirs()
}

func textReporter(w io.Writer) Reporter {
	return func(progress Progress) {
		fmt.Fprintf(w, "  [%d/%d] %s\n", progress.Index, progress.Total, progress.Step)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func findDaemonBinary(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if path, err := exec.LookPath("lexicon-daemon"); err == nil {
		return path
	}
	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), "lexicon-daemon")
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && leftAbs == rightAbs
}
