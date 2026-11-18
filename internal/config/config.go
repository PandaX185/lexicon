// Package config defines, loads, validates, and writes Lexicon's TOML
// configuration. The CLI is the only supported way to change it.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/PandaX185/lexicon/internal/paths"
)

// CurrentVersion is the configuration schema version. It is not written to the
// file; the layout is additive so missing keys take defaults.
const CurrentVersion = 1

// Config is the complete on-disk configuration.
type Config struct {
	General     General     `toml:"general" comment:"Query limits and general behavior."`
	Daemon      Daemon      `toml:"daemon" comment:"Daemon socket and logging."`
	Selection   Selection   `toml:"selection" comment:"Wayland primary-selection retrieval."`
	Dictionary  Dictionary  `toml:"dictionary" comment:"Dictionary location and language pair."`
	Translation Translation `toml:"translation" comment:"Optional offline Bergamot translation."`
	Popup       Popup       `toml:"popup" comment:"Popup window behavior."`
}

type General struct {
	// MaxQueryWords caps how many whitespace-separated words a lookup may contain.
	MaxQueryWords int `toml:"max_query_words" comment:"Maximum words in a lookup (1-16)."`
	// MaxQueryChars caps the normalized query length in Unicode code points.
	MaxQueryChars int `toml:"max_query_chars" comment:"Maximum characters in a lookup (1-512)."`
}

type Daemon struct {
	Socket   string `toml:"socket" comment:"Unix socket path. Empty uses $XDG_RUNTIME_DIR/lexicon/daemon.sock."`
	LogLevel string `toml:"log_level" comment:"debug, info, warn, or error."`
}

type Selection struct {
	Provider  string `toml:"provider" comment:"auto, wl-paste, or none."`
	TimeoutMS int    `toml:"timeout_ms" comment:"Selection read timeout in milliseconds (50-5000)."`
	MaxChars  int    `toml:"max_chars" comment:"Maximum selection characters read (1-512)."`
}

type Dictionary struct {
	DatabasePath      string `toml:"database_path" comment:"SQLite dictionary. Empty uses $XDG_DATA_HOME/lexicon/dictionary.sqlite."`
	SourceLanguage    string `toml:"source_language" comment:"BCP 47 tag of headwords."`
	TargetLanguage    string `toml:"target_language" comment:"BCP 47 tag of definitions."`
	ShowPronunciation bool   `toml:"show_pronunciation" comment:"Show IPA or respelling when present."`
}

type Translation struct {
	Enabled        bool   `toml:"enabled" comment:"Translate the looked-up word when source and target differ."`
	ModelDirectory string `toml:"model_directory" comment:"Installed Bergamot model packs. Empty uses $XDG_DATA_HOME/lexicon/models."`
	SourceLanguage string `toml:"source_language" comment:"BCP 47 tag translated from."`
	TargetLanguage string `toml:"target_language" comment:"BCP 47 tag translated to."`
	TimeoutSeconds int    `toml:"timeout_seconds" comment:"Translation timeout in seconds (5-300)."`
}

type Popup struct {
	AutoLaunch bool   `toml:"auto_launch" comment:"Start the popup when a lookup has no subscriber."`
	Binary     string `toml:"binary" comment:"Popup executable. Empty looks next to lexicon-daemon."`
	Width      int    `toml:"width" comment:"Popup width in logical pixels (240-1200)."`
	Height     int    `toml:"height" comment:"Popup height in logical pixels (160-1200)."`
	HideOnBlur bool   `toml:"hide_on_blur" comment:"Hide the popup when it loses focus."`

	Theme      string `toml:"theme" comment:"Built-in colour preset: system, one of the dark presets, or one of the light presets."`
	ShowHeader bool   `toml:"show_header" comment:"Show the brand header."`
	ShowStatus bool   `toml:"show_status" comment:"Show the daemon connection badge."`
	ShowFooter bool   `toml:"show_footer" comment:"Show the timestamp and close hint."`
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		General: General{
			MaxQueryWords: 4,
			MaxQueryChars: 128,
		},
		Daemon: Daemon{
			LogLevel: "info",
		},
		Selection: Selection{
			Provider:  "auto",
			TimeoutMS: 400,
			MaxChars:  128,
		},
		Dictionary: Dictionary{
			SourceLanguage:    "en",
			TargetLanguage:    "en",
			ShowPronunciation: true,
		},
		Translation: Translation{
			Enabled:        false,
			SourceLanguage: "en",
			TargetLanguage: "en",
			TimeoutSeconds: 30,
		},
		Popup: Popup{
			AutoLaunch: true,
			Width:      420,
			Height:     320,
			HideOnBlur: true,
			Theme:      "system",
		},
	}
}

