// Command lexicon-popup is the Wails popup that displays daemon lookup
// results. It is a separate module so the core CLI and daemon build without
// WebKitGTK.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/paths"
	"github.com/PandaX185/lexicon/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	os.Exit(run())
}

func run() int {
	fallback := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	slog.SetDefault(fallback)

	flags := flag.NewFlagSet("lexicon-popup", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", "", "configuration file (default: XDG config path)")
	socketPath := flags.String("socket", "", "daemon Unix socket (overrides configuration)")
	showVersion := flags.Bool("version", false, "print the version and exit")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(os.Stdout, version.StringFor("lexicon-popup"))
		return 0
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: lexicon-popup [--config PATH] [--socket PATH]")
		return 2
	}

	settings := loadSettings(*configPath, *socketPath, fallback)

	app := application.New(application.Options{
		Name:        "Lexicon",
		Description: "Offline dictionary lookups from the Wayland primary selection",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "popup",
		Title:            "Lexicon",
		Width:            settings.width,
		Height:           settings.height,
		MinWidth:         min(settings.width, 360),
		MinHeight:        min(settings.height, 200),
		AlwaysOnTop:      true,
		Frameless:        true,
		DisableResize:    true,
		Hidden:           true,
		InitialPosition:  application.WindowCentered,
		BackgroundType:   application.BackgroundTypeSolid,
		BackgroundColour: application.NewRGB(24, 24, 27),
		URL:              "/",
	})

	popup := newPopup(app, window, settings, fallback)

	window.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		popup.hide()
	})
	window.OnWindowEvent(events.Common.WindowLostFocus, func(*application.WindowEvent) {
		popup.hideOnBlur()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.OnShutdown(cancel)

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		popup.start(ctx)
	})

	if err := app.Run(); err != nil {
		fallback.Error("popup failed", "error", err)
		return 1
	}
	return 0
}

// settings is the subset of configuration the popup display needs. The socket
// still prefers an explicit flag so a daemon started with a custom
// configuration always reaches the matching instance.
type settings struct {
	socket            string
	width             int
	height            int
	hideOnBlur        bool
	showPronunciation bool
}

func loadSettings(configPath, socketPath string, logger *slog.Logger) settings {
	result := settings{
		width:             420,
		height:            320,
		hideOnBlur:        true,
		showPronunciation: true,
	}

	if configPath == "" {
		resolved, err := paths.ConfigPath()
		if err != nil {
			logger.Warn("could not resolve config path; using defaults", "error", err)
		} else {
			configPath = resolved
		}
	}
	if configPath != "" {
		cfg, _, err := config.Load(configPath)
		if err != nil {
			logger.Warn("could not read configuration; using defaults", "path", configPath, "error", err)
		} else {
			effective := config.Effective(cfg)
			result.socket = effective.Daemon.Socket
			result.width = effective.Popup.Width
			result.height = effective.Popup.Height
			result.hideOnBlur = effective.Popup.HideOnBlur
			result.showPronunciation = effective.Dictionary.ShowPronunciation
		}
	}

	if socketPath != "" {
		resolved, err := paths.ExpandHome(socketPath)
		if err != nil {
			logger.Warn("invalid --socket value", "error", err)
		} else {
			result.socket = resolved
		}
	}
	if result.socket == "" {
		resolved, err := paths.DefaultSocketPath()
		if err != nil {
			logger.Error("could not resolve daemon socket", "error", err)
		} else {
			result.socket = resolved
		}
	}
	return result
}
