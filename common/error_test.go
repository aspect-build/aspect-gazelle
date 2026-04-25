package gazelle

import (
	"context"
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
)

func TestSetupCancellableContext_RegistersExts(t *testing.T) {
	c := config.New()
	ctx := SetupCancellableContext(c, context.Background())

	if _, ok := c.Exts[gazelleContextKey].(context.Context); !ok {
		t.Errorf("expected %s to hold a context.Context", gazelleContextKey)
	}
	if _, ok := c.Exts[gazelleContextCancelKey].(context.CancelCauseFunc); !ok {
		t.Errorf("expected %s to hold a context.CancelCauseFunc", gazelleContextCancelKey)
	}
	if _, ok := c.Exts[gazelleContextCancelErrorsKey].(*errAccumulator); !ok {
		t.Errorf("expected %s to hold an *errAccumulator", gazelleContextCancelErrorsKey)
	}
	if ctx.Err() != nil {
		t.Errorf("context should not be cancelled at setup, got %v", ctx.Err())
	}
}

func TestCheckCancellation_NilBeforeError(t *testing.T) {
	c := config.New()
	SetupCancellableContext(c, context.Background())

	if err := CheckCancellation(c); err != nil {
		t.Errorf("expected nil before any error, got %v", err)
	}
}

func TestGenerationErrorf_CancelsAndReportsMessage(t *testing.T) {
	c := config.New()
	ctx := SetupCancellableContext(c, context.Background())

	GenerationErrorf(c, "boom %s", "kaboom")

	if ctx.Err() == nil {
		t.Fatal("expected context to be cancelled")
	}
	err := CheckCancellation(c)
	if err == nil {
		t.Fatal("expected non-nil error after cancellation")
	}
	if got := err.Error(); !strings.Contains(got, "boom kaboom") {
		t.Errorf("expected message to contain %q, got %q", "boom kaboom", got)
	}
}

func TestGenerationErrorf_JoinsMultipleErrors(t *testing.T) {
	c := config.New()
	SetupCancellableContext(c, context.Background())

	GenerationErrorf(c, "first")
	GenerationErrorf(c, "second")
	GenerationErrorf(c, "third")

	err := CheckCancellation(c)
	if err == nil {
		t.Fatal("expected non-nil error after cancellation")
	}
	got := err.Error()
	for _, want := range []string{"first", "second", "third"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected joined message to contain %q, got %q", want, got)
		}
	}
}

func TestCheckCancellation_SurfacesOnlyOnce(t *testing.T) {
	c := config.New()
	SetupCancellableContext(c, context.Background())
	GenerationErrorf(c, "x")

	if err := CheckCancellation(c); err == nil {
		t.Fatal("first call should return the accumulator")
	}
	if err := CheckCancellation(c); err != nil {
		t.Errorf("second call should return nil, got %v", err)
	}
}

func TestContextCause_TracksLateAdditions(t *testing.T) {
	c := config.New()
	ctx := SetupCancellableContext(c, context.Background())

	GenerationErrorf(c, "early")
	cause := context.Cause(ctx)
	GenerationErrorf(c, "late")

	got := cause.Error()
	if !strings.Contains(got, "early") || !strings.Contains(got, "late") {
		t.Errorf("expected cause to reflect both errors, got %q", got)
	}
}