// Load reads path. A missing file yields the defaults with exists=false; it is
// not an error so first-run use works before the installer has written config.
func Load(path string) (Config, bool, error) {
	cfg := Default()

	//nolint:gosec // the path is the caller-selected configuration file
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, false, nil
	}
	if err != nil {
		return Config{}, false, fmt.Errorf("read config %s: %w", path, err)
	}

	decoder := toml.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, false, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, false, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, true, nil
}

// Write validates cfg and writes it atomically to path.
func Write(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := Render(cfg)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o600)
}

// Render returns the canonical TOML representation with a short header.
func Render(cfg Config) ([]byte, error) {
	body, err := toml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	header := "# Lexicon configuration. Edit with `lex config set` or `lex config edit`.\n" +
		"# Values shown by `lex config show` are effective values, including XDG defaults.\n\n"
	return append([]byte(header), body...), nil
}

// Effective returns a copy with empty path fields resolved to XDG defaults.
func Effective(cfg Config) Config {
	if strings.TrimSpace(cfg.Daemon.Socket) == "" {
		if socket, err := paths.DefaultSocketPath(); err == nil {
			cfg.Daemon.Socket = socket
		}
	}
	if strings.TrimSpace(cfg.Dictionary.DatabasePath) == "" {
		if dictionary, err := paths.DefaultDictionaryPath(); err == nil {
			cfg.Dictionary.DatabasePath = dictionary
		}
	}
	if strings.TrimSpace(cfg.Translation.ModelDirectory) == "" {
		if models, err := paths.DefaultModelDir(); err == nil {
			cfg.Translation.ModelDirectory = models
		}
	}
	return cfg
}

// Ensure writes the default configuration when path does not exist.
func Ensure(path string) (created bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat config %s: %w", path, err)
	}
	if err := Write(path, Default()); err != nil {
		return false, err
	}
	return true, nil
}

// Validate checks enums and ranges. It never touches the filesystem.
func (c Config) Validate() error {
	var problems []string
	check := func(ok bool, format string, args ...any) {
		if !ok {
			problems = append(problems, fmt.Sprintf(format, args...))
		}
	}

	check(c.General.MaxQueryWords >= 1 && c.General.MaxQueryWords <= 16,
		"general.max_query_words must be between 1 and 16")
	check(c.General.MaxQueryChars >= 1 && c.General.MaxQueryChars <= 512,
		"general.max_query_chars must be between 1 and 512")

	if _, err := ParseLogLevel(c.Daemon.LogLevel); err != nil {
		problems = append(problems, "daemon.log_level must be debug, info, warn, or error")
	}

	switch strings.ToLower(strings.TrimSpace(c.Selection.Provider)) {
	case "auto", "wl-paste", "none":
	default:
		problems = append(problems, "selection.provider must be auto, wl-paste, or none")
	}
	check(c.Selection.TimeoutMS >= 50 && c.Selection.TimeoutMS <= 5000,
		"selection.timeout_ms must be between 50 and 5000")
	check(c.Selection.MaxChars >= 1 && c.Selection.MaxChars <= 512,
		"selection.max_chars must be between 1 and 512")

	check(validLanguage(c.Dictionary.SourceLanguage), "dictionary.source_language must be a lowercase BCP 47 tag")
	check(validLanguage(c.Dictionary.TargetLanguage), "dictionary.target_language must be a lowercase BCP 47 tag")

	check(c.Translation.TimeoutSeconds >= 5 && c.Translation.TimeoutSeconds <= 300,
		"translation.timeout_seconds must be between 5 and 300")
	check(validLanguage(c.Translation.SourceLanguage), "translation.source_language must be a lowercase BCP 47 tag")
	check(validLanguage(c.Translation.TargetLanguage), "translation.target_language must be a lowercase BCP 47 tag")

	check(c.Popup.Width >= 240 && c.Popup.Width <= 1200,
		"popup.width must be between 240 and 1200")
	check(c.Popup.Height >= 160 && c.Popup.Height <= 1200,
		"popup.height must be between 160 and 1200")

	check(ValidPopupTheme(c.Popup.Theme),
		"popup.theme must be one of: %s", strings.Join(PopupThemes, ", "))

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// ParseLogLevel maps the configured level to slog.Level. "warning" is accepted
// as an alias for "warn".
func ParseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", value)
	}
}

var languagePattern = regexp.MustCompile(`^[a-z]{2,3}(-[a-z0-9]{2,8})*$`)

func validLanguage(tag string) bool {
	return languagePattern.MatchString(strings.TrimSpace(tag))
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}
	temp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Chmod(tempName, mode); err != nil {
		return fmt.Errorf("chmod config: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
