package cli

import (
	"strings"
	"testing"
)

func TestDaemonStartHintMentionsBothModes(t *testing.T) {
	hint := daemonStartHint()
	for _, want := range []string{"systemctl --user start lexicon.service", "lexicon-daemon"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("hint = %q, want it to mention %q", hint, want)
		}
	}
}
