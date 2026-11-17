//go:build bergamot

// Package bergamotengine is the isolated cgo boundary around Bergamot.
//
// It is only compiled with the `bergamot` tag and requires the Bergamot
// headers and libraries. The C++ side owns all engine objects, converts
// exceptions into error strings, and exposes opaque handles; no C++ type or
// exception crosses into Go.
package bergamotengine

/*
#cgo CXXFLAGS: -std=c++17 -O2
#cgo LDFLAGS: -lbergamot-translator -lz -lpthread -lstdc++
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unsafe"
)

// Engine owns one native Bergamot service and lazily-created models.
type Engine struct {
	handle *C.lexicon_bergamot
}

// New creates the native service. Model creation happens on the first
// Translate call because it needs a pack-specific configuration.
func New(timeout time.Duration) (*Engine, error) {
	var cError *C.char
	handle := C.lexicon_bergamot_create(&cError)
	if handle == nil {
		return nil, errors.New(cErrorMessage(cError))
	}
	return &Engine{handle: handle}, nil
}

func (e *Engine) Name() string { return "bergamot" }

// Translate creates or reuses a model for modelConfig and translates text.
// The native call is bounded by timeout; ctx cancellation cannot interrupt it.
func (e *Engine) Translate(ctx context.Context, modelConfig, text string) (string, error) {
	if e == nil || e.handle == nil {
		return "", errors.New("translation engine is closed")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return "", context.DeadlineExceeded
		}
		timeout = remaining
	}

	cConfig := C.CString(modelConfig)
	defer C.free(unsafe.Pointer(cConfig))
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	var cError *C.char
	result := C.lexicon_bergamot_translate(
		e.handle,
		cConfig,
		cText,
		C.int(timeout.Milliseconds()),
		&cError,
	)
	if result == nil {
		return "", errors.New(cErrorMessage(cError))
	}
	defer C.lexicon_bergamot_string_free(result)
	return C.GoString(result), nil
}

// Close releases models and the native service.
func (e *Engine) Close() error {
	if e == nil || e.handle == nil {
		return nil
	}
	C.lexicon_bergamot_destroy(e.handle)
	e.handle = nil
	return nil
}

func cErrorMessage(cError *C.char) string {
	if cError == nil {
		return "bergamot: unknown native error"
	}
	defer C.lexicon_bergamot_string_free(cError)
	return fmt.Sprintf("bergamot: %s", C.GoString(cError))
}
