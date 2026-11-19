package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
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
	socket            string
	showPronunciation bool
	display           displaySettings
	logger            *slog.Logger

	mu         sync.Mutex
	last       *ipc.LookupResult
	connection connectionState

	clientMu  sync.Mutex
	client    *ipc.Client
	pairMu    sync.Mutex
	pairState *ipc.LanguagePairsState
}

func newPopup(app *application.App, window *application.WebviewWindow, settings settings, logger *slog.Logger) *Popup {
	if logger == nil {
		logger = slog.Default()
	}
	return &Popup{
		app:               app,
		window:            window,
		socket:            settings.socket,
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
		state, relookup, err := p.changeLanguagePair(ctx, client, request)
		if err != nil {
			if refreshed, refreshErr := client.LanguagePairs(ctx); refreshErr == nil {
				p.publishPairs(*refreshed)
			}
			p.pairErrorMessage(err.Error())
			return
		}
		p.publishPairs(state)
		if relookup != nil {
			p.handleResult(*relookup)
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
		requestStartupLookup(ctx, client, p.logger)
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
		case state, ok := <-client.LanguagePairEvents():
			if !ok {
				return
			}
			p.publishPairs(state)
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
}

func (p *Popup) changeLanguagePair(ctx context.Context, client *ipc.Client, request setLanguagePairRequest) (ipc.LanguagePairsState, *ipc.LookupResult, error) {
	state, err := client.SetLanguagePair(ctx, request.Source, request.Target)
	if err != nil {
		return ipc.LanguagePairsState{}, nil, err
	}
	var relookup *ipc.LookupResult
	if query := p.lastQuery(); query != "" {
		result, lookupErr := client.Lookup(ctx, query)
		if lookupErr == nil {
			relookup = result
		}
	}
	return *state, relookup, nil
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
