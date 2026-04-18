package main

import (
	"reflect"
	"testing"

	"github.com/aspect-build/aspect-gazelle/runner"
)

// TestEnvLanguages_Defaults verifies the baked-in default language set. The
// aspect-gazelle defaults must start with the upstream bazel-gazelle defaults
// (visibility, proto, go) in the same order, since gazelle language ordering
// affects resolution precedence.
//
// https://github.com/bazel-contrib/bazel-gazelle/blob/v0.47.0/def.bzl#L59-L63
func TestEnvLanguages_Defaults(t *testing.T) {
	wantPrefix := []runner.GazelleLanguage{
		runner.DefaultVisibility,
		runner.Protobuf,
		runner.Go,
	}
	if len(envLanguages) < len(wantPrefix) {
		t.Fatalf("envLanguages too short: got %v", envLanguages)
	}
	for i, want := range wantPrefix {
		if envLanguages[i] != want {
			t.Errorf("envLanguages[%d]: got %q, want %q", i, envLanguages[i], want)
		}
	}
}

func TestResolveLanguages(t *testing.T) {
	defaults := []string{runner.DefaultVisibility, runner.Protobuf, runner.Go}

	cases := []struct {
		name        string
		enableLangs string
		orionExt    string
		orionExtDir string
		want        []string
	}{
		{
			name: "empty ENABLE_LANGUAGES returns defaults",
			want: defaults,
		},
		// Orion auto-add applies to defaults too, so setting only
		// ORION_EXTENSIONS[_DIR] without ENABLE_LANGUAGES still works.
		{
			name:     "empty ENABLE_LANGUAGES with ORION_EXTENSIONS adds orion to defaults",
			orionExt: "plugin.star",
			want:     append(append([]string{}, defaults...), runner.Orion),
		},
		{
			name:        "empty ENABLE_LANGUAGES with ORION_EXTENSIONS_DIR adds orion to defaults",
			orionExtDir: "plugins/",
			want:        append(append([]string{}, defaults...), runner.Orion),
		},

		// Explicit list replaces defaults entirely.
		{
			name:        "single language",
			enableLangs: runner.JavaScript,
			want:        []string{runner.JavaScript},
		},
		{
			name:        "comma-separated list preserves order",
			enableLangs: runner.JavaScript + "," + runner.Python + "," + runner.Go,
			want:        []string{runner.JavaScript, runner.Python, runner.Go},
		},

		// Orion auto-add when an orion-env var is set.
		{
			name:        "ORION_EXTENSIONS auto-adds orion",
			enableLangs: runner.JavaScript,
			orionExt:    "plugin.star",
			want:        []string{runner.JavaScript, runner.Orion},
		},
		{
			name:        "ORION_EXTENSIONS_DIR auto-adds orion",
			enableLangs: runner.JavaScript,
			orionExtDir: "plugins/",
			want:        []string{runner.JavaScript, runner.Orion},
		},
		{
			name:        "both ORION_EXTENSIONS vars set adds orion once",
			enableLangs: runner.JavaScript,
			orionExt:    "plugin.star",
			orionExtDir: "plugins/",
			want:        []string{runner.JavaScript, runner.Orion},
		},
		{
			name:        "orion already present is not duplicated",
			enableLangs: runner.Orion + "," + runner.JavaScript,
			orionExt:    "plugin.star",
			want:        []string{runner.Orion, runner.JavaScript},
		},
		{
			name:        "orion not added when orion-env unset",
			enableLangs: runner.JavaScript,
			want:        []string{runner.JavaScript},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveLanguages(defaults, tc.enableLangs, tc.orionExt, tc.orionExtDir)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// Guard against append() aliasing the defaults slice when auto-adding orion.
func TestResolveLanguages_DoesNotMutateDefaults(t *testing.T) {
	defaults := []string{"a", "b", "c"}
	_ = resolveLanguages(defaults, "x,y", "ext", "")
	if !reflect.DeepEqual(defaults, []string{"a", "b", "c"}) {
		t.Errorf("defaults was mutated: %v", defaults)
	}
}
