// Package ipc implements Lexicon's versioned newline-delimited JSON protocol
// over a per-user Unix domain socket.
package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/PandaX185/lexicon/internal/dictionary"
)

// Version is the protocol version. It changes only for incompatible framing or
// field semantics.
const Version = 1

// MaxFrame bounds one JSON message, including the newline.
const MaxFrame = 1 << 20

// Type identifies a message.
type Type string

const (
	TypeHello               Type = "hello"
	TypeWelcome             Type = "welcome"
	TypeLookup              Type = "lookup"
	TypeLookupResult        Type = "lookup_result"
	TypeLookupEvent         Type = "lookup_event"
	TypeStatus              Type = "status"
	TypeStatusResult        Type = "status_result"
	TypeReload              Type = "reload"
	TypeAck                 Type = "ack"
	TypePing                Type = "ping"
	TypePong                Type = "pong"
	TypeError               Type = "error"
	TypeLanguagePairs       Type = "language_pairs"
	TypeLanguagePairsResult Type = "language_pairs_result"
	TypeSetLanguagePair     Type = "set_language_pair"
	TypeLanguagePairsEvent  Type = "language_pairs_event"
	// TypeDismissPopup tells an older popup that a newer one has taken over.
	// The receiver quits its whole process, enforcing a single popup instance.
	TypeDismissPopup Type = "dismiss_popup"
)

// Role identifies what a client is. The server uses it for logging and to
// avoid pointless popup launches.
type Role string

const (
	RoleCLI    Role = "cli"
	RolePopup  Role = "popup"
	RoleProbe  Role = "probe"
	RoleDaemon Role = "daemon"
)

// ErrorKind classifies lookup failures for the UI.
type ErrorKind string

const (
	KindValidation ErrorKind = "validation"
	KindSelection  ErrorKind = "selection"
	KindNotFound   ErrorKind = "not_found"
	KindDictionary ErrorKind = "dictionary"
	KindInternal   ErrorKind = "internal"
)

// Message is the single envelope used in both directions. Unused fields are
// omitted, keeping frames small and logs readable.
type Message struct {
	Version     int            `json:"v"`
	Type        Type           `json:"type"`
	ID          uint64         `json:"id,omitempty"`
	Role        Role           `json:"role,omitempty"`
	Text        string         `json:"text,omitempty"`
	Result      *LookupResult  `json:"result,omitempty"`
	Status      *Status        `json:"status,omitempty"`
	Error       *ProtocolError `json:"error,omitempty"`
	Subscribers int            `json:"subscribers,omitempty"`
	// HasLastResult is set on welcome so a freshly connected popup can tell
	// whether the daemon will replay a result or needs a fresh selection read.
	HasLastResult bool `json:"has_last_result,omitempty"`

	LanguagePairs  *LanguagePairsState `json:"language_pairs,omitempty"`
	SourceLanguage string              `json:"source_language,omitempty"`
	TargetLanguage string              `json:"target_language,omitempty"`
}

// ProtocolError is a transport or request-level failure.
type ProtocolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ProtocolError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// LookupResult is both the reply to a lookup and the event pushed to popups.
// A missing entry is represented by Found=false plus Error/ErrorKind; it is not
// a transport error.
type LookupResult struct {
	Query            string            `json:"query"`
	Normalized       string            `json:"normalized,omitempty"`
	Found            bool              `json:"found"`
	Entry            *dictionary.Entry `json:"entry,omitempty"`
	Translation      string            `json:"translation,omitempty"`
	TranslationError string            `json:"translation_error,omitempty"`
	Error            string            `json:"error,omitempty"`
	ErrorKind        ErrorKind         `json:"error_kind,omitempty"`
	At               time.Time         `json:"at"`
}

// Status is daemon health and configuration summary. It never includes
// selected text.
type Status struct {
	Version         string `json:"version"`
	PID             int    `json:"pid"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
	Subscribers     int    `json:"subscribers"`
	ProtocolVersion int    `json:"protocol_version"`
	DictionaryPath  string `json:"dictionary_path,omitempty"`
	DictionaryName  string `json:"dictionary_name,omitempty"`
	Selection       string `json:"selection,omitempty"`
	Translation     string `json:"translation,omitempty"`
	// TranslationAvailable reports whether the daemon's compiled-in engine can
	// translate. The CLI's own build may differ, so doctor trusts this field.
	TranslationAvailable bool   `json:"translation_available,omitempty"`
	Socket               string `json:"socket,omitempty"`
}

// ErrClosed is returned when the connection is gone.
var ErrClosed = errors.New("connection closed")

// Encode writes one framed message.
func Encode(encoder *json.Encoder, message Message) error {
	if message.Version == 0 {
		message.Version = Version
	}
	if message.Version != Version {
		return fmt.Errorf("unsupported protocol version %d", message.Version)
	}
	return encoder.Encode(message)
}
