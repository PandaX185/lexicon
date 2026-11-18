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
	"strings"

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
		BackgroundColour: themeBaseColour(settings.theme),
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
	theme             string
	showHeader        bool
	showStatus        bool
	showFooter        bool
}

func loadSettings(configPath, socketPath string, logger *slog.Logger) settings {
	result := settings{
		width:             420,
		height:            320,
		hideOnBlur:        true,
		showPronunciation: true,
		theme:             "system",
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
			result.theme = strings.ToLower(strings.TrimSpace(effective.Popup.Theme))
			result.showHeader = effective.Popup.ShowHeader
			result.showStatus = effective.Popup.ShowStatus
			result.showFooter = effective.Popup.ShowFooter
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
	if result.theme == "" {
		result.theme = "system"
	}
	return result
}

// themeBaseColour is the opaque window colour painted before the webview's
// first frame, so a light preset never flashes dark. The stylesheet applies
// the full palette afterwards. Keep this in sync with PopupThemes and
// frontend/src/style.css; theme tests enforce both.
func themeBaseColour(theme string) application.RGBA {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	// Dark presets.
	case "nord":
		return application.NewRGB(46, 52, 64)
	case "catppuccin":
		return application.NewRGB(30, 30, 46)
	case "gruvbox":
		return application.NewRGB(40, 40, 40)
	case "rose-pine":
		return application.NewRGB(25, 23, 36)
	case "dracula":
		return application.NewRGB(40, 42, 54)
	case "tokyo-night":
		return application.NewRGB(26, 27, 38)
	case "one-dark":
		return application.NewRGB(40, 44, 52)
	case "solarized-dark":
		return application.NewRGB(0, 43, 54)
	case "github-dark":
		return application.NewRGB(13, 17, 23)

	// Light presets.
	case "light", "one-light":
		return application.NewRGB(250, 250, 250)
	case "nord-light":
		return application.NewRGB(236, 239, 244)
	case "catppuccin-latte":
		return application.NewRGB(239, 241, 245)
	case "gruvbox-light":
		return application.NewRGB(251, 241, 199)
	case "rose-pine-dawn":
		return application.NewRGB(250, 244, 237)
	case "solarized-light":
		return application.NewRGB(253, 246, 227)
	case "github-light":
		return application.NewRGB(255, 255, 255)
	case "tokyo-night-light":
		return application.NewRGB(225, 226, 231)
	case "everforest-light":
		return application.NewRGB(253, 246, 221)

	// "system" and any unknown value use the neutral dark base.
	default:
		return application.NewRGB(24, 24, 27)
	}
}
