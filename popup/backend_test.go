package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/PandaX185/lexicon/internal/ipc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type recordingHandler struct {
	mu      sync.Mutex
	lookups []string
}

func (h *recordingHandler) Lookup(_ context.Context, text string) *ipc.LookupResult {
	h.mu.Lock()
	h.lookups = append(h.lookups, text)
	h.mu.Unlock()
	return &ipc.LookupResult{
		Query: text,
		Found: true,
		At:    time.Now().UTC(),
	}
}

func (h *recordingHandler) Status() ipc.Status {
	return ipc.Status{Version: "test", ProtocolVersion: ipc.Version}
}

func (h *recordingHandler) Reload(context.Context) error { return nil }

func (h *recordingHandler) recorded() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.lookups...)
}

func startIPCServer(t *testing.T, handler ipc.Handler) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "daemon.sock")
	server, err := ipc.NewServer(ipc.ServerOptions{Path: path, Handler: handler})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})
	return path
}

func TestStartupLookupReadsSelection(t *testing.T) {
	handler := &recordingHandler{}
	path := startIPCServer(t, handler)
	ctx := context.Background()

	client, err := ipc.Dial(ctx, path, ipc.RolePopup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	popup := &Popup{
		logger:     newTestLogger(t),
		emitResult: func(ipc.LookupResult) {},
	}
	popup.requestStartupLookup(ctx, client)

	if got := handler.recorded(); len(got) != 1 || got[0] != "" {
		t.Fatalf("lookups = %q, want one empty-text selection lookup", got)
	}
}

func TestManualStartupLookupReadsSelectionDespiteReplayAdvertisement(t *testing.T) {
	handler := &recordingHandler{}
	path := startIPCServer(t, handler)
	ctx := context.Background()

	// Populate the daemon's replay cache with a found result.
	seed, err := ipc.Dial(ctx, path, ipc.RoleCLI)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Lookup(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	_ = seed.Close()

	client, err := ipc.Dial(ctx, path, ipc.RolePopup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if !client.HasLastResult() {
		t.Fatal("test setup: daemon must advertise a replay result")
	}

	popup := &Popup{
		logger:     newTestLogger(t),
		emitResult: func(ipc.LookupResult) {},
	}
	popup.requestStartupLookup(ctx, client)

	if got := handler.recorded(); len(got) != 2 || got[0] != "hello" || got[1] != "" {
		t.Fatalf("lookups = %q, want the seeding lookup followed by an empty-text selection lookup", got)
	}
}

func TestReplayStartupLookupSkipsFreshLookup(t *testing.T) {
	handler := &recordingHandler{}
	path := startIPCServer(t, handler)
	ctx := context.Background()

	seed, err := ipc.Dial(ctx, path, ipc.RoleCLI)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Lookup(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	_ = seed.Close()

	client, err := ipc.Dial(ctx, path, ipc.RolePopup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if !client.HasLastResult() {
		t.Fatal("test setup: daemon must advertise a replay result")
	}

	popup := &Popup{replay: true, logger: newTestLogger(t)}
	popup.requestStartupLookup(ctx, client)

	if got := handler.recorded(); len(got) != 1 || got[0] != "hello" {
		t.Fatalf("lookups = %q, want only the seeding lookup for a replay popup", got)
	}
}

// gatedLookupHandler holds the startup selection lookup open so the test can
// observe events that arrive before the direct reply completes.
type gatedLookupHandler struct {
	recordingHandler
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (h *gatedLookupHandler) Lookup(ctx context.Context, text string) *ipc.LookupResult {
	if text == "" {
		h.once.Do(func() { close(h.entered) })
		select {
		case <-h.release:
		case <-ctx.Done():
		}
	}
	return h.recordingHandler.Lookup(ctx, text)
}

func newReplaySeed(t *testing.T, ctx context.Context, path, query string) {
	t.Helper()
	seed, err := ipc.Dial(ctx, path, ipc.RoleCLI)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = seed.Close() }()
	if _, err := seed.Lookup(ctx, query); err != nil {
		t.Fatal(err)
	}
}

func waitForDisplayedQuery(t *testing.T, shown <-chan ipc.LookupResult, want string) {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case result := <-shown:
			if result.Query == "stale" {
				t.Fatal("stale replay result reached the UI")
			}
			if result.Query == want {
				return
			}
		case <-timeout:
			t.Fatalf("timed out waiting for displayed result %q", want)
		}
	}
}

func TestManualPopupIgnoresReplayEventBeforeStartupCompletes(t *testing.T) {
	handler := &gatedLookupHandler{entered: make(chan struct{}), release: make(chan struct{})}
	path := startIPCServer(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	newReplaySeed(t, ctx, path, "stale")
	client, err := ipc.Dial(ctx, path, ipc.RolePopup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	shown := make(chan ipc.LookupResult, 16)
	popup := &Popup{
		logger:     newTestLogger(t),
		emitResult: func(result ipc.LookupResult) { shown <- result },
	}

	startupDone := make(chan struct{})
	go func() {
		popup.requestStartupLookup(ctx, client)
		close(startupDone)
	}()
	select {
	case <-handler.entered:
	case <-ctx.Done():
		t.Fatal("startup lookup never reached the daemon")
	}

	consumeDone := make(chan struct{})
	go func() {
		popup.consume(ctx, client)
		close(consumeDone)
	}()

	// The replay result is already queued; it must not reach the UI while the
	// fresh selection lookup is still in flight.
	select {
	case result := <-shown:
		t.Fatalf("replay shown before startup completed: query %q", result.Query)
	case <-time.After(100 * time.Millisecond):
	}

	close(handler.release)
	select {
	case <-startupDone:
	case <-ctx.Done():
		t.Fatal("startup lookup did not complete")
	}
	waitForDisplayedQuery(t, shown, "")

	// A real broadcast after startup must flow normally.
	newReplaySeed(t, ctx, path, "fresh")
	waitForDisplayedQuery(t, shown, "fresh")

	cancel()
	_ = client.Close()
	select {
	case <-consumeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("consume did not exit")
	}
}

func TestManualPopupDropsReplayBufferedBeforeConsumeStarts(t *testing.T) {
	handler := &recordingHandler{}
	path := startIPCServer(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	newReplaySeed(t, ctx, path, "stale")
	client, err := ipc.Dial(ctx, path, ipc.RolePopup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	shown := make(chan ipc.LookupResult, 16)
	popup := &Popup{
		logger:     newTestLogger(t),
		emitResult: func(result ipc.LookupResult) { shown <- result },
	}

	// Production ordering: the startup lookup finishes before consume starts,
	// so the stale replay is already buffered on the events channel.
	popup.requestStartupLookup(ctx, client)
	waitForDisplayedQuery(t, shown, "")

	consumeDone := make(chan struct{})
	go func() {
		popup.consume(ctx, client)
		close(consumeDone)
	}()

	newReplaySeed(t, ctx, path, "fresh")
	waitForDisplayedQuery(t, shown, "fresh")

	cancel()
	_ = client.Close()
	select {
	case <-consumeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("consume did not exit")
	}
}

type pairIPCHandler struct {
	recordingHandler
	state        ipc.LanguagePairsState
	pairRequests [][2]string
}

func (h *pairIPCHandler) LanguagePairs() ipc.LanguagePairsState { return h.state }

func (h *pairIPCHandler) SetLanguagePair(_ context.Context, source, target string) (ipc.LanguagePairsState, error) {
	h.pairRequests = append(h.pairRequests, [2]string{source, target})
	h.state.Source = source
	h.state.Target = target
	return h.state, nil
}

func TestSetLanguagePairEventCarriesRequestedPair(t *testing.T) {
	if eventSetLanguagePair != "lexicon:set-language-pair" {
		t.Fatalf("eventSetLanguagePair = %q, want lexicon:set-language-pair", eventSetLanguagePair)
	}
	events := []*application.CustomEvent{
		{Name: eventSetLanguagePair, Data: json.RawMessage(`{"source":"en","target":"fr"}`)},
		{Name: eventSetLanguagePair, Data: map[string]any{"source": "en", "target": "fr"}},
	}
	for _, event := range events {
		var request setLanguagePairRequest
		if err := decodeEventData(event, &request); err != nil {
			t.Fatalf("decodeEventData(%T): %v", event.Data, err)
		}
		if request.Source != "en" || request.Target != "fr" {
			t.Fatalf("request = %+v, want en/fr", request)
		}
	}
}

func TestShouldDismissOnBlurGuard(t *testing.T) {
	shownAt := time.Date(2026, time.November, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		shownAt time.Time
		now     time.Time
		want    bool
	}{
		{"never shown", time.Time{}, shownAt.Add(time.Hour), false},
		{"inside guard", shownAt, shownAt.Add(blurGuard - time.Millisecond), false},
		{"at guard boundary", shownAt, shownAt.Add(blurGuard), false},
		{"after guard", shownAt, shownAt.Add(blurGuard + time.Millisecond), true},
		{"later focus loss", shownAt, shownAt.Add(time.Hour), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldDismissOnBlur(tt.shownAt, tt.now); got != tt.want {
				t.Fatalf("shouldDismissOnBlur = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldDismissOnBlurIsRaceFree(t *testing.T) {
	popup := &Popup{shownAt: time.Now().Add(-time.Hour)}

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			popup.mu.Lock()
			popup.shownAt = time.Now()
			popup.mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			_ = popup.shouldDismissOnBlur(time.Now())
		}()
	}
	wg.Wait()

	// A focused in-webview control is not tracked: after the guard, any focus
	// loss dismisses the popup.
	if !popup.shouldDismissOnBlur(time.Now().Add(time.Hour)) {
		t.Fatal("focus loss after the guard must dismiss")
	}
}

func TestChangeLanguagePairRelooksUpLastQuery(t *testing.T) {
	handler := &pairIPCHandler{state: ipc.LanguagePairsState{
		Enabled: true,
		Source:  "en",
		Target:  "de",
		Pairs: []ipc.LanguagePair{
			{Source: "en", Target: "de", PackID: "en-de"},
			{Source: "en", Target: "fr", PackID: "en-fr"},
		},
	}}
	path := startIPCServer(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := ipc.Dial(ctx, path, ipc.RolePopup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	popup := &Popup{logger: newTestLogger(t)}
	popup.last = &ipc.LookupResult{Query: "hallo", Found: true}

	state, result, err := popup.changeLanguagePair(ctx, client, setLanguagePairRequest{Source: "en", Target: "fr"})
	if err != nil {
		t.Fatalf("changeLanguagePair: %v", err)
	}
	if state.Source != "en" || state.Target != "fr" {
		t.Fatalf("state = %+v", state)
	}
	if got := handler.pairRequests; len(got) != 1 || got[0] != [2]string{"en", "fr"} {
		t.Fatalf("pair requests = %q, want en/fr", got)
	}
	if result == nil || result.Query != "hallo" {
		t.Fatalf("relookup result = %+v", result)
	}
	got := handler.recorded()
	if len(got) != 1 || got[0] != "hallo" {
		t.Fatalf("lookups = %q, want last query relookup", got)
	}
}
