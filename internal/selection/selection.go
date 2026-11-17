// Package selection reads the Wayland primary selection. Only the primary
// selection is used: the clipboard is never read.
package selection

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

// Errors returned by providers.
var (
	// ErrNoText means the selection is empty or whitespace only.
	ErrNoText = errors.New("no text is currently selected")
	// ErrNotText means the selected data is not valid UTF-8 text.
	ErrNotText = errors.New("the current selection is not valid UTF-8 text")
	// ErrTooLong means the selection exceeds the configured character limit.
	ErrTooLong = errors.New("the selection is too long to look up")
	// ErrDisabled means selection retrieval is turned off in the config.
	ErrDisabled = errors.New("primary selection lookup is disabled")
	// ErrUnavailable means the platform helper is missing.
	ErrUnavailable = errors.New("primary selection provider is unavailable")
)

// Provider reads the current primary selection.
type Provider interface {
	// Name identifies the implementation in status output.
	Name() string
	// Primary returns trimmed selection text, or an error.
	Primary(ctx context.Context) (string, error)
}

// Options configures the provider.
type Options struct {
	// Provider is "auto", "wl-paste", or "none".
	Provider string
	// Timeout bounds the helper process.
	Timeout time.Duration
	// MaxChars rejects selections longer than this many runes.
	MaxChars int
}

// Runner abstracts process execution for tests.
type Runner interface {
	// Output runs name with args and returns at most limit bytes. When the
	// command writes more, the returned data is truncated and ErrTruncated is
	// returned with it.
	Output(ctx context.Context, limit int, name string, args ...string) ([]byte, error)
}

// ErrTruncated signals that a runner hit its output cap.
var ErrTruncated = errors.New("process output truncated")

// ExecRunner runs real subprocesses without a shell.
type ExecRunner struct{}

func (ExecRunner) Output(ctx context.Context, limit int, name string, args ...string) ([]byte, error) {
	// The runner is an intentional seam: callers provide the binary and args.
	//nolint:gosec // subprocess name and arguments come from trusted providers
	command := exec.CommandContext(ctx, name, args...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("prepare %s: %w", name, err)
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	data := make([]byte, 0, limit)
	buffer := make([]byte, 4096)
	truncated := false
	for {
		read, readErr := stdout.Read(buffer)
		if read > 0 {
			remaining := limit - len(data)
			if remaining <= 0 {
				truncated = true
				break
			}
			if read > remaining {
				data = append(data, buffer[:remaining]...)
				truncated = true
				break
			}
			data = append(data, buffer[:read]...)
		}
		if readErr != nil {
			break
		}
	}
	if truncated {
		_ = command.Process.Kill()
	}
	waitErr := command.Wait()
	if truncated {
		return data, ErrTruncated
	}
	if waitErr != nil {
		return data, fmt.Errorf("%s failed: %w", name, waitErr)
	}
	return data, nil
}

// New builds the configured provider.
func New(options Options, runner Runner) (Provider, error) {
	if options.Timeout <= 0 {
		options.Timeout = 400 * time.Millisecond
	}
	if options.MaxChars <= 0 {
		options.MaxChars = 128
	}
	if runner == nil {
		runner = ExecRunner{}
	}
	switch strings.ToLower(strings.TrimSpace(options.Provider)) {
	case "none":
		return disabledProvider{}, nil
	case "", "auto":
		if _, err := exec.LookPath("wl-paste"); err != nil {
			// Auto mode degrades instead of failing daemon startup; the
			// missing tool is reported on the next lookup.
			//nolint:nilerr // missing wl-paste is a runtime condition, not a construction error
			return missingProvider{reason: "wl-paste was not found; install wl-clipboard"}, nil
		}
		return &WlPaste{runner: runner, options: options}, nil
	case "wl-paste":
		return &WlPaste{runner: runner, options: options}, nil
	default:
		return nil, fmt.Errorf("unknown selection provider %q", options.Provider)
	}
}

// WlPaste reads the primary selection through wl-clipboard.
type WlPaste struct {
	runner  Runner
	options Options
}

func (p *WlPaste) Name() string { return "wl-paste" }

func (p *WlPaste) Primary(parent context.Context) (string, error) {
	// Bound the raw read well above MaxChars so an oversized selection is
	// detected as such instead of being silently truncated.
	rawLimit := p.options.MaxChars*4 + 64
	if rawLimit < 4096 {
		rawLimit = 4096
	}

	ctx, cancel := context.WithTimeout(parent, p.options.Timeout)
	defer cancel()

	raw, err := p.runner.Output(ctx, rawLimit, "wl-paste", "--primary", "--no-newline")
	if errors.Is(err, ErrTruncated) {
		return "", fmt.Errorf("%w (maximum %d characters)", ErrTooLong, p.options.MaxChars)
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("reading the primary selection timed out after %s", p.options.Timeout)
		}
		return "", fmt.Errorf("read primary selection: %w", err)
	}

	// A truncated read can end in the middle of a multi-byte rune, so reject
	// anything that is not valid UTF-8 rather than dropping bytes.
	if !utf8.Valid(raw) {
		return "", ErrNotText
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "", ErrNoText
	}
	if utf8.RuneCountInString(text) > p.options.MaxChars {
		return "", fmt.Errorf("%w (maximum %d characters)", ErrTooLong, p.options.MaxChars)
	}
	return text, nil
}

type disabledProvider struct{}

func (disabledProvider) Name() string { return "none" }
func (disabledProvider) Primary(context.Context) (string, error) {
	return "", ErrDisabled
}

type missingProvider struct{ reason string }

func (missingProvider) Name() string { return "unavailable" }
func (m missingProvider) Primary(context.Context) (string, error) {
	return "", fmt.Errorf("%w: %s", ErrUnavailable, m.reason)
}
