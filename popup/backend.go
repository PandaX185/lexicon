package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/ipc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	eventLookup          = "lexicon:lookup"
	eventConnection      = "lexicon:connection"
	eventReady           = "lexicon:ready"
	eventSettings        = "lexicon:settings"
	eventLanguagePairs   = "lexicon:language_pairs"
	eventPairError       = "lexicon:pair_error"
	eventSetLanguagePair = "lexicon:set-language-pair"

	dialRetryDelay  = 1500 * time.Millisecond
	maxDialFailures = 12
	idleShowDelay   = 900 * time.Millisecond

	// blurGuard gives the webview time to settle after Show/Focus before a
	// focus-loss event can dismiss a freshly opened popup.
	blurGuard = 350 * time.Millisecond
)

// connectionState is sent to the frontend so it can distinguish an empty
// result from a daemon that is unreachable.
type connectionState struct {
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`
}

type setLanguagePairRequest struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type pairErrorMessage struct {
	Message string `json:"message"`
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
	quitter           popupQuitter
	socket            string
	showPronunciation bool
	display           displaySettings
	logger            *slog.Logger
	// replay marks a daemon-launched popup, which should display the result
	// already waiting on the socket instead of reading PRIMARY again.
	replay      bool
	startupDone atomic.Bool
	// emitResult, emitPairs, and showWindow are test seams for observing
	// frontend events and visibility without a running Wails application.
	// Production leaves them nil.
	emitResult func(ipc.LookupResult)
	emitPairs  func(ipc.LanguagePairsState)
	showWindow func()

	mu         sync.Mutex
	last       *ipc.LookupResult
	connection connectionState
	shownAt    time.Time

	clientMu  sync.Mutex
	client    *ipc.Client
	pairMu    sync.Mutex
	pairState *ipc.LanguagePairsState
}

func newPopup(app *application.App, window *application.WebviewWindow, settings settings, logger *slog.Logger, replay bool) *Popup {
	if logger == nil {
		logger = slog.Default()
	}
	return &Popup{
		app:               app,
		window:            window,
		quitter:           app,
		socket:            settings.socket,
		showPronunciation: settings.showPronunciation,
		display: displaySettings{
			Theme:      settings.theme,
			ShowHeader: settings.showHeader,
			ShowStatus: settings.showStatus,
			ShowFooter: settings.showFooter,
		},
		logger: logger.With("component", "popup"),
		replay: replay,
	}
}

func (p *Popup) setClient(client *ipc.Client) {
	p.clientMu.Lock()
	p.client = client
	p.clientMu.Unlock()
}

func (p *Popup) currentClient() *ipc.Client {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()
	return p.client
}

func (p *Popup) publishPairs(state ipc.LanguagePairsState) {
	copied := state
	p.pairMu.Lock()
	p.pairState = &copied
	p.pairMu.Unlock()
	if p.emitPairs != nil {
		p.emitPairs(copied)
		return
	}
	p.app.Event.Emit(eventLanguagePairs, copied)
}

func (p *Popup) pairErrorMessage(message string) {
	p.app.Event.Emit(eventPairError, pairErrorMessage{Message: message})
}

func decodeEventData(event *application.CustomEvent, target any) error {
	switch value := event.Data.(type) {
	case json.RawMessage:
		return json.Unmarshal(value, target)
	case []byte:
		return json.Unmarshal(value, target)
	case string:
		return json.Unmarshal([]byte(value), target)
	case map[string]any:
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return json.Unmarshal(encoded, target)
	default:
		return fmt.Errorf("unexpected event data %T", event.Data)
	}
}

func (p *Popup) lastQuery() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.last == nil {
		return ""
	}
	if strings.TrimSpace(p.last.Normalized) != "" {
		return p.last.Normalized
	}
	return p.last.Query
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
		p.pairMu.Lock()
		pairState := p.pairState
		p.pairMu.Unlock()
		if pairState != nil {
			p.app.Event.Emit(eventLanguagePairs, *pairState)
		}
	})
	p.app.Event.On(eventSetLanguagePair, func(event *application.CustomEvent) {
		var request setLanguagePairRequest
		if err := decodeEventData(event, &request); err != nil {
			p.pairErrorMessage("invalid language-pair request: " + err.Error())
			return
		}
		client := p.currentClient()
		if client == nil {
			p.pairErrorMessage("The Lexicon daemon is not connected.")
			return
		}
		if err := p.applyLanguagePairChange(ctx, client, request); err != nil {
			if refreshed, refreshErr := client.LanguagePairs(ctx); refreshErr == nil {
				p.publishPairs(*refreshed)
			}
			p.pairErrorMessage(err.Error())
			return
		}
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
				dismissPopup(p.app, nil)
				return
			}
			if !sleepContext(ctx, dialRetryDelay) {
				return
			}
			continue
		}

		failures = 0
		p.setClient(client)
		p.setConnection(true, "")
		if state, err := client.LanguagePairs(ctx); err == nil {
			copied := *state
			p.pairMu.Lock()
			p.pairState = &copied
			p.pairMu.Unlock()
		} else {
			var protocolErr *ipc.ProtocolError
			if errors.As(err, &protocolErr) && (protocolErr.Code == "unknown_type" || protocolErr.Code == "unsupported") {
				p.pairErrorMessage("This popup needs a newer Lexicon daemon. Update Lexicon to use the language pair controls.")
			}
		}
		// Each connection gets its own startup window: a reconnect must not
		// treat results replayed on the new socket as already validated.
		p.startupDone.Store(false)
		p.requestStartupLookup(ctx, client)
		p.consume(ctx, client)
		p.setClient(nil)
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
// lookup arrives, so subscribing alone is not enough. A daemon-launched popup
// (replay=true) already has the triggering result on the socket and skips this
// lookup, avoiding a duplicate read and translation.
//
// The direct reply is displayed here rather than relying on the broadcast, so
// the fresh selection is shown even when its queued event is dropped below.
func (p *Popup) requestStartupLookup(ctx context.Context, client *ipc.Client) {
	if p.replay && client.HasLastResult() {
		return
	}
	result, err := client.Lookup(ctx, "")
	if err != nil {
		p.logger.Debug("startup selection lookup failed", "error", err)
	}
	if !p.replay {
		p.discardReplayEvents(client)
	}
	p.startupDone.Store(true)
	if err == nil && result != nil {
		p.handleResult(*result)
	}
}

// discardReplayEvents drops lookup events queued before the direct startup
// reply. The server writes the subscribe replay before it can write the reply
// on the same connection, so a stale result is already buffered by the time
// Lookup returns. No lock is held while reading the channel.
func (p *Popup) discardReplayEvents(client *ipc.Client) {
	for {
		select {
		case _, ok := <-client.Events():
			if !ok {
				return
			}
		default:
			return
		}
	}
}

// consume forwards lookup events until the subscription ends. The events
// channel is closed after the reader loop drains buffered results, so no
// result is lost on a clean disconnect.
func (p *Popup) consume(ctx context.Context, client *ipc.Client) {
	dismiss := client.DismissEvents()
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-client.Events():
			if !ok {
				return
			}
			// A manual popup must not display the replay sent on subscribe:
			// it can contain a selection from before this popup existed.
			if !p.replay && !p.startupDone.Load() {
				continue
			}
			p.handleResult(result)
		case state, ok := <-client.LanguagePairEvents():
			if !ok {
				return
			}
			p.publishPairs(state)
		case _, ok := <-dismiss:
			if !ok {
				dismiss = nil
				continue
			}
			// A newer popup took over. Quit the entire process so the old
			// window cannot linger behind the replacement.
			dismissPopup(p.quitter, nil)
			return
		}
	}
}

// updateResult publishes a lookup result to the frontend without changing
// window visibility. Pair changes use it so the translation updates in place.
func (p *Popup) updateResult(result ipc.LookupResult) {
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

	if p.emitResult != nil {
		p.emitResult(result)
		return
	}
	p.app.Event.Emit(eventLookup, result)
}

func (p *Popup) handleResult(result ipc.LookupResult) {
	p.updateResult(result)
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
	p.mu.Lock()
	p.shownAt = time.Now()
	p.mu.Unlock()
	if p.showWindow != nil {
		p.showWindow()
		return
	}
	p.window.Show()
	p.window.Focus()
}

// shouldDismissOnBlur is pure so the early-blur guard can be tested without a
// window. A zero shownAt means the popup was never displayed. The language-pair
// control lives in the webview, so losing the whole window's focus is always a
// dismissal after the guard, regardless of DOM focus.
func shouldDismissOnBlur(shownAt, now time.Time) bool {
	return !shownAt.IsZero() && now.Sub(shownAt) > blurGuard
}

func (p *Popup) shouldDismissOnBlur(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return shouldDismissOnBlur(p.shownAt, now)
}

// dismissOnBlur is called by the WindowLostFocus hook. The guard prevents an
// immediate blur during show from closing the popup.
func (p *Popup) dismissOnBlur() {
	if p.shouldDismissOnBlur(time.Now()) {
		dismissPopup(p.quitter, nil)
	}
}

func (p *Popup) changeLanguagePair(ctx context.Context, client *ipc.Client, request setLanguagePairRequest) (ipc.LanguagePairsState, *ipc.LookupResult, error) {
	state, err := client.SetLanguagePair(ctx, request.Source, request.Target)
	if err != nil {
		return ipc.LanguagePairsState{}, nil, err
	}
	var relookup *ipc.LookupResult
	if query := p.lastQuery(); query != "" {
		result, lookupErr := client.LookupNoBroadcast(ctx, query)
		if lookupErr != nil {
			return ipc.LanguagePairsState{}, nil, fmt.Errorf("language pair changed, but its translation could not be refreshed: %w", lookupErr)
		}
		relookup = result
	}
	return *state, relookup, nil
}

// applyLanguagePairChange publishes the new pair and refreshes the visible
// result in place. It deliberately avoids show(), which is only for a new
// selection lookup or an idle window.
func (p *Popup) applyLanguagePairChange(ctx context.Context, client *ipc.Client, request setLanguagePairRequest) error {
	state, relookup, err := p.changeLanguagePair(ctx, client, request)
	if err != nil {
		return err
	}
	p.publishPairs(state)
	if relookup != nil {
		p.updateResult(*relookup)
	}
	return nil
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
