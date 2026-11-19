package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// ErrAlreadyRunning means another daemon owns the socket lock.
var ErrAlreadyRunning = errors.New("another lexicon daemon is already running")

// Handler is the daemon side of the protocol.
type Handler interface {
	// Lookup resolves text, or reads the primary selection when text is empty.
	Lookup(ctx context.Context, text string) *LookupResult
	// Status reports daemon state.
	Status() Status
	// Reload re-reads configuration and swaps resources atomically.
	Reload(ctx context.Context) error
}

// PostLookupHandler is an optional extension called after a lookup reply and
// event have been sent, with the current subscriber count.
type PostLookupHandler interface {
	AfterLookup(subscribers int)
}

// ServerOptions configures a Server.
type ServerOptions struct {
	Path    string
	Logger  *slog.Logger
	Handler Handler
}

// Server owns the listening socket, the single-instance lock, and the set of
// subscribed popups.
type Server struct {
	path    string
	logger  *slog.Logger
	handler Handler

	listener net.Listener
	lockFile *os.File
	lockPath string

	mu          sync.RWMutex
	subscribers map[*connection]struct{}
	popupSeq    uint64
	last        *LookupResult

	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

type connection struct {
	conn net.Conn
	role Role
	// seq orders popup subscriptions so the newest popup dismisses older ones
	// without two simultaneous launches dismissing each other.
	seq uint64
	mu  sync.Mutex
}

// NewServer creates the runtime directory, takes the single-instance lock, and
// binds the socket. The caller must Serve or Close it.
func NewServer(options ServerOptions) (*Server, error) {
	if options.Path == "" {
		return nil, errors.New("ipc socket path is empty")
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if options.Handler == nil {
		return nil, errors.New("ipc handler is nil")
	}

	directory := filepath.Dir(options.Path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create runtime directory: %w", err)
	}
	// XDG_RUNTIME_DIR is private; make Lexicon's child explicit.
	//nolint:gosec // directories need the execute bit; 0700 is owner-only
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("secure runtime directory: %w", err)
	}

	lockPath := options.Path + ".lock"
	//nolint:gosec // lockPath is derived from the configured runtime socket
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open daemon lock: %w", err)
	}
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lockFile.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("lock daemon socket: %w", err)
	}

	// The lock is held: any existing socket is stale.
	if info, err := os.Lstat(options.Path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
			_ = lockFile.Close()
			return nil, fmt.Errorf("refusing to replace non-socket path %s", options.Path)
		}
		if err := os.Remove(options.Path); err != nil {
			_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
			_ = lockFile.Close()
			return nil, fmt.Errorf("remove stale socket: %w", err)
		}
	}

	listener, err := net.Listen("unix", options.Path)
	if err != nil {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
		return nil, fmt.Errorf("listen on %s: %w", options.Path, err)
	}
	if err := os.Chmod(options.Path, 0o600); err != nil {
		_ = listener.Close()
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
		return nil, fmt.Errorf("secure socket: %w", err)
	}

	return &Server{
		path:        options.Path,
		logger:      logger,
		handler:     options.Handler,
		listener:    listener,
		lockFile:    lockFile,
		lockPath:    lockPath,
		subscribers: map[*connection]struct{}{},
		done:        make(chan struct{}),
	}, nil
}

// Path returns the bound socket path.
func (s *Server) Path() string { return s.path }

// SubscriberCount reports connected popups.
func (s *Server) SubscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers)
}

// dismissOlderPopups tells every popup that subscribed before the given one to
// quit. The daemon owns the subscriber set, so this enforces a single popup
// regardless of whether the popups were auto-launched or started by hand. The
// sequence comparison keeps two simultaneous launches from dismissing each
// other: only the older one is asked to quit.
func (s *Server) dismissOlderPopups(newest *connection) {
	s.mu.RLock()
	older := make([]*connection, 0, len(s.subscribers))
	for connection := range s.subscribers {
		if connection != newest && connection.seq < newest.seq {
			older = append(older, connection)
		}
	}
	s.mu.RUnlock()

	for _, connection := range older {
		if err := s.send(connection, Message{Type: TypeDismissPopup}); err != nil {
			s.mu.Lock()
			delete(s.subscribers, connection)
			s.mu.Unlock()
			_ = connection.conn.Close()
		}
	}
}

