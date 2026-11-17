//go:build bergamot

package translation

import (
	bergamotengine "github.com/PandaX185/lexicon/internal/translation/bergamotengine"
)

// newEngine links the native adapter. Building this tag requires a Bergamot
// build tree; see docs/TRANSLATION.md.
func newEngine(options Options) (engine, error) {
	return bergamotengine.New(options.Timeout)
}
