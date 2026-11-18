package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/systemd"
)

// Progress describes one apply step.
type Progress struct {
	Step  string
	Index int
	Total int
}

// Reporter receives progress updates. It is called from the goroutine running
// Apply, so a UI reporter must not block indefinitely.
type Reporter func(Progress)

// Apply performs the plan. On failure it restores the previous configuration,
// dictionary, and unit file before returning the original error.
func Apply(ctx context.Context, plan Plan, report Reporter) error {
	steps := plan.Steps()
	total := len(steps)
	index := 0
	next := func(name string) {
		index++
		if report != nil {
			report(Progress{Step: name, Index: index, Total: total})
		}
	}

	var (
		backups     []*fileBackup
		downloaded  string
		unitTouched bool
	)
	rollback := func() error {
		var rollbackErr error
		for _, backup := range slices.Backward(backups) {
			if err := backup.restore(); err != nil && rollbackErr == nil {
				rollbackErr = err
			}
		}
		if unitTouched {
			reloadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if err := systemd.Reload(reloadCtx); err != nil && rollbackErr == nil {
				rollbackErr = err
			}
		}
		return rollbackErr
	}
	fail := func(cause error) error {
		if downloaded != "" {
			_ = os.Remove(downloaded)
		}
		if rollbackErr := rollback(); rollbackErr != nil {
			return fmt.Errorf("%w; rollback: %w", cause, rollbackErr)
		}
		return cause
	}

	if plan.WriteConfig {
		next("Writing configuration")
		backup, err := backupFile(plan.ConfigPath)
		if err != nil {
			return fail(fmt.Errorf("back up configuration: %w", err))
		}
		backups = append(backups, backup)
		if err := config.Write(plan.ConfigPath, plan.Config); err != nil {
			return fail(fmt.Errorf("write configuration: %w", err))
		}
	}

	if plan.DownloadURL != "" {
		next("Downloading dictionary")
		var err error
		downloaded, err = downloadDictionary(ctx, plan.DownloadURL, plan.DownloadSHA256, nil)
		if err != nil {
			return fail(err)
		}
		if err := validateDictionary(ctx, downloaded); err != nil {
			return fail(fmt.Errorf("downloaded dictionary: %w", err))
		}
		plan.DictionarySource = downloaded
	}

	if plan.CopyDictionary {
		next("Installing dictionary")
		backup, err := backupFile(plan.DictionaryTarget)
		if err != nil {
			return fail(fmt.Errorf("back up dictionary: %w", err))
		}
		backups = append(backups, backup)
		if err := copyFile(plan.DictionarySource, plan.DictionaryTarget); err != nil {
			return fail(fmt.Errorf("install dictionary: %w", err))
		}
	}

	if plan.InstallUnit {
		next("Installing systemd unit")
		backup, err := backupFile(plan.UnitPath)
		if err != nil {
			return fail(fmt.Errorf("back up systemd unit: %w", err))
		}
		backups = append(backups, backup)
		unitTouched = true
		if err := systemd.Install(ctx, plan.UnitPath, plan.Unit); err != nil {
			return fail(fmt.Errorf("install systemd unit: %w", err))
		}
	}

	if plan.EnableUnit {
		next("Starting service")
		if err := systemd.Enable(ctx); err != nil {
			return fail(fmt.Errorf("start service: %w", err))
		}
	}

	for _, backup := range backups {
		backup.discard()
	}
	if downloaded != "" {
		_ = os.Remove(downloaded)
	}
	return nil
}

// fileBackup holds a copy of a file that Apply may replace. Restoring is a
// copy-and-rename so a partially written file is never left behind.
type fileBackup struct {
	path    string
	backup  string
	existed bool
}

func backupFile(path string) (*fileBackup, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return &fileBackup{path: path}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".lexicon-backup-*")
	if err != nil {
		return nil, err
	}
	tempName := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}

	//nolint:gosec // path is a configuration, unit, or dictionary file
	input, err := os.Open(path)
	if err != nil {
		cleanup()
		return nil, err
	}
	if _, err := io.Copy(temp, input); err != nil {
		_ = input.Close()
		cleanup()
		return nil, err
	}
	if err := input.Close(); err != nil {
		cleanup()
		return nil, err
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return nil, err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return nil, err
	}
	if err := os.Chmod(tempName, info.Mode().Perm()); err != nil {
		_ = os.Remove(tempName)
		return nil, err
	}
	return &fileBackup{path: path, backup: tempName, existed: true}, nil
}

func (b *fileBackup) restore() error {
	if !b.existed {
		if err := os.Remove(b.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", b.path, err)
		}
		return nil
	}
	if err := os.Rename(b.backup, b.path); err != nil {
		return fmt.Errorf("restore %s: %w", b.path, err)
	}
	b.backup = ""
	return nil
}

func (b *fileBackup) discard() {
	if b.backup == "" {
		return
	}
	_ = os.Remove(b.backup)
	b.backup = ""
}
