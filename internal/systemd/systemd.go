// Package systemd installs the per-user Lexicon service. The daemon has no
// custom supervisor; service lifecycle stays with systemctl.
package systemd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// UnitName is the per-user daemon unit.
const UnitName = "lexicon.service"

// UnitPath returns $XDG_CONFIG_HOME/systemd/user/lexicon.service.
func UnitPath() (string, error) {
	base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "systemd", "user", UnitName), nil
}

// RenderUnit returns a user unit for the given daemon binary and config path.
// The popup is launched by the daemon, so the user manager needs
// WAYLAND_DISPLAY; see docs/INSTALL.md.
func RenderUnit(daemonBinary, configPath string) string {
	if strings.TrimSpace(configPath) == "" {
		configPath = "%h/.config/lexicon/config.toml"
	}
	return fmt.Sprintf(`[Unit]
Description=Lexicon selection lookup daemon
Documentation=https://github.com/PandaX185/lexicon
PartOf=graphical-session.target
After=graphical-session.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=2
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=%%t/lexicon
Environment=LEXICON_CONFIG=%s

[Install]
WantedBy=graphical-session.target
`, systemdEscape(daemonBinary), systemdEscape(configPath))
}

// Install writes the unit atomically and reloads the user manager.
func Install(ctx context.Context, unitPath, content string) error {
	//nolint:gosec // ~/.config/systemd/user is conventionally 0755
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("create systemd user directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(unitPath), ".lexicon-*.service")
	if err != nil {
		return fmt.Errorf("create temporary unit: %w", err)
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()

	if _, err := io.WriteString(temp, content); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write unit: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close unit: %w", err)
	}
	//nolint:gosec // unit files are read by the user's systemd manager, not secret
	if err := os.Chmod(tempName, 0o644); err != nil {
		return fmt.Errorf("chmod unit: %w", err)
	}
	if err := os.Rename(tempName, unitPath); err != nil {
		return fmt.Errorf("install unit: %w", err)
	}
	if err := run(ctx, "daemon-reload"); err != nil {
		return fmt.Errorf("reload systemd user manager: %w", err)
	}
	return nil
}

// Enable starts the unit now and on future logins.
func Enable(ctx context.Context) error {
	return run(ctx, "enable", "--now", UnitName)
}

// Reload asks the user manager to re-read unit files.
func Reload(ctx context.Context) error {
	return run(ctx, "daemon-reload")
}

func run(ctx context.Context, args ...string) error {
	path, err := exec.LookPath("systemctl")
	if err != nil {
		return errors.New("systemctl is not available on this system")
	}
	//nolint:gosec // callers pass fixed systemctl actions
	command := exec.CommandContext(ctx, path, append([]string{"--user"}, args...)...)
	if err := command.Run(); err != nil {
		return fmt.Errorf("systemctl --user %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func systemdEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, " ", `\x20`)
}
