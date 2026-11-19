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

// LanguagePairs reports installed, selectable translation directions and the
// active pair for this daemon session.
func (d *Daemon) LanguagePairs() ipc.LanguagePairsState {
	d.mu.RLock()
	cfg := d.cfg
	available := false
	if d.translator != nil {
		available = d.translator.Available()
	}
	d.mu.RUnlock()

	d.pairMu.Lock()
	source, target := d.pairSource, d.pairTarget
	d.pairMu.Unlock()

	return languagePairsFromConfig(cfg, available, source, target)
}

// SetLanguagePair validates a requested pair and activates it for the current
// daemon session only. The IPC server broadcasts TypeLanguagePairsEvent with
// the returned state; nothing is written to configuration.
func (d *Daemon) SetLanguagePair(_ context.Context, source, target string) (ipc.LanguagePairsState, error) {
	source = translation.CanonicalLanguageCode(source)
	target = translation.CanonicalLanguageCode(target)
	if !translation.ValidLanguageCode(source) || !translation.ValidLanguageCode(target) {
		return ipc.LanguagePairsState{}, fmt.Errorf("invalid language pair %q -> %q", source, target)
	}

	cfg := d.snapshotConfig()
	available := d.LanguagePairs().Available
	d.pairMu.Lock()
	sessionSource, sessionTarget := d.pairSource, d.pairTarget
	d.pairMu.Unlock()
	state := languagePairsFromConfig(cfg, available, sessionSource, sessionTarget)
	if !state.Enabled {
		return state, errors.New("translation is disabled")
	}
	if _, err := translation.FindPack(cfg.Translation.ModelDirectory, source, target); err != nil {
		if errors.Is(err, translation.ErrNoModel) {
			return state, fmt.Errorf("no installed model pack translates %s -> %s", source, target)
		}
		return state, fmt.Errorf("find model pack: %w", err)
	}

	d.pairMu.Lock()
	d.pairSource = source
	d.pairTarget = target
	d.pairMu.Unlock()

	state.Source = source
	state.Target = target
	return state, nil
}

func languagePairsFromConfig(cfg config.Config, available bool, sessionSource, sessionTarget string) ipc.LanguagePairsState {
	state := ipc.LanguagePairsState{
		Enabled:   cfg.Translation.Enabled,
		Available: available,
		Pairs:     []ipc.LanguagePair{},
	}
	state.Source, state.Target = languagePairsFromSession(sessionSource, sessionTarget)
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

	if state.Source == "" || state.Target == "" {
		if len(state.Pairs) > 0 {
			state.Source, state.Target = state.Pairs[0].Source, state.Pairs[0].Target
		}
	}
	return state
}

func languagePairsFromSession(source, target string) (string, string) {
	return translation.CanonicalLanguageCode(source), translation.CanonicalLanguageCode(target)
}
