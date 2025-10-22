package gazelle

import (
	"context"
	"fmt"
	"os"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"

	"github.com/bazelbuild/bazel-gazelle/config"
)

// NOTE: must align with patched/vendored gazelle code injecting context into config.Exts
const gazelleContextCancelKey = "aspect:context.cancel"

// MisconfiguredErrorf reports a misconfiguration error on the user's part.
//
// This indicates a problem with the gazelle configuration such as directive values or
// other static setup that should be fixed in the gazelle setup.
//
// If possible, the gazelle execution is cancelled. If cancellation is not setup, the
// process may exit.
func MisconfiguredErrorf(c *config.Config, msg string, args ...interface{}) {
	cancelOrFatal(c, msg, args...)
}

func GenerationErrorf(c *config.Config, msg string, args ...interface{}) {
	cancelOrFatal(c, msg, args...)
}

func ImportErrorf(c *config.Config, msg string, args ...interface{}) {
	// TODO: only log if running in non-strict mode?

	cancelOrFatal(c, msg, args...)
}

func cancelOrFatal(c *config.Config, msg string, args ...interface{}) {
	if ctxCancel, ctxExists := c.Exts[gazelleContextCancelKey]; ctxExists {
		ctxCancel.(context.CancelCauseFunc)(fmt.Errorf(msg, args...))
		return
	}

	fmt.Fprintf(os.Stderr, msg, args...)
	BazelLog.Fatalf(msg, args...)
}
