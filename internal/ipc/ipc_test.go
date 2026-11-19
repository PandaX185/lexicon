package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/PandaX185/lexicon/internal/dictionary"
)

type fakeHandler struct {
	mu        sync.Mutex
	lookups   []string
	reloads   int
	reloadErr error
}

func (h *fakeHandler) Lookup(_ context.Context, text string) *LookupResult {
	h.mu.Lock()
	h.lookups = append(h.lookups, text)
	h.mu.Unlock()
	return &LookupResult{
		Query: text,
		Found: true,
		Entry: &dictionary.Entry{Word: "hello", Definitions: []dictionary.Definition{
			{PartOfSpeech: "interjection", Text: "a greeting"},
		}},
		At: time.Now().UTC(),
	}
}

func (h *fakeHandler) Status() Status {
	return Status{Version: "test", PID: 1, ProtocolVersion: Version}
}

func (h *fakeHandler) Reload(context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reloads++
	return h.reloadErr
}

func startServer(t *testing.T, handler Handler) (*Server, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime", "daemon.sock")
	server, err := NewServer(ServerOptions{Path: path, Handler: handler})
	if err != nil {
		t.Fatalf("new server: %v", err)
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
	return server, path
}

func waitForSubscriber(t *testing.T, server *Server) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for server.SubscriberCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if server.SubscriberCount() == 0 {
		t.Fatal("popup did not subscribe")
	}
}

