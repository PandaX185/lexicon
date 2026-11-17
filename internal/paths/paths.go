// Package paths resolves Lexicon's XDG locations.
package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoRuntimeDir is returned when the session does not provide a secure
// XDG_RUNTIME_DIR. Lexicon refuses to place its socket in a world-writable
// fallback location.
var ErrNoRuntimeDir = errors.New("XDG_RUNTIME_DIR is not set; cannot create a secure runtime directory")

// ConfigPath returns the configuration file location.
//
// LEXICON_CONFIG overrides the location; it is intended for tests and for
// running a second instance during development.
func ConfigPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("LEXICON_CONFIG")); override != "" {
		return expandHome(override)
	}
	base, err := configHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "lexicon", "config.toml"), nil
}

// DataDir returns the per-user data directory for Lexicon.
func DataDir() (string, error) {
	base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "lexicon"), nil
}

// DefaultDictionaryPath returns where the installer expects the dictionary.
func DefaultDictionaryPath() (string, error) {
	data, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "dictionary.sqlite"), nil
}

// DefaultModelDir returns where optional translation models live.
func DefaultModelDir() (string, error) {
	data, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "models"), nil
}

// RuntimeDir returns the per-user runtime directory that holds the socket.
func RuntimeDir() (string, error) {
	base := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR"))
	if base == "" {
		return "", ErrNoRuntimeDir
	}
	return filepath.Join(base, "lexicon"), nil
}

// DefaultSocketPath returns the daemon socket location.
func DefaultSocketPath() (string, error) {
	runtimeDir, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(runtimeDir, "daemon.sock"), nil
}

// ExpandHome expands a leading ~/ and returns an absolute path for relative
// input.
func ExpandHome(path string) (string, error) {
	return expandHome(path)
}

func expandHome(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is empty")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		path = filepath.Join(home, path[2:])
	}
	if !filepath.IsAbs(path) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("resolve %q: %w", path, err)
		}
		return absolute, nil
	}
	return filepath.Clean(path), nil
}

func configHome() (string, error) {
	if base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); base != "" {
		return base, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}
