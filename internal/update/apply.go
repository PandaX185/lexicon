package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type replacement struct {
	target string
	staged string
	backup string
	mode   os.FileMode
	done   bool
}

func newReplacement(target string, data []byte, mode os.FileMode) (replacement, error) {
	info, err := os.Stat(target)
	if err != nil {
		return replacement{}, fmt.Errorf("inspect %s: %w", target, err)
	}
	if !info.Mode().IsRegular() {
		return replacement{}, fmt.Errorf("%s is not a regular file", target)
	}
	original, err := os.ReadFile(filepath.Clean(target))
	if err != nil {
		return replacement{}, fmt.Errorf("read %s: %w", target, err)
	}
	dir := filepath.Dir(target)
	staged, err := writeTemp(dir, data, mode)
	if err != nil {
		return replacement{}, err
	}
	backup, err := writeTemp(dir, original, info.Mode().Perm())
	if err != nil {
		_ = os.Remove(staged)
		return replacement{}, err
	}
	return replacement{target: target, staged: staged, backup: backup, mode: mode}, nil
}

func writeTemp(dir string, data []byte, mode os.FileMode) (string, error) {
	file, err := os.CreateTemp(dir, ".lexicon-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	name := file.Name()
	fail := func(cause error) (string, error) {
		_ = file.Close()
		_ = os.Remove(name)
		return "", cause
	}
	if _, err := file.Write(data); err != nil {
		return fail(fmt.Errorf("write %s: %w", name, err))
	}
	if err := file.Chmod(mode); err != nil {
		return fail(fmt.Errorf("chmod %s: %w", name, err))
	}
	if err := file.Close(); err != nil {
		return fail(fmt.Errorf("close %s: %w", name, err))
	}
	return name, nil
}

func (r *replacement) commit() error {
	if err := os.Rename(r.staged, r.target); err != nil {
		return fmt.Errorf("replace %s: %w", r.target, err)
	}
	r.done = true
	return nil
}

func (r *replacement) rollback() error {
	if err := os.Rename(r.backup, r.target); err != nil {
		return fmt.Errorf("restore %s: %w", r.target, err)
	}
	return nil
}

func rollbackAll(items []*replacement) error {
	var result error
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.done {
			if err := item.rollback(); err != nil {
				result = errors.Join(result, err)
				continue // keep the backup so the user can recover manually
			}
		}
		_ = os.Remove(item.staged)
		_ = os.Remove(item.backup)
	}
	return result
}

// Apply stages every replacement, swaps them, and restarts the daemon. Any
// failure restores the original files and restarts the old daemon.
func (r *Runner) Apply(ctx context.Context, plan *Plan) error {
	if plan == nil {
		return errors.New("nil update plan")
	}
	targets := make([]struct {
		path string
		data []byte
		mode os.FileMode
	}, 0, len(binaryNames)+1)
	for _, name := range binaryNames {
		data, ok := plan.Binaries[name]
		if !ok {
			return fmt.Errorf("plan has no binary %s", name)
		}
		targets = append(targets, struct {
			path string
			data []byte
			mode os.FileMode
		}{filepath.Join(plan.BinDir, name), data, 0o755})
	}
	if plan.DictionaryPath != "" && plan.DictionarySkipped == "" && plan.Dictionary != nil {
		targets = append(targets, struct {
			path string
			data []byte
			mode os.FileMode
		}{plan.DictionaryPath, plan.Dictionary, 0o600})
	}

	items := make([]*replacement, 0, len(targets))
	fail := func(cause error) error {
		return errors.Join(cause, rollbackAll(items))
	}
	for _, target := range targets {
		item, err := newReplacement(target.path, target.data, target.mode)
		if err != nil {
			return fail(err)
		}
		items = append(items, &item)
	}
	for _, item := range items {
		if err := item.commit(); err != nil {
			return fail(err)
		}
	}
	if r.Restarter != nil && !r.NoRestart {
		if err := r.Restarter.Restart(ctx); err != nil {
			if errors.Is(err, ErrRestartUnavailable) {
				// systemctl is optional: keep the new files and let the CLI
				// report the notice instead of rolling back a valid update.
				plan.RestartSkipped = err.Error()
			} else {
				rollbackErr := rollbackAll(items)
				restartOldErr := r.Restarter.Restart(ctx)
				return errors.Join(fmt.Errorf("restart service: %w", err), rollbackErr, restartOldErr)
			}
		}
	}
	for _, item := range items {
		_ = os.Remove(item.backup)
	}
	return nil
}
