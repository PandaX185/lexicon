package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/update"
	"github.com/PandaX185/lexicon/internal/version"
)

type updateRunnerInterface interface {
	Plan(context.Context, update.Options) (*update.Plan, error)
	Apply(context.Context, *update.Plan) error
}

func (a *App) updater() updateRunnerInterface {
	if a.updateRunner != nil {
		return a.updateRunner
	}
	return update.NewRunner()
}

func (a *App) runUpdate(ctx context.Context, args []string) int {
	flags := a.newFlagSet("update", "[--check] [--version X.Y.Z] [--yes] [--no-restart] [--bin-dir PATH]")
	check := flags.Bool("check", false, "report the available version without changing anything")
	target := flags.String("version", "", "install a specific X.Y.Z release")
	yes := flags.Bool("yes", false, "apply without a prompt")
	noRestart := flags.Bool("no-restart", false, "do not restart lexicon.service")
	binDir := flags.String("bin-dir", "", "install binaries into this directory")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if len(flags.Args()) != 0 {
		a.errorf("usage: lexicon update [--check] [--version X.Y.Z] [--yes] [--no-restart] [--bin-dir PATH]")
		return ExitUsage
	}

	runner := a.updaterFor(*noRestart)
	plan, err := runner.Plan(ctx, update.Options{Version: *target, ConfigPath: a.ConfigPath, BinDir: *binDir, Check: *check})
	if err != nil {
		a.errorf("plan update: %v", err)
		return ExitFailure
	}

	fmt.Fprintf(a.Stdout, "installed: %s\n", version.Version)
	fmt.Fprintf(a.Stdout, "available: %s (%s)\n", plan.Version, plan.Tag)
	fmt.Fprintf(a.Stdout, "binaries: %s\n", plan.BinDir)
	if plan.DictionarySkipped != "" {
		fmt.Fprintf(a.Stdout, "dictionary: %s\n", plan.DictionarySkipped)
	} else {
		fmt.Fprintf(a.Stdout, "dictionary: %s (%s)\n", plan.DictionaryPath, dictionaryStatus(plan.DictionaryPath))
	}
	if *check {
		return ExitOK
	}

	interactive := isInteractive(a.Stdin)
	if !*yes && !interactive {
		a.errorf("non-interactive update requires --yes; retry with: %s", updateRetryCommand(*target, *binDir, *noRestart))
		return ExitFailure
	}
	if !*yes {
		printUpdateAssets(a.Stdout, plan)
		fmt.Fprint(a.Stdout, "Apply this update? [y/N] ")
		answer, err := bufio.NewReader(a.Stdin).ReadString('\n')
		if err != nil && len(answer) == 0 {
			a.errorf("read confirmation: %v", err)
			return ExitFailure
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
		default:
			fmt.Fprintln(a.Stdout, "update cancelled")
			return ExitOK
		}
	}

	if err := runner.Apply(ctx, plan); err != nil {
		a.errorf("update failed: %v", err)
		return ExitFailure
	}
	fmt.Fprintf(a.Stdout, "updated to %s\n", plan.Version)
	if plan.DictionarySkipped != "" {
		fmt.Fprintf(a.Stdout, "dictionary: not updated (%s)\n", plan.DictionarySkipped)
	} else {
		fmt.Fprintf(a.Stdout, "dictionary: updated to pinned %s\n", update.DictionaryTag)
	}
	if *noRestart {
		fmt.Fprintln(a.Stdout, "service was not restarted; run systemctl --user restart lexicon.service")
	} else if plan.RestartSkipped == "" {
		fmt.Fprintln(a.Stdout, "service: restarted")
	}
	if plan.RestartSkipped != "" {
		fmt.Fprintf(a.Stdout, "notice: %s\n", plan.RestartSkipped)
	}
	return ExitOK
}

func (a *App) updaterFor(noRestart bool) updateRunnerInterface {
	runner := a.updater()
	if concrete, ok := runner.(*update.Runner); ok && noRestart {
		copied := *concrete
		copied.NoRestart = true
		return &copied
	}
	return runner
}

// printUpdateAssets shows exactly what will be installed and the checksums
// each asset was verified against, before the user confirms the update.
func printUpdateAssets(out io.Writer, plan *update.Plan) {
	fmt.Fprintf(out, "assets: lexicon, lexicon-daemon, lexicon-installer, lexicon-popup, %s\n", update.DictionaryTag)
	fmt.Fprintln(out, "checksums:")
	if plan != nil && plan.TarballName != "" {
		fmt.Fprintf(out, "  %s  %s\n", plan.TarballName, plan.TarballSHA256)
	}
	fmt.Fprintf(out, "  en.sqlite  %s\n", update.EnSQLiteSHA256)
	fmt.Fprintf(out, "  manifest.json  %s\n", update.ManifestSHA256)
}

// updateRetryCommand builds the exact command a user can re-run non-interactively,
// preserving every flag that changes what gets installed.
func updateRetryCommand(target, binDir string, noRestart bool) string {
	parts := []string{"lexicon", "update"}
	if target != "" {
		parts = append(parts, "--version", target)
	}
	if binDir != "" {
		parts = append(parts, "--bin-dir", shellQuote(binDir))
	}
	if noRestart {
		parts = append(parts, "--no-restart")
	}
	return strings.Join(append(parts, "--yes"), " ")
}

// shellQuote quotes only when the value contains shell-significant characters.
func shellQuote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n\r\"'\\$`;&|()<>!*?[]{}~#") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func dictionaryStatus(path string) string {
	if path == "" {
		return "not installed"
	}
	if err := dictionary.VerifySHA256(path, update.EnSQLiteSHA256); err != nil {
		return "does not match pinned " + update.DictionaryTag
	}
	return "matches pinned " + update.DictionaryTag
}

func isInteractive(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
