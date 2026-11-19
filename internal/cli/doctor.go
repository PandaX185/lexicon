package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/ipc"
	"github.com/PandaX185/lexicon/internal/paths"
	"github.com/PandaX185/lexicon/internal/selection"
	"github.com/PandaX185/lexicon/internal/translation"
)

type diagnostic struct {
	status string
	name   string
	detail string
}

func (a *App) runDoctor(ctx context.Context, args []string) int {
	if len(args) != 0 {
		a.errorf("usage: lexicon doctor")
		return ExitUsage
	}

	var diagnostics []diagnostic
	add := func(status, name, detail string) {
		diagnostics = append(diagnostics, diagnostic{status: status, name: name, detail: detail})
	}

	cfg, exists, err := a.loadConfig()
	switch {
	case err != nil:
		add("fail", "config", err.Error())
		cfg = config.Default()
	case !exists:
		add("warn", "config", "not found; using defaults at "+a.ConfigPath)
	default:
		add("ok", "config", a.ConfigPath)
	}
	effective := config.Effective(cfg)

	if effective.Daemon.Socket == "" {
		add("fail", "socket", "no socket path; XDG_RUNTIME_DIR is required")
	} else {
		add("ok", "socket", effective.Daemon.Socket)
	}

	store, storeErr := dictionary.Open(effective.Dictionary.DatabasePath)
	if storeErr != nil {
		add("fail", "dictionary", storeErr.Error())
	} else {
		schemaErr := store.ValidateLookupSchema()
		describeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		info, describeErr := store.Describe(describeCtx)
		cancel()
		_ = store.Close()
		switch {
		case schemaErr != nil:
			add("fail", "dictionary", effective.Dictionary.DatabasePath+" ("+schemaErr.Error()+")")
		case describeErr != nil:
			add("warn", "dictionary", effective.Dictionary.DatabasePath+" ("+describeErr.Error()+")")
		default:
			detail := fmt.Sprintf("%s [%s→%s] %s",
				info.Name, info.SourceLanguage, info.TargetLanguage, effective.Dictionary.DatabasePath)
			if attribution := strings.TrimSpace(info.Attribution); attribution != "" {
				detail += " — " + attribution
			}
			add("ok", "dictionary", detail)
		}
	}

	provider, providerErr := selection.New(selection.Options{
		Provider: effective.Selection.Provider,
		Timeout:  time.Duration(effective.Selection.TimeoutMS) * time.Millisecond,
		MaxChars: effective.Selection.MaxChars,
	}, nil)
	switch {
	case providerErr != nil:
		add("fail", "selection", providerErr.Error())
	case provider.Name() == "unavailable":
		add("warn", "selection", "wl-paste not found; install wl-clipboard")
	case provider.Name() == "none":
		add("warn", "selection", "disabled in configuration")
	default:
		add("ok", "selection", provider.Name()+" (primary selection only)")
	}

	var daemonStatus *ipc.Status
	if effective.Daemon.Socket != "" {
		statusCtx, cancel := context.WithTimeout(ctx, time.Second)
		client, dialErr := ipc.Dial(statusCtx, effective.Daemon.Socket, ipc.RoleProbe)
		if dialErr != nil {
			add("warn", "daemon", "not running")
		} else {
			status, statusErr := client.Status(statusCtx)
			_ = client.Close()
			if statusErr != nil {
				add("warn", "daemon", statusErr.Error())
			} else {
				daemonStatus = status
				add("ok", "daemon", fmt.Sprintf("pid %d, %d subscriber(s), uptime %ds",
					status.PID, status.Subscribers, status.UptimeSeconds))
			}
		}
		cancel()
	}

	if daemonStatus != nil {
		status, detail := checkSocketPermissions(effective.Daemon.Socket)
		add(status, "socket mode", detail)
	}
	modelStatus, modelDetail := describeModelDirectory(effective.Translation.ModelDirectory)
	add(modelStatus, "models", modelDetail)

	translator := translation.New(translation.Options{
		Enabled:        effective.Translation.Enabled,
		ModelDirectory: effective.Translation.ModelDirectory,
		Timeout:        time.Duration(effective.Translation.TimeoutSeconds) * time.Second,
	})
	engineName := translator.Name()
	available := translator.Available()
	if daemonStatus != nil && daemonStatus.Translation != "" {
		engineName = daemonStatus.Translation
		available = daemonStatus.TranslationAvailable
	}
	switch {
	case !effective.Translation.Enabled:
		add("ok", "translation", "disabled")
	case available:
		add("ok", "translation", engineName)
	default:
		add("warn", "translation", engineName)
	}
	_ = translator.Close()

	if binary, ok := findPopupBinary(effective); ok {
		add("ok", "popup", binary)
	} else {
		add("warn", "popup", "lexicon-popup not found; build it with `just build-popup`")
	}

	failures := 0
	for _, item := range diagnostics {
		if item.status == "fail" {
			failures++
		}
		fmt.Fprintf(a.Stdout, "  %-4s  %-12s %s\n", item.status, item.name, item.detail)
	}
	if failures > 0 {
		return ExitFailure
	}
	return ExitOK
}

func checkSocketPermissions(path string) (status, detail string) {
	info, err := os.Stat(path)
	if err != nil {
		return "warn", fmt.Sprintf("cannot inspect %s: %v", path, err)
	}
	perm := info.Mode().Perm()
	if perm&0o077 != 0 {
		return "warn", fmt.Sprintf("%s is group/world accessible (mode %04o)", path, perm)
	}
	return "ok", fmt.Sprintf("owner-only socket (mode %04o)", perm)
}

func describeModelDirectory(dir string) (status, detail string) {
	if strings.TrimSpace(dir) == "" {
		return "fail", "no model directory configured"
	}
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "warn", dir + " (not created)"
	case err != nil:
		return "warn", fmt.Sprintf("%s (%v)", dir, err)
	case !info.IsDir():
		return "fail", dir + " is not a directory"
	}
	packs, err := translation.InstalledPacks(dir)
	if err != nil {
		return "warn", fmt.Sprintf("%s (%v)", dir, err)
	}
	return "ok", fmt.Sprintf("%s: %d pack(s)", dir, len(packs))
}

func findPopupBinary(cfg config.Config) (string, bool) {
	if configured := strings.TrimSpace(cfg.Popup.Binary); configured != "" {
		expanded, err := paths.ExpandHome(configured)
		if err != nil {
			return "", false
		}
		info, err := os.Stat(expanded)
		return expanded, err == nil && info.Mode().IsRegular()
	}
	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), "lexicon-popup")
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, true
		}
	}
	path, err := exec.LookPath("lexicon-popup")
	return path, err == nil
}
