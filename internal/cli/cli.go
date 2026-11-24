// Package cli implements the Lexicon `lexicon` command-line interface.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/paths"
	"github.com/PandaX185/lexicon/internal/version"
)

// Exit codes.
const (
	ExitOK      = 0
	ExitFailure = 1
	ExitUsage   = 2
)

// App holds process streams and the resolved config path.
type App struct {
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	ConfigPath   string
	updateRunner updateRunnerInterface
}

// New returns an App bound to the process streams.
func New() *App {
	return &App{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
}

// Run dispatches a command and returns a process exit code.
func (a *App) Run(ctx context.Context, args []string) int {
	if a.Stdin == nil {
		a.Stdin = strings.NewReader("")
	}
	if a.Stdout == nil {
		a.Stdout = io.Discard
	}
	if a.Stderr == nil {
		a.Stderr = io.Discard
	}

	configPath, args, err := extractConfigFlag(args)
	if err != nil {
		a.errorf("%v", err)
		return ExitUsage
	}
	if configPath == "" {
		configPath = a.ConfigPath
	}
	if configPath == "" {
		resolved, err := paths.ConfigPath()
		if err != nil {
			a.errorf("resolve config path: %v", err)
			return ExitFailure
		}
		configPath = resolved
	}
	a.ConfigPath = configPath

	if len(args) == 0 {
		a.printUsage()
		return ExitUsage
	}

	switch args[0] {
	case "help", "-h", "--help":
		a.printUsage()
		return ExitOK
	case "version", "--version":
		fmt.Fprintln(a.Stdout, version.StringFor("lexicon"))
		return ExitOK
	case "lookup":
		return a.runLookup(ctx, args[1:])
	case "config":
		return a.runConfig(ctx, args[1:])
	case "languages":
		return a.runLanguages(ctx, args[1:])
	case "update":
		return a.runUpdate(ctx, args[1:])
	case "doctor":
		return a.runDoctor(ctx, args[1:])
	default:
		a.errorf("unknown command %q", args[0])
		a.printUsage()
		return ExitUsage
	}
}

func (a *App) loadConfig() (config.Config, bool, error) {
	return config.Load(a.ConfigPath)
}

func (a *App) effectiveConfig() (config.Config, error) {
	cfg, _, err := a.loadConfig()
	if err != nil {
		return config.Config{}, err
	}
	return config.Effective(cfg), nil
}

func (a *App) errorf(format string, args ...any) {
	fmt.Fprintf(a.Stderr, "lexicon: "+format+"\n", args...)
}

func (a *App) newFlagSet(name string, usage string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(a.Stderr)
	flags.Usage = func() {
		fmt.Fprintf(a.Stderr, "Usage: lexicon %s\n", usage)
		flags.PrintDefaults()
	}
	return flags
}

func (a *App) printUsage() {
	fmt.Fprint(a.Stdout, `lexicon — offline dictionary lookups from the Wayland primary selection

Usage:
  lexicon lookup [--json] [text...]
  lexicon config show|get|set|reset|edit|path
  lexicon languages list|install|remove|use
  lexicon update [--check] [--version X.Y.Z] [--yes] [--no-restart] [--bin-dir PATH]
  lexicon doctor
  lexicon version

Global option:
  --config PATH    configuration file (default $XDG_CONFIG_HOME/lexicon/config.toml)

The daemon is managed by systemd. The CLI is the only configuration interface.
Run "lexicon help" for this text.
`)
}

func extractConfigFlag(args []string) (string, []string, error) {
	var configPath string
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config" || arg == "-config":
			if i+1 >= len(args) {
				return "", nil, errors.New("--config requires a path")
			}
			configPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-config="):
			configPath = strings.TrimPrefix(arg, "-config=")
		default:
			remaining = append(remaining, arg)
		}
	}
	return configPath, remaining, nil
}