func TestSecondPopupDismissesFirst(t *testing.T) {
	handler := &fakeHandler{}
	server, path := startServer(t, handler)
	ctx := context.Background()

	first, err := Dial(ctx, path, RolePopup)
	if err != nil {
		t.Fatalf("dial first popup: %v", err)
	}
	defer func() { _ = first.Close() }()
	waitForSubscriber(t, server)

	second, err := Dial(ctx, path, RolePopup)
	if err != nil {
		t.Fatalf("dial second popup: %v", err)
	}
	defer func() { _ = second.Close() }()

	select {
	case _, ok := <-first.DismissEvents():
		if !ok {
			t.Fatal("first popup disconnected instead of receiving a dismiss")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first popup was not dismissed by the second popup")
	}

	select {
	case _, ok := <-second.DismissEvents():
		if ok {
			t.Fatal("second popup dismissed itself")
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func TestLookupBroadcastAndStatus(t *testing.T) {
	handler := &fakeHandler{}
	server, path := startServer(t, handler)
	ctx := context.Background()

	popup, err := Dial(ctx, path, RolePopup)
	if err != nil {
		t.Fatalf("dial popup: %v", err)
	}
	defer func() { _ = popup.Close() }()
	waitForSubscriber(t, server)

	cli, err := Dial(ctx, path, RoleCLI)
	if err != nil {
		t.Fatalf("dial cli: %v", err)
	}
	defer func() { _ = cli.Close() }()

	result, err := cli.Lookup(ctx, "hello")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !result.Found || result.Entry == nil || result.Entry.Word != "hello" {
		t.Fatalf("unexpected result: %+v", result)
	}

	select {
	case event := <-popup.Events():
		if !event.Found || event.Entry == nil || event.Entry.Word != "hello" {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("popup did not receive the lookup event")
	}

	status, err := cli.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.ProtocolVersion != Version || status.Subscribers != 1 {
		t.Fatalf("unexpected status: %+v", status)
	}

	if err := cli.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if err := cli.Reload(ctx); err != nil {
		t.Fatalf("reload: %v", err)
	}
	handler.mu.Lock()
	reloads := handler.reloads
	handler.mu.Unlock()
	if reloads != 1 {
		t.Fatalf("reloads = %d, want 1", reloads)
	}
}

func TestLookupWithoutBroadcastStillUpdatesReplay(t *testing.T) {
	handler := &fakeHandler{}
	_, path := startServer(t, handler)
	ctx := context.Background()

	popup, err := Dial(ctx, path, RolePopup)
	if err != nil {
		t.Fatalf("dial popup: %v", err)
	}
	defer func() { _ = popup.Close() }()

	cli, err := Dial(ctx, path, RoleCLI)
	if err != nil {
		t.Fatalf("dial cli: %v", err)
	}
	defer func() { _ = cli.Close() }()

	result, err := cli.LookupNoBroadcast(ctx, "hello")
	if err != nil {
		t.Fatalf("lookup without broadcast: %v", err)
	}
	if result.Query != "hello" {
		t.Fatalf("query = %q, want hello", result.Query)
	}
	select {
	case event := <-popup.Events():
		t.Fatalf("popup received broadcast for silent lookup: %+v", event)
	case <-time.After(200 * time.Millisecond):
	}

	// The silent lookup must still replace the daemon's replayable result.
	if err := popup.Close(); err != nil {
		t.Fatalf("close first popup: %v", err)
	}
	replayPopup, err := Dial(ctx, path, RolePopup)
	if err != nil {
		t.Fatalf("dial replay popup: %v", err)
	}
	defer func() { _ = replayPopup.Close() }()
	select {
	case event := <-replayPopup.Events():
		if event.Query != "hello" {
			t.Fatalf("replay query = %q, want hello", event.Query)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("silent lookup was not replayed to a new popup")
	}
}

func TestPopupReceivesReplay(t *testing.T) {
	_, path := startServer(t, &fakeHandler{})
	ctx := context.Background()

	cli, err := Dial(ctx, path, RoleCLI)
	if err != nil {
		t.Fatalf("dial cli: %v", err)
	}
	defer func() { _ = cli.Close() }()
	if _, err := cli.Lookup(ctx, "hello"); err != nil {
		t.Fatalf("lookup: %v", err)
	}

	popup, err := Dial(ctx, path, RolePopup)
	if err != nil {
		t.Fatalf("dial popup: %v", err)
	}
	defer func() { _ = popup.Close() }()

	select {
	case event := <-popup.Events():
		if !event.Found || event.Entry == nil || event.Entry.Word != "hello" {
			t.Fatalf("unexpected replay: %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("popup did not receive the replay")
	}
}

func TestWelcomeAdvertisesReplayableResult(t *testing.T) {
	_, path := startServer(t, &fakeHandler{})
	ctx := context.Background()

	first, err := Dial(ctx, path, RolePopup)
	if err != nil {
		t.Fatalf("dial first popup: %v", err)
	}
	defer func() { _ = first.Close() }()
	if first.HasLastResult() {
		t.Fatal("fresh daemon reported a replayable result")
	}

	cli, err := Dial(ctx, path, RoleCLI)
	if err != nil {
		t.Fatalf("dial cli: %v", err)
	}
	defer func() { _ = cli.Close() }()
	if _, err := cli.Lookup(ctx, "hello"); err != nil {
		t.Fatalf("lookup: %v", err)
	}

	second, err := Dial(ctx, path, RolePopup)
	if err != nil {
		t.Fatalf("dial second popup: %v", err)
	}
	defer func() { _ = second.Close() }()
	if !second.HasLastResult() {
		t.Fatal("daemon did not advertise its replayable result")
	}
}

func TestReloadErrorIsReported(t *testing.T) {
	handler := &fakeHandler{reloadErr: errors.New("bad config")}
	_, path := startServer(t, handler)
	cli, err := Dial(context.Background(), path, RoleCLI)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = cli.Close() }()

	err = cli.Reload(context.Background())
	var protocolError *ProtocolError
	if !errors.As(err, &protocolError) {
		t.Fatalf("error = %v, want ProtocolError", err)
	}
	if protocolError.Code != "reload" {
		t.Fatalf("code = %q", protocolError.Code)
	}
}

func TestHandshakeVersionMismatch(t *testing.T) {
	_, path := startServer(t, &fakeHandler{})
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := fmt.Fprintf(conn, `{"v":99,"type":"hello","role":"cli"}`+"\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	var message Message
	if err := json.NewDecoder(conn).Decode(&message); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if message.Type != TypeError || message.Error == nil || message.Error.Code != "handshake" {
		t.Fatalf("unexpected response: %+v", message)
	}
}

func TestSingleInstance(t *testing.T) {
	server, path := startServer(t, &fakeHandler{})
	if server.SubscriberCount() != 0 {
		t.Fatal("expected no subscribers")
	}
	if _, err := NewServer(ServerOptions{Path: path, Handler: &fakeHandler{}}); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("error = %v, want ErrAlreadyRunning", err)
	}
}

func TestOversizedFrameIsRejected(t *testing.T) {
	_, path := startServer(t, &fakeHandler{})
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := fmt.Fprintf(conn, `{"v":1,"type":"hello","role":"cli"}`+"\n"); err != nil {
		t.Fatalf("hello: %v", err)
	}
	reader := newFrameReader(conn)
	if _, err := reader.read(); err != nil {
		t.Fatalf("welcome: %v", err)
	}
	if _, err := fmt.Fprintf(conn, `{"v":1,"type":"lookup","text":"%s"}`+"\n", string(make([]byte, MaxFrame))); err != nil {
		t.Fatalf("write oversized frame: %v", err)
	}
	if _, err := reader.read(); err == nil {
		t.Fatal("expected the connection to be closed")
	}
}
