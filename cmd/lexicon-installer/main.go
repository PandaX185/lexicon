// Command lexicon-installer is the terminal setup wizard. The daemon is
// installed as a systemd user service, not as a CLI subcommand.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/PandaX185/lexicon/internal/installer"
)

func main() {
	options := installer.Options{}
	flag.StringVar(&options.ConfigPath, "config", "", "configuration path (default $XDG_CONFIG_HOME/lexicon/config.toml)")
	flag.StringVar(&options.Dictionary, "dictionary", "", "dictionary .sqlite file to install")
	flag.StringVar(&options.DictionaryURL, "dictionary-url", "", "explicit HTTPS URL of a dictionary asset (requires --dictionary-sha256)")
	flag.StringVar(&options.DictionarySHA256, "dictionary-sha256", "", "expected SHA-256 of --dictionary or --dictionary-url")
	flag.StringVar(&options.DaemonBinary, "daemon", "", "lexicon-daemon binary for the systemd unit")
	flag.StringVar(&options.SourceLang, "source-lang", "en", "headword language tag")
	flag.StringVar(&options.TargetLang, "target-lang", "en", "definition language tag")
	flag.BoolVar(&options.Translation, "translation", false, "enable offline translation")
	flag.BoolVar(&options.SkipService, "no-service", false, "do not install the systemd unit")
	flag.BoolVar(&options.Force, "force", false, "overwrite an existing config or dictionary")
	flag.BoolVar(&options.Yes, "yes", false, "accept defaults without prompting")
	flag.BoolVar(&options.DryRun, "dry-run", false, "print the plan without changing anything")
	flag.Parse()

	if err := installer.Run(context.Background(), options); err != nil {
		fmt.Fprintf(os.Stderr, "lexicon-installer: %v\n", err)
		os.Exit(1)
	}
}
