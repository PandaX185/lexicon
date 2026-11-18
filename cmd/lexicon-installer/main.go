// Command lexicon-installer is the terminal setup wizard. The daemon is
// installed as a systemd user service, not as a CLI subcommand.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/PandaX185/lexicon/internal/installer"
	"github.com/PandaX185/lexicon/internal/installer/tui"
)

func main() {
	options := installer.Options{}
	flag.StringVar(&options.ConfigPath, "config", "", "configuration path (default $XDG_CONFIG_HOME/lexicon/config.toml)")
	flag.StringVar(&options.Dictionary, "dictionary", "", "dictionary .sqlite file to install")
	flag.StringVar(&options.DictionaryURL, "dictionary-url", "", "explicit HTTPS URL of a dictionary asset (requires --dictionary-sha256)")
	flag.StringVar(&options.DictionarySHA256, "dictionary-sha256", "", "expected SHA-256 of --dictionary or --dictionary-url")
	flag.StringVar(&options.DaemonBinary, "daemon", "", "lexicon-daemon binary for the systemd unit")
	flag.StringVar(&options.ModelDir, "model-dir", "", "translation model directory (default $XDG_DATA_HOME/lexicon/models)")
	flag.StringVar(&options.SourceLang, "source-lang", "en", "headword language tag")
	flag.StringVar(&options.TargetLang, "target-lang", "en", "definition language tag")
	flag.BoolVar(&options.Translation, "translation", false, "enable offline translation")
	flag.BoolVar(&options.SkipService, "no-service", false, "do not install the systemd unit")
	flag.BoolVar(&options.SkipService, "skip-service", false, "alias for --no-service")
	flag.BoolVar(&options.Force, "force", false, "replace an existing configuration, dictionary, or unit")
	flag.BoolVar(&options.Yes, "yes", false, "run without the TUI and accept the flag values")
	flag.BoolVar(&options.DryRun, "dry-run", false, "print the resolved plan without changing anything")
	theme := flag.String("theme", "", "popup theme preset")
	showPronunciation := flag.Bool("show-pronunciation", true, "show pronunciation when present")
	hideOnBlur := flag.Bool("hide-on-blur", true, "hide the popup when it loses focus")
	showHeader := flag.Bool("show-header", false, "show the popup header")
	showStatus := flag.Bool("show-status", false, "show the popup connection badge")
	showFooter := flag.Bool("show-footer", false, "show the popup footer")
	maxQueryWords := flag.Int("max-query-words", 4, "maximum words in a lookup")
	maxQueryChars := flag.Int("max-query-chars", 128, "maximum characters in a lookup")
	var packs stringList
	flag.Var(&packs, "pack", "model pack id to install; repeatable")
	flag.Parse()

	options.Appearance = installer.AppearanceSettings{
		Theme:             *theme,
		ShowPronunciation: showPronunciation,
		HideOnBlur:        hideOnBlur,
		ShowHeader:        showHeader,
		ShowStatus:        showStatus,
		ShowFooter:        showFooter,
		MaxQueryWords:     maxQueryWords,
		MaxQueryChars:     maxQueryChars,
	}
	options.Packs = append([]string(nil), packs...)

	ctx := context.Background()
	var err error
	terminal := isTerminal(os.Stdin)
	switch {
	case options.Yes || options.DryRun:
		err = installer.Run(ctx, options)
	case !terminal:
		err = errors.New("stdin is not a terminal; rerun with --yes for non-interactive setup")
	default:
		err = tui.Run(ctx, options)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "lexicon-installer: %v\n", err)
		os.Exit(1)
	}
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("value must not be empty")
	}
	*s = append(*s, value)
	return nil
}
