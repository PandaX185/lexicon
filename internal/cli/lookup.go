package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PandaX185/lexicon/internal/ipc"
)

func (a *App) runLookup(ctx context.Context, args []string) int {
	flags := a.newFlagSet("lookup", "[--json] [--timeout 60s] [text...]")
	jsonOutput := flags.Bool("json", false, "print the result as JSON")
	timeout := flags.Duration("timeout", 60*time.Second, "overall request timeout")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}

	text := strings.Join(flags.Args(), " ")
	cfg, err := a.effectiveConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}

	requestCtx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	client, err := ipc.Dial(requestCtx, cfg.Daemon.Socket, ipc.RoleCLI)
	if err != nil {
		a.errorf("%v", err)
		fmt.Fprintln(a.Stderr, daemonStartHint())
		return ExitFailure
	}
	defer func() { _ = client.Close() }()

	result, err := client.Lookup(requestCtx, text)
	if err != nil {
		a.errorf("lookup: %v", err)
		return ExitFailure
	}

	if *jsonOutput {
		encoded, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			a.errorf("encode result: %v", marshalErr)
			return ExitFailure
		}
		fmt.Fprintln(a.Stdout, string(encoded))
	} else if result.Found {
		a.renderEntry(result, cfg.Dictionary.ShowPronunciation)
	}

	if !result.Found {
		if result.Error != "" {
			a.errorf("%s", result.Error)
		} else {
			a.errorf("lookup failed")
		}
		return ExitFailure
	}
	return ExitOK
}

func daemonStartHint() string {
	return "Start the daemon with: systemctl --user start lexicon.service, or run lexicon-daemon directly"
}

func (a *App) renderEntry(result *ipc.LookupResult, showPronunciation bool) {
	entry := result.Entry
	if entry == nil {
		return
	}
	title := entry.Word
	if entry.Transliteration != "" {
		title += "  (" + entry.Transliteration + ")"
	}
	fmt.Fprintln(a.Stdout, title)
	if entry.MatchedForm != "" {
		fmt.Fprintf(a.Stdout, "  inflected form: %s\n", entry.MatchedForm)
	}
	if showPronunciation {
		if entry.Pronunciation.Respelling != "" {
			fmt.Fprintf(a.Stdout, "  /%s/\n", entry.Pronunciation.Respelling)
		}
		if entry.Pronunciation.IPA != "" {
			fmt.Fprintf(a.Stdout, "  [%s]\n", entry.Pronunciation.IPA)
		}
	}
	for _, definition := range entry.Definitions {
		label := ""
		if definition.Label != "" {
			label = "  [" + definition.Label + "]"
		}
		fmt.Fprintf(a.Stdout, "\n  %s%s\n    %s\n", definition.PartOfSpeech, label, definition.Text)
	}
	if result.Translation != "" {
		fmt.Fprintf(a.Stdout, "\ntranslation: %s\n", result.Translation)
	}
	if source := entry.Metadata["source"]; source != "" {
		fmt.Fprintf(a.Stdout, "source: %s\n", source)
	}
	if license := entry.Metadata["license"]; license != "" {
		fmt.Fprintf(a.Stdout, "license: %s\n", license)
	}
}
