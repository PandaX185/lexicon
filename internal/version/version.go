// Package version exposes build metadata. Values are overridden with -ldflags.
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version of this build.
	Version = "0.1.0-dev"
	// Commit is the source revision, set by the build.
	Commit = "unknown"
	// BuildDate is the RFC 3339 build timestamp, set by the build.
	BuildDate = "unknown"
)

// String returns a version line for the Lexicon project.
func String() string { return StringFor("lexicon") }

// StringFor returns a version line prefixed with the given command name.
func StringFor(name string) string {
	return fmt.Sprintf("%s %s (%s, %s, %s/%s, go %s)",
		name, Version, Commit, BuildDate, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
