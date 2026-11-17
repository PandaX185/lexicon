package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PandaX185/lexicon/internal/paths"
)

// Status is the outcome of a preflight check.
type Status int

const (
	StatusOK Status = iota
	StatusWarn
	StatusFail
)

// String returns a short label suitable for text output.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusWarn:
		return "warn"
	default:
		return "fail"
	}
}

// Check is one preflight result.
type Check struct {
	Name   string
	Status Status
	Detail string
}

// CheckEnvironment reports on the session, tools, directories, and packaged
// assets. It never writes anything and never downloads anything.
func CheckEnvironment(ctx context.Context, options Options) []Check {
	var checks []Check
	add := func(name string, status Status, detail string) {
		checks = append(checks, Check{Name: name, Status: status, Detail: detail})
	}

	if display := strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")); display != "" {
		add("wayland", StatusOK, "WAYLAND_DISPLAY="+display)
	} else {
		add("wayland", StatusWarn, "WAYLAND_DISPLAY is not set; start the installer from a Wayland session")
	}

	if path, err := exec.LookPath("wl-paste"); err == nil {
		add("selection", StatusOK, path)
	} else {
		add("selection", StatusWarn, "wl-paste not found; install wl-clipboard for selection lookups")
	}

	if path, err := exec.LookPath("systemctl"); err == nil {
		add("systemd", StatusOK, path)
	} else {
		add("systemd", StatusWarn, "systemctl not found; the background service will not be installed")
	}

	if runtime := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); runtime != "" {
		add("runtime", StatusOK, runtime)
	} else {
		add("runtime", StatusWarn, "XDG_RUNTIME_DIR is not set; start the installer from a graphical session")
	}

	configPath := options.ConfigPath
	if configPath == "" {
		if resolved, err := paths.ConfigPath(); err == nil {
			configPath = resolved
		}
	}
	configDir := writableDir(filepath.Dir(configPath))
	add("config directory", configDir.Status, configDir.Detail)

	dataDir, err := paths.DataDir()
	if err != nil {
		add("data directory", StatusFail, err.Error())
	} else {
		dataCheck := writableDir(dataDir)
		add("data directory", dataCheck.Status, dataCheck.Detail)
	}

	daemon := findDaemonBinary(options.DaemonBinary)
	if daemon != "" {
		add("daemon", StatusOK, daemon)
	} else {
		add("daemon", StatusWarn, "lexicon-daemon not found; the background service will not be installed")
	}

	if popup := findPopupBinary(daemon); popup != "" {
		add("popup", StatusOK, popup)
	} else {
		add("popup", StatusWarn, "lexicon-popup not found; popups will not open")
	}

	_, statErr := os.Stat(configPath)
	configExists := statErr == nil
	packaged, err := findPackagedDictionary(searchDirs(options))
	switch {
	case options.DictionaryURL != "":
		add("dictionary", StatusWarn, "will be downloaded from "+options.DictionaryURL)
	case options.Dictionary != "" && (options.Force || !configExists):
		path, expandErr := paths.ExpandHome(options.Dictionary)
		switch {
		case expandErr != nil:
			add("dictionary", StatusFail, expandErr.Error())
		default:
			if validateErr := validateDictionary(ctx, path); validateErr != nil {
				add("dictionary", StatusFail, path+": "+validateErr.Error())
			} else {
				add("dictionary", StatusOK, path)
			}
		}
	case err != nil:
		add("dictionary", StatusFail, err.Error())
	case packaged != nil:
		version := ""
		if packaged.manifest != nil {
			version = packaged.manifest.Version
		}
		add("dictionary", StatusOK, packagedDetail(packaged.path, version, true))
	default:
		add("dictionary", StatusWarn, "no packaged dictionary found; provide one with --dictionary or --dictionary-url")
	}
	return checks
}

// replaceCheck swaps an existing check with the same name, or appends it.
func replaceCheck(checks []Check, replacement Check) []Check {
	for i := range checks {
		if checks[i].Name == replacement.Name {
			checks[i] = replacement
			return checks
		}
	}
	return append(checks, replacement)
}

// DiscoverPackaged returns the packaged dictionary path and version, or empty
// strings when none is installed.
func DiscoverPackaged(options Options) (path, version string, err error) {
	packaged, err := findPackagedDictionary(searchDirs(options))
	if err != nil {
		return "", "", err
	}
	if packaged == nil {
		return "", "", nil
	}
	if packaged.manifest != nil {
		version = packaged.manifest.Version
	}
	return packaged.path, version, nil
}

// writableDir checks the nearest existing ancestor. It does not create
// directories, so a dry run leaves the filesystem untouched.
func writableDir(dir string) Check {
	if strings.TrimSpace(dir) == "" {
		return Check{Name: "directory", Status: StatusWarn, Detail: "path could not be resolved"}
	}
	current := dir
	for {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return Check{Name: "directory", Status: StatusFail, Detail: current + " is not a directory"}
			}
			if info.Mode().Perm()&0o200 == 0 {
				return Check{Name: "directory", Status: StatusFail, Detail: current + " is not writable"}
			}
			return Check{Name: "directory", Status: StatusOK, Detail: dir}
		}
		if !errors.Is(err, os.ErrNotExist) {
			return Check{Name: "directory", Status: StatusFail, Detail: err.Error()}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return Check{Name: "directory", Status: StatusFail, Detail: "no writable parent for " + dir}
		}
		current = parent
	}
}

func findPopupBinary(daemon string) string {
	if path, err := exec.LookPath("lexicon-popup"); err == nil {
		return path
	}
	dirs := []string{}
	if daemon != "" {
		dirs = append(dirs, filepath.Dir(daemon))
	}
	if executable, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(executable))
	}
	for _, dir := range dirs {
		candidate := filepath.Join(dir, "lexicon-popup")
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}
