//go:build !bergamot

package translation

import "errors"

// newEngine is the default build: translation is compiled out.
func newEngine(Options) (engine, error) {
	return nil, errors.New("rebuild with -tags bergamot and a linked Bergamot library to enable translation")
}
