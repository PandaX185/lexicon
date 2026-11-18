package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/ipc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	eventLookup     = "lexicon:lookup"
	eventConnection = "lexicon:connection"
	eventReady      = "lexicon:ready"
	eventHide       = "lexicon:hide"
	eventSettings   = "lexicon:settings"

	dialRetryDelay  = 1500 * time.Millisecond
	maxDialFailures = 12
	idleShowDelay   = 900 * time.Millisecond
	blurGuard       = 350 * time.Millisecond
)

// connectionState is sent to the frontend so it can distinguish an empty
// result from a daemon that is unreachable.
type connectionState struct {
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`
}

// displaySettings carries the window appearance to the frontend. The values
// come from config at popup start; there is no runtime settings screen.
type displaySettings struct {
	Theme      string `json:"theme"`
	ShowHeader bool   `json:"show_header"`
	ShowStatus bool   `json:"show_status"`
	ShowFooter bool   `json:"show_footer"`
}

// Popup owns the IPC subscription and the Wails window. The daemon launches it
// with the socket path already resolved, so it stays a thin display process.
type Popup struct {
	app               *application.App
	window            *application.WebviewWindow
	socket            string
	blurHides         bool
	showPronunciation bool
	display           displaySettings
	logger            *slog.Logger

	mu         sync.Mutex
	last       *ipc.LookupResult
	connection connectionState
	shownAt    time.Time
}

func newPopup(app *application.App, window *application.WebviewWindow, settings settings, logger *slog.Logger) *Popup {
	if logger == nil {
		logger = slog.Default()
	}
	return &Popup{
		app:               app,
		window:            window,
		socket:            settings.socket,
		blurHides:         settings.hideOnBlur,
		showPronunciation: settings.showPronunciation,
		display: displaySettings{
			Theme:      settings.theme,
			ShowHeader: settings.showHeader,
			ShowStatus: settings.showStatus,
			ShowFooter: settings.showFooter,
		},
		logger: logger.With("component", "popup"),
	}
}

// start wires frontend requests and begins subscribing. It is called once the
// Wails application has started, so emitting events is safe.
func (p *Popup) start(ctx context.Context) {
	p.app.Event.On(eventReady, func(*application.CustomEvent) {
		p.mu.Lock()
		state := p.connection
		var last *ipc.LookupResult
		if p.last != nil {
			copied := *p.last
			last = &copied
		}
		p.mu.Unlock()

		p.app.Event.Emit(eventSettings, p.display)
		p.app.Event.Emit(eventConnection, state)
		if last != nil {
			p.app.Event.Emit(eventLookup, *last)
		}
	})
	p.app.Event.On(eventHide, func(*application.CustomEvent) {
		p.hide()
	})

	go p.connectLoop(ctx)
	go p.showWhenIdle(ctx)
}

func (p *Popup) connectLoop(ctx context.Context) {
	failures := 0
	for ctx.Err() == nil {
		client, err := ipc.Dial(ctx, p.socket, ipc.RolePopup)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			failures++
			p.setConnection(false, err.Error())
			if failures >= maxDialFailures {
				p.logger.Warn("giving up on the daemon", "socket", p.socket, "attempts", failures)
				p.app.Quit()
				return
			}
			if !sleepContext(ctx, dialRetryDelay) {
				return
			}
			continue
		}

		failures = 0
		p.setConnection(true, "")
		requestStartupLookup(ctx, client, p.logger)
		p.consume(ctx, client)
		_ = client.Close()
		if ctx.Err() != nil {
			return
		}
		p.setConnection(false, "The Lexicon daemon disconnected.")
		if !sleepContext(ctx, 500*time.Millisecond) {
			return
		}
	}
}

// requestStartupLookup makes a manually launched popup immediately show the
// current Wayland primary selection. The daemon only reads PRIMARY when a
// lookup arrives, so subscribing alone is not enough. When it already has a
// result to replay (the auto-launch path), skip this to avoid looking up and
// translating the same word twice.
func requestStartupLookup(ctx context.Context, client *ipc.Client, logger *slog.Logger) {
	if client.HasLastResult() {
		return
	}
	if _, err := client.Lookup(ctx, ""); err != nil {
		logger.Debug("startup selection lookup failed", "error", err)
	}
}

// consume forwards lookup events until the subscription ends. The events
// channel is closed after the reader loop drains buffered results, so no
// result is lost on a clean disconnect.
func (p *Popup) consume(ctx context.Context, client *ipc.Client) {
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-client.Events():
			if !ok {
				return
			}
			p.handleResult(result)
		}
	}
}

func (p *Popup) handleResult(result ipc.LookupResult) {
	if !p.showPronunciation && result.Entry != nil {
		// Copy before stripping so a replayed result keeps its original data.
		entry := *result.Entry
		entry.Pronunciation = dictionary.Pronunciation{}
		result.Entry = &entry
	}

	p.mu.Lock()
	copied := result
	p.last = &copied
	p.mu.Unlock()

	p.app.Event.Emit(eventLookup, result)
	p.show()
}

func (p *Popup) setConnection(connected bool, message string) {
	state := connectionState{Connected: connected, Error: message}
	p.mu.Lock()
	changed := p.connection != state
	p.connection = state
	p.mu.Unlock()
	if changed {
		p.app.Event.Emit(eventConnection, state)
	}
}

func (p *Popup) show() {
	p.window.Show()
	p.window.Focus()
	p.mu.Lock()
	p.shownAt = time.Now()
	p.mu.Unlock()
}

func (p *Popup) hide() {
	p.mu.Lock()
	p.shownAt = time.Time{}
	p.mu.Unlock()
	p.window.Hide()
}

// hideOnBlur closes the popup when the user clicks elsewhere. The short guard
// avoids hiding while the window is still acquiring focus on show.
func (p *Popup) hideOnBlur() {
	if !p.blurHides {
		return
	}
	p.mu.Lock()
	shownAt := p.shownAt
	p.mu.Unlock()
	if !shownAt.IsZero() && time.Since(shownAt) > blurGuard {
		p.hide()
	}
}

// showWhenIdle makes a manually launched popup visible even when the daemon
// has no result to replay. Daemon-launched popups receive the replay first and
// skip this path.
func (p *Popup) showWhenIdle(ctx context.Context) {
	timer := time.NewTimer(idleShowDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		p.mu.Lock()
		hasResult := p.last != nil
		p.mu.Unlock()
		if !hasResult {
			p.show()
		}
	}
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
