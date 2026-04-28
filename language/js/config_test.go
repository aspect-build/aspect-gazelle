package gazelle

import (
	"slices"
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
