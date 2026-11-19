package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func TestLoadSettingsAppearance(t *testing.T) {
	cfg := config.Default()
	cfg.Popup.Theme = "catppuccin-latte"
	cfg.Popup.ShowHeader = true
	cfg.Popup.ShowStatus = true
	cfg.Popup.ShowFooter = true

	data, err := config.Render(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got := loadSettings(path, "", newTestLogger(t))
	if got.theme != "catppuccin-latte" {
		t.Fatalf("theme = %q, want catppuccin-latte", got.theme)
	}
	if !got.showHeader || !got.showStatus || !got.showFooter {
		t.Fatalf("chrome toggles = %+v, want all enabled", got)
	}
}

func TestLoadSettingsAppearanceDefaults(t *testing.T) {
	got := loadSettings(filepath.Join(t.TempDir(), "missing.toml"), "", newTestLogger(t))
	if got.theme != "system" {
		t.Fatalf("theme = %q, want system", got.theme)
	}
	if got.showHeader || got.showStatus || got.showFooter {
		t.Fatalf("chrome defaults = %+v, want all disabled", got)
	}
}

type recordingQuitter struct {
	quits int
}

func (q *recordingQuitter) Quit() {
	q.quits++
}

func TestWindowCloseQuitsAfterCancelling(t *testing.T) {
	quitter := &recordingQuitter{}
	event := application.NewWindowEvent()

	dismissPopup(quitter, event)

	if quitter.quits != 1 {
		t.Fatalf("quits = %d, want 1", quitter.quits)
	}
	if !event.IsCancelled() {
		t.Fatal("window close was not cancelled; Wails would close the window without quitting the app")
	}
}

func TestEscapeQuitsPopup(t *testing.T) {
	quitter := &recordingQuitter{}

	dismissPopup(quitter, nil)

	if quitter.quits != 1 {
		t.Fatalf("quits = %d, want 1", quitter.quits)
	}
}

type recordedHook struct {
	eventType events.WindowEventType
	callback  func(*application.WindowEvent)
}

// fakeWindow records RegisterHook calls so tests can fire the registered
// callbacks without starting a Wails application.
type fakeWindow struct {
	hooks []recordedHook
}

func (w *fakeWindow) RegisterHook(eventType events.WindowEventType, callback func(*application.WindowEvent)) func() {
	w.hooks = append(w.hooks, recordedHook{eventType: eventType, callback: callback})
	return func() {}
}

func (w *fakeWindow) hook(eventType events.WindowEventType) func(*application.WindowEvent) {
	for _, hook := range w.hooks {
		if hook.eventType == eventType {
			return hook.callback
		}
	}
	return nil
}

func TestRegisterPopupHooksDismissOnFocusLoss(t *testing.T) {
	window := &fakeWindow{}
	quitter := &recordingQuitter{}
	popup := &Popup{quitter: quitter, shownAt: time.Now().Add(-time.Second)}

	registerPopupHooks(window, quitter, popup)

	focusLost := window.hook(events.Common.WindowLostFocus)
	if focusLost == nil {
		t.Fatalf("no WindowLostFocus hook registered; got %d hooks", len(window.hooks))
	}
	focusLost(nil)
	if quitter.quits != 1 {
		t.Fatalf("quits = %d after focus loss, want 1", quitter.quits)
	}
}

func TestWindowLostFocusRespectsEarlyBlurGuard(t *testing.T) {
	window := &fakeWindow{}
	quitter := &recordingQuitter{}
	popup := &Popup{quitter: quitter, shownAt: time.Now()}

	registerPopupHooks(window, quitter, popup)

	focusLost := window.hook(events.Common.WindowLostFocus)
	if focusLost == nil {
		t.Fatalf("no WindowLostFocus hook registered; got %d hooks", len(window.hooks))
	}
	focusLost(nil)
	if quitter.quits != 0 {
		t.Fatalf("quits = %d inside the early-blur guard, want 0", quitter.quits)
	}
}

func TestNewPopupUsesAppAsQuitter(t *testing.T) {
	var app *application.App
	popup := newPopup(app, nil, settings{}, newTestLogger(t), false)
	if popup.quitter != app {
		t.Fatal("popup must quit the Wails app when the blur guard passes")
	}
	if popup.replay {
		t.Fatal("manual popup must not be marked as a replay popup")
	}
}

func TestNewPopupKeepsReplayFlag(t *testing.T) {
	var app *application.App
	popup := newPopup(app, nil, settings{}, newTestLogger(t), true)
	if !popup.replay {
		t.Fatal("--replay must be carried into the popup backend")
	}
}

func TestRegisterPopupHooksKeepCloseDismissal(t *testing.T) {
	window := &fakeWindow{}
	quitter := &recordingQuitter{}

	registerPopupHooks(window, quitter, &Popup{})

	for _, eventType := range []events.WindowEventType{events.Common.WindowClosing, events.Linux.WindowDeleteEvent} {
		callback := window.hook(eventType)
		if callback == nil {
			t.Fatalf("no hook registered for %v", eventType)
		}
		event := application.NewWindowEvent()
		callback(event)
		if !event.IsCancelled() {
			t.Fatalf("close event %v was not cancelled", eventType)
		}
	}
	if quitter.quits != 2 {
		t.Fatalf("quits = %d after two close events, want 2", quitter.quits)
	}
}

func TestPopupWindowIsResizable(t *testing.T) {
	options := popupWindowOptions(nil, settings{width: 420, height: 320})
	if options.DisableResize {
		t.Fatal("popup window must allow edge resizing")
	}
	if options.MinWidth > 320 || options.MinHeight > 200 {
		t.Fatalf("minimum size %dx%d is larger than the compact translation layout", options.MinWidth, options.MinHeight)
	}
}

func TestEscapeIsNativeWindowKeyBinding(t *testing.T) {
	options := popupWindowOptions(nil, settings{})
	if _, ok := options.KeyBindings["escape"]; !ok {
		t.Fatal("Escape must be handled by Wails so it works when the webview is not focused")
	}
}

func TestThemeBaseColour(t *testing.T) {
	tests := map[string]application.RGBA{
		"system":            application.NewRGB(24, 24, 27),
		"unknown":           application.NewRGB(24, 24, 27),
		"dark":              application.NewRGB(24, 24, 27),
		"nord":              application.NewRGB(46, 52, 64),
		"catppuccin":        application.NewRGB(30, 30, 46),
		"gruvbox":           application.NewRGB(40, 40, 40),
		"rose-pine":         application.NewRGB(25, 23, 36),
		"dracula":           application.NewRGB(40, 42, 54),
		"tokyo-night":       application.NewRGB(26, 27, 38),
		"one-dark":          application.NewRGB(40, 44, 52),
		"solarized-dark":    application.NewRGB(0, 43, 54),
		"github-dark":       application.NewRGB(13, 17, 23),
		"light":             application.NewRGB(250, 250, 250),
		"nord-light":        application.NewRGB(236, 239, 244),
		"catppuccin-latte":  application.NewRGB(239, 241, 245),
		"gruvbox-light":     application.NewRGB(251, 241, 199),
		"rose-pine-dawn":    application.NewRGB(250, 244, 237),
		"solarized-light":   application.NewRGB(253, 246, 227),
		"github-light":      application.NewRGB(255, 255, 255),
		"one-light":         application.NewRGB(250, 250, 250),
		"tokyo-night-light": application.NewRGB(225, 226, 231),
		"everforest-light":  application.NewRGB(253, 246, 221),
	}
	for _, theme := range config.PopupThemes {
		if _, ok := tests[theme]; !ok {
			t.Errorf("TestThemeBaseColour is missing a case for %q", theme)
		}
	}
	for theme, want := range tests {
		if got := themeBaseColour(theme); got != want {
			t.Errorf("themeBaseColour(%q) = %+v, want %+v", theme, got, want)
		}
	}
}

// TestThemeStylesheetCoversEveryPreset keeps the compiled CSS in step with the
// config enum; otherwise a newly added preset would render with the base palette.
func TestThemeStylesheetCoversEveryPreset(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("frontend", "src", "style.css"))
	if err != nil {
		t.Fatal(err)
	}
	css := string(data)
	for _, theme := range config.PopupThemes {
		selector := fmt.Sprintf(`:root[data-theme="%s"]`, theme)
		if !strings.Contains(css, selector) {
			t.Errorf("style.css is missing %s", selector)
		}
	}
}

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
