// Command lexicon-daemon is the long-running per-user process started by
// systemd. It has no lifecycle subcommands; systemctl manages the unit.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/daemon"
	"github.com/PandaX185/lexicon/internal/ipc"
	"github.com/PandaX185/lexicon/internal/paths"
	"github.com/PandaX185/lexicon/internal/version"
)

func main() {
	os.Exit(run())
}

func run() int {
	flags := flag.NewFlagSet("lexicon-daemon", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", "", "configuration file (default: XDG config path)")
	logLevel := flags.String("log-level", "", "override daemon.log_level")
	showVersion := flags.Bool("version", false, "print the version and exit")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: lexicon-daemon [--config PATH] [--log-level debug|info|warn|error]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(os.Stdout, version.StringFor("lexicon-daemon"))
		return 0
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}

	if *configPath == "" {
		resolved, err := paths.ConfigPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "lexicon-daemon: resolve config path: %v\n", err)
			return 1
		}
		*configPath = resolved
	}

	cfg, _, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lexicon-daemon: %v\n", err)
		return 1
	}
	levelText := cfg.Daemon.LogLevel
	if *logLevel != "" {
		levelText = *logLevel
	}
	level, err := config.ParseLogLevel(levelText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lexicon-daemon: %v\n", err)
		return 2
	}

	levelVar := &slog.LevelVar{}
	levelVar.Set(level)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: levelVar}))
	slog.SetDefault(logger)

	instance, err := daemon.New(*configPath, logger, levelVar)
	if err != nil {
		logger.Error("startup failed", "error", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := instance.Run(ctx); err != nil {
		if errors.Is(err, ipc.ErrAlreadyRunning) {
			logger.Error("another lexicon daemon is already running")
			return 1
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return 0
		}
		logger.Error("daemon stopped", "error", err)
		return 1
	}
	return 0
}
