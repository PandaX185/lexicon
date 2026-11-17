// Package installer implements the interactive terminal setup wizard.
package installer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/paths"
	"github.com/PandaX185/lexicon/internal/systemd"
)

// Options carries flags and process streams.
type Options struct {
	ConfigPath   string
	Dictionary   string
	DaemonBinary string
	SourceLang   string
	TargetLang   string
	Translation  bool
	SkipService  bool
	Force        bool
	Yes          bool
	DryRun       bool

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Plan is the resolved set of actions, also useful in dry-run mode.
type Plan struct {
	ConfigPath       string
	DictionarySource string
	DictionaryTarget string
	CopyDictionary   bool
	UnitPath         string
	Unit             string
	InstallUnit      bool
	EnableUnit       bool
}

// Run performs the wizard and applies the plan.
func Run(ctx context.Context, options Options) error {
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
		resolved, err := paths.ConfigPath()
		if err != nil {
			return err
		}
		options.ConfigPath = resolved
	}
	if options.SourceLang == "" {
		options.SourceLang = "en"
	}
	if options.TargetLang == "" {
		options.TargetLang = "en"
	}

	prompter := newPrompter(options.Stdin, options.Stdout)
	fmt.Fprintln(options.Stdout, "Lexicon installer")
	fmt.Fprintln(options.Stdout, "This writes user configuration; it does not modify system files.")
	fmt.Fprintln(options.Stdout)

	if _, err := os.Stat(options.ConfigPath); err == nil && !options.Force {
		replace, err := prompter.confirm("Configuration already exists at "+options.ConfigPath+". Replace it?", false, options.Yes)
		if err != nil {
			return err
		}
		if !replace {
			fmt.Fprintln(options.Stdout, "Nothing changed.")
			return nil
		}
	}

	dictionarySource, err := prompter.ask("Dictionary .sqlite path (leave empty to configure later)", options.Dictionary, options.Yes)
	if err != nil {
		return err
	}
	dictionaryTarget, err := paths.DefaultDictionaryPath()
	if err != nil {
		return err
	}
	if dictionarySource != "" {
		dictionarySource, err = paths.ExpandHome(dictionarySource)
		if err != nil {
			return err
		}
		if err := validateDictionary(ctx, dictionarySource); err != nil {
			return fmt.Errorf("dictionary %s: %w", dictionarySource, err)
		}
	}

	sourceLang, err := prompter.ask("Headword language (BCP 47)", options.SourceLang, options.Yes)
	if err != nil {
		return err
	}
	targetLang, err := prompter.ask("Definition language (BCP 47)", options.TargetLang, options.Yes)
	if err != nil {
		return err
	}

	translation := options.Translation
	if !options.Yes {
		translation, err = prompter.confirm("Enable offline Bergamot translation when languages differ?", options.Translation, false)
		if err != nil {
			return err
		}
	}

	installService := !options.SkipService
	if installService && !options.Yes {
		installService, err = prompter.confirm("Install the systemd user service?", true, false)
		if err != nil {
			return err
		}
	}
	if installService {
		if _, err := exec.LookPath("systemctl"); err != nil {
			fmt.Fprintln(options.Stderr, "warning: systemctl not found; skipping the service unit")
			installService = false
		}
	}

	daemonBinary := findDaemonBinary(options.DaemonBinary)
	if installService && daemonBinary == "" {
		fmt.Fprintln(options.Stderr, "warning: lexicon-daemon binary not found; skipping the service unit")
		installService = false
	}

	cfg := config.Default()
	cfg.Dictionary.DatabasePath = dictionaryTarget
	cfg.Dictionary.SourceLanguage = strings.TrimSpace(sourceLang)
	cfg.Dictionary.TargetLanguage = strings.TrimSpace(targetLang)
	cfg.Translation.Enabled = translation

	plan := Plan{
		ConfigPath:       options.ConfigPath,
		DictionarySource: dictionarySource,
		DictionaryTarget: dictionaryTarget,
		CopyDictionary:   dictionarySource != "" && !samePath(dictionarySource, dictionaryTarget),
		InstallUnit:      installService,
		EnableUnit:       installService,
	}
	if plan.CopyDictionary {
		if info, err := os.Stat(dictionaryTarget); err == nil && info.Mode().IsRegular() && !options.Force {
			plan.CopyDictionary = false
			fmt.Fprintf(options.Stderr, "note: keeping existing dictionary at %s\n", dictionaryTarget)
		}
	}
	if installService {
		unitPath, err := systemd.UnitPath()
		if err != nil {
			return err
		}
		plan.UnitPath = unitPath
		plan.Unit = systemd.RenderUnit(daemonBinary, options.ConfigPath)
	}

	fmt.Fprintln(options.Stdout)
	fmt.Fprintln(options.Stdout, "Summary")
	fmt.Fprintf(options.Stdout, "  config:      %s\n", plan.ConfigPath)
	switch {
	case plan.CopyDictionary:
		fmt.Fprintf(options.Stdout, "  dictionary:  %s -> %s\n", plan.DictionarySource, plan.DictionaryTarget)
	case dictionarySource != "":
		fmt.Fprintf(options.Stdout, "  dictionary:  %s (unchanged)\n", plan.DictionaryTarget)
	default:
		fmt.Fprintf(options.Stdout, "  dictionary:  %s (to be provided)\n", plan.DictionaryTarget)
	}
	fmt.Fprintf(options.Stdout, "  languages:   %s -> %s\n", cfg.Dictionary.SourceLanguage, cfg.Dictionary.TargetLanguage)
	fmt.Fprintf(options.Stdout, "  translation: %t\n", cfg.Translation.Enabled)
	if plan.InstallUnit {
		fmt.Fprintf(options.Stdout, "  service:     %s\n", plan.UnitPath)
	} else {
		fmt.Fprintln(options.Stdout, "  service:     not installed")
	}
	if options.DryRun {
		fmt.Fprintln(options.Stdout, "\nDry run: no changes were made.")
		return nil
	}

	confirmed, err := prompter.confirm("Apply this configuration?", true, options.Yes)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(options.Stdout, "Nothing changed.")
		return nil
	}

	if err := apply(ctx, plan, cfg, options); err != nil {
		return err
	}

	fmt.Fprintln(options.Stdout)
	fmt.Fprintln(options.Stdout, "Installed.")
	fmt.Fprintln(options.Stdout, "Next steps:")
	fmt.Fprintln(options.Stdout, "  lex doctor")
	switch {
	case plan.InstallUnit:
		fmt.Fprintln(options.Stdout, "  systemctl --user status lexicon.service")
	case daemonBinary != "":
		fmt.Fprintf(options.Stdout, "  %s --config %s\n", daemonBinary, options.ConfigPath)
	default:
		fmt.Fprintln(options.Stdout, "  lexicon-daemon --config "+options.ConfigPath)
	}
	fmt.Fprintln(options.Stdout, "  systemctl --user import-environment WAYLAND_DISPLAY   # for popup auto-launch")
	return nil
}

