// Package translation defines the optional offline translation boundary.
//
// Default builds contain no engine and report ErrUnavailable. The
// `bergamot` build tag links the native Bergamot adapter; see
// docs/TRANSLATION.md.
package translation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Errors returned by the translation service.
var (
	// ErrDisabled means translation is turned off in the config.
	ErrDisabled = errors.New("offline translation is disabled")
	// ErrUnavailable means this build has no translation engine.
	ErrUnavailable = errors.New("this build of Lexicon does not include the translation engine")
	// ErrNoModel means no installed pack covers the requested pair.
	ErrNoModel = errors.New("no offline translation model is installed for this language pair")
)

// Options configures the service.
type Options struct {
	Enabled        bool
	ModelDirectory string
	Timeout        time.Duration
}

// Service resolves installed model packs and delegates to the engine.
type Service struct {
	enabled  bool
	modelDir string
	timeout  time.Duration
	engine   engine
	reason   string
}

// engine is the native (or stub) translation implementation.
type engine interface {
	Name() string
	Translate(ctx context.Context, modelConfig, text string) (string, error)
	Close() error
}

// New builds the service. A disabled or unavailable configuration is not an
// error: Translate reports it, so lookups still return definitions.
func New(options Options) *Service {
	service := &Service{
		enabled:  options.Enabled,
		modelDir: options.ModelDirectory,
		timeout:  options.Timeout,
	}
	if service.timeout <= 0 {
		service.timeout = 30 * time.Second
	}
	if !options.Enabled {
		return service
	}
	built, err := newEngine(options)
	if err != nil {
		service.reason = err.Error()
		return service
	}
	service.engine = built
	return service
}

// Name reports the active engine or why translation is unavailable.
func (s *Service) Name() string {
	if !s.enabled {
		return "disabled"
	}
	if s.engine == nil {
		if s.reason != "" {
			return "unavailable: " + s.reason
		}
		return "unavailable"
	}
	return s.engine.Name()
}

// Available reports whether translation can be attempted.
func (s *Service) Available() bool { return s.enabled && s.engine != nil }

// Close releases the engine, if any.
func (s *Service) Close() error {
	if s == nil || s.engine == nil {
		return nil
	}
	return s.engine.Close()
}

// Translate returns text translated from source to target. Translation is
// best-effort metadata: callers must not discard a definition when this fails.
func (s *Service) Translate(ctx context.Context, text, source, target string) (string, error) {
	if !s.enabled {
		return "", ErrDisabled
	}
	if s.engine == nil {
		if s.reason != "" {
			return "", fmt.Errorf("%w: %s", ErrUnavailable, s.reason)
		}
		return "", ErrUnavailable
	}
	source = CanonicalLanguageCode(source)
	target = CanonicalLanguageCode(target)
	if source == "" || target == "" {
		return "", errors.New("a source and target language are required")
	}
	if source == target {
		return strings.TrimSpace(text), nil
	}

	pack, err := FindPack(s.modelDir, source, target)
	if err != nil {
		return "", err
	}
	modelConfig, err := pack.ModelConfig()
	if err != nil {
		return "", err
	}

	callCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		callCtx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	return s.engine.Translate(callCtx, modelConfig, strings.TrimSpace(text))
}

// CanonicalLanguageCode maps common hand-written tags to the lowercase BCP 47
// codes used by model packs, matching the C++ implementation.
func CanonicalLanguageCode(code string) string {
	normalized := strings.ToLower(strings.TrimSpace(code))
	if canonical, ok := languageAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

var languageAliases = map[string]string{
	"jp": "ja", "jpn": "ja",
	"cn": "zh", "zho": "zh", "chi": "zh",
	"kr": "ko", "kor": "ko",
	"deu": "de", "ger": "de",
	"fra": "fr", "fre": "fr",
	"spa": "es",
	"ita": "it",
	"por": "pt",
	"rus": "ru",
	"ukr": "uk",
	"ell": "el", "gre": "el",
	"ces": "cs", "cze": "cs",
	"nld": "nl", "dut": "nl",
	"pol": "pl",
	"tur": "tr",
	"ara": "ar",
	"hin": "hi",
}