// Serve accepts connections until ctx is cancelled or the listener fails.
func (s *Server) Serve(ctx context.Context) error {
	defer func() { _ = s.Close() }()
	go func() {
		select {
		case <-ctx.Done():
			_ = s.Close()
		case <-s.done:
		}
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				s.wg.Wait()
				return nil
			case <-ctx.Done():
				_ = s.Close()
				continue
			default:
				return fmt.Errorf("accept ipc connection: %w", err)
			}
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handle(ctx, conn)
		}()
	}
}

// Close stops the listener, disconnects clients, and removes the socket. It is
// safe to call more than once.
func (s *Server) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		close(s.done)
		if s.listener != nil {
			closeErr = s.listener.Close()
		}

		s.mu.Lock()
		for connection := range s.subscribers {
			_ = connection.conn.Close()
		}
		s.subscribers = map[*connection]struct{}{}
		s.mu.Unlock()

		if s.lockFile != nil {
			_ = syscall.Flock(int(s.lockFile.Fd()), syscall.LOCK_UN)
			_ = s.lockFile.Close()
		}
		_ = os.Remove(s.path)
		_ = os.Remove(s.lockPath)
	})
	if errors.Is(closeErr, net.ErrClosed) {
		return nil
	}
	return closeErr
}

func (s *Server) handle(ctx context.Context, raw net.Conn) {
	reader := newFrameReader(raw)
	_ = raw.SetReadDeadline(time.Now().Add(5 * time.Second))

	hello, err := reader.read()
	if err != nil {
		s.logger.Debug("ipc handshake failed", "error", err)
		_ = raw.Close()
		return
	}
	if hello.Version != Version || hello.Type != TypeHello {
		s.protocolError(&connection{conn: raw}, 0, "handshake", "expected protocol version 1 hello")
		_ = raw.Close()
		return
	}
	if !validRole(hello.Role) {
		s.protocolError(&connection{conn: raw}, 0, "handshake", "unknown client role")
		_ = raw.Close()
		return
	}

	connection := &connection{conn: raw, role: hello.Role}
	subscribed := hello.Role == RolePopup
	if subscribed {
		s.mu.Lock()
		s.popupSeq++
		connection.seq = s.popupSeq
		s.subscribers[connection] = struct{}{}
		s.mu.Unlock()
		s.dismissOlderPopups(connection)
	}

	s.mu.RLock()
	hasLast := s.last != nil
	s.mu.RUnlock()
	if err := s.send(connection, Message{
		Type:          TypeWelcome,
		Subscribers:   s.SubscriberCount(),
		HasLastResult: hasLast,
	}); err != nil {
		s.mu.Lock()
		delete(s.subscribers, connection)
		s.mu.Unlock()
		_ = raw.Close()
		return
	}
	_ = raw.SetReadDeadline(time.Time{})

	defer func() {
		s.mu.Lock()
		delete(s.subscribers, connection)
		s.mu.Unlock()
		_ = raw.Close()
	}()

	if subscribed {
		s.mu.RLock()
		last := s.last
		s.mu.RUnlock()
		s.logger.Info("popup subscribed", "subscribers", s.SubscriberCount())
		if last != nil {
			_ = s.send(connection, Message{Type: TypeLookupEvent, Result: last})
		}
	}

	for {
		message, err := reader.read()
		if err != nil {
			if !errors.Is(err, ErrClosed) {
				s.logger.Debug("ipc read ended", "role", hello.Role, "error", err)
			}
			return
		}
		if message.Version != Version {
			s.protocolError(connection, message.ID, "protocol_version", "unsupported protocol version")
			return
		}
		switch message.Type {
		case TypeLookup:
			result := s.handler.Lookup(ctx, message.Text)
			s.remember(result)
			if err := s.send(connection, Message{Type: TypeLookupResult, ID: message.ID, Result: result}); err != nil {
				return
			}
			s.broadcast(result)
			if post, ok := s.handler.(PostLookupHandler); ok {
				post.AfterLookup(s.SubscriberCount())
			}
		case TypeStatus:
			status := s.handler.Status()
			status.Subscribers = s.SubscriberCount()
			if err := s.send(connection, Message{Type: TypeStatusResult, ID: message.ID, Status: &status}); err != nil {
				return
			}
		case TypeReload:
			if err := s.handler.Reload(ctx); err != nil {
				s.protocolError(connection, message.ID, "reload", err.Error())
				continue
			}
			if err := s.send(connection, Message{Type: TypeAck, ID: message.ID}); err != nil {
				return
			}
		case TypeLanguagePairs:
			handler, ok := s.handler.(LanguagePairHandler)
			if !ok {
				s.protocolError(connection, message.ID, "unsupported", "language pairs are not supported by this daemon")
				continue
			}
			state := handler.LanguagePairs()
			if err := s.send(connection, Message{
				Type:          TypeLanguagePairsResult,
				ID:            message.ID,
				LanguagePairs: &state,
			}); err != nil {
				return
			}
		case TypeSetLanguagePair:
			if connection.role != RolePopup {
				s.protocolError(connection, message.ID, "role", "only popup clients may change the language pair")
				continue
			}
			handler, ok := s.handler.(LanguagePairHandler)
			if !ok {
				s.protocolError(connection, message.ID, "unsupported", "language pairs are not supported by this daemon")
				continue
			}
			state, err := handler.SetLanguagePair(ctx, message.SourceLanguage, message.TargetLanguage)
			if err != nil {
				s.protocolError(connection, message.ID, "set_language_pair", err.Error())
				continue
			}
			if err := s.send(connection, Message{
				Type:          TypeLanguagePairsResult,
				ID:            message.ID,
				LanguagePairs: &state,
			}); err != nil {
				return
			}
			s.broadcastLanguagePairs(state)
		case TypePing:
			if err := s.send(connection, Message{Type: TypePong, ID: message.ID}); err != nil {
				return
			}
		case TypeHello:
			s.protocolError(connection, message.ID, "handshake", "already initialized")
		default:
			s.protocolError(connection, message.ID, "unknown_type", "unknown message type")
		}
	}
}