func apply(ctx context.Context, plan Plan, cfg config.Config, options Options) error {
	if err := config.Write(plan.ConfigPath, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Fprintf(options.Stdout, "wrote %s\n", plan.ConfigPath)

	if plan.CopyDictionary {
		if err := copyFile(plan.DictionarySource, plan.DictionaryTarget); err != nil {
			return fmt.Errorf("copy dictionary: %w", err)
		}
		fmt.Fprintf(options.Stdout, "copied dictionary to %s\n", plan.DictionaryTarget)
	}

	if plan.InstallUnit {
		if err := systemd.Install(ctx, plan.UnitPath, plan.Unit); err != nil {
			return err
		}
		fmt.Fprintf(options.Stdout, "installed %s\n", plan.UnitPath)
		if plan.EnableUnit {
			if err := systemd.Enable(ctx); err != nil {
				return fmt.Errorf("enable service: %w", err)
			}
			fmt.Fprintln(options.Stdout, "enabled and started lexicon.service")
		}
	}
	return nil
}

func validateDictionary(ctx context.Context, path string) error {
	store, err := dictionary.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	info, err := store.Describe(ctx)
	if err != nil {
		return err
	}
	if info.Name == "" {
		return errors.New("dictionary metadata has no name")
	}
	return nil
}

func copyFile(source, target string) error {
	//nolint:gosec // source is the dictionary path chosen by the installer user
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()

	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".dictionary-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()

	if _, err := io.Copy(temp, input); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempName, 0o600); err != nil {
		return err
	}
	return os.Rename(tempName, target)
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

type prompter struct {
	reader *bufio.Reader
	out    io.Writer
}

func newPrompter(in io.Reader, out io.Writer) *prompter {
	return &prompter{reader: bufio.NewReader(in), out: out}
}

func (p *prompter) ask(label, defaultValue string, nonInteractive bool) (string, error) {
	if nonInteractive {
		return defaultValue, nil
	}
	if defaultValue != "" {
		fmt.Fprintf(p.out, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(p.out, "%s: ", label)
	}
	answer, err := p.reader.ReadString('\n')
	if err != nil && answer == "" {
		if errors.Is(err, io.EOF) {
			return defaultValue, nil
		}
		return "", err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return defaultValue, nil
	}
	return answer, nil
}

func (p *prompter) confirm(label string, defaultValue, nonInteractive bool) (bool, error) {
	if nonInteractive {
		return defaultValue, nil
	}
	suffix := "[y/N]"
	if defaultValue {
		suffix = "[Y/n]"
	}
	fmt.Fprintf(p.out, "%s %s: ", label, suffix)
	answer, err := p.reader.ReadString('\n')
	if err != nil && answer == "" {
		if errors.Is(err, io.EOF) {
			return defaultValue, nil
		}
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "":
		return defaultValue, nil
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
