package selection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	data  []byte
	err   error
	limit int
	name  string
	args  []string
}

func (f *fakeRunner) Output(_ context.Context, limit int, name string, args ...string) ([]byte, error) {
	f.limit = limit
	f.name = name
	f.args = args
	return f.data, f.err
}

func TestWlPasteReadsPrimarySelection(t *testing.T) {
	runner := &fakeRunner{data: []byte("  hello  ")}
	provider, err := New(Options{Provider: "wl-paste", Timeout: time.Second, MaxChars: 128}, runner)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	text, err := provider.Primary(context.Background())
	if err != nil {
		t.Fatalf("primary: %v", err)
	}
	if text != "hello" {
		t.Fatalf("text = %q, want hello", text)
	}
	if runner.name != "wl-paste" {
		t.Fatalf("binary = %q", runner.name)
	}
	joined := strings.Join(runner.args, " ")
	if !strings.Contains(joined, "--primary") || !strings.Contains(joined, "--no-newline") {
		t.Fatalf("args = %v, want primary selection without trailing newline", runner.args)
	}
}

func TestWlPasteErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		run  error
		want error
	}{
		{"empty", []byte("   "), nil, ErrNoText},
		{"invalid utf8", []byte{0xff, 0xfe}, nil, ErrNotText},
		{"too long", []byte(strings.Repeat("a", 129)), nil, ErrTooLong},
		{"truncated", []byte("partial"), ErrTruncated, ErrTooLong},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider, err := New(Options{Provider: "wl-paste", Timeout: time.Second, MaxChars: 128},
				&fakeRunner{data: test.data, err: test.run})
			if err != nil {
				t.Fatalf("new: %v", err)
			}
			if _, err := provider.Primary(context.Background()); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestWlPasteCommandFailure(t *testing.T) {
	provider, err := New(Options{Provider: "wl-paste", Timeout: time.Second, MaxChars: 128},
		&fakeRunner{err: errors.New("compositor unavailable")})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := provider.Primary(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDisabledProvider(t *testing.T) {
	provider, err := New(Options{Provider: "none"}, nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if provider.Name() != "none" {
		t.Fatalf("name = %q", provider.Name())
	}
	if _, err := provider.Primary(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Fatalf("error = %v, want ErrDisabled", err)
	}
}

func TestAutoProviderWithoutWlPaste(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	provider, err := New(Options{Provider: "auto", Timeout: time.Second, MaxChars: 128}, &fakeRunner{})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if provider.Name() != "unavailable" {
		t.Fatalf("name = %q, want unavailable", provider.Name())
	}
	if _, err := provider.Primary(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestNewRejectsUnknownProvider(t *testing.T) {
	if _, err := New(Options{Provider: "at-spi"}, nil); err == nil {
		t.Fatal("expected an error")
	}
}

func TestExecRunnerTruncates(t *testing.T) {
	runner := ExecRunner{}
	data, err := runner.Output(context.Background(), 3, "printf", "%s", "abcdef")
	if !errors.Is(err, ErrTruncated) {
		t.Fatalf("error = %v, want ErrTruncated", err)
	}
	if string(data) != "abc" {
		t.Fatalf("data = %q, want abc", data)
	}
}

func TestExecRunnerReads(t *testing.T) {
	runner := ExecRunner{}
	data, err := runner.Output(context.Background(), 16, "printf", "%s", "hello")
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("data = %q", data)
	}
}
