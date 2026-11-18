package main

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/PandaX185/lexicon/internal/ipc"
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

	requestStartupLookup(ctx, client, newTestLogger(t))

	if got := handler.recorded(); len(got) != 1 || got[0] != "" {
		t.Fatalf("lookups = %q, want one empty-text selection lookup", got)
	}
}

func TestStartupLookupSkippedWhenDaemonHasReplay(t *testing.T) {
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

	requestStartupLookup(ctx, client, newTestLogger(t))

	if got := handler.recorded(); len(got) != 1 {
		t.Fatalf("lookups = %q, want only the seeding lookup", got)
	}
}

type pairIPCHandler struct {
	recordingHandler
	state ipc.LanguagePairsState
}

func (h *pairIPCHandler) LanguagePairs() ipc.LanguagePairsState { return h.state }

func (h *pairIPCHandler) SetLanguagePair(_ context.Context, source, target string) (ipc.LanguagePairsState, error) {
	h.state.Source = source
	h.state.Target = target
	return h.state, nil
}

func TestChangeLanguagePairRelooksUpLastQuery(t *testing.T) {
	handler := &pairIPCHandler{state: ipc.LanguagePairsState{
		Enabled: true,
		Source:  "en",
		Target:  "de",
		Pairs:   []ipc.LanguagePair{{Source: "en", Target: "de", PackID: "en-de"}},
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

	state, result, err := popup.changeLanguagePair(ctx, client, setLanguagePairRequest{Source: "en", Target: "de"})
	if err != nil {
		t.Fatalf("changeLanguagePair: %v", err)
	}
	if state.Source != "en" || state.Target != "de" {
		t.Fatalf("state = %+v", state)
	}
	if result == nil || result.Query != "hallo" {
		t.Fatalf("relookup result = %+v", result)
	}
	got := handler.recorded()
	if len(got) != 1 || got[0] != "hallo" {
		t.Fatalf("lookups = %q, want last query relookup", got)
	}
}

func TestBlurIsSuppressedWhileSelectorOpen(t *testing.T) {
	now := time.Now()
	if !shouldHideOnBlur(true, now.Add(-time.Second), now, false) {
		t.Fatal("expected normal blur to hide")
	}
	if shouldHideOnBlur(true, now.Add(-time.Second), now, true) {
		t.Fatal("selector must suppress blur hide")
	}
}
