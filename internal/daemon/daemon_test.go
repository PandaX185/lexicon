package daemon

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/ipc"
)

func TestPopupBinaryErrorSuggestsCurrentBuildCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := popupBinary(config.Default())
	if err == nil {
		t.Fatal("expected an error when the popup binary is missing")
	}
	if !strings.Contains(err.Error(), "just build-popup") {
		t.Fatalf("error = %q, want the current build command", err)
	}
	if strings.Contains(err.Error(), "make popup-build") {
		t.Fatalf("error = %q, want no stale make guidance", err)
	}
}

func TestPopupArgumentsIncludeReplayForAutoLaunch(t *testing.T) {
	cfg := config.Default()
	cfg.Daemon.Socket = "/tmp/lexicon-test.sock"

	got := popupArguments(cfg, "/tmp/lexicon-test.toml", true)
	want := []string{"--socket", "/tmp/lexicon-test.sock", "--replay", "--config", "/tmp/lexicon-test.toml"}
	if !slices.Equal(got, want) {
		t.Fatalf("auto-launch arguments = %q, want %q", got, want)
	}
}

func TestPopupArgumentsOmitReplayForManualLaunch(t *testing.T) {
	cfg := config.Default()
	cfg.Daemon.Socket = "/tmp/lexicon-test.sock"

	got := popupArguments(cfg, "", false)
	want := []string{"--socket", "/tmp/lexicon-test.sock"}
	if !slices.Equal(got, want) {
		t.Fatalf("manual arguments = %q, want %q", got, want)
	}
}

func TestWaitForPopupReapsChild(t *testing.T) {
	command := exec.Command("/bin/true")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	pid := command.Process.Pid
	daemon := &Daemon{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	daemon.waitForPopup(command)

	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("process %d still exists after Wait: %v", pid, err)
	}
}

func TestLookupAttributesNeverIncludeText(t *testing.T) {
	const (
		secretQuery = "SECRET-SELECTED-TEXT"
		secretEntry = "SECRET-DICTIONARY-WORD"
		secretTrans = "SECRET-TRANSLATION"
	)
	result := &ipc.LookupResult{
		Found:       true,
		Query:       secretQuery,
		Normalized:  secretQuery,
		Translation: secretTrans,
		Entry: &dictionary.Entry{
			Word: secretEntry,
		},
	}

	attributes := lookupAttributes(result, time.Now())
	rendered := fmt.Sprint(attributes...)
	for _, secret := range []string{secretQuery, secretEntry, secretTrans} {
		if strings.Contains(rendered, secret) {
			t.Fatalf("log attributes contain %q: %s", secret, rendered)
		}
	}

	allowed := map[string]bool{
		"found":             true,
		"query_chars":       true,
		"duration_ms":       true,
		"kind":              true,
		"translation_error": true,
	}
	for i := 0; i < len(attributes); i += 2 {
		key, ok := attributes[i].(string)
		if !ok {
			t.Fatalf("attribute key %v is not a string", attributes[i])
		}
		if !allowed[key] {
			t.Fatalf("unexpected log attribute %q", key)
		}
	}
}
