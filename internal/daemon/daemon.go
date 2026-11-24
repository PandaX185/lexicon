// Package daemon wires configuration, selection, dictionary, translation, IPC,
// and popup launch into the long-running per-user process.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/dictionary"
	"github.com/PandaX185/lexicon/internal/ipc"
	"github.com/PandaX185/lexicon/internal/paths"
	"github.com/PandaX185/lexicon/internal/selection"
	"github.com/PandaX185/lexicon/internal/translation"
	"github.com/PandaX185/lexicon/internal/version"
)

const popupDebounce = 2 * time.Second

// Daemon owns all shared resources. Lookup holds a read lock for its whole
// duration so Reload can swap resources without closing a database mid-query.
type Daemon struct {
	configPath string
	logger     *slog.Logger
	levelVar   *slog.LevelVar

	mu         sync.RWMutex
	cfg        config.Config
	store      *dictionary.Store
	storeErr   error
	selection  selection.Provider
	translator *translation.Service

	started time.Time
	server  atomic.Pointer[ipc.Server]

	// The active translation pair lives only in this process; a restart falls
	// back to the first installed pair.
	pairMu     sync.Mutex
	pairSource string
	pairTarget string

	popupMu      sync.Mutex
	lastPopupTry time.Time
}

// New loads configuration and prepares resources. Missing dictionaries and
// unavailable translation engines are reported per lookup rather than
// preventing the daemon from starting.
func New(configPath string, logger *slog.Logger, levelVar *slog.LevelVar) (*Daemon, error) {
	if logger == nil {
		logger = slog.Default()
	}
	cfg, exists, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		logger.Warn("config file not found; using defaults", "path", configPath)
	}
	cfg = config.Effective(cfg)

	store, storeErr := openStore(cfg.Dictionary.DatabasePath)
	provider, err := selection.New(selection.Options{
		Provider: cfg.Selection.Provider,
		Timeout:  time.Duration(cfg.Selection.TimeoutMS) * time.Millisecond,
		MaxChars: cfg.Selection.MaxChars,
	}, nil)
	if err != nil {
		closeStore(store)
		return nil, err
	}

	return &Daemon{
		configPath: configPath,
		logger:     logger,
		levelVar:   levelVar,
		cfg:        cfg,
		store:      store,
		storeErr:   storeErr,
		selection:  provider,
		translator: translation.New(translation.Options{
			Enabled:        cfg.Translation.Enabled,
			ModelDirectory: cfg.Translation.ModelDirectory,
			Timeout:        time.Duration(cfg.Translation.TimeoutSeconds) * time.Second,
		}),
		started: time.Now(),
	}, nil
}

// Run binds the socket and serves until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	cfg := d.snapshotConfig()
	server, err := ipc.NewServer(ipc.ServerOptions{
		Path:    cfg.Daemon.Socket,
		Logger:  d.logger,
		Handler: d,
	})
	if err != nil {
		return err
	}
	d.server.Store(server)
	defer func() {
		_ = server.Close()
		d.closeResources()
	}()

	d.logger.Info("daemon started",
		"socket", server.Path(),
		"dictionary", cfg.Dictionary.DatabasePath,
		"selection", d.selectionName(),
		"translation", d.translationName(),
		"version", version.Version,
	)
	return server.Serve(ctx)
}

