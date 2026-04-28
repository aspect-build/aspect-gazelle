package gazelle

import (
	"slices"
	"strings"
	"testing"
)

func TestAddTargetGlobNormalization(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{name: "leading ./ is stripped", in: "./foo.ts", want: "foo.ts"},
		{name: "leading ./ with wildcard", in: "./*.config.ts", want: "*.config.ts"},
		{name: "no leading ./ is preserved", in: "*.po.ts", want: "*.po.ts"},
		// path.Clean would collapse ".." segments and break valid doublestar patterns.
		{name: "..  segments are preserved", in: "foo/../bar.ts", want: "foo/../bar.ts"},
		// path.Clean("") returns ".", which would silently match nothing.
		{name: "empty glob is preserved", in: "", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newRootConfig()
			if err := c.addTargetGlob("custom", tc.in, false); err != nil {
				t.Fatalf("addTargetGlob(%q) error: %v", tc.in, err)
			}
			target := c.GetSourceTarget("custom")
			if target == nil {
				t.Fatalf("target %q not found", "custom")
			}
			if !slices.Equal(target.customSources, []string{tc.want}) {
				t.Errorf("customSources = %q, want [%q]", target.customSources, tc.want)
			}
		})
	}
}

// Mutations via addTargetGlob must not leak across configs or into DefaultSourceGlobs.
func TestNewRootConfig_TargetsAreDeepCopied(t *testing.T) {
	a := newRootConfig()
	b := newRootConfig()

	if a.targets[0] == b.targets[0] {
		t.Fatalf("expected distinct TargetGroup pointers across configs")
	}
	if a.targets[0] == DefaultSourceGlobs[0] {
		t.Fatalf("expected config target to differ from DefaultSourceGlobs entry")
	}

	if err := a.addTargetGlob(DefaultLibraryName, "x/**/*.ts", false); err != nil {
		t.Fatalf("addTargetGlob: %v", err)
	}
	if got := b.GetSourceTarget(DefaultLibraryName).customSources; len(got) != 0 {
		t.Errorf("mutation leaked into sibling config: %v", got)
	}
	if got := DefaultSourceGlobs[0].customSources; len(got) != 0 {
		t.Errorf("mutation leaked into DefaultSourceGlobs: %v", got)
	}
}

func TestAddTargetGlob(t *testing.T) {
	// An unmatched bracket is rejected by doublestar.ValidatePattern.
	const invalidGlob = "src/[broken"

	t.Run("new target with valid glob is created", func(t *testing.T) {
		c := newRootConfig()
		before := len(c.targets)

		if err := c.addTargetGlob("custom", "src/**/*.ts", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := len(c.targets); got != before+1 {
			t.Fatalf("expected %d targets, got %d", before+1, got)
		}
		added := c.targets[len(c.targets)-1]
		if added.name != "custom" || added.testonly {
			t.Fatalf("unexpected target: %+v", added)
		}
		if len(added.customSources) != 1 || added.customSources[0] != "src/**/*.ts" {
			t.Fatalf("expected one custom source, got %v", added.customSources)
		}
	})

	t.Run("new target with invalid glob is rejected", func(t *testing.T) {
		c := newRootConfig()
		before := len(c.targets)

		err := c.addTargetGlob("custom", invalidGlob, false)
		if err == nil {
			t.Fatalf("expected error for invalid glob, got nil")
		}
		if !strings.Contains(err.Error(), "Invalid target") {
			t.Fatalf("expected 'Invalid target' error, got: %v", err)
		}
		if got := len(c.targets); got != before {
			t.Fatalf("target list mutated on validation failure: before=%d after=%d", before, got)
		}
	})

	t.Run("existing target appends valid glob", func(t *testing.T) {
		c := newRootConfig()
		// {dirname} is the default library target name.
		if err := c.addTargetGlob(DefaultLibraryName, "a/**/*.ts", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := c.addTargetGlob(DefaultLibraryName, "b/**/*.ts", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		lib := c.GetSourceTarget(DefaultLibraryName)
		if lib == nil {
			t.Fatalf("library target missing")
		}
		want := []string{"a/**/*.ts", "b/**/*.ts"}
		if len(lib.customSources) != len(want) {
			t.Fatalf("expected %v, got %v", want, lib.customSources)
		}
		for i, s := range want {
			if lib.customSources[i] != s {
				t.Fatalf("customSources[%d]: expected %q, got %q", i, s, lib.customSources[i])
			}
		}
	})

	t.Run("existing target rejects invalid glob", func(t *testing.T) {
		c := newRootConfig()
		if err := c.addTargetGlob(DefaultLibraryName, "a/**/*.ts", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		before := append([]string(nil), c.GetSourceTarget(DefaultLibraryName).customSources...)

		err := c.addTargetGlob(DefaultLibraryName, invalidGlob, false)
		if err == nil {
			t.Fatalf("expected error for invalid glob, got nil")
		}
		if !strings.Contains(err.Error(), "Invalid target") {
			t.Fatalf("expected 'Invalid target' error, got: %v", err)
		}

		got := c.GetSourceTarget(DefaultLibraryName).customSources
		if len(got) != len(before) {
			t.Fatalf("customSources mutated on validation failure: before=%v after=%v", before, got)
		}
	})

	t.Run("rootDirVar glob is accepted and stored unsubstituted", func(t *testing.T) {
		c := newRootConfig()
		// rootDirVar (${rootDir}) is the placeholder used by default globs;
		// custom globs may use it too. Substitution happens later, in
		// newSourceTargetClassifier — addTargetGlob must accept it as-is.
		glob := rootDirVar + "/**/*.ts"
		if err := c.addTargetGlob("custom", glob, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		added := c.GetSourceTarget("custom")
		if added == nil {
			t.Fatalf("custom target missing")
		}
		if len(added.customSources) != 1 || added.customSources[0] != glob {
			t.Fatalf("expected stored glob %q, got %v", glob, added.customSources)
		}
	})

	t.Run("rootDirVar glob with alternation is accepted", func(t *testing.T) {
		c := newRootConfig()
		glob := rootDirVar + "/**/*.{spec,test}.{ts,tsx}"
		if err := c.addTargetGlob(DefaultTestsName, glob, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tests := c.GetSourceTarget(DefaultTestsName)
		if len(tests.customSources) != 1 || tests.customSources[0] != glob {
			t.Fatalf("expected stored glob %q, got %v", glob, tests.customSources)
		}
	})

	t.Run("testonly mismatch returns error", func(t *testing.T) {
		c := newRootConfig()
		// DefaultLibraryName is a non-test target; passing isTestOnly=true must fail.
		err := c.addTargetGlob(DefaultLibraryName, "a/**/*.ts", true)
		if err == nil {
			t.Fatalf("expected testonly mismatch error, got nil")
		}
		if !strings.Contains(err.Error(), "can not override") {
			t.Fatalf("expected 'can not override' error, got: %v", err)
		}

		// And the inverse: a test target rejects a non-test glob.
		err = c.addTargetGlob(DefaultTestsName, "a/**/*.ts", false)
		if err == nil {
			t.Fatalf("expected testonly mismatch error, got nil")
		}
	})
}
