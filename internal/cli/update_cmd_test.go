package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PandaX185/lexicon/internal/update"
)

type fakeUpdater struct {
	plan           *update.Plan
	planErr        error
	applyErr       error
	restartSkipped string
	applied        int
}

func (f *fakeUpdater) Plan(context.Context, update.Options) (*update.Plan, error) {
	return f.plan, f.planErr
}
func (f *fakeUpdater) Apply(_ context.Context, plan *update.Plan) error {
	f.applied++
	if f.restartSkipped != "" && plan != nil {
		plan.RestartSkipped = f.restartSkipped
	}
	return f.applyErr
}

func newUpdateTestApp(t *testing.T, updater *fakeUpdater, stdin string) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app := &App{
		Stdin: strings.NewReader(stdin), Stdout: out, Stderr: errOut,
		ConfigPath: filepath.Join(dir, "config.toml"), updateRunner: updater,
	}
	return app, out, errOut
}

func TestRunUpdateCheckDoesNotApply(t *testing.T) {
	updater := &fakeUpdater{plan: &update.Plan{Version: "9.9.9", DictionaryPath: "/tmp/dictionary.sqlite"}}
	app, out, errOut := newUpdateTestApp(t, updater, "")
	code := app.Run(context.Background(), []string{"update", "--check"})
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, errOut.String())
	}
	if updater.applied != 0 {
		t.Fatal("--check applied an update")
	}
	if !strings.Contains(out.String(), "9.9.9") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRunUpdateRequiresYesWhenNonInteractive(t *testing.T) {
	updater := &fakeUpdater{plan: &update.Plan{Version: "9.9.9"}}
	app, _, errOut := newUpdateTestApp(t, updater, "")
	args := []string{"update", "--version", "9.9.9", "--bin-dir", "/opt/lexicon/bin", "--no-restart"}
	code := app.Run(context.Background(), args)
	if code != ExitFailure {
		t.Fatalf("code = %d, want %d", code, ExitFailure)
	}
	if updater.applied != 0 {
		t.Fatal("non-interactive update was applied")
	}
	want := "lexicon update --version 9.9.9 --bin-dir /opt/lexicon/bin --no-restart --yes"
	if !strings.Contains(errOut.String(), want) {
		t.Fatalf("stderr = %s, want exact retry command %q", errOut.String(), want)
	}
}

func TestUpdateRetryCommandQuotesBinDirWithSpaces(t *testing.T) {
	got := updateRetryCommand("9.9.9", "/opt/my lexicon/bin", true)
	want := "lexicon update --version 9.9.9 --bin-dir '/opt/my lexicon/bin' --no-restart --yes"
	if got != want {
		t.Fatalf("retry = %q, want %q", got, want)
	}
}

func TestPrintUpdateAssetsShowsChecksums(t *testing.T) {
	plan := &update.Plan{
		TarballName:   "lexicon-9.9.9-linux-amd64.tar.gz",
		TarballSHA256: "abc123",
	}
	var out bytes.Buffer
	printUpdateAssets(&out, plan)
	got := out.String()
	for _, want := range []string{
		"lexicon-9.9.9-linux-amd64.tar.gz",
		"abc123",
		update.EnSQLiteSHA256,
		update.ManifestSHA256,
		"en.sqlite",
		"manifest.json",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("asset summary = %q, missing %q", got, want)
		}
	}
}

func TestRunUpdateYesApplies(t *testing.T) {
	updater := &fakeUpdater{plan: &update.Plan{Version: "9.9.9"}}
	app, out, errOut := newUpdateTestApp(t, updater, "")
	code := app.Run(context.Background(), []string{"update", "--yes"})
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, errOut.String())
	}
	if updater.applied != 1 {
		t.Fatalf("applied = %d, want 1", updater.applied)
	}
	if !strings.Contains(out.String(), "9.9.9") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRunUpdateSuccessReportsDictionaryAndService(t *testing.T) {
	updater := &fakeUpdater{plan: &update.Plan{Version: "9.9.9", DictionaryPath: "/tmp/dictionary.sqlite"}}
	app, out, errOut := newUpdateTestApp(t, updater, "")
	code := app.Run(context.Background(), []string{"update", "--yes"})
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, errOut.String())
	}
	got := out.String()
	for _, want := range []string{"updated to 9.9.9", "dictionary: updated", "service: restarted"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output = %q, missing %q", got, want)
		}
	}
}

func TestRunUpdateRestartSkippedNotice(t *testing.T) {
	const notice = "systemctl is unavailable; the service was not restarted"
	updater := &fakeUpdater{plan: &update.Plan{Version: "9.9.9"}, restartSkipped: notice}
	app, out, errOut := newUpdateTestApp(t, updater, "")
	code := app.Run(context.Background(), []string{"update", "--yes"})
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, errOut.String())
	}
	if updater.applied != 1 {
		t.Fatalf("applied = %d, want 1", updater.applied)
	}
	if !strings.Contains(out.String(), notice) {
		t.Fatalf("output = %s, want restart-skipped notice %q", out.String(), notice)
	}
}
