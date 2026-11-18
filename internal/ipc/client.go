package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultDialTimeout bounds connecting and handshaking.
const DefaultDialTimeout = 3 * time.Second

// Client is a reconnecting-friendly IPC connection. A Client is safe for
// concurrent requests; it is not reusable after Close.
type Client struct {
	conn    net.Conn
	role    Role
	writeMu sync.Mutex
	nextID  atomic.Uint64

	pendingMu sync.Mutex
	pending   map[uint64]chan Message

	events             chan LookupResult
	languagePairEvents chan LanguagePairsState
	done               chan struct{}
	closed             atomic.Bool
	hasLastResult      bool
}

// Dial connects and performs the version handshake.
func Dial(ctx context.Context, path string, role Role) (*Client, error) {
	dialer := net.Dialer{Timeout: DefaultDialTimeout}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, err
		default:
			return nil, fmt.Errorf("daemon is not running (%w)", err)
		}
	}

	client := &Client{
		conn:               conn,
		role:               role,
		pending:            map[uint64]chan Message{},
		events:             make(chan LookupResult, 8),
		languagePairEvents: make(chan LanguagePairsState, 4),
		done:               make(chan struct{}),
	}

	_ = conn.SetDeadline(time.Now().Add(DefaultDialTimeout))
	encoder := json.NewEncoder(conn)
	if err := Encode(encoder, Message{Type: TypeHello, Role: role}); err != nil {
		_ = conn.Close()
		return nil, err
	}

	reader := newFrameReader(conn)
	welcome, err := reader.read()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ipc handshake: %w", err)
	}
	if welcome.Version != Version || welcome.Type != TypeWelcome {
		_ = conn.Close()
		if welcome.Error != nil {
			return nil, welcome.Error
		}
		return nil, errors.New("ipc handshake rejected")
	}
	_ = conn.SetDeadline(time.Time{})
	client.hasLastResult = welcome.HasLastResult

	go client.readLoop(reader)
	return client, nil
}

// HasLastResult reports whether the daemon advertised a result it will replay
// to this subscriber. A false value means a newly connected popup should ask
// for the current primary selection itself.
func (c *Client) HasLastResult() bool { return c.hasLastResult }

// Events returns lookup results pushed while subscribed as a popup. The channel
// closes when the connection ends.
func (c *Client) Events() <-chan LookupResult { return c.events }

// Done closes when the connection ends.
func (c *Client) Done() <-chan struct{} { return c.done }

// Close ends the connection.
func (c *Client) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	return c.conn.Close()
}

// Lookup requests a lookup. An empty text asks the daemon to read the primary
// selection. Validation and not-found conditions are reported in the result,
// not as a transport error.
func (c *Client) Lookup(ctx context.Context, text string) (*LookupResult, error) {
	reply, err := c.request(ctx, Message{Type: TypeLookup, Text: text})
	if err != nil {
		return nil, err
	}
	if reply.Result == nil {
		return nil, errors.New("daemon returned an empty lookup result")
	}
	return reply.Result, nil
}

// Status fetches daemon health.
func (c *Client) Status(ctx context.Context) (*Status, error) {
	reply, err := c.request(ctx, Message{Type: TypeStatus})
	if err != nil {
		return nil, err
	}
	if reply.Status == nil {
		return nil, errors.New("daemon returned an empty status")
	}
	return reply.Status, nil
}

// Reload asks the daemon to re-read configuration.
func (c *Client) Reload(ctx context.Context) error {
	_, err := c.request(ctx, Message{Type: TypeReload})
	return err
}

// LanguagePairs returns the installed translation pairs and the active pair.
func (c *Client) LanguagePairs(ctx context.Context) (*LanguagePairsState, error) {
	reply, err := c.request(ctx, Message{Type: TypeLanguagePairs})
	if err != nil {
		return nil, err
	}
	if reply.LanguagePairs == nil {
		return nil, errors.New("daemon returned an empty language-pair state")
	}
	return reply.LanguagePairs, nil
}

// SetLanguagePair persists a new active pair and returns the resulting state.
func (c *Client) SetLanguagePair(ctx context.Context, source, target string) (*LanguagePairsState, error) {
	reply, err := c.request(ctx, Message{
		Type:           TypeSetLanguagePair,
		SourceLanguage: source,
		TargetLanguage: target,
	})
	if err != nil {
		return nil, err
	}
	if reply.LanguagePairs == nil {
		return nil, errors.New("daemon returned an empty language-pair state")
	}
	return reply.LanguagePairs, nil
}

// LanguagePairEvents receives broadcasts after successful pair changes.
func (c *Client) LanguagePairEvents() <-chan LanguagePairsState { return c.languagePairEvents }

// Ping verifies liveness.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.request(ctx, Message{Type: TypePing})
	return err
}

func (c *Client) request(ctx context.Context, message Message) (Message, error) {
	if c.closed.Load() {
		return Message{}, ErrClosed
	}
	id := c.nextID.Add(1)
	message.ID = id
	message.Version = Version

	waiter := make(chan Message, 1)
	c.pendingMu.Lock()
	c.pending[id] = waiter
	c.pendingMu.Unlock()

	deadline := time.Now().Add(15 * time.Second)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	c.writeMu.Lock()
	_ = c.conn.SetWriteDeadline(deadline)
	err := Encode(json.NewEncoder(c.conn), message)
	c.writeMu.Unlock()
	if err != nil {
		c.dropPending(id)
		return Message{}, fmt.Errorf("send ipc request: %w", err)
	}

	select {
	case reply := <-waiter:
		if reply.Error != nil {
			return Message{}, reply.Error
		}
		return reply, nil
	case <-ctx.Done():
		c.dropPending(id)
		return Message{}, ctx.Err()
	case <-c.done:
		return Message{}, ErrClosed
	}
}

func (c *Client) dropPending(id uint64) {
	c.pendingMu.Lock()
	delete(c.pending, id)
	c.pendingMu.Unlock()
}

func (c *Client) readLoop(reader *frameReader) {
	defer func() {
		if c.closed.CompareAndSwap(false, true) {
			_ = c.conn.Close()
		}
		close(c.done)
		close(c.events)
		close(c.languagePairEvents)
	}()

	for {
		message, err := reader.read()
		if err != nil {
			return
		}
		if message.ID != 0 {
			c.pendingMu.Lock()
			waiter := c.pending[message.ID]
			delete(c.pending, message.ID)
			c.pendingMu.Unlock()
			if waiter != nil {
				waiter <- message
				continue
			}
		}
		if message.Type == TypeLookupEvent && message.Result != nil {
			c.emit(*message.Result)
		}
		if message.Type == TypeLanguagePairsEvent && message.LanguagePairs != nil {
			select {
			case c.languagePairEvents <- *message.LanguagePairs:
			default:
			}
		}
	}
}

func (c *Client) emit(result LookupResult) {
	select {
	case c.events <- result:
		return
	default:
	}
	// Keep the newest result for an unresponsive frontend.
	select {
	case <-c.events:
	default:
	}
	select {
	case c.events <- result:
	default:
	}
}