func (s *Server) remember(result *LookupResult) {
	if result == nil || !result.Found {
		return
	}
	s.mu.Lock()
	s.last = result
	s.mu.Unlock()
}

func (s *Server) broadcast(result *LookupResult) {
	if result == nil {
		return
	}
	s.mu.RLock()
	connections := make([]*connection, 0, len(s.subscribers))
	for connection := range s.subscribers {
		connections = append(connections, connection)
	}
	s.mu.RUnlock()

	for _, connection := range connections {
		if err := s.send(connection, Message{Type: TypeLookupEvent, Result: result}); err != nil {
			s.mu.Lock()
			delete(s.subscribers, connection)
			s.mu.Unlock()
			_ = connection.conn.Close()
		}
	}
}

func (s *Server) broadcastLanguagePairs(state LanguagePairsState) {
	s.mu.RLock()
	connections := make([]*connection, 0, len(s.subscribers))
	for connection := range s.subscribers {
		connections = append(connections, connection)
	}
	s.mu.RUnlock()
	for _, connection := range connections {
		if err := s.send(connection, Message{
			Type:          TypeLanguagePairsEvent,
			LanguagePairs: &state,
		}); err != nil {
			s.mu.Lock()
			delete(s.subscribers, connection)
			s.mu.Unlock()
			_ = connection.conn.Close()
		}
	}
}

func (s *Server) send(connection *connection, message Message) error {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	_ = connection.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(connection.conn).Encode(withVersion(message)); err != nil {
		return fmt.Errorf("write ipc message: %w", err)
	}
	return nil
}

func (s *Server) protocolError(connection *connection, id uint64, code, message string) {
	_ = s.send(connection, Message{
		Type:  TypeError,
		ID:    id,
		Error: &ProtocolError{Code: code, Message: message},
	})
}

func withVersion(message Message) Message {
	message.Version = Version
	return message
}

func validRole(role Role) bool {
	switch role {
	case RoleCLI, RolePopup, RoleProbe:
		return true
	default:
		return false
	}
}