// Lookup implements ipc.Handler.
func (d *Daemon) Lookup(ctx context.Context, raw string) *ipc.LookupResult {
	started := time.Now()

	// LanguagePairs takes its own RLock. It completes before d.mu.RLock() below,
	// so a writer waiting in between cannot deadlock a nested RLock.
	pair := d.LanguagePairs()

	d.mu.RLock()
	defer d.mu.RUnlock()
	translator := d.translator

	result := &ipc.LookupResult{
		Query: strings.TrimSpace(raw),
		At:    time.Now().UTC(),
	}

	text := raw
	if strings.TrimSpace(text) == "" {
		selected, err := d.selection.Primary(ctx)
		if err != nil {
			result.Error = selectionMessage(err)
			result.ErrorKind = ipc.KindSelection
			d.logLookup(result, started)
			return result
		}
		text = selected
		result.Query = strings.TrimSpace(text)
	}

	normalized, err := dictionary.Prepare(text, d.cfg.General.MaxQueryWords, d.cfg.General.MaxQueryChars)
	if err != nil {
		result.Error = err.Error()
		result.ErrorKind = ipc.KindValidation
		d.logLookup(result, started)
		return result
	}
	result.Normalized = normalized

	if d.store == nil {
		result.Error = "The dictionary is unavailable."
		if d.storeErr != nil {
			result.Error = d.storeErr.Error()
		}
		result.ErrorKind = ipc.KindDictionary
		d.logLookup(result, started)
		return result
	}

	entry, err := d.store.Lookup(ctx, normalized)
	if err != nil {
		if errors.Is(err, dictionary.ErrNotFound) {
			result.Error = fmt.Sprintf("No entry found for %q.", normalized)
			result.ErrorKind = ipc.KindNotFound
		} else {
			result.Error = err.Error()
			result.ErrorKind = ipc.KindDictionary
		}
		d.logLookup(result, started)
		return result
	}

	result.Found = true
	result.Entry = entry
	if pair.Available && translator != nil && translator.Available() && pair.Source != "" && pair.Target != "" && pair.Source != pair.Target {
		translated, translateErr := translator.Translate(ctx, normalized, pair.Source, pair.Target)
		if translateErr != nil {
			result.TranslationError = translateErr.Error()
		} else {
			result.Translation = translated
		}
	}
	d.logLookup(result, started)
	return result
}

// AfterLookup implements ipc.PostLookupHandler. The result is already stored
// for replay before this is called, so a newly launched popup will receive it.
func (d *Daemon) AfterLookup(subscribers int) {
	if subscribers > 0 {
		return
	}
	cfg := d.snapshotConfig()
	if !cfg.Popup.AutoLaunch {
		return
	}

	d.popupMu.Lock()
	if time.Since(d.lastPopupTry) < popupDebounce {
		d.popupMu.Unlock()
		return
	}
	d.lastPopupTry = time.Now()
	d.popupMu.Unlock()

	binary, err := popupBinary(cfg)
	if err != nil {
		d.logger.Warn("popup launch skipped", "error", err)
		return
	}
	// Only daemon-launched popups replay the lookup that triggered them. A
	// manually started popup must read the current PRIMARY instead.
	arguments := popupArguments(cfg, d.configPath, true)
	// binary is resolved from config or next to the daemon executable.
	//nolint:gosec // user-configurable popup binary path
	command := exec.Command(binary, arguments...)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		d.logger.Warn("popup launch failed", "binary", binary, "error", err)
		return
	}
	// Reap the detached child when the popup exits. Process.Release would let
	// every dismissed popup remain in the process table as a zombie.
	go d.waitForPopup(command)
	d.logger.Info("popup launched", "binary", binary)
}

func (d *Daemon) waitForPopup(command *exec.Cmd) {
	if err := command.Wait(); err != nil {
		d.logger.Debug("popup exited", "error", err)
	}
}

// Status implements ipc.Handler.
func (d *Daemon) Status() ipc.Status {
	status := ipc.Status{
		Version:         version.Version,
		PID:             os.Getpid(),
		UptimeSeconds:   int64(time.Since(d.started).Seconds()),
		ProtocolVersion: ipc.Version,
	}

	d.mu.RLock()
	status.Socket = d.cfg.Daemon.Socket
	status.DictionaryPath = d.cfg.Dictionary.DatabasePath
	status.Selection = d.selection.Name()
	status.Translation = d.translator.Name()
	status.TranslationAvailable = d.translator.Available()
	if d.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if info, err := d.store.Describe(ctx); err == nil {
			status.DictionaryName = info.Name
		}
		cancel()
	}
	d.mu.RUnlock()
	return status
}

