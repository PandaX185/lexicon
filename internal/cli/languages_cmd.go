package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/translation"
)

type languagePack struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	Architecture string `json:"architecture,omitempty"`
	Directory    string `json:"directory"`
	License      string `json:"license,omitempty"`
}

type languageCatalogEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	Architecture string `json:"architecture,omitempty"`
	License      string `json:"license,omitempty"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
	Installed    bool   `json:"installed"`
}

type languagesReport struct {
	Enabled        bool                   `json:"enabled"`
	Source         string                 `json:"source_language"`
	Target         string                 `json:"target_language"`
	ModelDirectory string                 `json:"model_directory"`
	Installed      []languagePack         `json:"installed"`
	Catalog        []languageCatalogEntry `json:"catalog"`
}

func (a *App) runLanguages(ctx context.Context, args []string) int {
	if len(args) == 0 {
		a.errorf("languages requires a subcommand: list, install, remove, use")
		return ExitUsage
	}
	switch args[0] {
	case "list":
		return a.languagesList(args[1:])
	case "install":
		return a.languagesInstall(ctx, args[1:])
	case "remove":
		return a.languagesRemove(args[1:])
	case "use":
		return a.languagesUse(ctx, args[1:])
	default:
		a.errorf("unknown languages subcommand %q", args[0])
		return ExitUsage
	}
}

func (a *App) languagesList(args []string) int {
	flags := a.newFlagSet("languages list", "[--json]")
	jsonOutput := flags.Bool("json", false, "print the report as JSON")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if len(flags.Args()) != 0 {
		a.errorf("usage: lex languages list [--json]")
		return ExitUsage
	}

	effective, err := a.effectiveConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	report, err := buildLanguagesReport(effective)
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}

	if *jsonOutput {
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			a.errorf("encode report: %v", err)
			return ExitFailure
		}
		fmt.Fprintln(a.Stdout, string(encoded))
		return ExitOK
	}

	state := "disabled"
	if report.Enabled {
		state = "enabled"
	}
	fmt.Fprintf(a.Stdout, "translation: %s (%s -> %s)\n", state, report.Source, report.Target)
	fmt.Fprintf(a.Stdout, "model directory: %s\n", report.ModelDirectory)
	fmt.Fprintln(a.Stdout)

	fmt.Fprintln(a.Stdout, "installed:")
	if len(report.Installed) == 0 {
		fmt.Fprintln(a.Stdout, "  (none)")
	}
	for _, pack := range report.Installed {
		fmt.Fprintf(a.Stdout, "  %s  %s -> %s", pack.ID, pack.Source, pack.Target)
		if pack.Architecture != "" {
			fmt.Fprintf(a.Stdout, "  %s", pack.Architecture)
		}
		fmt.Fprintln(a.Stdout)
	}
	fmt.Fprintln(a.Stdout)

	fmt.Fprintln(a.Stdout, "catalog:")
	if len(report.Catalog) == 0 {
		fmt.Fprintln(a.Stdout, "  (empty)")
	}
	for _, entry := range report.Catalog {
		suffix := ""
		if entry.Installed {
			suffix = "  installed"
		}
		fmt.Fprintf(a.Stdout, "  %s  %s -> %s  %s%s\n",
			entry.ID, entry.Source, entry.Target, entry.Name, suffix)
	}
	return ExitOK
}

func (a *App) languagesInstall(ctx context.Context, args []string) int {
	if len(args) != 1 {
		a.errorf("usage: lex languages install ID")
		return ExitUsage
	}
	id := strings.TrimSpace(args[0])
	entry, found, err := translation.FindCatalogEntry(id)
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	if !found {
		a.errorf("unknown model pack %q", id)
		fmt.Fprintln(a.Stderr, "Run \"lex languages list\" to see available packs.")
		return ExitUsage
	}

	effective, err := a.effectiveConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	fmt.Fprintf(a.Stdout, "downloading %s (%s)...\n", entry.ID, formatBytes(entry.Size))
	target, err := translation.InstallPack(ctx, entry, effective.Translation.ModelDirectory, nil)
	if err != nil {
		a.errorf("install %s: %v", entry.ID, err)
		return ExitFailure
	}
	fmt.Fprintf(a.Stdout, "installed %s in %s\n", entry.ID, target)
	fmt.Fprintf(a.Stdout, "hint: select it with \"lex languages use %s\"\n",
		pairArgument(entry.Source, entry.Target))
	if !effective.Translation.Enabled {
		fmt.Fprintln(a.Stdout, "hint: translation is disabled; enable it with \"lex config set translation.enabled true\"")
	}
	return ExitOK
}

func (a *App) languagesRemove(args []string) int {
	flags := a.newFlagSet("languages remove", "[--yes] ID")
	yes := flags.Bool("yes", false, "do not prompt")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if len(flags.Args()) != 1 {
		a.errorf("usage: lex languages remove [--yes] ID")
		return ExitUsage
	}
	id := strings.TrimSpace(flags.Arg(0))
	if !translation.ValidPackID(id) {
		a.errorf("%q is not a model pack id", id)
		return ExitUsage
	}

	effective, err := a.effectiveConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	target := filepath.Join(effective.Translation.ModelDirectory, id)
	info, err := os.Stat(target)
	switch {
	case errors.Is(err, os.ErrNotExist):
		a.errorf("model pack %q is not installed", id)
		return ExitFailure
	case err != nil:
		a.errorf("inspect model pack: %v", err)
		return ExitFailure
	case !info.IsDir():
		a.errorf("%s is not a model pack directory", target)
		return ExitFailure
	}
	if _, err := os.Stat(filepath.Join(target, "metadata.json")); err != nil {
		a.errorf("%s does not contain metadata.json", target)
		return ExitFailure
	}

	if !*yes {
		confirmed, err := a.confirm(fmt.Sprintf("Remove model pack %s from %s?", id, target))
		if err != nil {
			a.errorf("read confirmation: %v", err)
			return ExitFailure
		}
		if !confirmed {
			fmt.Fprintln(a.Stdout, "cancelled")
			return ExitOK
		}
	}

	if err := os.RemoveAll(target); err != nil {
		a.errorf("remove model pack: %v", err)
		return ExitFailure
	}
	fmt.Fprintf(a.Stdout, "removed %s\n", id)

	source := translation.CanonicalLanguageCode(effective.Translation.SourceLanguage)
	dest := translation.CanonicalLanguageCode(effective.Translation.TargetLanguage)
	if _, err := translation.FindPack(effective.Translation.ModelDirectory, source, dest); errors.Is(err, translation.ErrNoModel) {
		fmt.Fprintf(a.Stderr, "warning: no model pack remains for the configured pair %s -> %s\n", source, dest)
	}
	return ExitOK
}

func (a *App) languagesUse(ctx context.Context, args []string) int {
	if len(args) != 1 {
		a.errorf("usage: lex languages use SRC-TGT")
		return ExitUsage
	}
	source, target, err := parseLanguagePair(args[0])
	if err != nil {
		a.errorf("%v", err)
		return ExitUsage
	}

	previous, err := a.effectiveConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	if _, err := translation.FindPack(previous.Translation.ModelDirectory, source, target); err != nil {
		if errors.Is(err, translation.ErrNoModel) {
			a.errorf("no installed model pack translates %s -> %s", source, target)
			if entry, found, catalogErr := translation.CatalogEntryForPair(source, target); catalogErr == nil && found {
				fmt.Fprintf(a.Stderr, "Install it with: lex languages install %s\n", entry.ID)
			}
			return ExitFailure
		}
		a.errorf("find model pack: %v", err)
		return ExitFailure
	}

	cfg, _, err := a.loadConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	cfg.Translation.SourceLanguage = source
	cfg.Translation.TargetLanguage = target
	if err := config.Write(a.ConfigPath, cfg); err != nil {
		a.errorf("write config: %v", err)
		return ExitFailure
	}
	fmt.Fprintf(a.Stdout, "translation pair: %s -> %s\n", source, target)
	if !cfg.Translation.Enabled {
		fmt.Fprintln(a.Stdout, "hint: translation is disabled; enable it with \"lex config set translation.enabled true\"")
	}
	a.reloadDaemon(ctx, previous.Daemon.Socket)
	return ExitOK
}

func buildLanguagesReport(effective config.Config) (languagesReport, error) {
	report := languagesReport{
		Enabled:        effective.Translation.Enabled,
		Source:         translation.CanonicalLanguageCode(effective.Translation.SourceLanguage),
		Target:         translation.CanonicalLanguageCode(effective.Translation.TargetLanguage),
		ModelDirectory: effective.Translation.ModelDirectory,
		Installed:      []languagePack{},
		Catalog:        []languageCatalogEntry{},
	}

	packs, err := translation.InstalledPacks(effective.Translation.ModelDirectory)
	if err != nil {
		return languagesReport{}, err
	}
	installed := make(map[string]bool, len(packs)*2)
	for _, pack := range packs {
		installed[pack.ID] = true
		installed[filepath.Base(pack.Directory)] = true
		report.Installed = append(report.Installed, languagePack{
			ID:           pack.ID,
			Source:       pack.Source,
			Target:       pack.Target,
			Architecture: pack.Architecture,
			Directory:    pack.Directory,
			License:      pack.License,
		})
	}
	sort.Slice(report.Installed, func(i, j int) bool {
		return report.Installed[i].ID < report.Installed[j].ID
	})

	entries, err := translation.Catalog()
	if err != nil {
		return languagesReport{}, err
	}
	for _, entry := range entries {
		report.Catalog = append(report.Catalog, languageCatalogEntry{
			ID:           entry.ID,
			Name:         entry.Name,
			Source:       entry.Source,
			Target:       entry.Target,
			Architecture: entry.Architecture,
			License:      entry.License,
			URL:          entry.URL,
			SHA256:       entry.SHA256,
			Size:         entry.Size,
			Installed:    installed[entry.ID],
		})
	}
	sort.Slice(report.Catalog, func(i, j int) bool {
		if report.Catalog[i].Source == report.Catalog[j].Source {
			return report.Catalog[i].Target < report.Catalog[j].Target
		}
		return report.Catalog[i].Source < report.Catalog[j].Source
	})
	return report, nil
}

func parseLanguagePair(value string) (string, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	var parts []string
	switch {
	case strings.Contains(normalized, ":"):
		parts = strings.SplitN(normalized, ":", 2)
	case strings.Contains(normalized, ">"):
		parts = strings.SplitN(normalized, ">", 2)
	default:
		index := strings.Index(normalized, "-")
		if index < 0 {
			return "", "", fmt.Errorf("language pair %q must look like en-de", value)
		}
		parts = []string{normalized[:index], normalized[index+1:]}
	}
	source := translation.CanonicalLanguageCode(parts[0])
	target := translation.CanonicalLanguageCode(parts[1])
	if !translation.ValidLanguageCode(source) || !translation.ValidLanguageCode(target) {
		return "", "", fmt.Errorf("language pair %q must use lowercase BCP 47 tags", value)
	}
	if source == target {
		return "", "", fmt.Errorf("language pair %q needs two different languages", value)
	}
	return source, target, nil
}

func pairArgument(source, target string) string {
	if strings.Contains(source, "-") || strings.Contains(target, "-") {
		return source + ":" + target
	}
	return source + "-" + target
}

func formatBytes(size int64) string {
	const (
		kib = 1 << 10
		mib = 1 << 20
	)
	if size >= mib {
		return fmt.Sprintf("%.1f MiB", float64(size)/mib)
	}
	if size >= kib {
		return fmt.Sprintf("%.1f KiB", float64(size)/kib)
	}
	return fmt.Sprintf("%d B", size)
}
