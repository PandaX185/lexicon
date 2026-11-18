package ipc

import "context"

// LanguagePair is one installed, selectable translation direction.
type LanguagePair struct {
	Source string `json:"source"`
	Target string `json:"target"`
	PackID string `json:"pack_id"`
}

// LanguagePairsState is the popup selector's complete view of translation.
type LanguagePairsState struct {
	Enabled   bool           `json:"enabled"`
	Available bool           `json:"available"`
	Source    string         `json:"source"`
	Target    string         `json:"target"`
	Pairs     []LanguagePair `json:"pairs"`
}

// LanguagePairHandler is an optional Handler extension. Daemons that do not
// implement it cause unsupported protocol errors rather than build failures.
type LanguagePairHandler interface {
	LanguagePairs() LanguagePairsState
	SetLanguagePair(ctx context.Context, source, target string) (LanguagePairsState, error)
}