// Reload implements ipc.Handler. Resources are built before the swap; a
// failure leaves the previous configuration in place.
func (d *Daemon) Reload(context.Context) error {
	cfg, exists, err := config.Load(d.configPath)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("config file not found: %s", d.configPath)
	}
	cfg = config.Effective(cfg)

	d.mu.RLock()
	currentSocket := d.cfg.Daemon.Socket
	d.mu.RUnlock()
	if cfg.Daemon.Socket != currentSocket {
		return fmt.Errorf("daemon.socket changes require a restart: systemctl --user restart lexicon.service")
	}

	store, storeErr := openStore(cfg.Dictionary.DatabasePath)
	provider, err := selection.New(selection.Options{
		Provider: cfg.Selection.Provider,
		Timeout:  time.Duration(cfg.Selection.TimeoutMS) * time.Millisecond,
		MaxChars: cfg.Selection.MaxChars,
	}, nil)
	if err != nil {
		closeStore(store)
		return err
	}
	translator := translation.New(translation.Options{
		Enabled:        cfg.Translation.Enabled,
		ModelDirectory: cfg.Translation.ModelDirectory,
		Timeout:        time.Duration(cfg.Translation.TimeoutSeconds) * time.Second,
	})

	d.mu.Lock()
	oldStore := d.store
	oldTranslator := d.translator
	d.cfg = cfg
	d.store = store
	d.storeErr = storeErr
	d.selection = provider
	d.translator = translator
	d.mu.Unlock()

	closeStore(oldStore)
	_ = oldTranslator.Close()
	if level, err := config.ParseLogLevel(cfg.Daemon.LogLevel); err != nil {
		d.logger.Warn("could not apply log level", "error", err)
	} else if d.levelVar != nil {
		d.levelVar.Set(level)
	}
	d.logger.Info("configuration reloaded", "socket", cfg.Daemon.Socket)
	return nil
}

func (d *Daemon) closeResources() {
	d.mu.Lock()
	store := d.store
	translator := d.translator
	d.store = nil
	d.translator = nil
	d.mu.Unlock()
	closeStore(store)
	_ = translator.Close()
}

func (d *Daemon) snapshotConfig() config.Config {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cfg
}

func (d *Daemon) selectionName() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.selection.Name()
}

func (d *Daemon) translationName() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.translator.Name()
}

func lookupAttributes(result *ipc.LookupResult, started time.Time) []any {
	attributes := []any{
		"found", result.Found,
		"query_chars", len([]rune(result.Normalized)),
		"duration_ms", time.Since(started).Milliseconds(),
	}
	if result.ErrorKind != "" {
		attributes = append(attributes, "kind", string(result.ErrorKind))
	}
	if result.TranslationError != "" {
		attributes = append(attributes, "translation_error", result.TranslationError)
	}
	return attributes
}

func (d *Daemon) logLookup(result *ipc.LookupResult, started time.Time) {
	d.logger.Info("lookup", lookupAttributes(result, started)...)
}

func openStore(path string) (*dictionary.Store, error) {
	store, err := dictionary.Open(path)
	if err != nil {
		return nil, fmt.Errorf("dictionary unavailable: %w", err)
	}
	return store, nil
}

func closeStore(store *dictionary.Store) {
	if store != nil {
		_ = store.Close()
	}
}

// popupArguments builds the popup command line. The replay flag is set only
// for daemon launches, which already have a result waiting on the socket.
func popupArguments(cfg config.Config, configPath string, replay bool) []string {
	arguments := []string{"--socket", cfg.Daemon.Socket}
	if replay {
		arguments = append(arguments, "--replay")
	}
	if configPath != "" {
		arguments = append(arguments, "--config", configPath)
	}
	return arguments
}

func popupBinary(cfg config.Config) (string, error) {
	if configured := strings.TrimSpace(cfg.Popup.Binary); configured != "" {
		path, err := paths.ExpandHome(configured)
		if err != nil {
			return "", err
		}
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path, nil
		}
		return "", fmt.Errorf("popup binary not found: %s", path)
	}

	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), "lexicon-popup")
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath("lexicon-popup"); err == nil {
		return path, nil
	}
	return "", errors.New("lexicon-popup was not found; build it with `just build-popup` or set popup.binary")
}

func selectionMessage(err error) string {
	switch {
	case errors.Is(err, selection.ErrNoText):
		return "No text is currently selected."
	case errors.Is(err, selection.ErrNotText):
		return "The current selection is not valid UTF-8 text."
	case errors.Is(err, selection.ErrDisabled):
		return "Primary selection lookup is disabled in the configuration."
	default:
		return err.Error()
	}
}
