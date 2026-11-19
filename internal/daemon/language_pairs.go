package daemon

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/PandaX185/lexicon/internal/config"
	"github.com/PandaX185/lexicon/internal/ipc"
	"github.com/PandaX185/lexicon/internal/translation"
)

var _ ipc.LanguagePairHandler = (*Daemon)(nil)

// LanguagePairs reports installed, selectable translation directions.
func (d *Daemon) LanguagePairs() ipc.LanguagePairsState {
	d.mu.RLock()
	cfg := d.cfg
	available := false
	if d.translator != nil {
		available = d.translator.Available()
	}
	d.mu.RUnlock()
	return languagePairsFromConfig(cfg, available)
}

// SetLanguagePair validates a requested pair and reports it in the returned
// state. The pair is not persisted to configuration.
func (d *Daemon) SetLanguagePair(ctx context.Context, source, target string) (ipc.LanguagePairsState, error) {
	source = translation.CanonicalLanguageCode(source)
	target = translation.CanonicalLanguageCode(target)
	if !translation.ValidLanguageCode(source) || !translation.ValidLanguageCode(target) {
		return ipc.LanguagePairsState{}, fmt.Errorf("invalid language pair %q -> %q", source, target)
	}

	raw, exists, err := config.Load(d.configPath)
	if err != nil {
		return ipc.LanguagePairsState{}, fmt.Errorf("read configuration: %w", err)
	}
	if !exists {
		return ipc.LanguagePairsState{}, fmt.Errorf("config file not found: %s", d.configPath)
	}
	effective := config.Effective(raw)
	state := languagePairsFromConfig(effective, true)
	if !state.Enabled {
		return state, errors.New("translation is disabled")
	}
	found := false
	for _, pair := range state.Pairs {
		if pair.Source == source && pair.Target == target {
			found = true
			break
		}
	}
	if !found {
		if _, findErr := translation.FindPack(effective.Translation.ModelDirectory, source, target); findErr != nil {
			if errors.Is(findErr, translation.ErrNoModel) {
				return state, fmt.Errorf("no installed model pack translates %s -> %s", source, target)
			}
			return state, fmt.Errorf("find model pack: %w", findErr)
		}
	}

	state.Source = source
	state.Target = target
	return state, nil
}

func languagePairsFromConfig(cfg config.Config, available bool) ipc.LanguagePairsState {
	state := ipc.LanguagePairsState{
		Enabled:   cfg.Translation.Enabled,
		Available: available,
		Pairs:     []ipc.LanguagePair{},
	}
	packs, err := translation.InstalledPacks(cfg.Translation.ModelDirectory)
	if err != nil {
		return state
	}
	seen := map[string]bool{}
	for _, pack := range packs {
		key := pack.Source + "->" + pack.Target
		if seen[key] {
			continue
		}
		seen[key] = true
		state.Pairs = append(state.Pairs, ipc.LanguagePair{
			Source: pack.Source,
			Target: pack.Target,
			PackID: pack.ID,
		})
	}
	sort.Slice(state.Pairs, func(i, j int) bool {
		if state.Pairs[i].Source == state.Pairs[j].Source {
			return state.Pairs[i].Target < state.Pairs[j].Target
		}
		return state.Pairs[i].Source < state.Pairs[j].Source
	})
	return state
}
