package gazelle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"

	"github.com/bazelbuild/bazel-gazelle/config"
)

const (
	gazelleContextKey             = "aspect:context"
	gazelleContextCancelKey       = "aspect:context.cancel"
	gazelleContextCancelErrorsKey = "aspect:context.cancel.errors"
)

// SetupCancellableContext wraps ctx in a cancel-with-cause context, registers
// it (and a fresh error accumulator) on c.Exts so GenerationErrorf et al. can
// route their messages here, and returns the wrapped context.
func SetupCancellableContext(c *config.Config, ctx context.Context) context.Context {
	ctx, cancelWithCause := context.WithCancelCause(ctx)
	c.Exts[gazelleContextKey] = ctx
	c.Exts[gazelleContextCancelKey] = cancelWithCause
	c.Exts[gazelleContextCancelErrorsKey] = &errAccumulator{}
	return ctx
}

// CheckCancellation returns the joined accumulated errors the first time it
// observes a cancelled context, and nil on every subsequent call. Returning
// once prevents downstream collectors (e.g. walk's w.errs) from accumulating
// duplicate references to the same accumulator and emitting its joined
// message N times via errors.Join.
func CheckCancellation(c *config.Config) error {
	ctx, ok := c.Exts[gazelleContextKey].(context.Context)
	if !ok || ctx.Err() == nil {
		return nil
	}
	return c.Exts[gazelleContextCancelErrorsKey].(*errAccumulator).surface()
}

// MisconfiguredErrorf reports a misconfiguration error on the user's part.
//
// This indicates a problem with the gazelle configuration such as directive values or
// other static setup that should be fixed in the gazelle setup.
//
// If possible, the gazelle execution is cancelled. If cancellation is not setup, the
// process may exit.
func MisconfiguredErrorf(c *config.Config, msg string, args ...any) {
	cancelOrFatal(c, msg, args...)
}

func GenerationErrorf(c *config.Config, msg string, args ...any) {
	cancelOrFatal(c, msg, args...)
}

func ImportErrorf(c *config.Config, msg string, args ...any) {
	// TODO: only log if running in non-strict mode?

	cancelOrFatal(c, msg, args...)
}

func cancelOrFatal(c *config.Config, msg string, args ...any) {
	if acc, ok := c.Exts[gazelleContextCancelErrorsKey].(*errAccumulator); ok {
		acc.add(fmt.Errorf(msg, args...))
		if ctxCancel, ctxExists := c.Exts[gazelleContextCancelKey]; ctxExists {
			ctxCancel.(context.CancelCauseFunc)(acc)
			return
		}
	}

	fmt.Fprintf(os.Stderr, msg, args...)
	BazelLog.Fatalf(msg, args...)
}

// errAccumulator is an error that joins every error appended to it; passed as
// the cancel-cause so context.Cause(ctx) always reflects the up-to-date join.
type errAccumulator struct {
	mu       sync.Mutex
	errs     []error
	surfaced bool
}

// add appends err to the join.
func (a *errAccumulator) add(err error) {
	a.mu.Lock()
	a.errs = append(a.errs, err)
	a.mu.Unlock()
}

// surface returns the accumulator once, then nil — so walk's w.errs gets a single entry.
func (a *errAccumulator) surface() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.surfaced || len(a.errs) == 0 {
		return nil
	}
	a.surfaced = true
	return a
}

// Error returns the newline-joined messages of every error added so far.
func (a *errAccumulator) Error() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.errs) == 0 {
		return ""
	}
	return errors.Join(a.errs...).Error()
}
