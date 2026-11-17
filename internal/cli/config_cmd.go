package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/ipc"
)

func (a *App) runConfig(ctx context.Context, args []string) int {
	if len(args) == 0 {
		a.errorf("config requires a subcommand: show, get, set, reset, edit, path")
		return ExitUsage
	}
	switch args[0] {
	case "show":
		return a.configShow()
	case "get":
		return a.configGet(args[1:])
	case "set":
		return a.configSet(ctx, args[1:])
	case "reset":
		return a.configReset(ctx, args[1:])
	case "edit":
		return a.configEdit(ctx, args[1:])
	case "path":
		fmt.Fprintln(a.Stdout, a.ConfigPath)
		return ExitOK
	default:
		a.errorf("unknown config subcommand %q", args[0])
		return ExitUsage
	}
}

func (a *App) configShow() int {
	cfg, err := a.effectiveConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	rendered, err := config.Render(cfg)
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	fmt.Fprint(a.Stdout, string(rendered))
	return ExitOK
}

func (a *App) configGet(args []string) int {
	if len(args) != 1 {
		a.errorf("usage: lex config get KEY")
		return ExitUsage
	}
	cfg, err := a.effectiveConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	value, err := config.Get(cfg, args[0])
	if err != nil {
		a.errorf("%v", err)
		a.printConfigKeys()
		return ExitUsage
	}
	fmt.Fprintln(a.Stdout, value)
	return ExitOK
}

func (a *App) configSet(ctx context.Context, args []string) int {
	if len(args) != 2 {
		a.errorf("usage: lex config set KEY VALUE")
		return ExitUsage
	}
	key, value := args[0], args[1]

	previous, err := a.effectiveConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	cfg, _, err := a.loadConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	if err := config.Set(&cfg, key, value); err != nil {
		a.errorf("%v", err)
		a.printConfigKeys()
		return ExitUsage
	}
	if err := config.Write(a.ConfigPath, cfg); err != nil {
		a.errorf("write config: %v", err)
		return ExitFailure
	}
	fmt.Fprintf(a.Stdout, "%s = %s\n", key, value)
	a.reloadDaemon(ctx, previous.Daemon.Socket)
	return ExitOK
}

func (a *App) configReset(ctx context.Context, args []string) int {
	flags := a.newFlagSet("config reset", "[--yes]")
	yes := flags.Bool("yes", false, "do not prompt")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if !*yes {
		confirmed, err := a.confirm("Reset the configuration to defaults?")
		if err != nil {
			a.errorf("read confirmation: %v", err)
			return ExitFailure
		}
		if !confirmed {
			fmt.Fprintln(a.Stdout, "cancelled")
			return ExitOK
		}
	}

	previous, err := a.effectiveConfig()
	if err != nil {
		a.errorf("%v", err)
		return ExitFailure
	}
	if err := config.Write(a.ConfigPath, config.Default()); err != nil {
		a.errorf("write config: %v", err)
		return ExitFailure
	}
	fmt.Fprintf(a.Stdout, "configuration reset: %s\n", a.ConfigPath)
	a.reloadDaemon(ctx, previous.Daemon.Socket)
	return ExitOK
}

func (a *App) configEdit(ctx context.Context, args []string) int {
	if len(args) != 0 {
		a.errorf("usage: lex config edit")
		return ExitUsage
	}
	if _, err := config.Ensure(a.ConfigPath); err != nil {
		a.errorf("create config: %v", err)
		return ExitFailure
	}

	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		a.errorf("set $VISUAL or $EDITOR to edit the configuration")
		return ExitFailure
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		a.errorf("$VISUAL/$EDITOR is empty")
		return ExitFailure
	}
	// The user deliberately chooses $VISUAL/$EDITOR; no shell is involved.
	//nolint:gosec // editor command comes from the user's environment
	command := exec.CommandContext(ctx, parts[0], append(parts[1:], a.ConfigPath)...)
	command.Stdin = a.Stdin
	command.Stdout = a.Stdout
	command.Stderr = a.Stderr
	if err := command.Run(); err != nil {
		a.errorf("editor failed: %v", err)
		return ExitFailure
	}

	cfg, _, err := a.loadConfig()
	if err != nil {
		a.errorf("the edited file is invalid; fix it and run `lex config edit` again: %v", err)
		return ExitFailure
	}
	a.reloadDaemon(ctx, config.Effective(cfg).Daemon.Socket)
	return ExitOK
}

func (a *App) reloadDaemon(ctx context.Context, socket string) {
	if strings.TrimSpace(socket) == "" {
		return
	}
	reloadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client, err := ipc.Dial(reloadCtx, socket, ipc.RoleProbe)
	if err != nil {
		fmt.Fprintln(a.Stderr, "note: the daemon is not running; changes apply on next start")
		return
	}
	defer func() { _ = client.Close() }()

	if err := client.Reload(reloadCtx); err != nil {
		fmt.Fprintf(a.Stderr, "warning: the configuration was saved but the daemon did not reload: %v\n", err)
		return
	}
	fmt.Fprintln(a.Stdout, "daemon reloaded")
}

func (a *App) confirm(prompt string) (bool, error) {
	fmt.Fprintf(a.Stdout, "%s [y/N]: ", prompt)
	reader := bufio.NewReader(a.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil && answer == "" {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func (a *App) printConfigKeys() {
	fmt.Fprintln(a.Stderr, "Available keys:")
	for _, key := range config.Keys() {
		fmt.Fprintf(a.Stderr, "  %s\n", key)
	}
}
