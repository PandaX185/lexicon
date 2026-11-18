package ipc

import (
	"context"
	"errors"
	"testing"
	"time"
)

type pairHandler struct {
	fakeHandler
	state LanguagePairsState
}

func (h *pairHandler) LanguagePairs() LanguagePairsState { return h.state }

func (h *pairHandler) SetLanguagePair(_ context.Context, source, target string) (LanguagePairsState, error) {
	h.state.Source = source
	h.state.Target = target
	return h.state, nil
}

func TestLanguagePairRoundTrip(t *testing.T) {
	handler := &pairHandler{state: LanguagePairsState{
		Enabled: true,
		Source:  "en",
		Target:  "de",
		Pairs:   []LanguagePair{{Source: "en", Target: "de", PackID: "bergamot-en-de-tiny11"}},
	}}
	_, path := startServer(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	popup, err := Dial(ctx, path, RolePopup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = popup.Close() }()

	state, err := popup.LanguagePairs(ctx)
	if err != nil {
		t.Fatalf("LanguagePairs: %v", err)
	}
	if len(state.Pairs) != 1 || state.Pairs[0].PackID != "bergamot-en-de-tiny11" {
		t.Fatalf("state = %+v", state)
	}

	updated, err := popup.SetLanguagePair(ctx, "de", "en")
	if err != nil {
		t.Fatalf("SetLanguagePair: %v", err)
	}
	if updated.Source != "de" || updated.Target != "en" {
		t.Fatalf("updated = %+v", updated)
	}

	select {
	case event := <-popup.LanguagePairEvents():
		if event.Source != "de" || event.Target != "en" {
			t.Fatalf("event = %+v", event)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for pair event")
	}
}

func TestSetLanguagePairRejectsCLI(t *testing.T) {
	handler := &pairHandler{}
	_, path := startServer(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cli, err := Dial(ctx, path, RoleCLI)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cli.Close() }()

	_, err = cli.SetLanguagePair(ctx, "en", "de")
	var protocolError *ProtocolError
	if !errors.As(err, &protocolError) || protocolError.Code != "role" {
		t.Fatalf("error = %v, want protocol role error", err)
	}
}
